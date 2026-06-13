// Command gengolden regenerates the parity golden fixtures in testdata/golden by
// running the vendored Python RenderCV (third_party/rendercv) over the corpus
// declared in testdata/corpus.json.
//
// Goldens are the parity contract in file form (specs/000-parity-contract/spec.md).
// They are NEVER hand-written or hand-edited; this program is their only author.
// Regenerating them changes the contract — see
// .claude/skills/rendercv-golden-refresh.
//
// Usage:
//
//	go run ./tools/gengolden                # regenerate every case
//	go run ./tools/gengolden -case theme_ink
//	go run ./tools/gengolden -verify        # regenerate to a temp dir, compare, write nothing
//	go run ./tools/gengolden -upstream /abs/path/to/third_party/rendercv
//
// -verify byte-compares every golden against the committed manifest, with two
// named exceptions carrying D-011's non-reproducible Python tracebacks — see
// nonReproducibleStreams.
//
// The -upstream flag exists for git worktrees, which do not carry the
// submodule's checkout. Two goldens are Rich-rendered Python tracebacks that
// bake the *interpreter's* view of the source files (D-011), so generating from
// a worktree would rewrite them with a directory that is about to be deleted;
// pointing at the main checkout keeps them stable. specs/STATE.md records the
// same lesson — drive upstream by absolute path, never by a symlink.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image/png"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/conformance/cmdpanel"
	"github.com/nonamecat19/rendercv-go/internal/conformance/workroot"
)

const (
	upstreamDir = "third_party/rendercv"
	corpusPath  = "testdata/corpus.json"
	goldenDir   = "testdata/golden"
	// workDir holds the scratch golden tree only. The directory the *cases*
	// run in is workroot.Root, outside the checkout, because its absolute path
	// is recorded in the goldens — see that package's doc comment.
	workDir = "testdata/.work"
)

// Corpus is testdata/corpus.json.
type Corpus struct {
	Env   map[string]string `json:"env"`
	Cases []Case            `json:"cases"`
}

// Case is one conformance scenario: a set of input files plus one CLI invocation.
type Case struct {
	Name string `json:"name"`
	// Axis names the parity axis this case exercises: "artifacts", "cli" or "errors".
	Axis string `json:"axis"`
	// Files are copied from the upstream submodule into the case's working directory.
	Files []FileRef `json:"files"`
	// InlineFiles are written verbatim into the working directory.
	InlineFiles []InlineFile `json:"inline_files"`
	// Args is the upstream CLI invocation, without the program name.
	Args []string `json:"args"`
	// ExpectExit is the required exit code, or nil to accept whatever upstream returns.
	ExpectExit *int `json:"expect_exit"`
	// NoCurrentDatePin opts a case out of pinCurrentDate's auto-injected
	// `--settings.current_date`. It exists for the one class of case where that
	// injection is wrong rather than merely redundant: a case whose whole point
	// is `settings.current_date` itself. The CLI override is written into the
	// document before validation runs (spec 006 delta §9), so silently adding
	// it would overwrite the very value the case means to exercise — the
	// golden would capture a pinned, unrelated render instead of the
	// validation record `TestParity` actually needs to compare against, since
	// `TestParity` runs the args exactly as `corpus.json` declares them, not as
	// this tool's own pinning mutates them.
	NoCurrentDatePin bool `json:"no_current_date_pin,omitempty"`
}

// FileRef copies src (relative to the upstream submodule) to dst (relative to the case dir).
type FileRef struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}

// InlineFile writes Content to Dst (relative to the case dir).
type InlineFile struct {
	Dst     string `json:"dst"`
	Content string `json:"content"`
}

// Manifest records what the goldens were generated from, so drift is detectable.
type Manifest struct {
	UpstreamSHA     string            `json:"upstream_sha"`
	UpstreamVersion string            `json:"upstream_version"`
	Generator       string            `json:"generator"`
	CaseCount       int               `json:"case_count"`
	Files           map[string]string `json:"files"` // path relative to goldenDir -> sha256
}

func main() {
	var (
		only   = flag.String("case", "", "regenerate only this case")
		verify = flag.Bool("verify", false, "compare against committed goldens; write nothing")
		up     = flag.String("upstream", "",
			"absolute path to the vendored RenderCV checkout (default: <repo>/"+upstreamDir+")")
	)
	flag.Parse()

	if err := run(*only, *verify, *up); err != nil {
		fmt.Fprintf(os.Stderr, "gengolden: %v\n", err)
		os.Exit(1)
	}
}

func run(only string, verify bool, upstream string) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	if upstream == "" {
		upstream = filepath.Join(root, upstreamDir)
	}

	bin := filepath.Join(upstream, ".venv", "bin", "rendercv")
	if runtime.GOOS == "windows" {
		bin = filepath.Join(upstream, ".venv", "Scripts", "rendercv.exe")
	}
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("vendored RenderCV not installed at %s: run `just setup` (%w)", bin, err)
	}

	corpus, err := loadCorpus(filepath.Join(root, corpusPath))
	if err != nil {
		return err
	}

	sha, version, err := upstreamPin(upstream)
	if err != nil {
		return err
	}

	// Everything is generated into a scratch tree first. Only a successful full run
	// is promoted over the committed goldens, so a crash mid-run cannot leave the
	// contract half-rewritten.
	scratch := filepath.Join(root, workDir, "golden")
	if err := os.RemoveAll(scratch); err != nil {
		return err
	}
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		return err
	}

	manifest := Manifest{
		UpstreamSHA:     sha,
		UpstreamVersion: version,
		Generator:       "tools/gengolden",
		Files:           map[string]string{},
	}

	ran := 0
	for _, c := range corpus.Cases {
		if only != "" && c.Name != only {
			continue
		}
		if err := generateCase(upstream, bin, corpus.Env, c, scratch); err != nil {
			return fmt.Errorf("case %s: %w", c.Name, err)
		}
		ran++
	}
	if ran == 0 {
		return fmt.Errorf("no cases matched %q", only)
	}
	manifest.CaseCount = ran

	if err := hashTree(scratch, manifest.Files); err != nil {
		return err
	}

	if verify {
		return verifyAgainstCommitted(root, scratch, manifest, only)
	}

	if err := writeManifest(filepath.Join(scratch, "manifest.json"), manifest); err != nil {
		return err
	}
	if err := promote(scratch, filepath.Join(root, goldenDir), only); err != nil {
		return err
	}

	fmt.Printf("regenerated %d case(s) from upstream %s (%s)\n", ran, short(sha), version)
	fmt.Println("goldens changed the parity contract — review every diff before committing")
	return nil
}

// generateCase runs one corpus case in an isolated directory and captures everything
// the run produced: created files, stdout, stderr and the exit code.
func generateCase(upstream, bin string, env map[string]string, c Case, scratch string) error {
	caseWork, release, err := workroot.Prepare(c.Name)
	if err != nil {
		return err
	}
	defer release()

	for _, f := range c.Files {
		src := filepath.Join(upstream, f.Src)
		dst := filepath.Join(caseWork, f.Dst)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy %s: %w", f.Src, err)
		}
	}
	for _, f := range c.InlineFiles {
		dst := filepath.Join(caseWork, f.Dst)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, []byte(f.Content), 0o644); err != nil {
			return err
		}
	}

	// Snapshot the inputs so we can tell afterwards which files the run created.
	before, err := treeSet(caseWork)
	if err != nil {
		return err
	}

	var stdout, stderr strings.Builder
	if !c.NoCurrentDatePin {
		c.Args = pinCurrentDate(c.Args)
	}
	cmd := exec.Command(bin, c.Args...)
	cmd.Dir = caseWork
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = childEnv(env)

	exitCode := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			return fmt.Errorf("running upstream: %w", err)
		}
		exitCode = ee.ExitCode()
	}
	if c.ExpectExit != nil && exitCode != *c.ExpectExit {
		return fmt.Errorf("exit code %d, corpus expects %d\nstdout:\n%s\nstderr:\n%s",
			exitCode, *c.ExpectExit, stdout.String(), stderr.String())
	}

	out := filepath.Join(scratch, c.Name)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}

	after, err := treeSet(caseWork)
	if err != nil {
		return err
	}
	created := make([]string, 0, len(after))
	for p := range after {
		if !before[p] {
			created = append(created, p)
		}
	}
	sort.Strings(created)

	// PNG bytes are excluded from the golden tree: they are ~90% of its size and PNG
	// parity is specified on page count and pixel dimensions only
	// (specs/000-parity-contract/spec.md §1.2). Their geometry is captured instead.
	var pngMeta []string
	for _, rel := range created {
		src := filepath.Join(caseWork, rel)
		if strings.EqualFold(filepath.Ext(rel), ".png") {
			w, h, err := pngGeometry(src)
			if err != nil {
				return fmt.Errorf("reading %s: %w", rel, err)
			}
			pngMeta = append(pngMeta, fmt.Sprintf("%s %dx%d", rel, w, h))
			continue
		}
		dst := filepath.Join(out, "files", rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(out, "pngs.txt"),
		[]byte(strings.Join(pngMeta, "\n")+"\n"), 0o644); err != nil {
		return err
	}

	// The file list is itself part of the contract: output layout and naming must match.
	if err := os.WriteFile(filepath.Join(out, "files.txt"),
		[]byte(strings.Join(created, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "stdout.txt"),
		[]byte(normalize(stdout.String())), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "stderr.txt"),
		[]byte(normalize(stderr.String())), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "exit_code.txt"),
		[]byte(fmt.Sprintf("%d\n", exitCode)), 0o644); err != nil {
		return err
	}

	meta, err := json.MarshalIndent(map[string]any{
		"name": c.Name,
		"axis": c.Axis,
		"args": c.Args,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "case.json"), append(meta, '\n'), 0o644)
}

// durationPattern matches the per-step timings RenderCV prints ("42 ms", "1.2 s"),
// together with the padding that follows them. Timings are wall-clock and are not
// part of the contract; the padding must be absorbed with them, because the column
// is right-padded to a fixed width and a shorter timing means more spaces.
var durationPattern = regexp.MustCompile(`\b\d+(\.\d+)?\s?(ms|s)\b[ \t]*`)

// normalize strips the parts of CLI output that vary between runs on the same input,
// or between checkouts of the same upstream commit. Everything else — wording, layout,
// box drawing — is contractual, and so is every ordering upstream's source states:
// cmdpanel.Sort touches the one panel whose order upstream reads off a directory
// listing instead (D-019), and no other.
//
// internal/conformance applies the identical transform to rendercv-go's output, so
// any change here must be mirrored there.
//
// **It does not append a trailing newline.** It used to, and so did the
// harness, on both sides of every comparison — which made the last byte of
// every golden unverifiable by construction: a stream ending without a newline
// was recorded as if it ended with one, and a port emitting the wrong final
// byte compared equal. 23 of the corpus's 42 recorded streams genuinely end
// without a newline.
func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = durationPattern.ReplaceAllString(s, "<duration> ")
	return cmdpanel.Sort(s)
}

// pngGeometry reads a PNG's pixel dimensions without decoding the image.
func pngGeometry(path string) (width, height int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = f.Close() }()

	cfg, err := png.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

// childEnv builds a minimal, deterministic environment. Terminal width in particular
// changes Rich's table layout, so it is pinned by the corpus.
func childEnv(env map[string]string) []string {
	out := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"LC_ALL=C.UTF-8",
		"LANG=C.UTF-8",
		"PYTHONHASHSEED=0",
		"PYTHONUTF8=1",
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, k+"="+env[k])
	}
	return out
}

// nonReproducibleStream is a golden stream whose bytes `-verify` may not compare,
// together with the check that stands in for the comparison.
//
// **Exactly two streams qualify, and only under D-011.** Both are Rich-rendered
// Python tracebacks of an *unhandled* exception — not a `RenderCVUserError`
// panel — so the interpreter bakes absolute filesystem paths into them twice
// over: once per stack frame (`…/third_party/rendercv/src/rendercv/…`, the
// generating checkout) and once for the CPython frames under the uv-managed
// interpreter (`~/.local/share/uv/python/cpython-3.12.13-linux-x86_64-gnu/…`).
// `specs/divergences.md` D-011 calls this out as "non-reproducible even for
// upstream: regenerating them on a different checkout or machine produces
// different bytes, which is not true of any other golden in the corpus."
//
// The work-root change (`internal/conformance/workroot`) fixed the *third* path
// these goldens carry — the directory the case ran in — and reaches neither of
// the other two. So the byte comparison could never pass off the machine that
// generated the goldens, which is every CI runner.
//
// Everything else about these two cases stays byte-compared: their `stdout.txt`,
// `exit_code.txt`, `files.txt`, `pngs.txt` and `case.json` are not listed here,
// so a crash, a stream swapped for the other, or a changed exit code still fails
// `-verify` exactly as before. What is checked here instead is that both the
// committed and the freshly generated stream are still recognizably *this*
// traceback.
type nonReproducibleStream struct {
	// Path is relative to goldenDir, in the OS's separator, as the manifest spells it.
	Path string
	// Divergence is the approved entry that makes the bytes non-reproducible.
	Divergence string
	// Must are substrings every rendering of this stream contains regardless of
	// which machine produced it. Each is short enough that Rich's fixed-width
	// wrapping cannot split it.
	Must []string
}

// nonReproducibleStreams is the whole exemption list. Two entries, one divergence.
//
// It is a Go literal rather than corpus data on purpose: the corpus describes
// what upstream does, and "these bytes depend on where the interpreter lives" is
// a property of the fixture, not of upstream's behavior.
var nonReproducibleStreams = []nonReproducibleStream{
	{
		Path:       filepath.Join("err_missing_file", "stderr.txt"),
		Divergence: "D-011",
		Must: []string{
			"Traceback (most recent call last)",
			"FileNotFoundError: [Errno 2] No such file or directory:",
		},
	},
	{
		Path:       filepath.Join("err_bad_override_key", "stderr.txt"),
		Divergence: "D-011",
		Must: []string{
			"Traceback (most recent call last)",
			"ValidationError: 1 validation error for RenderCVModel",
			"KeyError: 'no_such_field'",
		},
	},
}

// isNonReproducible reports whether path is exempt from the byte comparison.
func isNonReproducible(path string) bool {
	_, ok := nonReproducibleFor(path)
	return ok
}

func nonReproducibleFor(path string) (nonReproducibleStream, bool) {
	for _, s := range nonReproducibleStreams {
		if s.Path == path {
			return s, true
		}
	}
	return nonReproducibleStream{}, false
}

// checkNonReproducible applies the weaker check to both copies of an exempt
// stream: the committed golden and the one this run just generated. Both must
// still be the traceback the corpus case is about.
func checkNonReproducible(committedDir, scratch, path string) []string {
	s, ok := nonReproducibleFor(path)
	if !ok {
		return []string{"not an exempt stream: " + path}
	}
	var problems []string
	for origin, dir := range map[string]string{
		"the committed golden": committedDir,
		"the generated stream": scratch,
	} {
		raw, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s %s is unreadable: %v", origin, path, err))
			continue
		}
		problems = append(problems, checkStreamContent(s, origin, string(raw))...)
	}
	sort.Strings(problems)
	return problems
}

// checkStreamContent is the check itself, kept pure so it can be tested without
// a filesystem: non-empty, and carrying every marker the stream must have.
func checkStreamContent(s nonReproducibleStream, origin, content string) []string {
	if strings.TrimSpace(content) == "" {
		return []string{fmt.Sprintf(
			"%s %s is empty; %s exempts its bytes from comparison, not its existence",
			origin, s.Path, s.Divergence)}
	}
	var problems []string
	for _, marker := range s.Must {
		if !strings.Contains(content, marker) {
			problems = append(problems, fmt.Sprintf(
				"%s %s no longer contains %q; %s exempts only the machine-specific paths",
				origin, s.Path, marker, s.Divergence))
		}
	}
	return problems
}

func verifyAgainstCommitted(root, scratch string, got Manifest, only string) error {
	committedPath := filepath.Join(root, goldenDir, "manifest.json")
	raw, err := os.ReadFile(committedPath)
	if err != nil {
		return fmt.Errorf("no committed manifest at %s: run `just golden` first (%w)", committedPath, err)
	}
	var want Manifest
	if err := json.Unmarshal(raw, &want); err != nil {
		return err
	}

	var problems []string
	if want.UpstreamSHA != got.UpstreamSHA {
		problems = append(problems, fmt.Sprintf(
			"upstream pin moved: goldens generated from %s, submodule is at %s",
			short(want.UpstreamSHA), short(got.UpstreamSHA)))
	}

	for path, sum := range got.Files {
		if only != "" && !strings.HasPrefix(path, only+string(os.PathSeparator)) {
			continue
		}
		w, ok := want.Files[path]
		switch {
		case !ok:
			problems = append(problems, "golden missing from the committed set: "+path)
		case isNonReproducible(path):
			// D-011: these bytes are a property of the machine, not of upstream.
			// The presence check above still applies; the byte check is replaced.
			problems = append(problems, checkNonReproducible(
				filepath.Join(root, goldenDir), scratch, path)...)
		case w != sum:
			problems = append(problems, "golden differs from the committed bytes: "+path)
		}
	}
	if only == "" {
		for path := range want.Files {
			if _, ok := got.Files[path]; !ok {
				problems = append(problems, "committed golden is no longer generated: "+path)
			}
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("goldens are out of sync with the pinned upstream:\n  %s",
			strings.Join(problems, "\n  "))
	}
	fmt.Printf("goldens match the pinned upstream %s (%d files)\n", short(got.UpstreamSHA), len(got.Files))
	return nil
}

// promote replaces the committed goldens with the freshly generated tree.
func promote(scratch, dst, only string) error {
	if only != "" {
		src := filepath.Join(scratch, only)
		target := filepath.Join(dst, only)
		if err := os.RemoveAll(target); err != nil {
			return err
		}
		return copyTree(src, target)
	}
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return copyTree(scratch, dst)
}

func loadCorpus(path string) (*Corpus, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Corpus
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	seen := map[string]bool{}
	for _, cs := range c.Cases {
		if cs.Name == "" {
			return nil, errors.New("corpus contains a case with no name")
		}
		if seen[cs.Name] {
			return nil, fmt.Errorf("duplicate case name %q", cs.Name)
		}
		seen[cs.Name] = true
	}
	return &c, nil
}

func upstreamPin(dir string) (sha, version string, err error) {
	sha, err = git(dir, "rev-parse", "HEAD")
	if err != nil {
		return "", "", err
	}
	version, err = git(dir, "describe", "--tags", "--always")
	if err != nil {
		return "", "", err
	}
	dirty, err := git(dir, "status", "--porcelain")
	if err != nil {
		return "", "", err
	}
	if dirty != "" {
		return "", "", fmt.Errorf(
			"%s has local modifications; the vendored upstream must be pristine (AGENTS.md §10.4):\n%s",
			dir, dirty)
	}
	return sha, version, nil
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; ; {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found above %s", wd)
		}
		dir = parent
	}
}

// PinnedCurrentDate is what every `render` case is generated with.
//
// **Without it the goldens expire.** `settings.current_date` defaults to
// `today`, so the footer and every `end_date: present` duration baked the day
// the corpus was generated: 19 goldens carried `Aug 2026` and a `3 years 3
// months` span that grew each month. `tools/docprobe` learned this first and
// pins the same date; this is the corpus catching up.
const PinnedCurrentDate = "2025-03-05"

// pinCurrentDate appends the pin to a `render` invocation.
//
// **Only `render`.** The `new`, `create-theme` and `--help` cases take no such
// flag, and adding it would change output that has nothing to do with dates.
// The flag lands in the case's own `args`, so the conformance harness replays
// the identical command — the pin is part of the case, not a hidden setting.
func pinCurrentDate(args []string) []string {
	if len(args) == 0 || args[0] != "render" {
		return args
	}
	for _, arg := range args {
		if arg == "--settings.current_date" {
			return args
		}
	}
	return append(append([]string(nil), args...), "--settings.current_date", PinnedCurrentDate)
}

// treeSet returns every regular file under dir, as slash-free relative paths.
func treeSet(dir string) (map[string]bool, error) {
	set := map[string]bool{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		set[rel] = true
		return nil
	})
	return set, err
}

func hashTree(dir string, into map[string]string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		sum, err := sha256File(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		into[rel] = sum
		return nil
	})
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func writeManifest(path string, m Manifest) error {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// Command gengolden regenerates the parity golden fixtures in testdata/golden by
// running the vendored Python RenderCV (third_party/rendercv) over the corpus
// declared in testdata/corpus.json.
//
// Goldens are the parity contract in file form (specs/000-parity-contract/spec.md).
// They are NEVER hand-written or hand-edited; this program is their only author.
// Regenerating them changes the contract and is human-gated — see
// .claude/skills/rendercv-golden-refresh.
//
// Usage:
//
//	go run ./tools/gengolden                # regenerate every case
//	go run ./tools/gengolden -case theme_ink
//	go run ./tools/gengolden -verify        # regenerate to a temp dir, compare, write nothing
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
)

const (
	upstreamDir = "third_party/rendercv"
	corpusPath  = "testdata/corpus.json"
	goldenDir   = "testdata/golden"
	workDir     = "testdata/.work"
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
	)
	flag.Parse()

	if err := run(*only, *verify); err != nil {
		fmt.Fprintf(os.Stderr, "gengolden: %v\n", err)
		os.Exit(1)
	}
}

func run(only string, verify bool) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}

	bin := filepath.Join(root, upstreamDir, ".venv", "bin", "rendercv")
	if runtime.GOOS == "windows" {
		bin = filepath.Join(root, upstreamDir, ".venv", "Scripts", "rendercv.exe")
	}
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("vendored RenderCV not installed at %s: run `just setup` (%w)", bin, err)
	}

	corpus, err := loadCorpus(filepath.Join(root, corpusPath))
	if err != nil {
		return err
	}

	sha, version, err := upstreamPin(root)
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
		if err := generateCase(root, bin, corpus.Env, c, scratch); err != nil {
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
func generateCase(root, bin string, env map[string]string, c Case, scratch string) error {
	caseWork := filepath.Join(root, workDir, "run", c.Name)
	if err := os.RemoveAll(caseWork); err != nil {
		return err
	}
	if err := os.MkdirAll(caseWork, 0o755); err != nil {
		return err
	}

	for _, f := range c.Files {
		src := filepath.Join(root, upstreamDir, f.Src)
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

// normalize strips the parts of CLI output that vary between runs on the same input.
// Everything else — wording, layout, box drawing, ordering — is contractual.
//
// internal/conformance applies the identical transform to rendercv-go's output, so
// any change here must be mirrored there.
func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = durationPattern.ReplaceAllString(s, "<duration> ")
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

// pngGeometry reads a PNG's pixel dimensions without decoding the image.
func pngGeometry(path string) (width, height int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

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

func upstreamPin(root string) (sha, version string, err error) {
	dir := filepath.Join(root, upstreamDir)
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
			upstreamDir, dirty)
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
	defer f.Close()

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
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
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

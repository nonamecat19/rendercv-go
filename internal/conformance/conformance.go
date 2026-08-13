// Package conformance provides the helpers that measure the parity contract
// (specs/000-parity-contract/spec.md): it loads the golden fixtures produced by
// tools/gengolden from the vendored Python RenderCV, runs rendercv-go over the same
// corpus, and compares the two.
//
// This package is the only sanctioned way to assert parity. Tests that compare
// output by hand tend to normalise away exactly the whitespace the contract is about.
package conformance

import (
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/conformance/workroot"
)

const (
	goldenDir  = "testdata/golden"
	corpusFile = "testdata/corpus.json"
	binaryPath = "bin/rendercv-go"
	upstream   = "third_party/rendercv"
)

// Corpus mirrors testdata/corpus.json.
type Corpus struct {
	Env   map[string]string `json:"env"`
	Cases []Case            `json:"cases"`
}

// Case is one conformance scenario.
type Case struct {
	Name        string       `json:"name"`
	Axis        string       `json:"axis"`
	Files       []FileRef    `json:"files"`
	InlineFiles []InlineFile `json:"inline_files"`
	Args        []string     `json:"args"`
	ExpectExit  *int         `json:"expect_exit"`
}

// FileRef copies Src (relative to the upstream submodule) to Dst (relative to the case dir).
type FileRef struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}

// InlineFile writes Content to Dst (relative to the case dir).
type InlineFile struct {
	Dst     string `json:"dst"`
	Content string `json:"content"`
}

// Golden is the recorded upstream behavior for one case.
type Golden struct {
	Name     string
	dir      string
	Files    []string          // paths the run created, in sorted order
	PNGs     map[string]string // path -> "WxH"
	Stdout   string
	Stderr   string
	ExitCode int
}

// Result is what rendercv-go did for one case.
type Result struct {
	Files    []string
	PNGs     map[string]string
	Stdout   string
	Stderr   string
	ExitCode int
	dir      string
}

// LoadCorpus reads testdata/corpus.json.
func LoadCorpus(t *testing.T) Corpus {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(RepoRoot(t), corpusFile))
	if err != nil {
		t.Fatalf("reading corpus: %v", err)
	}
	var c Corpus
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("parsing corpus: %v", err)
	}
	return c
}

// LoadGolden reads the recorded upstream behavior for a case. It fails the test if the
// golden is missing, because a missing golden means the contract was never recorded —
// never because the case is optional.
func LoadGolden(t *testing.T, name string) Golden {
	t.Helper()

	dir := filepath.Join(RepoRoot(t), goldenDir, name)
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("no golden for case %q at %s — run `just golden` (%v)", name, dir, err)
	}

	g := Golden{Name: name, dir: dir, PNGs: map[string]string{}}
	g.Files = readLines(t, filepath.Join(dir, "files.txt"))
	g.Stdout = readFile(t, filepath.Join(dir, "stdout.txt"))
	g.Stderr = readFile(t, filepath.Join(dir, "stderr.txt"))

	code, err := strconv.Atoi(strings.TrimSpace(readFile(t, filepath.Join(dir, "exit_code.txt"))))
	if err != nil {
		t.Fatalf("case %s: parsing exit_code.txt: %v", name, err)
	}
	g.ExitCode = code

	for _, line := range readLines(t, filepath.Join(dir, "pngs.txt")) {
		path, geom, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("case %s: malformed pngs.txt line %q", name, line)
		}
		g.PNGs[path] = geom
	}
	return g
}

// Artifact returns the recorded bytes of one file the upstream run created.
func (g Golden) Artifact(t *testing.T, rel string) []byte {
	t.Helper()

	raw, err := g.artifactBytes(rel)
	if err != nil {
		t.Fatalf("case %s: golden artifact %s: %v", g.Name, rel, err)
	}
	return raw
}

// artifactBytes is Artifact without a *testing.T, for the comparisons that must
// weigh a read failure themselves rather than end the test.
func (g Golden) artifactBytes(rel string) ([]byte, error) {
	return os.ReadFile(filepath.Join(g.dir, "files", rel))
}

// Run executes rendercv-go for a case in an isolated directory and captures everything
// it produced, in the same shape gengolden captured for upstream.
func Run(t *testing.T, c Case, env map[string]string) Result {
	t.Helper()

	root := RepoRoot(t)
	bin := filepath.Join(root, binaryPath)
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("rendercv-go is not built at %s — run `just build` (%v)", bin, err)
	}

	dir := caseWorkDir(t, c.Name)
	for _, f := range c.Files {
		writeFile(t, filepath.Join(dir, f.Dst), readFile(t, filepath.Join(root, upstream, f.Src)))
	}
	for _, f := range c.InlineFiles {
		writeFile(t, filepath.Join(dir, f.Dst), f.Content)
	}

	before := treeSet(t, dir)

	var stdout, stderr strings.Builder
	cmd := exec.Command(bin, c.Args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = childEnv(env)

	code := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("case %s: running rendercv-go: %v", c.Name, err)
		}
		code = ee.ExitCode()
	}

	res := Result{
		Stdout:   Normalize(stdout.String()),
		Stderr:   Normalize(stderr.String()),
		ExitCode: code,
		PNGs:     map[string]string{},
		dir:      dir,
	}
	for _, rel := range treeSet(t, dir) {
		if before2 := contains(before, rel); before2 {
			continue
		}
		res.Files = append(res.Files, rel)
		if strings.EqualFold(filepath.Ext(rel), ".png") {
			res.PNGs[rel] = pngGeometry(t, filepath.Join(dir, rel))
		}
	}
	sort.Strings(res.Files)
	return res
}

// caseWorkDir is the directory a case runs in, and it is **the same path
// `tools/gengolden` used** — `workroot.CaseDir(name)` — rather than a
// `t.TempDir`.
//
// The reason is a defect in upstream's output, not a preference. Several
// goldens bake an absolute path into the text they compare: `err_unknown_theme`'s
// message names the custom theme folder it could not find, and upstream prints
// `custom_theme_folder.absolute()` (`design.py:79`). Run anywhere else, that
// line can never match no matter how correct the port is.
//
// The directory used to be `testdata/.work/run/<case>` under *the generating
// checkout*, which made the goldens pass on one machine and nowhere else — not
// from a git worktree, a second clone, or CI. `workroot.Root` is a fixed
// absolute path instead, so the recorded string is the same everywhere. See
// that package for the whole argument, and for the lock that keeps a shared
// path from being a shared race.
func caseWorkDir(t *testing.T, name string) string {
	t.Helper()

	dir, release, err := workroot.Prepare(name)
	if err != nil {
		t.Fatalf("case %s: %v", name, err)
	}
	// Released at the end of the test rather than when the command exits: the
	// comparisons below read the artifacts the run left in this directory.
	t.Cleanup(release)
	return dir
}

// Artifact returns the bytes rendercv-go wrote for one file.
func (r Result) Artifact(t *testing.T, rel string) []byte {
	t.Helper()

	raw, err := r.artifactBytes(rel)
	if err != nil {
		t.Fatalf("rendercv-go did not produce %s: %v", rel, err)
	}
	return raw
}

// artifactBytes is Artifact without a *testing.T. See Golden.artifactBytes.
func (r Result) artifactBytes(rel string) ([]byte, error) {
	return os.ReadFile(filepath.Join(r.dir, rel))
}

// durationPattern must stay identical to the one in tools/gengolden.
var durationPattern = regexp.MustCompile(`\b\d+(\.\d+)?\s?(ms|s)\b[ \t]*`)

// Normalize applies the same transform gengolden applied to upstream output: it removes
// wall-clock timings (and the padding that follows them) and nothing else. Any change
// here must be mirrored in tools/gengolden.
func Normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = durationPattern.ReplaceAllString(s, "<duration> ")
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

// RepoRoot walks up from the test's working directory to the directory holding go.mod.
func RepoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for dir := wd; ; {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod found above %s", wd)
		}
		dir = parent
	}
}

func childEnv(env map[string]string) []string {
	out := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"LC_ALL=C.UTF-8",
		"LANG=C.UTF-8",
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

func treeSet(t *testing.T, dir string) []string {
	t.Helper()

	var out []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	sort.Strings(out)
	return out
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func pngGeometry(t *testing.T, path string) string {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	cfg, err := png.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
	return fmt.Sprintf("%dx%d", cfg.Width, cfg.Height)
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(raw)
}

func readLines(t *testing.T, path string) []string {
	t.Helper()

	var out []string
	for _, line := range strings.Split(readFile(t, path), "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

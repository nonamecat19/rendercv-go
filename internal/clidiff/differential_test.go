//go:build conformance

package clidiff

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

const (
	// upstreamBinary is the vendored Python CLI, the specification itself
	// (AGENTS.md §1). It is absent until `just setup` has run `uv sync` inside
	// the submodule, so every test here skips rather than fails on it.
	upstreamBinary = "third_party/rendercv/.venv/bin/rendercv"
	// portBinary is what `just build` writes.
	portBinary = "bin/rendercv-go"
)

// scratchNames are the two run directories. **They are the same length on
// purpose.** Both sides run in a directory of their own (spec 013 §8), and any
// absolute path a panel prints is therefore the same number of bytes on both —
// so a byte-count comparison measures the port, not the temporary directory's
// name.
var scratchNames = [2]string{"up", "go"}

// invocation is one differential case: a file tree, an argument vector, and an
// optional extra step run inside the scratch directory before the binary.
type invocation struct {
	args    []string
	files   map[string]string
	prepare func(t *testing.T, dir string)
}

// outcome is everything one process did. Nothing here is normalized: no
// trailing newline is appended, no duration is erased, no CRLF is folded.
type outcome struct {
	stdout []byte
	stderr []byte
	exit   int
	tree   []string
	dir    string
}

// lastByte renders the final byte of s the way spec 013 §3.4's table does — two
// hex digits, or `-` for an empty stream.
func lastByte(s []byte) string {
	if len(s) == 0 {
		return "-"
	}
	return fmt.Sprintf("%02x", s[len(s)-1])
}

// stream names which of the two streams carried the output, which is the third
// column of behavior 31's table and a guarantee of its own (§6.4: no measured
// vector writes to both).
func (o outcome) stream() string {
	switch {
	case len(o.stdout) > 0 && len(o.stderr) > 0:
		return "both"
	case len(o.stdout) > 0:
		return "stdout"
	case len(o.stderr) > 0:
		return "stderr"
	default:
		return "silent"
	}
}

func (o outcome) String() string {
	return fmt.Sprintf("stdout=%dB/%s stderr=%dB/%s stream=%s exit=%d",
		len(o.stdout), lastByte(o.stdout), len(o.stderr), lastByte(o.stderr), o.stream(), o.exit)
}

// repoRoot walks up from the test's working directory to the directory holding
// go.mod.
func repoRoot(t *testing.T) string {
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

// skipEnvVar is the explicit opt-out that turns a missing binary back into a
// skip.
//
// The default is a hard failure, because a skip here is indistinguishable from
// a pass in the run that gates the iteration: `just test-parity` (`justfile:71`)
// depends on `build`, so the port binary is always present, but **nothing
// creates the vendored venv**, and a skipped differential still prints `ok`.
// A gate that disappears when its subject is missing is not a gate.
const skipEnvVar = "RENDERCV_DIFF_ALLOW_SKIP"

// binaries returns the absolute paths of the two CLIs. A missing binary fails
// the test unless skipEnvVar is set to a true boolean.
func binaries(t *testing.T) (upstream, port string) {
	t.Helper()

	root := repoRoot(t)
	upstream = filepath.Join(root, upstreamBinary)
	port = filepath.Join(root, portBinary)
	for _, bin := range []string{upstream, port} {
		if _, err := os.Stat(bin); err != nil {
			if allowSkip(t) {
				t.Skipf("%s is absent and %s is set (%v)", bin, skipEnvVar, err)
			}
			t.Fatalf("%s is absent, so this differential would silently not run — "+
				"run `just setup` and `just build`, or set %s=1 to skip it on purpose (%v)",
				bin, skipEnvVar, err)
		}
	}
	return upstream, port
}

// allowSkip reads skipEnvVar. An unparseable value is a failure rather than a
// silent false: a typo in the opt-out must not read as "run the gate" any more
// than it reads as "skip it".
func allowSkip(t *testing.T) bool {
	t.Helper()

	raw, ok := os.LookupEnv(skipEnvVar)
	if !ok || raw == "" {
		return false
	}
	allow, err := strconv.ParseBool(raw)
	if err != nil {
		t.Fatalf("%s=%q is not a boolean: %v", skipEnvVar, raw, err)
	}
	return allow
}

// childEnv is the smallest environment both binaries need, with COLUMNS pinned:
// Rich honours it even when stdout is a pipe (`internal/cli/panel.go:24`), and
// every byte count in spec 013 was measured at 80.
func childEnv() []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"COLUMNS=80",
		"LC_ALL=C.UTF-8",
		"LANG=C.UTF-8",
	}
}

// run executes one binary in a fresh scratch directory **outside the
// repository** (spec 013 §7.6) and captures its raw output and the file tree it
// left behind.
func run(t *testing.T, bin, dir string, inv invocation) outcome {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating %s: %v", dir, err)
	}
	for rel, content := range inv.files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	if inv.prepare != nil {
		inv.prepare(t, dir)
	}

	before := tree(t, dir)

	var stdout, stderr strings.Builder
	cmd := exec.Command(bin, inv.args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = childEnv()

	code := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("running %s %v: %v", bin, inv.args, err)
		}
		code = ee.ExitCode()
	}

	out := outcome{
		stdout: []byte(stdout.String()),
		stderr: []byte(stderr.String()),
		exit:   code,
		dir:    dir,
	}
	for _, rel := range tree(t, dir) {
		if !slices.Contains(before, rel) {
			out.tree = append(out.tree, rel)
		}
	}
	return out
}

// differential runs both binaries on the same invocation, in two sibling
// scratch directories under the OS temporary directory.
func differential(t *testing.T, inv invocation) (upstream, port outcome) {
	t.Helper()

	upstreamBin, portBin := binaries(t)
	root := t.TempDir()

	return run(t, upstreamBin, filepath.Join(root, scratchNames[0]), inv),
		run(t, portBin, filepath.Join(root, scratchNames[1]), inv)
}

// compare asserts the four dimensions spec 013 §8 names — byte count, last
// byte, stream, exit code — on both streams plus the created file tree, with no
// normalization of any kind.
//
// Nothing here is downgradable. An earlier version took a proposal name and
// weakened the stdout size check for the row P-3 covers; because that row also
// set neither rawEqual nor wantBytes, the size check was the *last* assertion
// reading those bytes and the row went blind to its own content — a same-length
// swap of the port's whole panel passed. A row that needs a narrower contract
// states it in an extra assertion (see osErrorRow), never by removing one.
func compare(t *testing.T, upstream, port outcome) {
	t.Helper()

	if len(upstream.stdout) != len(port.stdout) {
		t.Errorf("stdout byte count: upstream %d, port %d", len(upstream.stdout), len(port.stdout))
	}
	if lastByte(upstream.stdout) != lastByte(port.stdout) {
		t.Errorf("stdout last byte: upstream %s, port %s (§6.2: path A ends 0a, path B ends af)",
			lastByte(upstream.stdout), lastByte(port.stdout))
	}
	if len(upstream.stderr) != len(port.stderr) {
		t.Errorf("stderr byte count: upstream %d, port %d", len(upstream.stderr), len(port.stderr))
	}
	if lastByte(upstream.stderr) != lastByte(port.stderr) {
		t.Errorf("stderr last byte: upstream %s, port %s", lastByte(upstream.stderr), lastByte(port.stderr))
	}
	if upstream.stream() != port.stream() {
		t.Errorf("stream: upstream %s, port %s", upstream.stream(), port.stream())
	}
	if upstream.exit != port.exit {
		t.Errorf("exit code: upstream %d, port %d", upstream.exit, port.exit)
	}
	if u, p := strings.Join(upstream.tree, "\n"), strings.Join(port.tree, "\n"); u != p {
		t.Errorf("created file tree:\nupstream:\n%s\nport:\n%s", u, p)
	}
}

// osErrorPrefix is `run_rendercv`'s third clause, `f"OS Error: {e}"`
// (`cli/render_command/run_rendercv.py:194-196`). Both sides must announce the
// failure with it; only what follows is P-3's.
const osErrorPrefix = "OS Error: "

// absolutePathToken matches one absolute path in an unwrapped panel body.
//
// It stops at whitespace and at either quote, which is what lets the same
// pattern count upstream's quoted `'/abs/path'` and the port's bare one as a
// single token each. A path split across two wrapped lines is already rejoined
// by unwrapPanel, so it counts once and not twice.
var absolutePathToken = regexp.MustCompile(`/[^\s'"]+`)

// osErrorRow is row 6 of behavior 31's table, whose message body P-3 covers.
//
// The narrowing is to the **wording** and nothing else. The frame is compared
// exactly, both sides must carry osErrorPrefix, and each side must interpolate
// the absolute path of the file *this test* told it to write — asserted
// symmetrically, because the two scratch directories are equal-length on
// purpose and that trick would otherwise hide one side silently dropping its
// path. What is left unasserted is the errno phrasing between the prefix and
// the path, which is exactly what P-3 proposes to accept:
//
//	upstream: [Errno 13] Permission denied: '<path>'
//	port:     open <path>: permission denied
func osErrorRow(t *testing.T, upstream, port outcome) {
	t.Helper()

	compareFrames(t, upstream.stdout, port.stdout)

	var bodies [2]string
	for i, side := range []struct {
		name string
		out  outcome
	}{{"upstream", upstream}, {"port", port}} {
		body := unwrapPanel(side.out.stdout)
		if !strings.HasPrefix(body, osErrorPrefix) {
			t.Errorf("%s panel body does not start with %q: %q", side.name, osErrorPrefix, body)
		}
		// The path the test itself chose, not one scraped from the other
		// side's output: scraping would prove only that the port echoes
		// whatever upstream printed in a shape the scraper recognises.
		want := filepath.Join(side.out.dir, "out", "John_Doe_CV.typ")
		if !strings.Contains(body, want) {
			t.Errorf("%s panel body does not name the file it failed to write.\nwant path: %s\nbody: %s",
				side.name, want, body)
		}
		// Exactly one, not merely at least one. The panel pads each body line
		// to a fixed width, so the byte count is a function of the line count
		// and constrains nothing the frame does not already — measured, this
		// row has roughly ±50 characters of slack at 722 bytes. That slack is
		// wide enough for the port to append a *second* absolute path, a home
		// directory or another user's file, and still match on every other
		// dimension. Counting them is the property the row actually wants, and
		// unlike a length tolerance it needs no magic number.
		if found := absolutePathToken.FindAllString(body, -1); len(found) != 1 {
			t.Errorf("%s panel body names %d absolute paths, want exactly 1: %q\nbody: %s",
				side.name, len(found), found, body)
		}
		bodies[i] = strings.ReplaceAll(strings.TrimPrefix(body, osErrorPrefix), want, "<path>")
	}

	// Only claim P-3 when everything P-3 does *not* cover actually held.
	// Reported unconditionally, this line described a row failing on precisely
	// the things it said had matched, which is worse than silence.
	if t.Failed() {
		return
	}
	if bodies[0] != bodies[1] {
		// Three confirmations, not four: the byte count is not independent
		// evidence. Every body line is padded to a fixed width, so the total
		// is a function of the line count that the frame already pins.
		reportKnownOpen(t, "P-3", fmt.Sprintf(
			"the OS Error wording differs; the prefix, the single absolute path and the frame "+
				"all match (the byte count follows from the frame).\nupstream: %s\nport:     %s",
			bodies[0], bodies[1]))
	}
}

// compareFrames asserts the two panels have the same geometry: the same number
// of lines, and the same box-drawing skeleton on each. It is what keeps the
// wording narrowing honest — a same-length swap that alters the border, the
// title or the line count is not wording and goes red here.
func compareFrames(t *testing.T, upstream, port []byte) {
	t.Helper()

	u, p := frame(upstream), frame(port)
	if len(u) != len(p) {
		t.Errorf("panel line count: upstream %d, port %d", len(u), len(p))
		return
	}
	for i := range u {
		if u[i] != p[i] {
			t.Errorf("panel frame differs on line %d:\nupstream: %q\nport:     %q", i+1, u[i], p[i])
		}
	}
}

// frame reduces a panel to its skeleton.
//
// A border line is kept verbatim, which pins the title and the box width. A
// body line keeps its two `│` and collapses everything between them to `x` of
// the same display width, which pins how many lines the text wrapped into and
// how wide the box is without pinning a single character of what it says —
// where the words fall inside the body *is* the wording, and P-3 covers it.
func frame(stdout []byte) []string {
	lines := strings.Split(strings.TrimSuffix(string(stdout), "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		inner, ok := strings.CutPrefix(line, "│")
		if !ok {
			out = append(out, line)
			continue
		}
		inner, ok = strings.CutSuffix(inner, "│")
		if !ok {
			out = append(out, line)
			continue
		}
		out = append(out, "│"+strings.Repeat("x", utf8.RuneCountInString(inner))+"│")
	}
	return out
}

// unwrapPanel joins a panel's body lines back into the string rich wrapped.
//
// Rich breaks a token too long for the box mid-token and adds nothing, so the
// pieces concatenate directly — which is the only way a `contains` check can
// see an absolute path that spans two lines. A break at a real space loses that
// space, so this is usable for locating a path or a prefix and not for
// reconstructing prose.
func unwrapPanel(stdout []byte) string {
	var b strings.Builder
	for _, line := range strings.Split(string(stdout), "\n") {
		inner, ok := strings.CutPrefix(line, "│")
		if !ok {
			continue
		}
		inner, ok = strings.CutSuffix(inner, "│")
		if !ok {
			continue
		}
		b.WriteString(strings.TrimSpace(inner))
	}
	return b.String()
}

// findingsPath is where reportKnownOpen leaves its record, outside the
// repository (spec 013 §7.6). TestMain truncates it, so it always describes the
// run that just happened.
//
// **That freshness depends on `test-parity: build`** (`justfile:71`), and the
// dependency is not obvious. TestMain only truncates when the test binary
// actually runs, and `go test` serves a cached result instead whenever nothing
// the package reads has changed — leaving a fossil from an earlier run for the
// recipe to print as if it were current. What saves the gating path is that
// `build` rewrites `bin/rendercv-go` on every invocation and `binaries` stats
// it, so the cache always misses. Drop the `build` dependency and the findings
// go stale silently. A bare cached `go test` has this hazard today.
var findingsPath = filepath.Join(os.TempDir(), "rendercv-clidiff-findings.log")

// TestMain empties the findings file so a stale line from an earlier run cannot
// be read as this one's.
func TestMain(m *testing.M) {
	if err := os.WriteFile(findingsPath, nil, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "clidiff: cannot truncate %s: %v\n", findingsPath, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// reportKnownOpen records a divergence a human-gated proposal of spec 013 §10
// covers.
//
// It does not use t.Logf, which `go test` shows only under -v. It does not rely
// on stderr alone either: **`go test` discards a passing package's output
// entirely without -v**, so the line a downgrade is announced with is invisible
// in exactly the run that gates the iteration (`justfile:58`, no -v) — which is
// how the P-3 downgrade went unnoticed long enough to grow blind. So it writes
// both: stderr for a verbose or failing run, and findingsPath for the gating
// one, where nothing else survives.
func reportKnownOpen(t *testing.T, proposal, detail string) {
	t.Helper()

	line := fmt.Sprintf("KNOWN OPEN %s [%s]: %s\n", proposal, t.Name(), detail)
	fmt.Fprint(os.Stderr, line)

	file, err := os.OpenFile(findingsPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		t.Errorf("cannot record the %s finding in %s: %v", proposal, findingsPath, err)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("closing %s: %v", findingsPath, err)
		}
	}()
	if _, err := file.WriteString(line); err != nil {
		t.Errorf("writing the %s finding to %s: %v", proposal, findingsPath, err)
	}
}

// compareRaw asserts full byte equality of a stream. It is only usable for the
// invocations whose output carries neither a wall-clock duration nor a path
// below the scratch directory; the case table says which.
func compareRaw(t *testing.T, name string, upstream, port []byte) {
	t.Helper()

	if string(upstream) == string(port) {
		return
	}
	t.Errorf("%s differs byte for byte:\nupstream:\n%s\nport:\n%s", name, upstream, port)
}

func tree(t *testing.T, dir string) []string {
	t.Helper()

	var out []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// A directory the case made unreadable on purpose is not a
			// failure of the walk; its contents simply do not exist for us.
			if os.IsPermission(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
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

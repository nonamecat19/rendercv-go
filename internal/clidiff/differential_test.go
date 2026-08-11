//go:build conformance

package clidiff

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
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
// a pass in the run that gates the iteration: `just test-parity` (`justfile:58`)
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
// openProposal names a human-gated divergence proposal of spec 013 §10 when one
// covers the *content* of this row's message. It downgrades the stdout size
// comparison to a logged finding and nothing else: the last byte, the stream,
// the exit code and the file tree are still the contract, because none of them
// is what the proposal is about. An empty string asserts everything.
func compare(t *testing.T, upstream, port outcome, openProposal string) {
	t.Helper()

	if len(upstream.stdout) != len(port.stdout) {
		if openProposal != "" {
			t.Logf("KNOWN OPEN %s: stdout byte count upstream %d, port %d\nupstream:\n%s\nport:\n%s",
				openProposal, len(upstream.stdout), len(port.stdout), upstream.stdout, port.stdout)
		} else {
			t.Errorf("stdout byte count: upstream %d, port %d", len(upstream.stdout), len(port.stdout))
		}
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

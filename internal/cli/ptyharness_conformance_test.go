//go:build conformance && linux

package cli_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/nonamecat19/rendercv-go/internal/conformance"
)

// The PTY differential — spec 012 delta §6 (`spec-delta-colour.md`).
//
// **Why a PTY at all.** Rich disables colour when `isatty()` is false
// (`rich/console.py:937-984`), and every golden is captured through a pipe
// (`tools/gengolden/main.go:214-218`) with a corpus that additionally pins
// `NO_COLOR=1` and `TERM=dumb`. So the goldens do not merely miss the terminal
// surface, they turn it off, and nothing in the suite has ever been able to see
// it. This file is the only thing that can.
//
// **It is deliberately not a golden.** The repaint stream is non-deterministic
// (delta §5: two identical upstream renders gave 2963 B and 2178 B, because the
// repaint count depends on where step boundaries fall in a 250 ms window), so
// captures are compared live, side against side, and never committed.
//
// Linux only: it allocates the pty through `/dev/ptmx`.

const (
	upstreamDir     = "../../third_party/rendercv"
	upstreamConsole = upstreamDir + "/.venv/bin/rendercv"

	// consoleColumns is the width both sides are driven at. It is the goldens'
	// own width (`testdata/corpus.json`), which is what lets this file reuse
	// `conformance.RebindBinaryName` — that helper re-pads a full-width line
	// against the same constant.
	consoleColumns = "80"
)

// upstreamInterpreter is the vendored submodule's own console script.
//
// **The console script, not `python -m rendercv`.** Click takes the program
// name from `argv[0]`, so `-m` makes every usage line read
// `Usage: python -m rendercv …` and turns `--help` into a difference that is
// the harness's own doing.
//
// `RENDERCV_UPSTREAM_BIN` overrides the path, which is what makes this runnable
// from a git worktree: a worktree checks out the submodule's *files* but not
// its `.venv`, so the script is absent there and the tests below skip rather
// than fail. The override points them at a full checkout's venv.
func upstreamInterpreter(t *testing.T) string {
	t.Helper()
	if override := os.Getenv("RENDERCV_UPSTREAM_BIN"); override != "" {
		return override
	}
	path, err := filepath.Abs(upstreamConsole)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// requireUpstream skips when the vendored interpreter is not installed.
//
// **The skip is narrow on purpose.** It fires only when the interpreter file is
// missing; once it exists, every failure below is reported. A test that cannot
// run upstream cannot report anything about parity, and saying so is honest,
// but a skip that swallowed a real mismatch would not be.
func requireUpstream(t *testing.T) string {
	t.Helper()
	console := upstreamInterpreter(t)
	if _, err := os.Stat(console); err != nil {
		t.Skipf("vendored rendercv absent (%s); set RENDERCV_UPSTREAM_BIN to run", console)
	}
	return console
}

// portBinary builds the port once per test binary and returns its path.
//
// **It is built outside the repository tree.** A capture whose path contains
// `rendercv-go` is corrupted by the D-009 binary-name rebinding, which is one
// of the two harness bugs that invalidated four measurement runs on this
// surface (delta §6.3).
var (
	buildOnce sync.Once
	buildPath string
	buildErr  error
)

func portBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "rcvport")
		if err != nil {
			buildErr = err
			return
		}
		buildPath = filepath.Join(dir, "port")
		cmd := exec.Command("go", "build", "-o", buildPath, "../../cmd/rendercv-go")
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("build the port: %w\n%s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return buildPath
}

// runSide runs one side of a differential: a fresh directory holding `input` as
// `CV.yaml`, then the command under a pty.
func runSide(t *testing.T, dir string, argv []string, env map[string]string, input string) string {
	t.Helper()
	prepare(t, dir, input)
	return runPTY(t, dir, argv, env)
}

// prepare re-creates the working directory and writes the input document.
func prepare(t *testing.T, dir, input string) {
	t.Helper()
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if input != "" {
		if err := os.WriteFile(filepath.Join(dir, "CV.yaml"), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// runPTY runs a command under a fresh pty and returns everything it wrote.
//
// **The directory is re-created per side, not per case.** `new` and
// `create-theme` write files, so a shared directory makes the second side
// report `Your YAML input file already exists` — a phantom diff that cost this
// investigation a run (delta §6.2.1). Both sides still see the *same* absolute
// path, which is what keeps a path quoted in a message comparable.
func runPTY(t *testing.T, dir string, argv []string, env map[string]string) string {
	t.Helper()

	primary, secondary, err := openPTY()
	if err != nil {
		t.Fatalf("allocate a pty: %v", err)
	}
	defer func() { _ = primary.Close() }()

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = secondary, secondary, secondary
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	cmd.Env = childEnv(env)

	if err := cmd.Start(); err != nil {
		_ = secondary.Close()
		t.Fatalf("start %s: %v", argv[0], err)
	}
	// The child owns the secondary end now. Closing it here is what makes the
	// read below end at EIO when the child exits, instead of blocking forever.
	_ = secondary.Close()

	var out bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		// A pty read returns EIO rather than EOF when the last writer goes
		// away, which is normal termination and not an error to report.
		_, _ = io.Copy(&out, primary)
	}()
	_ = cmd.Wait()
	<-done

	return out.String()
}

// childEnv is a fixed, minimal base plus the case's own variables, so a
// variable inherited from the developer's shell — `COLORTERM` above all —
// cannot change what is measured.
func childEnv(env map[string]string) []string {
	base := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"LC_ALL=C.UTF-8",
		"LANG=C.UTF-8",
		"COLUMNS=" + consoleColumns,
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	}
	for name, value := range env {
		base = append(base, name+"="+value)
	}
	return base
}

// openPTY allocates a pty pair through `/dev/ptmx`.
func openPTY() (primary, secondary *os.File, err error) {
	primary, err = os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			_ = primary.Close()
		}
	}()

	if err = unix.IoctlSetPointerInt(int(primary.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		return nil, nil, fmt.Errorf("unlock the pty: %w", err)
	}
	number, err := unix.IoctlGetInt(int(primary.Fd()), unix.TIOCGPTN)
	if err != nil {
		return nil, nil, fmt.Errorf("name the pty: %w", err)
	}
	// A window size, because Rich falls back to `os.get_terminal_size` whenever
	// COLUMNS is unset — and under `TERM=dumb` it ignores COLUMNS entirely
	// (delta §3.4), which is a case this harness has to be able to drive.
	if err = unix.IoctlSetWinsize(int(primary.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 40, Col: 80}); err != nil {
		return nil, nil, fmt.Errorf("size the pty: %w", err)
	}

	secondary, err = os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	return primary, secondary, nil
}

// upstreamArgv runs the vendored rendercv through the submodule's interpreter.
func upstreamArgv(t *testing.T, args []string) []string {
	t.Helper()
	return append([]string{requireUpstream(t)}, args...)
}

func portArgv(t *testing.T, args []string) []string {
	t.Helper()
	return append([]string{portBinary(t)}, args...)
}

// escapePattern matches every escape sequence the two sides emit: CSI, OSC
// terminated by ST or BEL, and the two-character forms.
var escapePattern = regexp.MustCompile("\x1b\\][^\x1b\a]*(?:\x1b\\\\|\a)|\x1b\\[[0-9;?]*[ -/]*[@-~]|\x1b[@-Z\\\\-_]")

// plain is a capture with every escape sequence removed — the text a reader
// sees, which is what the two sides must agree on before any style question is
// meaningful.
func plain(capture string) string {
	return escapePattern.ReplaceAllString(capture, "")
}

// sequences is every escape sequence in a capture, in order.
func sequences(capture string) []string {
	return escapePattern.FindAllString(capture, -1)
}

// inventory counts each distinct escape sequence.
//
// **This is the comparable form**, not the raw bytes: the repaint count varies
// between two runs of the *same* binary (delta §5), so a byte comparison of any
// progress surface flakes by construction. The deterministic surfaces are
// compared as bytes; everything else is compared as an inventory.
func inventory(capture string) map[string]int {
	counts := map[string]int{}
	for _, sequence := range sequences(capture) {
		counts[readable(sequence)]++
	}
	return counts
}

// `readable` — spelling the escape byte out so a failure is legible — lives in
// `style_test.go`, which is untagged and so is always compiled into this test
// binary.

// eraseBlock is the sequence `Live` writes to wipe the frame it is about to
// replace: a carriage return, then one `ESC[2K` per line with `ESC[1A` between
// them (`rich/live.py`, via `Control.move_to_column` and friends).
var eraseBlock = regexp.MustCompile("(?:\r?(?:\x1b\\[2K|\x1b\\[1A))+")

// finalFrame is everything written after the last erase — the frame a reader
// actually ends up looking at.
//
// **This is what makes a Live surface comparable at all.** Upstream repaints
// the growing panel once per step and the port paints once, so the raw captures
// differ in *content*, not merely in control sequences, and the difference is
// the §5 decision rather than a geometry defect. Collapsing to the final frame
// asks the question that survives that decision: does the reader see the same
// thing when it is over?
func finalFrame(capture string) string {
	matches := eraseBlock.FindAllStringIndex(capture, -1)
	if len(matches) == 0 {
		return capture
	}
	return capture[matches[len(matches)-1][1]:]
}

// frameText is the final frame's plain text with its trailing line ending
// removed.
//
// The trim is for the pty's own `\r`, and for Live's epilogue: stopping a Live
// display writes one more newline than a plain print does, so upstream's last
// line carries a `\r` the port's does not. That is the §5 protocol again, not a
// difference in what is drawn. Only `\r` and `\n` are trimmed — trailing
// *spaces* stay, because a panel's right-hand padding is part of the geometry.
func frameText(capture string) string {
	// **The `\r` is the pty's, not the program's.** A terminal line discipline
	// translates every `\n` on output to `\r\n` (ONLCR), so both sides get it
	// and neither wrote it. Leaving it in makes every line one column wider than
	// it is, which defeats `RebindBinaryName`'s own width check and turns a
	// re-padded row into a three-column difference.
	text := strings.ReplaceAll(plain(finalFrame(capture)), "\r\n", "\n")
	return strings.TrimRight(text, "\r\n")
}

// portText is frameText plus D-009's binary-name rebinding, applied through the
// **established** helper rather than by substituting the captured bytes.
//
// A bare `s/rendercv-go/rendercv/` matches the text and leaves the padding
// three columns too wide, which is the phantom this harness exists to avoid
// (delta §6.3). `RebindBinaryName` re-pads the rows it shortens, and records
// the cost: on those rows the right-hand padding is reconstructed rather than
// compared.
func portText(capture string) string {
	return conformance.RebindBinaryName(frameText(capture))
}

// osc8ID normalizes the random link id Rich puts in a hyperlink —
// `ESC]8;id=919290;URL` — which differs between two runs of the same binary
// (delta §2.5) and is the one styled element whose exact bytes are not
// reproducible at all.
var osc8ID = regexp.MustCompile(`id=\d+;`)

func normalize(capture string) string {
	return osc8ID.ReplaceAllString(capture, "id=<n>;")
}

// diffLines reports the first difference between two captures, with the escape
// byte spelled out so a failure is readable in test output.
func diffLines(t *testing.T, got, want string) {
	t.Helper()
	gotLines := strings.Split(readable(got), "\n")
	wantLines := strings.Split(readable(want), "\n")
	for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			t.Errorf("line %d differs:\n  port     %q\n  upstream %q", i+1, g, w)
			return
		}
	}
}

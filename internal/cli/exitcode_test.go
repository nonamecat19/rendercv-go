package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestSuccessPathExitCodes pins what `execute` returns when a command body
// returns.
//
// **A fresh-context audit measured `render cv.yaml -nopdf -nopng` and
// `render cv.yaml -typ out.typ` exiting 70 while writing every artifact and
// printing the success panel.** 70 is the initial value of `execute`'s `code`,
// so any path that reached cobra's error return — or failed to run the body at
// all — reported an internal failure on a run that had succeeded. That is worse
// than a wrong byte: a caller cannot tell a finished render from a broken one.
//
// The defect is gone, and nothing held it. These cases hold it. They stub the
// body deliberately: the value under test is the plumbing between the parser
// and the exit code, not what `Render` does.
func TestSuccessPathExitCodes(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "plain render", args: []string{"render", "cv.yaml"}},
		{name: "quiet", args: []string{"render", "cv.yaml", "--quiet"}},
		{name: "quiet short", args: []string{"render", "cv.yaml", "-q"}},
		{name: "two negatives", args: []string{"render", "cv.yaml", "-nopdf", "-nopng"}},
		{
			name: "every negative",
			args: []string{"render", "cv.yaml", "-notyp", "-nopdf", "-nopng", "-nomd", "-nohtml"},
		},
		{name: "one path flag", args: []string{"render", "cv.yaml", "-typ", "out.typ"}},
		{
			name: "path flags and negatives",
			args: []string{"render", "cv.yaml", "-typ", "out.typ", "-nopdf", "-nopng"},
		},
		{name: "an override", args: []string{"render", "cv.yaml", "--cv.phone", "123"}},
		{name: "an overlay", args: []string{"render", "cv.yaml", "-d", "design.yaml"}},
		{name: "new", args: []string{"new", "John Doe"}},
		{name: "new with a theme", args: []string{"new", "John Doe", "--theme", "moderncv"}},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := execute(row.args, &stdout, &stderr, runners{
				render: func(RenderOptions, io.Writer, io.Writer) int { return 0 },
				newCV:  func(NewOptions, io.Writer, io.Writer) int { return 0 },
			})
			if code != 0 {
				t.Errorf("exit code = %d, want 0; stderr = %q", code, stderr.String())
			}
		})
	}
}

// TestExitCodeComesFromTheBody is the other half: a body that fails must have
// its code returned rather than replaced. Without this, the case above would
// pass on an `execute` that returned 0 unconditionally.
func TestExitCodeComesFromTheBody(t *testing.T) {
	for _, want := range []int{0, 1, 2} {
		var stdout, stderr bytes.Buffer
		code := execute([]string{"render", "cv.yaml"}, &stdout, &stderr, runners{
			render: func(RenderOptions, io.Writer, io.Writer) int { return want },
			newCV:  func(NewOptions, io.Writer, io.Writer) int { return 0 },
		})
		if code != want {
			t.Errorf("exit code = %d, want %d", code, want)
		}
	}
}

// TestNoArgumentsPrintsTheRootHelp is spec 012 help.md acceptance criterion 3,
// and it exists because a fresh-context verifier found the criterion met by the
// code and gated by nothing.
//
// **`HelpPage("")` being right is not the same as the CLI reaching it.** The
// help tests call the renderer directly; this one goes through `execute`, which
// is where the bare invocation used to exit 70 in silence. It is the fifth time
// this port has shipped a correct function no input could reach, so the edge
// gets its own case rather than an inference.
func TestNoArgumentsPrintsTheRootHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := execute(nil, &stdout, &stderr, runners{
		render: func(RenderOptions, io.Writer, io.Writer) int {
			t.Error("render ran on a bare invocation")
			return 70
		},
		newCV: func(NewOptions, io.Writer, io.Writer) int {
			t.Error("new ran on a bare invocation")
			return 70
		},
	})

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty — the help goes to stdout", stderr.String())
	}
	if got, want := stdout.String(), HelpPage(""); got != want {
		t.Errorf("stdout is not the root help page (%d bytes vs %d)", len(got), len(want))
	}
}

// TestHelpFlagsReachTheRenderer pins the other half: `--help` and `-h`, on the
// root and on a subcommand, print that command's page and exit 0 without
// running the command.
func TestHelpFlagsReachTheRenderer(t *testing.T) {
	cases := []struct {
		args []string
		page string
	}{
		{args: []string{"--help"}, page: ""},
		{args: []string{"-h"}, page: ""},
		{args: []string{"render", "--help"}, page: "render"},
		{args: []string{"render", "-h"}, page: "render"},
		{args: []string{"new", "--help"}, page: "new"},
		{args: []string{"new", "-h"}, page: "new"},
	}

	for _, row := range cases {
		t.Run(strings.Join(row.args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := execute(row.args, &stdout, &stderr, runners{
				render: func(RenderOptions, io.Writer, io.Writer) int {
					t.Error("render ran on a help request")
					return 70
				},
				newCV: func(NewOptions, io.Writer, io.Writer) int {
					t.Error("new ran on a help request")
					return 70
				},
			})
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
			if got, want := stdout.String(), HelpPage(row.page); got != want {
				t.Errorf("stdout is not the %q page (%d bytes vs %d)", row.page, len(got), len(want))
			}
		})
	}
}

// TestVersionExitsZero pins `cli_version`, the first parity case the port ever
// passed, at the level of the exit code its golden records.
func TestVersionExitsZero(t *testing.T) {
	for _, flag := range []string{"--version", "-v"} {
		var stdout, stderr bytes.Buffer
		code := execute([]string{flag}, &stdout, &stderr, runners{
			render: func(RenderOptions, io.Writer, io.Writer) int { return 70 },
			newCV:  func(NewOptions, io.Writer, io.Writer) int { return 70 },
		})
		if code != 0 {
			t.Errorf("%s: exit code = %d, want 0", flag, code)
		}
		if stdout.String() != "RenderCV v"+Version+"\n" {
			t.Errorf("%s: stdout = %q", flag, stdout.String())
		}
	}
}

package cli

import (
	"bytes"
	"io"
	"testing"
)

// TestUsageErrors is spec 012 §2 behavior 11c.
//
// **These four surfaces produced exit 70 and not one byte of output.** 70 is
// `Execute`'s initial value, returned whenever cobra reports an error, so an
// unknown command and a missing argument were indistinguishable from an
// internal failure — and a caller could not tell either from a successful run
// except by the code.
//
// Upstream writes three parts to **stderr** and exits **2**: click's usage
// line, its `Try ... for help.` line, and the same `Error` panel every
// `RenderCVUserError` uses. The stream, the panel and the code were each
// measured against the vendored CLI at `COLUMNS=80`.
//
// The binary name is the sanctioned divergence of `AGENTS.md` §1, so every
// `rendercv` upstream prints is `rendercv-go` here.
func TestUsageErrors(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		stderr string
	}{
		{
			name: "unknown command",
			args: []string{"bogus"},
			stderr: "Usage: rendercv-go [OPTIONS] COMMAND [ARGS]...\n" +
				"Try 'rendercv-go -h' for help.\n" +
				Panel("Error", []PanelRow{{Text: "No such command 'bogus'."}}),
		},
		{
			name: "render without an input file",
			args: []string{"render"},
			stderr: "Usage: rendercv-go render [OPTIONS] INPUT_FILE_NAME\n" +
				"Try 'rendercv-go render -h' for help.\n" +
				Panel("Error", []PanelRow{{Text: "Missing argument 'INPUT_FILE_NAME'."}}),
		},
		{
			name: "new without a name",
			args: []string{"new"},
			stderr: "Usage: rendercv-go new [OPTIONS] FULL_NAME\n" +
				"Try 'rendercv-go new -h' for help.\n" +
				Panel("Error", []PanelRow{{Text: "Missing argument 'FULL_NAME'."}}),
		},
		{
			// **G-3, and it prints differently from every other usage error**:
			// the panel alone, with no usage line and no `Try …` line. Measured
			// against the vendored CLI, which quotes the spelling the user
			// typed.
			name: "an option missing its value",
			args: []string{"render", "cv.yaml", "--output-folder"},
			stderr: Panel("Error", []PanelRow{
				{Text: "Option '--output-folder' requires an argument."},
			}),
		},
		{
			name: "a short option missing its value",
			args: []string{"render", "cv.yaml", "-o"},
			stderr: Panel("Error", []PanelRow{
				{Text: "Option '-o' requires an argument."},
			}),
		},
		{
			// **G-4.** `render` never reaches this — its unknowns go to the
			// extras — so the case has to be driven through another command.
			name: "an unknown option on new",
			args: []string{"new", "Jane", "-d", "foo"},
			stderr: "Usage: rendercv-go new [OPTIONS] FULL_NAME\n" +
				"Try 'rendercv-go new -h' for help.\n" +
				Panel("Error", []PanelRow{{Text: "No such option: -d"}}),
		},
		{
			name: "an unknown long option on new",
			args: []string{"new", "Jane", "--nope"},
			stderr: "Usage: rendercv-go new [OPTIONS] FULL_NAME\n" +
				"Try 'rendercv-go new -h' for help.\n" +
				Panel("Error", []PanelRow{{Text: "No such option: --nope"}}),
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := execute(row.args, &stdout, &stderr, runners{
				render: func(RenderOptions, io.Writer, io.Writer) int {
					t.Error("render ran on a usage error")
					return 0
				},
				newCV: func(NewOptions, io.Writer, io.Writer) int {
					t.Error("new ran on a usage error")
					return 0
				},
			})

			if code != exitUsageError {
				t.Errorf("exit code = %d, want %d", code, exitUsageError)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty — the usage error is stderr's", stdout.String())
			}
			if stderr.String() != row.stderr {
				t.Errorf("stderr =\n%s\nwant\n%s", stderr.String(), row.stderr)
			}
		})
	}
}

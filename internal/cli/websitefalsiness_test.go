package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/cli"
)

// TestWebsiteShapesMatchUpstreamExitCodes is the whole falsy-`website` measurement,
// one row per value, taken from the vendored binary with `NO_COLOR=1 TERM=dumb
// COLUMNS=80` on `cv:\n  name: A\n  website: <value>\n`:
//
//	value    upstream                                                       port
//	[]       exit 1, RenderCVInternalError on stderr (traceback)            exit 1
//	{}       exit 1, `URL input should be a string or URL.`                 same
//	null     exit 0, renders                                                same
//	""       exit 1, `This is not a valid URL.`                             same
//	0        exit 1, `URL input should be a string or URL.`                 same
//	false    exit 1, `URL input should be a string or URL.`                 same
//	a URL    exit 0, renders                                                same
//
// **Only the first row was wrong**, and it was wrong on the exit code — axis 2
// — not on the message: the port rendered a complete CV at exit 0 where
// upstream refuses. The traceback itself is out of reach (D-011's class), so
// the row is a validation record here; the exit code is not, and the other six
// rows are in the table to keep the fix from over-reaching into them.
func TestWebsiteShapesMatchUpstreamExitCodes(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		value    string
		wantExit int
		// wantMessage is a substring of the flattened panel. It is empty for a
		// run that must succeed.
		wantMessage string
	}{
		{
			name:  "an empty sequence",
			value: "[]", wantExit: 1,
			wantMessage: "website key present but value is None",
		},
		{
			name:  "an empty mapping",
			value: "{}", wantExit: 1,
			wantMessage: "URL input should be a string or URL.",
		},
		{name: "an explicit null", value: "null", wantExit: 0},
		{
			name:  "an empty string",
			value: `""`, wantExit: 1,
			wantMessage: "This is not a valid URL.",
		},
		{
			name:  "zero",
			value: "0", wantExit: 1,
			wantMessage: "URL input should be a string or URL.",
		},
		{
			name:  "false",
			value: "false", wantExit: 1,
			wantMessage: "URL input should be a string or URL.",
		},
		{name: "a URL", value: `"https://example.com"`, wantExit: 0},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "cv.yaml")
			document := "cv:\n  name: A\n  website: " + testCase.value + "\n"
			if err := os.WriteFile(input, []byte(document), 0o644); err != nil {
				t.Fatal(err)
			}

			var stdout, stderr bytes.Buffer
			// **PDF and PNG off**: the typst compile costs seconds and this
			// case never reaches it — the failure is in the `.typ` generation.
			code := cli.Render(cli.RenderOptions{
				InputPath: input, OutputFolder: filepath.Join(dir, "out"),
				NoPDF: true, NoPNG: true,
			}, &stdout, &stderr)

			if code != testCase.wantExit {
				t.Errorf("exit code = %d, want %d\nstdout:\n%s", code, testCase.wantExit, stdout.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want nothing written", stderr.String())
			}
			if testCase.wantMessage == "" {
				return
			}

			flat := flatten(stdout.String())
			if !strings.Contains(flat, testCase.wantMessage) {
				t.Errorf("stdout = %q, want it to carry %q", flat, testCase.wantMessage)
			}
			// A refusal is a validation table, not the one-message `Error` box
			// — the shape D-012 promises for this class.
			if !strings.Contains(stdout.String(), "There are validation errors!") {
				t.Errorf("stdout = %q, want the validation panel", stdout.String())
			}
			if !strings.Contains(flat, "cv.website") {
				t.Errorf("stdout = %q, want the location column to read cv.website", flat)
			}
		})
	}
}

// A falsy `website` must not leave half a render behind: upstream raises before
// it writes the `.typ`, and the port's record has to arrive at the same point.
func TestAFalsyWebsiteWritesNoArtifact(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(input, []byte("cv:\n  name: A\n  website: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "out")

	var stdout, stderr bytes.Buffer
	if code := cli.Render(cli.RenderOptions{
		InputPath: input, OutputFolder: output,
		NoPDF: true, NoPNG: true,
	}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}

	entries, err := os.ReadDir(output)
	if err == nil && len(entries) != 0 {
		t.Errorf("output folder holds %d entries, want none", len(entries))
	}
}

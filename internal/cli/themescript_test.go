package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/cli"
)

// renderWithScript writes a CV, a custom theme folder with one template, and
// the given `init.lua`, then renders — the whole path a broken theme script
// travels, from the file on disk to the panel a user reads.
func renderWithScript(t *testing.T, script string) (code int, stdout, stderr string) {
	t.Helper()
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(input,
		[]byte("cv:\n  name: John Doe\ndesign:\n  theme: mytheme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "mytheme"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mytheme", "Preamble.j2.typ"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mytheme", "init.lua"),
		[]byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code = cli.Render(cli.RenderOptions{
		InputPath: input, OutputFolder: filepath.Join(dir, "out"),
		NoPDF: true, NoPNG: true,
	}, &out, &errOut)
	return code, out.String(), errOut.String()
}

// panelText strips the table's borders and collapses its wrapping, so a test
// can assert on the sentence a user reads rather than on where the box happened
// to break it. `flatten` is not enough here: it keeps the interior column
// separators, and a message wrapped across two rows comes back with a `│ │` in
// the middle of it.
func panelText(stdout string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(stdout, "│", " ")), " ")
}

// **The period on a script failure's message comes from the pipeline, exactly
// once.** The record used to be synthesized at render time, which skipped
// `errorpipeline.Parse` — so `scriptRecordOf` appended the period by hand to
// stand in for step 8 (`appendPeriod`). Now the record is produced by
// `design.Validate` and flows through `Parse` like every other one, and the
// by-hand append is gone; leaving both in place would print `…executive'..`
//
// Mode 4's stdout is byte-identical to upstream's for this vector, so a second
// period would break the only differential parity this behavior has. Nothing
// else in the tree tests for it, which is why this test exists.
func TestABrokenScriptMessageEndsInOnePeriod(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{{
		// Upstream's own sentence, unprefixed — the one mode that is exact
		// parity rather than D-013's substitute wording.
		name:   "a value the field rejects",
		script: `return { page = { size = "bogus" } }`,
		want:   "Input should be 'a4', 'a5', 'us-letter' or 'us-executive'.",
	}, {
		name:   "a parse error",
		script: "return {",
		want:   "syntax error.",
	}, {
		name:   "a non-table return",
		script: "return 42",
		want:   "did not return a table of theme options.",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, stdout, stderr := renderWithScript(t, test.script)

			if code != 1 {
				t.Errorf("exit = %d, want 1 — a broken script refuses to render", code)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want nothing written", stderr)
			}
			text := panelText(stdout)
			if !strings.Contains(text, test.want) {
				t.Errorf("stdout = %q, want it to contain %q", text, test.want)
			}
			// The period is the whole point: one, from the pipeline. Checking
			// for `..` anywhere would trip on the Input Value column, which is
			// the elided `...` for three of these modes.
			if strings.Contains(text, test.want+".") {
				t.Errorf("stdout = %q, has a doubled period after %q", text, test.want)
			}
		})
	}
}

// **A colour a script declares badly gets the error dictionary's text.** The
// synthesized record bypassed `errorpipeline.Parse`, so it never met the
// dictionary and printed pydantic's raw sentence where upstream prints row
// 13's replacement. Measured against the vendored binary with the same mistake
// in an `__init__.py`:
//
//	upstream: This is not a valid color. Here are some examples of valid
//	          colors: "red", "#ff0000", "rgb(255, 0, 0)", "hsl(0, 100%, 50%)".
//	port:     value is not a valid color: string not recognised as a valid color.
//
// Routing the record through `Parse` is what fixes it, which is the same change
// that removes the by-hand period above — one bypass, two symptoms.
func TestABrokenScriptColorUsesTheDictionary(t *testing.T) {
	code, stdout, _ := renderWithScript(t, `return { colors = { name = "notacolor" } }`)

	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	text := panelText(stdout)
	if !strings.Contains(text, "This is not a valid color. Here are some examples") {
		t.Errorf("stdout = %q, want the dictionary's colour text", text)
	}
	if strings.Contains(text, "value is not a valid color") {
		t.Errorf("stdout = %q, still carries pydantic's raw sentence", text)
	}
}

// A theme folder with no `init.lua` still renders, which is upstream's
// no-module fallback (`design.py:139-142`) and the reason "absent" and "broken"
// must not share a path.
func TestAnAbsentScriptStillRenders(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(input,
		[]byte("cv:\n  name: John Doe\ndesign:\n  theme: mytheme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "mytheme"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mytheme", "Preamble.j2.typ"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := cli.Render(cli.RenderOptions{
		InputPath: input, OutputFolder: filepath.Join(dir, "out"),
		NoPDF: true, NoPNG: true,
	}, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d, want 0 — an absent script is not a failure: %s", code, stdout.String())
	}
}

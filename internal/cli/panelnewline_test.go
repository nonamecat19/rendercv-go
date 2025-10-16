package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPanelTrailingNewline pins the **last byte** of every panel `render`
// writes, which is the one byte the conformance suite cannot see:
// `conformance.Normalize` and `tools/gengolden`'s `normalize` both append a
// trailing newline to each side before comparing, so `err_empty_yaml` reported
// green while the port was one byte short of upstream.
//
// Upstream has two panel printers and picks between them by **where** the
// failure was raised, not by its type:
//
//   - Anything raised before `with ProgressPanel(...)`
//     (`render_command.py:231`) — that is, inside `collect_input_file_paths`
//     (`run_rendercv.py:105-124`) or `parse_override_arguments`
//     (`render_command.py:228`) — escapes to the `@handle_user_errors`
//     decorator, which calls `rich.print` (`cli/error_handler.py:40-47`).
//     `rich.print` **ends with a newline**.
//   - Anything raised inside `run_rendercv` reaches
//     `ProgressPanel.print_user_error` / `.print_validation_errors`
//     (`progress_panel.py:119-168`), which update a `rich.live.Live` whose
//     final render has **no trailing newline** when stdout is not a terminal.
//
// Both columns below were measured against the vendored Python with
// `COLUMNS=80`, comparing `wc -c` and `tail -c1 | xxd -p`.
func TestPanelTrailingNewline(t *testing.T) {
	const validCV = "cv:\n  name: John Doe\n"

	cases := []struct {
		name string
		// yaml is the input file's content.
		yaml string
		// extras are the trailing `--key value` tokens.
		extras []string
		// wantNewline is what upstream's last byte is: true for `\n`.
		wantNewline bool
	}{
		// Upstream: 553 bytes, last byte 0a. `collect_input_file_paths`
		// suppresses only `RenderCVUserValidationError`
		// (`run_rendercv.py:113`), so an *empty* file — a plain
		// `RenderCVUserError` — escapes to the decorator.
		{name: "empty input file", yaml: "", wantNewline: true},

		// Upstream: 638 bytes, last byte 0a. `parse_override_arguments` runs
		// at `render_command.py:228`, before the `ProgressPanel`.
		{
			name:        "odd override argument count",
			yaml:        validCV,
			extras:      []string{"--cv.name"},
			wantNewline: true,
		},

		// Upstream: 553 bytes, last byte 0a. The other failure mode of the
		// same pre-panel function.
		{
			name:        "override key without dashes",
			yaml:        validCV,
			extras:      []string{"cv.name", "Jane"},
			wantNewline: true,
		},

		// Upstream: last byte af (`╯`). A YAML *syntax* error is suppressed by
		// `collect_input_file_paths` and re-raised inside the Live phase.
		{name: "malformed yaml", yaml: "cv: [\n", wantNewline: false},

		// Upstream: last byte af. Validation runs inside `run_rendercv`.
		{
			name:        "validation error",
			yaml:        "cv:\n  name: John Doe\ndesign:\n  theme: nosuchtheme\n",
			wantNewline: false,
		},

		// Upstream: 552 bytes, last byte af. The override *application* is
		// inside `build_rendercv_dictionary_and_model`, unlike its parsing.
		{
			name:        "override index out of range",
			yaml:        "cv:\n  name: John Doe\n  sections:\n    experience: []\n",
			extras:      []string{"--cv.sections.experience.9.company", "X"},
			wantNewline: false,
		},

		// Upstream: last byte af. The success panel is the Live's own final
		// render.
		{name: "success panel", yaml: validCV, wantNewline: false},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "cv.yaml")
			if err := os.WriteFile(input, []byte(row.yaml), 0o600); err != nil {
				t.Fatal(err)
			}

			var stdout, stderr bytes.Buffer
			Render(RenderOptions{
				InputPath: input,
				Extras:    row.extras,
				// The artifacts are not what this measures, and switching
				// them off keeps the success case off the Typst compiler.
				NoTypst:    true,
				NoPDF:      true,
				NoPNG:      true,
				NoMarkdown: true,
				NoHTML:     true,
			}, &stdout, &stderr)

			got := stdout.String()
			if got == "" {
				t.Fatalf("no panel on stdout; stderr = %q", stderr.String())
			}
			if hasNewline := strings.HasSuffix(got, "\n"); hasNewline != row.wantNewline {
				t.Errorf("panel ends with a newline = %v, want %v (last bytes %q)",
					hasNewline, row.wantNewline, got[max(len(got)-8, 0):])
			}
		})
	}
}

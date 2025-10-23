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

// TestNewPanelTrailingNewline is the same rule for `new`, which the first
// version of the split missed. `new_command.py` builds **no** `ProgressPanel`
// at all and carries `@handle_user_errors` (`new_command.py:27`), so every
// `RenderCVUserError` it raises takes the `rich.print` path unconditionally —
// there is no Live branch for it to fall into.
//
// Measured against the vendored CLI at `COLUMNS=80`: 638 bytes for the theme
// case and 894 for the locale case, both ending 0a, both byte-identical to the
// port after this fix.
func TestNewPanelTrailingNewline(t *testing.T) {
	cases := []struct {
		name    string
		options NewOptions
	}{
		{name: "unknown theme", options: NewOptions{Name: "John Doe", Theme: "nosuch"}},
		{name: "unknown locale", options: NewOptions{Name: "John Doe", Locale: "nosuch"}},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			var stdout, stderr bytes.Buffer
			if code := New(row.options, &stdout, &stderr); code == 0 {
				t.Fatalf("exit = 0, want a failure; stdout = %q", stdout.String())
			}

			got := stdout.String()
			if got == "" {
				t.Fatalf("no panel on stdout; stderr = %q", stderr.String())
			}
			if !strings.HasSuffix(got, "\n") {
				t.Errorf("panel does not end with a newline (last bytes %q)",
					got[max(len(got)-8, 0):])
			}
		})
	}
}

// TestQuietSilencesTheLiveConsoleOnly pins which panels `--quiet` suppresses.
//
// Upstream builds the whole `ProgressPanel` on
// `rich.console.Console(quiet=quiet)` (`progress_panel.py:62`), so **every**
// panel that renders through the Live — the success box, `print_user_error`
// and `print_validation_errors` alike — emits nothing under `-q`. The port
// gated only the success box, so a validation failure printed its full
// 1599-byte table where upstream prints zero bytes.
//
// The `@handle_user_errors` panel is a plain `rich.print` on a different
// console and is **not** suppressed: an empty input file under `-q` is 553
// bytes on both sides. All four rows measured against the vendored CLI.
func TestQuietSilencesTheLiveConsoleOnly(t *testing.T) {
	const validCV = "cv:\n  name: John Doe\n"

	cases := []struct {
		name   string
		yaml   string
		silent bool
	}{
		{name: "validation error", yaml: validCV[:len(validCV)-1] + "\ndesign:\n  theme: nosuchtheme\n", silent: true},
		{name: "malformed yaml", yaml: "cv: [\n", silent: true},
		{name: "success panel", yaml: validCV, silent: true},
		// The decorator path, outside the Live's console.
		{name: "empty input file", yaml: "", silent: false},
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
				InputPath: input, Quiet: true,
				NoTypst: true, NoPDF: true, NoPNG: true, NoMarkdown: true, NoHTML: true,
			}, &stdout, &stderr)

			if silent := stdout.Len() == 0; silent != row.silent {
				t.Errorf("stdout empty = %v, want %v (%d bytes: %q)",
					silent, row.silent, stdout.Len(), stdout.String())
			}
		})
	}
}

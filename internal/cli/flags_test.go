package cli

import (
	"io"
	"reflect"
	"slices"
	"testing"
)

// captured is what the injected runner saw, so a case can assert on the parsed
// options without a render happening.
type captured struct {
	render  *RenderOptions
	newCV   *NewOptions
	invoked bool
}

func parse(t *testing.T, args ...string) (captured, int) {
	t.Helper()

	var seen captured
	code := execute(args, io.Discard, io.Discard, runners{
		render: func(options RenderOptions, _, _ io.Writer) int {
			seen.render, seen.invoked = &options, true
			return 0
		},
		newCV: func(options NewOptions, _, _ io.Writer) int {
			seen.newCV, seen.invoked = &options, true
			return 0
		},
	})
	return seen, code
}

// TestRenderFlagInventory is spec 012 §2 behavior 6's table, one subtest per
// spelling.
//
// **Every option upstream declares has two spellings and the corpus uses only
// one of them.** `render_typst_only` and `render_custom_paths` between them pass
// ten of the seventeen options and name no long form at all, so a port could —
// and did — register `--typ` where upstream declares `--typst-path` and pass
// every case in the suite. The expectations here are read off
// `render_command.py`'s signature rather than off a golden, for that reason.
func TestRenderFlagInventory(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		check func(*testing.T, RenderOptions)
	}{
		{
			name: "output folder long",
			args: []string{"--output-folder", "out"},
			check: func(t *testing.T, o RenderOptions) {
				if o.OutputFolder != "out" {
					t.Errorf("OutputFolder = %q, want %q", o.OutputFolder, "out")
				}
			},
		},
		{
			name: "output folder short",
			args: []string{"-o", "out"},
			check: func(t *testing.T, o RenderOptions) {
				if o.OutputFolder != "out" {
					t.Errorf("OutputFolder = %q, want %q", o.OutputFolder, "out")
				}
			},
		},
		{
			name: "typst path long",
			args: []string{"--typst-path", "a.typ"},
			check: func(t *testing.T, o RenderOptions) {
				if o.TypstPath != "a.typ" {
					t.Errorf("TypstPath = %q, want %q", o.TypstPath, "a.typ")
				}
			},
		},
		{
			name: "typst path short",
			args: []string{"-typ", "a.typ"},
			check: func(t *testing.T, o RenderOptions) {
				if o.TypstPath != "a.typ" {
					t.Errorf("TypstPath = %q, want %q", o.TypstPath, "a.typ")
				}
			},
		},
		{
			name: "pdf path long",
			args: []string{"--pdf-path", "a.pdf"},
			check: func(t *testing.T, o RenderOptions) {
				if o.PDFPath != "a.pdf" {
					t.Errorf("PDFPath = %q, want %q", o.PDFPath, "a.pdf")
				}
			},
		},
		{
			name: "pdf path short",
			args: []string{"-pdf", "a.pdf"},
			check: func(t *testing.T, o RenderOptions) {
				if o.PDFPath != "a.pdf" {
					t.Errorf("PDFPath = %q, want %q", o.PDFPath, "a.pdf")
				}
			},
		},
		{
			name: "png path long",
			args: []string{"--png-path", "a.png"},
			check: func(t *testing.T, o RenderOptions) {
				if o.PNGPath != "a.png" {
					t.Errorf("PNGPath = %q, want %q", o.PNGPath, "a.png")
				}
			},
		},
		{
			name: "png path short",
			args: []string{"-png", "a.png"},
			check: func(t *testing.T, o RenderOptions) {
				if o.PNGPath != "a.png" {
					t.Errorf("PNGPath = %q, want %q", o.PNGPath, "a.png")
				}
			},
		},
		{
			name: "markdown path long",
			args: []string{"--markdown-path", "a.md"},
			check: func(t *testing.T, o RenderOptions) {
				if o.MarkdownPath != "a.md" {
					t.Errorf("MarkdownPath = %q, want %q", o.MarkdownPath, "a.md")
				}
			},
		},
		{
			name: "markdown path short",
			args: []string{"-md", "a.md"},
			check: func(t *testing.T, o RenderOptions) {
				if o.MarkdownPath != "a.md" {
					t.Errorf("MarkdownPath = %q, want %q", o.MarkdownPath, "a.md")
				}
			},
		},
		{
			name: "html path long",
			args: []string{"--html-path", "a.html"},
			check: func(t *testing.T, o RenderOptions) {
				if o.HTMLPath != "a.html" {
					t.Errorf("HTMLPath = %q, want %q", o.HTMLPath, "a.html")
				}
			},
		},
		{
			name: "html path short",
			args: []string{"-html", "a.html"},
			check: func(t *testing.T, o RenderOptions) {
				if o.HTMLPath != "a.html" {
					t.Errorf("HTMLPath = %q, want %q", o.HTMLPath, "a.html")
				}
			},
		},
		{
			name: "design long",
			args: []string{"--design", "d.yaml"},
			check: func(t *testing.T, o RenderOptions) {
				if o.DesignPath != "d.yaml" {
					t.Errorf("DesignPath = %q, want %q", o.DesignPath, "d.yaml")
				}
			},
		},
		{
			name: "design short",
			args: []string{"-d", "d.yaml"},
			check: func(t *testing.T, o RenderOptions) {
				if o.DesignPath != "d.yaml" {
					t.Errorf("DesignPath = %q, want %q", o.DesignPath, "d.yaml")
				}
			},
		},
		{
			// **The long form is `--locale-catalog`, not `--locale`** — `new`
			// spells the same concept `--locale` and `render` does not.
			name: "locale catalog long",
			args: []string{"--locale-catalog", "l.yaml"},
			check: func(t *testing.T, o RenderOptions) {
				if o.LocalePath != "l.yaml" {
					t.Errorf("LocalePath = %q, want %q", o.LocalePath, "l.yaml")
				}
			},
		},
		{
			name: "locale catalog short",
			args: []string{"-lc", "l.yaml"},
			check: func(t *testing.T, o RenderOptions) {
				if o.LocalePath != "l.yaml" {
					t.Errorf("LocalePath = %q, want %q", o.LocalePath, "l.yaml")
				}
			},
		},
		{
			name: "settings long",
			args: []string{"--settings", "s.yaml"},
			check: func(t *testing.T, o RenderOptions) {
				if o.SettingsPath != "s.yaml" {
					t.Errorf("SettingsPath = %q, want %q", o.SettingsPath, "s.yaml")
				}
			},
		},
		{
			name: "settings short",
			args: []string{"-s", "s.yaml"},
			check: func(t *testing.T, o RenderOptions) {
				if o.SettingsPath != "s.yaml" {
					t.Errorf("SettingsPath = %q, want %q", o.SettingsPath, "s.yaml")
				}
			},
		},
		{
			name: "dont generate typst long",
			args: []string{"--dont-generate-typst"},
			check: func(t *testing.T, o RenderOptions) {
				if !o.NoTypst {
					t.Error("NoTypst = false, want true")
				}
			},
		},
		{
			name: "dont generate typst short",
			args: []string{"-notyp"},
			check: func(t *testing.T, o RenderOptions) {
				if !o.NoTypst {
					t.Error("NoTypst = false, want true")
				}
			},
		},
		{
			name: "dont generate pdf long",
			args: []string{"--dont-generate-pdf"},
			check: func(t *testing.T, o RenderOptions) {
				if !o.NoPDF {
					t.Error("NoPDF = false, want true")
				}
			},
		},
		{
			name: "dont generate pdf short",
			args: []string{"-nopdf"},
			check: func(t *testing.T, o RenderOptions) {
				if !o.NoPDF {
					t.Error("NoPDF = false, want true")
				}
			},
		},
		{
			name: "dont generate png long",
			args: []string{"--dont-generate-png"},
			check: func(t *testing.T, o RenderOptions) {
				if !o.NoPNG {
					t.Error("NoPNG = false, want true")
				}
			},
		},
		{
			name: "dont generate png short",
			args: []string{"-nopng"},
			check: func(t *testing.T, o RenderOptions) {
				if !o.NoPNG {
					t.Error("NoPNG = false, want true")
				}
			},
		},
		{
			name: "dont generate markdown long",
			args: []string{"--dont-generate-markdown"},
			check: func(t *testing.T, o RenderOptions) {
				if !o.NoMarkdown {
					t.Error("NoMarkdown = false, want true")
				}
			},
		},
		{
			name: "dont generate markdown short",
			args: []string{"-nomd"},
			check: func(t *testing.T, o RenderOptions) {
				if !o.NoMarkdown {
					t.Error("NoMarkdown = false, want true")
				}
			},
		},
		{
			name: "dont generate html long",
			args: []string{"--dont-generate-html"},
			check: func(t *testing.T, o RenderOptions) {
				if !o.NoHTML {
					t.Error("NoHTML = false, want true")
				}
			},
		},
		{
			name: "dont generate html short",
			args: []string{"-nohtml"},
			check: func(t *testing.T, o RenderOptions) {
				if !o.NoHTML {
					t.Error("NoHTML = false, want true")
				}
			},
		},
		{
			name: "quiet long",
			args: []string{"--quiet"},
			check: func(t *testing.T, o RenderOptions) {
				if !o.Quiet {
					t.Error("Quiet = false, want true")
				}
			},
		},
		{
			name: "quiet short",
			args: []string{"-q"},
			check: func(t *testing.T, o RenderOptions) {
				if !o.Quiet {
					t.Error("Quiet = false, want true")
				}
			},
		},
		{
			// **`--watch` must still *parse***, even though `Render` now
			// refuses it once parsed (G-10, spec §6.2 defers the watcher to
			// iteration 13): upstream accepts the flag on the argument
			// vector, so rejecting it at the parser would be a separate,
			// axis-2 difference from the one this port has chosen.
			name: "watch long",
			args: []string{"--watch"},
			check: func(t *testing.T, o RenderOptions) {
				if !o.Watch {
					t.Error("Watch = false, want true")
				}
			},
		},
		{
			name: "watch short",
			args: []string{"-w"},
			check: func(t *testing.T, o RenderOptions) {
				if !o.Watch {
					t.Error("Watch = false, want true")
				}
			},
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			args := append([]string{"render", "cv.yaml"}, row.args...)
			seen, code := parse(t, args...)
			if !seen.invoked {
				t.Fatalf("render was never reached; exit code %d", code)
			}
			if seen.render.InputPath != "cv.yaml" {
				t.Errorf("InputPath = %q, want %q", seen.render.InputPath, "cv.yaml")
			}
			row.check(t, *seen.render)
		})
	}
}

// TestInventedLongNamesAreNotDeclared is the negative half of the inventory
// above, and the class that hid longest: **the port once registered seven long
// option names upstream has never had.**
//
// `render_command.py:33-188` declares each option with exactly one long
// spelling and one whole-word single-dash spelling. `--nopdf` is not the second
// of those — it is `-nopdf` with a dash bolted on, and click's
// `_match_long_opt` is an exact-table lookup, so upstream has never matched it.
// Every corpus case writes the short form, so nothing in the suite could see
// the difference; `TestRenderFlagInventory` asserts only that the *correct*
// long names work, which a port declaring both would also satisfy.
//
// Measured against the vendored CLI: each of these is an ordinary unknown
// token, appended verbatim to `ctx.args` and reaching
// `parse_override_arguments`, which reports an extra-arguments error on the odd
// count (`parse_override_arguments.py:35-42`). The gate here is one step
// earlier and does not need a document: the token must land in `Extras` and
// **nothing else about the parse may move**.
//
// The table is derived from `renderShortFlags` rather than written out, so a
// short spelling added later is covered without an edit.
func TestInventedLongNamesAreNotDeclared(t *testing.T) {
	baseline, code := parse(t, "render", "cv.yaml")
	if !baseline.invoked {
		t.Fatalf("render was never reached for the bare vector; exit code %d", code)
	}

	shorts := make([]string, 0, len(renderShortFlags))
	for short := range renderShortFlags {
		shorts = append(shorts, short)
	}
	slices.Sort(shorts)

	for _, short := range shorts {
		t.Run("--"+short, func(t *testing.T) {
			token := "--" + short
			seen, code := parse(t, "render", "cv.yaml", token)
			if !seen.invoked {
				t.Fatalf("render was never reached; exit code %d — %s parsed as an option", code, token)
			}
			if !slices.Equal(seen.render.Extras, []string{token}) {
				t.Errorf("Extras = %q, want %q — %s is not an option upstream declares",
					seen.render.Extras, []string{token}, token)
			}
			assertOnlyExtrasDiffer(t, *baseline.render, *seen.render, token)
		})
	}
}

// TestInventedLongNamesWithValuesAreOverrideKeys is the same class in its other
// measured shape.
//
// The five path options have single-dash spellings that read like plausible
// long ones — `-typ`, `-pdf`, `-png`, `-md`, `-html` — and upstream answers
// `--typ out.typ` by making it an override **key and value**: two tokens, an
// even count, so it reaches the model as the key `typ` rather than as a usage
// error. A port that declared `--typ` would swallow both and set the path
// instead, silently.
func TestInventedLongNamesWithValuesAreOverrideKeys(t *testing.T) {
	baseline, code := parse(t, "render", "cv.yaml")
	if !baseline.invoked {
		t.Fatalf("render was never reached for the bare vector; exit code %d", code)
	}

	for _, row := range []struct{ token, value string }{
		{"--typ", "out.typ"},
		{"--pdf", "out.pdf"},
		{"--png", "out.png"},
		{"--md", "out.md"},
		{"--html", "out.html"},
		{"--o", "out"},
		{"--lc", "l.yaml"},
		{"--s", "s.yaml"},
	} {
		t.Run(row.token, func(t *testing.T) {
			seen, code := parse(t, "render", "cv.yaml", row.token, row.value)
			if !seen.invoked {
				t.Fatalf("render was never reached; exit code %d — %s parsed as an option", code, row.token)
			}
			if want := []string{row.token, row.value}; !slices.Equal(seen.render.Extras, want) {
				t.Errorf("Extras = %q, want %q — %s is an override key, not an option",
					seen.render.Extras, want, row.token)
			}
			assertOnlyExtrasDiffer(t, *baseline.render, *seen.render, row.token)
		})
	}
}

// assertOnlyExtrasDiffer reports every field of the parse the token moved. An
// invented long name must leave the options exactly as the bare vector left
// them — asserting on `Extras` alone would still pass a port that both
// collected the token *and* set the flag.
func assertOnlyExtrasDiffer(t *testing.T, baseline, got RenderOptions, token string) {
	t.Helper()

	baseline.Extras, got.Extras = nil, nil
	if !reflect.DeepEqual(baseline, got) {
		t.Errorf("%s changed the parsed options: %+v, want %+v", token, got, baseline)
	}
}

// TestNewFlagInventory is spec 012 §3 behaviors 13 and 14.
func TestNewFlagInventory(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		check func(*testing.T, NewOptions)
	}{
		{
			name: "theme",
			args: []string{"--theme", "moderncv"},
			check: func(t *testing.T, o NewOptions) {
				if o.Theme != "moderncv" {
					t.Errorf("Theme = %q, want %q", o.Theme, "moderncv")
				}
			},
		},
		{
			name: "locale",
			args: []string{"--locale", "french"},
			check: func(t *testing.T, o NewOptions) {
				if o.Locale != "french" {
					t.Errorf("Locale = %q, want %q", o.Locale, "french")
				}
			},
		},
		{
			name: "create typst templates",
			args: []string{"--create-typst-templates"},
			check: func(t *testing.T, o NewOptions) {
				if !o.CreateTypstTemplates {
					t.Error("CreateTypstTemplates = false, want true")
				}
			},
		},
		{
			// The companion of the above (`new_command.py:57`). No corpus case
			// passes it, which is why it was missing.
			name: "create markdown templates",
			args: []string{"--create-markdown-templates"},
			check: func(t *testing.T, o NewOptions) {
				if !o.CreateMarkdownTemplates {
					t.Error("CreateMarkdownTemplates = false, want true")
				}
			},
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			args := append([]string{"new", "John Doe"}, row.args...)
			seen, code := parse(t, args...)
			if !seen.invoked {
				t.Fatalf("new was never reached; exit code %d", code)
			}
			if seen.newCV.Name != "John Doe" {
				t.Errorf("Name = %q, want %q", seen.newCV.Name, "John Doe")
			}
			row.check(t, *seen.newCV)
		})
	}
}

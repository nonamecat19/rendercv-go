package cli

import (
	"io"
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
			// **`--watch` is declared and not implemented** (spec §6.2), but it
			// must still *parse*: upstream accepts it, so rejecting it is an
			// axis-2 difference on the argument vector regardless of what the
			// port then does with it.
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

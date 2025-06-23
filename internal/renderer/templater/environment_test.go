package templater_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/flosch/pongo2/v6"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater"
)

// The candidate order, asserted as a list rather than through which file wins.
//
// A test that only checks the winner cannot tell a correct order from a lucky
// one: with only a built-in present, every ordering produces the same answer.
func TestLoaderCandidates(t *testing.T) {
	loader := templater.Loader{}

	tests := []struct {
		name   string
		format templater.Format
		theme  string
		want   []string
	}{
		{
			// Typst asks for the theme-qualified path **first**, which is what
			// lets a user override one entry type for one theme.
			name: "typst tries the theme first", format: templater.FormatTypst, theme: "classic",
			want: []string{"classic/EducationEntry.j2.typ", "typst/EducationEntry.j2.typ"},
		},
		{
			// Markdown skips the theme lookup entirely (templater.py:160-165).
			name: "markdown has no theme lookup", format: templater.FormatMarkdown, theme: "classic",
			want: []string{"markdown/EducationEntry.j2.md"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := loader.Candidates(tc.format, tc.theme, "EducationEntry")
			if len(got) != len(tc.want) {
				t.Fatalf("= %q, want %q", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// `Full.html` carries no `.j2.` infix, unlike every other fragment.
func TestHTMLFragmentHasNoJ2Infix(t *testing.T) {
	got := templater.Loader{}.Candidates(templater.FormatHTML, "", "Full.html")
	if len(got) != 1 || got[0] != "html/Full.html" {
		t.Errorf("= %q, want [html/Full.html]", got)
	}
}

// A user override of **one entry type for one theme**, which is spec 008 §5's
// first acceptance criterion and the reason the loader has two directories.
func TestAUserOverridesOneEntryTypeForOneTheme(t *testing.T) {
	builtin := fstest.MapFS{
		"typst/EducationEntry.j2.typ": {Data: []byte("built-in")},
		"typst/NormalEntry.j2.typ":    {Data: []byte("built-in normal")},
	}

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "classic"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "classic", "EducationEntry.j2.typ"),
		[]byte("mine"), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	loader := templater.Loader{InputDir: dir, Builtin: builtin}

	got, err := loader.Load(templater.FormatTypst, "classic", "EducationEntry")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != "mine" {
		t.Errorf("= %q, want the override", got)
	}

	// The override is scoped: another entry type still comes from the built-ins.
	got, err = loader.Load(templater.FormatTypst, "classic", "NormalEntry")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != "built-in normal" {
		t.Errorf("= %q, want the built-in", got)
	}

	// And so is the theme: the same override under a different theme is not
	// found, because the first candidate names the theme.
	got, err = loader.Load(templater.FormatTypst, "sb2nov", "EducationEntry")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != "built-in" {
		t.Errorf("= %q, want the built-in for another theme", got)
	}
}

// A format-wide override, the second directory's other use.
func TestAUserOverridesAFormatWideTemplate(t *testing.T) {
	builtin := fstest.MapFS{"typst/Header.j2.typ": {Data: []byte("built-in")}}

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "typst"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "typst", "Header.j2.typ"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := templater.Loader{InputDir: dir, Builtin: builtin}
	got, err := loader.Load(templater.FormatTypst, "classic", "Header")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != "mine" {
		t.Errorf("= %q, want the override", got)
	}
}

// **Jinja's `indent`, not pongo2's.** Every row measured against Jinja with
// `trim_blocks` and `lstrip_blocks` on, which is upstream's environment.
func TestIndentFilterMatchesJinja(t *testing.T) {
	tests := []struct {
		name     string
		template string
		value    string
		want     string
	}{
		{
			// The first line is **not** indented. pongo2's filter indents it,
			// and that one difference is the port's most likely silent
			// whitespace diff.
			name:     "the first line is not indented",
			template: "{{ x|indent:4 }}", value: "a\nb\nc",
			want: "a\n    b\n    c",
		},
		{"a single line is untouched", "{{ x|indent:4 }}", "a", "a"},
		{
			// Blank lines are skipped: Jinja's `blank` defaults to False.
			name:     "a blank line stays blank",
			template: "{{ x|indent:4 }}", value: "a\n\nb",
			want: "a\n\n    b",
		},
		{"empty", "{{ x|indent:4 }}", "", ""},
		{"existing indentation is added to", "{{ x|indent:4 }}", "  a\n  b", "  a\n      b"},
		{"the width is the argument", "{{ x|indent:8 }}", "a\nb", "a\n        b"},
	}

	env, err := templater.NewEnvironment("", fstest.MapFS{}, "classic")
	if err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}
	_ = env // the filters are registered as a side effect

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			template, err := pongo2.FromString(tc.template)
			if err != nil {
				t.Fatalf("FromString: %v", err)
			}
			got, err := template.Execute(pongo2.Context{"x": tc.value})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if got != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}
}

// The `indent(4)` / `replace("    ", "")` pair the templates use four times.
// It cancels **exactly** only because the first line is excluded from both.
func TestIndentAndReplaceCancel(t *testing.T) {
	env, err := templater.NewEnvironment("", fstest.MapFS{}, "classic")
	if err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}
	_ = env

	template, err := pongo2.FromString(`{{ x|indent:4|replace:"/    //" }}`)
	if err != nil {
		t.Fatalf("FromString: %v", err)
	}
	got, err := template.Execute(pongo2.Context{"x": "a\nb\nc"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got != "a\nb\nc" {
		t.Errorf("= %q, want the original back", got)
	}
}

// The two custom filters are registered even though no built-in template uses
// them, so a user override that does still works.
func TestTheCustomFiltersAreAvailable(t *testing.T) {
	if _, err := templater.NewEnvironment("", fstest.MapFS{}, "classic"); err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}

	for _, tc := range []struct{ template, want string }{
		{`{{ "https://x.com/"|clean_url }}`, "x.com"},
		{`{{ "  a  "|strip }}`, "a"},
	} {
		template, err := pongo2.FromString(tc.template)
		if err != nil {
			t.Fatalf("FromString(%s): %v", tc.template, err)
		}
		got, err := template.Execute(pongo2.Context{})
		if err != nil {
			t.Fatalf("Execute(%s): %v", tc.template, err)
		}
		if got != tc.want {
			t.Errorf("%s = %q, want %q", tc.template, got, tc.want)
		}
	}
}

// **pongo2 autoescapes by default and a `.typ` cannot survive it.**
//
// `escape_typst_characters` deliberately emits `\"`, `\<` and `\>`; HTML
// entities in their place corrupt every Typst document at every interpolation.
// This is the test that says the environment turns it off, and it is worth its
// own name because the failure looks like a template bug rather than an engine
// setting.
func TestAutoescapingIsOff(t *testing.T) {
	if _, err := templater.NewEnvironment("", fstest.MapFS{}, "classic"); err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}

	template, err := pongo2.FromString("{{ x }}")
	if err != nil {
		t.Fatalf("FromString: %v", err)
	}
	got, err := template.Execute(pongo2.Context{"x": `a<b>&"c"`})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got != `a<b>&"c"` {
		t.Errorf("= %q, want it unescaped", got)
	}
}

// The `replace` filter's sed-shaped parameter, which exists because pongo2
// filters take one argument and Jinja's `replace` takes two.
func TestReplaceFilter(t *testing.T) {
	if _, err := templater.NewEnvironment("", fstest.MapFS{}, "classic"); err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}

	tests := []struct {
		name     string
		template string
		value    string
		want     string
	}{
		{"a substitution", `{{ x|replace:"/-/ /" }}`, "a-b", "a b"},
		{"a removal", `{{ x|replace:"/    //" }}`, "    a", "a"},
		{
			// The separator is the parameter's first character, so a pattern
			// containing `/` picks another.
			name: "another separator", template: `{{ x|replace:"|/|:|" }}`,
			value: "a/b", want: "a:b",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			template, err := pongo2.FromString(tc.template)
			if err != nil {
				t.Fatalf("FromString: %v", err)
			}
			got, err := template.Execute(pongo2.Context{"x": tc.value})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if got != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}
}

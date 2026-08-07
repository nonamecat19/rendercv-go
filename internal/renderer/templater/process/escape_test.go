package process_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// `escape_typst_characters`, every row measured against the vendored Python.
//
// The rows are grouped by which of the three phases they exercise, because the
// function's whole behavior is that the phases run in order and see each other's
// output.
func TestEscapeTypstCharacters(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// The early return, before anything.
		{"a lone newline", "\n", "\n"},
		{"a newline inside text is not special", "a\nb", "a\nb"},
		{"nothing to do", "a", "a"},

		// Phase 2 — the thirteen single characters.
		{"brackets", "a[b]c", `a\[b\]c`},
		{"underscore", "under_score", `under\_score`},
		{"slash", "a/b", `a\/b`},
		{"angles", "a<b>c", `a\<b\>c`},
		{"quotes", `say "hi"`, `say \"hi\"`},
		{"at", "e@mail.com", `e\@mail.com`},
		{"tilde", "~tilde", `\~tilde`},
		{
			// A single `$` is **not** math — only `$$…$$` is held out — so it
			// escapes like any other character.
			name: "a single dollar is escaped",
			in:   "$math$",
			want: `\$math\$`,
		},
		{
			// The one that proves phase 2 is simultaneous: a sequential pass
			// that escaped `\` after `[` would produce `a\\[b`.
			name: "a backslash is not re-escaped",
			in:   `back\slash`,
			want: `back\\slash`,
		},

		// Phase 1 — math and commands held out.
		{"math collapses its doubled dollars", "$$x$$", "$x$"},
		{"math inside text", "a $$x+y$$ b", "a $x+y$ b"},
		{
			// The footer's page counter (spec 008 §4D behavior 43). Without
			// phase 1 this becomes `\#str(here().page())` and every page footer
			// prints its own source.
			name: "a Typst command survives",
			in:   "#str(here().page())",
			want: "#str(here().page())",
		},
		{
			// And the connection list's, which carries a quoted URL — the `"`
			// and `/` inside it must not be escaped either.
			name: "a command with a quoted argument survives whole",
			in:   `#link("https://x")[y]`,
			want: `#link("https://x")[y]`,
		},
		{
			"a command with a bracketed body", `#connection-with-icon("phone")[+1]`,
			`#connection-with-icon("phone")[+1]`,
		},
		{"a bare command name", "#sym.ast.basic", "#sym.ast.basic"},
		{
			// `%` escapes, `#tag` does not — the two in one string.
			name: "escaping around a protected command",
			in:   "100% #tag",
			want: `100\% #tag`,
		},

		// Phase 3 — the two longer replacements, after phase 2.
		{"a bulleted asterisk", "* item", "#sym.ast.basic item"},
		{"two bulleted asterisks", "* a * b", "#sym.ast.basic a #sym.ast.basic b"},
		{
			// The bare-`*` rule inserts a `#` **after** phase 2 escaped every
			// `#`, which is why the inserted one survives unescaped.
			name: "a bare asterisk",
			in:   "a*b",
			want: "a#sym.ast.basic#h(0pt, weak: true) b",
		},
		{
			name: "markdown bold reaches here unparsed",
			in:   "*bold*",
			want: "#sym.ast.basic#h(0pt, weak: true) bold#sym.ast.basic#h(0pt, weak: true) ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := process.EscapeTypstCharacters(tc.in); got != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}
}

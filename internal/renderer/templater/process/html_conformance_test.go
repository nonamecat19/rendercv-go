//go:build conformance

package process_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// Differential against python-markdown over 75 shapes. The fixture is CPython's
// own output, generated through the vendored submodule's `markdown.markdown` —
// never hand-written (`AGENTS.md` §10.1).
//
// **It was red on purpose for one release and is not any more.** goldmark escapes
// `"` everywhere and python-markdown escapes it only inside an attribute, so any
// double quote in a CV produced a differing `.html`; `htmlescape.go` now
// implements python-markdown's three escaping contexts, its narrower
// backslash-escapable set, its verbatim treatment of entity-shaped tokens, its
// XHTML void elements, and its link-title scanning. Measured before the change:
// 33 of these 75 rows differed. After: the four in knownRemainder.
//
// It lives behind the conformance tag because it needs no upstream process but
// does encode upstream's exact output.
func TestMarkdownToHTMLMatchesPython(t *testing.T) {
	raw, err := os.ReadFile("testdata/html.json")
	if err != nil {
		t.Skipf("no fixture: %v", err)
	}
	var rows []struct{ In, Out string }
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		got, err := process.MarkdownToHTML(row.In)
		if err != nil {
			t.Fatalf("%q: %v", row.In, err)
		}
		if _, known := knownRemainder[row.In]; known {
			// Inverted, for the same reason conformance.AssertUnreachable is:
			// a list of tolerated mismatches that does not notice being fixed
			// becomes a mute button.
			if got == row.Out {
				t.Errorf("MarkdownToHTML(%q) now matches python-markdown"+
					" — remove it from knownRemainder", row.In)
			}
			continue
		}
		if got != row.Out {
			t.Errorf("MarkdownToHTML(%q)\n = %q\nwant %q", row.In, got, row.Out)
		}
	}
}

// knownRemainder is the four shapes still differing, with the reason each is a
// **separate** defect from the escaping this file's fix was about. All four were
// already failing before that fix — measured in an isolated worktree at the
// previous commit, where 33 of the 75 rows differed — so none is a regression,
// and none involves a quote in text.
//
// They are goldmark-versus-python-markdown differences in three areas this port
// has not yet reproduced: paragraph-trailing whitespace, nested emphasis
// ordering, and the escaping of a quote left inside a URL.
var knownRemainder = map[string]string{
	"&; ":                                "goldmark strips a paragraph's trailing space; python keeps it",
	"___strong em___":                    "the two libraries nest strong and em in opposite orders",
	"line with trailing spaces   \nnext": "python keeps one space before a hard break, goldmark strips it",
	"[t](u\"notitle)":                    "an unresolved title leaves a quote in the href, which python HTML-escapes and goldmark URL-escapes",
}

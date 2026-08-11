//go:build conformance

package process_test

import (
	"encoding/json"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/conformance"
	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// Differential against python-markdown over 113 shapes. The fixture is CPython's
// own output, generated through the vendored submodule's `markdown.markdown` —
// never hand-written (`AGENTS.md` §10.1).
//
// **The 75-row version of this fixture was the reason the port believed it was
// nearly done.** A fresh-context verifier found eight further divergence classes
// reachable from an ordinary CV highlight, none of them represented here and none
// recorded as a divergence; the rows below were added first, red, and the fixes
// followed one class per commit. Six of the eight closed, together with five more
// classes the reproduction turned up on the way — tab expansion, code-span
// newlines, URL escaping, line-break rules, and whitespace at the end of a block.
//
// Measured after that work: **5 of these 113 rows differ**, all of them in
// `knownRemainder`, checked by running `MarkdownToHTML` over every `In` and
// diffing against the fixture's `Out` — not read off a commit message.
//
// It lives behind the conformance tag because it needs no upstream process but
// does encode upstream's exact output.
// **An absent fixture fails this test rather than skipping it.** It used to
// skip, which meant the whole 113-row differential could leave `just
// test-parity` without changing a single character of its output —
// `conformance.RequireInput` makes the fixture's absence loud, with the shared
// opt-out for someone who deliberately has not generated it
// (`internal/conformance/requireinput.go`).
func TestMarkdownToHTMLMatchesPython(t *testing.T) {
	raw := conformance.RequireInput(t, "testdata/html.json",
		"the 113-row python-markdown differential",
		"regenerate it through the vendored submodule's `markdown.markdown` (AGENTS.md §10.1)")
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

// knownRemainder is the five shapes still differing. Each is pinned by an
// **inverted** assertion above — the case still runs, still has to produce
// output, and still has to differ — for the same reason
// `conformance.AssertUnreachable` is: a list of tolerated mismatches that cannot
// notice being fixed is a mute button.
//
// They fall into three classes, and each is a *reimplementation*, not an
// oversight. None is recorded in `specs/divergences.md` yet; that file is human-
// gated (`AGENTS.md` §5) and the proposals are with the iteration owner.
//
//  1. **Emphasis.** python-markdown resolves `*` and `_` with two regex-driven
//     tree processors (`AsteriskProcessor`, `UnderscoreProcessor`,
//     `inlinepatterns.py:93-94`) and CommonMark uses delimiter runs, so the two
//     disagree on nesting order, on strong inside em, and on `_` between word
//     characters. Matching would mean replacing goldmark's emphasis parser
//     wholesale.
//  2. **A destination with a space.** `getLink` balances parentheses and takes
//     whatever is between them (`inlinepatterns.py:716-830`); CommonMark requires
//     `<…>` around a space. goldmark's link parsing runs on a delimiter stack
//     whose label handling is unexported, so a replacement parser cannot parse
//     the label — the image one below it can only be written because upstream
//     never parses an image's label at all.
//  3. **A block-level tag inside a list item.** python-markdown stashes raw HTML
//     in a preprocessor before any block parsing, so the `<div>` is part of the
//     item's text; goldmark opens a real HTML block inside the item and the two
//     differ by a newline.
var knownRemainder = map[string]string{
	"___strong em___":    "the two libraries nest strong and em in opposite orders",
	"*a **bold** thing*": "python closes and reopens the em around a nested strong",
	"_a __b__ c_":        "python does not read `__` between word characters as strong",
	"[t](a b)":           "python accepts an unbracketed space in a destination",
	"- <div>block</div>": "python stashes the raw block before the list item is parsed",
}

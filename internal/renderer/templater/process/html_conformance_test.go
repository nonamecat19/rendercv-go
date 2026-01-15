//go:build conformance

package process_test

import (
	"encoding/json"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/conformance"
	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// Differential against python-markdown, now 256 shapes and growing. The
// fixture is CPython's own output, generated through the vendored submodule's
// `markdown.markdown` — never hand-written (`AGENTS.md` §10.1).
//
// **The 75-row version of this fixture was the reason the port believed it was
// nearly done.** A fresh-context verifier found eight further divergence classes
// reachable from an ordinary CV highlight, none of them represented here and none
// recorded as a divergence; the rows below were added first, red, and the fixes
// followed one class per commit. Six of the eight closed, together with five more
// classes the reproduction turned up on the way — tab expansion, code-span
// newlines, URL escaping, line-break rules, and whitespace at the end of a block.
// Iteration 11's Wave C (spec §7-8) added the emphasis and link-destination
// classes, then 17 more rows pinning a regression a fresh-context verifier
// found in Wave C's own first pass.
//
// **A second fresh-context verifier then measured 147 shapes that Wave C had
// broken and no fixture could see**, 124 of them one class: a delimiter run
// flanked by whitespace. The 35 rows at the end of this file pin all six
// classes it reported, and the mechanism they forced —
// resolving every pattern registered above `em_strong` before the emphasis
// matchers ever run — is `stash.go`'s `maskAbove`. Both verdicts came from a
// green suite, which is the whole argument for the differential being wide.
//
// A third verifier then found a panic and 300 more regressions, and a fourth
// round closed the registry-priority rule for the two emphasis processors.
//
// Measured after that work: 2 of these rows differ, both in `knownRemainder`,
// checked by running `MarkdownToHTML` over every `In` and diffing against the
// fixture's `Out` — not read off a commit message.
//
// It lives behind the conformance tag because it needs no upstream process but
// does encode upstream's exact output.
// **An absent fixture fails this test rather than skipping it.** It used to
// skip, which meant the whole differential could leave `just test-parity`
// without changing a single character of its output —
// `conformance.RequireInput` makes the fixture's absence loud, with the shared
// opt-out for someone who deliberately has not generated it
// (`internal/conformance/requireinput.go`).
func TestMarkdownToHTMLMatchesPython(t *testing.T) {
	raw := conformance.RequireInput(t, "testdata/html.json",
		"the python-markdown differential",
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

// knownRemainder is what still differs after Wave C (spec 011 §7-8, T8-T12):
// one shape spec §9.3 declines on purpose, the block-tag-in-a-list-item class
// §9.5 leaves open, and the blockquote soft-break class §12.5 records. Each is pinned by an **inverted** assertion above —
// the case still runs, still has to produce output, and still has to differ —
// for the same reason `conformance.AssertUnreachable` is: a list of tolerated
// mismatches that cannot notice being fixed is a mute button.
//
// None is recorded in `specs/divergences.md`; that file is human-gated
// (`AGENTS.md` §5) and spec 011 §9.4 argues none is parity-impossible.
//
//  1. **A destination spanning a line break.** `getLink` scans the block's
//     whole text, so `[t](a\nb)` upstream is one link with a literal newline
//     in its `href`; matching that needs a block-level scanner. Spec 011 §9.3.
//  2. **A block-level tag inside a list item.** python-markdown stashes raw HTML
//     in a preprocessor before any block parsing, so the `<div>` is part of the
//     item's text; goldmark opens a real HTML block inside the item and the two
//     differ by a newline. Spec 011 §9.5, nobody's taken it yet.
//  3. **Emphasis across a soft line break inside a blockquote.** `blockWindow`
//     ends its window at the first gap that is not pure whitespace, and a
//     blockquote continuation contributes `\n> `. Spanning it needs the text
//     the *reader* sees — markers stripped — together with a map back to source
//     offsets, which this parser does not carry: `advanceTo` fixed the position
//     half of that problem, not the text half. A list item's continuation is
//     only stripped whitespace, so `- *a\n  b*` does match. Pre-existing, not a
//     Wave C regression, and the reason spec 011 §12.5 states behavior 41's
//     limit explicitly rather than claiming soft breaks work everywhere.
var knownRemainder = map[string]string{
	"- <div>block</div>": "python stashes the raw block before the list item is parsed",
	"[t](a\nb)":          "spec 011 §9.3: a destination spanning a line break is out of scope, declined permanently",

	// Spec 011 §12.5: a blockquote's `> ` gap needs an offset map.
	"> *a\n> b*":   "blockWindow stops at the stripped `>`",
	"> **a\n> b**": "blockWindow stops at the stripped `>`",
	"> *a\n> b* c": "blockWindow stops at the stripped `>`",
	"> _a\n> b_":   "blockWindow stops at the stripped `>`",
}

package process_test

// A block-level tag inside a container — spec 011 §9.5's class, enumerated.
//
// The class has been open since iteration 11 with exactly one row standing for
// it, `- <div>block</div>`, pinned in `knownRemainder`. 33 shapes were added
// with `tools/mdprobe`: every list marker and both list kinds, a tag with text
// around it, a comment, a void tag, a `<script>`, a `<table>`, a multi-line
// tag, a continuation line, two nestings, three blockquote forms, the
// three-space window `at_line_start` allows outside any container, and the
// inline-level controls. **22 of the 33 differ**, so the one pinned key was
// standing for twenty-two.
//
// Upstream's rule is a *source* rule, which is why a container changes it.
// `HtmlBlockPreprocessor` (`preprocessors.py:86-91`) extracts raw HTML from the
// string before any block parsing, so there is no list item yet — only text —
// and `at_line_start` (`htmlparser.py:181-191`) asks whether everything before
// the tag on the **physical line** is whitespace, three characters at most. A
// `- ` marker is neither, so the tag never opens a raw block; it stays in the
// item and the *inline* `html` pattern (`inlinepatterns.py:90`) stashes it.
// goldmark asks after the marker has been consumed, so it sees a line-initial
// tag and opens a block inside the `<li>`. `htmlblock.go`'s `atLineStart` puts
// the question back where upstream asks it, and 18 of the 22 close.

// containerBlockTag is the four `at_line_start` does **not** close. None of the
// four is this class, which is why they are still here and not a regression:
//
//  1. `- <div>\nmulti\n</div>` — a block tag spanning lines inside an item.
//     Upstream keeps the whole run as the item's text; the port's inline stash
//     (`inline.go`'s `matchRawHTML`) is single-line, so the `</div>` on its own
//     line escapes the `<li>`. **This is the one shape in the enumeration that
//     needs what spec 011 §9.3 declines for link destinations** — a scanner
//     over the block's whole text — so it is recorded, not attempted.
//  2. `- <div>a</div>\n  <div>b</div>` — upstream puts a `<br />` between the
//     two. That is the **block** stash, not the inline one: the continuation
//     line *is* at line start, so `handle_starttag` takes it and
//     `htmlparser.py:216-218` appends a newline to `cleandoc`.
//  3. and 4. `- outer\n    - <div>x</div>` and `- <div>x</div>\n    - nested`
//     differ only by `outer<ul>` against `outer\n<ul>` — the **nested-list
//     newline**, which reproduces with no HTML in it at all: `- outer\n    -
//     inner` differs the same way and predates this work.
//
// The pins stay inverted, so any of the four that starts matching fails.
var containerBlockTag = map[string]bool{
	"- <div>\nmulti\n</div>":         true,
	"- <div>a</div>\n  <div>b</div>": true,
	"- outer\n    - <div>x</div>":    true,
	"- <div>x</div>\n    - nested":   true,
}

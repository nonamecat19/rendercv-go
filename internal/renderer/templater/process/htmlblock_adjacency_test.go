package process_test

// The raw-HTML block adjacency, which neither differential could see.
//
// `html.json` has carried `{"<div>block</div>", "<div>block</div>"}` since
// iteration 11 and it matches perfectly. What it never carried is a raw block
// with **another block after it**, and that is where the two libraries part
// company. 42 shapes were added with `tools/mdprobe` — a raw block before and
// after a paragraph, another raw block, a bullet list, an ordered list, an ATX
// heading, a setext heading, an indented code block, a blockquote and a
// thematic break; a multi-line block, a comment, a void tag, a `<script>`, a
// chain of three, and the inline-HTML twin of each — and 29 of them differ.
//
// They differ for **two unrelated reasons**. The first — a raw block and the
// block after it separated by a blank line upstream and by one newline here —
// is closed by `htmlblock.go`'s `htmlBlockRenderer` and its 20 rows are now
// ordinary matching rows. What is left is the second.

// rawBlockTail is a **second defect**, found by the same enumeration and not
// fixed here.
//
// When the closing tag is *not* followed by a blank line, upstream ends the raw
// block anyway and parses what follows as ordinary markdown —
// `htmlparser.py:246-247` sets `intail` instead of extending the stash, and
// `:258-263` returns the tail to the document. goldmark's HTML block runs to
// the next blank line, so the port swallows the following content whole:
//
//	<div>block</div>\nafter        upstream <div>block</div>\n<p>after</p>
//	                                   port <div>block</div>\nafter
//	<div>block</div> tail\n\nafter   upstream <div>block</div>\n<p>tail</p>…
//	                                   port <div>block</div> tail…
//
// It is a block-parser change, not a renderer one, and it is the only one of
// the two that also moves the **Typst** path — the last row below is the one
// Typst row in this class. Its own unit; nothing here is a `divergences.md`
// entry, and that file is human-gated in any case (`AGENTS.md` §5).
var rawBlockTail = map[string]bool{
	"<div>block</div>\nafter":          true,
	"<div>block</div>\n- one\n- two":   true,
	"<div>block</div>\n1. one\n2. two": true,
	"<div>block</div>\n# H":            true,
	"<div>block</div>\nH\n==":          true,
	"<div>block</div>\n    indented":   true,
	"<div>block</div>\n> quoted":       true,
	"<div>block</div>\n---":            true,
	"<div>block</div> tail\n\nafter":   true,
}

// rawBlockTailTypst is `rawBlockTail`'s Typst half. Only the shape with content
// after the closing tag reaches the Typst path, because the newline-separated
// forms are already line-split there (`markdown_parser.py:175-189`).
var rawBlockTailTypst = map[string]bool{
	"<div>block</div> tail\n\nafter": true,
}

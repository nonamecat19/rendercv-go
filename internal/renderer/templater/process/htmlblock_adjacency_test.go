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
// They differ for **two unrelated reasons**, which is why there are two maps.

// rawBlockSeparator is spec 011 §9.5's mechanism, landed red before the fix
// (`AGENTS.md` §7).
//
// A raw block's text does not travel through the tree; `HTMLExtractor` stashes
// it and `htmlparser.py:242-244` appends a newline **to the stashed string** —
// "Preserve blank line and end of raw block" — whenever `blank_line_re`
// (`:93`) matches what follows the closing tag. `RawHtmlPostprocessor`
// (`postprocessors.py:83-86`) puts that newline back into the serialized
// document on top of the ordinary inter-block separator
// (`treeprocessors.py:421`), so upstream writes a **blank line** after a raw
// block and the port writes one newline.
//
// These 20 rows are that difference and nothing else. The pins are inverted —
// a row that starts matching fails the suite — and the map goes away with the
// fix.
var rawBlockSeparator = map[string]bool{
	"<div>block</div>\n\nafter":                    true,
	"<div>block</div>\n\n<section>two</section>":   true,
	"<section>two</section>\n\n<div>block</div>":   true,
	"<div>block</div>\n\n- one\n- two":             true,
	"<div>block</div>\n\n1. one\n2. two":           true,
	"<div>block</div>\n\n# H":                      true,
	"<div>block</div>\n\nH\n==":                    true,
	"<div>block</div>\n\n    indented":             true,
	"<div>block</div>\n\n> quoted":                 true,
	"<div>block</div>\n\n---":                      true,
	"<div>block</div>\n\n\nafter":                  true,
	"<div>\nmulti\nline\n</div>\n\nafter":          true,
	"<!-- comment -->\n\nafter":                    true,
	"<hr />\n\nafter":                              true,
	"<script>var a=1;</script>\n\nafter":           true,
	"<div>a</div>\n\n<div>b</div>\n\n<div>c</div>": true,
	"para\n\n<div>x</div>\n\npara2":                true,
	"<div>x</div>\n\npara\n\n<div>y</div>":         true,
	"<p>already</p>\n\nafter":                      true,
	"<div>x</div>\n\n    indented":                 true,
}

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

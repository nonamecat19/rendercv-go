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
// tag and opens a block inside the `<li>`.

// containerBlockTag is those 22, landed red before the fix (`AGENTS.md` §7).
// The pins are inverted — a row that starts matching fails the suite — and 18
// of them go away with the fix.
var containerBlockTag = map[string]bool{
	"- <div>block</div>":                   true,
	"- <div>block</div>\n- second":         true,
	"- a\n- <div>x</div>":                  true,
	"- <div>x</div>\n\n- y":                true,
	"- <div>x</div> tail":                  true,
	"* <p>para</p>":                        true,
	"+ <div>x</div>":                       true,
	"1. <div>x</div>":                      true,
	"2. <div>x</div>\n3. second":           true,
	"- <hr />":                             true,
	"- <!-- comment -->":                   true,
	"- <script>var a=1;</script>":          true,
	"- <table><tr><td>a</td></tr></table>": true,
	"- <div>x</div>\n\nafter":              true,
	"before\n\n- <div>x</div>":             true,
	"> <div>x</div>":                       true,
	"> a\n> <div>x</div>":                  true,
	"> <div>x</div>\n\nafter":              true,
	"- <div>\nmulti\n</div>":               true,
	"- <div>a</div>\n  <div>b</div>":       true,
	"- outer\n    - <div>x</div>":          true,
	"- <div>x</div>\n    - nested":         true,
}

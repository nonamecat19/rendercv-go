package process

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// python-markdown escapes **three different things in three different places**,
// and conflating any two of them is what made every earlier attempt at this
// worse than the mismatch it fixed. Measured against `markdown.markdown` over
// the shapes in `testdata/html.json` plus ~40 probes:
//
//	                   `&`                     `<` `>`   `"`      backslash
//	text               kept if entity-shaped   escaped   kept     python's set
//	code span          always `&amp;`          escaped   kept     literal
//	attribute (alt)    kept if entity-shaped   escaped   `&quot;` python's set
//
// goldmark does one thing everywhere: resolve entities to characters, then
// escape `& < > "`. That is right for `<`, `>` and a bare `&`, and wrong for
// every other cell — most visibly for `"`, which made **any** double quote in a
// CV produce a differing `.html`.
//
// The three cells are reached through goldmark's own three writer methods
// (`Write` for text, `RawWrite` for code spans, and whatever the node renderer
// chooses for an attribute), which is why this is a Writer plus one node
// renderer rather than a wrapper around either alone.

// escapableChars is python-markdown's backslash-escapable set
// (`markdown/__init__.py`, `Markdown.ESCAPED_CHARS`).
//
// **It is narrower than CommonMark's, which is every ASCII punctuation mark.**
// `<`, `"` and `&` are absent, so `\<` keeps its backslash and renders as
// `\&lt;` where goldmark gives `&lt;` — measured on all three.
var escapableChars = [256]bool{
	'\\': true, '`': true, '*': true, '_': true, '{': true, '}': true,
	'[': true, ']': true, '(': true, ')': true, '>': true, '#': true,
	'+': true, '-': true, '.': true, '!': true,
}

// entityLength reports the length of the entity-shaped token starting at
// `source[i]`, which must be `&`, or 0 if there is none.
//
// python-markdown preserves anything of the shape `&name;`, `&#digits;` or
// `&#xhex;` **verbatim, without knowing whether it is a real entity** — measured:
// `&unknown;`, `&123;` and `&y;` all survive untouched, while `&;`, `&#;`,
// `&#abc;` and a bare `&` become `&amp;`. So the test is shape, not a table, and
// the port needs no entity list at all.
func entityLength(source []byte, i int) int {
	j := i + 1
	if j >= len(source) {
		return 0
	}

	if source[j] == '#' {
		j++
		if j < len(source) && (source[j] == 'x' || source[j] == 'X') {
			j++
			start := j
			for j < len(source) && isHexDigit(source[j]) {
				j++
			}
			if j > start && j < len(source) && source[j] == ';' {
				return j - i + 1
			}
			return 0
		}
		start := j
		for j < len(source) && source[j] >= '0' && source[j] <= '9' {
			j++
		}
		if j > start && j < len(source) && source[j] == ';' {
			return j - i + 1
		}
		return 0
	}

	start := j
	for j < len(source) && isAlphanumeric(source[j]) {
		j++
	}
	if j > start && j < len(source) && source[j] == ';' {
		return j - i + 1
	}
	return 0
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func isAlphanumeric(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// pythonWriter is goldmark's html.Writer with python-markdown's escaping rules.
//
// SecureWrite is delegated: it replaces characters that cannot appear in a URL,
// which is not an escaping question and where the two libraries agree.
type pythonWriter struct {
	inner html.Writer
}

// Write is the text cell: backslash escapes resolved against python's narrower
// set, entity-shaped tokens preserved, `& < >` escaped, `"` left alone.
func (p pythonWriter) Write(writer util.BufWriter, source []byte) {
	p.write(writer, source, true, false)
}

// RawWrite is the code-span cell: no backslash processing and **no entity
// preservation** — a code span's `&quot;` is `&amp;quot;` upstream, because its
// content is literal text — but still no `"` escaping.
func (p pythonWriter) RawWrite(writer util.BufWriter, source []byte) {
	p.write(writer, source, false, false)
}

// SecureWrite is delegated unchanged: it replaces characters that cannot appear
// in a URL, which is not an escaping question and where the two libraries agree.
func (p pythonWriter) SecureWrite(writer util.BufWriter, source []byte) {
	p.inner.SecureWrite(writer, source)
}

// writeAttribute is the attribute cell: the text rules plus `"` as `&quot;`,
// because an unescaped quote there closes the attribute and produces
// unparseable HTML. That defect is what reverted the previous attempt at this
// file.
func (p pythonWriter) writeAttribute(writer util.BufWriter, source []byte) {
	p.write(writer, source, true, true)
}

func (p pythonWriter) write(writer util.BufWriter, source []byte, unescape, quoteAttr bool) {
	for i := 0; i < len(source); i++ {
		c := source[i]

		if unescape && c == '\\' && i+1 < len(source) && escapableChars[source[i+1]] {
			// The escaped character still goes through HTML escaping: `\>` is
			// `&gt;` upstream, not a bare `>`.
			i++
			p.writeByte(writer, source[i], quoteAttr)
			continue
		}

		switch c {
		case '&':
			if unescape {
				if n := entityLength(source, i); n > 0 {
					_, _ = writer.Write(source[i : i+n])
					i += n - 1
					continue
				}
			}
			_, _ = writer.WriteString("&amp;")
		case 0:
			// goldmark's own rule, and python's: a NUL is not representable.
			_, _ = writer.WriteString("�")
		default:
			p.writeByte(writer, c, quoteAttr)
		}
	}
}

func (p pythonWriter) writeByte(writer util.BufWriter, c byte, quoteAttr bool) {
	switch {
	case c == '<':
		_, _ = writer.WriteString("&lt;")
	case c == '>':
		_, _ = writer.WriteString("&gt;")
	case c == '&':
		_, _ = writer.WriteString("&amp;")
	case c == '"' && quoteAttr:
		_, _ = writer.WriteString("&quot;")
	default:
		_ = writer.WriteByte(c)
	}
}

// imageRenderer overrides one node kind, and only because an image is the one
// place goldmark writes *text* into an *attribute*.
//
// Two further differences come free with owning the tag: python-markdown emits
// XHTML (`<img … />`) with its attributes in **alphabetical order** — `alt`,
// `src`, `title` — where goldmark emits `src` first and no trailing slash.
// Measured on `![alt "q"](img.png "title")`.
type imageRenderer struct {
	writer pythonWriter
}

// RegisterFuncs claims the image node, and only that one.
func (r imageRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindImage, r.renderImage)
}

func (r imageRenderer) renderImage(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Image)

	_, _ = w.WriteString(`<img alt="`)
	r.writer.writeAttribute(w, altText(n, source))
	_, _ = w.WriteString(`" src="`)
	// No URL escaping: python-markdown does none — see `link.go`.
	r.writer.writeAttribute(w, n.Destination)
	_ = w.WriteByte('"')
	if n.Title != nil {
		_, _ = w.WriteString(` title="`)
		r.writer.writeAttribute(w, n.Title)
		_ = w.WriteByte('"')
	}
	_, _ = w.WriteString(" />")

	return ast.WalkSkipChildren, nil
}

// altText is the image's own text content, flattened. goldmark's renderer walks
// the child nodes through the text writer; the alt attribute only ever holds
// their concatenated text, so gathering it is equivalent and keeps the escaping
// decision in one place.
func altText(n *ast.Image, source []byte) []byte {
	var out []byte
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		switch value := child.(type) {
		case *ast.Text:
			out = append(out, value.Segment.Value(source)...)
		case *ast.String:
			out = append(out, value.Value...)
		default:
			out = append(out, collectText(child, source)...)
		}
	}
	return out
}

func collectText(n ast.Node, source []byte) []byte {
	var out []byte
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		switch value := child.(type) {
		case *ast.Text:
			out = append(out, value.Segment.Value(source)...)
		case *ast.String:
			out = append(out, value.Value...)
		default:
			out = append(out, collectText(child, source)...)
		}
	}
	return out
}

// linkTitleSplitter reproduces python-markdown's title scanning, which is not
// CommonMark's.
//
// `LinkInlineProcessor.getLink` (`markdown/inlinepatterns.py:716-830`) treats
// **the first quote character anywhere inside the parentheses** as the start of a
// title — no whitespace required — and accepts it as a title if the matching
// quote is the last non-space character before the `)`. So
// `[t](https://x.com/?q="v")` is `href="https://x.com/?q="` with `title="v"`
// upstream, where CommonMark (and so goldmark) reads the whole thing as one URL.
// A query string with a quoted parameter is the shape that reaches this.
//
// It runs as an AST transformer rather than a parser replacement because
// goldmark's own link parsing is correct for every other form, including the
// ordinary space-separated title — which arrives here with Title already set and
// is left alone.
type linkTitleSplitter struct{}

// Transform rewrites every link and image whose destination carries a
// python-markdown-style title.
func (linkTitleSplitter) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.Link:
			if node.Title == nil {
				splitDestination(&node.Destination, &node.Title)
			}
		case *ast.Image:
			if node.Title == nil {
				splitDestination(&node.Destination, &node.Title)
			}
		}
		return ast.WalkContinue, nil
	})
}

// splitDestination applies the rule above to one destination, in place. It does
// nothing unless the destination really ends in a closed quote, so a URL merely
// containing a quote — `?q="unclosed` — keeps it.
func splitDestination(destination, title *[]byte) {
	dest := *destination

	// python tracks a **primary** quote — the first one it meets — and a
	// **secondary** one, so that `[t](u'"title")` still finds a title when the
	// primary never closes (`inlinepatterns.py:785-794`). Trying them in that
	// order is the whole of that fallback.
	primary := firstQuote(dest, 0)
	if primary < 0 {
		return
	}
	if closing := closingQuote(dest, primary); closing > 0 {
		*destination = trimSpaceBytes(dest[:primary])
		*title = dest[primary+1 : closing]
		return
	}

	secondary := firstQuote(dest, primary+1)
	if secondary < 0 || dest[secondary] == dest[primary] {
		return
	}
	if closing := closingQuote(dest, secondary); closing > 0 {
		*destination = trimSpaceBytes(dest[:secondary])
		*title = dest[secondary+1 : closing]
	}
}

// firstQuote is the index of the first `'` or `"` at or after `from`, or -1.
func firstQuote(dest []byte, from int) int {
	for i := from; i < len(dest); i++ {
		if dest[i] == '"' || dest[i] == '\'' {
			return i
		}
	}
	return -1
}

// closingQuote is the index of the quote that closes the one at `open`, but only
// when it is the **last non-space character** of the destination: python requires
// its `last` tracked character to be the quote when the link closes, so
// `[t](u"notitle)` is a URL containing a quote rather than a title.
func closingQuote(dest []byte, open int) int {
	closing := -1
	for i := open + 1; i < len(dest); i++ {
		if dest[i] == dest[open] {
			closing = i
		}
	}
	if closing < 0 {
		return -1
	}
	for i := closing + 1; i < len(dest); i++ {
		if dest[i] != ' ' {
			return -1
		}
	}
	return closing
}

// trimSpaceBytes is Python's `str.strip()` over a destination or a title —
// `href = self.unescape(href).strip()` (`inlinepatterns.py:828`) and the title's
// own `.strip()` at `:826`.
//
// **The predicate is `isPythonSpace`, not a literal space.** It used to be the
// latter, which was indistinguishable while `getLink` could only ever see one
// line: `![a](p.png\n)` now reaches it — `IMAGE_LINK_RE` is `re.DOTALL` and the
// destination is matched over the joined block — and kept the newline inside
// the `src` attribute.
func trimSpaceBytes(b []byte) []byte {
	return []byte(trimPythonSpace(string(b)))
}

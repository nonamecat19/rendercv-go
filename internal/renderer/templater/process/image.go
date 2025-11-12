package process

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// imageParser is `ImageInlineProcessor` (`markdown/inlinepatterns.py:852-873`),
// and it exists for one line of it:
//
//	el.set('alt', self.unescape(text))
//
// **The alt attribute is the label's raw source.** python-markdown never parses
// the label of an image as inline Markdown, so `![*em*](i.png)` is
// `alt="*em*"` upstream, where goldmark renders the label into child nodes and
// the attribute keeps only their flattened text — `alt="em"`, silently dropping
// the author's characters. Any decorated alt text in a CV reaches this.
//
// The label range is not recoverable from goldmark's AST once the label has been
// parsed, so the fix has to happen at parse time: this claims the plain inline
// form `![label](destination)` and builds the node itself, with the raw label as
// the node's only child and no inline parsing of it at all.
//
// It is deliberately narrow. Reference images (`![a][ref]`), an unbalanced
// label, a missing `(`, and anything continuing onto the next line are all
// declined, and goldmark's own link parser handles them exactly as before.
type imageParser struct{}

// Trigger is `!`, which goldmark's link parser also uses.
func (imageParser) Trigger() []byte { return []byte{'!'} }

// Parse claims one `![label](destination)` on the current line, or returns nil.
func (imageParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if len(line) < 2 || line[1] != '[' {
		return nil
	}

	label, after := matchBracketed(line, 2, '[', ']')
	if after < 0 || after >= len(line) || line[after] != '(' {
		return nil
	}
	inner, end := matchBracketed(line, after+1, '(', ')')
	if end < 0 {
		return nil
	}

	image := ast.NewImage(&ast.Link{})
	image.Destination = inner
	splitDestination(&image.Destination, &image.Title)
	image.Destination = trimSpaceBytes(image.Destination)
	// A String child rather than a Text one: the label is carried as bytes, not
	// as a range of the source, because `matchBracketed` returns a slice of the
	// line. `altText` collects both, and the attribute writer applies
	// python's backslash unescaping — its `self.unescape(text)`.
	image.AppendChild(image, ast.NewString(resolveCodeSpans(label)))

	block.Advance(end)

	return image
}

// resolveCodeSpans replaces every code span in an image label with its content.
//
// "Raw source" is not quite the whole rule: python-markdown's inline patterns run
// in priority order and only `backtick` (190) and `escape` (180) outrank
// `image_link` (150) (`inlinepatterns.py:72-76`), so those two have already been
// resolved into the label the image processor reads, and everything below —
// emphasis at 60 and 50, inline HTML at 90, the autolinks — has not. Backslash
// escapes are resolved by the attribute writer; this is the other one.
//
// `alt` then holds the code span's *text*, with no `<code>` around it, because
// `unescape` flattens a stashed element with `”.join(value.itertext())`
// (`inlinepatterns.py:264-281`).
func resolveCodeSpans(label []byte) []byte {
	var out []byte
	for i := 0; i < len(label); i++ {
		if label[i] != '`' {
			out = append(out, label[i])
			continue
		}
		open := i
		for i < len(label) && label[i] == '`' {
			i++
		}
		content, after := matchBackticks(label, i, i-open)
		if after < 0 {
			out = append(out, label[open:i]...)
			i--
			continue
		}
		out = append(out, stripSpace(content)...)
		i = after - 1
	}
	return out
}

// matchBackticks finds the run of exactly `width` backticks that closes the one
// just read, returning the content between and the index just past it.
func matchBackticks(label []byte, start, width int) ([]byte, int) {
	for i := start; i < len(label); i++ {
		if label[i] != '`' {
			continue
		}
		run := i
		for i < len(label) && label[i] == '`' {
			i++
		}
		if i-run == width {
			return label[start:run], i
		}
	}
	return nil, -1
}

// matchBracketed returns the content from `start` up to the delimiter closing
// the one already opened, together with the index just past it, or -1 when the
// line ends first.
//
// It counts nesting, which is python-markdown's rule in both places: `getText`
// balances `[` against `]` (`inlinepatterns.py:832-850`) and `getLink` balances
// `(` against `)` (`:716-830`), so `![a[b]c](d(e)f)` keeps both inner pairs.
func matchBracketed(line []byte, start int, open, close byte) ([]byte, int) {
	depth := 1
	for i := start; i < len(line); i++ {
		switch line[i] {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return line[start:i], i + 1
			}
		}
	}
	return nil, -1
}

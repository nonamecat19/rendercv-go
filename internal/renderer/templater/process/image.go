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
// label and a missing `(` are all declined, and goldmark's own link parser
// handles them exactly as before.
type imageParser struct{}

// Trigger is `!`, which goldmark's link parser also uses.
func (imageParser) Trigger() []byte { return []byte{'!'} }

// Parse claims one `![label](destination)` at the reader's position, or returns
// nil.
//
// **The match runs to the end of the block, not to the end of the line.**
// `IMAGE_LINK_RE` is applied to the joined block text and `re.DOTALL` is on
// (`inlinepatterns.py:143`, `:399`), so a label or a destination broken across a
// soft line break is still one image upstream — where this declined and left
// goldmark's own parser to build one whose `alt` had lost the break entirely:
// `![alt x\n y](p.png)` was `alt="alt xy"`. A wrapped alt text is what any CV
// long enough to wrap produces.
func (imageParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	_, segment := block.PeekLine()
	node, ok := buildImage(block, buildWindow(block, segment.Start, -1))
	if !ok {
		return nil
	}
	return node
}

// buildImage is the body of `imageParser.Parse`, taking the window to match
// against explicitly so that an emphasis body can hand it one bounded by the
// body's end rather than by the block's (`parseEmphasisBody`).
//
// The window is what makes a multi-line match expressible: it carries the
// source offset of every byte, so a label spanning a soft break is contiguous
// text here and the reader is still seeked to a real position afterwards.
func buildImage(block text.Reader, w window) (ast.Node, bool) {
	line := w.text
	if len(line) < 2 || line[1] != '[' {
		return nil, false
	}

	// Scanned over the escape-masked line for the reason `getLink` documents:
	// `escape` (180) outranks `image_link` (150) too.
	scan := []byte(maskAbove(string(line), prioImage))
	_, after := matchBracketed(scan, 2, '[', ']')
	if after < 0 || after >= len(scan) || scan[after] != '(' {
		return nil, false
	}
	label := line[2 : after-1]
	href, title, hasTitle, end, ok := getLink(scan, line, after)
	if !ok {
		return nil, false
	}

	image := ast.NewImage(&ast.Link{})
	image.Destination = href
	if hasTitle {
		image.Title = title
	}
	// A String child rather than a Text one: the label is carried as bytes, not
	// as a range of the source, because `matchBracketed` returns a slice of the
	// line. `altText` collects both, and the attribute writer applies
	// python's backslash unescaping — its `self.unescape(text)`.
	image.AppendChild(image, ast.NewString(resolveCodeSpans(label)))

	advanceTo(block, w.source(end))

	return image, true
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
//
// This is `BACKTICK_RE`'s `(`+)(.+?)(?<!`)\2(?!`)` (`inlinepatterns.py:104`)
// written out, because the `\2` backreference is the one thing RE2 cannot
// express. It is generic over bytes and strings because both paths need it: the
// HTML path reads goldmark's `[]byte` line, and the Typst inline pass
// (`inline.go`) reads a `string`.
func matchBackticks[T ~[]byte | ~string](label T, start, width int) (T, int) {
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
	var none T
	return none, -1
}

// indexByteFrom is the index of the first `c` at or after `from`, or -1.
func indexByteFrom(b []byte, c byte, from int) int {
	for i := from; i < len(b); i++ {
		if b[i] == c {
			return i
		}
	}
	return -1
}

// matchBracketed returns the content from `start` up to the delimiter closing
// the one already opened, together with the index just past it, or -1 when the
// line ends first.
//
// It counts nesting, which is python-markdown's rule in both places: `getText`
// balances `[` against `]` (`inlinepatterns.py:832-850`) and `getLink` balances
// `(` against `)` (`:716-830`), so `![a[b]c](d(e)f)` keeps both inner pairs.
func matchBracketed[T ~[]byte | ~string](line T, start int, open, close byte) (T, int) {
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
	var none T
	return none, -1
}

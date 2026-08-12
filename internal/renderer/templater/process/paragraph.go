package process

import (
	"regexp"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// pythonParagraphParser is goldmark's paragraph parser with the two string
// operations `ParagraphProcessor.run` performs (`blockprocessors.py:606-641`).
//
// Both are `str` methods over Python's whitespace, and both are invisible until
// the text starts with a character goldmark does not call a space — the same
// 25-character gap `pythonCodeBlockParser` closes for indented code
// (`codeblock.go`), reached here through a much wider door, because every
// paragraph in the document goes through this processor.
//
//   - **A block that is entirely whitespace is thrown away** (`:614`, the
//     `if block.strip():` with no `else`). `"\v"` alone is an empty document
//     upstream and was `<p>\v</p>` here.
//   - **The block is `lstrip`ped**, once, as one string (`:641` for a paragraph
//     and `:637` for the first block of a tight list item). Because the block
//     still has its newlines at that point, the strip runs *across* them: the
//     whole of `"\v\n    a"` reduces to `a`, first line and all. Interior lines
//     keep their indentation — `"a\n    b"` is `<p>a\n    b</p>` in both
//     libraries — so this is the leading edge of the block, not of each line.
//
// It is the block parser's `Close` rather than an AST transformer because the
// unit of `lstrip` is the block's *text*: acting on the lines before inline
// parsing keeps one string operation as one string operation, where a transform
// over the parsed tree would have to find and re-cut the first text node. Close
// is also where goldmark itself removes a paragraph that turned out to be empty
// (`parser/paragraph.go:44-62`), which is the same decision `:614` makes.
//
// **The paragraph must still open on the whitespace line.** Declining it there
// would hand the next line to another parser, and `"\v\n    a"` would become an
// indented code block; upstream never gets that far, because `parseChunk` gave
// the whole chunk to a processor chain that `CodeBlockProcessor.test` had
// already turned down (`blockprocessors.py:255`, `block.startswith('    ')`).
type pythonParagraphParser struct {
	parser.BlockParser
}

// Close delegates, then applies `:614` and the `lstrip`.
func (p pythonParagraphParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	p.BlockParser.Close(node, reader, pc)
	if node.Parent() == nil {
		// goldmark found it empty and removed it first.
		return
	}

	source := reader.Source()
	lines := node.Lines()
	first := 0
	for first < lines.Len() {
		line := lines.At(first)
		if !isPythonBlank(line.Value(source)) {
			break
		}
		first++
	}
	if first == lines.Len() {
		if line, _ := reader.PeekLine(); setextBar.Match(line) {
			// **Upstream never reaches `:614` here.** `SetextHeaderProcessor`
			// is registered above `ParagraphProcessor` and claims the whole
			// chunk (`blockprocessors.py:493-510`), so `"\v\n==="` is
			// `<h1></h1>` and not a discarded block. goldmark decides the same
			// thing one line later, at the bar, and the paragraph it needs is
			// this one — removing it here left `<p>===</p>`.
			return
		}
		node.Parent().RemoveChild(node.Parent(), node)
		return
	}

	if first > 0 {
		// The lines are dropped, not emptied: a zero-length line panics
		// goldmark's inline parser, which indexes the last byte of every line
		// it is given (`parser/parser.go:1174`).
		//
		// One shape pays for that. `continuationIndent` reads the block's own
		// share of the indentation off `Lines().At(0).Start` (`softbreak.go`),
		// so a paragraph that lost its first line takes the second line's
		// indentation for its own and a *third* line loses that much:
		// `"\v\n    a\n    b"` is `<p>a\n    b</p>` upstream and `<p>a\nb</p>`
		// here. It differed before this rule too, by the `\v`, and no other
		// measured shape moves.
		lines.SetSliced(first, lines.Len())
	}

	// goldmark's own Close has already trimmed the leading spaces, so what is
	// left is the run this port's whitespace set is wider by.
	line := lines.At(0)
	if width := pythonSpacePrefix(line.Value(source)); width > 0 {
		lines.Set(0, line.WithStart(line.Start+width))
	}
}

// pythonSpacePrefix is the byte length of the leading whitespace `str.lstrip()`
// would remove. The characters are counted as runes: `\xa0` and U+2028 are two
// and three bytes of UTF-8, and slicing a segment at a byte offset inside one
// would leave the paragraph starting mid-character.
func pythonSpacePrefix(line []byte) int {
	width := 0
	for width < len(line) {
		r, size := utf8.DecodeRune(line[width:])
		if !isPythonSpace(r) {
			break
		}
		width += size
	}
	return width
}

// setextBar is the second line of a setext heading as goldmark reads it: up to
// three spaces, then a run of `=` or a run of `-`, then optional spaces
// (`parser/setext_headings.go:15-35`, `matchesSetextHeadingBar`).
var setextBar = regexp.MustCompile(`^ {0,3}(?:=+|-+)[ ]*\n?$`)

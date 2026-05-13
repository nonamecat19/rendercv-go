package process

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// codeBlockRenderer is `CodeBlockProcessor` (`markdown/blockprocessors.py:251-281`)
// together with the `<pre>` clause of `PrettifyTreeprocessor.run`
// (`markdown/treeprocessors.py:445-451`) — the two places python-markdown
// removes trailing whitespace from an indented code block.
//
// **The rstrip is per chunk, not per line and not per block.** python-markdown
// never sees a code block as one thing: `parseChunk` splits the document on the
// literal `'\n\n'` (`markdown/blockparser.py:136`) and hands the pieces to the
// processors one at a time, so an indented run interrupted by a blank line
// arrives as several blocks and `CodeBlockProcessor.run` appends each to the
// `<code>` element the previous one created (`:261-270`). Every one of those
// appends escapes `block.rstrip()` (`:269` and `:276`), so the last line of each
// chunk loses its trailing whitespace and the lines above it keep theirs:
//
//	"    a  \n    b  "      ->  "a  \nb\n"     one chunk, one rstrip
//	"    a  \n\n    b  "    ->  "a\n\nb\n"     two chunks, two rstrips
//
// goldmark hands over a single `ast.CodeBlock` spanning the blank line, which is
// why this cannot be the one-line fix the Typst path took (`markdown.go`, where a
// code block is a single line by construction). The chunk boundaries are
// recovered here from the block's own lines.
//
// The final `rstrip` is the tree processor's, and it is what makes trailing blank
// lines inside the block disappear rather than accumulate — `EmptyBlockProcessor`
// appends `'\n\n'` for every empty block, including the two `NormalizeWhitespace`
// puts at the end of every document (`markdown/preprocessors.py:72`).
type codeBlockRenderer struct {
	writer pythonWriter
}

// RegisterFuncs claims the indented code block, and only that one. There is no
// fenced code block to claim: `markdown_to_html` enables no extensions, so
// `pythonBlockParsers` drops goldmark's fenced parser entirely (`html.go`).
func (r codeBlockRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindCodeBlock, r.renderCodeBlock)
}

func (r codeBlockRenderer) renderCodeBlock(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("<pre><code>")
		// `RawWrite` is the code cell of `htmlescape.go`'s three-context table,
		// which is `util.code_escape` — `&`, `<` and `>`, nothing else.
		r.writer.RawWrite(w, []byte(codeBlockText(node, source)))
	} else {
		_, _ = w.WriteString("</code></pre>\n")
	}
	return ast.WalkContinue, nil
}

// pythonCodeBlockParser is goldmark's indented code block parser with
// python-markdown's rule for a line that is whitespace to Python and is not
// whitespace to goldmark.
//
// The two libraries disagree about the character set. `detab` keeps a code block
// open across any line `str.strip()` empties (`blockprocessors.py:96-99`, the
// `elif not line.strip()` arm) and Python's whitespace includes `\v`, `\f`,
// `\x1c`-`\x1f`, `\x85` and `\xa0`; goldmark's `util.IsSpace` is the six ASCII
// characters minus those two (`util/util.go:806`, `spaceTable`), so its
// `Continue` sees an unindented non-blank line and closes the block
// (`parser/code_block.go:47-53`). `"    a\n\v\n    b"` is one `<pre>` upstream
// and was a `<pre>` plus a `<p>` here.
//
// **The line is emptied, not treated as blank.** A blank line ends a chunk and
// the next chunk has to start with four spaces of its own to be code at all
// (`blockparser.py:136`), which is why `"    a\n\n\v\n    b"` really is a code
// block followed by a paragraph upstream, where a `\v` line inside a chunk is
// only emptied. The two differ in the output — one `rstrip` against two — so
// the parser records which lines it emptied and `codeBlockText` reads it back.
//
// The fix is a parser wrapper rather than a rewrite of the input, for
// `listitem.go`'s reason: the same line means different things by context. A `\v`
// line after a blank one must still close the block, and only the parser knows
// whether the previous line was blank. Rewriting it to an empty line in the
// source would close nothing, and rewriting it to four spaces would erase the
// difference between a chunk boundary and an emptied line that this type exists
// to keep.
type pythonCodeBlockParser struct {
	parser.BlockParser
}

// Continue empties a Python-whitespace line instead of closing the block, and
// otherwise delegates.
func (p pythonCodeBlockParser) Continue(
	node ast.Node, reader text.Reader, pc parser.Context,
) parser.State {
	line, segment := reader.PeekLine()
	if !detabEmpties(node, line, reader) {
		return p.BlockParser.Continue(node, reader, pc)
	}
	markDetabEmptied(node, node.Lines().Len())
	// The same append goldmark makes for a blank line, including leaving the
	// line for the caller's `AdvanceLine` (`parser/code_block.go:47-50`).
	node.Lines().Append(segment.TrimLeftSpaceWidth(4, reader.Source()))
	return parser.Continue | parser.NoChildren
}

// detabEmpties reports whether `detab` would empty this line and carry on where
// goldmark would close the block.
//
// Three conditions, all of them upstream's. The line is whitespace to Python but
// not to goldmark, or goldmark's own blank arm already handles it. It is
// indented by less than a tab length, or `detab`'s first arm dedents it and
// keeps its text — `"    \v"` is a `\v` in the block, not an empty line. And the
// previous line is not blank, because a blank line ends the chunk and
// `CodeBlockProcessor.test` (`:255`) then requires the next one to start with
// four spaces.
func detabEmpties(node ast.Node, line []byte, reader text.Reader) bool {
	if util.IsBlank(line) || !isPythonBlank(line) {
		return false
	}
	if pos, _ := util.IndentPosition(line, reader.LineOffset(), pythonMarkdownTabLength); pos >= 0 {
		return false
	}
	return !lastLineIsBlank(node, reader.Source())
}

// isPythonBlank is `not line.strip()` for a line that still has its newline.
func isPythonBlank(line []byte) bool {
	return trimPythonSpace(string(line)) == ""
}

// lastLineIsBlank reports whether the line the block last took was one
// `NormalizeWhitespace` empties, which is where a chunk ends.
func lastLineIsBlank(node ast.Node, source []byte) bool {
	lines := node.Lines()
	if lines.Len() == 0 {
		return false
	}
	last := lines.At(lines.Len() - 1)
	return strings.Trim(string(last.Value(source)), " \n") == ""
}

// detabEmptiedAttribute is where `pythonCodeBlockParser` leaves the line indices
// for `codeBlockText`. goldmark's own renderers read node attributes, and the
// alternative — recovering the original line from its offset in the source — is
// not available inside a list item, where the segment has already lost the
// item's indentation.
const detabEmptiedAttribute = "rendercvDetabEmptied"

// markDetabEmptied records that the line about to be appended at index is one
// `detab` emptied.
func markDetabEmptied(node ast.Node, index int) {
	emptied := detabEmptiedLines(node)
	if emptied == nil {
		emptied = map[int]bool{}
		node.SetAttributeString(detabEmptiedAttribute, emptied)
	}
	emptied[index] = true
}

// detabEmptiedLines is the set markDetabEmptied built, or nil when the block has
// no such line — which is every code block in an ordinary CV.
func detabEmptiedLines(node ast.Node) map[int]bool {
	value, ok := node.AttributeString(detabEmptiedAttribute)
	if !ok {
		return nil
	}
	emptied, _ := value.(map[int]bool)
	return emptied
}

// codeBlockText is the text of the `<code>` element: the block's own lines, cut
// into python-markdown's blank-line-delimited chunks, each `rstrip`'d, and the
// whole `rstrip`'d once more.
//
// **Emptied and boundary are two different things**, which is why the lines are
// not simply blanked and split on `"\n\n"`. `detab` empties a line
// (`blockprocessors.py:98-99`) *after* `parseChunk` has already cut the document
// (`blockparser.py:136`), so a line `detab` empties is inside a chunk, not
// between two — `"    a  \n\v\n    b  "` is one chunk and therefore one
// `rstrip`, and upstream keeps the trailing spaces of `a  `.
func codeBlockText(node ast.Node, source []byte) string {
	lines := node.Lines()
	emptied := detabEmptiedLines(node)
	kept := make([]string, lines.Len())
	boundary := make([]bool, lines.Len())
	for i := range kept {
		segment := lines.At(i)
		line := strings.TrimSuffix(string(segment.Value(source)), "\n")
		switch {
		case i == 0 && isRawTailLine(line):
			// `splitRawBlockTails` wrote this line: a whitespace-only tail that
			// upstream leaves in the document as an indented code block, plus
			// `rawTailMarker` so that goldmark does not read it as blank
			// (`htmlextract.go`). Only the marker comes off — the columns past
			// the fourth are the block's own text upstream, which is why
			// `<div>a</div>` followed by five spaces and then `    x` is
			// `" \nx\n"` and not `"\nx\n"`.
			//
			// **Only at the head of a block**, because that is the only place
			// the marker is ever written: a tail follows a raw block's closing
			// tag, so its line always opens the code block it makes.
			kept[i] = strings.TrimSuffix(line, rawTailMarker)
		case emptied[i]:
			// `detab` emptied it (`pythonCodeBlockParser`); the chunk
			// runs straight through it.
			kept[i] = ""
		case strings.Trim(line, " ") == "":
			// A line of nothing but spaces **is** a blank line to
			// python-markdown: `NormalizeWhitespace` empties it before the
			// parser ever runs, with `re.sub(r'(?<=\n) +\n', '\n', source)`
			// (`preprocessors.py:74`). So it both ends a chunk and contributes
			// no spaces of its own — measured on `"    a  \n       \n    b  "`,
			// which is `a\n\nb\n` upstream.
			kept[i] = ""
			boundary[i] = true
		default:
			kept[i] = line
		}
	}

	chunks := chunkOnBlankLines(kept, boundary)
	for i, chunk := range chunks {
		chunks[i] = trimPythonSpaceRight(chunk)
	}
	return trimPythonSpaceRight(strings.Join(chunks, "\n\n")) + "\n"
}

// isRawTailLine reports whether a code block's first line is the one
// `splitRawBlockTails` wrote for a whitespace-only tail: spaces and then the
// marker, nothing else.
func isRawTailLine(line string) bool {
	rest, ok := strings.CutSuffix(line, rawTailMarker)
	return ok && strings.Trim(rest, " ") == ""
}

// chunkOnBlankLines is `'\n'.join(lines).split('\n\n')` over lines whose
// blankness is given separately, so that a line `detab` emptied does not cut a
// chunk the way an originally blank one does.
//
// The separator is the newline **after** a blank line as well as the one before
// it, so a trailing blank line cuts nothing and a run of three blank lines
// leaves one of them at the head of the next chunk — `"a\n\n\nb"` is `["a",
// "\nb"]` in Python, not `["a", "b"]`.
func chunkOnBlankLines(lines []string, boundary []bool) []string {
	chunks := []string{}
	var current []string
	for i := 0; i < len(lines); i++ {
		current = append(current, lines[i])
		if i+1 <= len(lines)-2 && boundary[i+1] {
			chunks = append(chunks, strings.Join(current, "\n"))
			current = nil
			i++
		}
	}
	return append(chunks, strings.Join(current, "\n"))
}

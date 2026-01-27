package process

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
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

// codeBlockText is the text of the `<code>` element: the block's own lines, cut
// into python-markdown's blank-line-delimited chunks, each `rstrip`'d, and the
// whole `rstrip`'d once more.
func codeBlockText(node ast.Node, source []byte) string {
	lines := node.Lines()
	kept := make([]string, 0, lines.Len())
	for i := 0; i < lines.Len(); i++ {
		segment := lines.At(i)
		line := strings.TrimSuffix(string(segment.Value(source)), "\n")
		// A line of nothing but spaces **is** a blank line to python-markdown:
		// `NormalizeWhitespace` empties it before the parser ever runs, with
		// `re.sub(r'(?<=\n) +\n', '\n', source)` (`preprocessors.py:74`). So it
		// both ends a chunk and contributes no spaces of its own — measured on
		// `"    a  \n       \n    b  "`, which is `a\n\nb\n` upstream.
		if strings.Trim(line, " ") == "" {
			line = ""
		}
		kept = append(kept, line)
	}

	chunks := strings.Split(strings.Join(kept, "\n"), "\n\n")
	for i, chunk := range chunks {
		chunks[i] = trimPythonSpaceRight(chunk)
	}
	return trimPythonSpaceRight(strings.Join(chunks, "\n\n")) + "\n"
}

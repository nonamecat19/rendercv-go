package process

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// blockRenderer restores the trailing whitespace at the end of a block.
//
// python-markdown strips whitespace at the *left* of a line and never at the
// right of the last one, so it survives into the output — invisibly, but byte
// for byte:
//
//	"a  "        →  <p>a  </p>
//	"- x  "      →  <ul>\n<li>x  </li>\n</ul>
//	"*a* "       →  <p><em>a</em> </p>
//
// goldmark trims it while building the inline text, which made **any** CV field
// with a stray space at the end produce a differing `.html` — the single easiest
// divergence in this file to hit by accident and the hardest to see.
//
// It hangs off the block rather than off the text because the whitespace may
// follow a node that is not text at all (the `*a* ` row above), so there is no
// last text node to append it to. The block's own last line still has it.
type blockRenderer struct {
	writer pythonWriter
	inner  map[ast.NodeKind]renderer.NodeRendererFunc
}

// RegisterFuncs claims the two blocks whose content is a run of inlines with no
// wrapper of its own: a paragraph and a list item's implicit text block.
func (r blockRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindParagraph, r.render)
	reg.Register(ast.KindTextBlock, r.render)
}

func (r blockRenderer) render(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	if !entering {
		r.writer.Write(w, trailingSpace(source, node))
	}
	// A paragraph that is nothing but one block-level raw tag loses its
	// wrapper, and keeps the newline the wrapper would have carried —
	// `postprocessors.go`.
	if unwrapsParagraph(source, node) {
		if !entering {
			_ = w.WriteByte('\n')
		}
		return ast.WalkContinue, nil
	}
	return r.inner[node.Kind()](w, source, node, entering)
}

// trailingSpace is the run of spaces and tabs ending the block's last line.
//
// It reads *forward* from where the block's last line stops, because goldmark
// has already trimmed that line's segment — the whitespace is still in the
// source, just no longer inside any node.
func trailingSpace(source []byte, node ast.Node) []byte {
	lines := node.Lines()
	if lines.Len() == 0 {
		return nil
	}
	start := lines.At(lines.Len() - 1).Stop
	if start > len(source) {
		return nil
	}
	end := start
	for end < len(source) && (source[end] == ' ' || source[end] == '\t') {
		end++
	}
	return source[start:end]
}

package process

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// textRenderer restores the indentation goldmark drops from a continuation line.
//
// python-markdown's block processors only ever strip a *leading* `tab_length` of
// indentation; the whitespace at the head of a line that continues a paragraph
// is ordinary text and survives into the output verbatim:
//
//	a\n  b            →  <p>a\n  b</p>
//	- item\n  lazy    →  <ul>\n<li>item\n  lazy</li>\n</ul>
//
// goldmark trims it as block padding, so both came out flush left. Any wrapped
// line in a multi-line CV field reaches this, and the diff is invisible in a
// browser — which is exactly why it needs a test rather than an eyeball.
//
// It delegates the whole of the text rendering to goldmark and only appends the
// dropped spaces, so the escaping, the hard-break `<br />` and the East-Asian
// line-break rules all stay where they are.
type textRenderer struct {
	inner renderer.NodeRendererFunc
}

// RegisterFuncs claims the text node, and only that one.
func (r textRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindText, r.renderText)
}

func (r textRenderer) renderText(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	status, err := r.inner(w, source, node, entering)
	if err != nil || !entering {
		return status, err
	}

	n := node.(*ast.Text)
	if n.IsRaw() || (!n.SoftLineBreak() && !n.HardLineBreak()) {
		return status, err
	}
	_, _ = w.Write(continuationIndent(source, node))

	return status, err
}

// continuationIndent is the whitespace the next line keeps: what stands in front
// of its content, **less the prefix its container owns**, which python-markdown
// strips and this port must strip too.
//
//	> q\n>   r          →  <p>q\n  r</p>   one space belongs to the quote
//	- a\n\n    x\n    y  →  <p>x\ny</p>     all four belong to the item
//	1. a\n   b          →  <li>a\n   b</li> a lazy line owes the marker nothing
//
// The container's share is read off the first line of the block this text sits
// in — except when that line carries a list marker, because then the block is
// the item's own first paragraph and its continuation lines are lazy ones, which
// `ListIndentProcessor` never reaches.
func continuationIndent(source []byte, node ast.Node) []byte {
	next := indentBefore(source, nextContentStart(source, node))
	block := node
	for block != nil && block.Type() != ast.TypeBlock {
		block = block.Parent()
	}
	if block == nil || block.Lines().Len() == 0 {
		return next
	}
	start := block.Lines().At(0).Start
	if listMarker.MatchString(" " + string(lineAt(source, start))) {
		return next
	}
	own := indentBefore(source, start)
	if len(next) <= len(own) {
		return nil
	}
	return next[len(own):]
}

// nextContentStart is where the text after this line break resumes, taken from
// the AST rather than from the source: a blockquote's `>` and a list's marker are
// in the source but not in the content, and only goldmark knows where it put the
// boundary.
func nextContentStart(source []byte, node ast.Node) int {
	for n := node; n != nil && n.Type() == ast.TypeInline; n = n.Parent() {
		for sibling := n.NextSibling(); sibling != nil; sibling = sibling.NextSibling() {
			if start := firstSegmentStart(sibling); start >= 0 {
				return tokenStart(source, start)
			}
		}
	}
	return len(source)
}

// tokenStart backs up over the delimiters an inline node opens with — the `*` of
// an emphasis, the backtick of a code span — which stand between the line's
// indentation and the first text goldmark records a position for.
func tokenStart(source []byte, at int) int {
	for at > 0 && source[at-1] != '\n' && !isSpaceByte(source[at-1]) {
		at--
	}
	return at
}

// firstSegmentStart is the source offset of the first text in a subtree, or -1.
func firstSegmentStart(n ast.Node) int {
	if text, ok := n.(*ast.Text); ok {
		return text.Segment.Start
	}
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if start := firstSegmentStart(child); start >= 0 {
			return start
		}
	}
	return -1
}

// lineAt is the line containing `at`, from its first character.
func lineAt(source []byte, at int) []byte {
	start := at
	for start > 0 && source[start-1] != '\n' {
		start--
	}
	end := at
	for end < len(source) && source[end] != '\n' {
		end++
	}
	return source[start:end]
}

// indentBefore is the run of spaces and tabs immediately preceding `at`.
func indentBefore(source []byte, at int) []byte {
	if at > len(source) {
		at = len(source)
	}
	start := at
	for start > 0 && (source[start-1] == ' ' || source[start-1] == '\t') {
		start--
	}
	return source[start:at]
}

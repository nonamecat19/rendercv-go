package process

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// linkRenderer exists because **python-markdown does not URL-escape a
// destination at all.**
//
// `LinkInlineProcessor.handleMatch` puts the href on the element as it was
// written (`inlinepatterns.py:706-711`) and the serializer's only job on an
// attribute is HTML escaping (`markdown/serializers.py`). goldmark runs
// `util.URLEscape` first, so a perfectly ordinary CV link came out percent-
// encoded where upstream left it alone:
//
//	[t](héllo.png)   →  href="héllo.png"     not  "h%C3%A9llo.png"
//	[t](<a b>)       →  href="a b"           not  "a%20b"
//	[t](a<b)         →  href="a&lt;b"        not  "a%3Cb"
//
// A non-ASCII filename and a path with a space are both things a user writes
// without thinking about it, so this is not an exotic shape.
type linkRenderer struct {
	writer pythonWriter
}

// RegisterFuncs claims the link node, and only that one.
func (r linkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindLink, r.renderLink)
}

func (r linkRenderer) renderLink(
	w util.BufWriter, _ []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</a>")
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Link)

	_, _ = w.WriteString(`<a href="`)
	r.writer.writeAttribute(w, n.Destination)
	_ = w.WriteByte('"')
	if n.Title != nil {
		_, _ = w.WriteString(` title="`)
		r.writer.writeAttribute(w, n.Title)
		_ = w.WriteByte('"')
	}
	_ = w.WriteByte('>')

	return ast.WalkContinue, nil
}

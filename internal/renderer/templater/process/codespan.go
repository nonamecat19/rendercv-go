package process

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// codeSpanRenderer is python-markdown's `BacktickInlineProcessor`
// (`markdown/inlinepatterns.py:444-456`), whose whole content rule is
// `m.group(3).strip()`: **a full whitespace strip at both ends, and the interior
// left exactly as written.**
//
// goldmark's own code-span renderer instead rewrites a trailing newline into a
// space (`renderer/html/html.go:548-551`) and leans on CommonMark's strip-one-
// space rule, so a code span spanning two lines came out as `a b` where upstream
// gives `a\nb`. That shape is reachable from any multi-line CV field: with
// `fenced_code` absent (see `withoutFencedCode`), a ``` fence's backticks open a
// code span that runs across the lines between them.
type codeSpanRenderer struct {
	writer pythonWriter
}

// RegisterFuncs claims the code-span node, and only that one.
func (r codeSpanRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindCodeSpan, r.renderCodeSpan)
}

func (r codeSpanRenderer) renderCodeSpan(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</code>")
		return ast.WalkContinue, nil
	}

	_, _ = w.WriteString("<code>")
	var value []byte
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		text, ok := child.(*ast.Text)
		if !ok {
			continue
		}
		value = append(value, text.Segment.Value(source)...)
	}
	// RawWrite is the code-span escaping cell — see `htmlescape.go`.
	r.writer.RawWrite(w, stripSpace(value))

	return ast.WalkSkipChildren, nil
}

// stripSpace is Python's `str.strip()` with no argument. It is generic for the
// same reason `matchBackticks` is: the HTML path's `[]byte`, the image label's
// `[]byte` and the Typst inline pass's `string` all need
// `BacktickInlineProcessor`'s `m.group(3).strip()`
// (`inlinepatterns.py:444-456`).
//
// **The predicate is Python's whole 29-character set, not the ASCII six.** It
// used to be the latter, so a code span padded with a non-breaking space — which
// a double-quoted YAML scalar carries into a highlight verbatim — kept its
// padding in the .typ and in the .html alike, where upstream drops it. The set
// is `isPythonSpace` (`markdown.go:199`), the same one the code-block `rstrip`
// next door reads; the only difference here is that `strip()` trims both ends.
func stripSpace[T ~[]byte | ~string](b T) T {
	return T(trimPythonSpace(string(b)))
}

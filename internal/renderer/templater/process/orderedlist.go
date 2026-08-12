package process

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// lazyOrderedList is python-markdown's `LAZY_OL`
// (`markdown/blockprocessors.py:337-338`), which is `True` on the bare
// `markdown.Markdown` that `markdown_to_html` builds and cannot be turned off
// from `markdown.markdown(string)`.
//
// `OListProcessor.get_items` does read the first item's integer into
// `STARTSWITH` (`:425-429`), but `run` only writes it out as a `start`
// attribute when the flag is off (`:401-403`), so upstream's `<ol>` never
// carries one: `3. a` is `<ol>\n<li>a</li>\n</ol>`, the same as `1. a`.
//
// CommonMark keeps the index, so goldmark renders `<ol start="3">`
// (`renderer/html/html.go:417-419`, on `List.Start`). Dropping the index at
// parse time rather than filtering the attribute out at render time is where
// upstream drops it too, and it leaves goldmark's own list renderer in place.
type lazyOrderedList struct{}

// Transform discards the start index of every ordered list in the document.
func (lazyOrderedList) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if list, ok := node.(*ast.List); ok && list.IsOrdered() {
			list.Start = 1
		}
		return ast.WalkContinue, nil
	})
}

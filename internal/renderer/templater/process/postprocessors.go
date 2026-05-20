package process

import (
	"regexp"

	"github.com/yuin/goldmark/ast"
)

// stashBlockLevelTag is `RawHtmlPostprocessor.BLOCK_LEVEL_REGEX`
// (`markdown/postprocessors.py:71`), which reads the tag name off the **stashed
// source**, not off a parsed node: everything up to the first space or `>`.
var stashBlockLevelTag = regexp.MustCompile(`^</?([^ >]+)`)

// unwrapsParagraph reports whether a paragraph is one `RawHtmlPostprocessor`
// strips the `<p>` off (`markdown/postprocessors.py:73-93`).
//
// # Why a paragraph can lose its wrapper
//
// A tag that is not at the start of its line never opens a raw block
// (`htmlblock.go`'s `atLineStart`); the **inline** `html` pattern stashes it
// instead (`inlinepatterns.py:90`) and the block pass then wraps the placeholder
// in a paragraph, because a placeholder is ordinary text to it. The postprocessor
// substitutes the source back afterwards, and its pattern is
// `<p>PLACEHOLDER</p>|PLACEHOLDER` (`:90`): when the **whole paragraph** is one
// placeholder and the stashed source is block-level, the `<p>` wrapper is dropped
// (`:84-86`) and the raw HTML stands alone.
//
//	</div>        →  </div>          not <p></div></p>
//	</div>\ntext  →  <p></div>\ntext</p>
//
// The first is reachable because a closing tag never opens a raw block —
// `handle_endtag` has no branch that sets `inraw` (`htmlparser.py:230-255`),
// unlike `handle_starttag` (`:215`) — so a lone `</div>` arrives here as a
// paragraph holding a single stash entry.
//
// The name test is `is_block_level` again (`:102`, the same
// `BLOCK_LEVEL_ELEMENTS` as `htmlblock.go`), with comments, processing
// instructions and the rest passing on their leading punctuation (`:99-101`).
func unwrapsParagraph(source []byte, node ast.Node) bool {
	if node.Kind() != ast.KindParagraph {
		return false
	}
	child := node.FirstChild()
	if child == nil || child != node.LastChild() {
		return false
	}
	raw, ok := child.(*ast.RawHTML)
	if !ok {
		return false
	}
	var text []byte
	for i := 0; i < raw.Segments.Len(); i++ {
		segment := raw.Segments.At(i)
		text = append(text, segment.Value(source)...)
	}
	match := stashBlockLevelTag.FindSubmatch(text)
	if match == nil {
		return false
	}
	switch match[1][0] {
	case '!', '?', '@', '%':
		return true
	}
	return blockLevelElements[lower(match[1])]
}

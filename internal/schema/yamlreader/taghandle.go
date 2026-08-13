package yamlreader

import (
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/token"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// UndefinedTagHandleError is ruamel's `ParserError` for a *named* tag handle
// that no `%TAG` directive bound: `parse_node` builds a `Tag` against the
// document's handle table and raises when `check_handle` finds the handle
// missing (`ruamel/yaml/parser.py:404-411`, `ruamel/yaml/tag.py:123-126`).
//
// **goccy has no handle table, so it has no such failure.** It hands `!e!x`
// back as an ordinary local tag, the port resolved it into a `TaggedScalar` and
// the document went on to fail — or render — somewhere else entirely: the
// measured vector `locale.language: [!e!x v]` came out as an entry-type
// mismatch at `cv.sections.x`, and `!e!x` over the whole top-level mapping
// rendered a CV at exit 0. Spec `002-yaml-and-core-model/spec-delta-directives.md`
// §4.2.4.
//
// The two marks are ruamel's own and are not always the tag's: see
// undefinedTagHandle.
type UndefinedTagHandleError struct {
	// Handle is the handle as the scanner read it, `!e!` — the text ruamel
	// `repr`s into `found undefined tag handle '!e!'`.
	Handle string
	// Start is ruamel's context mark, End its problem mark.
	Start yamldoc.Position
	End   yamldoc.Position
}

// Error is the first line of ruamel's message, which is the only part upstream
// shows: `read_yaml_with_validation_errors` interpolates `str(e).splitlines()[0]`
// (`rendercv_model_builder.py:87-89`), so the handle name never reaches the
// user.
func (e *UndefinedTagHandleError) Error() string { return "while parsing a node" }

// checkTagHandles walks a document in the order ruamel composes it — a mapping
// key before its value, an element before its successor — and returns the first
// undefined named handle, because ruamel raises at the first node it builds.
//
// The recursion returns a concrete pointer and this wrapper does the widening,
// so a clean document returns a nil `error` rather than a non-nil interface
// holding a nil pointer.
func checkTagHandles(src string, body ast.Node, handles TagHandles) error {
	if err := undefinedTagHandle(src, body, handles, nil); err != nil {
		return err
	}
	return nil
}

// undefinedTagHandle is checkTagHandles' recursion. `anchor` is the `&` token of
// an anchor the node is wrapped in, which is what makes the marks awkward:
//
//   - no anchor — both marks are the tag (`parser.py:389-398`).
//   - `&a !e!x` — the anchor is read first, so it is the context mark and the
//     tag stays the problem mark (`parser.py:375-388`).
//   - `!e!x &a` — the anchor is read second and overwrites *both*
//     (`parser.py:399-403`: `start_mark = tag_mark = token.start_mark`), so
//     neither mark points at the tag at all.
//
// Measured on all three; the display shows only the line, which is the same for
// each, but the marks are what `get_yaml_error_location` reads and a later
// multi-line shape would expose them.
func undefinedTagHandle(
	src string,
	n ast.Node,
	handles TagHandles,
	anchor *token.Token,
) *UndefinedTagHandleError {
	switch v := n.(type) {
	case *ast.AnchorNode:
		return undefinedTagHandle(src, v.Value, handles, v.Start)
	case *ast.TagNode:
		if err := tagHandleFault(src, v, handles, anchor); err != nil {
			return err
		}
		return undefinedTagHandle(src, v.Value, handles, nil)
	case *ast.MappingNode:
		for _, mv := range v.Values {
			if err := undefinedTagHandle(src, mv.Key, handles, nil); err != nil {
				return err
			}
			if err := undefinedTagHandle(src, mv.Value, handles, nil); err != nil {
				return err
			}
		}
	case *ast.MappingValueNode:
		if err := undefinedTagHandle(src, v.Key, handles, nil); err != nil {
			return err
		}
		return undefinedTagHandle(src, v.Value, handles, nil)
	case *ast.SequenceNode:
		for _, entry := range v.Values {
			if err := undefinedTagHandle(src, entry, handles, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// tagHandleFault reports the failure one tag carries, or nil when its handle is
// one the document defines.
//
// **A verbatim `!<…>` tag names no handle** (`ruamel/yaml/scanner.py:1055-1064`
// sets `handle = None`, and `check_handle` passes a `None` handle), and its URI
// may legally contain a `!` — `!<tag:x!y>` — so reading one as a handle would
// invent a failure upstream does not have.
//
// **A handle with an empty suffix is a different failure and is left alone**:
// `!e!` is ruamel's *scanner* refusing an empty URI (`scanner.py:1676-1682`,
// `while parsing an tag`), raised before the handle table is ever consulted. It
// is out of this unit; the port still reads it as a local tag.
func tagHandleFault(
	src string,
	node *ast.TagNode,
	handles TagHandles,
	anchor *token.Token,
) *UndefinedTagHandleError {
	written := tagName(node)
	if strings.HasPrefix(written, "!<") {
		return nil
	}
	handle, _, ok := splitTagHandle(written)
	if !ok {
		return nil
	}
	if _, bound := handles[handle]; bound {
		return nil
	}

	at := tokenPosition(node.Start)
	err := &UndefinedTagHandleError{Handle: handle, Start: at, End: at}
	if anchor != nil {
		err.Start = anchorMark(src, tokenPosition(anchor))
	}
	if inner, ok := node.Value.(*ast.AnchorNode); ok {
		at = anchorMark(src, tokenPosition(inner.Start))
		err.Start, err.End = at, at
	}
	return err
}

// tokenPosition is a token's 1-based coordinates, or the zero position when the
// parser left it without any.
func tokenPosition(tok *token.Token) yamldoc.Position {
	if tok == nil || tok.Position == nil {
		return yamldoc.Position{}
	}
	return yamldoc.Position{Line: tok.Position.Line, Column: tok.Position.Column}
}

// anchorMark is where an anchor's `&` really is.
//
// **goccy misplaces an anchor that follows a tag on the same line by exactly
// one column** — measured, `k: [!e!x &a v]` reports the `&` at column 9 where
// it is at 10, and `k: !e!x   &a v` reports 10 where it is 11 — because the tag
// token's origin swallowed the separating space. An anchor that *precedes* the
// tag, or sits on a line of its own, is reported correctly.
//
// Rather than model goccy's arithmetic, the `&` is found in the source: it is
// the first one at or after the reported column, and everything between the two
// is the separation whitespace, which cannot contain another `&`.
func anchorMark(src string, at yamldoc.Position) yamldoc.Position {
	lines := strings.Split(src, "\n")
	if at.Line < 1 || at.Line > len(lines) || at.Column < 1 {
		return at
	}
	text := lines[at.Line-1]
	if at.Column > len(text) {
		return at
	}
	found := strings.IndexByte(text[at.Column-1:], '&')
	if found < 0 {
		return at
	}
	return yamldoc.Position{Line: at.Line, Column: at.Column + found}
}

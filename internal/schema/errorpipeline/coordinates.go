package errorpipeline

import (
	"fmt"
	"strconv"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// The two internal-failure messages a walk can raise
// (pydantic_error_handling.py:206-208, :211-213), pinned by
// `tests/schema/test_pydantic_error_handling.py:233-246`.
const (
	messageIndexOutOfRange = "Index %d is out of range in the YAML file."
	messageKeyNotFound     = "Key '%s' not found in the YAML file."
)

// coordinatePath is step 10's first half (pydantic_error_handling.py:106-108).
//
// **The comparison is against the literal code `missing`, not a class of
// absence failures.** A missing key has no node of its own, so the coordinates
// point at the parent container; every other kind resolves at its own element.
// Measured: `expected_errors.yaml:56-61` and `:63-67` report the same
// `[[23, 7], [23, 8]]` — the second entry's mapping — for the two missing
// `EducationEntry` fields.
func coordinatePath(location []string, code schemaerr.Code) []string {
	if code != "missing" || len(location) == 0 {
		return location
	}
	return location[:len(location)-1]
}

// resolveCoordinates walks the coordinate document one location element at a
// time and returns the span of the last one
// (pydantic_error_handling.py:222-257, per-step at :179-219).
//
// An empty path gives the document's own zero span, which is upstream's
// `((0, 0), (0, 0))` starting value returned untouched.
//
// A walk that runs off the end of a sequence, or names a key the document does
// not have, is an internal failure rather than a validation one — the document
// being walked is the user's own, so a miss means the location was built wrong.
func resolveCoordinates(doc *yamldoc.Node, path []string) (*yamldoc.Span, error) {
	if doc == nil {
		return nil, nil
	}

	current := doc
	var span yamldoc.Span

	for _, element := range path {
		next, elementSpan, err := stepInto(current, element)
		if err != nil {
			return nil, err
		}
		current, span = next, elementSpan
	}
	return &span, nil
}

// stepInto navigates one level and reports the span of the element it stepped
// through (pydantic_error_handling.py:179-219).
//
// An element that parses as an integer is a sequence index and anything else is
// a mapping key — upstream's `try: int(...) except ValueError:`, which means a
// mapping whose keys are digits is unreachable by this walk. That is upstream's
// behavior and the port keeps it.
func stepInto(node *yamldoc.Node, element string) (*yamldoc.Node, yamldoc.Span, error) {
	if index, err := strconv.Atoi(element); err == nil {
		if node.Kind != yamldoc.KindSequence || index < 0 || index >= len(node.Elems) {
			return nil, yamldoc.Span{}, &schemaerr.InternalError{
				Message: fmt.Sprintf(messageIndexOutOfRange, index),
			}
		}
		return node.Elems[index], sequenceCoordinates(node.Elems[index].Span), nil
	}

	if node.Kind == yamldoc.KindMapping {
		for _, item := range node.Items {
			if item.Key == element {
				return item.Value, mappingCoordinates(item.KeySpan), nil
			}
		}
	}
	return nil, yamldoc.Span{}, &schemaerr.InternalError{
		Message: fmt.Sprintf(messageKeyNotFound, element),
	}
}

// The two coordinate formulas (pydantic_error_handling.py:196, :204), applied to
// the reader's spans.
//
// The reader stores 1-indexed positions; ruamel's `lc.data` is 0-indexed, and
// upstream then adjusts each end differently. Composing the two conversions
// gives the arithmetic below, which is why neither formula is the identity and
// why the sequence one reaches two columns back.
//
// A mapping key: `((sl+1, sc+1), (el+1, ec))` over 0-indexed `lc.data`.
func mappingCoordinates(key yamldoc.Span) yamldoc.Span {
	return yamldoc.Span{
		Start: yamldoc.Position{Line: key.Start.Line, Column: key.Start.Column},
		End:   yamldoc.Position{Line: key.End.Line, Column: key.End.Column - 1},
	}
}

// A sequence element: `((line+1, col-1), (line+1, col))` over the single
// 0-indexed pair `lc.data` holds for an index. Both ends are on the element's
// own line, and the start column is one **before** the reported one.
func sequenceCoordinates(element yamldoc.Span) yamldoc.Span {
	return yamldoc.Span{
		Start: yamldoc.Position{Line: element.Start.Line, Column: element.Start.Column - 2},
		End:   yamldoc.Position{Line: element.Start.Line, Column: element.Start.Column - 1},
	}
}

//go:build conformance

package entries_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// Spec 004 §3.9a behaviors 33a and 33c. The binder's field order must be the
// descriptor's, which is upstream's `model_fields` order — own fields first, then
// the inherited ones, because pydantic emits the last-listed base's own fields
// first (spec 003 §3.2).
//
// Lands red behind the conformance tag and stays red until spec 004 A3 emits the
// date failures at their declared position and moves the `doi` pattern check to
// `doi`'s position: today `start_date` is reported after `highlights` and `doi`
// after `journal`, because both are appended after the field pass.
//
// Iteration 3 composed base-first, so every base-field error preceded every
// own-field error. That is invisible to a per-field test and only shows up when
// two fields fail at once, which is why this test exists as an invariant over all
// eight types rather than as a row per type.
//
// Asserted as a **subsequence**: not every field can be made to fail (the date
// fields are still ValueAny until spec 004 A3/A4), so the property is that the
// reported errors appear in descriptor order relative to one another, with no
// inversion. An inversion is exactly the iteration-3 defect.
func TestFieldErrorsFollowDescriptorOrder(t *testing.T) {
	reference := time.Date(2025, 11, 3, 0, 0, 0, 0, time.UTC)

	// A mapping value is wrong for every declared shape this iteration checks —
	// text and list alike — so it makes as many fields fail at once as possible.
	const badValue = "\n  wrong: 1\n"

	for _, descriptor := range entries.Default().Descriptors() {
		t.Run(string(descriptor.Name), func(t *testing.T) {
			var document strings.Builder
			for _, field := range descriptor.Fields {
				document.WriteString(field)
				document.WriteString(":")
				document.WriteString(badValue)
			}

			node := parseNode(t, document.String())
			errs, err := entries.Validate(
				node, descriptor.Name, nil, schemaerr.SourceMain, reference,
			)
			if err != nil {
				t.Fatalf("internal error: %v", err)
			}
			if len(errs) == 0 {
				t.Fatalf("no errors for a document whose every field is a mapping")
			}

			// Walk the descriptor and the error list together. Every error that
			// names a declared field must appear in descriptor order.
			position := make(map[string]int, len(descriptor.Fields))
			for i, field := range descriptor.Fields {
				position[field] = i
			}

			last := -1
			lastName := ""
			for _, e := range errs {
				if len(e.SchemaLocation) == 0 {
					continue
				}
				name := e.SchemaLocation[len(e.SchemaLocation)-1]
				at, declared := position[name]
				if !declared {
					continue
				}
				if at < last {
					t.Errorf(
						"%s reported after %s, but the descriptor order is %v",
						name, lastName, descriptor.Fields,
					)
				}
				last, lastName = at, name
			}
		})
	}
}

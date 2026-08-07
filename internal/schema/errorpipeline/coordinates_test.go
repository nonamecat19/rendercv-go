package errorpipeline

import (
	"errors"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

const walkDocument = `cv:
  name: John
  sections:
    education:
      - institution: MIT
        area: CS
      - area: Physics
`

// Spec 004 §3.10 behaviors 35 and 36. A missing key has no node of its own, so
// its coordinates point at the parent container — and the comparison is against
// the literal code `missing`, not a class of absence failures.
func TestCoordinatePathTruncatesOnlyForMissing(t *testing.T) {
	location := []string{"cv", "sections", "education", "1", "institution"}

	truncated := coordinatePath(location, "missing")
	if len(truncated) != 4 {
		t.Errorf("missing: path = %v, want the last element dropped", truncated)
	}

	// Every other code keeps the full path, including the ones that also mean
	// something is wrong with an absent-ish value.
	for _, code := range []schemaerr.Code{"string_type", "value_error", "rendercv_other_error", ""} {
		if got := coordinatePath(location, code); len(got) != 5 {
			t.Errorf("code %q: path = %v, want the full location", code, got)
		}
	}

	// An empty location cannot be truncated further.
	if got := coordinatePath(nil, "missing"); len(got) != 0 {
		t.Errorf("empty location: path = %v", got)
	}
}

// The two consequences of behavior 35, stated on one document: two missing
// fields of the same entry report identical coordinates, and a non-missing code
// at the same location resolves one level deeper.
func TestResolveCoordinatesForMissingAndPresent(t *testing.T) {
	doc, err := yamlreader.ReadString(walkDocument)
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}

	entry := []string{"cv", "sections", "education", "1"}
	first, err := resolveCoordinates(doc, coordinatePath(append(entry, "institution"), "missing"))
	if err != nil {
		t.Fatalf("first missing field: %v", err)
	}
	second, err := resolveCoordinates(doc, coordinatePath(append(entry, "degree"), "missing"))
	if err != nil {
		t.Fatalf("second missing field: %v", err)
	}
	if *first != *second {
		t.Errorf("two missing fields of one entry gave %+v and %+v, want the same"+
			" enclosing mapping", *first, *second)
	}

	deeper, err := resolveCoordinates(doc, coordinatePath([]string{"cv", "sections", "education", "1", "area"}, "string_type"))
	if err != nil {
		t.Fatalf("present field: %v", err)
	}
	if *deeper == *first {
		t.Errorf("a present field resolved to the entry's own coordinates %+v", *deeper)
	}
}

// A walk that misses is an internal failure, not a validation one: the document
// is the user's own, so a miss means the location was built wrong.
func TestResolveCoordinatesReportsAMissedWalk(t *testing.T) {
	doc, err := yamlreader.ReadString(walkDocument)
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}

	tests := []struct {
		name string
		path []string
		want string
	}{
		{
			name: "a key the document does not have",
			path: []string{"cv", "nope"},
			want: "Key 'nope' not found in the YAML file.",
		},
		{
			name: "an index past the end of a sequence",
			path: []string{"cv", "sections", "education", "7"},
			want: "Index 7 is out of range in the YAML file.",
		},
		{
			name: "an index into a mapping",
			path: []string{"cv", "0"},
			want: "Index 0 is out of range in the YAML file.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := resolveCoordinates(doc, test.path)

			var internal *schemaerr.InternalError
			if !errors.As(err, &internal) {
				t.Fatalf("err = %v (%T), want *schemaerr.InternalError", err, err)
			}
			if internal.Message != test.want {
				t.Errorf("message = %q, want %q", internal.Message, test.want)
			}
		})
	}
}

// An empty path returns the zero span, which is upstream's `((0, 0), (0, 0))`
// starting value returned untouched. The filter can empty a location, so this
// is reachable.
func TestResolveCoordinatesForAnEmptyPath(t *testing.T) {
	doc, err := yamlreader.ReadString(walkDocument)
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}

	span, err := resolveCoordinates(doc, nil)
	if err != nil {
		t.Fatalf("empty path: %v", err)
	}
	if span == nil || span.Start.Line != 0 || span.Start.Column != 0 ||
		span.End.Line != 0 || span.End.Column != 0 {
		t.Errorf("span = %+v, want the zero span", span)
	}
}

// Upstream's own two cases, with its own inputs and its own assertion
// substrings (`tests/schema/test_pydantic_error_handling.py:233-246`).
//
// They are ported as a pair because they are the only upstream tests of the
// walk's failure paths, and because §4.17 and §4.18's exact text is part of the
// contract even though neither is reachable from a valid location.
func TestUpstreamsOwnWalkFailureCases(t *testing.T) {
	t.Run("an index out of range", func(t *testing.T) {
		doc, err := yamlreader.ReadString("items:\n  - first\n  - second\n")
		if err != nil {
			t.Fatalf("ReadString: %v", err)
		}

		_, err = resolveCoordinates(doc, []string{"items", "10"})
		var internal *schemaerr.InternalError
		if !errors.As(err, &internal) {
			t.Fatalf("err = %v (%T), want *schemaerr.InternalError", err, err)
		}
		if !strings.Contains(internal.Message, "Index 10 is out of range") {
			t.Errorf("message = %q, want it to contain %q",
				internal.Message, "Index 10 is out of range")
		}
	})

	t.Run("a key that is not there", func(t *testing.T) {
		doc, err := yamlreader.ReadString("name: John\n")
		if err != nil {
			t.Fatalf("ReadString: %v", err)
		}

		_, err = resolveCoordinates(doc, []string{"nonexistent"})
		var internal *schemaerr.InternalError
		if !errors.As(err, &internal) {
			t.Fatalf("err = %v (%T), want *schemaerr.InternalError", err, err)
		}
		if !strings.Contains(internal.Message, "Key 'nonexistent' not found") {
			t.Errorf("message = %q, want it to contain %q",
				internal.Message, "Key 'nonexistent' not found")
		}
	})
}

// The failure reaches the caller rather than being swallowed. A fallback span
// would turn a port bug — a location the user's document cannot answer — into a
// wrong coordinate nobody notices.
func TestParseSurfacesAFailedWalk(t *testing.T) {
	doc, err := yamlreader.ReadString(walkDocument)
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}

	_, err = Parse([]schemaerr.ValidationError{{
		SchemaLocation: []string{"cv", "nope"}, Message: "boom",
	}}, doc, nil)

	var internal *schemaerr.InternalError
	if !errors.As(err, &internal) {
		t.Fatalf("err = %v (%T), want *schemaerr.InternalError", err, err)
	}
}

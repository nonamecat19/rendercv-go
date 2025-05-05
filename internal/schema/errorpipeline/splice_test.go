package errorpipeline

import (
	"errors"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// Spec 004 §3.7 behaviors 21-24, on the measured wrapper.
func TestSpliceChildren(t *testing.T) {
	wrapper := schemaerr.ValidationError{
		Code:           CodeEntryValidation,
		SchemaLocation: []string{"cv", "sections", "welcome_to_rendercv_tests_2"},
		Message:        "There are problems with the entries.",
		Children: []schemaerr.ValidationError{
			// Behavior 22's measured child: the leading `entries` goes and the
			// wrapper's location is prepended.
			{Code: "missing", SchemaLocation: []string{"entries", "1", "institution"}, Message: "Field required"},
			// Behavior 24: a model-level failure carries only `("entries", i)`, so
			// it splices to the entry rather than to a field. This is how the
			// start-after-end rule and the generated-URL length check report.
			{Code: "rendercv_other_error", SchemaLocation: []string{"entries", "0"}, Message: "start after end"},
		},
	}

	got := mustParse(t, []schemaerr.ValidationError{wrapper}, nil, nil)

	want := []string{
		// The wrapper's own record is kept, first.
		"cv.sections.welcome_to_rendercv_tests_2",
		"cv.sections.welcome_to_rendercv_tests_2.1.institution",
		"cv.sections.welcome_to_rendercv_tests_2.0",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d records, want %d", len(got), len(want))
	}
	for i := range want {
		if location := strings.Join(got[i].SchemaLocation, "."); location != want[i] {
			t.Errorf("record %d location = %q, want %q", i, location, want[i])
		}
	}

	// Each child went through the whole per-error transform, not just the
	// splice: the missing child picked up the dictionary's replacement.
	if got[1].Message != "This field is required." {
		t.Errorf("child message = %q, want the dictionary's replacement", got[1].Message)
	}
}

// Behavior 23: the prepended part is the wrapper's **raw** location, and the
// spliced whole is filtered once, in the child's own pass.
//
// A section key containing one of the seven substrings is the case that tells
// the two apart — filtering the wrapper first and then prepending would give the
// same answer here, but filtering the child's tail separately would not, because
// the tail's index would survive a filter the section key does not.
func TestSpliceUsesTheRawWrapperLocation(t *testing.T) {
	got := mustParse(t, []schemaerr.ValidationError{{
		Code: CodeEntryValidation,
		// `interests` contains `int`, so the filter deletes it.
		SchemaLocation: []string{"cv", "sections", "interests"},
		Message:        "problems",
		Children: []schemaerr.ValidationError{
			{Code: "missing", SchemaLocation: []string{"entries", "0", "name"}, Message: "Field required"},
		},
	}}, nil, nil)

	if location := strings.Join(got[1].SchemaLocation, "."); location != "cv.sections.0.name" {
		t.Errorf("child location = %q, want cv.sections.0.name — the section key"+
			" is filtered out of the spliced whole", location)
	}
}

// Behavior 25: a wrapper with no children is an internal failure, not a
// validation record. It means the producer built the wrapper wrong.
func TestSpliceRejectsAWrapperWithoutChildren(t *testing.T) {
	_, err := Parse([]schemaerr.ValidationError{{
		Code:           CodeEntryValidation,
		SchemaLocation: []string{"cv", "sections", "x"},
		Message:        "problems",
	}}, nil, nil)

	var internal *schemaerr.InternalError
	if !errors.As(err, &internal) {
		t.Fatalf("err = %v (%T), want *schemaerr.InternalError", err, err)
	}
	if internal.Message != "entry_validation error missing ctx or caused_by" {
		t.Errorf("message = %q", internal.Message)
	}
}

// Behavior 26: one level only. Nested wrappers do not occur upstream — only
// section.py raises this code, never from inside a child — so a child carrying
// the code is spliced as an ordinary record and its own children are dropped
// rather than recursed into.
func TestSpliceDoesNotRecurse(t *testing.T) {
	got := mustParse(t, []schemaerr.ValidationError{{
		Code:           CodeEntryValidation,
		SchemaLocation: []string{"cv", "sections", "x"},
		Message:        "outer",
		Children: []schemaerr.ValidationError{{
			Code:           CodeEntryValidation,
			SchemaLocation: []string{"entries", "0"},
			Message:        "inner",
			Children: []schemaerr.ValidationError{
				{Code: "missing", SchemaLocation: []string{"entries", "0", "deep"}, Message: "Field required"},
			},
		}},
	}}, nil, nil)

	// The wrapper and its one child, and nothing from the grandchild.
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2 — the nested wrapper must not be"+
			" recursed into: %v", len(got), locationsOf(got))
	}
	if got[1].Children != nil {
		t.Errorf("the spliced child kept its own children: %v", got[1].Children)
	}
}

// A record that is not a wrapper contributes exactly itself, whatever it
// carries in Children.
func TestOnlyTheWrapperCodeIsUnpacked(t *testing.T) {
	got := mustParse(t, []schemaerr.ValidationError{{
		Code:           "rendercv_other_error",
		SchemaLocation: []string{"cv", "sections", "x"},
		Message:        "not a wrapper",
		Children: []schemaerr.ValidationError{
			{Code: "missing", SchemaLocation: []string{"entries", "0", "name"}, Message: "Field required"},
		},
	}}, nil, nil)

	if len(got) != 1 {
		t.Errorf("got %d records, want 1: %v", len(got), locationsOf(got))
	}
}

func locationsOf(records []schemaerr.ValidationError) []string {
	out := make([]string, 0, len(records))
	for _, r := range records {
		out = append(out, strings.Join(r.SchemaLocation, "."))
	}
	return out
}

// A child whose location is empty even before the `entries` element is dropped
// splices to the wrapper's own location, and dedup then drops it as a duplicate
// of the wrapper.
//
// No upstream input produces such a child — every `caused_by` entry carries at
// least `("entries", i)` — so this pins what the port does with one rather than
// claiming upstream behavior. It matters because the splice and dedup interact:
// a porter who "fixed" the collapse by special-casing an empty tail would emit a
// record upstream never emits.
func TestAChildWithNoLocationCollapsesIntoTheWrapper(t *testing.T) {
	got := mustParse(t, []schemaerr.ValidationError{{
		Code:           CodeEntryValidation,
		SchemaLocation: []string{"cv", "sections", "x"},
		Message:        "problems",
		Children: []schemaerr.ValidationError{
			{Code: "rendercv_other_error", SchemaLocation: nil, Message: "no location"},
		},
	}}, nil, nil)

	if len(got) != 1 {
		t.Fatalf("got %d records, want 1: %v", len(got), locationsOf(got))
	}
	if got[0].Message != "problems." {
		t.Errorf("message = %q, want the wrapper's — dedup keeps the first", got[0].Message)
	}
}

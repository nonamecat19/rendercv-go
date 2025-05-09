package binder_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// shapeSpec declares one field of each shape, all optional, so a test can aim a
// value at the shape it cares about without tripping the required branch.
func shapeSpec() binder.Spec {
	return binder.Spec{
		Fields: []binder.Field{
			{Name: "raw"},
			{Name: "text", Value: binder.ValueString},
			{Name: "list", Value: binder.ValueStringList},
		},
		Policy: binder.ForbidExtra,
	}
}

type wantError struct {
	code     schemaerr.Code
	location string
}

func codesAndLocations(errs []schemaerr.ValidationError) []wantError {
	out := make([]wantError, 0, len(errs))
	for _, err := range errs {
		out = append(out, wantError{code: err.Code, location: strings.Join(err.SchemaLocation, ".")})
	}
	return out
}

func assertErrors(t *testing.T, got []schemaerr.ValidationError, want []wantError) {
	t.Helper()
	gotPairs := codesAndLocations(got)
	if len(gotPairs) != len(want) {
		t.Fatalf("errors = %+v, want %+v", gotPairs, want)
	}
	for i := range want {
		if gotPairs[i] != want[i] {
			t.Errorf("error %d = %+v, want %+v", i, gotPairs[i], want[i])
		}
	}
}

// Plan §4's table, row by row. ValueAny never checks; ValueString accepts only
// KindString; ValueStringList accepts only a sequence, and checks each element
// as a string.
func TestDeclaredValueShapes(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []wantError
	}{
		{name: "raw accepts a mapping", src: "raw:\n  a: 1\n"},
		{name: "raw accepts a scalar", src: "raw: 5\n"},
		{name: "text accepts a string", src: "text: hello\n"},
		{name: "text accepts an empty string", src: "text: ''\n"},
		{
			// Spec 002 plan §3 classifies `2020` as KindInt, so a year written
			// into a text field must fail rather than coerce.
			name: "text rejects an int",
			src:  "text: 2020\n",
			want: []wantError{{code: binder.CodeStringType, location: "text"}},
		},
		{
			name: "text rejects a float",
			src:  "text: 1.5\n",
			want: []wantError{{code: binder.CodeStringType, location: "text"}},
		},
		{
			name: "text rejects a bool",
			src:  "text: true\n",
			want: []wantError{{code: binder.CodeStringType, location: "text"}},
		},
		{
			name: "text rejects a mapping",
			src:  "text:\n  a: 1\n",
			want: []wantError{{code: binder.CodeStringType, location: "text"}},
		},
		{
			name: "text rejects a sequence",
			src:  "text:\n  - a\n",
			want: []wantError{{code: binder.CodeStringType, location: "text"}},
		},
		{name: "optional text accepts null", src: "text: null\n"},
		{name: "list accepts a string sequence", src: "list:\n  - a\n  - b\n"},
		{name: "list accepts an empty sequence", src: "list: []\n"},
		{name: "optional list accepts null", src: "list: null\n"},
		{
			name: "list rejects a scalar",
			src:  "list: notalist\n",
			want: []wantError{{code: binder.CodeListType, location: "list"}},
		},
		{
			name: "list rejects a mapping",
			src:  "list:\n  a: 1\n",
			want: []wantError{{code: binder.CodeListType, location: "list"}},
		},
		{
			// Element errors are located at the element's own index as a decimal
			// string, and every bad element is reported, not just the first.
			name: "list reports each bad element at its index",
			src:  "list:\n  - 1\n  - 2\n",
			want: []wantError{
				{code: binder.CodeStringType, location: "list.0"},
				{code: binder.CodeStringType, location: "list.1"},
			},
		},
		{
			name: "list reports only the bad element",
			src:  "list:\n  - a\n  - 2\n  - c\n",
			want: []wantError{{code: binder.CodeStringType, location: "list.1"}},
		},
		{
			name: "list rejects a null element",
			src:  "list:\n  - a\n  - null\n",
			want: []wantError{{code: binder.CodeStringType, location: "list.1"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errs := binder.Bind(parse(t, test.src), shapeSpec(), nil, schemaerr.SourceMain)
			assertErrors(t, errs, test.want)
		})
	}
}

// Spec 003 §5.7: the key is present, so only its value is wrong. A required text
// field written as null reports string_type, not missing — verified upstream with
// `PublicationEntry(title=None, authors=["a"])`, which reports string_type on
// `title` alone.
func TestRequiredFieldWrittenNullIsATypeError(t *testing.T) {
	spec := binder.Spec{
		Fields: []binder.Field{
			{Name: "title", Required: true, Value: binder.ValueString},
			{Name: "authors", Required: true, Value: binder.ValueStringList},
		},
		Policy: binder.ForbidExtra,
	}

	t.Run("text", func(t *testing.T) {
		_, errs := binder.Bind(parse(t, "title: null\nauthors:\n  - a\n"), spec, nil, schemaerr.SourceMain)
		assertErrors(t, errs, []wantError{{code: binder.CodeStringType, location: "title"}})
	})

	t.Run("list", func(t *testing.T) {
		_, errs := binder.Bind(parse(t, "title: t\nauthors: null\n"), spec, nil, schemaerr.SourceMain)
		assertErrors(t, errs, []wantError{{code: binder.CodeListType, location: "authors"}})
	})
}

// Spec 003 §5.10. Pydantic emits one pass in declaration order, interleaving
// absences and shape failures rather than reporting all absences first. Every row
// here was measured against the vendored Python, using PublicationEntry's own
// field order (title, authors, summary, doi, url, journal, date).
func TestFieldErrorsFollowDeclarationOrder(t *testing.T) {
	spec := binder.Spec{
		Fields: []binder.Field{
			{Name: "title", Required: true, Value: binder.ValueString},
			{Name: "authors", Required: true, Value: binder.ValueStringList},
			{Name: "summary", Value: binder.ValueString},
			{Name: "journal", Value: binder.ValueString},
		},
		Policy: binder.AllowExtra,
	}

	tests := []struct {
		name string
		src  string
		want []wantError
	}{
		{
			// The missing `title` precedes the bad `authors` even though
			// `authors` is the only key in the input.
			name: "missing precedes a later shape failure",
			src:  "authors: notalist\n",
			want: []wantError{
				{code: binder.CodeMissing, location: "title"},
				{code: binder.CodeListType, location: "authors"},
			},
		},
		{
			name: "three fields interleave in declared order",
			src:  "summary: 5\nauthors: x\n",
			want: []wantError{
				{code: binder.CodeMissing, location: "title"},
				{code: binder.CodeListType, location: "authors"},
				{code: binder.CodeStringType, location: "summary"},
			},
		},
		{
			name: "input order does not affect report order",
			src:  "journal: 5\nauthors:\n  - a\n",
			want: []wantError{
				{code: binder.CodeMissing, location: "title"},
				{code: binder.CodeStringType, location: "journal"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errs := binder.Bind(parse(t, test.src), spec, nil, schemaerr.SourceMain)
			assertErrors(t, errs, test.want)
		})
	}
}

// The two texts are pydantic's own, and are part of the contract (spec 003 §4.4,
// §4.5). Pinned here so a reword has to be deliberate.
func TestValueShapeMessages(t *testing.T) {
	_, errs := binder.Bind(parse(t, "text: 1\nlist: nope\n"), shapeSpec(), nil, schemaerr.SourceMain)
	if len(errs) != 2 {
		t.Fatalf("errs = %+v, want two", errs)
	}
	if got, want := errs[0].Message, "Input should be a valid string"; got != want {
		t.Errorf("string message = %q, want %q", got, want)
	}
	if got, want := errs[1].Message, "Input should be a valid list"; got != want {
		t.Errorf("list message = %q, want %q", got, want)
	}
}

// ValueAny is the zero value, so every Field literal written before this change
// keeps binding raw nodes with no check.
func TestValueAnyIsTheZeroValue(t *testing.T) {
	if binder.ValueAny != 0 {
		t.Errorf("ValueAny = %d, want 0", binder.ValueAny)
	}
	var field binder.Field
	if field.Value != binder.ValueAny {
		t.Errorf("zero Field.Value = %d, want ValueAny", field.Value)
	}
}

// A field with no declared shape can still carry a scalar constraint.
//
// That combination is what a required-but-nullable field needs: an explicit null
// is its declared default and must pass, so it cannot be ValueString, but a
// value it does carry is still checked. `CustomConnection.url` is the case.
func TestScalarRunsForValueAny(t *testing.T) {
	failing := errors.New("nope")
	spec := binder.Spec{Fields: []binder.Field{{
		Name:       "url",
		Required:   true,
		Scalar:     func(string, bool) error { return failing },
		ScalarCode: "custom_code",
	}}}

	tests := []struct {
		name      string
		src       string
		wantCount int
	}{
		{name: "a value is checked", src: "url: something\n", wantCount: 1},
		{name: "an explicit null is not", src: "url: null\n", wantCount: 0},
		{name: "a mapping is not", src: "url:\n  a: 1\n", wantCount: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errs := binder.Bind(parse(t, test.src), spec, nil, schemaerr.SourceMain)
			if len(errs) != test.wantCount {
				t.Fatalf("errs = %+v, want %d", errs, test.wantCount)
			}
			if test.wantCount == 1 && errs[0].Code != "custom_code" {
				t.Errorf("code = %q, want the registered one", errs[0].Code)
			}
		})
	}
}

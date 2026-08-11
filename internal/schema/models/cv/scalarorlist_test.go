package cv_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// Spec §3.47 — the shared rule governs exactly these three fields.
func TestScalarOrListFields(t *testing.T) {
	want := []string{"website", "email", "phone"}
	got := cv.ScalarOrListFields()
	if len(got) != len(want) {
		t.Fatalf("fields = %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("fields = %v, want %v", got, want)
		}
	}
}

// Spec §3.47 — absent, list and scalar route differently; the list branch is taken
// only when the input actually is a list.
func TestScalarOrListRouting(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		input     string
		wantCalls int
	}{
		{name: "scalar email", field: "email", input: "john@example.com", wantCalls: 1},
		{name: "list of emails", field: "email", input: "- a@example.com\n- b@example.com", wantCalls: 2},
		{name: "empty list", field: "email", input: "[]", wantCalls: 0},
		{name: "scalar website", field: "website", input: "https://example.com", wantCalls: 1},
		{name: "scalar phone", field: "phone", input: "\"+905419999999\"", wantCalls: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			node := parseValue(t, tc.input)
			calls := 0
			restore := cv.SetElementValidatorForTest(tc.field, func(
				*yamldoc.Node, []string, schemaerr.YamlSource,
			) []schemaerr.ValidationError {
				calls++
				return nil
			})
			defer restore()

			errs, err := cv.ValidateScalarOrList(tc.field, node, []string{"cv", tc.field}, schemaerr.SourceMain)
			if err != nil {
				t.Fatalf("unexpected internal error: %v", err)
			}
			if len(errs) != 0 {
				t.Fatalf("errs = %+v, want none", errs)
			}
			if calls != tc.wantCalls {
				t.Errorf("element validator called %d times, want %d", calls, tc.wantCalls)
			}
		})
	}
}

// Spec §3.47 — an absent or null value validates nothing.
func TestScalarOrListAbsentAndNull(t *testing.T) {
	for _, node := range []*yamldoc.Node{nil, {Kind: yamldoc.KindNull}} {
		calls := 0
		restore := cv.SetElementValidatorForTest("email", func(
			*yamldoc.Node, []string, schemaerr.YamlSource,
		) []schemaerr.ValidationError {
			calls++
			return nil
		})
		errs, err := cv.ValidateScalarOrList("email", node, []string{"cv", "email"}, schemaerr.SourceMain)
		restore()

		if err != nil || len(errs) != 0 || calls != 0 {
			t.Errorf("node %+v: errs=%+v err=%v calls=%d, want a no-op", node, errs, err, calls)
		}
	}
}

// Spec §3.48, §4.7 — invoking the rule without a field name is an internal error.
func TestScalarOrListWithoutFieldName(t *testing.T) {
	_, err := cv.ValidateScalarOrList("", parseValue(t, "john@example.com"), nil, schemaerr.SourceMain)

	var internal *schemaerr.InternalError
	if !errors.As(err, &internal) {
		t.Fatalf("err = %v (%T), want *schemaerr.InternalError", err, err)
	}
	if internal.Message != "field_name is None in validator" {
		t.Errorf("message = %q, want %q", internal.Message, "field_name is None in validator")
	}
}

// List elements are located by index.
func TestScalarOrListElementLocations(t *testing.T) {
	var seen [][]string
	restore := cv.SetElementValidatorForTest("email", func(
		_ *yamldoc.Node, location []string, _ schemaerr.YamlSource,
	) []schemaerr.ValidationError {
		seen = append(seen, location)
		return nil
	})
	defer restore()

	_, err := cv.ValidateScalarOrList(
		"email", parseValue(t, "- a@example.com\n- b@example.com"),
		[]string{"cv", "email"}, schemaerr.SourceMain,
	)
	if err != nil {
		t.Fatalf("unexpected internal error: %v", err)
	}
	want := [][]string{{"cv", "email", "0"}, {"cv", "email", "1"}}
	if len(seen) != len(want) {
		t.Fatalf("locations = %v, want %v", seen, want)
	}
	for i := range want {
		if len(seen[i]) != 3 || seen[i][2] != want[i][2] {
			t.Errorf("locations = %v, want %v", seen, want)
		}
	}
}

// Spec §3.49 — serialization removes the `tel:` scheme.
func TestSerializePhone(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "tel:+90-541-999-99-99", want: "+90-541-999-99-99"},
		{in: "+90-541-999-99-99", want: "+90-541-999-99-99"},
		{in: "", want: ""},
	}
	for _, tc := range tests {
		if got := cv.SerializePhone(tc.in); got != tc.want {
			t.Errorf("SerializePhone(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// parseValue parses a value in isolation by wrapping it in a one-key mapping.
func parseValue(t *testing.T, src string) *yamldoc.Node {
	t.Helper()
	var wrapped string
	if len(src) > 0 && src[0] == '-' {
		wrapped = "v:\n" + src + "\n"
	} else {
		wrapped = "v: " + src + "\n"
	}
	node := parse(t, wrapped)
	return node.Items[0].Value
}

// **A format check never runs on a value that is not a string.** EmailStr,
// PhoneNumber and HttpUrl are all validated after pydantic has read the input as
// a string, so `cv.email: 5` is `Input should be a valid string.` upstream — the
// port handed the raw token to the format validator and said `An email address
// must have an @-sign.` instead.
//
// Measured against the vendored Python on 24 documents: int, float, bool,
// mapping, a tagged scalar, and each of those inside the list form, for all
// three fields.
func TestScalarOrListFieldsRejectANonStringByType(t *testing.T) {
	tests := []struct {
		field   string
		input   string
		message string
	}{
		{field: "email", input: "5", message: "Input should be a valid string"},
		{field: "email", input: "0.5", message: "Input should be a valid string"},
		{field: "email", input: "true", message: "Input should be a valid string"},
		{field: "email", input: "{a: 1}", message: "Input should be a valid string"},
		{field: "email", input: "!!str a@b.com", message: "Input should be a valid string"},
		{field: "phone", input: "5", message: "Input should be a valid string"},
		{field: "phone", input: "{a: 1}", message: "Input should be a valid string"},
		{field: "phone", input: "!!str +1-541-754-3010", message: "Input should be a valid string"},
		{field: "website", input: "5", message: "URL input should be a string or URL"},
		{field: "website", input: "true", message: "URL input should be a string or URL"},
		{field: "website", input: "!!str https://a.com", message: "URL input should be a string or URL"},
	}

	for _, tc := range tests {
		t.Run(tc.field+": "+tc.input, func(t *testing.T) {
			node, err := yamlreader.ReadString(tc.field + ": " + tc.input + "\n")
			if err != nil {
				t.Fatalf("ReadString = %v", err)
			}
			value := node.Items[0].Value
			errs, internalErr := cv.ValidateScalarOrList(
				tc.field, value, []string{"cv", tc.field}, schemaerr.SourceMain)
			if internalErr != nil {
				t.Fatalf("internal error = %v", internalErr)
			}
			if len(errs) != 1 {
				t.Fatalf("errors = %+v, want exactly one", errs)
			}
			if errs[0].Message != tc.message {
				t.Errorf("message = %q, want %q", errs[0].Message, tc.message)
			}
		})
	}
}

// The element form reports per element, at the element's own index, and a valid
// element beside an invalid one is still valid.
func TestScalarOrListElementTypeErrorsAreIndexed(t *testing.T) {
	node, err := yamlreader.ReadString("email: [\"a@b.com\", 5]\n")
	if err != nil {
		t.Fatalf("ReadString = %v", err)
	}
	errs, internalErr := cv.ValidateScalarOrList(
		"email", node.Items[0].Value, []string{"cv", "email"}, schemaerr.SourceMain)
	if internalErr != nil {
		t.Fatalf("internal error = %v", internalErr)
	}
	if len(errs) != 1 {
		t.Fatalf("errors = %+v, want exactly one", errs)
	}
	if got := strings.Join(errs[0].SchemaLocation, "."); got != "cv.email.1" {
		t.Errorf("location = %q, want cv.email.1", got)
	}
	if errs[0].Message != "Input should be a valid string" {
		t.Errorf("message = %q", errs[0].Message)
	}
}

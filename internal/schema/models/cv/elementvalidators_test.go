package cv_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/errorpipeline"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// The three scalar-or-list element validators, through cv.ValidateScalarOrList.
//
// Each asserts the **raw** message, because what a validator emits and what a
// user sees are different strings and the difference is the whole point of the
// raw/final split.
func TestElementValidatorRawMessages(t *testing.T) {
	tests := []struct {
		name  string
		field string
		src   string
		want  string
	}{
		{
			// Pydantic's own template, reproduced so the pipeline's strip runs
			// on production data (spec 004 §3.2 behavior 4a).
			name:  "a bad email carries the template prefix",
			field: "email",
			src:   "not_a_valid_email\n",
			want:  "value is not a valid email address: An email address must have an @-sign.",
		},
		{
			// The dictionary key, not §4.8's replacement.
			name:  "a bad phone is the dictionary key",
			field: "phone",
			src:   "not_a_valid_phone_number\n",
			want:  "value is not a valid phone number",
		},
		{
			// Also a dictionary key.
			name:  "a bad website is the URL dictionary key",
			field: "website",
			src:   "not a url\n",
			want:  "Input should be a valid URL",
		},
		{
			// Not a dictionary key: no row matches, so this reaches the user
			// with only a period appended.
			name:  "a wrong scheme keeps its own message",
			field: "website",
			src:   "ftp://example.com\n",
			want:  "URL scheme should be 'http' or 'https'",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs, err := cv.ValidateScalarOrList(
				test.field, parse(t, "k: "+test.src).Items[0].Value,
				[]string{"cv", test.field}, schemaerr.SourceMain,
			)
			if err != nil {
				t.Fatalf("internal error: %v", err)
			}
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Message != test.want {
				t.Errorf("raw message =\n  %q\nwant\n  %q", errs[0].Message, test.want)
			}
		})
	}
}

// The email path end to end, which is the only production path that exercises
// step 1 of the pipeline. Its final text is what records 1 and 2 of upstream's
// 25-record fixture contain.
func TestEmailReachesTheUserStripped(t *testing.T) {
	errs, err := cv.ValidateScalarOrList(
		"email", parse(t, "k: not_a_valid_email\n").Items[0].Value,
		[]string{"cv", "email"}, schemaerr.SourceMain,
	)
	if err != nil {
		t.Fatalf("internal error: %v", err)
	}

	final, err := errorpipeline.Parse(errs, nil, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(final) != 1 {
		t.Fatalf("final = %+v, want one record", final)
	}

	const want = "An email address must have an @-sign."
	if final[0].Message != want {
		t.Errorf("final message = %q, want %q", final[0].Message, want)
	}
	if strings.Contains(final[0].Message, "value is not a valid") {
		t.Errorf("the template prefix survived: %q", final[0].Message)
	}
}

// A list value reports one record per bad element, at the element's own index,
// and good elements report nothing.
func TestElementValidatorsReportPerIndex(t *testing.T) {
	errs, err := cv.ValidateScalarOrList(
		"phone",
		parse(t, "k:\n  - \"+905419999999\"\n  - nope\n  - \"+1-415-555-0142\"\n  - also_nope\n").Items[0].Value,
		[]string{"cv", "phone"}, schemaerr.SourceMain,
	)
	if err != nil {
		t.Fatalf("internal error: %v", err)
	}

	want := []string{"cv.phone.1", "cv.phone.3"}
	if len(errs) != len(want) {
		t.Fatalf("errs = %+v, want %d", errs, len(want))
	}
	for i := range want {
		if got := strings.Join(errs[i].SchemaLocation, "."); got != want[i] {
			t.Errorf("record %d location = %q, want %q", i, got, want[i])
		}
	}
}

// Spec 004 §3.14 behavior 48: the stored value is re-grouped, and SerializePhone
// is what renders it. Iteration 2's `tel:` strip alone produced the input
// grouping, which two golden .typ files contradict.
func TestSerializePhoneForwardsToTheLibrary(t *testing.T) {
	if got := cv.SerializePhone("tel:+34-612-34-56-78"); got != "+34-612-34-56-78" {
		t.Errorf("SerializePhone = %q", got)
	}
}

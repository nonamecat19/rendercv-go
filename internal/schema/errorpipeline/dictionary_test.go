package errorpipeline

import "testing"

// Order is contractual (spec 004 §6 rule 5) even though it is currently
// unobservable: rows 9 and 10 map to the same replacement, and every other pair
// is disjoint on every measured message. A future upstream row could overlap, so
// the position of each key is asserted rather than its mere presence.
//
// The submodule diff in dictionary_conformance_test.go is the check on the
// *text*; this one is the check that nothing reorders or drops a row when the
// submodule is not available.
func TestDictionaryOrder(t *testing.T) {
	want := []string{
		`Input should be 'present'`,
		`Input should be a valid integer, unable to parse string as an integer`,
		`String should match pattern '\\d{4}-\\d{2}(-\\d{2})?'`,
		`String should match pattern '\\b10\\..*'`,
		`Input should be a valid URL`,
		`Field required`,
		`value is not a valid phone number`,
		`month must be in 1..12`,
		`day is out of range for month`,
		`must be in range`,
		`Extra inputs are not permitted`,
		`Input should be a valid list`,
		`value is not a valid color`,
	}

	if len(dictionary) != len(want) {
		t.Fatalf("dictionary has %d rows, want %d", len(dictionary), len(want))
	}
	for i, key := range want {
		if dictionary[i].Old != key {
			t.Errorf("row %d key = %q, want %q", i+1, dictionary[i].Old, key)
		}
	}
}

// Substitution is **containment**, not equality. Both rows below are reached by
// a message strictly longer than the key, which is the whole reason the port
// cannot implement this as a map lookup.
func TestSubstituteMatchesOnContainment(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			// Row 5. Every URL parse failure carries its own reason after the
			// common prefix, and they all flatten to one message
			// (spec 004 §3.15 behavior 40).
			name:    "a URL failure keeps only the common prefix's replacement",
			message: "Input should be a valid URL, relative URL without a base",
			want:    "This is not a valid URL.",
		},
		{
			// Row 13, measured on `design.colors.body: notacolor`.
			name:    "a color failure matches a key that is a strict prefix",
			message: "value is not a valid color: string not recognised as a valid color",
			want:    `This is not a valid color. Here are some examples of valid colors: "red", "#ff0000", "rgb(255, 0, 0)", "hsl(0, 100%, 50%)"`,
		},
		{
			// Row 4's key carries doubled backslashes, so pydantic's
			// single-backslash message does not contain it and the message
			// survives unreplaced. This is the row a porter is most likely to
			// "fix" (spec 004 §3.4 behavior 13).
			name:    "the DOI pattern message is not replaced",
			message: `String should match pattern '\b10\..*'`,
			want:    `String should match pattern '\b10\..*'`,
		},
		{
			name:    "an unmatched message is returned unchanged",
			message: "Input should be a valid dictionary",
			want:    "Input should be a valid dictionary",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := substitute(test.message); got != test.want {
				t.Errorf("substitute(%q) =\n  %q\nwant\n  %q", test.message, got, test.want)
			}
		})
	}
}

// First match wins and then stops. Rows 6 and 12 are disjoint on every real
// message, so the property needs a message containing two keys at once to be
// observable at all — it is asserted on a synthetic one rather than left to the
// reader.
func TestSubstituteStopsAtTheFirstMatch(t *testing.T) {
	const both = "Input should be a valid list; Field required"
	if got := substitute(both); got != "This field is required." {
		t.Errorf("substitute(%q) = %q, want row 6's value — `Field required`"+
			" is row 6 and `Input should be a valid list` is row 12", both, got)
	}
}

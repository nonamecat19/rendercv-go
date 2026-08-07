package errorpipeline

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

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

// Rows 1 to 4 are unreachable **through the pipeline**, each for its own reason,
// and this is the test that says so rather than the comment on the data.
//
// Their being dead is not a curiosity: three of the four map to a friendlier
// message than the one users actually see, so a porter who "fixes" any of them
// changes real output. Each row below is the measured end-to-end result.
func TestDeadDictionaryRowsStayDead(t *testing.T) {
	tests := []struct {
		name     string
		row      int
		location []string
		message  string
		want     string
	}{
		{
			// Row 1's key is `Input should be 'present'`, but step 5 has already
			// replaced the message by the time the dictionary runs.
			name:     "row 1 is pre-empted by the end_date override",
			row:      1,
			location: []string{"cv", "end_date"},
			message:  "Input should be 'present'",
			want: "This is not a valid `end_date`! Please use either YYYY-MM-DD," +
				` YYYY-MM, or YYYY format or "present"!.`,
		},
		{
			// Row 2's key needs the `, unable to parse string as an integer`
			// suffix, which only an int-only field produces. Every int-typed
			// field upstream is `int | str`, so the bare form is what arrives.
			name:     "row 2 needs a suffix no field produces",
			row:      2,
			location: []string{"cv", "date"},
			message:  "Input should be a valid integer",
			want:     "Input should be a valid integer.",
		},
		{
			// Rows 3 and 4 carry doubled backslashes; pydantic's messages carry
			// single ones, so neither key is ever contained.
			name:     "row 4's DOI pattern message passes through",
			row:      4,
			location: []string{"cv", "doi"},
			message:  `String should match pattern '\b10\..*'`,
			want:     `String should match pattern '\b10\..*'.`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Parse([]schemaerr.ValidationError{{
				SchemaLocation: test.location, Message: test.message,
			}}, nil, nil)[0]

			if got.Message != test.want {
				t.Errorf("row %d:\n  got  %q\n  want %q", test.row, got.Message, test.want)
			}
			if got.Message == dictionary[test.row-1].New+"." {
				t.Errorf("row %d fired; it is supposed to be unreachable", test.row)
			}
		})
	}
}

// A live row, end to end, so the test above cannot pass because the dictionary
// is wired up wrong altogether.
func TestALiveDictionaryRowFiresThroughParse(t *testing.T) {
	got := Parse([]schemaerr.ValidationError{{
		SchemaLocation: []string{"cv", "name"}, Message: "Field required",
	}}, nil, nil)[0]

	if got.Message != "This field is required." {
		t.Errorf("message = %q, want row 6's replacement", got.Message)
	}
}

// Step 1 runs before step 7 because upstream orders them that way, but on every
// measured message the order is **unobservable**, and this test is the reason to
// believe that rather than assume it.
//
// Substitution matches by containment and replaces the whole message, so a
// prefix can only ever add a match, never remove one. It adds none: neither
// prefix contains a dictionary key, and stripping one never changes which row
// fires. Swapping the two steps therefore passes the entire suite.
//
// Spec 004 §3.2's table gives the reason for the ordering as "or `value is not a
// valid phone number` never matches". That reason does not hold — the phone
// message carries no prefix at all (measured: pydantic reports it bare). The
// ordering is still reproduced, because it is upstream's and a future dictionary
// row could depend on it; what is corrected here is the justification, so nobody
// later writes a test that claims to prove an ordering it cannot.
func TestPrefixStripDoesNotChangeWhichRowMatches(t *testing.T) {
	messages := []string{
		"value is not a valid email address: An email address must have an @-sign.",
		"value is not a valid email address: The part after the @-sign is not valid.",
		"Value error, month must be in 1..12",
		"Value error, day is out of range for month",
		"value is not a valid phone number",
	}

	for _, message := range messages {
		t.Run(message, func(t *testing.T) {
			if before, after := matchingRow(message), matchingRow(stripPrefixes(message)); before != after {
				t.Errorf("row %d matches with the prefix and row %d without;"+
					" the step order has become observable and spec 004 §3.2"+
					" needs re-measuring", before, after)
			}
		})
	}

	// Neither prefix contains a key, which is the underlying reason.
	for _, prefix := range []string{emailPrefix, valueErrorPrefix} {
		if row := matchingRow(prefix); row != 0 {
			t.Errorf("prefix %q contains dictionary row %d's key", prefix, row)
		}
	}
}

// matchingRow reports the 1-based row a message matches, or 0 for none.
func matchingRow(message string) int {
	for i, row := range dictionary {
		if strings.Contains(message, row.Old) {
			return i + 1
		}
	}
	return 0
}

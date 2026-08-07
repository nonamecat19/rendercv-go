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

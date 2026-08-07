package entries_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
)

// Spec 004 §3.9b behavior 33g: a bool satisfies a date union's `int` arm and
// pydantic's lax mode converts it, so `date: true` is accepted and stored as 1,
// `date: false` as 0. Measured against the vendored Python.
//
// The arm order and the branch locations are asserted in TestEntryFailureOrder;
// what these two pin is the coercion itself.
func TestBoolArbitraryDateIsAccepted(t *testing.T) {
	for _, src := range []string{"name: n\ndate: true\n", "name: n\ndate: false\n"} {
		got := formatErrors(validateEntry(t, entries.TypeName("NormalEntry"), src), true)
		if len(got) != 0 {
			t.Fatalf("%q: errors = %v, want none", src, got)
		}
	}
}

// The same coercion on an exact date, which is the row that proves the bool
// became an *integer* rather than being waved through. Upstream:
// `start_date: true` coerces to 1, `get_date_object` builds `"1-01-01"`, and
// `Date.fromisoformat` rejects it — so a port that merely skipped bools would
// report nothing here and pass a weaker assertion.
func TestBoolExactDateCoercesToAnIntegerYear(t *testing.T) {
	got := formatErrors(
		validateEntry(t, entries.TypeName("ExperienceEntry"), "company: c\nposition: p\nstart_date: true\n"),
		true,
	)
	if len(got) != 1 {
		t.Fatalf("errors = %v, want exactly one", got)
	}
	if !strings.HasPrefix(got[0], "start_date:") {
		t.Fatalf("errors = %v, want one at `start_date`", got)
	}
}

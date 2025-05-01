package errorpipeline

import (
	"strings"
	"testing"
)

// Spec 004 §3.3 behavior 9's four measured rows, plus the two guards.
func TestSkipDiscriminator(t *testing.T) {
	tests := []struct {
		name     string
		location []string
		want     []string
	}{
		{
			name:     "a design branch value is dropped",
			location: []string{"design", "classic", "nope"},
			want:     []string{"design", "nope"},
		},
		{
			name:     "and the rest of the path survives",
			location: []string{"design", "classic", "page", "top_margin"},
			want:     []string{"design", "page", "top_margin"},
		},
		{
			name:     "a locale branch value is dropped",
			location: []string{"locale", "english", "month"},
			want:     []string{"locale", "month"},
		},
		{
			// `loc[:1] + loc[2:]` on a one-element location is that element.
			name:     "a one-element location is unchanged",
			location: []string{"design"},
			want:     []string{"design"},
		},
		{
			// `settings` is not a discriminated union, so nothing is dropped —
			// this is the row that fails if the root list is widened.
			name:     "settings keeps every element",
			location: []string{"settings", "render_command", "design"},
			want:     []string{"settings", "render_command", "design"},
		},
		{
			// Upstream raises IndexError here; the port guards instead.
			name:     "an empty location does not panic",
			location: nil,
			want:     nil,
		},
		{
			name:     "a non-discriminated root is untouched",
			location: []string{"cv", "sections", "design"},
			want:     []string{"cv", "sections", "design"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := skipDiscriminator(test.location)
			if strings.Join(got, ".") != strings.Join(test.want, ".") {
				t.Errorf("skipDiscriminator(%v) = %v, want %v", test.location, got, test.want)
			}
		})
	}
}

// The caller's slice is never re-sliced. Parse walks the raw list more than
// once, so an aliased result would corrupt the second pass — and a re-slice
// passes every row above while failing this.
func TestSkipDiscriminatorDoesNotAliasItsInput(t *testing.T) {
	location := []string{"design", "classic", "page", "top_margin"}
	got := skipDiscriminator(location)

	got[1] = "clobbered"
	if location[2] != "page" {
		t.Errorf("the input was aliased: location = %v", location)
	}
}

// Spec 004 §3.3 behavior 6's measured synthetic elements, one row per substring
// the filter is meant to catch.
func TestFilterLocationDropsSyntheticElements(t *testing.T) {
	tests := []struct {
		name    string
		element string
	}{
		{"an exact-date after-validator", "function-after[validate_exact_date(), union[str,int]]"},
		{"a wrap validator", "function-wrap[wrap_val()]"},
		{"the present literal", "literal['present']"},
		{"the today literal", "literal['today']"},
		{"a bare union arm", "int"},
		{"the other bare union arm", "str"},
		{"a tagged union tag", "tagged-union[EducationEntry,NormalEntry]"},
		{
			"the photo path branch, the longest measured one",
			"function-after[<lambda>(), lax-or-strict[lax=union[json-or-python[json=" +
				"function-after[path_validator(), str],python=is-instance[Path]]," +
				"function-after[path_validator(), str]],strict=json-or-python[json=" +
				"function-after[path_validator(), str],python=is-instance[Path]]]]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := filterLocation([]string{"cv", test.element, "tail"})
			if strings.Join(got, ".") != "cv.tail" {
				t.Errorf("filterLocation kept %q: got %v", test.element, got)
			}
		})
	}
}

// Behavior 7, the part that is not a nicety: the filter deletes **real user
// keys**, and the four truncated locations are equal, so dedup later reports
// four failing sections as one that names none of them.
//
// Asserted as equality between the four, not just as truncation, because the
// collapse is the observable consequence.
func TestFilterLocationDeletesRealSectionKeys(t *testing.T) {
	truncated := []string{"interests", "my_list", "strengths", "literally_fine"}
	for _, key := range truncated {
		got := filterLocation([]string{"cv", "sections", key})
		if strings.Join(got, ".") != "cv.sections" {
			t.Errorf("section %q: got %v, want [cv sections]", key, got)
		}
	}

	// The control: a section key containing none of the seven survives, so the
	// test above is not passing because the filter eats everything.
	got := filterLocation([]string{"cv", "sections", "normal_key"})
	if strings.Join(got, ".") != "cv.sections.normal_key" {
		t.Errorf("normal_key: got %v, want it kept", got)
	}
}

// Indices are stringified before the test and no decimal string contains any of
// the seven, so they always survive. `expected_errors.yaml` depends on this at
// every entry location.
func TestFilterLocationKeepsIndices(t *testing.T) {
	for _, index := range []string{"0", "1", "9", "10", "123"} {
		got := filterLocation([]string{"cv", "sections", "x", index, "area"})
		if strings.Join(got, ".") != "cv.sections.x."+index+".area" {
			t.Errorf("index %q: got %v", index, got)
		}
	}
}

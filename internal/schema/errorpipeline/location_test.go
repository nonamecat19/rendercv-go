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

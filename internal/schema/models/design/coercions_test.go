package design_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
)

// `convert_section_titles_to_snake_case` (classic_theme.py:493-500).
//
// A coercion, not a validation — nothing rejects a spaced title. What it
// produces is what the renderer matches section titles against, so getting it
// wrong makes time spans silently not appear rather than making anything fail.
func TestSnakeCaseSectionTitles(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"the documented case", []string{"Work Experience"}, []string{"work_experience"}},
		{"already snake case", []string{"experience"}, []string{"experience"}},
		{"several words", []string{"A B C"}, []string{"a_b_c"}},
		{"mixed list", []string{"Experience", "Education"}, []string{"experience", "education"}},
		{"empty", nil, []string{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := design.SnakeCaseSectionTitles(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("= %q, want %q", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

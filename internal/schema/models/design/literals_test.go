package design_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
)

// Every union's members, in upstream's declaration order.
//
// The lists are restated here rather than compared to themselves: the point is
// that a reordering fails, and a test that iterated the same slice could not
// see one.
func TestLiteralUnionsAreInDeclarationOrder(t *testing.T) {
	tests := []struct {
		name string
		got  []string
		want []string
	}{
		{
			"Bullet", design.Bullets,
			[]string{"●", "•", "◦", "-", "◆", "★", "■", "—", "○"},
		},
		{
			"BodyAlignment", design.BodyAlignments,
			[]string{"left", "justified", "justified-with-no-hyphenation"},
		},
		{
			"Alignment", design.Alignments,
			[]string{"left", "center", "right"},
		},
		{"SectionTitleType", design.SectionTitleTypes, []string{
			"with_partial_line", "with_full_line", "without_line", "moderncv",
			"centered_without_line", "centered_with_partial_line",
			"centered_with_centered_partial_line", "centered_with_full_line",
		}},
		{
			"PhoneNumberFormatType", design.PhoneNumberFormats,
			[]string{"national", "international", "E164"},
		},
		{
			"PageSize", design.PageSizes,
			[]string{"a4", "a5", "us-letter", "us-executive"},
		},
		{
			"photo_position", design.PhotoPositions,
			[]string{"left", "right"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.got) != len(tc.want) {
				t.Fatalf("%d members, want %d: %q", len(tc.got), len(tc.want), tc.got)
			}
			for i := range tc.want {
				if tc.got[i] != tc.want[i] {
					t.Errorf("[%d] = %q, want %q", i, tc.got[i], tc.want[i])
				}
			}
		})
	}
}

// `PageSize`'s `literal_error`, measured against the vendored Python on
// `{page: {size: a3}}`.
func TestPageSizeLiteralMessage(t *testing.T) {
	const want = "Input should be 'a4', 'a5', 'us-letter' or 'us-executive'"
	got := binder.LiteralMessage(design.PageSizes, "Input should be a valid page size")
	if got != want {
		t.Errorf("= %q, want %q", got, want)
	}
}

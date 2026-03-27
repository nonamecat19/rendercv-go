package cli

import (
	"bytes"
	"slices"
	"strconv"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// A table inside a panel is cropped to the panel, never folded into it.
//
// Upstream nests the `rich.table.Table` in the `Panel` as a renderable, and
// `render_lines` pads or crops each of its lines to the child width — a Table
// has no wrapping of its own. The port lays the table out first and feeds its
// lines back through `Panel` as rows of text, which *wrap*: below eight columns
// the box's own dividers no longer fit and the port drew one table across four
// bordered rows where upstream shows a single cropped `╭`.
//
// Expectations are upstream's own render of a CV whose `cv.phone` is `12345`,
// captured from the vendored CLI at 5, 6 and 7 columns.
func TestValidationTableIsCroppedNotFolded(t *testing.T) {
	records := []schemaerr.ValidationError{{
		SchemaLocation: []string{"cv", "phone"},
		Input:          "12345",
		Message:        "Input should be a valid string.",
	}}

	tests := []struct {
		columns int
		want    []string
	}{{
		columns: 5,
		want: []string{
			"╭─ ─╮",
			"│ ╭ │",
			"│ │ │",
			"│ ├ │",
			"│ │ │",
			"│ ╰ │",
			"╰───╯",
		},
	}, {
		columns: 6,
		want: []string{
			"╭─ T─╮",
			"│ ╭┬ │",
			"│ ││ │",
			"│ ├┼ │",
			"│ ││ │",
			"│ ╰┴ │",
			"╰────╯",
		},
	}, {
		columns: 7,
		want: []string{
			"╭─ Th─╮",
			"│ ╭┬┬ │",
			"│ │││ │",
			"│ ├┼┼ │",
			"│ │││ │",
			"│ ╰┴┴ │",
			"╰─────╯",
		},
	}}

	for _, test := range tests {
		t.Run(strconv.Itoa(test.columns), func(t *testing.T) {
			t.Setenv("COLUMNS", strconv.Itoa(test.columns))

			var out bytes.Buffer
			validationPanel(&out, records)
			got := splitLines(out.String() + "\n")
			if !slices.Equal(got, test.want) {
				t.Errorf("validation panel at %d columns:\n got %q\nwant %q",
					test.columns, got, test.want)
			}
		})
	}
}

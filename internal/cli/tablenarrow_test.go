package cli

import (
	"strconv"
	"strings"
	"testing"
)

// A column is never measured wider than the table it sits in.
//
// `Table._measure_column` ends in `.with_maximum(max_width)`
// (`rich/table.py:748`), so a natural width larger than the whole table is
// clamped to it **before** the collapse and the even reduction run. The port
// carried the natural width through, and the two stages then divided a
// different excess: at 20 columns upstream's validation table is 5 + 7 + 0 and
// the port's was 4 + 8 + 0.
//
// The reduction is where the clamp shows, because `ratio_reduce` splits the
// excess by ratio (`rich/_ratio.py`) and the excess is what the clamp changes.
// Every expectation is upstream's own `rendercv render` on a CV whose
// `cv.phone` is `12345`, captured from the vendored CLI at the width named.
func TestValidationTableWidthsAtNarrowWidths(t *testing.T) {
	columns := []TableColumn{
		{Header: "Location", NoWrap: true},
		{Header: "Input Value", NoWrap: true},
		{Header: "Explanation"},
	}
	rows := [][]string{{"cv.phone", "12345", "Input should be a valid string."}}

	tests := []struct {
		columns int
		want    []string
	}{{
		columns: 9,
		want: []string{
			"╭─┬┬╮",
			"│ │││",
			"├─┼┼┤",
			"│ │││",
			"╰─┴┴╯",
		},
	}, {
		columns: 14,
		want: []string{
			"╭───┬───┬╮",
			"│ … │ … ││",
			"├───┼───┼┤",
			"│ … │ … ││",
			"╰───┴───┴╯",
		},
	}, {
		columns: 20,
		want: []string{
			"╭─────┬───────┬╮",
			"│ Lo… │ Inpu… ││",
			"├─────┼───────┼┤",
			"│ cv… │ 12345 ││",
			"╰─────┴───────┴╯",
		},
	}, {
		columns: 24,
		want: []string{
			"╭──────┬──────────┬╮",
			"│ Loc… │ Input V… ││",
			"├──────┼──────────┼┤",
			"│ cv.… │ 12345    ││",
			"╰──────┴──────────┴╯",
		},
	}, {
		columns: 30,
		want: []string{
			"╭──────────┬────────────┬╮",
			"│ Location │ Input Val… ││",
			"├──────────┼────────────┼┤",
			"│ cv.phone │ 12345      ││",
			"╰──────────┴────────────┴╯",
		},
	}, {
		// Wide enough for the Explanation to survive, which is the shape every
		// golden captures.
		columns: 40,
		want: []string{
			"╭──────────┬─────────────┬─────────╮",
			"│ Location │ Input Value │ Explan… │",
			"├──────────┼─────────────┼─────────┤",
			"│ cv.phone │ 12345       │ Input   │",
			"│          │             │ should  │",
			"│          │             │ be a    │",
			"│          │             │ valid   │",
			"│          │             │ string. │",
			"╰──────────┴─────────────┴─────────╯",
		},
	}}

	for _, test := range tests {
		t.Run(strconv.Itoa(test.columns), func(t *testing.T) {
			t.Setenv("COLUMNS", strconv.Itoa(test.columns))

			// The table is laid out at the panel's inner width, which is what
			// `validationPanel` passes it.
			got := strings.Split(strings.TrimRight(
				Table(columns, rows, ConsoleWidth()-4), "\n"), "\n")
			if strings.Join(got, "\n") != strings.Join(test.want, "\n") {
				t.Errorf("table at %d columns:\n got %q\nwant %q", test.columns, got, test.want)
			}
		})
	}
}

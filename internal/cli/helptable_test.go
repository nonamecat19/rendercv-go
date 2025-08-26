package cli

import (
	"slices"
	"testing"
)

// optionColumns is an `Options` panel's six default columns, and argumentColumns
// the seven an `Arguments` panel gets once some parameter is required.
func optionColumns() []helpColumn {
	return []helpColumn{{}, {}, {}, {}, {}, {Flexible: true}}
}

func argumentColumns() []helpColumn {
	return []helpColumn{{}, {}, {}, {}, {}, {}, {Flexible: true}}
}

// TestHelpTableWidths is spec 012 help.md acceptance criterion 4: the seven
// panels whose widths were traced out of `rich.table.Table` while the vendored
// CLI ran.
//
// **A layout that distributed the slack instead of collapsing would pass no
// row here and would look entirely plausible.** That is the mistake an earlier
// attempt made, so these numbers are the gate.
func TestHelpTableWidths(t *testing.T) {
	cases := []struct {
		name    string
		columns []helpColumn
		rows    [][]helpCell
		want    []int
	}{
		{
			name:    "root Options",
			columns: optionColumns(),
			rows: [][]helpCell{
				{plain("--version"), plain("-v"), plain(""), plain(""), plain(""), plain("Show the version")},
				{plain("--install-completion"), plain(""), plain(""), plain(""), plain(""), plain("Install completion for the current shell.")},
				{plain("--help"), plain("-h"), plain(""), plain(""), plain(""), plain("Show this message and exit.")},
			},
			want: []int{21, 4, 2, 2, 2, 45},
		},
		{
			name:    "create-theme Options",
			columns: optionColumns(),
			rows: [][]helpCell{
				{plain("--help"), plain("-h"), plain(""), plain(""), plain(""), plain("Show this message and exit.")},
			},
			want: []int{7, 4, 2, 2, 2, 59},
		},
		{
			name:    "render Options",
			columns: optionColumns(),
			rows: [][]helpCell{
				{plain("--dont-generate-markdown"), plain("-nomd"), plain(""), plain(""), plain(""), plain("…")},
				{plain("--html-path"), plain("-nohtml"), plain(""), plain(""), plain("PATH"), plain("…")},
			},
			want: []int{25, 9, 2, 2, 6, 32},
		},
		{
			name:    "new Options",
			columns: optionColumns(),
			rows: [][]helpCell{
				{plain("--create-markdown-templates"), plain(""), plain(""), plain(""), plain(""), plain("…")},
				{plain("--theme"), plain("-h"), plain(""), plain(""), plain("TEXT"), plain("…")},
			},
			want: []int{28, 4, 2, 2, 6, 34},
		},
		{
			name:    "render Arguments",
			columns: argumentColumns(),
			rows: [][]helpCell{
				{plain("*"), plain(""), plain("input_file_name"), plain(""), plain(""), plain("PATH"), plain("…")},
			},
			want: []int{2, 2, 17, 2, 2, 6, 45},
		},
		{
			name:    "new Arguments",
			columns: argumentColumns(),
			rows: [][]helpCell{
				{plain("*"), plain(""), plain("full_name"), plain(""), plain(""), plain("TEXT"), plain("…")},
			},
			want: []int{2, 2, 11, 2, 2, 6, 51},
		},
		{
			name:    "create-theme Arguments",
			columns: argumentColumns(),
			rows: [][]helpCell{
				{plain("*"), plain(""), plain("theme_name"), plain(""), plain(""), plain("TEXT"), plain("…")},
			},
			want: []int{2, 2, 12, 2, 2, 6, 50},
		},
		{
			// The `Commands` panel is the one that declares its columns: a
			// fixed, `no_wrap` first and a `ratio=10` second. The same collapse
			// rule produces its widths too.
			name:    "root Commands",
			columns: []helpColumn{{Width: 12, NoWrap: true}, {Flexible: true}},
			rows: [][]helpCell{
				{plain("create-theme"), plain("…")},
				{plain("new"), plain("…")},
			},
			want: []int{13, 63},
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			got := helpTableWidths(row.columns, row.rows, PanelWidth-4)
			if !slices.Equal(got, row.want) {
				t.Errorf("widths = %v, want %v", got, row.want)
			}
			if total := sum(got); total != PanelWidth-4 {
				t.Errorf("widths sum to %d, want %d", total, PanelWidth-4)
			}
		})
	}
}

// TestHelpTableRowOffsets checks the other half of the model: where each
// column's text actually begins. help.md §3.4 predicts these from the widths,
// and all five agree with the golden bytes.
func TestHelpTableRowOffsets(t *testing.T) {
	lines := helpTable(optionColumns(), [][]helpCell{
		{plain("--help"), plain("-h"), plain(""), plain(""), plain(""), plain("Show this message and exit.")},
	}, PanelWidth-4)

	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1: %#v", len(lines), lines)
	}
	// `cli_create_theme_help` line 10, verbatim from the golden.
	want := "--help  -h        Show this message and exit."
	if lines[0] != want {
		t.Errorf("line = %q\nwant   %q", lines[0], want)
	}
}

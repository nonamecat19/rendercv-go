package cli

import (
	"slices"
	"testing"
)

// TestHelpColumns is spec 012 help.md §4 and acceptance criterion 5.
//
// The two cases that matter are the two the goldens disagree about: a short
// pair shares a line, a long one stacks. Both expectations are lifted from the
// golden bytes, and the column widths from the traced layout of help.md §3.3.
func TestHelpColumns(t *testing.T) {
	cases := []struct {
		name  string
		items []string
		width int
		lines []string
	}{
		{
			// `cli_render_help` line 7, in the 44-cell help column of the
			// `Arguments` panel. 20 + 1 + 10 fits, so the two share a line.
			name:  "a short pair shares a line",
			items: []string{"The YAML input file.", "[required]"},
			width: 44,
			lines: []string{"The YAML input file. [required]"},
		},
		{
			// `cli_create_theme_help` line 7, help column 44 wide.
			name:  "theme name and required",
			items: []string{"The name of the new theme", "[required]"},
			width: 44,
			lines: []string{"The name of the new theme [required]"},
		},
		{
			// `cli_new_help` lines 10-15, help column 33 wide. The prose alone
			// fills the column, so `[default: classic]` drops to its own line
			// rather than joining the last one.
			name: "a long pair stacks",
			items: []string{
				"The name of the theme (available themes are: classic, ember," +
					" engineeringclassic, engineeringresumes, harvard, ink, moderncv, opal," +
					" sb2nov)",
				"[default: classic]",
			},
			width: 33,
			lines: []string{
				"The name of the theme (available",
				"themes are: classic, ember,",
				"engineeringclassic,",
				"engineeringresumes, harvard, ink,",
				"moderncv, opal, sb2nov)",
				"[default: classic]",
			},
		},
		{
			// The common case: one item, wrapped to the column.
			name:  "a single item wraps to the column",
			items: []string{"Show completion for the current shell, to copy it or customize the installation."},
			width: 44,
			lines: []string{
				"Show completion for the current shell, to",
				"copy it or customize the installation.",
			},
		},
		{
			name:  "a single short item is one line",
			items: []string{"Show this message and exit."},
			width: 58,
			lines: []string{"Show this message and exit."},
		},
		{
			name:  "no items",
			items: nil,
			width: 44,
			lines: nil,
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			lines := HelpColumns(row.items, row.width)
			if !slices.Equal(lines, row.lines) {
				t.Errorf("lines =\n%#v\nwant\n%#v", lines, row.lines)
			}
		})
	}
}

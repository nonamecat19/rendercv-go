package cli

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

// `new`'s greeting is a `rich.print` and wraps like everything else.
//
// `print_welcome.py:13` prints `f"\nWelcome to [dodger_blue3]RenderCV
// v{__version__}[/dodger_blue3]!\n"`, and `Console.print` lays a plain string
// out at the console width under the default overflow, `"fold"`. The port wrote
// it with a bare `Fprintf`, so at 20 columns it printed a 25-column line where
// upstream prints two — the one CLI surface outside a box, and the only one no
// border made obvious.
//
// The four-column case is the whitespace rule again: `RenderCV ` folds to
// `Rend`, `erCV` and a line holding just the space that followed it
// (`rich/_wrap.py:59`).
//
// Expectations are upstream's own `rendercv new "John Doe"`, captured from the
// vendored CLI at the width named.
func TestNewGreetingWraps(t *testing.T) {
	tests := []struct {
		columns int
		want    []string
	}{{
		columns: 4,
		want:    []string{"", "Welc", "ome ", "to ", "Rend", "erCV", " ", "v2.8", "!", ""},
	}, {
		columns: 9,
		want:    []string{"", "Welcome ", "to ", "RenderCV ", "v2.8!", ""},
	}, {
		columns: 14,
		want:    []string{"", "Welcome to ", "RenderCV v2.8!", ""},
	}, {
		columns: 20,
		want:    []string{"", "Welcome to RenderCV ", "v2.8!", ""},
	}, {
		// Wide enough to fit, which is where every golden is captured.
		columns: 80,
		want:    []string{"", "Welcome to RenderCV v2.8!", ""},
	}}

	for _, test := range tests {
		t.Run(strconv.Itoa(test.columns), func(t *testing.T) {
			t.Setenv("COLUMNS", strconv.Itoa(test.columns))

			// A pipe, which is what every golden was captured through and
			// what leaves the greeting free of escape sequences.
			banner := newBanner("John_Doe_CV.yaml", true, nil, Terminal{})
			got := splitLines(banner)
			got = got[:min(len(got), len(test.want))]
			if !slices.Equal(got, test.want) {
				t.Errorf("greeting at %d columns:\n got %q\nwant %q", test.columns, got, test.want)
			}
			if !strings.HasPrefix(banner, "\n") {
				t.Errorf("banner does not open with Rich's blank line: %q", banner[:1])
			}
		})
	}
}

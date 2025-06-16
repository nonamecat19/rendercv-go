package process_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

var (
	noteDate    = time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC)
	noteCatalog = func() process.Catalog {
		catalog := english
		catalog.LastUpdated = "Last updated in"
		return catalog
	}()
)

// The top note, measured against the vendored Python.
func TestRenderTopNote(t *testing.T) {
	tests := map[string]string{
		// The classic theme's default.
		"*LAST_UPDATED CURRENT_DATE*": "*Last updated in Mar 2025*",
		"NAME":                        "John Doe",
		"LAST_UPDATED":                "Last updated in",
		// All eight date placeholders are here too, not only CURRENT_DATE.
		"YEAR-MONTH_IN_TWO_DIGITS": "2025-03",
		// **The footer's placeholders are not in this map**, so they stay
		// literal. A shared map would silently make them work here.
		"PAGE_NUMBER": "PAGE_NUMBER",
	}

	for template, want := range tests {
		t.Run(template, func(t *testing.T) {
			got := process.RenderTopNote(template, noteCatalog, noteDate, "John Doe",
				templates.SingleDate, nil)
			if got != want {
				t.Errorf("= %q, want %q", got, want)
			}
		})
	}
}

// The footer, including the wrapper's exact spacing.
func TestRenderFooter(t *testing.T) {
	tests := map[string]string{
		"*NAME -- PAGE_NUMBER/TOTAL_PAGES*": "context { [*John Doe -- " +
			"#str(here().page())/#str(counter(page).final().first())*] }",
		"CURRENT_DATE": "context { [Mar 2025] }",
		// And the mirror of the top note's last row.
		"LAST_UPDATED": "context { [LAST_UPDATED] }",
	}

	for template, want := range tests {
		t.Run(template, func(t *testing.T) {
			got := process.RenderFooter(template, noteCatalog, noteDate, "John Doe",
				templates.SingleDate, nil)
			if got != want {
				t.Errorf("= %q, want %q", got, want)
			}
		})
	}
}

// An absent name substitutes the empty string rather than leaving the
// placeholder — and `substitute_placeholders`' strip then removes what it left
// behind.
func TestAnAbsentNameIsEmpty(t *testing.T) {
	got := process.RenderFooter("NAME!", noteCatalog, noteDate, "", templates.SingleDate, nil)
	if got != "context { [!] }" {
		t.Errorf("= %q, want %q", got, "context { [!] }")
	}
}

// **The cross-module interaction of spec 008 §4D behavior 43.**
//
// `PAGE_NUMBER` substitutes to Typst source *before* the string processors run,
// so `EscapeTypstCharacters` sees a `#`-command and must hold it out. Without
// its first phase every page footer would print `\#str(here().page())`.
//
// This test belongs here rather than in either module: neither one is wrong on
// its own, and the pair is what ships.
func TestFooterPageCountersSurviveEscaping(t *testing.T) {
	got := process.RenderFooter("PAGE_NUMBER/TOTAL_PAGES", noteCatalog, noteDate, "",
		templates.SingleDate,
		[]process.StringProcessor{process.EscapeTypstCharacters})

	// The `/` between the two commands **is** escaped — it is not part of
	// either one. Measured, and it is why this test asserts the whole string
	// rather than only that the commands survived.
	const want = `context { [#str(here().page())\/#str(counter(page).final().first())] }`
	if got != want {
		t.Errorf("= %q, want %q", got, want)
	}
	if strings.Contains(got, `\#`) {
		t.Error("the page counters were escaped; the escape's first phase did not hold them out")
	}
}

// Substitute first, process second. Reversing the two escapes the placeholder
// *names* and the values never land.
func TestPlaceholderValuesAreProcessed(t *testing.T) {
	got := process.RenderTopNote("NAME", noteCatalog, noteDate, "a_b",
		templates.SingleDate,
		[]process.StringProcessor{process.EscapeTypstCharacters})
	if got != `a\_b` {
		t.Errorf("= %q, want %q — the value must go through the processors", got, `a\_b`)
	}
}

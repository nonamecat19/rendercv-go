//go:build conformance

package locale_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/conformance"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
)

const (
	upstreamDir = "../../../../third_party/rendercv"
	interpreter = upstreamDir + "/.venv/bin/python"
	driver      = "testdata/render_locales.py"
)

// upstreamRenders is testdata/render_locales.py's output.
type upstreamRenders struct {
	Date      string                       `json:"date"`
	Locales   []string                     `json:"locales"`
	Documents map[string]map[string]string `json:"documents"`
}

// The rendered document, for every locale, against the vendored Python.
//
// **This is the axis a previous pass called verified without rendering
// anything.** "22/22 locales byte-identical" was measured on the catalog data
// and on the starter CV `new` writes; neither prints a month, so a wrong
// `month_names[7]` could not have failed either. Here the catalog reaches
// `.typ`, `.md` and `.html` through the real templater, and the comparison is
// against a live process rather than a stored digest — a submodule bump moves
// the expectation on the next run.
//
// Reachability is a separate, untagged test
// (TestEveryCatalogStringReachesTheDocument): this one would still pass on a
// fixture that had quietly stopped exercising the catalogs, because both sides
// would stop together.
//
// It costs about 1.2 seconds: one interpreter start and twenty-two renders,
// with the port's twenty-two in process.
func TestRenderedDocumentsMatchUpstreamForEveryLocale(t *testing.T) {
	// The submodule's own interpreter, by path: `uv run … rendercv` from
	// outside the submodule resolves to whatever is on PATH. Its absence is
	// fatal unless the documented opt-out is set, the same arrangement every
	// other input in this suite uses.
	conformance.RequireInput(t, interpreter, "the locale differential",
		"run `just submodule` to check out third_party/rendercv")

	before := time.Now().Format(time.DateOnly)

	command := exec.Command(mustAbs(t, interpreter), mustAbs(t, driver), mustAbs(t, matrixFixture))
	command.Dir = mustAbs(t, upstreamDir)
	command.Stderr = os.Stderr
	raw, err := command.Output()
	if err != nil {
		t.Fatalf("drive the vendored rendercv: %v", err)
	}

	var upstream upstreamRenders
	if err := json.Unmarshal(raw, &upstream); err != nil {
		t.Fatal(err)
	}

	// **The footer carries `locale.last_updated` beside today's date**, so a run
	// that straddles midnight would report a parity failure that is nothing of
	// the sort. Both dates are checked rather than the difference tolerated: a
	// tolerated footer is a hole exactly where `last_updated` is verified.
	if after := time.Now().Format(time.DateOnly); before != after || before != upstream.Date {
		t.Skipf("the run crossed midnight (%s, %s, upstream %s); rerun",
			before, after, upstream.Date)
	}

	if got, want := upstream.Locales, locale.AvailableLocales(); !equal(got, want) {
		t.Fatalf("upstream ships %v, the port ships %v", got, want)
	}

	for _, language := range upstream.Locales {
		t.Run(language, func(t *testing.T) {
			got := renderForLocale(t, language)
			for _, format := range []string{"typst", "markdown", "html"} {
				if got[format] != upstream.Documents[language][format] {
					t.Errorf("%s differs at %s", format,
						firstDifference(got[format], upstream.Documents[language][format]))
				}
			}
		})
	}
}

// firstDifference locates a mismatch for the failure message; the documents run
// to hundreds of lines and printing both is unreadable.
func firstDifference(got, want string) string {
	gotLines := strings.SplitAfter(got, "\n")
	wantLines := strings.SplitAfter(want, "\n")
	for i := range min(len(gotLines), len(wantLines)) {
		if gotLines[i] != wantLines[i] {
			return fmt.Sprintf("line %d:\n  port     = %q\n  upstream = %q",
				i+1, gotLines[i], wantLines[i])
		}
	}
	return fmt.Sprintf("line %d: the port has %d lines, upstream has %d",
		min(len(gotLines), len(wantLines))+1, len(gotLines), len(wantLines))
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return absolute
}

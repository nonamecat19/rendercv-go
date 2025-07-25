//go:build conformance

package process_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// Differential against python-markdown, over the shapes a fresh-context verifier
// found the port getting wrong. The fixture is CPython's own output.
//
// **This is red on purpose**, and it pins an open blocker rather than a bug
// being fixed in the same commit: goldmark escapes `"` as `&quot;` and
// python-markdown does not, so any double quote in a CV produces a differing
// `.html`. Raw HTML — the other half of the verifier's finding — now passes,
// because `WithUnsafe` matches python-markdown's passthrough.
//
// It lives behind the conformance tag so `go test ./...` stays green while the
// blocker is open, which is where this repo keeps its red-by-design cases.
func TestMarkdownToHTMLMatchesPython(t *testing.T) {
	raw, err := os.ReadFile("testdata/html.json")
	if err != nil {
		t.Skipf("no fixture: %v", err)
	}
	var rows []struct{ In, Out string }
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		got, err := process.MarkdownToHTML(row.In)
		if err != nil {
			t.Fatalf("%q: %v", row.In, err)
		}
		if got != row.Out {
			t.Errorf("MarkdownToHTML(%q)\n = %q\nwant %q", row.In, got, row.Out)
		}
	}
}

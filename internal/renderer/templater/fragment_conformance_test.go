//go:build conformance

package templater_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/flosch/pongo2/v6"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater"
)

// The fragment differential: upstream's Jinja templates rendered by Jinja,
// against the transformed pongo2 templates rendered by pongo2, over the same
// plain-dictionary contexts.
//
// **This is the gate spec 008 §8 said was one iteration away, and it was not.**
// The claim was that only a corpus `.typ` could check `plan.md` §2's transform,
// and that a corpus `.typ` needs iteration 9's model bridge. The second half is
// true; the first is not. A fragment needs no bridge, no effective theme tree
// and no typst — only a dictionary — and this test found a transform bug in its
// first run that the parse test and the twenty-one unit criteria all missed:
// pongo2 does not strip a template's final newline and Jinja does, so every
// fragment gained one and every entry and section gained a blank line.
//
// A `rendercv-parity-verifier` pass is what pointed this out. Recorded here
// rather than only in the ledger, because the next person to write "only the
// golden can check this" should see how that went.
func TestFragmentsMatchJinja(t *testing.T) {
	var rows []struct {
		Template string         `json:"template"`
		Context  map[string]any `json:"context"`
		Want     string         `json:"want"`
	}

	path := filepath.Join("testdata", "fragments.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	if len(rows) < 50 {
		t.Fatalf("%d rows, want the full fixture", len(rows))
	}

	templates := templater.BuiltinTemplates()
	if _, err := templater.NewEnvironment("", templates, "classic"); err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}

	for i, row := range rows {
		t.Run(row.Template+"#"+itoa(i), func(t *testing.T) {
			source, err := fs.ReadFile(templates, row.Template)
			if err != nil {
				t.Fatalf("reading %s: %v", row.Template, err)
			}

			template, err := pongo2.FromString(string(source))
			if err != nil {
				t.Fatalf("parsing %s: %v", row.Template, err)
			}

			got, err := template.Execute(withSplitLines(row.Context))
			if err != nil {
				t.Fatalf("rendering %s: %v", row.Template, err)
			}
			if got != row.Want {
				t.Errorf("=\n%q\nwant\n%q", got, row.Want)
			}
		})
	}
}

// withSplitLines adds the `…_lines` fields the transform's `splitlines` rule
// expects. Upstream's Jinja calls `.splitlines()` on the string; the port
// pre-splits, so the fixture's plain context has to gain them here — which is
// itself part of what the differential checks, since a wrong split shows up as a
// wrong render.
//
// Python's `str.splitlines()` on an **empty** string is `[]`, not `[""]`, and
// `EducationEntry` branches on that length being zero. `strings.Split` returns
// `[""]`, so the empty case is special-cased rather than inherited.
func withSplitLines(context map[string]any) pongo2.Context {
	out := make(pongo2.Context, len(context))
	for key, value := range context {
		nested, ok := value.(map[string]any)
		if !ok {
			out[key] = value
			continue
		}

		copied := make(map[string]any, len(nested)*2)
		for name, inner := range nested {
			copied[name] = inner
			text, isText := inner.(string)
			if !isText {
				continue
			}
			copied[name+"_lines"] = splitLines(text)
		}
		out[key] = copied
	}
	return out
}

func splitLines(text string) []string {
	if text == "" {
		return []string{}
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

func itoa(i int) string { return strconv.Itoa(i) }

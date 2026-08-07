package templater_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/flosch/pongo2/v6"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater"
)

// **Every generated template must parse.** The transform is a text rewrite with
// no syntax check of its own, so a rule that produces something pongo2 cannot
// read would otherwise surface as a runtime error on one corpus case — and only
// on the case that reaches that template.
//
// This is not a semantic check. Nothing here says a template still *means* what
// upstream's did; the golden byte diff of Wave C is the only thing that can.
func TestEveryGeneratedTemplateParses(t *testing.T) {
	if _, err := templater.NewEnvironment("", templater.BuiltinTemplates(), "classic"); err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}

	templates := templater.BuiltinTemplates()
	count := 0

	err := fs.WalkDir(templates, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		count++

		t.Run(path, func(t *testing.T) {
			source, err := fs.ReadFile(templates, path)
			if err != nil {
				t.Fatalf("reading: %v", err)
			}
			if _, err := pongo2.FromString(string(source)); err != nil {
				t.Errorf("does not parse: %v", err)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking: %v", err)
	}

	// Upstream ships 26: thirteen Typst, twelve Markdown and one HTML. Fewer
	// means the generator skipped some.
	if count != 26 {
		t.Errorf("%d templates, want 26", count)
	}
}

// The transform's output must contain none of the constructs it exists to
// remove. A rule that silently stopped firing would otherwise leave Jinja syntax
// in the tree, and only the template that used it would fail.
func TestNoJinjaConstructsSurvive(t *testing.T) {
	forbidden := map[string]string{
		".splitlines()": "the pre-split `…_lines` fields replace it",
		".as_rgb()":     "the model stores the rendered colour",
		"|indent(":      "pongo2 takes filter arguments with `:`",
		"|replace(":     "pongo2 has no two-argument replace",
		//nolint:gocritic // the leading space is the construct, not a typo.
		" in [": "pongo2 has no list literal",
	}

	templates := templater.BuiltinTemplates()
	err := fs.WalkDir(templates, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		source, err := fs.ReadFile(templates, path)
		if err != nil {
			return err
		}
		for construct, why := range forbidden {
			if strings.Contains(string(source), construct) {
				t.Errorf("%s still contains %q — %s", path, construct, why)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking: %v", err)
	}
}

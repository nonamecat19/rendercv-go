//go:build conformance

package design_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
)

const overridesDir = "../../../../third_party/rendercv/src/rendercv/schema/models/design/other_themes"

// `Themes()` is in **upstream's union order**, derived from the glob rather than
// from the Go data it is checking.
//
// `discover_other_themes` globs `other_themes/*.yaml` and sorts by filename
// (built_in_design.py:13-38), and `BuiltInDesign` puts `ClassicTheme` ahead of
// the reduction. The order decides the `$defs` collision numbering, which is the
// one thing about the design `$defs` that cannot be read off the output — the
// keys sort alphabetically whatever number they carry — so a submodule bump that
// added a theme would otherwise surface as many byte failures naming no cause.
//
// The same test iteration 7 wrote for locale, for the same reason.
func TestThemesAreInUnionOrder(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(overridesDir, "*.yaml"))
	if err != nil {
		t.Fatalf("globbing %s: %v", overridesDir, err)
	}
	if len(files) == 0 {
		t.Fatalf("no override files under %s — is the submodule initialized? `just setup`", overridesDir)
	}
	sort.Strings(files)

	want := []string{"classic"}
	for _, file := range files {
		want = append(want, strings.TrimSuffix(filepath.Base(file), ".yaml"))
	}

	got := design.Themes()
	if len(got) != len(want) {
		t.Fatalf("Themes() = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Themes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// Every override file, key for key, against the submodule (spec 006 §5
// criterion 6).
//
// `TestThemesAreInUnionOrder` above checks the filenames; this checks what is
// inside them. Without it the criterion's box was ticked by a test that could
// not see a changed colour.
//
// **It shares a parser with the tool that generated the data** — both read the
// YAML with `goccy/go-yaml` — which `tools/designprobe`'s head states. So this
// is drift detection after a submodule bump, not a check on a human copy. The
// stronger gate on the same data is the 161-row `$defs` differential, which
// compares against a file the port never wrote.
func TestOverridesMatchTheSubmodule(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(overridesDir, "*.yaml"))
	if err != nil {
		t.Fatalf("globbing %s: %v", overridesDir, err)
	}
	if len(files) != 8 {
		t.Fatalf("%d override files, want 8 — is the submodule initialized? `just setup`", len(files))
	}

	for _, file := range files {
		stem := strings.TrimSuffix(filepath.Base(file), ".yaml")
		t.Run(stem, func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("reading %s: %v", file, err)
			}
			var document struct {
				Design map[string]any `yaml:"design"`
			}
			if err := yaml.Unmarshal(raw, &document); err != nil {
				t.Fatalf("parsing %s: %v", file, err)
			}

			compare(t, stem, document.Design, design.Overrides(stem))
		})
	}
}

// compare walks both mappings, so a key present on one side and not the other
// fails rather than being skipped.
func compare(t *testing.T, path string, want, got map[string]any) {
	t.Helper()

	for key, wanted := range want {
		mine, present := got[key]
		if !present {
			t.Errorf("%s.%s is in the submodule and not in the port", path, key)
			continue
		}
		switch typed := wanted.(type) {
		case map[string]any:
			nested, ok := mine.(map[string]any)
			if !ok {
				t.Errorf("%s.%s is a mapping upstream and a %T here", path, key, mine)
				continue
			}
			compare(t, path+"."+key, typed, nested)
		case []any:
			list, ok := mine.([]string)
			if !ok || len(list) != len(typed) {
				t.Errorf("%s.%s = %v, want %v", path, key, mine, typed)
				continue
			}
			for i := range typed {
				if list[i] != typed[i] {
					t.Errorf("%s.%s[%d] = %q, want %v", path, key, i, list[i], typed[i])
				}
			}
		default:
			if mine != wanted {
				t.Errorf("%s.%s = %#v, want %#v", path, key, mine, wanted)
			}
		}
	}

	for key := range got {
		if _, present := want[key]; !present {
			t.Errorf("%s.%s is in the port and not in the submodule", path, key)
		}
	}
}

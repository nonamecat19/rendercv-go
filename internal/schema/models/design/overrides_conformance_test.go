//go:build conformance

package design_test

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

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

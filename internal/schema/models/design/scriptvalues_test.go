package design_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// A scripted theme's document values go through the base tree, which upstream
// gets from `theme_data_model_class(**design)` (`design.py:135`) — the script's
// class declares the same fields with the same types, so a value the built-in
// tree rejects is rejected here too. The port used to return unconditionally
// for every custom theme and render `page-size: "bogus"` into the artifact at
// exit 0.
//
// **The reported location is upstream's, which is not the true one for a
// top-level key.** `pydantic_error_handling.py:53-55` drops the second element
// of every `design` error's location to skip the discriminated union's tag —
// right for a built-in theme's `('design', 'classic', 'page', 'size')`, and
// wrong for a scripted theme's, whose error is raised inside the wrap validator
// and carries no tag element. So a depth-1 failure that upstream would call
// `design.page` reports at plain `design`. That is upstream's own defect and
// reproducing it is axis-4 parity; do not "fix" it.
func TestValidateScriptedThemeValues(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantLoc     string
		wantMessage string
	}{
		{
			name:    "a valid document validates",
			yaml:    "theme: mytheme\npage:\n  size: a5\n",
			wantLoc: "",
		},
		{
			// Measured upstream: exit 1, `Input should be 'a4', 'a5',
			// 'us-letter' or 'us-executive'`. Upstream's *panel* never
			// appears — the location strip above turns
			// `('design','page','size')` into `('design','size')`, which does
			// not resolve in the YAML, and its error formatter dies with
			// `RenderCVInternalError: Key 'size' not found in the YAML file.`
			// The port matches the exit code and the refusal to render, and
			// prints its own clean record at the true location instead of a
			// traceback — the D-011 substitution.
			name:        "a bad literal on a tree key",
			yaml:        "theme: mytheme\npage:\n  size: bogus\n",
			wantLoc:     "design.page.size",
			wantMessage: "Input should be 'a4', 'a5', 'us-letter' or 'us-executive'",
		},
		{
			name:        "a bad colour on a tree key",
			yaml:        "theme: mytheme\ncolors:\n  name: notacolor\n",
			wantLoc:     "design.colors.name",
			wantMessage: "value is not a valid color",
		},
		{
			// Depth 1, so this one upstream *does* print, at `design`.
			// Measured: `Input should be a valid dictionary or instance of
			// Page.` with input `notamapping`.
			name:        "a top-level shape failure reports at design",
			yaml:        "theme: mytheme\npage: notamapping\n",
			wantLoc:     "design",
			wantMessage: "Input should be a valid dictionary or instance of Page",
		},
		{
			// A key the tree never declared is the script's own business.
			// Forbidding the ones the script did not declare either is a
			// separate behavior; nothing here may reject this one.
			name:    "a script-invented key is not judged by the tree",
			yaml:    "theme: mytheme\ncustom_note: world\n",
			wantLoc: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := validateTheme(t, "mytheme", `return { custom_note = "hello" }`, test.yaml)

			if test.wantLoc == "" {
				if len(errs) != 0 {
					t.Fatalf("errs = %+v, want none", errs)
				}
				return
			}
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if got := strings.Join(errs[0].SchemaLocation, "."); got != test.wantLoc {
				t.Errorf("location = %q, want %q", got, test.wantLoc)
			}
			if !strings.Contains(errs[0].Message, test.wantMessage) {
				t.Errorf("message = %q, want it to contain %q", errs[0].Message, test.wantMessage)
			}
		})
	}
}

// **A theme folder with no script validates nothing at all.** Upstream's
// no-module fallback builds `ThemeOptionsAreNotProvided(theme=theme_name)`
// (`design.py:139-142`) — the document's `design` block is never passed to it,
// so nothing in the block is judged. Measured against the vendored binary:
// `page: {size: bogus}` on a script-less theme folder is **exit 0**, the same
// document that is exit 1 once an `__init__.py` appears beside it.
//
// This is why the loader reports `Exists` separately from its options: gating
// on "are there options" would validate a script-less theme the moment someone
// made the loader return an empty map instead of nil.
func TestValidateWithoutAScriptJudgesNothing(t *testing.T) {
	errs := validateTheme(t, "mytheme", "", "theme: mytheme\npage:\n  size: bogus\n")

	if len(errs) != 0 {
		t.Errorf("errs = %+v, want none — a script-less theme validates nothing", errs)
	}
}

// A script that failed to load declares no shape, so there is nothing to judge
// the document against and the document must not be judged against the tree
// either — the theme still has a script, but the port could not read it.
func TestValidateWithABrokenScriptJudgesNothing(t *testing.T) {
	errs := validateTheme(t, "mytheme", "return {", "theme: mytheme\npage:\n  size: bogus\n")

	if len(errs) != 0 {
		t.Errorf("errs = %+v, want none — a broken script is another unit's report", errs)
	}
}

// **Two failures at depth 1 produce one record.** Upstream's location strip
// collapses both to `('design',)` and its deduplication keys on the schema
// location alone (`pydantic_error_handling.py:167-176`), keeping the first, so
// the second is dropped rather than printed. Measured: `colors: notamapping`
// plus `page: notamapping2` prints exactly one row — `page`'s, because pydantic
// reports in **field declaration order** and `page` precedes `colors` in the
// class, not in document order.
func TestValidateScriptedThemeCollapsesDepthOneRecords(t *testing.T) {
	errs := validateTheme(t, "mytheme", "return {}",
		"theme: mytheme\ncolors: notamapping\npage: notamapping2\n")

	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one — both collapse to design", errs)
	}
	if got := strings.Join(errs[0].SchemaLocation, "."); got != "design" {
		t.Errorf("location = %q, want design", got)
	}
	if errs[0].Input != "notamapping2" {
		t.Errorf("input = %q, want notamapping2 — page is declared before colors", errs[0].Input)
	}
}

// The built-in path must not move: it reports the true location, because a
// built-in theme's error really does carry the discriminator element that
// upstream's strip is there to remove.
func TestValidateBuiltInThemeLocationIsUnchanged(t *testing.T) {
	errs := validateTheme(t, "mytheme", "return {}", "theme: classic\npage:\n  size: bogus\n")

	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if got := strings.Join(errs[0].SchemaLocation, "."); got != "design.page.size" {
		t.Errorf("location = %q, want design.page.size", got)
	}
}

// A theme whose folder is missing still reports the folder message and never
// reaches the value checks — the two folder checks run first upstream
// (`design.py:72-88`), before the module is imported.
func TestValidateMissingFolderStillWins(t *testing.T) {
	input := writeTheme(t, "mytheme", "return {}")
	doc, err := yamlreader.ReadString("theme: othertheme\npage:\n  size: bogus\n")
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	node := &yamldoc.Node{Kind: yamldoc.KindMapping, Items: doc.Items}

	errs := design.Validate(node, []string{"design"}, schemaerr.SourceMain,
		&valctx.ValidationContext{InputFilePath: filepath.Join(filepath.Dir(input), "CV.yaml")})

	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if !strings.Contains(errs[0].Message, "does not exist") {
		t.Errorf("message = %q, want the folder message", errs[0].Message)
	}
}

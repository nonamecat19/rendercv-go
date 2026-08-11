package design_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// validateTheme runs `Validate` over a `design` block written beside a theme
// folder, which is the only way a scripted theme's script is reachable.
func validateTheme(t *testing.T, theme, script, yaml string) []schemaerr.ValidationError {
	t.Helper()
	input := writeTheme(t, theme, script)
	doc, err := yamlreader.ReadString(yaml)
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	node := &yamldoc.Node{Kind: yamldoc.KindMapping, Items: doc.Items}
	return design.Validate(node, []string{"design"}, schemaerr.SourceMain,
		&valctx.ValidationContext{InputFilePath: input})
}

// An unrecognised key in a scripted theme's `design` block is exit 1 upstream:
// the class `create-theme` generates inherits `BaseModelWithoutExtraKeys`
// (`base.py:5`, `extra="forbid"`), the same base the built-in tree uses, and
// `theme_data_model_class(**design)` (`design.py:135`) hands it the whole
// block. The port accepted it.
//
// **`extra="forbid"` applies to the union of the tree's fields and the
// script's**, which is the whole point of a scripted theme: measured against
// the vendored binary with a real `__init__.py` declaring `custom_note`,
// `design: {theme: mytheme, custom_note: world}` is exit 0 and the same
// document with `undeclared_key: x` is exit 1. A fix that ran a custom theme
// through the built-in tree alone would reject every option a script
// legitimately adds.
func TestValidateScriptedThemeUnknownKeys(t *testing.T) {
	const script = `return { custom_note = "hello", page = { custom_page_option = 1 } }`

	tests := []struct {
		name    string
		yaml    string
		wantLoc string
	}{
		{
			name:    "a tree key is known",
			yaml:    "theme: mytheme\npage:\n  size: a5\n",
			wantLoc: "",
		},
		{
			name:    "a script-declared key is known",
			yaml:    "theme: mytheme\ncustom_note: world\n",
			wantLoc: "",
		},
		{
			name:    "a script-declared key nested under a tree key is known",
			yaml:    "theme: mytheme\npage:\n  custom_page_option: 2\n",
			wantLoc: "",
		},
		{
			// Measured upstream: exit 1, `This field is unknown for this
			// object. Please remove it.`, input `x`, at location **`design`**
			// — not `design.undeclared_key`, which is where the same document
			// against `theme: classic` reports it.
			//
			// The location is upstream's own defect and reproducing it is
			// axis-4 parity. `pydantic_error_handling.py:53-55` drops the
			// second element of every `design` error's location to skip the
			// discriminated union's tag; a scripted theme's error is raised
			// inside the wrap validator and has no tag element, so the strip
			// eats the key name instead. Do not "fix" this to the true
			// location.
			name:    "an unknown top-level key reports at design",
			yaml:    "theme: mytheme\nundeclared_key: x\n",
			wantLoc: "design",
		},
		{
			// Upstream strips this to `('design', 'unknown')`, which does not
			// resolve in the YAML, and its formatter dies —
			// `RenderCVInternalError: Key 'unknown' not found in the YAML
			// file.`, exit 1 with a traceback and no table. The port matches
			// the exit code and the refusal to render and prints its own clean
			// row at the true location rather than inventing a record upstream
			// never shows.
			name:    "an unknown nested key reports at its true location",
			yaml:    "theme: mytheme\npage:\n  unknown: 1\n",
			wantLoc: "design.page.unknown",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := validateTheme(t, "mytheme", script, test.yaml)

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
			if !strings.Contains(errs[0].Message, "Extra inputs are not permitted") {
				t.Errorf("message = %q, want pydantic's extra-key text", errs[0].Message)
			}
		})
	}
}

// The unknown key's *value* reaches the panel's Input Value column, which is
// what upstream fills from the rejected input.
func TestValidateScriptedThemeUnknownKeyInput(t *testing.T) {
	errs := validateTheme(t, "mytheme", "return {}", "theme: mytheme\nundeclared_key: x\n")

	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if errs[0].Input != "x" {
		t.Errorf("input = %q, want x", errs[0].Input)
	}
}

// **Two unknown top-level keys produce one record.** Both collapse onto
// `('design',)` and upstream deduplicates on the schema location alone
// (`pydantic_error_handling.py:167-176`), keeping the first. Measured with
// `zzz_last` written before `aaa_first`: one row, input `x` — the extra keys
// are reported in **document** order, so the one written first wins.
func TestValidateScriptedThemeCollapsesUnknownKeys(t *testing.T) {
	errs := validateTheme(t, "mytheme", "return {}",
		"theme: mytheme\nzzz_last: x\naaa_first: y\n")

	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one — both collapse to design", errs)
	}
	if got := strings.Join(errs[0].SchemaLocation, "."); got != "design" {
		t.Errorf("location = %q, want design", got)
	}
	if errs[0].Input != "x" {
		t.Errorf("input = %q, want x — the first key written wins", errs[0].Input)
	}
}

// **A theme folder with no script forbids nothing.** Upstream's fallback builds
// `ThemeOptionsAreNotProvided(theme=theme_name)` (`design.py:139-142`) without
// the document's block, so no key in it is ever judged — measured as exit 0 for
// an unknown key that is exit 1 the moment an `__init__.py` appears in the same
// folder.
func TestValidateWithoutAScriptForbidsNothing(t *testing.T) {
	errs := validateTheme(t, "mytheme", "", "theme: mytheme\nundeclared_key: x\n")

	if len(errs) != 0 {
		t.Errorf("errs = %+v, want none — a script-less theme judges nothing", errs)
	}
}

// A script that could not be loaded declares no shape, so there is no union to
// forbid against. Reporting the broken script itself is another unit's.
func TestValidateWithABrokenScriptForbidsNothing(t *testing.T) {
	errs := validateTheme(t, "mytheme", "return {", "theme: mytheme\nundeclared_key: x\n")

	if len(errs) != 0 {
		t.Errorf("errs = %+v, want none — a broken script declares no union", errs)
	}
}

// The built-in path keeps the true location: a built-in theme's error really
// does carry the discriminator element upstream's strip removes, so
// `design.undeclared_key` is both what upstream prints and what the port has
// always printed. Measured against `theme: classic`.
func TestValidateBuiltInThemeUnknownKeyIsUnchanged(t *testing.T) {
	errs := validateTheme(t, "mytheme", "return {}", "theme: classic\nundeclared_key: x\n")

	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if got := strings.Join(errs[0].SchemaLocation, "."); got != "design.undeclared_key" {
		t.Errorf("location = %q, want design.undeclared_key", got)
	}
}

package design_test

import (
	"strings"
	"testing"
)

// A broken theme script is **reported**, not discarded (spec 014 §2 behavior 9,
// tasks 014 T4/T5). Every one of these used to be indistinguishable from a
// theme folder with no script at all, so the document rendered with the theme's
// base defaults at exit 0 with no signal — a silently wrong CV from a script
// the user got wrong.
//
// **These records are raw**, so their messages carry no trailing period: they
// flow through `errorpipeline.Parse` like every other validation record now,
// and its step 8 (`appendPeriod`) is what puts it on. T4 had to append it by
// hand because the record was synthesized at render time and skipped the
// pipeline entirely; `internal/cli/themescript_test.go` pins the final,
// single-period text a user sees.
//
// **The four modes stay distinguishable**: a user needs to know whether their
// script failed to parse, returned the wrong type, declared a shape the design
// tree cannot hold, or declared a value the field rejects.
func TestABrokenThemeScriptIsReported(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
		input  string
	}{{
		name:   "a parse error",
		script: "return {",
		want: "The custom theme mytheme's init.lua file could not be run: " +
			"<string> at EOF:   syntax error",
		input: "...",
	}, {
		name:   "a runtime failure",
		script: "error('boom')",
		want: "The custom theme mytheme's init.lua file could not be run: " +
			"<string>:1: boom",
		input: "...",
	}, {
		name:   "a non-table return",
		script: "return 42",
		want:   "The custom theme mytheme's init.lua file did not return a table of theme options",
		input:  "...",
	}, {
		name:   "a shape the design tree cannot hold",
		script: `return { page = { size = { a = 1 } } }`,
		want: "The custom theme mytheme's init.lua file declares an option the design tree " +
			"cannot hold: design.page.size is a group of options in this theme's script, " +
			"but should be a value",
		input: "...",
	}, {
		// **Upstream's own sentence, unprefixed.** `theme_data_model_class(**design)`
		// validates the declared defaults (`design.py:135`), so this text and this
		// input value are parity, not this port's wording.
		name:   "a value the field rejects",
		script: `return { page = { size = "bogus" } }`,
		want:   "Input should be 'a4', 'a5', 'us-letter' or 'us-executive'",
		input:  "bogus",
	}, {
		// A Lua boolean is echoed as Lua spells it (D-013): upstream prints
		// Python's `True` for the same mistake in `__init__.py`.
		name:   "a boolean where a value belongs",
		script: `return { page = { size = true } }`,
		want:   "Input should be 'a4', 'a5', 'us-letter' or 'us-executive'",
		input:  "true",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := validateTheme(t, "mytheme", test.script, "theme: mytheme\n")

			if len(errs) != 1 {
				t.Fatalf("got %d records, want 1: %+v", len(errs), errs)
			}
			if errs[0].Message != test.want {
				t.Errorf("message = %q, want %q", errs[0].Message, test.want)
			}
			if errs[0].Input != test.input {
				t.Errorf("input = %q, want %q", errs[0].Input, test.input)
			}
			// **`design` looks too shallow and is correct.** Upstream's column
			// reads `design` for every one of these — measured against the
			// vendored binary — and the reason is a bug in its own error
			// formatter, not a property of script errors:
			// `pydantic_error_handling.py:53-55` strips path element 2 to skip
			// the theme discriminator, which a **scripted** theme does not have,
			// because its error is raised by `theme_data_model_class(**design)`
			// (`design.py:135`) inside the wrap validator. So a real segment is
			// stripped and a depth-1 error collapses to `('design',)`.
			//
			// Anything narrower here would be *less* faithful. Do not "fix" it.
			if got := strings.Join(errs[0].SchemaLocation, "."); got != "design" {
				t.Errorf("location = %q, want design", got)
			}
		})
	}
}

// **A failing script stops the document being judged.** Upstream raises out of
// `validate_design` before `theme_data_model_class(**design)` ever runs
// (`design.py:107-135`), so the block's own mistakes are never reached — one
// record, the script's, not two.
func TestABrokenScriptSuppressesDocumentValidation(t *testing.T) {
	errs := validateTheme(t, "mytheme", "return {",
		"theme: mytheme\nundeclared_key: x\npage:\n  size: bogus\n")

	if len(errs) != 1 {
		t.Fatalf("got %d records, want only the script's: %+v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Message, "could not be run") {
		t.Errorf("message = %q, want the script failure", errs[0].Message)
	}
}

// **"Absent" and "broken" take different paths**, which is the whole of T4's
// finding: a theme folder with no `init.lua` is not a failure. Measured on both
// sides — upstream renders it at exit 0 and so does this port.
func TestAnAbsentThemeScriptIsSilent(t *testing.T) {
	errs := validateTheme(t, "mytheme", "", "theme: mytheme\n")

	if len(errs) != 0 {
		t.Errorf("errs = %+v, want none for a theme folder with no script", errs)
	}
}

// A **working** script is not a failure either — the report must not fire on
// the path every scripted theme takes.
func TestAWorkingThemeScriptReportsNothing(t *testing.T) {
	errs := validateTheme(t, "mytheme", `return { page = { size = "a5" } }`, "theme: mytheme\n")

	if len(errs) != 0 {
		t.Errorf("errs = %+v, want none", errs)
	}
}

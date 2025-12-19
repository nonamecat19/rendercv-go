package design_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/errorpipeline"
)

// A document value for an option the **script** declares and the design tree
// does not — `custom_note`, not `page.size`.
//
// Upstream's generated theme class carries the user's added fields alongside
// the copied tree ones, so `theme_data_model_class(**design)` (`design.py:135`)
// judges both. The port judged only the tree's, because a script-invented
// option has no tree field to be judged by, and rendered every one of these at
// exit 0.
//
// **The declared default is the type**, which is D-002's substitution: a Lua
// declaration has no annotation but always has a value, and a theme defaulting
// an option to `"hello"` has said it is a string as clearly as `str` would.
//
// Every row is measured against the vendored binary with the same options
// declared in an `__init__.py` (`custom_note: str`, `custom_count: int`,
// `custom_flag: bool`, `custom_ratio: float`, `custom_list: list[str]`,
// `custom_group: CustomGroup`). Location is plain `design` throughout, which is
// upstream's location-strip defect and not a simplification here.
func TestValidateScriptDeclaredOptionValues(t *testing.T) {
	const script = `return {
		custom_note = "hello",
		custom_count = 3,
		custom_flag = true,
		custom_ratio = 1.5,
		custom_list = { "a" },
		custom_group = { x = "1" },
	}`

	tests := []struct {
		name        string
		yaml        string
		wantMessage string
		wantInput   string
	}{
		// The seven upstream rejects.
		{
			name:        "a mapping where a string is declared",
			yaml:        "custom_note:\n  a: 1\n",
			wantMessage: "Input should be a valid string.",
			wantInput:   "...",
		},
		{
			name:        "a list where a string is declared",
			yaml:        "custom_note:\n  - 1\n",
			wantMessage: "Input should be a valid string.",
			wantInput:   "...",
		},
		{
			name:        "a number where a string is declared",
			yaml:        "custom_note: 5\n",
			wantMessage: "Input should be a valid string.",
			wantInput:   "5",
		},
		{
			// **Python's `str()`, so `True`** — `RenderInput` already does this.
			name:        "a boolean where a string is declared",
			yaml:        "custom_note: true\n",
			wantMessage: "Input should be a valid string.",
			wantInput:   "True",
		},
		{
			// **The date message is upstream's, and it is nonsense here.**
			// `error_dictionary.yaml:2` rewrites pydantic's integer-parse
			// failure and is keyed on the message text alone, with no notion of
			// which field produced it, so a custom theme's integer option tells
			// the user to write `YYYY-MM-DD`. Axis-4 text: reproduce it.
			name:        "a word where an integer is declared",
			yaml:        "custom_count: seven\n",
			wantMessage: "This is not a valid date. Please use either YYYY-MM-DD, YYYY-MM, or YYYY format.",
			wantInput:   "seven",
		},
		{
			name:        "a fractional number where an integer is declared",
			yaml:        "custom_count: 5.5\n",
			wantMessage: "Input should be a valid integer, got a number with a fractional part.",
			wantInput:   "5.5",
		},
		{
			name:        "a mapping where an integer is declared",
			yaml:        "custom_count:\n  a: 1\n",
			wantMessage: "Input should be a valid integer.",
			wantInput:   "...",
		},
		{
			name:        "a whole number where a boolean is declared",
			yaml:        "custom_flag: 7\n",
			wantMessage: "Input should be a valid boolean, unable to interpret input.",
			wantInput:   "7",
		},
		{
			// A non-integral number is `bool_type`, not `bool_parsing` — the
			// distinction `validBoolNode` already draws for the tree's bools.
			name:        "a fractional number where a boolean is declared",
			yaml:        "custom_flag: 1.5\n",
			wantMessage: "Input should be a valid boolean.",
			wantInput:   "1.5",
		},
		{
			name:        "a word where a number is declared",
			yaml:        "custom_ratio: seven\n",
			wantMessage: "Input should be a valid number, unable to parse string as a number.",
			wantInput:   "seven",
		},
		{
			name:        "a mapping where a number is declared",
			yaml:        "custom_ratio:\n  a: 1\n",
			wantMessage: "Input should be a valid number.",
			wantInput:   "...",
		},
		{
			// `error_dictionary.yaml:12` rewrites `Input should be a valid
			// list`.
			name:        "a scalar where a list is declared",
			yaml:        "custom_list: oops\n",
			wantMessage: "This field should contain a list of items but it doesn't.",
			wantInput:   "oops",
		},
		{
			name:        "a mapping where a list is declared",
			yaml:        "custom_list:\n  a: 1\n",
			wantMessage: "This field should contain a list of items but it doesn't.",
			wantInput:   "...",
		},
		{
			// **The model name is derived from the key**, because a Lua table
			// has no class name to carry. Upstream names the user's Python
			// class, which is `CustomGroup` for a group called `custom_group`
			// only by the convention its own `create-theme` follows.
			name:        "a scalar where a group is declared",
			yaml:        "custom_group: oops\n",
			wantMessage: "Input should be a valid dictionary or instance of CustomGroup.",
			wantInput:   "oops",
		},
		{
			name:        "a list where a group is declared",
			yaml:        "custom_group:\n  - 1\n",
			wantMessage: "Input should be a valid dictionary or instance of CustomGroup.",
			wantInput:   "...",
		},

		// The five upstream accepts. Coercion is pydantic's lax mode and the
		// port must not out-strict it: each of these renders at exit 0 on both
		// sides, measured.
		{name: "a string of digits for an integer", yaml: "custom_count: \"5\"\n"},
		{name: "a boolean for an integer", yaml: "custom_count: true\n"},
		{name: "an integral float for an integer", yaml: "custom_count: 5.0\n"},
		{name: "a bool word for a boolean", yaml: "custom_flag: no\n"},
		{name: "an integer for a number", yaml: "custom_ratio: 2\n"},
		{name: "a correct value of every declared type", yaml: "custom_note: bye\ncustom_count: 7\n" +
			"custom_flag: false\ncustom_ratio: 2.5\ncustom_list:\n  - b\ncustom_group:\n  x: \"2\"\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := validateTheme(t, "mytheme", script, "theme: mytheme\n"+test.yaml)
			// **Asserted after `errorpipeline.Parse`, because that is what the
			// user reads.** Two of these rows exist only on this side of it:
			// the dictionary rewrites the integer-parse message to the date one
			// and the list message to the "should contain a list" one, and step
			// 8 appends every trailing period.
			errs, err := errorpipeline.Parse(raw, nil, nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			if test.wantMessage == "" {
				if len(errs) != 0 {
					t.Fatalf("errs = %+v, want none — upstream renders this at exit 0", errs)
				}
				return
			}
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Message != test.wantMessage {
				t.Errorf("message = %q, want %q", errs[0].Message, test.wantMessage)
			}
			if errs[0].Input != test.wantInput {
				t.Errorf("input = %q, want %q", errs[0].Input, test.wantInput)
			}
			// Upstream's location strip collapses every depth-1 `design` error
			// to `design`, script-declared options included. Measured.
			if got := strings.Join(errs[0].SchemaLocation, "."); got != "design" {
				t.Errorf("location = %q, want design", got)
			}
		})
	}
}

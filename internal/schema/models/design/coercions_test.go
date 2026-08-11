package design_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// `convert_section_titles_to_snake_case` (classic_theme.py:493-500).
//
// A coercion, not a validation — nothing rejects a spaced title. What it
// produces is what the renderer matches section titles against, so getting it
// wrong makes time spans silently not appear rather than making anything fail.
func TestSnakeCaseSectionTitles(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"the documented case", []string{"Work Experience"}, []string{"work_experience"}},
		{"already snake case", []string{"experience"}, []string{"experience"}},
		{"several words", []string{"A B C"}, []string{"a_b_c"}},
		{"mixed list", []string{"Experience", "Education"}, []string{"experience", "education"}},
		{"empty", nil, []string{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := design.SnakeCaseSectionTitles(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("= %q, want %q", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// **pydantic's lax bool accepts a number by its value, not by its spelling.**
// Every row here is measured against the vendored Python on `design.classic`:
// upstream exits 0 and writes the boolean shown into the `.typ`, where the port
// used to raise a validation panel — its float arm rejected outright and its
// int arm compared the source *text* against `"0"`/`"1"`. Found by a
// fresh-context verifier (iteration 14's twenty-second re-verification).
//
// The value is asserted as a `bool`, not through `EffectiveBool`, because an
// uncoerced `"0o0"` string reads as `false` there and would pass a falsy row
// for the wrong reason.
func TestNumericBooleanSpellings(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"a whole float", "1.0", true},
		{"a zero float", "0.0", false},
		{"a negative zero float", "-0.0", false},
		{"an exponent float", "1e0", true},
		{"a hexadecimal one", "0x1", true},
		{"an octal one", "0o1", true},
		{"a binary one", "0b1", true},
		{"a signed one", "+1", true},
		{"a hexadecimal zero", "0x0", false},
		{"an octal zero", "0o0", false},
		{"a binary zero", "0b0", false},
		{"a signed zero", "+0", false},
		{"a plain one", "1", true},
		{"a plain zero", "0", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := yamlreader.ReadString("theme: classic\nlinks:\n  underline: " + tc.text + "\n")
			if err != nil {
				t.Fatalf("ReadString: %v", err)
			}
			node := &yamldoc.Node{Kind: yamldoc.KindMapping, Items: doc.Items}
			if errs := design.Validate(node, []string{"design"}, schemaerr.SourceMain, nil); len(errs) != 0 {
				t.Fatalf("errs = %+v, want none — upstream renders %s", errs, tc.text)
			}

			// `mappingOf` keeps a design scalar's source text, so this is the
			// shape the effective tree really receives from a document.
			values := design.Effective("classic", map[string]any{
				"links": map[string]any{"underline": tc.text},
			})
			links, ok := values["links"].(map[string]any)
			if !ok {
				t.Fatalf("links = %#v, want a mapping", values["links"])
			}
			if got := links["underline"]; got != any(tc.want) {
				t.Errorf("links.underline = %#v, want the bool %v", got, tc.want)
			}
		})
	}
}

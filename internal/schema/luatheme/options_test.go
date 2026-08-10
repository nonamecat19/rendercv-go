package luatheme_test

import (
	"reflect"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/luatheme"
)

func options(t *testing.T, script string) map[string]any {
	t.Helper()
	table, err := luatheme.Run(script)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return luatheme.Options(table)
}

// A declaration becomes the same shape a theme's YAML override has, so it can
// merge through the machinery iteration 6 already has.
func TestOptionsConvertsADeclaration(t *testing.T) {
	got := options(t, `
		return {
			colors = { name = "rgb(10, 20, 30)" },
			page = { show_footer = false },
			entries = { date_and_location_width = "5cm" },
		}
	`)

	want := map[string]any{
		"colors":  map[string]any{"name": "rgb(10, 20, 30)"},
		"page":    map[string]any{"show_footer": false},
		"entries": map[string]any{"date_and_location_width": "5cm"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("= %#v, want %#v", got, want)
	}
}

// **Lua has one numeric type**, so a whole number has to come back whole: an
// option declared `10` reaching a template as `10.0` changes the Typst.
func TestWholeNumbersStayWhole(t *testing.T) {
	got := options(t, `return { a = 10, b = 10.5 }`)

	if got["a"] != 10 {
		t.Errorf("a = %#v, want the int 10", got["a"])
	}
	if got["b"] != 10.5 {
		t.Errorf("b = %#v, want 10.5", got["b"])
	}
}

// Values that cannot be design options are dropped rather than reported — the
// sandbox already decided what a script may reach.
func TestNonValuesAreDropped(t *testing.T) {
	got := options(t, `return { f = function() end, ok = "yes", [1] = "indexed" }`)

	if len(got) != 1 || got["ok"] != "yes" {
		t.Errorf("= %#v, want only the string option", got)
	}
}

// **A Lua sequence must convert to a `[]string`, not an empty map.** A
// sequence's keys are integers, and `Options`' string-keyed walk found none —
// `sections = { show_time_spans_in = { "Experience" } }` used to become
// `sections = {}`, which `design.ValidateScript` then saw as a shape conflict
// against the tree's `[]string` field and dropped the **whole script**, every
// other option included. Found by a fresh-context verifier (iteration 14's
// fourth re-verification).
func TestASequenceBecomesAStringList(t *testing.T) {
	got := options(t, `return { sections = { show_time_spans_in = { "Experience", "Education" } } }`)

	sections, ok := got["sections"].(map[string]any)
	if !ok {
		t.Fatalf("sections = %#v, want a nested map", got["sections"])
	}
	list, ok := sections["show_time_spans_in"].([]string)
	if !ok || !reflect.DeepEqual(list, []string{"Experience", "Education"}) {
		t.Errorf("show_time_spans_in = %#v, want []string{\"Experience\", \"Education\"}", sections["show_time_spans_in"])
	}
}

// A non-string element inside a sequence is dropped, matching this file's own
// rule for a function or userdata value elsewhere.
// **A number or a bool survives as text, the same shape a document's
// equivalent YAML sequence element would carry** — this used to be
// "dropped", which was correct for the tree's one string-list field but
// silently emptied a script's colour tuple (`colors.name = {1, 2, 3}`), since
// `design.ParseColorTuple` parses exactly this text form. Found by a
// fresh-context verifier (iteration 14's thirteenth re-verification).
func TestASequenceKeepsNumberAndBoolElementsAsText(t *testing.T) {
	got := options(t, `return { tags = { "a", 5, true, "b" } }`)

	want := []string{"a", "5", "true", "b"}
	if list, ok := got["tags"].([]string); !ok || !reflect.DeepEqual(list, want) {
		t.Errorf("tags = %#v, want %#v", got["tags"], want)
	}
}

// A mixed table — some integer keys, some string keys — is not a sequence by
// this file's own definition (`isSequence` requires every key to be exactly
// `1..Len()`), so it still converts as a map and its non-string keys are
// still dropped, the same as the top-level table always has been.
func TestAMixedTableIsNotASequence(t *testing.T) {
	got := options(t, `return { odd = { [1] = "a", name = "b" } }`)

	odd, ok := got["odd"].(map[string]any)
	if !ok {
		t.Fatalf("odd = %#v, want a map (not a sequence)", got["odd"])
	}
	if len(odd) != 1 || odd["name"] != "b" {
		t.Errorf("odd = %#v, want only the string key", odd)
	}
}

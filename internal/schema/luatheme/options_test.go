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

package design

// Themes is `available_themes` (built_in_design.py:45-49) in the order the
// discriminated union enumerates: `classic` first, then the eight override
// stems sorted.
//
// **Derived from the override data, not restated.** `BuiltInThemes` was
// iteration 4's hand-written list for spec 004 §4.27's message, and a second
// list that could disagree with it is exactly the drift the locale iteration
// removed — so this one is built and `TestThemesMatchBuiltInThemes` asserts the
// two agree.
//
// The order is contractual twice: it is the union's arm order in the schema, and
// it is the emission order the `$defs` collision numbering counts along
// (plan §4).
func Themes() []string {
	stems := make([]string, 0, len(themeOverrides())+1)
	for stem := range themeOverrides() {
		stems = append(stems, stem)
	}
	sortStrings(stems)
	return append([]string{BaseTheme}, stems...)
}

// BaseTheme is `ClassicTheme`'s tag, the union's first arm and the only theme
// that is a hand-declared model rather than an override file.
const BaseTheme = "classic"

// Overrides returns one theme's override mapping, empty for `classic`, which
// has none because it *is* the base.
func Overrides(theme string) map[string]any {
	return themeOverrides()[theme]
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

// IsBuiltinTheme reports whether a name is one of the nine themes this port
// ships.
//
// It is the discriminator upstream tries **first**: only when it fails does
// `validate_design` look for a custom theme folder (`design.py:36-50`). The
// port needs it explicitly because its own custom-theme path is a file lookup,
// which would otherwise fire for a built-in name too.
func IsBuiltinTheme(name string) bool {
	if name == "classic" {
		return true
	}
	_, known := themeOverrides()[name]
	return known
}

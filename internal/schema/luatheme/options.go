package luatheme

import (
	lua "github.com/yuin/gopher-lua"
)

// Options converts a theme script's returned table into the plain values
// `design.Effective` merges (spec 014 §4, criteria 2 and 3).
//
// **A script is a declaration, so it becomes an override layer** — the same
// shape `other_themes/<theme>.yaml` already has. That is what makes a scripted
// theme "change a default": it merges over `ClassicTheme`'s declared values and
// under the document's own `design` block, so a user who overrides the option
// still wins. Upstream gets the same ordering by declaring a pydantic model
// whose defaults the document then overrides.
//
// Only the shapes a declaration can hold are converted: strings, numbers,
// booleans and nested tables. A function or userdata in the table is **dropped**
// rather than reported — it cannot be a design value, and the sandbox already
// decided what a script may reach.
func Options(table *lua.LTable) map[string]any {
	if table == nil {
		return nil
	}

	out := map[string]any{}
	table.ForEach(func(key, value lua.LValue) {
		name, ok := key.(lua.LString)
		if !ok {
			// A sequence index cannot name a design field.
			return
		}
		if converted, ok := convert(value); ok {
			out[string(name)] = converted
		}
	})
	return out
}

func convert(value lua.LValue) (any, bool) {
	switch typed := value.(type) {
	case lua.LString:
		return string(typed), true
	case lua.LBool:
		return bool(typed), true
	case lua.LNumber:
		// **A whole number stays whole.** Lua has one numeric type, so an
		// option like `font_size: 10` would otherwise reach a template as
		// `10.0` and change the Typst it produces.
		if float64(typed) == float64(int(typed)) {
			return int(typed), true
		}
		return float64(typed), true
	case *lua.LTable:
		return Options(typed), true
	}
	return nil, false
}

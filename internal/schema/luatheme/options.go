package luatheme

import (
	"sort"
	"strconv"

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
		// **A Lua sequence and a Lua map are the same type**, so a script
		// declaring `sections.show_time_spans_in = { "Experience" }` — the tree's
		// one list-valued option — used to fall into `Options`' string-keyed
		// walk below, which finds no `LString` keys in a sequence and silently
		// returns an empty map. That reached `design.ValidateScript` as a shape
		// conflict against the tree's `[]string` field and dropped the **whole**
		// script, every other option included, at exit 0 with no message. Found
		// by a fresh-context verifier (`specs/STATE.md`, iteration 14's fourth
		// re-verification).
		if isSequence(typed) {
			return sequenceOf(typed), true
		}
		// **A table with a hole is still a sequence, not a map.**
		// `{"education", nil, "publications"}` has keys `{1, 3}` — a gap
		// `isSequence` (correctly) refuses to call a clean sequence — but
		// it has no string keys either, so the walk below found nothing
		// and this fell through to an empty map, silently dropping both
		// real elements instead of just the hole. Every design field a
		// script can build this way with numeric keys is list-shaped
		// (`KindStringList`, or a colour tuple), never a genuine map keyed
		// by number, so a table with only numeric keys and at least one
		// gap is recovered as the sparse sequence its author almost
		// certainly meant, rather than silently emptied. Found by a
		// fresh-context verifier (iteration 14's Lua-slice sweep).
		if isNumericKeyedOnly(typed) {
			return sparseSequenceOf(typed), true
		}
		return Options(typed), true
	}
	return nil, false
}

// isSequence reports whether a table's keys are exactly `1..table.Len()` with
// nothing else mixed in — the shape `for _, v in ipairs(t)` walks, and the
// only shape a `[]string`-typed design field can hold.
func isSequence(table *lua.LTable) bool {
	length := table.Len()
	if length == 0 {
		return false
	}
	count := 0
	sequence := true
	table.ForEach(func(key, _ lua.LValue) {
		count++
		index, isNumber := key.(lua.LNumber)
		if !isNumber || float64(index) != float64(int(index)) ||
			int(index) < 1 || int(index) > length {
			sequence = false
		}
	})
	return sequence && count == length
}

// isNumericKeyedOnly reports whether every key in a non-empty table is a
// Lua number — the shape `isSequence` rejects only because of a gap, not
// because the table is genuinely a string-keyed map.
func isNumericKeyedOnly(table *lua.LTable) bool {
	hasAny := false
	allNumeric := true
	table.ForEach(func(key, _ lua.LValue) {
		hasAny = true
		if _, isNumber := key.(lua.LNumber); !isNumber {
			allNumeric = false
		}
	})
	return hasAny && allNumeric
}

// sparseSequenceOf is `sequenceOf` for a table whose numeric keys have a
// gap: it walks every key rather than `1..Len()` (`Len()` is undefined for
// a table with a hole), converts each value the same way `sequenceOf`
// does, and orders the result by key — so `{[1]="a", [3]="b"}` recovers as
// `["a", "b"]`, the hole simply skipped rather than losing everything.
func sparseSequenceOf(table *lua.LTable) []string {
	type indexed struct {
		index float64
		text  string
	}
	var items []indexed
	table.ForEach(func(key, value lua.LValue) {
		index, isNumber := key.(lua.LNumber)
		if !isNumber {
			return
		}
		switch element := value.(type) {
		case lua.LString:
			items = append(items, indexed{float64(index), string(element)})
		case lua.LNumber:
			if float64(element) == float64(int64(element)) {
				items = append(items, indexed{float64(index), strconv.FormatInt(int64(element), 10)})
			} else {
				items = append(items, indexed{float64(index), strconv.FormatFloat(float64(element), 'g', -1, 64)})
			}
		case lua.LBool:
			items = append(items, indexed{float64(index), strconv.FormatBool(bool(element))})
		}
	})
	sort.Slice(items, func(i, j int) bool { return items[i].index < items[j].index })
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.text
	}
	return out
}

// sequenceOf reads a Lua sequence into the `[]string` shape `EffectiveStrings`
// and the tree's `KindStringList` fields already expect — but a sequence is
// also how a colour tuple arrives (`colors.name = {1, 2, 3}`), whose elements
// are numbers, not strings. **Only a function or userdata is genuinely not a
// design value**; a number or a bool has to survive as the text a document's
// equivalent YAML sequence would carry (`design/validate.go`'s
// `ParseColorTuple` and `parseNumericText` parse exactly this shape), or a
// script's own colour tuple silently loses every element and reaches the
// artifact as an empty list. Found by a fresh-context verifier (iteration
// 14's thirteenth re-verification).
func sequenceOf(table *lua.LTable) []string {
	length := table.Len()
	out := make([]string, 0, length)
	for i := 1; i <= length; i++ {
		switch element := table.RawGetInt(i).(type) {
		case lua.LString:
			out = append(out, string(element))
		case lua.LNumber:
			if float64(element) == float64(int64(element)) {
				out = append(out, strconv.FormatInt(int64(element), 10))
			} else {
				out = append(out, strconv.FormatFloat(float64(element), 'g', -1, 64))
			}
		case lua.LBool:
			out = append(out, strconv.FormatBool(bool(element)))
		}
	}
	return out
}

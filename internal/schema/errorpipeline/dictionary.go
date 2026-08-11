// Package errorpipeline turns raw validation failures into the records RenderCV
// shows a user, mirroring schema/pydantic_error_handling.py.
//
// It is named for what it does rather than for the Python library upstream gets
// it from: the port has no pydantic, and a package called `pydanticerrors` would
// be a lie at every call site. It imports nothing from `models`, so the pipeline
// can be exercised on hand-built records with no model involved.
package errorpipeline

import "strings"

// dictionaryRow is one row of `schema/error_dictionary.yaml`. Old is matched by
// **substring containment** against the raw message, not by equality, and the
// first match wins (pydantic_error_handling.py:89-92).
type dictionaryRow struct{ Old, New string }

// dictionary is `error_dictionary.yaml` in file order, byte for byte.
//
// A slice and never a map: Go randomizes map iteration order, and
// first-match-wins over a randomized order is nondeterministic. Order is
// contractual (spec 004 §6 rule 5).
//
// **Four of the thirteen rows are unreachable** in the pinned tree
// (spec 004 §3.4 behavior 12), and they are kept anyway, unaltered, because
// reachability is upstream's to decide and not the port's:
//
//	1  pre-empted — §3.5's `end_date` override always replaces the message first
//	3  dead twice over, see below
//	4  dead, see below
//	10 no measured input produces it, and it maps to row 9's value anyway
//
// **Row 2 used to be listed here and is now live.** No field of the *design
// tree* is int-only — every int-typed one is `int | str` — but a **theme
// script** can declare one (`custom_count = 3`), and a non-numeric value for it
// produces exactly this message. The rewrite then tells the user to write
// `YYYY-MM-DD` for an option that has nothing to do with dates, which is
// upstream's behaviour and is reproduced: the dictionary is keyed on message
// text with no notion of which field produced it.
//
// Rows 3 and 4 are dead because **their keys carry doubled backslashes and
// pydantic's messages carry single ones**. The YAML scalars are plain, so YAML
// performs no escape processing and the keys literally read `'\\b10\\..*'`.
// Measured: an invalid `doi` produces `String should match pattern '\b10\..*'`,
// which row 4's key does not contain, so the message survives to the output
// unreplaced. The Go literals below are raw strings so they carry the same two
// backslashes — writing `\b10\..*` here would make a dead row live and break
// parity in the opposite direction from the obvious mistake. Row 3 is dead for a
// second, independent reason: no field declares a pydantic `pattern=` of
// `\d{4}-\d{2}(-\d{2})?` at all, because the date formats are checked with
// `re.fullmatch` inside hand-written validators that raise their own messages.
//
// Row 13's key matches a longer message: `value is not a valid color: string not
// recognised as a valid color` contains it, so the whole message is replaced
// (behavior 14).
var dictionary = []dictionaryRow{
	{`Input should be 'present'`, "This is not a valid `end_date`. Please use either YYYY-MM-DD, YYYY-MM, or YYYY format or 'present'."},
	{`Input should be a valid integer, unable to parse string as an integer`, "This is not a valid date. Please use either YYYY-MM-DD, YYYY-MM, or YYYY format."},
	{`String should match pattern '\\d{4}-\\d{2}(-\\d{2})?'`, "This is not a valid date. Please use either YYYY-MM-DD, YYYY-MM, or YYYY format."},
	{`String should match pattern '\\b10\\..*'`, `A DOI prefix should always start with "10.". For example, "10.1109/TASC.2023.3340648".`},
	{`Input should be a valid URL`, "This is not a valid URL."},
	{`Field required`, "This field is required."},
	{`value is not a valid phone number`, "This is not a valid phone number."},
	{`month must be in 1..12`, "The month must be between 1 and 12."},
	{`day is out of range for month`, "The day is out of range for the month."},
	{`must be in range`, "The day is out of range for the month."},
	{`Extra inputs are not permitted`, "This field is unknown for this object. Please remove it."},
	{`Input should be a valid list`, "This field should contain a list of items but it doesn't."},
	{`value is not a valid color`, `This is not a valid color. Here are some examples of valid colors: "red", "#ff0000", "rgb(255, 0, 0)", "hsl(0, 100%, 50%)"`},
}

// substitute applies the dictionary to a raw message
// (pydantic_error_handling.py:89-92): **substring containment**, in file order,
// first match wins. It is not equality — `Input should be a valid URL` also
// matches `Input should be a valid URL, relative URL without a base` and every
// other parse-failure reason, flattening them all to one message.
//
// A message no row matches is returned unchanged.
func substitute(message string) string {
	for _, row := range dictionary {
		if strings.Contains(message, row.Old) {
			return row.New
		}
	}
	return message
}

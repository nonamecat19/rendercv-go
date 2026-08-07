package errorpipeline

import "strings"

// unwantedLocations is pydantic_error_handling.py:24-32. Every location element
// whose string **contains** one of these is dropped.
//
// **This filter is not dead code in the port, and deleting it does not merely
// lose synthetic tags.** The reasoning that leads there is: Go has no
// pydantic-core, so it emits almost no synthetic branch elements, so the filter
// has nothing to remove. The second step is wrong. The test is containment, not
// equality, so it deletes **real user keys** — measured, four sections that each
// fail to match an entry type:
//
//	interests       contains `int`      → ("cv", "sections")
//	my_list         contains `list`     → ("cv", "sections")
//	strengths       contains `str`      → ("cv", "sections")
//	literally_fine  contains `literal`  → ("cv", "sections")
//	normal_key      —                   → ("cv", "sections", "normal_key")
//
// All four truncated locations are equal, so deduplication collapses them into
// **one** record that names no section at all. `interests` and `strengths` are
// ordinary CV section names, so an unmodified user reaches this. It is upstream
// behavior and the port reproduces it exactly (spec 004 §3.3 behavior 7,
// plan §2.2 consequence 1).
//
// Kept as an array in upstream's order, though the result is order-independent,
// so a diff against `:24-32` is mechanical. `constrained-str` is redundant —
// it contains `str` — and is kept because upstream has it.
var unwantedLocations = [...]string{
	"tagged-union",
	"list",
	"literal",
	"int",
	"str",
	"constrained-str",
	"function-",
}

// filterLocation is step 4 (pydantic_error_handling.py:64-68).
//
// A list index is stringified before the test, and no decimal integer string
// contains any of the seven, so indices always survive.
func filterLocation(location []string) []string {
	kept := make([]string, 0, len(location))
	for _, element := range location {
		if !isUnwanted(element) {
			kept = append(kept, element)
		}
	}
	return kept
}

func isUnwanted(element string) bool {
	for _, unwanted := range unwantedLocations {
		if strings.Contains(element, unwanted) {
			return true
		}
	}
	return false
}

// **Step 2 has no code here, and that is the port's shape rather than an
// omission** (pydantic_error_handling.py:53-55).
//
// Upstream's step 2 deletes the *second* element of any location rooted at
// `design` or `locale`, because pydantic-core inserts the resolved branch value
// there: `("design", "classic", "page", "top_margin")` becomes
// `("design", "page", "top_margin")`.
//
// The port never produces that element. `design.Validate` and `locale.Validate`
// resolve the union themselves and pass the block's own location straight
// through, so what they emit is already what step 2 would return. Reproducing
// the deletion anyway removes a real key: `design.colors.body` became
// `design.body`, which then fails to resolve against the document — an internal
// error where the user should have seen a colour message.
//
// `settings` was never in the set, because it is not a discriminated union and
// keeps every element of its location (spec 004 §3.3 behavior 9). Now nothing
// is, and the two blocks are treated the same way as `cv`.

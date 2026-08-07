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

// discriminatedRoots are the two top-level keys whose models are discriminated
// unions (pydantic_error_handling.py:53).
//
// `settings` is deliberately absent: it is not a discriminated union and keeps
// every element of its location (spec 004 §3.3 behavior 9).
var discriminatedRoots = [...]string{"design", "locale"}

// skipDiscriminator is step 2 (pydantic_error_handling.py:53-55).
//
// When a location's first element names a discriminated union, its **second**
// element is the branch value pydantic-core inserted — the theme name, the
// language name — not a key the user wrote, so it goes:
//
//	("design", "classic", "nope")            → ("design", "nope")
//	("design", "classic", "page", "top_margin") → ("design", "page", "top_margin")
//	("locale", "english", "month")           → ("locale", "month")
//	("design",)                              → ("design",)
//
// The slice is `loc[:1] + loc[2:]`, so a one-element location comes back
// unchanged rather than out of range.
//
// It runs before the context override, which re-pins `design.theme`, and
// therefore before the location filter too.
//
// Upstream indexes `loc[0]` without a guard, so an empty location raises an
// IndexError that escapes the whole function. That is unreachable through the
// document pipeline — the one failure with an empty location is
// PublicationEntry's generated-URL length check, and the entry-problems splice
// always prepends the wrapper's location to it — so the port guards the lookup
// instead of reproducing a crash (spec 004 §3.3 behavior 10, §5.19).
func skipDiscriminator(location []string) []string {
	if len(location) < 2 || !isDiscriminatedRoot(location[0]) {
		return location
	}

	// A fresh slice, never a re-slice of the caller's. Parse walks the raw list
	// more than once — for the record and again for its children — so aliasing
	// here would corrupt the second pass.
	shortened := make([]string, 0, len(location)-1)
	shortened = append(shortened, location[0])
	return append(shortened, location[2:]...)
}

func isDiscriminatedRoot(element string) bool {
	for _, root := range discriminatedRoots {
		if element == root {
			return true
		}
	}
	return false
}

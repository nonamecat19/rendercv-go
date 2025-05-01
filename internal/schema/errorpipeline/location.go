package errorpipeline

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

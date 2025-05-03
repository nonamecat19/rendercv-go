package errorpipeline

import "strings"

// The two forced messages of steps 5 and 6. Both are literals in
// `pydantic_error_handling.py`, not dictionary rows, which is why the
// dictionary's own `end_date` row is unreachable: the override has already
// replaced the message by the time the dictionary runs.
const (
	// messageEndDate ends in `!` — a stray one after the closing quote of
	// `"present"` (pydantic_error_handling.py:74). The dictionary finds no match
	// in it and step 8 appends a period, so the emitted text ends in `!.`.
	// **This looks like a typo and it is not the port's to fix**: there is no
	// exemption from the period rule for either special case (spec 004 §3.5
	// behavior 16, §4.12).
	messageEndDate = "This is not a valid `end_date`! Please use either YYYY-MM-DD," +
		` YYYY-MM, or YYYY format or "present"!`

	// messageCurrentDate already ends in a period, so step 8 changes nothing.
	// That is luck rather than an exemption (spec 004 §3.5 behavior 19, §4.13).
	messageCurrentDate = "This is not a valid `current_date`! Please use YYYY-MM-DD" +
		` format or "today".`
)

// overrideEndDate is step 5 (pydantic_error_handling.py:69-75).
//
// **Containment, not equality**: a field named `my_end_date` matches too. That
// is upstream's test verbatim.
//
// The override exists because one bad `end_date` produces two raw failures — the
// exact-date branch and the `literal['present']` branch — which the location
// filter collapses onto the same location. Forcing both to one message is what
// makes deduplication keep a sensible row rather than whichever branch happened
// to come first (upstream's own comment at :69-70).
//
// It runs before the dictionary, which is why the dictionary's `Input should be
// 'present'` row is dead.
func overrideEndDate(location []string, message string) string {
	if len(location) > 0 && strings.Contains(location[len(location)-1], "end_date") {
		return messageEndDate
	}
	return message
}

// stripCurrentDateSuffix is the first half of step 6
// (pydantic_error_handling.py:81-82).
//
// `settings.current_date` is `datetime.date | Literal["today"]`, and pydantic
// tags the first arm's failure with a trailing `date` element. Dropping it puts
// the field name at the end of the location, so the containment test below can
// match it the same way `end_date`'s does.
//
// **Inert in the port**: Go emits no `date` branch element for `current_date`,
// so nothing reaches this today. It is implemented anyway because iteration 7
// owns `settings.current_date` and may introduce a location shape that does.
//
// Both conditions are required and both are equality, not containment: the last
// element must be exactly `date` and the one before it exactly `current_date`.
func stripCurrentDateSuffix(location []string) []string {
	n := len(location)
	if n >= 2 && location[n-1] == "date" && location[n-2] == "current_date" {
		return location[:n-1]
	}
	return location
}

// overrideCurrentDate is the second half of step 6
// (pydantic_error_handling.py:83-87). Containment again, like step 5.
func overrideCurrentDate(location []string, message string) string {
	if len(location) > 0 && strings.Contains(location[len(location)-1], "current_date") {
		return messageCurrentDate
	}
	return message
}

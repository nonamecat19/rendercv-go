package errorpipeline

import (
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// The two prefixes step 1 removes, both with a trailing space
// (pydantic_error_handling.py:23).
//
// **They are asymmetric in the port, deliberately, and neither half should be
// "tidied".** Spec 004 §3.2 behavior 4a is the decision:
//
//   - `value is not a valid email address: ` is an explicit message template in
//     pydantic's source, so the port reproduces it: `emailaddr`'s caller builds
//     the prefixed message and this half runs on production data. Two records of
//     the 25-record differential are email failures, so it is gated by the
//     strongest test in the iteration.
//   - `Value error, ` is pydantic-core wrapping an exception that escaped a
//     validator. The port has no such mechanism and does not fabricate the
//     prefix, so `entrywithdate.go` stores `month must be in 1..12` unprefixed.
//     This half is implemented, exercised on synthetic records, and **inert**
//     for every message the port itself produces.
//
// So: do not delete the second replacement as dead, and do not add the prefix to
// `entrywithdate.go` to "make it used". Its inertness is asserted by
// `models.TestNoModelMessageCarriesTheValueErrorPrefix`.
//
// The prefix is not a function of the error code. `email: bad`, `phone: bad` and
// `date: 2020-13-01` are all `value_error` and only the last carries it
// (spec 004 §3.2 behavior 4b).
const (
	emailPrefix      = "value is not a valid email address: "
	valueErrorPrefix = "Value error, "
)

// stripPrefixes is step 1 (pydantic_error_handling.py:23, :50-51).
//
// Replacement, not a prefix test: `str.replace` removes **every** occurrence
// anywhere in the message, and spec 004 §6 rule 6 makes that contractual — a
// message carrying either prefix twice loses both copies.
//
// It runs before the dictionary, or `value is not a valid phone number` would
// never match its row.
func stripPrefixes(message string) string {
	message = strings.ReplaceAll(message, emailPrefix, "")
	return strings.ReplaceAll(message, valueErrorPrefix, "")
}

// appendPeriod is step 8 (pydantic_error_handling.py:94-95).
//
// **This is the last statement that touches a message**, and it applies to every
// one — dictionary-matched, specially overridden, or untouched alike. Three of
// its four measured consequences look like bugs and are not (spec 004 §3.6):
//
//	…or 'present'!   → …or 'present'!.   ends `!.`   (§4.12)
//	…username."      → …username.".      ends `.".`  (§4.7)
//	…hsl(0, 100%, 50%)" → …50%)".        ends `)".`  (§4.11)
//
// The condition is on the final character only, so a message already ending in
// `.` is left alone and one ending in any other punctuation gets the period
// appended after it.
func appendPeriod(message string) string {
	if strings.HasSuffix(message, ".") {
		return message
	}
	return message + "."
}

// Parse turns raw validation records into final ones, mirroring
// parse_validation_errors (pydantic_error_handling.py:130-176).
//
// Input records are **raw** and output records are **final**; the distinction is
// on schemaerr.ValidationError. Parse is not idempotent — calling it on records
// it already returned applies the dictionary a second time and can append a
// second period — so it is called once, at the one site that assembles the
// model.
//
// Record order is the raw order, unsorted (spec 004 §6 rule 1). Any sort, stable
// or not, is a defect.
//
// TODO(spec 004 T18-T19): the entry-problems splice and the deduplication.
func Parse(raw []schemaerr.ValidationError) []schemaerr.ValidationError {
	final := make([]schemaerr.ValidationError, 0, len(raw))
	for _, record := range raw {
		final = append(final, parseOne(record))
	}
	return final
}

// parseOne applies the eleven steps of spec 004 §3.2 to one raw record.
//
// The steps are written out in order rather than factored into helpers, because
// **the order is the contract**: reordering any two changes observable output,
// and a reader has to be able to see the sequence at a glance.
//
// TODO(spec 004 T10-T20): steps 3 through 7 and 9 through 11. Only steps 1, 2
// and 8 are here, which is why this cannot yet be called from the model builder.
func parseOne(raw schemaerr.ValidationError) schemaerr.ValidationError {
	final := raw

	// Step 1: strip the unwanted message prefixes.
	final.Message = stripPrefixes(final.Message)

	// Step 2: drop the discriminated union's branch element.
	// Step 4: drop the synthetic elements — and, as it happens, any real key
	//         containing one of the seven substrings. See unwantedLocations.
	//
	// Both are skipped for a record whose validator pinned its own location —
	// that is what step 3's `ctx["loc"]` override means, and re-deriving would
	// undo it.
	if !final.LocationIsFinal {
		final.SchemaLocation = filterLocation(skipDiscriminator(final.SchemaLocation))
	}

	// Step 5: the `end_date` override, before the dictionary.
	final.Message = overrideEndDate(final.SchemaLocation, final.Message)

	// Step 6: the `current_date` suffix strip, then its override. The strip
	// must precede the containment test, or the field name is not last.
	final.SchemaLocation = stripCurrentDateSuffix(final.SchemaLocation)
	final.Message = overrideCurrentDate(final.SchemaLocation, final.Message)

	// Step 8: the trailing period. Last, always.
	final.Message = appendPeriod(final.Message)

	return final
}

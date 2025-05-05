package errorpipeline

import (
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
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
// It runs before the dictionary because upstream orders it that way. On every
// measured message the order is unobservable — substitution matches by
// containment and replaces the whole message, so a prefix can only add a match,
// and neither prefix contains a key. TestPrefixStripDoesNotChangeWhichRowMatches
// asserts that, so if a future row makes the order matter, it fails.
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
// doc is the main document and overlays are the design/locale/settings
// documents, if any were supplied. Both are needed for steps 9, 10 and 12: a
// record rooted at an overlay key resolves its coordinates against that
// overlay's document, not the main one.
//
// A wrapped entry failure contributes its own record **and** one per child,
// spliced in immediately after it — see spliceChildren.
func Parse(
	raw []schemaerr.ValidationError,
	doc *yamldoc.Node,
	overlays map[schemaerr.OverlayKey]*yamldoc.Node,
) ([]schemaerr.ValidationError, error) {
	final := make([]schemaerr.ValidationError, 0, len(raw))
	for _, record := range raw {
		final = append(final, parseOne(record, doc, overlays))

		if record.Code != CodeEntryValidation {
			continue
		}
		children, err := spliceChildren(record, doc, overlays)
		if err != nil {
			return nil, err
		}
		final = append(final, children...)
	}
	return dedup(final), nil
}

// dedup is pydantic_error_handling.py:167-176.
//
// **On the schema location only** — not on the message, not on the code. Two
// genuinely different failures at one location collapse to the first, and that
// is load-bearing rather than incidental at three sites (spec 004 §3.8
// behavior 28):
//
//	end_date: invalid_date        two records at ("end_date",)          → 1
//	settings.current_date: todady two at ("settings","current_date")    → 1
//	photo: photo_doesnt_exist.jpg the path failure, then a URL failure  → 1
//
// The `photo` row is the one where the surviving *message* differs: the second
// record would report a URL parse failure, and dedup is the only thing
// suppressing it.
//
// First occurrence wins and the survivors keep their relative order, so this is
// an ordered set and not a map (spec 004 §6 rule 4).
func dedup(records []schemaerr.ValidationError) []schemaerr.ValidationError {
	seen := make(map[string]struct{}, len(records))
	kept := make([]schemaerr.ValidationError, 0, len(records))

	for _, record := range records {
		key := locationKey(record.SchemaLocation)
		if _, already := seen[key]; already {
			continue
		}
		seen[key] = struct{}{}
		kept = append(kept, record)
	}
	return kept
}

// locationKey flattens a location into a map key.
//
// The separator is NUL rather than a dot because a section key is a user-chosen
// string that may contain dots: joining `("cv", "a.b")` and `("cv", "a", "b")`
// with a dot would make two different locations collide and silently drop the
// second record. NUL cannot appear in a YAML key.
func locationKey(location []string) string {
	return strings.Join(location, "\x00")
}

// CodeEntryValidation is the one code that carries child failures
// (models/custom_error_types.py:5, raised only at section.py:230).
const CodeEntryValidation schemaerr.Code = "rendercv_entry_validation_error"

// spliceChildren unpacks a wrapped entry failure
// (pydantic_error_handling.py:153-165).
//
// The wrapper's own record is kept and each child becomes a record of its own,
// appended immediately after it in the child list's order.
//
// A child's location is rebuilt twice over:
//
//   - **its first element is dropped** — the literal `entries`, which exists
//     only because section.py validates the shape `{"entries": [...]}`;
//   - the **wrapper's raw location** is prepended, before the filter has touched
//     it, and the spliced whole is then filtered as one in the child's own pass.
//
// Measured: child `("entries", 1, "institution")` under wrapper
// `("cv", "sections", "welcome_to_rendercv_tests_2")` becomes
// `["cv", "sections", "welcome_to_rendercv_tests_2", "1", "institution"]`.
//
// A child with an empty location splices to the wrapper's location and nothing
// else, so it lands one level shallower — that is how the start-after-end rule
// reports at the entry rather than at a field.
//
// **One level only. Do not recurse.** Only section.py raises this code and it is
// never raised from inside a child, so a nested wrapper does not occur; treating
// one as a wrapper would invent a location no upstream input produces.
func spliceChildren(
	wrapper schemaerr.ValidationError,
	doc *yamldoc.Node,
	overlays map[schemaerr.OverlayKey]*yamldoc.Node,
) ([]schemaerr.ValidationError, error) {
	if len(wrapper.Children) == 0 {
		return nil, &schemaerr.InternalError{Message: messageWrapperWithoutChildren}
	}

	spliced := make([]schemaerr.ValidationError, 0, len(wrapper.Children))
	for _, child := range wrapper.Children {
		tail := child.SchemaLocation
		if len(tail) > 0 {
			tail = tail[1:]
		}

		located := child
		located.SchemaLocation = make([]string, 0, len(wrapper.SchemaLocation)+len(tail))
		located.SchemaLocation = append(located.SchemaLocation, wrapper.SchemaLocation...)
		located.SchemaLocation = append(located.SchemaLocation, tail...)
		located.Children = nil

		spliced = append(spliced, parseOne(located, doc, overlays))
	}
	return spliced, nil
}

// messageWrapperWithoutChildren is spec 004 §4.16, an internal failure rather
// than a validation record: a wrapper with no children means the producer built
// it wrong.
const messageWrapperWithoutChildren = "entry_validation error missing ctx or caused_by"

// selectSource is steps 9 and 12 (pydantic_error_handling.py:97-104).
//
// The source starts as the main file and the coordinate document as the main
// document. All three conditions must hold to replace them: overlays were
// supplied, the location is non-empty, and its first element names an overlay.
// A record rooted anywhere else — `cv`, or a location the filter emptied —
// keeps the main document.
//
// It reads location[0], so it has to follow step 4.
func selectSource(
	location []string,
	doc *yamldoc.Node,
	overlays map[schemaerr.OverlayKey]*yamldoc.Node,
) (schemaerr.YamlSource, *yamldoc.Node) {
	if len(overlays) == 0 || len(location) == 0 {
		return schemaerr.SourceMain, doc
	}

	key := schemaerr.OverlayKey(location[0])
	overlay, supplied := overlays[key]
	source, known := schemaerr.OverlayToSource[key]
	if !supplied || !known {
		return schemaerr.SourceMain, doc
	}
	return source, overlay
}

// parseOne applies the eleven steps of spec 004 §3.2 to one raw record.
//
// The steps are written out in order rather than factored into helpers, because
// **the order is the contract**: reordering any two changes observable output,
// and a reader has to be able to see the sequence at a glance.
//
// TODO(spec 004 T16): step 11. Everything else is here.
func parseOne(
	raw schemaerr.ValidationError,
	doc *yamldoc.Node,
	overlays map[schemaerr.OverlayKey]*yamldoc.Node,
) schemaerr.ValidationError {
	final := raw

	// Step 1: strip the unwanted message prefixes.
	final.Message = stripPrefixes(final.Message)

	// Step 3: the context overrides, `input` then `loc`
	// (pydantic_error_handling.py:57-62).
	//
	// There is no separate context map to read here. Upstream's `ctx` carries
	// two things and the port carries both on the record itself: an overriding
	// `input` is simply the Input the validator wrote, and an overriding `loc` is
	// LocationIsFinal. Exactly one producer sets either — the theme-name failure
	// of spec 004 §4.27, whose raw location is `("design",)` and whose final one
	// is `("design", "theme")`.
	//
	// So step 3 is not a statement but a condition on the two that follow: a
	// pinned location must not be re-derived, or the override is undone.
	//
	// Steps 2 and 4: drop the discriminated union's branch element, then the
	// synthetic elements — and, as it happens, any real key containing one of
	// the seven substrings. See unwantedLocations.
	if !final.LocationIsFinal {
		final.SchemaLocation = filterLocation(skipDiscriminator(final.SchemaLocation))
	}

	// Step 5: the `end_date` override, before the dictionary.
	final.Message = overrideEndDate(final.SchemaLocation, final.Message)

	// Step 6: the `current_date` suffix strip, then its override. The strip
	// must precede the containment test, or the field name is not last.
	final.SchemaLocation = stripCurrentDateSuffix(final.SchemaLocation)
	final.Message = overrideCurrentDate(final.SchemaLocation, final.Message)

	// Step 7: dictionary substitution, first containment match wins.
	final.Message = substitute(final.Message)

	// Step 8: the trailing period. Last, always.
	final.Message = appendPeriod(final.Message)

	// Steps 9 and 12: the source literal and the document coordinates resolve
	// against.
	source, coordinateDoc := selectSource(final.SchemaLocation, doc, overlays)
	final.YamlSource = source

	// Step 10: resolve the coordinates by walking that document.
	//
	// A walk that misses is an internal failure — the document is the user's own,
	// so a miss means the location was built wrong, not that the input was bad.
	// It cannot be reported as a validation error without inventing a message, so
	// the record keeps whatever span its producer attached.
	//
	// TODO(spec 004 T20): surface the internal failure at the call site rather
	// than falling back, once the model builder owns the call.
	if span, err := resolveCoordinates(coordinateDoc, coordinatePath(final.SchemaLocation, final.Code)); err == nil && span != nil {
		final.YamlLocation = span
	}

	return final
}

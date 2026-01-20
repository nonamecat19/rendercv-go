package entries

import (
	"strconv"

	"github.com/nonamecat19/rendercv-go/internal/schema/httpurl"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// Dump is pydantic's `model_dump(exclude_none=True)` for a validated entry
// (entry_templates_from_input.py:128-130), which is the dictionary
// `render_entry_templates` upper-cases into placeholders.
//
// **Every key the mapping carries is dumped, not only the declared ones.** The
// entry bases are `BaseModelWithExtraKeys` (base.py:9), so a user's `supervisor`
// survives validation and becomes a `SUPERVISOR` placeholder with no
// registration step (spec 008 §4G behavior 59). Driving the dump off the
// declared field set would silently drop exactly those.
//
// It returns a second value, the set of fields the document wrote as a **bare
// integer**. `FormatDateRange` branches on `isinstance(x, int)` (spec 008 §4C),
// and `date: 2020` and `date: "2020"` format differently — the port sees the
// same text for both, so the distinction has to be read off the node here,
// while the node still exists.
//
// Nothing is defaulted: every optional entry field defaults to `None` upstream
// and `exclude_none=True` then omits it. An **empty string is kept**, which
// matters because `render_entry_templates` drops empty values itself, one step
// later and for its own reason.
func Dump(node *yamldoc.Node, name TypeName) (map[string]any, map[string]bool) {
	if node == nil || node.Kind != yamldoc.KindMapping {
		return nil, nil
	}

	fields := make(map[string]any, len(node.Items))
	yearOnly := map[string]bool{}

	for _, item := range node.Items {
		if item.Value == nil || item.Value.Kind == yamldoc.KindNull {
			// A key written `null` binds to nothing, which is the same `None`
			// `exclude_none=True` removes.
			continue
		}
		if item.Value.Kind == yamldoc.KindInt {
			yearOnly[item.Key] = true
		}
		fields[item.Key] = dumpValue(item.Value)
	}

	applyModelValidators(fields, name)
	if hasComplexFields(name) {
		adjustDates(fields, yearOnly)
	}
	return fields, yearOnly
}

// hasComplexFields names the three entry types built on
// `BaseEntryWithComplexFields` (education.py, experience.py, normal.py — the
// three that call `ComplexSpec`).
//
// **`check_and_adjust_dates` lives on that base alone**
// (entry_with_complex_fields.py:90,133). `PublicationEntry` inherits
// `BaseEntryWithDate` only, so applying the rewrites to it synthesized an
// `end_date: present` for a publication carrying a stray `start_date` and
// rendered a date range — where upstream aborts the render outright. A
// re-audit caught that; it was introduced by the fix for the rewrites
// themselves.
func hasComplexFields(name TypeName) bool {
	switch name {
	case "EducationEntry", "ExperienceEntry", "NormalEntry":
		return true
	default:
		return false
	}
}

// adjustDates is `check_and_adjust_dates`' three silent rewrites
// (entry_with_complex_fields.py:133-171), which decide what a date column
// contains before any formatting runs.
//
// **They were computed and then thrown away.** `bases.adjustDates` applies them
// to the typed model, `Dump` reads the *node*, and nothing carried the result
// across — so an audit measured three artifact defects at once: a `date`
// alongside a range rendered the range, a lone `end_date` vanished, and a lone
// `start_date` **blanked the entire entry**, company and position included,
// because `EntryDate` failed and the whole expansion was skipped. A lone
// `start_date` is one of the most ordinary shapes a CV has.
//
// Step 4 — the ordering check — is not here: it is the one step that can *fail*,
// and failing belongs to validation, which already reports it.
func adjustDates(fields map[string]any, yearOnly map[string]bool) {
	_, hasDate := fields["date"]
	_, hasStart := fields["start_date"]
	_, hasEnd := fields["end_date"]

	switch {
	case hasDate:
		// Step 1: `date` silently wins.
		delete(fields, "start_date")
		delete(fields, "end_date")
		delete(yearOnly, "start_date")
		delete(yearOnly, "end_date")
	case !hasStart && hasEnd:
		// Step 2: a lone `end_date` migrates to `date`, carrying its int-ness.
		fields["date"] = fields["end_date"]
		if yearOnly["end_date"] {
			yearOnly["date"] = true
		}
		delete(fields, "end_date")
		delete(yearOnly, "end_date")
	case hasStart && !hasEnd:
		// Step 3: a lone `start_date` implies an ongoing event.
		fields["end_date"] = "present"
		delete(yearOnly, "end_date")
	}
}

// applyModelValidators reproduces the two `mode="after"` validators that change
// what a *valid* entry dumps — not the ones that only accept or reject.
//
//  1. `ignore_url_if_doi_is_given` (publication.py:46-62) clears `url` when a
//     DOI is present, so a publication with both dumps **no** `url` at all and
//     its `URL` placeholder comes from the DOI instead.
//  2. `pydantic.HttpUrl` serializes rather than echoes: `https://example.com`
//     dumps as `https://example.com/`. Measured — it is the difference between a
//     trailing slash in the artifact and none.
func applyModelValidators(fields map[string]any, name TypeName) {
	if name != "PublicationEntry" {
		return
	}
	if _, hasDOI := fields["doi"]; hasDOI {
		delete(fields, "url")
		return
	}
	raw, isString := fields["url"].(string)
	if !isString {
		return
	}
	if normalized, err := httpurl.Validate(raw); err == nil {
		fields["url"] = normalized
	}
}

// dumpValue projects one node onto the Go value pydantic would have dumped.
//
// A scalar keeps its resolved type — an integer stays an integer — because
// `process.RunFields` stringifies with Python's rules and a `2020.0` would reach
// the artifact if a year arrived as a float.
func dumpValue(node *yamldoc.Node) any {
	switch node.Kind {
	case yamldoc.KindInt:
		if value, err := strconv.Atoi(node.Raw); err == nil {
			return value
		}
		return node.Raw
	case yamldoc.KindFloat:
		if value, err := strconv.ParseFloat(node.Raw, 64); err == nil {
			return value
		}
		return node.Raw
	case yamldoc.KindBool:
		return yamldoc.BoolIsTrue(node.Raw)
	case yamldoc.KindSequence:
		return dumpSequence(node)
	case yamldoc.KindMapping:
		nested := make(map[string]any, len(node.Items))
		for _, item := range node.Items {
			if item.Value == nil || item.Value.Kind == yamldoc.KindNull {
				continue
			}
			nested[item.Key] = dumpValue(item.Value)
		}
		return nested
	case yamldoc.KindNull, yamldoc.KindString, yamldoc.KindTagged:
		// A null, a string and a tagged scalar all dump as their text: a
		// `TaggedScalar`'s `str()` is its value, which is what `process.RunFields`
		// stringifies. Named one by one rather than left to a `default` so a new
		// kind stops here instead of quietly joining this set (kindguard).
		return node.Raw
	}
	return node.Raw
}

// dumpSequence returns `[]string` when every element is a scalar, which is every
// declared list field — `highlights` and `authors` are both `list[str]`. A
// sequence with a non-scalar element can only come from an extra key, and it
// keeps the general shape so nothing is lost.
func dumpSequence(node *yamldoc.Node) any {
	strings := make([]string, 0, len(node.Elems))
	for _, elem := range node.Elems {
		if elem == nil || isContainerKind(elem.Kind) {
			values := make([]any, 0, len(node.Elems))
			for _, each := range node.Elems {
				if each == nil {
					values = append(values, nil)
					continue
				}
				values = append(values, dumpValue(each))
			}
			return values
		}
		strings = append(strings, elem.Raw)
	}
	return strings
}

// isContainerKind reports whether a kind holds other nodes rather than text.
//
// **Written as an exhaustive switch on purpose**, the shape `fitsNoScalarArm`
// documents (`binder.go:520-544`). A tagged scalar answers *no*: its `str()` is
// its text, so it dumps through `elem.Raw` alongside the other scalars rather
// than forcing the sequence into its general shape.
func isContainerKind(kind yamldoc.Kind) bool {
	switch kind {
	case yamldoc.KindMapping, yamldoc.KindSequence:
		return true
	case yamldoc.KindNull, yamldoc.KindBool, yamldoc.KindInt,
		yamldoc.KindFloat, yamldoc.KindString, yamldoc.KindTagged:
		return false
	}
	return false
}

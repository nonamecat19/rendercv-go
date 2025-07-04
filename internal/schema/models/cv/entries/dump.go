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
	return fields, yearOnly
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
		return node.Raw == "true" || node.Raw == "True" || node.Raw == "yes" || node.Raw == "on"
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
	default:
		return node.Raw
	}
}

// dumpSequence returns `[]string` when every element is a scalar, which is every
// declared list field — `highlights` and `authors` are both `list[str]`. A
// sequence with a non-scalar element can only come from an extra key, and it
// keeps the general shape so nothing is lost.
func dumpSequence(node *yamldoc.Node) any {
	strings := make([]string, 0, len(node.Elems))
	for _, elem := range node.Elems {
		if elem == nil || elem.Kind == yamldoc.KindMapping || elem.Kind == yamldoc.KindSequence {
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

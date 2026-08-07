package settings

import (
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// Resolved is the three settings fields the renderer reads
// (settings.py:11-52). It is a read of a validated block, not a second
// validation: every value here has a declared default, so a document with no
// `settings` block at all resolves to the defaults.
type Resolved struct {
	// CurrentDate is `_resolved_current_date` (`:72-76`): the declared date, or
	// today when the value is the literal `today`.
	CurrentDate time.Time
	// BoldKeywords is `settings.bold_keywords`, deduplicated.
	BoldKeywords []string
	// PDFTitle is `settings.pdf_title`, before its placeholders are substituted.
	PDFTitle string
}

// DefaultPDFTitle is `pdf_title`'s declared default (`:32-33`).
const DefaultPDFTitle = "NAME - CV"

// Resolve reads a `settings` block, filling in the declared defaults.
//
// `now` is what the literal `today` resolves to, passed in rather than read from
// the clock so a render is reproducible — it is the same reference date
// validation used, and the CLI's `--current-date` reaches this through it.
func Resolve(node *yamldoc.Node, now time.Time) Resolved {
	out := Resolved{CurrentDate: now, PDFTitle: DefaultPDFTitle}
	if node == nil || node.Kind != yamldoc.KindMapping {
		return out
	}

	for _, item := range node.Items {
		if item.Value == nil || item.Value.Kind == yamldoc.KindNull {
			continue
		}
		switch item.Key {
		case "current_date":
			if item.Value.Raw == "today" {
				continue
			}
			if parsed, err := time.Parse("2006-01-02", item.Value.Raw); err == nil {
				out.CurrentDate = parsed
			}
		case "bold_keywords":
			out.BoldKeywords = uniqueKeywords(item.Value)
		case "pdf_title":
			out.PDFTitle = item.Value.Raw
		}
	}
	return out
}

// uniqueKeywords is `keep_unique_keywords` (`:54-69`), whose body is
// `list(set(value))`.
//
// **Upstream's result order is a Python set's**, which is hash-dependent and, with
// the default randomized string hashing, differs between runs. This port
// deduplicates in input order instead, because a renderer whose output depends
// on the interpreter's hash seed cannot be diffed against anything.
//
// It mostly does not show: `build_keyword_matcher_pattern` sorts the keywords by
// descending length, so the order only survives between keywords of *equal*
// length, and two equal-length keywords both matching at the same position is
// what it would take to see a difference. Recorded here rather than in
// `divergences.md` because upstream has no fixed behavior to diverge from.
func uniqueKeywords(node *yamldoc.Node) []string {
	if node.Kind != yamldoc.KindSequence {
		return nil
	}

	seen := make(map[string]struct{}, len(node.Elems))
	out := make([]string, 0, len(node.Elems))
	for _, elem := range node.Elems {
		if elem == nil || elem.Kind == yamldoc.KindNull {
			continue
		}
		if _, already := seen[elem.Raw]; already {
			continue
		}
		seen[elem.Raw] = struct{}{}
		out = append(out, elem.Raw)
	}
	return out
}

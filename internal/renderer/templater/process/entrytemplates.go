package process

import "strings"

// Highlights is `process_highlights` (entry_templates_from_input.py:226-251).
//
// Each highlight becomes `"- " + highlight` with `" - "` — **space, hyphen,
// space** — replaced by a newline and an indented sub-bullet. So one highlight
// string can carry its own nesting, and a hyphen without surrounding spaces is
// left alone.
func Highlights(highlights []string) string {
	out := make([]string, 0, len(highlights))
	for _, highlight := range highlights {
		out = append(out, "- "+strings.ReplaceAll(highlight, " - ", "\n  - "))
	}
	return strings.Join(out, "\n")
}

// Authors is `process_authors` (`:254-263`): a plain comma join, with no
// `and` before the last and no serial comma.
func Authors(authors []string) string {
	return strings.Join(authors, ", ")
}

// EntryURL is `process_url` (`:357-378`). Named for the entry rather than
// `ProcessURL`, which would stutter as `process.ProcessURL`.
//
// **A publication with a DOI delegates**, so a `URL` placeholder on such an
// entry shows the DOI link rather than the entry's own URL.
func EntryURL(url, doi, doiURL string) string {
	if doi != "" {
		return EntryDOI(doi, doiURL)
	}
	return "[" + CleanURL(url) + "](" + url + ")"
}

// EntryDOI is `process_doi` (`:381-399`): the DOI is its own display text.
func EntryDOI(doi, doiURL string) string {
	return "[" + doi + "](" + doiURL + ")"
}

// SummaryBlock is `process_summary` (`:402-420`): a Markdown admonition whose
// body is indented four spaces.
//
// **This is the only producer of an admonition**, and it is why MarkdownToTypst
// keeps `!!!` blocks together — the four-space indent is what the block
// collector looks for.
func SummaryBlock(summary string) string {
	lines := strings.Split(summary, "\n")
	for i, line := range lines {
		// `textwrap.indent` skips lines that are entirely whitespace.
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines[i] = "    " + line
	}
	return "!!! summary\n" + strings.Join(lines, "\n")
}

// SummaryIsStandalone is the test `render_entry_templates` makes before wrapping
// (`:203-208`): some template line is **exactly** `SUMMARY` after stripping.
//
// A summary interpolated mid-line stays plain, so the wrap depends on the theme
// rather than on the entry.
func SummaryIsStandalone(templates map[string]string) bool {
	for _, tmpl := range templates {
		for _, line := range strings.Split(tmpl, "\n") {
			if strings.TrimSpace(line) == "SUMMARY" {
				return true
			}
		}
	}
	return false
}

// ExpandPhrases is the locale-phrase pass of `render_entry_templates`
// (`:141-147`).
//
// Each `locale.phrases` key, upper-cased, is replaced by its template text —
// `DEGREE_WITH_AREA` becomes `DEGREE in AREA` for English and `DEGREE en AREA`
// for French. **The sub-placeholders stay placeholders**, so the rest of the
// pipeline treats them like any other; this is spec 007 §4.2's deferred
// substitution, and doing it any later would leave `DEGREE` and `AREA`
// unresolved.
func ExpandPhrases(templates map[string]string, phrases map[string]string) map[string]string {
	out := make(map[string]string, len(templates))
	for key, tmpl := range templates {
		for phrase, text := range phrases {
			tmpl = strings.ReplaceAll(tmpl, strings.ToUpper(phrase), text)
		}
		out[key] = tmpl
	}
	return out
}

// EntryFields is `entry.model_dump(exclude_none=True)` with the keys
// upper-cased, minus the empty strings (`:129-135`).
//
// **An empty-string value counts as not provided**, so its surrounding `**` and
// commas are cleaned up by the removal passes rather than left wrapping nothing.
// A port that kept empty strings would render `****` where a field is blank.
func EntryFields(fields map[string]string) map[string]string {
	out := make(map[string]string, len(fields))
	for key, value := range fields {
		if value == "" {
			continue
		}
		out[strings.ToUpper(key)] = value
	}
	return out
}

// SkippedFields is `process_fields`' skip set (`:166`). They are the four whose
// value has to survive unprocessed for a link or a date to work.
//
// A field whose name starts with `_` is skipped too, which the caller applies.
var SkippedFields = map[string]bool{
	"start_date": true, "end_date": true, "doi": true, "url": true,
}

// IsSkippedField reports whether `process_fields` leaves a field alone.
func IsSkippedField(name string) bool {
	return SkippedFields[name] || strings.HasPrefix(name, "_")
}

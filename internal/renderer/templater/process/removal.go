package process

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// The three patterns the removal passes use
// (entry_templates_from_input.py:14-20, :484).
//
// `connectorWordPattern` is the one worth reading. Go's regexp has no
// lookaround, so the Python original —
// `(?<=\s)(?![A-Z])[^\W\d_]\S*(?=\s)` — is reproduced by matching the
// surrounding spaces and putting them back, and by testing the first rune
// separately. The three conditions are: surrounded by whitespace, first
// character not an uppercase ASCII letter, first character a Unicode letter
// that is not a digit or underscore.
//
// What they buy is that `*in*` and `--` **survive** while a bare `in` does not:
// formatting and punctuation are kept, bare words between a missing placeholder
// and its neighbour are dropped.
var (
	uppercaseWordPattern = regexp.MustCompile(`\b[A-Z_]+\b`)
	spacedWordPattern    = regexp.MustCompile(`(\s)(\S+)(?:\s)`)
	multipleSpacePattern = regexp.MustCompile(` {2,}`)

	// `[^A-Za-z0-9.!?\[\]\(\)\*_%]+$` — the trailing run `clean_trailing_parts`
	// removes, which keeps Markdown's own punctuation and drops separators.
	unwantedTrailingPattern = regexp.MustCompile(`[^A-Za-z0-9.!?\[\]()*_%]+$`)
)

// RemoveNotProvidedPlaceholders is `remove_not_provided_placeholders`
// (entry_templates_from_input.py:423-481).
//
// **The missing set is every uppercase word in any template minus the provided
// field names** (`:448-453`). Nothing distinguishes a placeholder from a literal
// uppercase word, so a template containing `CV` or `PHD` treats it as missing
// and removes it. Upstream does not try to tell them apart and neither does
// this.
//
// Two passes, in order, and the order matters: connector words go first, while
// the placeholders they sit between are still there to be found.
func RemoveNotProvidedPlaceholders(
	templates map[string]string,
	fields map[string]string,
) map[string]string {
	missing := missingPlaceholders(templates, fields)
	if len(missing) == 0 {
		return templates
	}

	out := make(map[string]string, len(templates))
	for key, template := range templates {
		text := removeConnectorsOfMissingPlaceholders(template, missing)
		text = multipleSpacePattern.ReplaceAllString(text, " ")

		// The second pattern eats **adjacent non-space characters**, which is
		// what takes the `**` off a missing bold field and the comma after it.
		text = missingPattern(missing).ReplaceAllString(text, "")
		text = multipleSpacePattern.ReplaceAllString(text, " ")
		out[key] = CleanTrailingParts(text)
	}
	return out
}

func missingPlaceholders(templates map[string]string, fields map[string]string) map[string]bool {
	// Upstream joins the templates with a space before scanning (`:448-450`), so
	// a placeholder split across two templates cannot be found by accident.
	joined := make([]string, 0, len(templates))
	for _, template := range templates {
		joined = append(joined, template)
	}
	sort.Strings(joined)

	missing := map[string]bool{}
	for _, word := range uppercaseWordPattern.FindAllString(strings.Join(joined, " "), -1) {
		if _, provided := fields[word]; !provided {
			missing[word] = true
		}
	}
	return missing
}

// missingPattern is `\S*(?:A|B|…)\S*`, which is why removing a placeholder takes
// its surrounding punctuation with it.
func missingPattern(missing map[string]bool) *regexp.Regexp {
	names := make([]string, 0, len(missing))
	for name := range missing {
		names = append(names, regexp.QuoteMeta(name))
	}
	sort.Strings(names)
	return regexp.MustCompile(`\S*(?:` + strings.Join(names, "|") + `)\S*`)
}

// removeConnectorsOfMissingPlaceholders is
// `remove_connectors_of_missing_placeholders` (entry_templates_from_input.py:23-92).
//
// The template is split on placeholders, and each *separator* between two
// placeholders where at least one side is missing has its bare connector words
// deleted. A separator at either end of the string — with no placeholder on one
// side — is left alone.
func removeConnectorsOfMissingPlaceholders(template string, missing map[string]bool) string {
	tokens := splitKeepingPlaceholders(template)

	for i, token := range tokens {
		if uppercaseWordPattern.FindString(token) == token && token != "" {
			continue
		}

		previous := nearestPlaceholder(tokens, i, -1)
		next := nearestPlaceholder(tokens, i, +1)
		if previous == "" || next == "" {
			continue
		}
		if !missing[previous] && !missing[next] {
			continue
		}
		tokens[i] = removeConnectorWords(token)
	}
	return strings.Join(tokens, "")
}

// splitKeepingPlaceholders is Python's `re.split` with a capturing group: the
// separators stay in the result, alternating with the placeholders.
func splitKeepingPlaceholders(template string) []string {
	matches := uppercaseWordPattern.FindAllStringIndex(template, -1)
	tokens := make([]string, 0, len(matches)*2+1)

	previous := 0
	for _, match := range matches {
		tokens = append(tokens, template[previous:match[0]], template[match[0]:match[1]])
		previous = match[1]
	}
	return append(tokens, template[previous:])
}

func nearestPlaceholder(tokens []string, from, step int) string {
	for i := from + step; i >= 0 && i < len(tokens); i += step {
		if tokens[i] != "" && uppercaseWordPattern.FindString(tokens[i]) == tokens[i] {
			return tokens[i]
		}
	}
	return ""
}

// removeConnectorWords deletes each whitespace-surrounded bare word, keeping the
// whitespace — which is what leaves the double spaces the caller then collapses.
//
// Overlapping matches are why this loops rather than calling ReplaceAllString:
// two adjacent connectors share the space between them, and Python's `re.sub`
// resumes **after** each match, so `a b` deletes only `a`. Restarting the scan
// from the end of the replacement reproduces that.
func removeConnectorWords(separator string) string {
	var out strings.Builder
	rest := separator

	for {
		match := spacedWordPattern.FindStringSubmatchIndex(rest)
		if match == nil {
			break
		}
		word := rest[match[4]:match[5]]
		out.WriteString(rest[:match[3]])
		if !isConnectorWord(word) {
			out.WriteString(word)
		}
		// Resume at the word's end, leaving the trailing space for the next
		// match's lookbehind — Python's zero-width `(?=\s)` does the same.
		rest = rest[match[5]:]
	}
	out.WriteString(rest)
	return out.String()
}

// isConnectorWord is `(?![A-Z])[^\W\d_]` applied to the first character: not an
// uppercase ASCII letter, and a letter rather than a digit, underscore or
// punctuation. So `in` and `at` go; `*in*`, `--` and `2` stay.
func isConnectorWord(word string) bool {
	first, size := utf8.DecodeRuneInString(word)
	if size == 0 {
		return false
	}
	if first >= 'A' && first <= 'Z' {
		return false
	}
	return isLetter(first)
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || r > 0x7F && !isDigitOrUnderscore(r)
}

func isDigitOrUnderscore(r rune) bool {
	return (r >= '0' && r <= '9') || r == '_'
}

// CleanTrailingParts is `clean_trailing_parts` (entry_templates_from_input.py:487-514).
//
// Line by line: right-trim, **drop the line entirely if it is now empty**, then
// strip a trailing run of anything outside `[A-Za-z0-9.!?\[\]()*_%]` and
// right-trim again.
//
// Dropping the empty lines is not a formatting nicety — it is what removes a
// template line that consisted only of a missing placeholder, and a port that
// kept them would emit blank rows into every entry.
func CleanTrailingParts(text string) string {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r\n\v\f")
		if trimmed == "" {
			continue
		}
		trimmed = unwantedTrailingPattern.ReplaceAllString(trimmed, "")
		kept = append(kept, strings.TrimRight(trimmed, " \t\r\n\v\f"))
	}
	return strings.Join(kept, "\n")
}

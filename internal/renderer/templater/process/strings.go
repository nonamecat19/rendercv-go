// Package process holds the templater's string and model processors, mirroring
// upstream's `renderer/templater/` file for file (spec 008 plan §4).
//
// The file-for-file mirror matters more here than `AGENTS.md` §9's general rule
// asks: the modules interact in ways spec 008 §4D behavior 43 showed are
// load-bearing — the footer's Typst-source placeholders survive only because the
// escape function holds `#`-commands out — and a reviewer diffing mentally needs
// the same boundaries upstream has.
package process

import (
	"regexp"
	"sort"
	"strings"
)

// SubstitutePlaceholders is `substitute_placeholders`
// (string_processor.py:100-127).
//
// Two behaviors that a straightforward loop of `strings.ReplaceAll` gets wrong:
//
//   - **Longest name first.** The names go into one alternation sorted by
//     descending length, so `YEAR_IN_TWO_DIGITS` matches before `YEAR`. Replacing
//     one name at a time in map order would rewrite the `YEAR` inside
//     `YEAR_IN_TWO_DIGITS` and leave `_IN_TWO_DIGITS` behind.
//   - **The result is stripped**, which callers substituting into the middle of a
//     longer string do not expect and which spec 008 §4B behavior 34 shows is the
//     *only* strip in the whole date path. Trimming each substitution instead
//     would differ by interior spaces.
//
// An empty map returns the string unchanged, **without** the strip — the early
// return happens first (`:122-123`).
func SubstitutePlaceholders(text string, placeholders map[string]string) string {
	if len(placeholders) == 0 {
		return text
	}

	pattern := keywordPattern(placeholders)
	replaced := pattern.ReplaceAllStringFunc(text, func(match string) string {
		return placeholders[match]
	})
	return strings.TrimSpace(replaced)
}

// keywordPattern is `build_keyword_matcher_pattern` without the word-boundary
// option, which `substitute_placeholders` does not use
// (string_processor.py:42-70).
//
// Sorting by descending length is the whole point; ties are broken by the name
// so the pattern is deterministic, which upstream's `sorted(..., key=len,
// reverse=True)` does not guarantee but which no two placeholders exercise.
func keywordPattern(placeholders map[string]string) *regexp.Regexp {
	names := make([]string, 0, len(placeholders))
	for name := range placeholders {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if len(names[i]) != len(names[j]) {
			return len(names[i]) > len(names[j])
		}
		return names[i] < names[j]
	})

	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, regexp.QuoteMeta(name))
	}
	return regexp.MustCompile(strings.Join(quoted, "|"))
}

// CleanURL is `clean_url` (string_processor.py:130-152), one of the two custom
// Jinja filters — and one that **no template uses**: spec 008 §4F behavior 53
// found both are called from Go instead.
//
// Two surprises, both from it being two `str.replace` calls and one suffix test
// rather than a URL parse:
//
//   - the schemes are removed **anywhere in the string**, not only as a prefix,
//     so `https://a/https://b/` becomes `a/b`;
//   - exactly **one** trailing slash is removed, so `a//` becomes `a/`.
func CleanURL(url string) string {
	url = strings.ReplaceAll(url, "https://", "")
	url = strings.ReplaceAll(url, "http://", "")
	return strings.TrimSuffix(url, "/")
}

// Strip is the second custom filter, `lambda string: string.strip()`
// (templater.py:46).
//
// Python's `str.strip()` with no argument removes Unicode whitespace, which is
// what `strings.TrimSpace` does. They agree on every character either calls
// whitespace.
func Strip(text string) string { return strings.TrimSpace(text) }

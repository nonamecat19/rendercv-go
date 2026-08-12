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
	"unicode"
	"unicode/utf8"
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

// MakeKeywordsBold is `make_keywords_bold` (string_processor.py:72-97): every
// occurrence of a configured keyword wrapped in Markdown `**`.
//
// Two differences from SubstitutePlaceholders' matcher, both deliberate
// upstream:
//
//   - it is built with `word_boundary=True`, so `Java` does not match inside
//     `JavaScript`. The placeholder matcher has no boundaries, which is why
//     `YEAR` matches inside `YEAR_IN_TWO_DIGITS` and has to rely on ordering.
//   - it produces **Markdown**, not Typst. It runs before `markdown_to_typst` in
//     the chain (spec 008 §4A behavior 18), so the markers it inserts are
//     converted by the next stage rather than emitted raw.
//
// An empty keyword list returns the string untouched — upstream raises from the
// pattern builder and guards the call, which is the same result.
func MakeKeywordsBold(text string, keywords []string) string {
	if len(keywords) == 0 {
		return text
	}

	pattern := keywordBoldPattern(keywords)
	var out strings.Builder
	pending := 0

	for _, match := range pattern.FindAllStringIndex(text, -1) {
		if !isWordBoundary(text, match[0]) || !isWordBoundary(text, match[1]) {
			continue
		}
		out.WriteString(text[pending:match[0]])
		out.WriteString("**" + text[match[0]:match[1]] + "**")
		pending = match[1]
	}
	out.WriteString(text[pending:])
	return out.String()
}

// isWordBoundary is Python's `\b`, which is **Unicode-aware** where Go's is
// ASCII-only.
//
// Go's `\b` in a pattern would silently never match a keyword whose first or
// last character is non-ASCII: `bold_keywords: ["Café"]` on `"Café au lait"`
// would leave it unbolded, and `bold_keywords` is an unconstrained `list[str]`
// with a diacritics case already in the corpus. So the boundary is checked here
// rather than in the pattern.
func isWordBoundary(text string, at int) bool {
	before, _ := utf8.DecodeLastRuneInString(text[:at])
	after, _ := utf8.DecodeRuneInString(text[at:])
	return isWordRune(before) != isWordRune(after)
}

// isWordRune is Python's `\w` under `re.UNICODE`: a letter, a digit or an
// underscore. `utf8.RuneError` from an empty side is not a word rune, which
// makes a match at either end of the string a boundary.
//
// **It is `unicode.IsDigit` where `pyclass.go`'s `isPyWordRune` is
// `unicode.IsNumber`, and that is a known gap, not a distinction.** `\w` takes
// all of `N*`, so a keyword next to `²` (`No`) or `Ⅷ` (`Nl`) gets a boundary
// here that Python does not give it. Closing it changes `MakeKeywordsBold`'s
// output, which is a different feature with its own differential and no
// measurement yet — spec 011-E §11.1, its own unit. Whoever takes it deletes
// this function and calls `isPyWordRune`.
func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// keywordBoldPattern is `build_keyword_matcher_pattern` with boundaries.
//
// **Longest first**, so `Machine Learning` wins over `Machine`. Ties break on
// the keyword itself, which upstream leaves to set order and no pair of
// keywords in the corpus exercises.
func keywordBoldPattern(keywords []string) *regexp.Regexp {
	// **An empty keyword is kept.** Upstream builds `\b(Java|)\b` from a
	// frozenset that contains `""`, and the empty alternative matches at every
	// word boundary — so `bold_keywords: ["", "Java"]` bolds every word twice
	// over. That output is garbage and it is upstream's; nothing in the schema
	// rejects an empty string, so dropping it here would be a silent divergence.
	unique := make(map[string]bool, len(keywords))
	ordered := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		if unique[keyword] {
			continue
		}
		unique[keyword] = true
		ordered = append(ordered, keyword)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if len(ordered[i]) != len(ordered[j]) {
			return len(ordered[i]) > len(ordered[j])
		}
		return ordered[i] < ordered[j]
	})

	quoted := make([]string, 0, len(ordered))
	for _, keyword := range ordered {
		quoted = append(quoted, regexp.QuoteMeta(keyword))
	}
	return regexp.MustCompile(`(` + strings.Join(quoted, "|") + `)`)
}

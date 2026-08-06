package cv

import (
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var stopWords = map[string]bool{
	"a": true, "and": true, "as": true, "at": true, "but": true,
	"by": true, "for": true, "from": true, "if": true, "in": true,
	"into": true, "like": true, "near": true, "nor": true, "of": true,
	"off": true, "on": true, "onto": true, "or": true, "over": true,
	"so": true, "than": true, "that": true, "to": true, "upon": true,
	"when": true, "with": true, "yet": true,
}

// TitleFromKey formats a section key into its title (spec §3.62-§3.64).
func TitleFromKey(key string) string {
	if strings.Contains(key, " ") || containsUpper(key) {
		return key
	}

	title := strings.ReplaceAll(key, "_", " ")
	words := strings.Split(title, " ")

	for i, word := range words {
		if stopWords[word] {
			words[i] = word
		} else {
			words[i] = capitalize(word)
		}
	}

	return strings.Join(words, " ")
}

// SnakeCaseTitle is a title lowercased with spaces replaced by underscores
// (spec §3.66).
func SnakeCaseTitle(title string) string {
	return strings.ReplaceAll(strings.ToLower(title), " ", "_")
}

func containsUpper(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

// firstRuneTitler applies Unicode's full Titlecase_Mapping, including the
// special cases where one rune maps to several — `ß` → `Ss`, `ﬁ` → `Fi`,
// `ŉ` → `ʼN`. Go's unicode.ToTitle is rune-to-rune and silently leaves those
// unchanged, so it cannot stand in for Python's str.capitalize().
var firstRuneTitler = cases.Title(language.Und, cases.NoLower)

// capitalize is Python's str.capitalize() (section.py:315): titlecase the first
// character and lowercase the rest (spec §3.64, §5.10).
func capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	first := firstRuneTitler.String(string(runes[0]))
	var rest strings.Builder
	for i := 1; i < len(runes); i++ {
		rest.WriteRune(unicode.ToLower(runes[i]))
	}
	return first + rest.String()
}

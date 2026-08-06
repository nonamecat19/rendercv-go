package cv

import (
	"strings"
	"unicode"
)

var stopWords = map[string]bool{
	"a": true, "and": true, "as": true, "at": true, "but": true,
	"by": true, "for": true, "from": true, "if": true, "in": true,
	"into": true, "like": true, "near": true, "nor": true, "of": true,
	"off": true, "on": true, "onto": true, "or": true, "over": true,
	"so": true, "than": true, "that": true, "to": true, "upon": true,
	"when": true, "with": true, "yet": true,
}

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

func capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	first := string(unicode.ToTitle(runes[0]))
	var rest strings.Builder
	for i := 1; i < len(runes); i++ {
		rest.WriteRune(unicode.ToLower(runes[i]))
	}
	return first + rest.String()
}

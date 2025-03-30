package cv_test

import (
	"testing"

	cv "github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
)

func TestTitleFromKey(t *testing.T) {
	tests := []struct {
		key   string
		title string
	}{
		{"this_is_a_test", "This Is a Test"},
		{"welcome_to_rendercv!", "Welcome to Rendercv!"},
		{"Welcome to RenderCV!", "Welcome to RenderCV!"},
		{`\faGraduationCap_education`, `\faGraduationCap_education`},
		{`\faGraduationCap Education`, `\faGraduationCap Education`},
		{"Hello_World", "Hello_World"},
		{"Hello World", "Hello World"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := cv.TitleFromKey(tt.key)
			if got != tt.title {
				t.Errorf("TitleFromKey(%q) = %q, want %q", tt.key, got, tt.title)
			}
		})
	}
}

func TestStopWords(t *testing.T) {
	stopWords := []string{
		"a", "and", "as", "at", "but", "by", "for", "from", "if", "in",
		"into", "like", "near", "nor", "of", "off", "on", "onto", "or",
		"over", "so", "than", "that", "to", "upon", "when", "with", "yet",
	}
	for _, word := range stopWords {
		t.Run("first_"+word, func(t *testing.T) {
			key := word + "_test"
			got := cv.TitleFromKey(key)
			if got[0:len(word)] != word {
				t.Errorf("first word of %q should be %q, got %q", key, word, got)
			}
		})
		t.Run("nonfirst_"+word, func(t *testing.T) {
			key := "hello_" + word + "_world"
			got := cv.TitleFromKey(key)
			expected := "Hello " + word + " World"
			if got != expected {
				t.Errorf("TitleFromKey(%q) = %q, want %q", key, got, expected)
			}
		})
	}
}

func TestCapitalizationUnicode(t *testing.T) {
	tests := []struct {
		key   string
		title string
	}{
		// Spec §5.10: all four cases, verified against CPython's
		// str.capitalize(). The first two exercise Unicode's full
		// Titlecase_Mapping, where one rune maps to several.
		{"ßeta", "Sseta"},
		{"ﬁle", "File"},
		{"ǆab", "ǅab"},
		{"çay", "Çay"},
		{"ǳip", "ǲip"},
		{"ŉx", "ʼNx"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := cv.TitleFromKey(tt.key)
			if got != tt.title {
				t.Errorf("TitleFromKey(%q) = %q, want %q", tt.key, got, tt.title)
			}
		})
	}
}

func TestSnakeCaseTitle(t *testing.T) {
	tests := []struct {
		title string
		snake string
	}{
		{"Education and Training", "education_and_training"},
		{"Hello World", "hello_world"},
		{"My Section", "my_section"},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := cv.SnakeCaseTitle(tt.title)
			if got != tt.snake {
				t.Errorf("SnakeCaseTitle(%q) = %q, want %q", tt.title, got, tt.snake)
			}
		})
	}
}

package entries

import (
	"regexp"
	"testing"
)

// TestMatchDOIPattern is spec §3.11 behavior 19's table, verbatim and in its order.
// Every row was verified against the vendored Python.
func TestMatchDOIPattern(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"10.1109/TASC.2023.3340648", true},
		{"10.1", true},
		{"10.", true},
		{"prefix 10.5/x", true},
		{"-10.5", true},
		{"①10.5", true},
		{".10.5", true},
		{"\t10.5", true},
		{"10.5\n", true},
		{"10", false},
		{"notadoi", false},
		{"abc10.5x", false},
		{"9910.5", false},
		{"_10.5", false},
		{"ü10.5", false},
		{"ß10.5", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			if got := matchDOIPattern(tt.value); got != tt.want {
				t.Errorf("matchDOIPattern(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// TestMatchDOIPatternDivergesFromRegexp pins the reason matchDOIPattern is hand
// written: Go's regexp gives `\b` ASCII semantics, so the literal pattern accepts
// `ü10.5`, which pydantic-core's Rust `regex` rejects (spec §5.1). If someone
// "simplifies" matchDOIPattern to regexp, this test fails.
func TestMatchDOIPatternDivergesFromRegexp(t *testing.T) {
	const value = "ü10.5"

	goRegexp := regexp.MustCompile(`\b10\..*`)
	if !goRegexp.MatchString(value) {
		t.Fatalf("premise changed: Go regexp no longer accepts %q", value)
	}
	if matchDOIPattern(value) {
		t.Errorf("matchDOIPattern(%q) = true, want false — it must not use regexp's ASCII \\b", value)
	}
}

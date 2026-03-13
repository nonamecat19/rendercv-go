package sample

import (
	"strings"
	"testing"
)

// The `cv.name` battery is spec 013 §8's name criterion, and it lives in
// upstream_conformance_test.go: what ruamel emits for a given name is upstream's
// to say, so the expectation is read from a live interpreter rather than from a
// captured list. What stays here is everything that is the port's own — the
// name's route through the document, and the two Python-shaped predicates the
// emitter is built on.

// TestNameLineInDocument checks that the name reaches the generated document
// and reaches nothing else: upstream assigns it to `cv.name` after the model is
// built (`schema/sample_generator.py:73`), so the other occurrences of
// `John Doe` in the sample content — the publication author list among them —
// stay as they are (§3.1 behavior 2).
func TestNameLineInDocument(t *testing.T) {
	john, err := Generate("John Doe", "classic", "english")
	if err != nil {
		t.Fatal(err)
	}
	jane, err := Generate("Jane Roe", "classic", "english")
	if err != nil {
		t.Fatal(err)
	}

	johnLines := strings.Split(john, "\n")
	janeLines := strings.Split(jane, "\n")
	if len(johnLines) != len(janeLines) {
		t.Fatalf("the two documents have %d and %d lines", len(johnLines), len(janeLines))
	}
	var differing []int
	for i := range johnLines {
		if johnLines[i] != janeLines[i] {
			differing = append(differing, i+1)
		}
	}
	if len(differing) != 1 || johnLines[differing[0]-1] != "  name: John Doe" {
		t.Errorf("two names differ on lines %v, want line 3 only", differing)
	}
	if !strings.Contains(jane, "John Doe") {
		t.Error("the sample content's own John Doe was rewritten too")
	}
}

// TestSplitLines pins the boundary set the *representer* uses to decide whether
// a value is multi-line (`schema/sample_generator.py:36`). It is Python's, not
// YAML's: a `\r` starts a new line here but is ordinary text to the emitter,
// which is why `"a\rb"` becomes a literal block with one line in it.
func TestSplitLines(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"a", 1},
		{"a\n", 1},
		{"a\nb", 2},
		{"a\n\nb", 3},
		{"a\r\nb", 2},
		{"a\rb", 2},
		{"a\vb", 2},
		{"a\fb", 2},
		{"a\u0085b", 2},
		{"a\u2028b", 2},
		{"\n", 1},
		{"\n\n", 2},
	}
	for _, c := range cases {
		if got := len(splitLines([]rune(c.in))); got != c.want {
			t.Errorf("splitLines(%q) = %d lines, want %d", c.in, got, c.want)
		}
	}
}

// TestResolvesToString is the first-character bucketing of ruamel's implicit
// resolvers (`resolver.py:24-93`): a scalar is only offered to the resolvers
// registered for its own first character, which is why `_` stays plain even
// though the int pattern would match it.
func TestResolvesToString(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"John Doe", true},
		{"yes", true},   // 1.1 only
		{"true", false}, // 1.2 bool
		{"", false},     // null
		{"~", false},
		{"123", false},
		{"1_000", false},
		{"_", true}, // the int pattern matches, the bucket does not
		{"_1", true},
		{"12:30", true}, // sexagesimal is 1.1 only
		{"2020-01-01", false},
		{"<<", false},
		{"=", false},
		{"*", false},
	}
	for _, c := range cases {
		if got := resolvesToString([]rune(c.in)); got != c.want {
			t.Errorf("resolvesToString(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

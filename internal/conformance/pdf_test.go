package conformance

import "testing"

func TestStripSubsetTag(t *testing.T) {
	cases := []struct{ in, want string }{
		{"GRREMH+SourceSans3-Italic", "SourceSans3-Italic"},
		{"HZBOSV+FontAwesome7Free-Solid-Identity-H", "FontAwesome7Free-Solid-Identity-H"},
		{"NoTag-Regular", "NoTag-Regular"},
		// Five letters, not six, is not the tag shape.
		{"ABCDE+Font", "ABCDE+Font"},
		// Lowercase is not the tag shape.
		{"abcdef+Font", "abcdef+Font"},
		// No `+` at all.
		{"Font", "Font"},
	}
	for _, c := range cases {
		if got := stripSubsetTag(c.in); got != c.want {
			t.Errorf("stripSubsetTag(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSameStringSet(t *testing.T) {
	cases := []struct {
		name string
		a, b []string
		want bool
	}{
		{"equal", []string{"A", "B"}, []string{"A", "B"}, true},
		{"reordered", []string{"A", "B"}, []string{"B", "A"}, true},
		{"duplicate ignored", []string{"A", "A", "B"}, []string{"A", "B"}, true},
		{"missing", []string{"A", "B"}, []string{"A"}, false},
		{"extra", []string{"A"}, []string{"A", "B"}, false},
		{"both empty", nil, nil, true},
	}
	for _, c := range cases {
		if got := sameStringSet(c.a, c.b); got != c.want {
			t.Errorf("%s: sameStringSet(%v, %v) = %v, want %v", c.name, c.a, c.b, got, c.want)
		}
	}
}

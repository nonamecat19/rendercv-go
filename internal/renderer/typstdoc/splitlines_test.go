package typstdoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// splitLines against CPython's own `str.splitlines()`, over the twenty inputs in
// testdata/splitlines.json — which was written by CPython, not by hand.
//
// The port split on `\n` alone until a verifier measured it: a `summary` with
// Windows line endings left a bare carriage return inside the rendered Typst,
// and a ` ` produced one line where upstream produces two. Neither shape is
// in the corpus, so the byte differential could not see either.
func TestSplitLinesMatchesPython(t *testing.T) {
	path := filepath.Join("testdata", "splitlines.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	var cases []struct {
		In  string   `json:"in"`
		Out []string `json:"out"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	if len(cases) == 0 {
		t.Fatalf("%s is empty", path)
	}

	for _, row := range cases {
		t.Run(strconvQuote(row.In), func(t *testing.T) {
			got := splitLines(row.In)
			want := row.Out
			if want == nil {
				want = []string{}
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("splitLines(%q) = %q, want %q", row.In, got, want)
			}
		})
	}
}

func strconvQuote(s string) string {
	if s == "" {
		return "empty"
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r < 0x20 || r > 0x7e {
			out = append(out, '.')
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

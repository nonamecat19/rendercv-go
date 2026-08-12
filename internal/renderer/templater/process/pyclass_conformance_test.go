//go:build conformance

package process

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/conformance"
)

// The vendored interpreter, by path. `uv run` from outside the submodule
// resolves to whatever is on PATH, and the whole point of this test is *which*
// Python.
const pythonInterpreter = "../../../../third_party/rendercv/.venv/bin/python"

// classSweepScript prints Python's `\w` and `\s` as sorted, inclusive ranges
// over every non-surrogate codepoint.
//
// `re.match` rather than `re.fullmatch` is what the emphasis patterns' own
// lookarounds do — they test one character — and surrogates are excluded
// because `chr` produces a codepoint Go cannot hold in a rune's UTF-8 form.
const classSweepScript = `
import json
import re

def ranges(pattern):
    out = []
    for c in range(0x110000):
        if 0xD800 <= c <= 0xDFFF:
            continue
        if not re.match(pattern, chr(c)):
            continue
        if out and out[-1][1] == c - 1:
            out[-1][1] = c
        else:
            out.append([c, c])
    return out

print(json.dumps({"w": ranges(r"\w"), "s": ranges(r"\s")}))
`

// TestPyClassesMatchPython sweeps isPyWordRune and isPySpaceRune against the
// vendored Python's own `re` over all 1,112,064 non-surrogate codepoints.
//
// **This is the gate that says the predicates are Python's classes and not
// merely close to them.** `\w` is 137,936 codepoints and `\s` is 29; a reading
// of the documentation cannot tell you that `\s` contains `U+001C`–`U+001F`,
// which `unicode.IsSpace` does not, or that `\w` contains `Nl` and `No`, which
// `unicode.IsDigit` does not. Both facts were found here and both change the
// parse of an ordinary CV highlight (spec 011-E §3.3).
//
// It also fails if Go's Unicode tables and the vendored Python's diverge —
// today both are 15.0.0 — which is a finding about the submodule bump and not
// something to paper over.
func TestPyClassesMatchPython(t *testing.T) {
	classes := sweepPython(t)

	for _, sweep := range []struct {
		name  string
		key   string
		is    func(rune) bool
		count int
	}{
		{"\\w", "w", isPyWordRune, 137936},
		{"\\s", "s", isPySpaceRune, 29},
	} {
		t.Run(sweep.name, func(t *testing.T) {
			want := membership(classes[sweep.key])
			if got := len(want); got != sweep.count {
				t.Errorf("python's %s is %d codepoints, want %d — the submodule's"+
					" Unicode tables moved", sweep.name, got, sweep.count)
			}
			differences, swept := 0, 0
			for r := rune(0); r < 0x110000; r++ {
				if r >= 0xD800 && r <= 0xDFFF {
					continue
				}
				swept++
				if want[r] == sweep.is(r) {
					continue
				}
				differences++
				if differences <= 10 {
					t.Errorf("%#x (%q): python %s = %v, port = %v",
						r, r, sweep.name, want[r], sweep.is(r))
				}
			}
			if swept != 1112064 {
				t.Errorf("swept %d codepoints, want 1112064", swept)
			}
			if differences > 0 {
				t.Errorf("%d of %d codepoints differ from python's %s",
					differences, swept, sweep.name)
			}
		})
	}
}

// sweepPython runs classSweepScript and returns the two range lists.
func sweepPython(t *testing.T) map[string][][2]rune {
	t.Helper()

	interpreter, err := filepath.Abs(pythonInterpreter)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(interpreter); err != nil {
		if conformance.AllowMissingInput(t) {
			t.Skipf("%s is absent and %s is set, so the character-class sweep does not run (%v)",
				pythonInterpreter, conformance.AllowMissingInputEnvVar, err)
		}
		t.Fatalf("%s is absent, so the character-class sweep would silently not run —"+
			" run `just setup` to check out and sync the submodule, or set %s=1 to skip it"+
			" on purpose (%v)", pythonInterpreter, conformance.AllowMissingInputEnvVar, err)
	}

	command := exec.Command(interpreter, "-c", classSweepScript)
	command.Stderr = os.Stderr
	out, err := command.Output()
	if err != nil {
		t.Fatalf("running the vendored python: %v", err)
	}

	var classes map[string][][2]rune
	if err := json.Unmarshal(out, &classes); err != nil {
		t.Fatalf("python did not print the ranges: %v", err)
	}
	if len(classes) != 2 {
		t.Fatalf("python printed %d classes, want 2", len(classes))
	}
	return classes
}

// membership expands inclusive ranges into a lookup.
func membership(ranges [][2]rune) map[rune]bool {
	in := make(map[rune]bool)
	for _, r := range ranges {
		for c := r[0]; c <= r[1]; c++ {
			in[c] = true
		}
	}
	return in
}

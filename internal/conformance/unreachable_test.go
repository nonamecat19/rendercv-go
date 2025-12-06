// Tests of the inverted comparison. These run under a plain `go test ./...`, for
// the reason `binaryname_test.go` and `requireinput_test.go` do: a guard that
// decides whether a forbidden case stays red is only trustworthy if its own
// verdicts are pinned.
//
// This file is in package `conformance` rather than `conformance_test` because a
// verdict about artifact *contents* needs Golden and Result pointed at real
// directories, and the field holding that path is unexported.
package conformance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEvaluateUnreachable pins each verdict, and case by case the reason for it.
//
// The two that matter most are "only the binary name differs" and "the template
// source differs": before iteration 13, `new_typst_templates` was held red by the
// first — a difference D-009 says to rewrite away — while the second, the
// difference D-008 is actually about, was never looked at at all.
func TestEvaluateUnreachable(t *testing.T) {
	tests := []struct {
		name    string
		stdout  [2]string         // golden, port
		stderr  [2]string         // golden, port
		golden  map[string]string // recorded artifact -> contents
		got     map[string]string // produced artifact -> contents
		phantom []string          // listed in files.txt, never recorded under files/
		want    verdictKind
		evidenc string // substring of the detail line
	}{
		{
			name:   "only the binary name differs",
			stdout: [2]string{"Run rendercv render CV.yaml\n", "Run rendercv-go render CV.yaml\n"},
			want:   verdictMatches,
		},
		{
			name:   "the template source differs",
			stdout: [2]string{"wrote classic/Header.j2.typ\n", "wrote classic/Header.j2.typ\n"},
			golden: map[string]string{"classic/Header.j2.typ": "{{ design.header }}"},
			got:    map[string]string{"classic/Header.j2.typ": "{{ design_header }}"},
			want:   verdictDiffers, evidenc: "classic/Header.j2.typ",
		},
		{
			name:   "only stderr differs",
			stderr: [2]string{"Traceback (most recent call last):\n", "File not found.\n"},
			want:   verdictDiffers, evidenc: "stderr",
		},
		{
			name:   "everything matches",
			stdout: [2]string{"done\n", "done\n"},
			golden: map[string]string{"a.typ": "x"},
			got:    map[string]string{"a.typ": "x"},
			want:   verdictMatches,
		},
		{
			name: "the port produced nothing",
			want: verdictSilent,
		},
		{
			name:   "the file sets differ",
			stdout: [2]string{"done\n", "done\n"},
			golden: map[string]string{"a.typ": "x"},
			got:    map[string]string{"b.typ": "x"},
			want:   verdictDiffers, evidenc: "files",
		},
		{
			name:    "a listed artifact was never recorded",
			stdout:  [2]string{"done\n", "done\n"},
			got:     map[string]string{"a.typ": "x"},
			phantom: []string{"a.typ"},
			want:    verdictUnreadable, evidenc: "a.typ",
		},
		{
			// PDF bytes carry a timestamp and a document ID (spec §1.2), so a
			// difference in them is never evidence of a divergence.
			name:   "PDF bytes are not evidence",
			stdout: [2]string{"done\n", "done\n"},
			golden: map[string]string{"out.pdf": "%PDF-1.7 golden"},
			got:    map[string]string{"out.pdf": "%PDF-1.7 port"},
			want:   verdictMatches,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			golden, got := fixturePair(t, tc.stdout, tc.stderr, tc.golden, tc.got)
			golden.Files = append(golden.Files, tc.phantom...)

			verdict := evaluateUnreachable(golden, got)
			if verdict.kind != tc.want {
				t.Fatalf("verdict = %v (%s), want %v", verdict.kind, verdict.detail, tc.want)
			}
			if tc.evidenc != "" && !strings.Contains(verdict.detail, tc.evidenc) {
				t.Errorf("detail = %q, want it to name %q", verdict.detail, tc.evidenc)
			}
		})
	}
}

// fixturePair writes both sides to disk in the layout LoadGolden and Run produce:
// the golden's artifacts under `<dir>/files/`, the port's directly under its own.
func fixturePair(t *testing.T, stdout, stderr [2]string, want, got map[string]string) (Golden, Result) {
	t.Helper()

	goldenDir := t.TempDir()
	gotDir := t.TempDir()

	g := Golden{Name: "fixture", dir: goldenDir, Stdout: stdout[0], Stderr: stderr[0], PNGs: map[string]string{}}
	for rel, content := range want {
		writeFile(t, filepath.Join(goldenDir, "files", rel), content)
		g.Files = append(g.Files, rel)
	}

	r := Result{dir: gotDir, Stdout: stdout[1], Stderr: stderr[1], PNGs: map[string]string{}}
	for rel, content := range got {
		writeFile(t, filepath.Join(gotDir, rel), content)
		r.Files = append(r.Files, rel)
	}
	return g, r
}

// TestUnreachableRebindMatchesNormalPath pins the property the fix turns on: the
// inverted comparison rebinds the port's name exactly as `parity_test.go` does,
// so a case can never be held red by D-009 while claiming to be held by another
// divergence.
func TestUnreachableRebindMatchesNormalPath(t *testing.T) {
	const line = "│ rendercv-go render CV.yaml                                                 │"

	golden, got := fixturePair(t,
		[2]string{RebindBinaryName(line) + "\n", line + "\n"},
		[2]string{"", ""}, nil, nil)

	if verdict := evaluateUnreachable(golden, got); verdict.kind != verdictMatches {
		t.Errorf("a stdout that only spells the binary differently is %v (%s), want verdictMatches",
			verdict.kind, verdict.detail)
	}
}

// TestUnreachableRebindIsNotBlanket guards the other direction: rebinding must not
// erase a real difference that merely sits on a line containing the binary name.
func TestUnreachableRebindIsNotBlanket(t *testing.T) {
	golden, got := fixturePair(t,
		[2]string{"rendercv render CV.yaml\n", "rendercv-go render Resume.yaml\n"},
		[2]string{"", ""}, nil, nil)

	if verdict := evaluateUnreachable(golden, got); verdict.kind != verdictDiffers {
		t.Errorf("a real stdout difference is %v (%s), want verdictDiffers",
			verdict.kind, verdict.detail)
	}
}

// TestUnreachableArtifactBytes pins the readers the verdict depends on, including
// the error a missing recording produces.
func TestUnreachableArtifactBytes(t *testing.T) {
	golden, got := fixturePair(t, [2]string{"", ""}, [2]string{"", ""},
		map[string]string{"a.typ": "golden"}, map[string]string{"a.typ": "port"})

	if raw, err := golden.artifactBytes("a.typ"); err != nil || string(raw) != "golden" {
		t.Errorf("golden.artifactBytes = %q, %v", raw, err)
	}
	if raw, err := got.artifactBytes("a.typ"); err != nil || string(raw) != "port" {
		t.Errorf("got.artifactBytes = %q, %v", raw, err)
	}
	if _, err := golden.artifactBytes("absent.typ"); !os.IsNotExist(err) {
		t.Errorf("golden.artifactBytes(absent) error = %v, want a not-exist error", err)
	}
}

// Tests of the harness itself. These run under a plain `go test ./...` — the parity
// suite is only as trustworthy as the fixtures it reads, so the fixtures are checked
// unconditionally, long before rendercv-go can render anything.
package conformance_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/conformance"
)

// TestCorpusIsWellFormed guards the corpus declaration.
func TestCorpusIsWellFormed(t *testing.T) {
	corpus := conformance.LoadCorpus(t)
	root := conformance.RepoRoot(t)

	if len(corpus.Cases) == 0 {
		t.Fatal("corpus declares no cases")
	}
	if corpus.Env["COLUMNS"] == "" {
		t.Error("corpus must pin COLUMNS: Rich lays out its panels to terminal width, " +
			"so unpinned width makes CLI goldens machine-dependent")
	}

	seen := map[string]bool{}
	for _, c := range corpus.Cases {
		if seen[c.Name] {
			t.Errorf("duplicate case name %q", c.Name)
		}
		seen[c.Name] = true

		if len(c.Args) == 0 {
			t.Errorf("case %s: no CLI arguments", c.Name)
		}
		switch c.Axis {
		case "artifacts", "cli", "errors":
		default:
			t.Errorf("case %s: unknown axis %q", c.Name, c.Axis)
		}
		for _, f := range c.Files {
			src := filepath.Join(root, "third_party/rendercv", f.Src)
			if _, err := os.Stat(src); err != nil {
				t.Errorf("case %s: input %s missing from the submodule — "+
					"run `just submodule` (%v)", c.Name, f.Src, err)
			}
		}
	}
}

// TestCorpusCoversTheContract checks that the corpus actually exercises what
// specs/000-parity-contract/spec.md §7 requires.
func TestCorpusCoversTheContract(t *testing.T) {
	corpus := conformance.LoadCorpus(t)

	names := make([]string, 0, len(corpus.Cases))
	byAxis := map[string]int{}
	for _, c := range corpus.Cases {
		names = append(names, c.Name)
		byAxis[c.Axis]++
	}
	joined := strings.Join(names, " ")

	for _, theme := range []string{
		"classic", "ember", "engineeringclassic", "engineeringresumes",
		"harvard", "ink", "moderncv", "opal", "sb2nov",
	} {
		if !strings.Contains(joined, "theme_"+theme) {
			t.Errorf("no corpus case renders the %s theme", theme)
		}
	}

	// An RTL locale is required: it exercises the bidirectional layout path that
	// nothing else touches.
	if !strings.Contains(joined, "locale_arabic") {
		t.Error("corpus must include an RTL locale case")
	}
	if byAxis["errors"] < 5 {
		t.Errorf("contract §7 requires at least 5 invalid-input cases, corpus has %d",
			byAxis["errors"])
	}
}

// TestGoldensExistForEveryCase catches a corpus case added without regenerating the
// fixtures, which would otherwise surface as a confusing parity failure.
func TestGoldensExistForEveryCase(t *testing.T) {
	corpus := conformance.LoadCorpus(t)
	root := conformance.RepoRoot(t)

	for _, c := range corpus.Cases {
		dir := filepath.Join(root, "testdata/golden", c.Name)
		for _, f := range []string{"files.txt", "stdout.txt", "stderr.txt", "exit_code.txt", "pngs.txt", "case.json"} {
			if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
				t.Errorf("case %s: golden %s missing — run `just golden` (%v)", c.Name, f, err)
			}
		}
	}
}

// TestManifestMatchesTheGoldenTree keeps the manifest honest: it is what CI uses to
// detect fixture drift, so an entry that no longer corresponds to a file on disk
// silently weakens the check.
func TestManifestMatchesTheGoldenTree(t *testing.T) {
	root := conformance.RepoRoot(t)
	goldens := filepath.Join(root, "testdata/golden")

	manifest := conformance.LoadManifest(t, filepath.Join(goldens, "manifest.json"))
	if manifest.UpstreamSHA == "" {
		t.Fatal("manifest records no upstream SHA")
	}

	onDisk := map[string]bool{}
	err := filepath.Walk(goldens, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(goldens, path)
		if err != nil {
			return err
		}
		if rel != "manifest.json" {
			onDisk[rel] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking goldens: %v", err)
	}

	for path := range manifest.Files {
		if !onDisk[path] {
			t.Errorf("manifest lists %s, which is not on disk", path)
		}
	}
	for path := range onDisk {
		if _, ok := manifest.Files[path]; !ok {
			t.Errorf("%s is not covered by the manifest", path)
		}
	}
}

// TestNormalizeOnlyRemovesTimings pins the one transform applied to CLI output. If it
// ever removes more than wall-clock timings, every CLI and error golden silently
// weakens.
func TestNormalizeOnlyRemovesTimings(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"milliseconds", "✓ 42 ms  Generated Typst\n", "✓ <duration> Generated Typst\n"},
		{"seconds", "✓ 1.2 s   Generated PDF\n", "✓ <duration> Generated PDF\n"},
		{"padding absorbed", "✓ 7 ms      x\n", "✓ <duration> x\n"},
		{"no timing", "│ cv.email.0 │ not_a_valid_email │\n", "│ cv.email.0 │ not_a_valid_email │\n"},
		{"trailing newline added", "no newline", "no newline\n"},
		{"empty stays empty", "", ""},
		{"crlf normalised", "a\r\nb\r\n", "a\nb\n"},
		{"version string untouched", "RenderCV v2.8\n", "RenderCV v2.8\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := conformance.Normalize(tc.in); got != tc.want {
				t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

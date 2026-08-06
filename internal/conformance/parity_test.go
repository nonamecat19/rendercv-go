//go:build conformance

// Parity suite. Runs only under `go test -tags conformance` (`just test-parity`), so the
// ordinary `go test ./...` stays fast and green while the port is incomplete.
//
// These tests are written before the implementation exists and are expected to fail
// until the corresponding iteration in specs/STATE.md lands. They fail loudly rather
// than skipping: a skipped parity case is how a port convinces itself it is done.
package conformance_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/conformance"
)

// TestParity runs every corpus case and compares rendercv-go against the recorded
// behavior of the vendored Python RenderCV.
func TestParity(t *testing.T) {
	corpus := conformance.LoadCorpus(t)

	for _, c := range corpus.Cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()

			golden := conformance.LoadGolden(t, c.Name)
			got := conformance.Run(t, c, corpus.Env)

			// Axis 2: exit code and output layout.
			conformance.AssertExitCode(t, golden.ExitCode, got.ExitCode)
			conformance.AssertFileSet(t, golden.Files, got.Files)

			// Axis 1: every text artifact, byte for byte.
			for _, rel := range golden.Files {
				if !isByteComparable(rel) {
					continue
				}
				if !containsPath(got.Files, rel) {
					continue // already reported by AssertFileSet
				}
				conformance.AssertBytes(t, rel, golden.Artifact(t, rel), got.Artifact(t, rel))
			}

			// Spec §1.2: PNG page count and pixel dimensions.
			conformance.AssertPNGGeometry(t, golden.PNGs, got.PNGs)

			// Axes 2 and 4: the exact text RenderCV prints, including the error panels.
			conformance.AssertText(t, "stdout", golden.Stdout, got.Stdout)
			conformance.AssertText(t, "stderr", golden.Stderr, got.Stderr)
		})
	}
}

// TestSchemaParity is axis 3: the generated JSON schema must equal upstream's, key
// order included.
func TestSchemaParity(t *testing.T) {
	root := conformance.RepoRoot(t)
	want := readFile(t, filepath.Join(root, "third_party/rendercv/schema.json"))

	got := conformance.Run(t, conformance.Case{
		Name: "schema",
		Axis: "schema",
		Args: []string{"schema"},
	}, nil)

	conformance.AssertExitCode(t, 0, got.ExitCode)
	conformance.AssertText(t, "schema.json", want, got.Stdout)
}

// isByteComparable reports whether an artifact is subject to byte-for-byte parity.
//
// PDFs are not: they embed a creation timestamp and a document ID, and the Go build
// uses a different Typst binary. They are compared on page count, extracted text, page
// dimensions and font names instead — implemented in iteration 10, which owns the
// wazero/WASI Typst path. PNG bytes are likewise out (see AssertPNGGeometry).
func isByteComparable(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".pdf", ".png":
		return false
	default:
		return true
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(raw)
}

func containsPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

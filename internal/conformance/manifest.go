package conformance

import (
	"encoding/json"
	"os"
	"testing"
)

// Manifest records what testdata/golden was generated from. It mirrors the struct in
// tools/gengolden, which is its only author.
type Manifest struct {
	UpstreamSHA     string            `json:"upstream_sha"`
	UpstreamVersion string            `json:"upstream_version"`
	Generator       string            `json:"generator"`
	CaseCount       int               `json:"case_count"`
	Files           map[string]string `json:"files"` // path relative to testdata/golden -> sha256
}

// LoadManifest reads testdata/golden/manifest.json.
func LoadManifest(t *testing.T, path string) Manifest {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading manifest: %v — run `just golden`", err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parsing manifest: %v", err)
	}
	return m
}

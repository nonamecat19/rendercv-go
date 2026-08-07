//go:build conformance

package yamlreader_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

type coordFixture struct {
	File string           `json:"file"`
	Data map[string][]int `json:"data"`
}

func TestCoordinateParity(t *testing.T) {
	fixturePath := filepath.Join("testdata", "coords", "coords.json")
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("reading coordinate fixture: %v", err)
	}
	var fixtures []coordFixture
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatalf("parsing coordinate fixture: %v", err)
	}

	// The two real documents carry 620 of the 656 paths between them. Guarding
	// the total means a fixture silently dropped from the probe's list fails
	// here rather than shrinking the corpus unnoticed.
	total := 0
	for _, fx := range fixtures {
		total += len(fx.Data)
	}
	if total < 600 {
		t.Fatalf("the coordinate corpus has %d paths, want the full set — has a"+
			" document been dropped from tools/yamlprobe?", total)
	}

	for _, fx := range fixtures {
		t.Run(fx.File, func(t *testing.T) {
			// Repo-relative, so the shaped fixtures and the two submodule
			// documents live in one list.
			yamlPath := filepath.Join("..", "..", "..", fx.File)
			doc, err := yamlreader.ReadFile(yamlPath)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}

			for path, want := range fx.Data {
				span, ok := lookupKeySpan(doc, path)
				if !ok {
					t.Errorf("path %q not found in parsed document", path)
					continue
				}
				got := toRuamelCoords(span, path)
				if !coordsEqual(got, want) {
					t.Errorf("path %q:\n  got  %v\n  want %v", path, got, want)
				}
			}
		})
	}
}

func lookupKeySpan(node *yamldoc.Node, path string) (yamldoc.Span, bool) {
	parts := splitPath(path)
	if len(parts) == 0 {
		return yamldoc.Span{}, false
	}

	current := node
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if isIndex(part) {
			idx := parseIndex(part)
			if current.Kind != yamldoc.KindSequence || idx >= len(current.Elems) {
				return yamldoc.Span{}, false
			}
			current = current.Elems[idx]
		} else {
			if current.Kind != yamldoc.KindMapping {
				return yamldoc.Span{}, false
			}
			found := false
			for _, item := range current.Items {
				if item.Key == part {
					current = item.Value
					found = true
					break
				}
			}
			if !found {
				return yamldoc.Span{}, false
			}
		}
	}

	last := parts[len(parts)-1]
	if isIndex(last) {
		idx := parseIndex(last)
		if current.Kind != yamldoc.KindSequence || idx >= len(current.Elems) {
			return yamldoc.Span{}, false
		}
		return current.Elems[idx].Span, true
	}

	if current.Kind != yamldoc.KindMapping {
		return yamldoc.Span{}, false
	}
	for _, item := range current.Items {
		if item.Key == last {
			return item.KeySpan, true
		}
	}
	return yamldoc.Span{}, false
}

func isSequencePath(path string) bool {
	parts := splitPath(path)
	if len(parts) == 0 {
		return false
	}
	return isIndex(parts[len(parts)-1])
}

func toRuamelCoords(span yamldoc.Span, path string) []int {
	if isSequencePath(path) {
		return []int{span.Start.Line - 1, span.Start.Column - 1}
	}
	return []int{
		span.Start.Line - 1,
		span.Start.Column - 1,
		span.End.Line - 1,
		span.End.Column - 1,
	}
}

func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	if path[0] == '.' {
		path = path[1:]
	}
	var parts []string
	current := ""
	for _, ch := range path {
		if ch == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func isIndex(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func parseIndex(s string) int {
	var n int
	for _, ch := range s {
		n = n*10 + int(ch-'0')
	}
	return n
}

func coordsEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

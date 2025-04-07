package cv_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// forbidden is the parent package. It imports models/cv, so the reverse edge is
// an import cycle. ValidationContext lives in models/valctx and the path types
// in models/inputpath precisely so that cv never needs it (tasks 003 T1, T2).
//
// The cycle would otherwise stay invisible until iteration 3's T20 makes
// models.Validate call cv.Validate, which is far too late to discover it, so
// the edge is asserted here rather than left to review.
const forbidden = "github.com/nonamecat19/rendercv-go/internal/schema/models"

// TestCvDoesNotImportModels walks every Go file under models/cv, including
// subpackages and test files, and fails on an import of the parent package.
// Subpackages of the parent are fine — only the exact path is the cycle.
func TestCvDoesNotImportModels(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolving package directory: %v", err)
	}

	fset := token.NewFileSet()
	walked := 0

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		walked++

		for _, spec := range file.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if imported == forbidden {
				rel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					rel = path
				}
				t.Errorf(
					"%s imports %s, which imports this package: that is a cycle."+
						" Use models/valctx or models/inputpath instead.",
					rel, forbidden,
				)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}

	// A silent zero-file walk would make this test pass by doing nothing.
	if walked == 0 {
		t.Fatal("walked no Go files; the guard is not actually checking anything")
	}
}

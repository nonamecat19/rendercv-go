package modelbuilder

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

func buildResult(t *testing.T, src string) *BuildResult {
	t.Helper()
	doc, err := yamlreader.ReadString(src)
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	return &BuildResult{Document: doc}
}

// A valid document builds a model and no error.
func TestBuildModelAccepts(t *testing.T) {
	model, err := BuildModel(
		buildResult(t, "cv:\n  name: John Doe\n"), &valctx.ValidationContext{},
	)
	if err != nil {
		t.Fatalf("BuildModel = %v", err)
	}
	if model == nil {
		t.Fatal("model is nil")
	}
}

// A schema failure comes back as user-facing records — **final** ones, with the
// dictionary applied and the period appended.
//
// The two halves matter together: a caller that received raw records would
// print `Field required` where upstream prints `This field is required.`
func TestBuildModelReturnsFinalRecords(t *testing.T) {
	_, err := BuildModel(
		buildResult(t, "cv:\n  social_networks:\n    - username: johndoe\n"),
		&valctx.ValidationContext{},
	)

	var userErr *schemaerr.UserValidationError
	if !errors.As(err, &userErr) {
		t.Fatalf("err = %v (%T), want *schemaerr.UserValidationError", err, err)
	}
	if len(userErr.Errors) != 1 {
		t.Fatalf("errors = %+v, want exactly one", userErr.Errors)
	}

	record := userErr.Errors[0]
	if record.Message != "This field is required." {
		t.Errorf("message = %q, want the dictionary's replacement — these records"+
			" must be final", record.Message)
	}
	if got := strings.Join(record.SchemaLocation, "."); got != "cv.social_networks.0.network" {
		t.Errorf("location = %q", got)
	}
	// Coordinates were resolved against the document, not carried from the
	// producer.
	if record.YamlLocation == nil {
		t.Error("coordinates are absent")
	}
}

// Parse is not idempotent: it applies the dictionary and appends the period, so
// a second call double-substitutes some messages and adds a second period to
// others.
//
// The invariant is therefore "exactly one caller", and it is checked
// structurally rather than trusted — a walk over every non-test Go file under
// internal/, counting call sites.
func TestParseHasOneCaller(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolving internal/: %v", err)
	}

	fset := token.NewFileSet()
	var callers []string
	walked := 0

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		walked++

		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Parse" {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || pkg.Name != "errorpipeline" {
				return true
			}
			relative, _ := filepath.Rel(root, path)
			callers = append(callers, relative)
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking internal/: %v", err)
	}
	if walked < 20 {
		t.Fatalf("walked %d files, want the whole tree", walked)
	}

	if len(callers) != 1 {
		t.Errorf("errorpipeline.Parse has %d callers (%v), want exactly one —"+
			" it is not idempotent", len(callers), callers)
	}
}

// publicationsWithDOIs is a CV whose publication section carries one entry per
// `doi`, each otherwise minimal and valid.
func publicationsWithDOIs(dois ...string) string {
	var b strings.Builder
	b.WriteString("cv:\n  name: John Doe\n  sections:\n    publications:\n")
	for _, doi := range dois {
		b.WriteString("      - title: T\n        authors:\n          - A\n")
		b.WriteString("        doi: " + doi + "\n")
	}
	return b.String()
}

// longDOI is a `doi` whose generated URL exceeds the 2083-character limit —
// `https://doi.org/` is 16 characters, so 2068 of `doi` makes 2084.
func longDOI(n int) string { return "10." + strings.Repeat("a", n-3) }

// The DOI-URL-length failure has to reach the user, at the entry it belongs to.
//
// **It did not.** The producer gave the record an empty schema location, so the
// splice rebuilt it as its own wrapper's location and dedup deleted it as a
// duplicate — upstream printed two rows and the port printed one, for every
// input that trips this rule. The splice and dedup are both correct and pinned
// (`errorpipeline/splice_test.go`); the producer was wrong.
//
// The index rows are what make this more than a one-shape fix: the entry's
// position is the only thing separating the record from its wrapper, so a fix
// that hard-coded `0` would pass the first row and fail the second.
//
// Every expectation was measured against the vendored Python through
// `build_rendercv_dictionary_and_model`, and the boundary either side of it:
// 2083 characters validates, 2084 does not.
func TestDOIURLLengthReachesTheUser(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{{
		name: "at the limit, no error at all",
		src:  publicationsWithDOIs(longDOI(2067)),
	}, {
		name: "one over the limit",
		src:  publicationsWithDOIs(longDOI(2068)),
		want: []string{"cv.sections.publications", "cv.sections.publications.0"},
	}, {
		name: "the second entry, not the first",
		src:  publicationsWithDOIs("10.ok", longDOI(2200)),
		want: []string{"cv.sections.publications", "cv.sections.publications.1"},
	}, {
		name: "both entries report",
		src:  publicationsWithDOIs(longDOI(2200), longDOI(2300)),
		want: []string{
			"cv.sections.publications",
			"cv.sections.publications.0",
			"cv.sections.publications.1",
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := BuildModel(buildResult(t, test.src), &valctx.ValidationContext{})
			if len(test.want) == 0 {
				if err != nil {
					t.Fatalf("BuildModel = %v, want no error", err)
				}
				return
			}

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("err = %v (%T), want *schemaerr.UserValidationError", err, err)
			}
			got := make([]string, 0, len(userErr.Errors))
			for _, record := range userErr.Errors {
				got = append(got, strings.Join(record.SchemaLocation, "."))
			}
			if strings.Join(got, " | ") != strings.Join(test.want, " | ") {
				t.Errorf("locations = %v\nwant        %v", got, test.want)
			}
			for _, record := range userErr.Errors[1:] {
				const want = "URL should have at most 2083 characters."
				if record.Message != want {
					t.Errorf("message = %q, want %q", record.Message, want)
				}
			}
		})
	}
}

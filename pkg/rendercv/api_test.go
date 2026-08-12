package rendercv_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The two checks in this file are the iteration's own gates (spec 016 §5.1,
// §5.4). Both are the shape internal/kindguard established: a convention that
// lives only in a comment is not a constraint, and this port has shipped
// vacuous tests before (STATE.md pass 20). Each has a companion test below
// proving it rejects a violation.

// citation is a reference to an upstream Python source line. A doc comment
// merely containing a colon is not enough — that is exactly how this check
// would go vacuous.
//
// Spec §5.4 wrote the shape as `src/rendercv/...:LINE`. The rest of this port
// cites upstream by file name and line (`pdf_png.py:16-44`), without the
// directory, and matching one convention in one package would be worse than
// following the one already in use — so what is required is a `.py` file and a
// line number, which is the part that carries the meaning.
var citation = regexp.MustCompile(`[A-Za-z0-9_/]+\.py:\d+`)

// TestEveryExportedSymbolCitesItsUpstreamConstruct is AGENTS.md §9: every
// exported symbol in this package carries a doc comment naming the upstream
// construct it mirrors.
func TestEveryExportedSymbolCitesItsUpstreamConstruct(t *testing.T) {
	for name, doc := range exportedDocs(t, ".") {
		if doc == "" {
			t.Errorf("%s has no doc comment; AGENTS.md §9 requires one naming its upstream construct", name)
			continue
		}
		if !citation.MatchString(doc) {
			t.Errorf("%s's doc comment cites no upstream construct; expected a src/rendercv/...:LINE reference", name)
		}
	}
}

// TestNoInternalTypeInAnExportedSignature is spec §5.1 as amended by plan §3.3:
// a caller must never have to name an internal path. Aliases declared in
// types.go are how the model and error types cross the boundary, and they are
// the only mention allowed.
func TestNoInternalTypeInAnExportedSignature(t *testing.T) {
	// **types.go is not exempt.** Only an *alias declaration* is allowed to
	// name an internal package, and an alias is a TypeSpec — never a function
	// signature. Exempting the whole file, as this test first did, let an
	// exported function declared there leak an internal type unnoticed.
	for _, file := range parseSources(t, ".") {
		// The qualifier in `yamldoc.Node` says nothing about where yamldoc
		// lives, so it has to be resolved through the file's imports. Checking
		// the qualifier itself is how the first version of this test passed a
		// planted violation.
		imports := importPaths(file)

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			// **Methods count**, for the same reason they do in the citation
			// check: a receiver does not make a signature less public.
			if !ok || !fn.Name.IsExported() {
				continue
			}
			for _, qualifier := range signatureQualifiers(fn) {
				path, ok := imports[qualifier]
				if !ok {
					continue
				}
				if strings.Contains(path, "/internal/") {
					t.Errorf("%s's signature names %s.…, which is %s; an internal type must cross the boundary as an alias in types.go",
						qualifiedName(fn), qualifier, path)
				}
			}
		}
	}
}

// TestEveryExportedFieldIsDocumented covers what the citation check cannot
// sensibly demand of a field.
//
// A field of an exported struct is reachable from a caller, so it is part of
// the surface — but requiring a `.py` citation on each of BuildOptions' fifteen
// would be noise, since the type itself cites the construct it mirrors
// field-for-field. What it must not be is *undocumented*, which is how a field
// gets added to a public API by accident.
func TestEveryExportedFieldIsDocumented(t *testing.T) {
	for _, file := range parseSources(t, ".") {
		for _, decl := range file.Decls {
			node, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range node.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !typeSpec.Name.IsExported() {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				// Fields are commonly documented in groups — one comment
				// naming several fields, which the AST hangs on the first of
				// them. So a field counts as documented if it carries its own
				// comment or if some comment in the struct names it. A field
				// added without either is what this catches.
				described := typeSpec.Doc.Text()
				for _, field := range structType.Fields.List {
					described += field.Doc.Text() + field.Comment.Text()
				}
				for _, field := range structType.Fields.List {
					for _, name := range field.Names {
						if !name.IsExported() {
							continue
						}
						own := field.Doc.Text() != "" || field.Comment.Text() != ""
						if !own && !strings.Contains(described, name.Name) {
							t.Errorf("%s.%s is named in no comment; an exported field is part of the surface",
								typeSpec.Name.Name, name.Name)
						}
					}
				}
			}
		}
	}
}

// parseSources parses every non-test Go file in dir.
//
// It walks the directory rather than calling parser.ParseDir, which Go 1.25
// deprecated.
func parseSources(t *testing.T, dir string) map[string]*ast.File {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files[path] = file
	}
	return files
}

// importPaths maps each import's local name to its full path, honouring an
// explicit rename and otherwise taking the last path segment, which is what a
// qualifier in a signature refers to.
func importPaths(file *ast.File) map[string]string {
	paths := map[string]string{}
	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, `"`)
		name := path[strings.LastIndex(path, "/")+1:]
		if spec.Name != nil {
			name = spec.Name.Name
		}
		paths[name] = path
	}
	return paths
}

// signatureQualifiers returns every package qualifier appearing in a function's
// parameters and results.
func signatureQualifiers(fn *ast.FuncDecl) []string {
	var found []string
	record := func(fields *ast.FieldList) {
		if fields == nil {
			return
		}
		for _, field := range fields.List {
			ast.Inspect(field.Type, func(node ast.Node) bool {
				if sel, ok := node.(*ast.SelectorExpr); ok {
					if ident, ok := sel.X.(*ast.Ident); ok {
						found = append(found, ident.Name)
					}
				}
				return true
			})
		}
	}
	record(fn.Type.Params)
	record(fn.Type.Results)
	return found
}

func exportedDocs(t *testing.T, dir string) map[string]string {
	t.Helper()
	docs := map[string]string{}
	for _, file := range parseSources(t, dir) {
		for _, decl := range file.Decls {
			switch node := decl.(type) {
			case *ast.FuncDecl:
				// **Methods count too.** Skipping them left (*Model).Name
				// undocumented against AGENTS.md §9 for a whole iteration; a
				// receiver does not make a symbol less exported.
				if node.Name.IsExported() {
					docs[qualifiedName(node)] = node.Doc.Text()
				}
			case *ast.GenDecl:
				for _, spec := range node.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Name.IsExported() {
							doc := s.Doc.Text()
							if doc == "" {
								doc = node.Doc.Text()
							}
							docs[s.Name.Name] = doc
						}
					case *ast.ValueSpec:
						for _, name := range s.Names {
							if name.IsExported() {
								doc := s.Doc.Text()
								if doc == "" {
									doc = node.Doc.Text()
								}
								docs[name.Name] = doc
							}
						}
					}
				}
			}
		}
	}
	return docs
}

// qualifiedName spells a method as Receiver.Method so a failure says which type
// it is on.
func qualifiedName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	receiver := ""
	ast.Inspect(fn.Recv.List[0].Type, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok && receiver == "" {
			receiver = ident.Name
		}
		return true
	})
	return receiver + "." + fn.Name.Name
}

//go:build conformance

package clidiff

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// fallbackStrings are §4.8's two empty-message substitutions. Upstream's path B
// prints the first when `e.message` is falsy
// (`cli/render_command/progress_panel.py:129`) and path A prints nothing at all
// (`cli/error_handler.py:40`). Behavior 38 measured that **no upstream raise
// site constructs a RenderCVUserError without a message**, so both branches are
// dead code upstream and the port must not grow a live version of either.
var fallbackStrings = []string{
	"An unknown error occurred.",
}

// sourceRoots are the directories holding the port's own Go source. `pkg/` is
// listed for the day it exists; a missing root is skipped rather than fatal.
var sourceRoots = []string{"internal", "cmd", "pkg"}

// TestUserErrorMessagesAreNonEmpty is behavior 38 of
// specs/013-parity-closeout/spec.md, checked the only way it can be checked
// without a raise site inventory by hand: every error the port constructs is
// enumerated from source and asserted to carry a non-empty message.
//
// The enumeration resolves a constructor's first argument to a string when it
// is a literal or a package-level constant of the same package. What it cannot
// resolve — a format built at run time, a message threaded in from a caller —
// it lists rather than passes over silently, so the reader can see the size of
// the unmeasured remainder.
func TestUserErrorMessagesAreNonEmpty(t *testing.T) {
	root := repoRoot(t)

	var (
		empty      []string
		fallback   []string
		unresolved []string
		total      int
		userErrors int
	)

	var dirs []string
	for _, srcRoot := range sourceRoots {
		dirs = append(dirs, packageDirs(t, filepath.Join(root, srcRoot))...)
	}

	for _, dir := range dirs {
		fset := token.NewFileSet()
		files := parsePackage(t, fset, dir)
		constants := stringConstants(files)

		for _, file := range files {
			ast.Inspect(file, func(n ast.Node) bool {
				var messageExpr ast.Expr
				switch node := n.(type) {
				case *ast.CallExpr:
					if !isErrorConstructor(node.Fun) || len(node.Args) == 0 {
						return true
					}
					messageExpr = node.Args[0]
				case *ast.CompositeLit:
					if !isUserError(node.Type) {
						return true
					}
					userErrors++
					// A literal with no Message field at all zero-values it,
					// which is exactly §4.8's empty message.
					messageExpr = fieldValue(node, "Message")
					if messageExpr == nil {
						empty = append(empty, position(fset, root, node.Pos()))
						total++
						return true
					}
				default:
					return true
				}
				total++

				where := position(fset, root, n.Pos())

				message, resolved := resolveString(messageExpr, constants, 0)
				switch {
				case !resolved:
					unresolved = append(unresolved, where)
				case message == "":
					empty = append(empty, where)
				default:
					for _, s := range fallbackStrings {
						if strings.Contains(message, s) {
							fallback = append(fallback, where)
						}
					}
				}
				return true
			})
		}
	}

	if total == 0 {
		t.Fatal("found no error constructors at all — the walk is broken, not the port")
	}
	if userErrors == 0 {
		t.Fatal("found no schemaerr.UserError construction at all — the walk is broken, " +
			"not the port")
	}
	t.Logf("%d error constructors, %d of them schemaerr.UserError; %d unresolved",
		total, userErrors, len(unresolved))

	// An unresolvable message is a failure, not a note. Behavior 38's claim is
	// that *no* construction can carry an empty message; a message this walk
	// cannot read is a message the claim is not made about, and logging it
	// lets the guarantee erode one `&schemaerr.UserError{Message: m}` at a
	// time. The remainder is 0 today, so this costs nothing until someone
	// writes the construction that would have slipped through — at which point
	// the fix is to give resolveString the shape, or to hoist the message to
	// where it can be read.
	for _, where := range unresolved {
		t.Errorf("%s constructs an error whose message cannot be resolved statically, so "+
			"behavior 38 is unchecked for it", where)
	}
	for _, where := range empty {
		t.Errorf("%s constructs an error with an empty message, which would reach §4.8's "+
			"unreachable fallback", where)
	}
	for _, where := range fallback {
		t.Errorf("%s spells one of §4.8's fallback strings, which behavior 38 says no invocation "+
			"can reach", where)
	}
}

// packageDirs lists every directory under root that holds Go source.
func packageDirs(t *testing.T, root string) []string {
	t.Helper()

	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	}

	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			seen[filepath.Dir(path)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}

	out := make([]string, 0, len(seen))
	for dir := range seen {
		out = append(out, dir)
	}
	sort.Strings(out)
	return out
}

// parsePackage parses every non-test Go file in one directory. Files that no
// build tag selects are parsed too — an unreachable empty message is still a
// finding.
func parsePackage(t *testing.T, fset *token.FileSet, dir string) []*ast.File {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	var out []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", filepath.Join(dir, name), err)
		}
		out = append(out, file)
	}
	return out
}

// stringConstants maps a package's constant and variable names to the
// expression bound to them, which is how the message catalogues in
// `internal/schema/models/**` spell their text — sometimes as a literal,
// sometimes as a concatenation of two more constants.
func stringConstants(files []*ast.File) map[string]ast.Expr {
	out := map[string]ast.Expr{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || (gen.Tok != token.CONST && gen.Tok != token.VAR) {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range value.Names {
					if i < len(value.Values) {
						out[name.Name] = value.Values[i]
					}
				}
			}
		}
	}
	return out
}

// isUserError recognises the composite-literal type `schemaerr.UserError`,
// which is the port's `RenderCVUserError` (`schemaerr/error.go:59-63`) and the
// only error whose message reaches §4.8's fallback.
func isUserError(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "UserError" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "schemaerr"
}

// fieldValue returns the expression a keyed composite literal binds to one
// field, or nil when the literal omits it.
func fieldValue(lit *ast.CompositeLit, name string) ast.Expr {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := kv.Key.(*ast.Ident); ok && key.Name == name {
			return kv.Value
		}
	}
	return nil
}

// position renders a node's location relative to the repository root.
func position(fset *token.FileSet, root string, pos token.Pos) string {
	return strings.TrimPrefix(fset.Position(pos).String(), root+string(filepath.Separator))
}

// isErrorConstructor recognises `errors.New` and `fmt.Errorf`.
func isErrorConstructor(fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return (pkg.Name == "errors" && sel.Sel.Name == "New") ||
		(pkg.Name == "fmt" && sel.Sel.Name == "Errorf")
}

// resolveString evaluates a message expression as far as a single package's
// source allows: a string literal, a concatenation of literals, or a name the
// package binds to one.
// The depth limit is what stands in for a cycle check; a constant chain in this
// port is two links at most.
func resolveString(expr ast.Expr, constants map[string]ast.Expr, depth int) (string, bool) {
	if depth > 8 {
		return "", false
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(e.Value)
		return s, err == nil
	case *ast.Ident:
		bound, ok := constants[e.Name]
		if !ok {
			return "", false
		}
		return resolveString(bound, constants, depth+1)
	case *ast.CallExpr:
		// `fmt.Sprintf(format, ...)` is resolvable for this test's purpose:
		// interpolation can only add bytes, so a non-empty format guarantees a
		// non-empty message.
		sel, ok := e.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Sprintf" || len(e.Args) == 0 {
			return "", false
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "fmt" {
			return "", false
		}
		return resolveString(e.Args[0], constants, depth+1)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		left, okL := resolveString(e.X, constants, depth+1)
		right, okR := resolveString(e.Y, constants, depth+1)
		if !okL || !okR {
			return "", false
		}
		return left + right, true
	default:
		return "", false
	}
}

// TestFallbackStringsAbsentFromSource is the coarse half of the same criterion:
// neither of §4.8's strings may appear anywhere in the port's source, whatever
// shape it is written in.
func TestFallbackStringsAbsentFromSource(t *testing.T) {
	root := repoRoot(t)

	for _, srcRoot := range sourceRoots {
		dir := filepath.Join(root, srcRoot)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			// This file names the strings in order to forbid them.
			if filepath.Dir(path) == filepath.Join(root, "internal", "clidiff") {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, s := range fallbackStrings {
				if strings.Contains(string(raw), s) {
					t.Errorf("%s contains %q, which behavior 38 says is unreachable upstream",
						strings.TrimPrefix(path, root+string(filepath.Separator)), s)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
}

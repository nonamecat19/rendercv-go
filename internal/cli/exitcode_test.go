package cli

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestSuccessPathExitCodes pins what `execute` returns when a command body
// returns.
//
// **A fresh-context audit measured `render cv.yaml -nopdf -nopng` and
// `render cv.yaml -typ out.typ` exiting 70 while writing every artifact and
// printing the success panel.** 70 is the initial value of `execute`'s `code`,
// so any path that reached cobra's error return — or failed to run the body at
// all — reported an internal failure on a run that had succeeded. That is worse
// than a wrong byte: a caller cannot tell a finished render from a broken one.
//
// The defect is gone, and nothing held it. These cases hold it. They stub the
// body deliberately: the value under test is the plumbing between the parser
// and the exit code, not what `Render` does.
func TestSuccessPathExitCodes(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "plain render", args: []string{"render", "cv.yaml"}},
		{name: "quiet", args: []string{"render", "cv.yaml", "--quiet"}},
		{name: "quiet short", args: []string{"render", "cv.yaml", "-q"}},
		{name: "two negatives", args: []string{"render", "cv.yaml", "-nopdf", "-nopng"}},
		{
			name: "every negative",
			args: []string{"render", "cv.yaml", "-notyp", "-nopdf", "-nopng", "-nomd", "-nohtml"},
		},
		{name: "one path flag", args: []string{"render", "cv.yaml", "-typ", "out.typ"}},
		{
			name: "path flags and negatives",
			args: []string{"render", "cv.yaml", "-typ", "out.typ", "-nopdf", "-nopng"},
		},
		{name: "an override", args: []string{"render", "cv.yaml", "--cv.phone", "123"}},
		{name: "an overlay", args: []string{"render", "cv.yaml", "-d", "design.yaml"}},
		{name: "new", args: []string{"new", "John Doe"}},
		{name: "new with a theme", args: []string{"new", "John Doe", "--theme", "moderncv"}},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := execute(row.args, &stdout, &stderr, runners{
				render: func(RenderOptions, io.Writer, io.Writer) int { return 0 },
				newCV:  func(NewOptions, io.Writer, io.Writer) int { return 0 },
			})
			if code != 0 {
				t.Errorf("exit code = %d, want 0; stderr = %q", code, stderr.String())
			}
		})
	}
}

// TestExitCodeComesFromTheBody is the other half: a body that fails must have
// its code returned rather than replaced. Without this, the case above would
// pass on an `execute` that returned 0 unconditionally.
func TestExitCodeComesFromTheBody(t *testing.T) {
	for _, want := range []int{0, 1, 2} {
		var stdout, stderr bytes.Buffer
		code := execute([]string{"render", "cv.yaml"}, &stdout, &stderr, runners{
			render: func(RenderOptions, io.Writer, io.Writer) int { return want },
			newCV:  func(NewOptions, io.Writer, io.Writer) int { return 0 },
		})
		if code != want {
			t.Errorf("exit code = %d, want %d", code, want)
		}
	}
}

// TestNoArgumentsPrintsTheRootHelp is spec 012 help.md acceptance criterion 3,
// and it exists because a fresh-context verifier found the criterion met by the
// code and gated by nothing.
//
// **`HelpPage("")` being right is not the same as the CLI reaching it.** The
// help tests call the renderer directly; this one goes through `execute`, which
// is where the bare invocation used to exit 70 in silence. It is the fifth time
// this port has shipped a correct function no input could reach, so the edge
// gets its own case rather than an inference.
func TestNoArgumentsPrintsTheRootHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := execute(nil, &stdout, &stderr, runners{
		render: func(RenderOptions, io.Writer, io.Writer) int {
			t.Error("render ran on a bare invocation")
			return 70
		},
		newCV: func(NewOptions, io.Writer, io.Writer) int {
			t.Error("new ran on a bare invocation")
			return 70
		},
	})

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty — the help goes to stdout", stderr.String())
	}
	if got, want := stdout.String(), HelpPage(""); got != want {
		t.Errorf("stdout is not the root help page (%d bytes vs %d)", len(got), len(want))
	}
}

// TestHelpFlagsReachTheRenderer pins the other half: `--help` and `-h`, on the
// root and on a subcommand, print that command's page and exit 0 without
// running the command.
func TestHelpFlagsReachTheRenderer(t *testing.T) {
	cases := []struct {
		args []string
		page string
	}{
		{args: []string{"--help"}, page: ""},
		{args: []string{"-h"}, page: ""},
		{args: []string{"render", "--help"}, page: "render"},
		{args: []string{"render", "-h"}, page: "render"},
		{args: []string{"new", "--help"}, page: "new"},
		{args: []string{"new", "-h"}, page: "new"},
	}

	for _, row := range cases {
		t.Run(strings.Join(row.args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := execute(row.args, &stdout, &stderr, runners{
				render: func(RenderOptions, io.Writer, io.Writer) int {
					t.Error("render ran on a help request")
					return 70
				},
				newCV: func(NewOptions, io.Writer, io.Writer) int {
					t.Error("new ran on a help request")
					return 70
				},
			})
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
			if got, want := stdout.String(), HelpPage(row.page); got != want {
				t.Errorf("stdout is not the %q page (%d bytes vs %d)", row.page, len(got), len(want))
			}
		})
	}
}

// TestExitCodeInventory is spec 013 §6.5 rule 5 and behavior 42: upstream can
// exit 0, 1 or 2 and **there is no other code**. 0 is success, `--version` and
// help; 1 is every `RenderCVUserError`, every validation failure and every
// unhandled exception; 2 is every click usage error.
//
// **The port carried a fourth, `70`** — `execute`'s initial `code` and its
// return for an unclassified cobra error. Nothing upstream can produce it, so a
// caller reading the port's exit status could see a value the contract does not
// define.
//
// The set is derived from the source, not enumerated by hand: the walk starts
// at `Execute` and follows every function whose result reaches an exit-code
// position, so a new command body or a new sentinel is covered the day it is
// written. An expression the walk cannot resolve fails the test rather than
// being skipped — a silent skip is how a fourth code would come back.
func TestExitCodeInventory(t *testing.T) {
	sites := exitCodeSites(t)

	codes := make([]int, 0, len(sites))
	for code := range sites {
		codes = append(codes, code)
	}
	sort.Ints(codes)

	for _, code := range codes {
		if code != 0 && code != 1 && code != 2 {
			t.Errorf("exit code %d is producible at %s; §6.5 allows only 0, 1 and 2",
				code, strings.Join(sites[code], ", "))
		}
	}
	for _, want := range []int{0, 1, 2} {
		if len(sites[want]) == 0 {
			t.Errorf("exit code %d is not producible anywhere; the walk found %v", want, codes)
		}
	}
}

// TestUnclassifiedFailureExitsOne is the same rule from the outside. A pflag
// error that is neither a missing value nor an unknown option — here a
// non-boolean value for a boolean flag — is the one vector that reaches
// `execute`'s final return without a `usageError`. Upstream has no such branch:
// the equivalent is an unhandled exception, which typer's excepthook exits 1
// for (behavior 40).
func TestUnclassifiedFailureExitsOne(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := execute([]string{"render", "cv.yaml", "--watch=maybe"}, &stdout, &stderr, runners{
		render: func(RenderOptions, io.Writer, io.Writer) int { return 0 },
		newCV:  func(NewOptions, io.Writer, io.Writer) int { return 0 },
	})
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

// exitCodeSites walks the package's non-test source and returns every integer
// an exit-code path can yield, mapped to the positions that yield it.
//
// The walk is a reachability closure, not a grep: `columnWidths` also returns
// `int` and is not an exit code. A function enters the set only by having its
// result returned from a function already in it — or by being injected into one
// through the `runners` struct, which is how `Execute` reaches the three
// command bodies.
func exitCodeSites(t *testing.T) map[int][]string {
	t.Helper()

	fset := token.NewFileSet()
	files := parsePackageSource(t, fset)

	functions := map[string]*ast.FuncDecl{}
	constants := map[string]int{}
	// injected maps a `runners` field to the function `Execute` puts in it.
	injected := map[string]string{}

	for _, file := range files {
		for _, decl := range file.Decls {
			switch node := decl.(type) {
			case *ast.FuncDecl:
				if node.Recv == nil {
					functions[node.Name.Name] = node
				}
			case *ast.GenDecl:
				if node.Tok != token.CONST {
					continue
				}
				for _, spec := range node.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range value.Names {
						if i >= len(value.Values) {
							continue
						}
						if number, ok := intLiteral(value.Values[i]); ok {
							constants[name.Name] = number
						}
					}
				}
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if name, ok := literal.Type.(*ast.Ident); !ok || name.Name != "runners" {
				return true
			}
			for _, element := range literal.Elts {
				pair, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, keyOK := pair.Key.(*ast.Ident)
				value, valueOK := pair.Value.(*ast.Ident)
				if keyOK && valueOK {
					injected[key.Name] = value.Name
				}
			}
			return true
		})
	}

	sites := map[int][]string{}
	seen := map[string]bool{}
	queue := []string{"Execute"}

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if seen[name] {
			continue
		}
		seen[name] = true

		declaration, ok := functions[name]
		if !ok {
			t.Fatalf("exit-code function %q is not declared in this package", name)
		}

		for _, expression := range exitExpressions(declaration) {
			switch node := expression.(type) {
			case *ast.CallExpr:
				callee, ok := calleeName(node, injected)
				if !ok {
					t.Fatalf("%s: unresolved call in an exit-code position; the inventory "+
						"cannot be trusted until the walk understands it",
						fset.Position(node.Pos()))
				}
				queue = append(queue, callee)
			case *ast.Ident:
				if value, ok := constants[node.Name]; ok {
					sites[value] = append(sites[value], position(fset, node, name))
					continue
				}
				if _, ok := functions[node.Name]; ok {
					queue = append(queue, node.Name)
					continue
				}
				t.Fatalf("%s: unresolved identifier %q in an exit-code position",
					fset.Position(node.Pos()), node.Name)
			default:
				value, ok := intLiteral(expression)
				if !ok {
					t.Fatalf("%s: unresolved expression in an exit-code position",
						fset.Position(expression.Pos()))
				}
				sites[value] = append(sites[value], position(fset, expression, name))
			}
		}
	}
	return sites
}

// exitExpressions are the expressions whose value becomes the function's
// result: its own `return`s, plus every assignment to a local the function
// returns bare. The second clause is what catches `execute`, which accumulates
// into `code` from inside cobra's callbacks and returns it at the end.
func exitExpressions(declaration *ast.FuncDecl) []ast.Expr {
	accumulators := map[string]bool{}
	var expressions []ast.Expr

	// Returns inside a closure belong to the closure, not to this function.
	inspectOwnBody(declaration, func(node ast.Node) {
		statement, ok := node.(*ast.ReturnStmt)
		if !ok || len(statement.Results) != 1 {
			return
		}
		if name, ok := statement.Results[0].(*ast.Ident); ok && isLocal(declaration, name.Name) {
			accumulators[name.Name] = true
			return
		}
		expressions = append(expressions, statement.Results[0])
	})

	ast.Inspect(declaration, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
			return true
		}
		if name, ok := assignment.Lhs[0].(*ast.Ident); ok && accumulators[name.Name] {
			expressions = append(expressions, assignment.Rhs[0])
		}
		return true
	})

	return expressions
}

// inspectOwnBody walks a function body without descending into function
// literals.
func inspectOwnBody(declaration *ast.FuncDecl, visit func(ast.Node)) {
	ast.Inspect(declaration.Body, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		if node != nil {
			visit(node)
		}
		return true
	})
}

// isLocal reports whether the name is declared inside the function — a
// short variable declaration or a `var`. A package constant returned bare is
// not an accumulator.
func isLocal(declaration *ast.FuncDecl, name string) bool {
	found := false
	ast.Inspect(declaration, func(node ast.Node) bool {
		switch statement := node.(type) {
		case *ast.AssignStmt:
			if statement.Tok != token.DEFINE {
				return true
			}
			for _, target := range statement.Lhs {
				if ident, ok := target.(*ast.Ident); ok && ident.Name == name {
					found = true
				}
			}
		case *ast.ValueSpec:
			for _, ident := range statement.Names {
				if ident.Name == name {
					found = true
				}
			}
		}
		return true
	})
	return found
}

// calleeName is the function an exit-code call resolves to: a plain call, or a
// call through one of the injected `runners` fields.
func calleeName(call *ast.CallExpr, injected map[string]string) (string, bool) {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name, true
	case *ast.SelectorExpr:
		if name, ok := injected[fun.Sel.Name]; ok {
			return name, true
		}
	}
	return "", false
}

// parsePackageSource parses every non-test `.go` file in the working directory,
// which under `go test` is the package's own directory.
func parsePackageSource(t *testing.T, fset *token.FileSet) []*ast.File {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package directory: %v", err)
	}
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		t.Fatal("no non-test source found in the working directory")
	}
	return files
}

func intLiteral(expression ast.Expr) (int, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.INT {
		return 0, false
	}
	value, err := strconv.Atoi(literal.Value)
	if err != nil {
		return 0, false
	}
	return value, true
}

func position(fset *token.FileSet, node ast.Node, function string) string {
	return fmt.Sprintf("%s (%s)", fset.Position(node.Pos()), function)
}

// TestVersionExitsZero pins `cli_version`, the first parity case the port ever
// passed, at the level of the exit code its golden records.
func TestVersionExitsZero(t *testing.T) {
	for _, flag := range []string{"--version", "-v"} {
		var stdout, stderr bytes.Buffer
		code := execute([]string{flag}, &stdout, &stderr, runners{
			render: func(RenderOptions, io.Writer, io.Writer) int { return 70 },
			newCV:  func(NewOptions, io.Writer, io.Writer) int { return 70 },
		})
		if code != 0 {
			t.Errorf("%s: exit code = %d, want 0", flag, code)
		}
		if stdout.String() != "RenderCV v"+Version+"\n" {
			t.Errorf("%s: stdout = %q", flag, stdout.String())
		}
	}
}

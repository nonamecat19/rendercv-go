package main

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"
	"testing"
)

// The other half of spec 013 §6.5 rule 5 — exit codes are 0, 1, 2 and nothing
// else.
//
// `internal/cli/exitcode_test.go`'s `TestExitCodeInventory` proves the rule over
// **package `cli`'s closure**: every integer that can reach an exit-code
// position in a function reachable from `Execute`. It says plainly what it does
// not cover — a code produced outside that closure. This file covers **package
// `main`'s closure**, which is the only code between `Execute`'s return and the
// process's status, and the two together are the whole path.
//
// The split matters because the two halves fail differently. `cli`'s walk fails
// when a fourth *value* appears; this one fails when a second *exit* appears —
// an `os.Exit` somewhere other than `main`'s one line, a `log.Fatal`, a
// `runtime.Goexit`. Neither walk would see the other's defect.

// mainExitCall is the one sanctioned exit in the whole program.
const mainExitCall = "os.Exit(cli.Execute(os.Args[1:], os.Stdout, os.Stderr))"

// TestMainPassesExecuteThrough asserts that `main` is exactly one statement and
// that the statement hands `Execute`'s return to `os.Exit` unaltered.
//
// **Unaltered is the whole assertion.** `os.Exit(cli.Execute(…) + 1)`,
// `os.Exit(1)` after a discarded call, or a clamp to some "safe" range would
// each satisfy a test that only checked an `os.Exit` was present, and each would
// break Axis 2 without touching a single byte of output.
func TestMainPassesExecuteThrough(t *testing.T) {
	fileSet := token.NewFileSet()
	files := parsePackage(t, fileSet)

	declaration, ok := files.function("main")
	if !ok {
		t.Fatal("package main declares no main function")
	}
	if len(declaration.Body.List) != 1 {
		t.Fatalf("main has %d statements, want 1: every one of them is an exit path",
			len(declaration.Body.List))
	}

	statement, ok := declaration.Body.List[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("main's only statement is %T, want the os.Exit call", declaration.Body.List[0])
	}
	if got := render(t, fileSet, statement.X); got != mainExitCall {
		t.Errorf("main's exit is\n\t%s\nwant\n\t%s", got, mainExitCall)
	}
}

// TestNoExitOutsideMain is the reachability half: nothing in this package may
// end the process except that one line.
//
// Each banned form ends a Go program with a status `cli.Execute` never chose.
// `log.Fatal` and its family are exit 1 with a timestamped line on stderr, which
// is the right code for the wrong reason and the wrong bytes. `runtime.Goexit`
// from the main goroutine is exit 2 with `no goroutines (main called
// runtime.Goexit) - deadlock!`. `os.Exit` anywhere else is whatever integer it
// was handed.
func TestNoExitOutsideMain(t *testing.T) {
	banned := []string{
		"os.Exit",
		"log.Fatal", "log.Fatalf", "log.Fatalln",
		"log.Panic", "log.Panicf", "log.Panicln",
		"runtime.Goexit",
		"syscall.Exit",
	}

	fileSet := token.NewFileSet()
	files := parsePackage(t, fileSet)
	sanctioned := files.sanctionedExit(t)

	for _, file := range files.parsed {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if call == sanctioned {
				// Its own argument still gets walked, which is how a nested
				// `os.Exit` inside the sanctioned one would be caught.
				return true
			}
			name := render(t, fileSet, call.Fun)
			for _, forbidden := range banned {
				if name == forbidden {
					t.Errorf("%s: %s outside main's single exit statement",
						fileSet.Position(call.Pos()), name)
				}
			}
			// A panic that escapes main exits 2 — inside §6.5's set by
			// accident, but with a goroutine dump on stderr that no upstream
			// invocation produces. The port has no business panicking here.
			if name == "panic" {
				t.Errorf("%s: panic in package main; a panic that escapes exits 2 "+
					"and prints a stack trace upstream never prints",
					fileSet.Position(call.Pos()))
			}
			return true
		})
	}
}

// TestNoDeclarationsBesidesMain closes the last door: an `init` function, or a
// package-level variable whose initialiser runs code, executes before `main` and
// can end the process before `Execute` is ever called. The package has none, and
// this is what keeps it that way.
func TestNoDeclarationsBesidesMain(t *testing.T) {
	fileSet := token.NewFileSet()
	files := parsePackage(t, fileSet)

	for _, file := range files.parsed {
		for _, declaration := range file.Decls {
			switch node := declaration.(type) {
			case *ast.FuncDecl:
				if node.Name.Name != "main" || node.Recv != nil {
					t.Errorf("%s: package main declares %s besides main",
						fileSet.Position(node.Pos()), node.Name.Name)
				}
			case *ast.GenDecl:
				if node.Tok != token.IMPORT {
					t.Errorf("%s: package main declares a %s at package level; "+
						"its initialiser runs before main",
						fileSet.Position(node.Pos()), node.Tok)
				}
			}
		}
	}
}

// sourceFiles are the package's non-test files, parsed.
type sourceFiles struct {
	parsed []*ast.File
	names  []string
}

// parsePackage reads every non-test `.go` file in the working directory, which
// under `go test` is the package's own directory. Sorted by name, so a failure
// reports the same position twice in a row.
func parsePackage(t *testing.T, fileSet *token.FileSet) sourceFiles {
	t.Helper()

	directory, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package directory: %v", err)
	}

	var files sourceFiles
	for _, entry := range directory {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		files.parsed = append(files.parsed, file)
		files.names = append(files.names, name)
	}
	if len(files.parsed) == 0 {
		t.Fatal("no non-test source found in the working directory")
	}
	return files
}

func (s sourceFiles) function(name string) (*ast.FuncDecl, bool) {
	for _, file := range s.parsed {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && function.Name.Name == name {
				return function, true
			}
		}
	}
	return nil, false
}

// sanctionedExit is the `os.Exit` call `main`'s single statement makes — the one
// node TestNoExitOutsideMain is allowed to find.
func (s sourceFiles) sanctionedExit(t *testing.T) *ast.CallExpr {
	t.Helper()

	declaration, ok := s.function("main")
	if !ok {
		t.Fatal("package main declares no main function")
	}
	if len(declaration.Body.List) != 1 {
		return nil
	}
	statement, ok := declaration.Body.List[0].(*ast.ExprStmt)
	if !ok {
		return nil
	}
	call, ok := statement.X.(*ast.CallExpr)
	if !ok {
		return nil
	}
	return call
}

// render prints an expression back as source, so an assertion can be written
// against the text a reader of main.go sees.
func render(t *testing.T, fileSet *token.FileSet, node ast.Expr) string {
	t.Helper()

	var out strings.Builder
	if err := printer.Fprint(&out, fileSet, node); err != nil {
		t.Fatalf("printing %T: %v", node, err)
	}
	return out.String()
}

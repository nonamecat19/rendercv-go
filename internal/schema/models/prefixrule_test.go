package models_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// valueErrorPrefix is what pydantic puts in front of a message when a plain
// Python `ValueError` escapes a validator. Spec 004 §3.2 step 1 strips every
// occurrence of it.
const valueErrorPrefix = "Value error, "

// Spec 004 §3.9c behavior 33j and §3.2 behavior 4b: **the code does not tell you
// whether a message carries the prefix.** All three rows below are `value_error`
// upstream and they do not agree, so a rewriter that infers "code `value_error`
// ⇒ strip" would eat the front of the email and phone messages.
//
// The messages are upstream's raw `e.errors()[0]['msg']`, measured on the
// vendored Python and written as literals. What the table proves is a property
// of the *rule*, not of the port: one code, both answers.
func TestThePrefixRuleIsNotKeyedOnTheCode(t *testing.T) {
	rows := []struct {
		name       string
		code       schemaerr.Code
		message    string
		wantPrefix bool
	}{
		{
			name:       "a bad email — pydantic's own wrapper, no prefix",
			code:       "value_error",
			message:    "value is not a valid email address: An email address must have an @-sign.",
			wantPrefix: false,
		},
		{
			name:       "a bad phone — likewise",
			code:       "value_error",
			message:    "value is not a valid phone number",
			wantPrefix: false,
		},
		{
			// The one `value_error` in the tree whose message *is* wrapped, and it
			// is wrapped because `Date.fromisoformat`'s exception escapes uncaught
			// (entry_with_date.py:26-29) — not because of its code.
			name:       "an out-of-range arbitrary date — a bare ValueError escaped",
			code:       "value_error",
			message:    "Value error, month must be in 1..12",
			wantPrefix: true,
		},
	}

	sawBoth := map[bool]bool{}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			if got := strings.HasPrefix(row.message, valueErrorPrefix); got != row.wantPrefix {
				t.Errorf("HasPrefix(%q) = %v, want %v", row.message, got, row.wantPrefix)
			}
		})
		if row.code == "value_error" {
			sawBoth[row.wantPrefix] = true
		}
	}

	if !sawBoth[true] || !sawBoth[false] {
		t.Fatal("the table no longer contains both answers under one code, so it" +
			" no longer refutes a code-keyed strip")
	}
}

// The port stores its messages already unprefixed, so spec 004 §3.2 step 1 is
// inert against the port's own records and is exercised only by synthetic input
// (spec §6 rule 6).
//
// That is a claim about every message in the tree, and stating it in prose is
// not the same as being able to fail on it. This walks every Go file under
// `internal/schema/models/**` and fails on a string literal containing the
// prefix, which is what makes the inertness claim falsifiable rather than
// merely asserted. Test files are skipped: the table above holds the prefix on
// purpose.
func TestNoModelMessageCarriesTheValueErrorPrefix(t *testing.T) {
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
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		walked++

		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				return true
			}
			if strings.Contains(value, valueErrorPrefix) {
				relative, _ := filepath.Rel(root, path)
				t.Errorf("%s:%d: message carries %q — the port stores messages"+
					" already stripped (spec 004 §3.2 behavior 4a)",
					relative, fset.Position(literal.Pos()).Line, valueErrorPrefix)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}

	// A walk that silently found nothing would pass for the wrong reason.
	if walked < 10 {
		t.Fatalf("walked %d files, want the whole models tree", walked)
	}
}

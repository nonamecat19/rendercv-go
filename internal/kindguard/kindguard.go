// Package kindguard enforces the shape every branch over a `yamldoc.Kind`
// must take.
//
// It has no upstream counterpart. It exists because iteration 15's central
// design bet — a new `KindTagged` that typed fields reject *by not naming it*
// — only pays off where the omission is visible. `golangci-lint`'s `exhaustive`
// check sees a `switch`; it does not see `k == A || k == B`, and it treats a
// `switch` carrying a `default` as already exhaustive
// (`.golangci.yaml`: `default-signifies-exhaustive: true`). Adding `KindTagged`
// therefore flagged nothing at `binder.go`'s `isNonScalar`, a tagged scalar
// routed to the scalar arm, and a wrong CV rendered at exit 0 (STATE.md
// blocker B1).
//
// Two rules close that gap, and both are decidable from the syntax tree:
//
//  1. A `switch` that branches on a Kind names every Kind and carries no
//     `default`. Adding a Kind then breaks the build at every such switch.
//  2. No single decision compares a Kind against more than one constant
//     outside such a switch — no `k == A || k == B`, no tagless
//     `switch { case k == A: ...; case k == B: }`, no
//     `if k == A { } else if k == B { }`. A subset enumerated by hand is
//     exactly the shape that silently gained a wrong answer, and an
//     `if`/`else if` chain is a `switch` written in the one spelling the
//     `exhaustive` check cannot see. A composite literal naming two or more
//     Kinds — `map[yamldoc.Kind]bool{A: true, B: true}`, `[]yamldoc.Kind{A, B}`
//     — is the same subset with the decision moved to a lookup, where a new
//     Kind answers `false` rather than failing to compile.
//
// A *single* comparison (`k != yamldoc.KindMapping`) stays legal: it is total
// by construction — it splits every Kind, present and future, into the named
// one and the rest — so a new Kind cannot land somewhere unconsidered.
package kindguard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Rule names which of the two constraints a Violation breaks.
type Rule string

// The rules kindguard enforces. See the package comment for why each exists.
const (
	// RuleSwitchNotExhaustive is a Kind switch that leaves a Kind unnamed.
	RuleSwitchNotExhaustive Rule = "switch-not-exhaustive"
	// RuleSwitchHasDefault is a Kind switch with a `default` clause, which
	// absorbs a new Kind silently.
	RuleSwitchHasDefault Rule = "switch-has-default"
	// RuleMultiConstantPredicate is one decision comparing a Kind against
	// more than one constant outside a Kind switch.
	RuleMultiConstantPredicate Rule = "multi-constant-predicate"
	// RuleKindSetLiteral is a composite literal naming more than one Kind,
	// which enumerates the same subset by hand as a lookup table.
	RuleKindSetLiteral Rule = "kind-set-literal"
)

// Violation is one place that breaks a rule.
type Violation struct {
	File   string
	Line   int
	Rule   Rule
	Detail string
}

// String renders a violation as `file:line: rule: detail`.
func (v Violation) String() string {
	return fmt.Sprintf("%s:%d: %s: %s", v.File, v.Line, v.Rule, v.Detail)
}

// yamldocPkgDir is where the Kind constants are declared, relative to the
// module root.
const yamldocPkgDir = "internal/schema/yamldoc"

// skippedDirs are trees kindguard never reads: vendored upstream (read-only
// by contract), fixtures, and build output.
var skippedDirs = map[string]bool{
	"third_party": true,
	"testdata":    true,
	".git":        true,
	"bin":         true,
}

// Kinds returns the names of the `yamldoc.Kind` constants declared under root.
//
// The set is read from the source rather than listed here on purpose: a new
// Kind must tighten every existing switch the moment it is declared, with no
// second place to remember to update.
func Kinds(root string) ([]string, error) {
	dir := filepath.Join(root, yamldocPkgDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, parseErr)
		}
		if file.Name.Name != "yamldoc" {
			continue
		}
		for _, decl := range file.Decls {
			genDecl, isGen := decl.(*ast.GenDecl)
			if !isGen || genDecl.Tok != token.CONST {
				continue
			}
			declaredType := ""
			for _, spec := range genDecl.Specs {
				valueSpec, isValue := spec.(*ast.ValueSpec)
				if !isValue {
					continue
				}
				if ident, isIdent := valueSpec.Type.(*ast.Ident); isIdent {
					declaredType = ident.Name
				}
				if declaredType != "Kind" {
					continue
				}
				for _, name := range valueSpec.Names {
					names = append(names, name.Name)
				}
			}
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no Kind constants found in %s", dir)
	}
	sort.Strings(names)
	return names, nil
}

// CheckTree reports every violation in the Go sources under root, tests
// included. A test that branches on a Kind encodes a belief about the Kind set
// exactly as a non-test file does.
func CheckTree(root string) ([]Violation, error) {
	kinds, err := Kinds(root)
	if err != nil {
		return nil, err
	}

	var violations []Violation
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if skippedDirs[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("reading %s: %w", path, readErr)
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			relative = path
		}
		found, checkErr := CheckSource(relative, source, kinds)
		if checkErr != nil {
			return checkErr
		}
		violations = append(violations, found...)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].File != violations[j].File {
			return violations[i].File < violations[j].File
		}
		return violations[i].Line < violations[j].Line
	})
	return violations, nil
}

// CheckSource reports every violation in one file's source. name is used only
// for reporting, and kinds is the full set of Kind constant names.
func CheckSource(name string, source []byte, kinds []string) ([]Violation, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, source, 0)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", name, err)
	}
	checker := &checker{
		fset:    fset,
		name:    name,
		kinds:   kinds,
		chained: map[*ast.IfStmt]bool{},
		// Inside package yamldoc the constants are unqualified; everywhere
		// else they must be reached through the package name, which no file
		// aliases.
		unqualified: strings.HasPrefix(file.Name.Name, "yamldoc"),
	}
	checker.check(file)
	return checker.violations, nil
}

type checker struct {
	fset        *token.FileSet
	name        string
	kinds       []string
	unqualified bool
	violations  []Violation
	// suppress holds the extents of already-reported predicate groups, so an
	// inner `||` is not reported again under the outer one that contains it.
	suppress []span
	// chained holds the `else if` members of chains already walked from their
	// head, so a chain is considered once and not again from each link.
	chained map[*ast.IfStmt]bool
}

type span struct{ from, to token.Pos }

func (c *checker) check(file *ast.File) {
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.SwitchStmt:
			c.checkSwitch(typed)
		case *ast.IfStmt:
			c.checkIfChain(typed)
		case *ast.CompositeLit:
			c.checkCompositeLit(typed)
		case *ast.BinaryExpr:
			if typed.Op == token.LAND || typed.Op == token.LOR {
				c.checkPredicateGroup(typed, c.kindComparisons(typed), "boolean expression")
			}
		}
		return true
	})
}

// checkSwitch applies rule 1 to a switch that branches on a Kind, and rule 2
// to a tagless switch whose cases compare Kinds — `switch { case k == A: }` is
// the same hand-enumerated subset as `k == A || k == B`.
func (c *checker) checkSwitch(stmt *ast.SwitchStmt) {
	named := map[string]bool{}
	hasDefault := false
	var compared []*ast.BinaryExpr
	for _, clause := range stmt.Body.List {
		caseClause, isCase := clause.(*ast.CaseClause)
		if !isCase {
			continue
		}
		if caseClause.List == nil {
			hasDefault = true
			continue
		}
		for _, expr := range caseClause.List {
			if kind, isKind := c.kindConstant(expr); isKind {
				named[kind] = true
				continue
			}
			compared = append(compared, c.kindComparisons(expr)...)
		}
	}

	if len(named) > 0 {
		c.checkSwitchArms(stmt, named, hasDefault)
		return
	}
	if stmt.Tag == nil {
		c.checkPredicateGroup(stmt, compared, "tagless switch")
	}
}

func (c *checker) checkSwitchArms(stmt *ast.SwitchStmt, named map[string]bool, hasDefault bool) {
	if hasDefault {
		c.report(stmt.Pos(), RuleSwitchHasDefault,
			"a switch over yamldoc.Kind must not have a default clause; "+
				"spell the remaining kinds out so a new one breaks the build here")
	}
	var missing []string
	for _, kind := range c.kinds {
		if !named[kind] {
			missing = append(missing, kind)
		}
	}
	if len(missing) > 0 {
		c.report(stmt.Pos(), RuleSwitchNotExhaustive,
			"a switch over yamldoc.Kind must name every kind; missing "+strings.Join(missing, ", "))
	}
}

// checkCompositeLit applies rule 2 to a literal that names Kinds directly —
// `map[yamldoc.Kind]bool{A: true, B: true}`, `[]yamldoc.Kind{A, B}` — where
// the decision has moved from a comparison into a lookup, and a ninth Kind
// reads back the zero value instead of breaking the build.
//
// Only the literal's own keys and values count, never a nested literal's: a
// test table of nodes that each carry one Kind is a corpus, not a decision.
func (c *checker) checkCompositeLit(lit *ast.CompositeLit) {
	var named []string
	for _, element := range lit.Elts {
		if pair, isPair := element.(*ast.KeyValueExpr); isPair {
			if kind, isKind := c.kindConstant(pair.Key); isKind {
				named = append(named, kind)
				continue
			}
			if kind, isKind := c.kindConstant(pair.Value); isKind {
				named = append(named, kind)
			}
			continue
		}
		if kind, isKind := c.kindConstant(element); isKind {
			named = append(named, kind)
		}
	}
	if len(named) < 2 {
		return
	}
	c.report(lit.Pos(), RuleKindSetLiteral,
		"this literal enumerates yamldoc.Kind by hand ("+strings.Join(named, ", ")+"); "+
			"use an exhaustive switch so a new kind cannot read back the zero value")
}

// checkIfChain applies rule 2 to an `if`/`else if` chain, which is one
// decision spread over several conditions: `if k == A { } else if k == B { }`
// is the same hand-enumerated subset as `k == A || k == B`, written in the
// shape neither `exhaustive` nor rule 1 looks at.
//
// Only the head of a chain is walked; a lone `if`, with or without an `else`
// block, holds a single condition and stays legal for the same reason a single
// comparison does.
func (c *checker) checkIfChain(stmt *ast.IfStmt) {
	if c.chained[stmt] {
		return
	}
	var conditions []span
	var compared []*ast.BinaryExpr
	for current := stmt; ; {
		conditions = append(conditions, span{current.Cond.Pos(), current.Cond.End()})
		compared = append(compared, c.kindComparisons(current.Cond)...)
		next, isElseIf := current.Else.(*ast.IfStmt)
		if !isElseIf {
			break
		}
		c.chained[next] = true
		current = next
	}
	if len(conditions) < 2 {
		return
	}
	// Only the conditions are suppressed, never the bodies: a predicate inside
	// an arm is its own decision and must still be reported.
	c.checkPredicateGroupIn(conditions, stmt.Pos(), compared, "if/else chain")
}

// checkPredicateGroup applies rule 2 to one decision's worth of comparisons.
func (c *checker) checkPredicateGroup(node ast.Node, compared []*ast.BinaryExpr, shape string) {
	c.checkPredicateGroupIn([]span{{node.Pos(), node.End()}}, node.Pos(), compared, shape)
}

// checkPredicateGroupIn reports at pos and suppresses further reports inside
// covered, which is the extent the decision spans.
func (c *checker) checkPredicateGroupIn(covered []span, pos token.Pos, compared []*ast.BinaryExpr, shape string) {
	if len(compared) < 2 || c.suppressed(pos) {
		return
	}
	c.suppress = append(c.suppress, covered...)

	names := make([]string, 0, len(compared))
	for _, comparison := range compared {
		if kind, isKind := c.kindConstant(comparison.Y); isKind {
			names = append(names, kind)
			continue
		}
		kind, _ := c.kindConstant(comparison.X)
		names = append(names, kind)
	}
	c.report(pos, RuleMultiConstantPredicate,
		"this "+shape+" enumerates yamldoc.Kind by hand ("+strings.Join(names, ", ")+"); "+
			"use an exhaustive switch so a new kind cannot fall to the unwritten arm")
}

func (c *checker) suppressed(pos token.Pos) bool {
	for _, reported := range c.suppress {
		if pos >= reported.from && pos < reported.to {
			return true
		}
	}
	return false
}

// kindComparisons collects every `==`/`!=` against a Kind constant in expr.
func (c *checker) kindComparisons(expr ast.Expr) []*ast.BinaryExpr {
	var found []*ast.BinaryExpr
	ast.Inspect(expr, func(node ast.Node) bool {
		binary, isBinary := node.(*ast.BinaryExpr)
		if !isBinary || (binary.Op != token.EQL && binary.Op != token.NEQ) {
			return true
		}
		if _, isKind := c.kindConstant(binary.X); isKind {
			found = append(found, binary)
			return true
		}
		if _, isKind := c.kindConstant(binary.Y); isKind {
			found = append(found, binary)
		}
		return true
	})
	return found
}

// kindConstant reports whether expr names a Kind constant.
func (c *checker) kindConstant(expr ast.Expr) (string, bool) {
	switch typed := expr.(type) {
	case *ast.SelectorExpr:
		pkg, isIdent := typed.X.(*ast.Ident)
		if isIdent && pkg.Name == "yamldoc" && c.isKindName(typed.Sel.Name) {
			return typed.Sel.Name, true
		}
	case *ast.Ident:
		if c.unqualified && c.isKindName(typed.Name) {
			return typed.Name, true
		}
	}
	return "", false
}

func (c *checker) isKindName(name string) bool {
	for _, kind := range c.kinds {
		if kind == name {
			return true
		}
	}
	return false
}

func (c *checker) report(pos token.Pos, rule Rule, detail string) {
	position := c.fset.Position(pos)
	c.violations = append(c.violations, Violation{
		File:   c.name,
		Line:   position.Line,
		Rule:   rule,
		Detail: detail,
	})
}

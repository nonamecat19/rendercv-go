package luatheme

import (
	"fmt"
	"sort"
	"strings"
)

// TypeError is a document value whose kind does not match what the theme script
// declared for that option.
type TypeError struct {
	Path     string
	Declared string
	Got      string
}

func (e *TypeError) Error() string {
	return fmt.Sprintf("design.%s should be %s, not %s", e.Path, e.Declared, e.Got)
}

// Validate checks a document's `design` block against the options a theme script
// declared (spec 014 §4, criterion 2's remaining half).
//
// **The declared default is the type.** Upstream's `__init__.py` declares a
// pydantic field with an explicit annotation; a Lua declaration has no
// annotation, but it always has a value — and a theme that defaults an option to
// `"1cm"` has said it is a string as clearly as `TypstDimension` would. Using
// the default's kind means a script cannot declare a type it does not
// demonstrate, which is a smaller contract than upstream's and an honest one.
//
// It checks **only options the script itself declared**. Everything the base
// design tree owns is iteration 6's business and is already validated there, so
// this cannot second-guess it.
func Validate(script, document map[string]any) []error {
	if len(script) == 0 || len(document) == 0 {
		return nil
	}
	var errs []error
	walk(script, document, "", &errs)
	return errs
}

func walk(script, document map[string]any, prefix string, errs *[]error) {
	for _, name := range sortedKeys(script) {
		value, present := document[name]
		if !present || value == nil {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}

		declared := script[name]
		nestedScript, scriptIsTable := declared.(map[string]any)
		nestedDocument, documentIsTable := value.(map[string]any)

		if scriptIsTable && documentIsTable {
			walk(nestedScript, nestedDocument, path, errs)
			continue
		}
		if kindOf(declared) != kindOf(value) && !isBoolWordAgainstDeclaredBool(declared, value) {
			*errs = append(*errs, &TypeError{
				Path:     path,
				Declared: kindOf(declared),
				Got:      kindOf(value),
			})
		}
	}
}

// kindOf names a value's shape the way an error message should.
//
// **A document read from YAML carries every scalar as a string** (the reader
// resolves kinds but the design block's projection keeps text), so a number
// declared by a script and written by a document would always mismatch on Go
// types alone. Numbers and strings are therefore one kind here: what this
// catches is a *mapping* written where a scalar belongs and the reverse, which
// is the mistake that produces an unreadable failure further down.
func kindOf(value any) string {
	switch value.(type) {
	case map[string]any:
		return "a group of options"
	case []string, []any:
		return "a list"
	case bool:
		return "true or false"
	default:
		return "a value"
	}
}

// isBoolWordAgainstDeclaredBool is the one exemption `kindOf`'s plain shape
// match cannot express. `show_footer: no` resolves to the raw string `"no"`,
// not a Go `bool` (`ResolveScalar` only recognizes `true`/`false`), but
// pydantic's `str_as_bool` — and this port's own `validBoolNode` at the
// schema-validation layer — accept it as a real boolean, so a document
// writing it against a script-declared `bool` option is not a conflict.
//
// **This has to be gated on `declared` actually being a Go `bool`, not on
// `value`'s text alone.** An earlier version made `kindOf` itself treat any
// bool-word *string* as kind "true or false" on both sides of the comparison,
// which meant a script declaring a **string** default like
// `degree_column = "**DEGREE**"` no longer conflicted with a document writing
// `degree_column: 'On'` — a legal string that happens to spell a bool word —
// and the override was pruned as if it agreed with the script, silently
// discarding it. Found by a fresh-context verifier (iteration 14's sixth
// re-verification).
func isBoolWordAgainstDeclaredBool(declared, value any) bool {
	if _, isBool := declared.(bool); !isBool {
		return false
	}
	text, isString := value.(string)
	return isString && boolWords[strings.ToLower(text)]
}

// boolWords is the set of strings pydantic's `str_as_bool` coerces to a bool,
// matched case-insensitively. Mirrors
// `internal/schema/models/design/validate.go`'s table of the same name —
// duplicated rather than imported because that package already imports this
// one (`luatheme.Validate`), and a shared bool-words table is not worth an
// import cycle.
var boolWords = map[string]bool{
	"0": true, "off": true, "f": true, "false": true, "n": true, "no": true,
	"1": true, "on": true, "t": true, "true": true, "y": true, "yes": true,
}

func sortedKeys(values map[string]any) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

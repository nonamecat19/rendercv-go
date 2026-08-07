package jsonschema

import (
	"fmt"
	"strings"
)

// DefName returns a model's `$defs` key (spec 005 §3.3).
//
// Pydantic uses the bare class name when it is unique across the whole model
// tree, and the fully qualified module path with `.` replaced by `__` when it is
// not. 173 of upstream's 227 entries are qualified, which is why `EducationEntry`
// appears as
// `rendercv__schema__models__cv__entries__education__EducationEntry`.
//
// The port cannot *derive* uniqueness — it has no module graph — so each model
// states which form it needs and the conformance differential checks the result
// against upstream's key set. That is the same trade iteration 3 made for field
// orders: stated data, mechanical check.
//
// module is the Python module path, dots included, or empty for the bare form.
func DefName(class, module string) string {
	if module == "" {
		return class
	}
	return strings.ReplaceAll(module, ".", "__") + "__" + class
}

// DefNameWithSuffix is spec 005 §3.3 behavior 12: distinct models that collide
// even after qualification get `__1`, `__2`, … appended.
//
// **ordinal is 1-based and follows pydantic's emission order, not the alphabet**
// (behavior 13). That distinction is the whole difficulty: the keys themselves
// sort alphabetically in `$defs`, so a suffix assigned alphabetically produces a
// file that looks sorted and pairs the wrong model with the wrong number.
// SuffixedNames is the helper that gets it right by construction.
func DefNameWithSuffix(class, module string, ordinal int) string {
	return fmt.Sprintf("%s__%d", DefName(class, module), ordinal)
}

// SuffixedNames numbers a colliding model once per member of an emission order.
//
// The caller passes the order pydantic walks the union in — for locale that is
// the twenty-two languages, for design the nine themes — and gets back the
// `$defs` keys in the same order, numbered from 1.
//
// Taking the order as an argument rather than deriving it is deliberate. There
// is nothing in the Go tree to derive it *from*: the numbering is a property of
// how pydantic happened to walk the model graph, and the only faithful source is
// the union's own declared member order, which each package already carries and
// already pins against upstream.
//
// Locale is the easy case and is why this lands with iteration 7 rather than 6:
// a flat list of twenty-two, where design's is nine themes × their nested
// models. Same rule, visibly right on the flat one first.
func SuffixedNames(class, module string, emissionOrder []string) []string {
	names := make([]string, 0, len(emissionOrder))
	for i := range emissionOrder {
		names = append(names, DefNameWithSuffix(class, module, i+1))
	}
	return names
}

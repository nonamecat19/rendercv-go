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

// DefNameWithSuffix is spec 005 §3.3 behavior 12: two distinct models that
// collide even after qualification get `__1`, `__2`, … in the order pydantic
// first emitted them.
//
// **Not implemented, deliberately.** The rule is exercised only by the per-theme
// variant classes, which are iteration 6's models — building it now would mean
// building against models that do not exist and testing against nothing. The
// numbering is emission-order-dependent rather than alphabetical (behavior 13),
// so a plausible-looking implementation would be a wrong answer that reads
// right.
//
// It panics rather than returning a guess for the same reason.
//
// TODO(iteration-6): implement the emission-order numbering alongside the theme
// variants that produce the collisions.
func DefNameWithSuffix(class, module string, ordinal int) string {
	panic(fmt.Sprintf(
		"jsonschema: the $defs collision suffix is iteration 6's (spec 005 §7.2);"+
			" asked for %s.%s__%d", module, class, ordinal))
}

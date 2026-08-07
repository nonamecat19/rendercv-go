package design

import "github.com/nonamecat19/rendercv-go/internal/schema/jsonschema"

// SchemaDefs is the 161 `$defs` the design tree owns: `BuiltInDesign`, the nine
// theme models, the named `Literal` unions, and the nested option models with
// the `__1`…`__9` collision suffixes of spec 005 §3.3 behavior 12.
//
// **Empty until Wave D.** It is wired into the differential now, and the absent
// count is already 227−224, so the suite is red until the entries land — which
// is `AGENTS.md` §4's red-before-green for a body of data whose gate is the
// differential itself (tasks §T6).
func SchemaDefs() map[string]*jsonschema.Object {
	return map[string]*jsonschema.Object{}
}

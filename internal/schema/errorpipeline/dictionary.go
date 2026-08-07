// Package errorpipeline turns raw validation failures into the records RenderCV
// shows a user, mirroring schema/pydantic_error_handling.py.
//
// It is named for what it does rather than for the Python library upstream gets
// it from: the port has no pydantic, and a package called `pydanticerrors` would
// be a lie at every call site. It imports nothing from `models`, so the pipeline
// can be exercised on hand-built records with no model involved.
package errorpipeline

// dictionaryRow is one row of `schema/error_dictionary.yaml`. Old is matched by
// **substring containment** against the raw message, not by equality, and the
// first match wins (pydantic_error_handling.py:89-92).
type dictionaryRow struct{ Old, New string }

// dictionary is `error_dictionary.yaml` in file order.
//
// A slice and never a map: Go randomizes map iteration order, and
// first-match-wins over a randomized order is nondeterministic. Order is
// contractual (spec 004 §6 rule 5).
//
// TODO(spec 004 T2): the thirteen rows. Empty until then, which is what keeps
// dictionary_conformance_test.go red.
var dictionary = []dictionaryRow{}

package schemaerr

import (
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// Code identifies a failure kind. It mirrors pydantic's error type
// discriminator, which the error renderer keys on.
type Code string

// ValidationError is one validation failure: where it happened in the schema
// and in the document, what went wrong, and any nested failures.
//
// A record has two states, and confusing them is the easiest way to break
// parity twice over:
//
//   - **Raw** is what everything under `internal/schema/models/**` produces. Its
//     Message is pre-dictionary — the validator's own text, with no substitution
//     and no trailing period — and its SchemaLocation is pre-filter, still
//     carrying any union-branch elements the shape check added.
//   - **Final** is what `errorpipeline.Parse` returns, and the only thing fit to
//     show a user.
//
// Parse is not idempotent: running it on records that are already final applies
// the dictionary a second time and can append a second period. Nothing but Parse
// finalizes a record.
type ValidationError struct {
	Code           Code
	SchemaLocation []string
	YamlLocation   *yamldoc.Span
	YamlSource     YamlSource
	Message        string
	Input          string
	Children       []ValidationError

	// LocationIsFinal marks a record whose SchemaLocation the validator pinned
	// itself, so the pipeline must not re-derive it: the discriminator skip and
	// the location filter are both skipped for that record.
	//
	// It is how upstream's `ctx["loc"]` override reaches the pipeline
	// (design.py:67, spec 004 §3.2 step 3, §3.17 behavior 65). Exactly one
	// producer needs it — the theme-name failure of spec 004 §4.27, which
	// re-pins to `("design", "theme")` — which is why this is a boolean rather
	// than a context map mirroring pydantic's. A map would put a naked `any` on
	// the one type most in need of a real type, for one caller.
	LocationIsFinal bool
}

func (e *ValidationError) Error() string {
	return e.Message
}

// UserError is a plain user-facing failure with no document location, mirroring
// RenderCVUserError.
type UserError struct {
	Message string
}

func (e *UserError) Error() string {
	return e.Message
}

// UserValidationError carries every validation failure of one run, in the order
// they were produced, mirroring RenderCVUserValidationError.
type UserValidationError struct {
	Errors []ValidationError
}

func (e *UserValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "validation error"
	}
	return e.Errors[0].Message
}

// InternalError is a failure that should never reach a user, mirroring
// RenderCVInternalError.
type InternalError struct {
	Message string
}

func (e *InternalError) Error() string {
	return e.Message
}

// Coded is an error that knows the code its record should carry.
//
// It exists for validators whose failures are not all one kind — a URL can fail
// to parse, carry the wrong scheme, or be too long, and the pipeline dispatches
// on the difference. A validator with a single failure kind does not need it.
type Coded interface {
	error
	ErrorCode() Code
}

// RenderInput is step 11 of the error pipeline (pydantic_error_handling.py:122-126):
// the offending value as text.
//
// Three cases, all measured:
//
//   - a mapping or a sequence renders as the literal three dots, never as its
//     contents — a missing field's input is the whole enclosing mapping, so it
//     renders `...` (`expected_errors.yaml:59`, `:65`, `:77`);
//   - a null renders `None`, Python's spelling, whatever the YAML wrote it as
//     (`null`, `~`, or nothing at all);
//   - anything else renders as the user typed it.
//
// It lives here, beside the record, rather than in `errorpipeline`, because the
// port's records carry input as **text** while upstream's carry the value: by
// the time the pipeline sees a record the kind is gone. So producers render at
// construction and the pipeline has nothing left to do for step 11. The rule is
// still in one place, which is what stops the three cases drifting apart across
// call sites.
func RenderInput(node *yamldoc.Node) string {
	if node == nil {
		return "None"
	}
	switch node.Kind {
	case yamldoc.KindMapping, yamldoc.KindSequence:
		return InputEllipsis
	case yamldoc.KindNull:
		return "None"
	case yamldoc.KindBool:
		// **The Input Value column carries the parsed Python object's
		// `str()`, not the YAML token.** `str(True)` is `"True"`, so a
		// plain lowercase `true` — the common spelling, reachable on any
		// design field — showed the wrong case in the table. The function
		// already made this distinction for `null`/`None`; it was only
		// inconsistent for its bool arm. A numeric field's literal form
		// (`0x1f`, `1_000`) has the same gap and is deliberately left open,
		// same as `design.themeNameRepr`'s: it needs a full YAML-number-to-
		// Python-`str()` resolver for a case no plausible CV reaches.
		// Found by a fresh-context verifier (iteration 14's fourteenth
		// re-verification).
		if yamldoc.BoolIsTrue(node.Raw) {
			return "True"
		}
		return "False"
	case yamldoc.KindInt, yamldoc.KindFloat, yamldoc.KindString:
	}
	return node.Raw
}

// InputEllipsis is what a mapping or a sequence renders as
// (pydantic_error_handling.py:126, spec 004 §4.15).
const InputEllipsis = "..."

package schemaerr

import "github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"

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

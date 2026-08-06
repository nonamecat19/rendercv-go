package schemaerr

import "github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"

// Code identifies a failure kind. It mirrors pydantic's error type
// discriminator, which the error renderer keys on.
type Code string

// ValidationError is one validation failure: where it happened in the schema
// and in the document, what went wrong, and any nested failures.
type ValidationError struct {
	Code           Code
	SchemaLocation []string
	YamlLocation   *yamldoc.Span
	YamlSource     YamlSource
	Message        string
	Input          string
	Children       []ValidationError
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

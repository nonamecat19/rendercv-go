package schemaerr

import "github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"

type Code string

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

type UserError struct {
	Message string
}

func (e *UserError) Error() string {
	return e.Message
}

type UserValidationError struct {
	Errors []ValidationError
}

func (e *UserValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "validation error"
	}
	return e.Errors[0].Message
}

type InternalError struct {
	Message string
}

func (e *InternalError) Error() string {
	return e.Message
}

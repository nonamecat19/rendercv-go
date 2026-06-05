package entries

import (
	"fmt"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// Validate binds and validates one entry against the named type, mirroring what
// pydantic does when `validate_section` hands the whole entries list to the
// chosen section model (section.py:219-235).
//
// The name must be one the registry knows. TextEntry is the near-identity case:
// it is a bare `str` upstream with no model (section.py:23-24), so there is
// nothing to bind — but the section model still declares `entries: list[str]`
// (section.py:116), so the element is type-checked. See validateTextEntry.
//
// An unregistered name is a programming error, not user input — the caller
// discriminates before dispatching — so it returns an InternalError rather than a
// validation error, mirroring RenderCVInternalError.
func Validate(
	node *yamldoc.Node,
	name TypeName,
	location []string,
	source schemaerr.YamlSource,
	reference time.Time,
) ([]schemaerr.ValidationError, error) {
	switch name {
	case "OneLineEntry":
		_, errs := ValidateOneLineEntry(node, location, source, reference)
		return errs, nil
	case "NormalEntry":
		_, errs := ValidateNormalEntry(node, location, source, reference)
		return errs, nil
	case "ExperienceEntry":
		_, errs := ValidateExperienceEntry(node, location, source, reference)
		return errs, nil
	case "EducationEntry":
		_, errs := ValidateEducationEntry(node, location, source, reference)
		return errs, nil
	case "PublicationEntry":
		_, errs := ValidatePublicationEntry(node, location, source, reference)
		return errs, nil
	case "BulletEntry":
		_, errs := ValidateBulletEntry(node, location, source, reference)
		return errs, nil
	case "NumberedEntry":
		_, errs := ValidateNumberedEntry(node, location, source, reference)
		return errs, nil
	case "ReversedNumberedEntry":
		_, errs := ValidateReversedNumberedEntry(node, location, source, reference)
		return errs, nil
	case TextEntryName:
		return validateTextEntry(node, location, source), nil
	default:
		return nil, &schemaerr.InternalError{
			Message: fmt.Sprintf("no validator registered for entry type %q", name),
		}
	}
}

// validateTextEntry is a `TextEntry`'s whole validation: the node must be a
// string.
//
// A text entry has no model and no fields (spec 003 §3.14 behavior 27), which is
// why this looks like nothing — but "no model" is not "no check". Upstream's
// section model for text entries declares `entries: list[str]`
// (`section.py:106-118`), so pydantic type-checks every element against `str`
// exactly as it does for a modelled type, and a non-string element reports
// `string_type` at its own index.
//
// Skipping the check made the port render a document upstream refuses, and only
// in one order: a section is typed from its first resolvable entry
// (`section.py:199-210`, spec 002 §3.59), so `[<string>, {institution: ...}]` is
// a `TextEntry` section holding a mapping, while the reverse order is an
// `EducationEntry` section holding a string and was rejected by the education
// binder all along.
//
// KindString and nothing else, mirroring the binder's own `str` test: pydantic
// rejects `int`, `float` and `bool` for a `str` field with strict mode off
// (`binder.go:631-640`), so `- 2020` in a text section is a failure and not a
// coercion.
func validateTextEntry(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if node != nil && node.Kind == yamldoc.KindString {
		return nil
	}
	err := schemaerr.ValidationError{
		Code:           binder.CodeStringType,
		SchemaLocation: append([]string(nil), location...),
		YamlSource:     source,
		Message:        binder.MessageStringType,
		Input:          schemaerr.RenderInput(node),
	}
	if node != nil {
		span := node.Span
		err.YamlLocation = &span
	}
	return []schemaerr.ValidationError{err}
}

// TextEntryName is the ninth entry type's name. It is a literal appended after
// the eight model names (section.py:37-39), not a model's `__name__`, because
// upstream's TextEntry is the bare `str` type.
const TextEntryName TypeName = "TextEntry"

package entries

import (
	"fmt"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// Validate binds and validates one entry against the named type, mirroring what
// pydantic does when `validate_section` hands the whole entries list to the
// chosen section model (section.py:219-235).
//
// The name must be one the registry knows. TextEntry is the identity case: it is
// a bare `str` upstream with no model (section.py:23-24), so a string node is
// always valid and there is nothing to bind.
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
		// A string is always a valid TextEntry (plan §3.2). Its own type check —
		// that the node is a string at all — belongs to the caller, which
		// discriminated it as text in the first place.
		return nil, nil
	default:
		return nil, &schemaerr.InternalError{
			Message: fmt.Sprintf("no validator registered for entry type %q", name),
		}
	}
}

// TextEntryName is the ninth entry type's name. It is a literal appended after
// the eight model names (section.py:37-39), not a model's `__name__`, because
// upstream's TextEntry is the bare `str` type.
const TextEntryName TypeName = "TextEntry"

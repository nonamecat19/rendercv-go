package cv

import (
	"fmt"
	"strconv"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// TextEntry is the ninth entry-type name: it exists as an entry type but not as
// a model (spec §3.57, section.py:37-39).
const TextEntry entries.TypeName = "TextEntry"

// CodeSection marks every section-level failure. Upstream raises them all as
// the same custom pydantic error type (section.py:159, :170, :215, :231, :241).
const CodeSection schemaerr.Code = "rendercv_other_error"

// The five section messages of spec §4.8–§4.12, verbatim.
const (
	messageNotAList     = "Each section should be a list of entries! This is not a list."
	messageNoType       = "The entry does not match any entry type."
	messageNullEntry    = "The entry cannot be None."
	messageNoEntryTypes = "RenderCV couldn't match this section with any entry types." +
		" Please check the entries and make sure they are provided correctly."
	messageEntryProblems = "There are problems with the entries. RenderCV detected the entry" +
		" type of this section to be %s. The problems are shown below."
)

// EntryValidator validates one entry against a decided entry type.
//
// TODO(iteration-3): the eight concrete entry types are iteration 3's (spec
// §7). Until they exist, the registered validator accepts everything, so the
// inference and error-shaping rules of spec §3.53–§3.61 can be tested on their
// own.
type EntryValidator func(
	node *yamldoc.Node,
	entryType entries.TypeName,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError

var entryValidator EntryValidator = func(
	*yamldoc.Node, entries.TypeName, []string, schemaerr.YamlSource,
) []schemaerr.ValidationError {
	return nil
}

// InferEntryType mirrors the per-entry inference of spec §3.58
// (section.py:128-178): a mapping resolves to the first type whose
// characteristic fields intersect its keys, a bare string is a `TextEntry`, and
// null is the null-entry error.
func InferEntryType(node *yamldoc.Node, registry *entries.Registry) (entries.TypeName, error) {
	if node == nil || node.Kind == yamldoc.KindNull {
		return "", &inferenceError{message: messageNullEntry}
	}

	if node.Kind == yamldoc.KindMapping {
		keys := make([]string, 0, len(node.Items))
		for _, item := range node.Items {
			keys = append(keys, item.Key)
		}
		if name, ok := registry.Discriminate(keys); ok {
			return name, nil
		}
		return "", &inferenceError{message: messageNoType}
	}

	if node.Kind == yamldoc.KindString {
		return TextEntry, nil
	}

	// Spec §5.14: a non-mapping, non-string, non-null entry — `[1]`, `[[]]` —
	// reaches upstream's already-validated-object branch and fails with an
	// unhandled key lookup rather than a validation error.
	//
	// TODO(iteration-4): decide whether to reproduce the crash or keep §4.9;
	// this is one of the open items flagged for the iteration-4 gate.
	return "", &inferenceError{message: messageNoType}
}

type inferenceError struct {
	message string
}

func (e *inferenceError) Error() string {
	return e.message
}

// ValidateSection mirrors the section validator (section.py:190-242). It
// returns the decided entry type — empty for an empty section, which infers
// nothing (spec §3.54) — together with the errors it produced.
func ValidateSection(
	node *yamldoc.Node,
	registry *entries.Registry,
	location []string,
	source schemaerr.YamlSource,
) (entries.TypeName, []schemaerr.ValidationError) {
	// Spec §3.53: a section value must be a list.
	if node == nil || node.Kind != yamldoc.KindSequence {
		return "", []schemaerr.ValidationError{sectionError(messageNotAList, node, location, source)}
	}

	// Spec §3.54: an empty list is returned unchanged, with no inference.
	if len(node.Elems) == 0 {
		return "", nil
	}

	// Spec §3.59: the first resolvable entry decides the type; an entry that
	// raises during inference is skipped and the next is tried. This is why a
	// null entry never surfaces §4.10 here (spec §5.6).
	var entryType entries.TypeName
	resolved := false
	for _, elem := range node.Elems {
		name, err := InferEntryType(elem, registry)
		if err != nil {
			continue
		}
		entryType = name
		resolved = true
		break
	}

	// Spec §3.60: no entry resolved.
	if !resolved {
		return "", []schemaerr.ValidationError{
			sectionError(messageNoEntryTypes, node, location, source),
		}
	}

	// Spec §3.61: every entry is validated against the one decided type, and
	// any failure is re-raised as §4.12 carrying the nested failures.
	var children []schemaerr.ValidationError
	for i, elem := range node.Elems {
		children = append(
			children,
			entryValidator(elem, entryType, elemLocation(location, i), source)...,
		)
	}
	if len(children) > 0 {
		problems := sectionError(fmt.Sprintf(messageEntryProblems, entryType), node, location, source)
		problems.Children = children
		return entryType, []schemaerr.ValidationError{problems}
	}

	return entryType, nil
}

func sectionError(
	message string,
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) schemaerr.ValidationError {
	err := schemaerr.ValidationError{
		Code:           CodeSection,
		SchemaLocation: append([]string(nil), location...),
		YamlSource:     source,
		Message:        message,
		Input:          "...",
	}
	if node != nil {
		span := node.Span
		err.YamlLocation = &span
	}
	return err
}

func elemLocation(location []string, index int) []string {
	out := make([]string, 0, len(location)+1)
	out = append(out, location...)
	return append(out, strconv.Itoa(index))
}

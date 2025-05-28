// Package locale is a deliberately minimal slice of upstream's `locale` model.
//
// **It holds only what spec 004 §7.9 needs**: the twenty-two language names and
// the discriminated-union tag check. The catalogs themselves — every month name,
// every abbreviation, the twenty-two YAML files — are iteration 7's.
package locale

import (
	"fmt"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// Languages is the discriminated union's tag set (models/locale/locale.py:38-41),
// in the order pydantic enumerates it: `english` first, then the rest in sorted
// filename order.
//
// `norwegian_bokmål` is non-ASCII, which is worth knowing before it appears in
// a byte comparison.
var Languages = []string{
	"english",
	"arabic",
	"danish",
	"dutch",
	"french",
	"german",
	"hebrew",
	"hindi",
	"hungarian",
	"indonesian",
	"italian",
	"japanese",
	"korean",
	"mandarin_chinese",
	"norwegian_bokmål",
	"norwegian_nynorsk",
	"persian",
	"portuguese",
	"russian",
	"spanish",
	"turkish",
	"vietnamese",
}

// The three codes a `locale` block can fail with before any member is chosen.
// None matches a dictionary row, so the pipeline only appends a period.
const (
	// CodeUnionTag is a tag that is not one of the twenty-two.
	CodeUnionTag schemaerr.Code = "union_tag_invalid"
	// CodeUnionTagNotFound is a block with no `language` key at all.
	CodeUnionTagNotFound schemaerr.Code = "union_tag_not_found"
	// CodeModelAttributesType is a `locale` that is not a mapping.
	CodeModelAttributesType schemaerr.Code = "model_attributes_type"
)

// messageTagNotFound and messageNotAMapping are pydantic's own, measured
// against the vendored Python on `{locale: {}}` and `{locale: null}`.
const (
	messageTagNotFound = "Unable to extract tag using discriminator 'language'"
	messageNotAMapping = "Input should be a valid dictionary or object to" +
		" extract fields from"
)

// Validate is the whole `locale` block: the discriminator, then the member it
// selects (locale.py:43-46).
//
// **The two steps do not both run.** Pydantic resolves the tag first and reports
// nothing else when it fails, so a block with an unknown language never reaches
// its fields — which is also why every failure below reports at `("locale",)`
// rather than at `language`.
//
// The member's own failures report at `("locale", <tag>, <field>)` upstream and
// the pipeline drops the middle element (pydantic_error_handling.py:52-54), so
// passing the block's own location straight through is already what the user
// sees.
func Validate(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if node == nil {
		return nil
	}
	if node.Kind != yamldoc.KindMapping {
		return []schemaerr.ValidationError{
			blockError(node, CodeModelAttributesType, messageNotAMapping, location, source),
		}
	}

	language, present := mappingValue(node, "language")
	if !present {
		return []schemaerr.ValidationError{
			blockError(node, CodeUnionTagNotFound, messageTagNotFound, location, source),
		}
	}

	if errs := ValidateLanguage(language, location, source); len(errs) > 0 {
		return errs
	}
	return ValidateCatalog(node, language.Raw, location, source)
}

// mappingValue reads one key of a mapping, reporting whether it was there.
func mappingValue(node *yamldoc.Node, key string) (*yamldoc.Node, bool) {
	for _, item := range node.Items {
		if item.Key == key {
			return item.Value, true
		}
	}
	return nil, false
}

func blockError(
	node *yamldoc.Node,
	code schemaerr.Code,
	message string,
	location []string,
	source schemaerr.YamlSource,
) schemaerr.ValidationError {
	span := node.Span
	return schemaerr.ValidationError{
		Code:           code,
		SchemaLocation: append([]string(nil), location...),
		YamlLocation:   &span,
		YamlSource:     source,
		Message:        message,
		Input:          schemaerr.RenderInput(node),
	}
}

// ValidateLanguage checks the `language` discriminator of a `locale` block.
//
// The failure reports at the **locale block**, not at `language`: pydantic
// raises it while resolving which union member to use, before any member's
// fields are reached. Measured — `{language: klingon}` gives `("locale",)`.
//
// The locale package raises no custom failures of its own; it has neither a
// field nor a model validator, so every other locale failure is a plain
// pydantic message through the ordinary path, with the discriminator element
// dropped by the pipeline's step 2 (spec 004 §3.17 behavior 67).
func ValidateLanguage(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if node == nil {
		return nil
	}

	// **A null tag is a failure, not an absence.** Measured: `{language: null}`
	// gives `Input tag 'None'`, because pydantic reads the key, finds `None`, and
	// matches it against the tags. Treating it as "unspecified, use the default"
	// is the reading that looks right and accepts a document upstream rejects.
	value := schemaerr.RenderInput(node)
	for _, known := range Languages {
		if value == known {
			return nil
		}
	}

	span := node.Span
	return []schemaerr.ValidationError{{
		Code:           CodeUnionTag,
		SchemaLocation: append([]string(nil), location...),
		YamlLocation:   &span,
		YamlSource:     source,
		Message:        unknownLanguageMessage(value),
		Input:          schemaerr.RenderInput(node),
	}}
}

// unknownLanguageMessage is pydantic's `union_tag_invalid` text, built from the
// tag list so it cannot drift from what it enumerates.
func unknownLanguageMessage(tag string) string {
	quoted := make([]string, 0, len(Languages))
	for _, language := range Languages {
		quoted = append(quoted, "'"+language+"'")
	}
	return fmt.Sprintf(
		"Input tag '%s' found using 'language' does not match any of the"+
			" expected tags: %s", tag, strings.Join(quoted, ", "))
}

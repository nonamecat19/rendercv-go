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

// CodeUnionTag is pydantic's discriminated-union tag failure. No dictionary row
// matches its message, so the pipeline only appends a period.
const CodeUnionTag schemaerr.Code = "union_tag_invalid"

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
	if node == nil || node.Kind == yamldoc.KindNull {
		return nil
	}

	value := node.Raw
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

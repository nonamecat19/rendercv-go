// Package design is a deliberately minimal slice of upstream's `design` models.
//
// **It holds only what spec 004 §7.9 pulls forward**: the built-in theme names
// and the custom-theme-name shape check, because the 25-record differential
// needs a `design.theme` record and cannot reach 25 without one.
//
// Everything else in `design` — the theme models, every dimension, colour and
// font option, the two custom-theme folder checks — is iteration 6's. A porter
// who finds themselves adding a second design field has left scope.
package design

import (
	"fmt"
	"regexp"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// BuiltInThemes is `available_themes` (design/built_in_design.py:45-49):
// `classic` plus the eight YAML-defined variants, in the order the discriminated
// union discovers them, which is the sorted order of `other_themes/*.yaml`.
var BuiltInThemes = []string{
	"classic",
	"ember",
	"engineeringclassic",
	"engineeringresumes",
	"harvard",
	"ink",
	"moderncv",
	"opal",
	"sb2nov",
}

// customThemeNamePattern is `^[a-z0-9]+$` (design/design.py:17). A name that
// fails it cannot be a folder name upstream is willing to load.
var customThemeNamePattern = regexp.MustCompile(`^[a-z0-9]+$`)

// CodeTheme is the kind the shape check raises
// (models/custom_error_types.py:6). No dictionary row matches its message.
const CodeTheme schemaerr.Code = "rendercv_other_error"

// ValidateTheme checks a `design.theme` value.
//
// A built-in name is accepted. Anything else is a custom theme, and upstream
// then requires the name to be lowercase alphanumeric **before** it would look
// for a folder (design.py:60-70) — so this check is reachable without any theme
// code existing, which is why it can land here while the rest of `design`
// waits.
//
// The returned record has **LocationIsFinal set**. Upstream re-pins the location
// through `ctx["loc"]` to `("design", "theme")` even though the failure is
// raised while validating `design` as a whole, and it re-pins the input to the
// theme name rather than the whole mapping (design.py:64-68). Without the flag
// the pipeline would drop `theme` as a discriminator branch value, since
// `design` is a discriminated root.
//
// The two folder checks that follow upstream (§4.28, §4.29) are iteration 6's:
// they are only reachable once custom themes can be loaded at all.
func ValidateTheme(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if node == nil || node.Kind == yamldoc.KindNull {
		return nil
	}

	name := node.Raw
	for _, builtIn := range BuiltInThemes {
		if name == builtIn {
			return nil
		}
	}
	if customThemeNamePattern.MatchString(name) {
		return nil
	}

	span := node.Span
	return []schemaerr.ValidationError{{
		Code: CodeTheme,
		// Already final: `("design", "theme")`, whatever the caller's location.
		SchemaLocation:  append(append([]string(nil), location...), "theme"),
		LocationIsFinal: true,
		YamlLocation:    &span,
		YamlSource:      source,
		Message: fmt.Sprintf(
			"The custom theme name should only contain lowercase letters and"+
				" digits. The provided value is `%s`.", name),
		Input: name,
	}}
}

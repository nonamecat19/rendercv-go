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
	"strings"

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
//
// **A trailing `\n` is not a failure.** Python's `$` matches at the end of
// the string *or* immediately before one trailing newline — a documented
// quirk of `re`, not specific to this pattern — so `re.match(r"^[a-z0-9]+$",
// "mytheme\n")` is true. A theme name ending in `\n` (reachable through a
// block scalar) used to fail the shape check here and report the wrong one
// of iteration 14's own two messages; upstream falls through to the folder
// checks instead. Found by a fresh-context verifier (iteration 14's
// non-colour-design-slice sweep).
var customThemeNamePattern = regexp.MustCompile(`^[a-z0-9]+\n?$`)

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
// TODO(iteration-14): the two folder checks that follow upstream (§4.28, §4.29)
// are **not** iteration 6's, though spec 006 §3 behavior 7 placed them there.
// Iteration 6's plan §7 moved them out with the D-002 Lua path, because a
// sandbox bundled with 161 `$defs` makes both unreviewable, and STATE.md carries
// the deferral as cut scope. They are only reachable once custom themes can be
// loaded at all.
func ValidateTheme(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	// **The candidate name is Python's `str(design["theme"])`, not the raw
	// token** (`design.py:57`: `theme_name = str(design["theme"])`). A null
	// used to skip this whole check — `design.py` never sees an absent key
	// here either, but a *null* value stringifies to `"None"`, which fails
	// the pattern the same as any other non-lowercase-alphanumeric name.
	// `true`/`false` stringify to `"True"`/`"False"` (capitalized, so they
	// fail too), and a sequence or mapping stringifies to Python's own
	// bracketed repr, which is also what the rejection message quotes.
	// Found by a fresh-context verifier (iteration 14's twelfth
	// re-verification).
	name := themeNameRepr(node)
	if node != nil && node.Kind == yamldoc.KindString {
		// Only a real string can be the discriminator's own value; a tagged
		// scalar spelling `classic` is a TaggedScalar upstream and matches no
		// member (see `isBuiltIn`). It still reaches the name-shape check
		// below, which runs on `str(design["theme"])`.
		for _, builtIn := range BuiltInThemes {
			if name == builtIn {
				return nil
			}
		}
	}
	if customThemeNamePattern.MatchString(name) {
		return nil
	}

	var span yamldoc.Span
	if node != nil {
		span = node.Span
	}
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

// themeNameRepr is Python's `str()` of the raw `design.theme` value — the
// candidate name upstream tests against the pattern, at the top level (an
// unquoted scalar; `None`/`True`/`False` for the three non-string scalars;
// Python's own bracketed repr for a sequence or mapping).
func themeNameRepr(node *yamldoc.Node) string {
	if node == nil || node.Kind == yamldoc.KindNull {
		return "None"
	}
	switch node.Kind {
	case yamldoc.KindBool:
		return pythonBoolRepr(node)
	case yamldoc.KindSequence:
		parts := make([]string, 0, len(node.Elems))
		for _, elem := range node.Elems {
			parts = append(parts, pythonElemRepr(elem))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case yamldoc.KindMapping:
		parts := make([]string, 0, len(node.Items))
		for _, item := range node.Items {
			parts = append(parts, "'"+item.Key+"': "+pythonElemRepr(item.Value))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case yamldoc.KindNull, yamldoc.KindInt, yamldoc.KindFloat,
		yamldoc.KindString, yamldoc.KindTagged:
		// The remaining kinds are their own text: an unquoted scalar at the top
		// level, and a `TaggedScalar` whose `str()` is its value. Spelled out
		// rather than left to a `default` so a new kind stops here (kindguard).
		// `KindNull` is unreachable — the guard above returns "None" — and is
		// named only to make the set complete.
		return node.Raw
	}
	return node.Raw
}

// pythonElemRepr is the same repr, one level down — where a string is
// quoted, because Python's `repr()` is what a container's own `str()` uses
// for its elements.
func pythonElemRepr(node *yamldoc.Node) string {
	if node == nil || node.Kind == yamldoc.KindNull {
		return "None"
	}
	switch node.Kind {
	case yamldoc.KindString:
		return "'" + node.Raw + "'"
	case yamldoc.KindBool:
		return pythonBoolRepr(node)
	case yamldoc.KindSequence, yamldoc.KindMapping:
		return themeNameRepr(node)
	case yamldoc.KindNull, yamldoc.KindInt, yamldoc.KindFloat, yamldoc.KindTagged:
		// Numbers repr as their own text. Spelled out rather than left to a
		// `default` (kindguard); `KindNull` is unreachable, the guard above
		// returns "None".
		//
		// **`KindTagged` is knowingly wrong here and left alone.** A
		// `TaggedScalar`'s `repr()` is not its text — measured against the
		// vendored ruamel, `[!!str x]` reprs its element as
		// `TaggedScalar(value='x', style=None, tag=Tag('tag:yaml.org,2002:str'))`,
		// not `x`. Matching it needs the tag itself, which `yamldoc.Node` does
		// not keep; that is a spec change, not a rename. Reachable only from
		// `design.theme` written as a container with a tagged element.
		return node.Raw
	}
	return node.Raw
}

func pythonBoolRepr(node *yamldoc.Node) string {
	if yamldoc.BoolIsTrue(node.Raw) {
		return "True"
	}
	return "False"
}

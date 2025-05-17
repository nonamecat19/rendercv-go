// Package modelbuilder mirrors src/rendercv/schema/rendercv_model_builder.py.
package modelbuilder

import (
	"errors"
	"fmt"
	"strings"

	goyaml "github.com/goccy/go-yaml"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// ReadYamlWithValidationErrors mirrors read_yaml_with_validation_errors
// (rendercv_model_builder.py:64-101): it parses YAML content and converts a
// parser failure into a single user validation error so syntax errors travel
// the same pipeline as schema validation errors.
//
// Non-parser failures from the reader (empty input, string root) pass through
// unchanged, as they do upstream.
func ReadYamlWithValidationErrors(
	content string,
	source schemaerr.YamlSource,
) (*yamldoc.Node, error) {
	node, err := yamlreader.ReadString(content)
	if err == nil {
		return node, nil
	}

	var parserErr goyaml.Error
	if !errors.As(err, &parserErr) {
		return nil, err
	}

	return nil, &schemaerr.UserValidationError{
		Errors: []schemaerr.ValidationError{yamlSyntaxValidationError(parserErr, source)},
	}
}

// yamlSyntaxValidationError builds the single record described by spec §3.83:
// no schema location, a location derived from the parser's marks, the source of
// the document being parsed, the message of §4.17, and the literal input echo.
func yamlSyntaxValidationError(
	parserErr goyaml.Error,
	source schemaerr.YamlSource,
) schemaerr.ValidationError {
	return schemaerr.ValidationError{
		SchemaLocation: nil,
		YamlLocation:   yamlErrorLocation(parserErr),
		YamlSource:     source,
		Message:        fmt.Sprintf("This is not a valid YAML file. %s", parserMessage(parserErr.Error())),
		Input:          schemaerr.InputEllipsis,
	}
}

// ruamelPhrasing maps goccy's error taxonomy onto ruamel's, for the syntax
// failures the corpus contains (spec 004 §7.5, plan §6 option B).
//
// The two libraries describe the same failures differently: goccy names the
// token it wanted, ruamel names the construct it was in. Upstream interpolates
// ruamel's phrasing into a user-visible message, so parity needs the mapping.
//
// **The set is deliberately small and grows only when a corpus case is added.**
// The corpus has exactly one syntax case today — `this: [is, not, a, cv`, whose
// golden reads `while parsing a flow sequence` — and the other four rows were
// measured alongside it because they are the shapes a user is next most likely
// to write. An unmapped failure falls through to goccy's own first line, which
// is option A for the remainder: wrong, but visibly wrong rather than silently
// misattributed.
//
// Each key is a substring of goccy's message; each value is ruamel's verbatim
// first line, measured against the vendored Python.
var ruamelPhrasing = []struct{ goccy, ruamel string }{
	{"sequence end token", "while parsing a flow sequence"},
	{"flow map", "while parsing a flow mapping"},
	{"quoted text", "while scanning a quoted scalar"},
	{"tab character", "while scanning for the next token"},
	{"already defined", "while constructing a mapping"},
}

// parserMessage mirrors rendercv_model_builder.py:87-89: the first line of the
// parser's own error text, stripped, with a period appended when absent.
//
// The first line is mapped onto ruamel's phrasing first, so what upstream
// interpolates and what the port interpolates agree for the mapped set.
func parserMessage(text string) string {
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}
	text = strings.TrimSpace(text)

	for _, row := range ruamelPhrasing {
		if strings.Contains(text, row.goccy) {
			text = row.ruamel
			break
		}
	}

	if text == "" {
		return "."
	}
	if !strings.HasSuffix(text, ".") {
		text += "."
	}
	return text
}

// yamlErrorLocation mirrors get_yaml_error_location
// (rendercv_model_builder.py:42-62). Upstream picks the start mark from the
// context mark falling back to the problem mark and the end mark the other way
// around; goccy reports a single offending token, so both ends come from it.
// Its positions are already 1-indexed, so no conversion is needed.
func yamlErrorLocation(parserErr goyaml.Error) *yamldoc.Span {
	tok := parserErr.GetToken()
	if tok == nil || tok.Position == nil {
		return nil
	}
	pos := yamldoc.Position{Line: tok.Position.Line, Column: tok.Position.Column}
	return &yamldoc.Span{Start: pos, End: pos}
}

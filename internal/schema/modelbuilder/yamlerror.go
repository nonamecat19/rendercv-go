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
		Errors: []schemaerr.ValidationError{yamlSyntaxValidationError(parserErr, content, source)},
	}
}

// yamlSyntaxValidationError builds the single record described by spec §3.83:
// no schema location, a location derived from the parser's marks, the source of
// the document being parsed, the message of §4.17, and the literal input echo.
func yamlSyntaxValidationError(
	parserErr goyaml.Error,
	content string,
	source schemaerr.YamlSource,
) schemaerr.ValidationError {
	return schemaerr.ValidationError{
		SchemaLocation: nil,
		YamlLocation:   yamlErrorLocation(parserErr, content),
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
// context mark, falling back to the problem mark, and the end mark the other
// way around. goccy's token is the *context* mark for an unterminated
// construct — measured on `this: [is, not, a, cv`, where the token is the
// `[` that opened the flow sequence, at (1, 7) — but goccy exposes no second,
// *problem* mark of its own.
//
// **For that one measured shape, the problem mark is EOF.** ruamel's scanner
// reads to the true end of the stream hunting for the closing bracket, so its
// problem_mark's line is the total newline count of the document (0-indexed,
// so `+1` to display) regardless of where the scan started. Widening the span
// to that line reproduces upstream's `line 1 to line 2` exactly.
//
// **Scoped to the one mapped case that needs it.** The corpus has exactly one
// syntax error (spec 004 §7.5, plan §6 option B, `ruamelPhrasing` above), so
// widening the span for every syntax error would be guessing at shapes never
// measured — an unmapped or differently-shaped error keeps a single-line
// span, which was already right before this fix, rather than being pushed to
// EOF on the assumption it works the same way.
func yamlErrorLocation(parserErr goyaml.Error, content string) *yamldoc.Span {
	tok := parserErr.GetToken()
	if tok == nil || tok.Position == nil {
		return nil
	}
	start := yamldoc.Position{Line: tok.Position.Line, Column: tok.Position.Column}
	end := start

	if strings.Contains(parserErr.Error(), "sequence end token") {
		if eof := strings.Count(content, "\n") + 1; eof > end.Line {
			end = yamldoc.Position{Line: eof, Column: 1}
		}
	}

	return &yamldoc.Span{Start: start, End: end}
}

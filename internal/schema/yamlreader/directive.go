package yamlreader

import (
	"fmt"

	"github.com/goccy/go-yaml/ast"
)

// A YAML *directive* is a `%`-prefixed line before the `---` marker. Upstream
// inherits full directive support from ruamel — `yaml_reader.py:81` builds a
// plain `ruamel.yaml.YAML()`, so `process_directives`
// (`ruamel/yaml/parser.py:288-330`) runs on every document — and a directive
// the parser does not recognise is dropped rather than refused.
//
// **goccy emits a directive line as a `*ast.DocumentNode` of its own**, whose
// Body is an `*ast.DirectiveNode`. The port therefore saw two documents where
// ruamel sees one and rejected *every* directive-headed document with
// `expected a single document in the stream`, including `%YAML 1.2` and
// `%FOO bar`, which upstream treats as no-ops. Spec
// `002-yaml-and-core-model/spec-delta-directives.md` §4.4.
//
// The predicate below is "the body **is** a directive", not the spec's
// provisional "has no body": measured, goccy gives the pseudo-document a body,
// and a genuine two-document stream whose second document is empty must keep
// failing — `---\nk: v\n---\n` is ruamel's `ComposerError`.

// directiveOnly reports whether a goccy document is nothing but a directive
// line, and so is not a document at all in ruamel's counting.
func directiveOnly(doc *ast.DocumentNode) bool {
	_, ok := doc.Body.(*ast.DirectiveNode)
	return ok
}

// contentDocuments drops the pseudo-documents goccy makes out of directive
// lines, leaving the documents ruamel would compose.
func contentDocuments(docs []*ast.DocumentNode) []*ast.DocumentNode {
	content := make([]*ast.DocumentNode, 0, len(docs))
	for _, doc := range docs {
		if !directiveOnly(doc) {
			content = append(content, doc)
		}
	}
	return content
}

// yamlVersionDirective is the name goccy gives a `%YAML` directive.
const yamlVersionDirective = "YAML"

// unsupportedYamlVersion is the only `%YAML` version the port refuses outright.
// D-018: upstream switches ruamel's scalar resolver to the 1.1 tables, where
// `yes` is a bool, `010` is octal and `1:30` is sexagesimal
// (`ruamel/yaml/resolver.py:30-35`, `:45-53`, `:62-69`); the port resolves by
// 1.2 always.
const unsupportedYamlVersion = "1.1"

// UnsupportedDirectiveError is D-018's named refusal.
//
// **It exists because making directives parse at all made this case silent.**
// Before the fix above, a `%YAML 1.1` document was rejected for the wrong
// reason but still rejected; afterwards it would have rendered with 1.2 values,
// turning a message defect into a value defect. The divergence therefore
// requires the port to say so rather than guess.
type UnsupportedDirectiveError struct {
	// Directive is the directive as written, e.g. `%YAML 1.1`.
	Directive string
	// Line is the 1-based line the directive sits on.
	Line int
}

func (e *UnsupportedDirectiveError) Error() string {
	return fmt.Sprintf(
		"The '%s' directive is not supported: rendercv-go resolves plain scalars"+
			" by YAML 1.2 rules only.", e.Directive)
}

// checkDirectives refuses the directives the port does not implement, and only
// those: everything else is either handled or, as ruamel does with an
// unrecognised directive, ignored.
func checkDirectives(docs []*ast.DocumentNode) error {
	for _, doc := range docs {
		directive, ok := doc.Body.(*ast.DirectiveNode)
		if !ok {
			continue
		}
		name, values := directiveWords(directive)
		if name != yamlVersionDirective || len(values) == 0 ||
			values[0] != unsupportedYamlVersion {
			continue
		}
		return &UnsupportedDirectiveError{
			Directive: "%" + name + " " + values[0],
			Line:      directiveLine(directive),
		}
	}
	return nil
}

// tagDirective is the name goccy gives a `%TAG` directive.
const tagDirective = "TAG"

// TagHandles is ruamel's per-document tag-handle table: the map a `%TAG`
// directive writes into and `Tag.trval` reads back
// (`ruamel/yaml/tag.py:55-88`, `ruamel/yaml/parser.py:106`, `:327-329`).
//
// It is a property of the *document*, not of a node — a rebound handle changes
// the value of an existing `Node.Tag` and needs no new field anywhere — so it
// is carried beside the parse state and never hung off `yamldoc.Node`.
type TagHandles map[string]string

// DefaultTagHandles is ruamel's `DEFAULT_TAGS` (`ruamel/yaml/parser.py:106`),
// the table a document with no `%TAG` directive resolves against.
func DefaultTagHandles() TagHandles {
	return TagHandles{"!": "!", "!!": "tag:yaml.org,2002:"}
}

// tagHandlesFor is the table the document resolves its tags against: the
// defaults, with every handle a `%TAG` directive bound replaced.
//
// **goccy hands back the handle unexpanded** — measured, `k: !e!x v` under
// `%TAG !e! tag:example.com,2000:` leaves the `*ast.TagNode`'s token reading
// `!e!x` — so the expansion is the port's to do
// (spec-delta-directives §6.1's probe).
func tagHandlesFor(docs []*ast.DocumentNode) TagHandles {
	handles := DefaultTagHandles()
	for _, doc := range docs {
		directive, ok := doc.Body.(*ast.DirectiveNode)
		if !ok {
			continue
		}
		if name, values := directiveWords(directive); name == tagDirective && len(values) >= 2 {
			handles[values[0]] = values[1]
		}
	}
	return handles
}

// directiveWords splits a directive into its name and its arguments — `YAML`
// and `1.2`, or `TAG` and the handle and prefix.
func directiveWords(directive *ast.DirectiveNode) (name string, values []string) {
	if directive.Name != nil {
		name = directive.Name.String()
	}
	values = make([]string, 0, len(directive.Values))
	for _, value := range directive.Values {
		if value != nil {
			values = append(values, value.String())
		}
	}
	return name, values
}

// directiveLine is the 1-based line the directive's `%` sits on, or 0.
func directiveLine(directive *ast.DirectiveNode) int {
	if directive.Start == nil || directive.Start.Position == nil {
		return 0
	}
	return directive.Start.Position.Line
}

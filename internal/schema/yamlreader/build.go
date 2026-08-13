package yamlreader

import (
	"errors"
	"fmt"
	"strings"

	goyaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

func parse(src string) (*yamldoc.Node, error) {
	if err := checkTabs(src); err != nil {
		return nil, err
	}
	file, err := parseTolerantOfQuotedTabs(src)
	if err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}
	if len(file.Docs) == 0 {
		return nil, nil
	}
	if err := checkDirectives(file.Docs); err != nil {
		return nil, err
	}
	// A directive line is a document to goccy and not one to ruamel, so the
	// count and the body both have to be taken over the content documents —
	// see directive.go.
	docs := contentDocuments(file.Docs)
	if len(docs) == 0 {
		return nil, nil
	}
	if err := checkSingleDocument(docs); err != nil {
		return nil, err
	}
	body := docs[0].Body
	if body == nil {
		return nil, nil
	}
	return builder{handles: tagHandlesFor(file.Docs)}.node(body), nil
}

// builder carries the parse state a node needs beyond its own subtree, which
// today is only the document's tag-handle table. It is a value, not a pointer,
// because nothing in it is written during a build.
type builder struct {
	handles TagHandles
}

// TabError is ruamel's scanner refusing a tab used as separation whitespace.
//
// **goccy accepts tabs in seven measured positions where ruamel rejects them**
// — after a sequence dash, after a `:`, inside a plain scalar, trailing, before
// a comment, before a flow collection — so the port rendered documents
// upstream refuses at exit 1.
//
// ruamel reports no context mark for these, so the location is the tab's own
// line alone.
type TabError struct {
	Line int
}

func (e *TabError) Error() string { return "while scanning for the next token" }

// checkTabs reproduces where YAML forbids a tab, which is everywhere it would
// act as indentation or separation in block context.
//
// A tab is **legal** in four regions, all measured against ruamel: inside a
// quoted scalar (including its continuation indentation), inside a block
// scalar's content, inside a comment's text, and anywhere within a flow
// collection. Outside those it is an error at the line it appears on.
func checkTabs(src string) error {
	lines := strings.Split(src, "\n")

	var (
		flowDepth   int
		inSingle    bool
		inDouble    bool
		blockIndent = -1 // >= 0 while inside a block scalar's content
	)

	for number, text := range lines {
		// A block scalar's content is anything indented past its header, plus
		// blank lines. Tabs inside it are ordinary characters.
		if blockIndent >= 0 && !inSingle && !inDouble {
			if strings.TrimSpace(text) == "" || indentWidth(text) > blockIndent {
				continue
			}
			blockIndent = -1
		}

		inComment := false
		var prev byte

		for i := 0; i < len(text); i++ {
			c := text[i]

			switch {
			case inComment:
			case inSingle:
				if c == '\'' {
					inSingle = false
				}
			case inDouble:
				if c == '\\' && i+1 < len(text) {
					i++
				} else if c == '"' {
					inDouble = false
				}
			case c == '\t':
				if flowDepth == 0 {
					return &TabError{Line: number + 1}
				}
			case c == '#' && (prev == 0 || prev == ' ' || prev == '\t'):
				inComment = true
			case c == '\'':
				inSingle = true
			case c == '"':
				inDouble = true
			case c == '[' || c == '{':
				flowDepth++
			case c == ']' || c == '}':
				if flowDepth > 0 {
					flowDepth--
				}
			case (c == '|' || c == '>') && flowDepth == 0 && isBlockScalarHeader(text[i:]):
				blockIndent = indentWidth(text)
			}
			prev = c
		}
	}
	return nil
}

// isBlockScalarHeader reports whether a `|` or `>` at the end of a line opens
// a block scalar, allowing for the chomping and indentation indicators.
func isBlockScalarHeader(rest string) bool {
	return strings.TrimRight(strings.TrimLeft(rest[1:], "+-0123456789"), " \t") == ""
}

// indentWidth is the number of leading spaces and tabs on a line.
func indentWidth(text string) int {
	return len(text) - len(strings.TrimLeft(text, " \t"))
}

// quotedTabIndent is goccy's complaint about a continuation line inside a
// quoted scalar that is indented with a tab.
const quotedTabIndent = "tab character cannot be used for indentation in"

// parseTolerantOfQuotedTabs parses `src`, retrying lines goccy rejects only
// because they continue a quoted scalar with a tab.
//
// **ruamel accepts a tab there and the port rejected the document.** A tab is
// an error almost everywhere in YAML, but not inside a quoted scalar: `name:
// "a\n\tb"` loads fine upstream and rendered a CV, while goccy's scanner
// refuses it, so a valid input file failed here at exit 1.
//
// The leading whitespace of a continuation line inside a quoted scalar is
// folded away entirely, so a tab and a space produce **the same value** —
// which is what makes the substitution safe. It is applied one line at a time,
// and only to lines goccy itself named, so a document that already parses is
// never touched and no hand-written tab rule can reject something valid.
func parseTolerantOfQuotedTabs(src string) (*ast.File, error) {
	file, err := parser.Parse(Dealias(lexer.Tokenize(src)), 0)

	// Each pass fixes the one line goccy reported, so the bound is the number
	// of lines that could carry the fault.
	for attempts := strings.Count(src, "\n") + 1; err != nil && attempts > 0; attempts-- {
		if !strings.Contains(err.Error(), quotedTabIndent) {
			break
		}
		fixed, ok := untabLine(src, errorLine(err))
		if !ok {
			break
		}
		src = fixed
		file, err = parser.Parse(Dealias(lexer.Tokenize(src)), 0)
	}
	return file, err
}

// errorLine is the 1-based line a goccy error points at, or 0.
func errorLine(err error) int {
	var parserErr goyaml.Error
	if !errors.As(err, &parserErr) {
		return 0
	}
	if tok := parserErr.GetToken(); tok != nil && tok.Position != nil {
		return tok.Position.Line
	}
	return 0
}

// untabLine replaces the leading tabs of one line with spaces, one for one so
// that every later column is unchanged and the error coordinates a failure
// would report stay exact.
func untabLine(src string, line int) (string, bool) {
	if line < 1 {
		return src, false
	}
	lines := strings.Split(src, "\n")
	if line > len(lines) {
		return src, false
	}

	text := lines[line-1]
	trimmed := strings.TrimLeft(text, "\t")
	if trimmed == text {
		return src, false
	}

	lines[line-1] = strings.Repeat(" ", len(text)-len(trimmed)) + trimmed
	return strings.Join(lines, "\n"), true
}

// MultiDocumentError is ruamel's composer failure for a stream carrying more
// than one document. Upstream reads the input with a plain `YAML().load`,
// which composes a *single* document and raises when a second one begins —
// the port read `Docs[0]` and ignored the rest, so a two-document stream
// rendered the first document's CV at exit 0 where upstream exits 1.
//
// It carries its own marks because ruamel's are not the failing token's: the
// context mark is where the first document's *content* began and the problem
// mark is the second document's `---`. Measured on four shapes (explicit and
// implicit first document, a `...` end marker between the two, and three
// documents, which reports the first extra one).
type MultiDocumentError struct {
	Start yamldoc.Position
	End   yamldoc.Position
}

func (e *MultiDocumentError) Error() string {
	return "expected a single document in the stream"
}

// checkSingleDocument reproduces the marks described on MultiDocumentError.
// goccy exposes both directly: the first document's body token is ruamel's
// context mark, and the second document's `---` token is its problem mark.
func checkSingleDocument(docs []*ast.DocumentNode) error {
	if len(docs) < 2 {
		return nil
	}

	err := &MultiDocumentError{}
	if body := docs[0].Body; body != nil && body.GetToken() != nil {
		pos := body.GetToken().Position
		err.Start = yamldoc.Position{Line: pos.Line, Column: pos.Column}
	}
	if start := docs[1].Start; start != nil && start.Position != nil {
		err.End = yamldoc.Position{Line: start.Position.Line, Column: start.Position.Column}
	}
	return err
}

func (b builder) node(n ast.Node) *yamldoc.Node {
	switch v := n.(type) {
	case *ast.MappingNode:
		return b.mapping(v)
	case *ast.SequenceNode:
		return b.sequence(v)
	case *ast.AnchorNode:
		return b.node(v.Value)
	case *ast.TagNode:
		return b.tagged(v)
	case *ast.LiteralNode:
		return buildLiteral(v)
	case *ast.StringNode:
		return buildPlainScalar(v.Token)
	case *ast.IntegerNode:
		return buildPlainScalar(v.Token)
	case *ast.FloatNode:
		return buildPlainScalar(v.Token)
	case *ast.BoolNode:
		return buildPlainScalar(v.Token)
	case *ast.NullNode:
		return buildPlainScalar(v.Token)
	case *ast.InfinityNode:
		return buildPlainScalar(v.Token)
	case *ast.NanNode:
		return buildPlainScalar(v.Token)
	default:
		return &yamldoc.Node{Kind: yamldoc.KindNull}
	}
}

// buildTagged resolves a node carrying an explicit tag.
//
// **It branches on the node's shape before it looks at the tag**, because
// upstream does: ruamel registers `construct_unknown` for every tag it has no
// constructor for (`ruamel/yaml/constructor.py:1724`), and that function tests
// `isinstance(node, MappingNode)` / `SequenceNode` / `ScalarNode`
// (`:1598-1640`), so a tag on a collection is dropped and the collection is an
// ordinary one. `construct_yaml_str` defers to the same function whenever the
// node carries a tag handle (`:1181-1184`), which is why an explicit `!!str`
// is not the no-op it looks like.
//
// A *known scalar* tag over a collection (`!!str [1, 2]`, which upstream reads
// as a plain sequence) never reaches here: goccy's parser refuses the document
// with `unexpected scalar value type`. Recorded as a divergence.
func (b builder) tagged(node *ast.TagNode) *yamldoc.Node {
	if node.Value == nil {
		return &yamldoc.Node{Kind: yamldoc.KindNull}
	}

	// **A bare `!` is the non-specific tag and names no type.** It asks the
	// resolver for the answer it would have given anyway, so `! x` is the
	// string `x` and `! 31` the integer 31 — `inner`, exactly as built. It has
	// to be caught before `ResolveTag`, whose "everything else is opaque" arm
	// would make it a `TaggedScalar`; that is the kind every typed field
	// rejects, so `locale.language: ! english` failed here and raises nothing
	// upstream. Spec 015 delta §3.3.
	if tagName(node) == "!" {
		return b.node(node.Value)
	}

	inner := b.node(node.Value)
	switch inner.Kind {
	case yamldoc.KindMapping, yamldoc.KindSequence:
		return inner
	case yamldoc.KindNull, yamldoc.KindBool, yamldoc.KindInt, yamldoc.KindFloat,
		yamldoc.KindString, yamldoc.KindTagged:
		// A scalar; the tag decides its kind below.
	}

	// The tag is expanded through the document's handle table *before* the
	// constructor lookup, because ruamel looks the constructor up by the
	// resolved URI: `!!int` is not `tag:yaml.org,2002:int` in a document whose
	// `%TAG !!` directive rebound the handle.
	resolved := ResolveTagText(tagName(node), b.handles)
	kind, forced := ResolveTag(resolved)
	if !forced {
		// The resolved tag, kept for `repr(TaggedScalar)` — the one thing this
		// function used to compute and discard (spec 015 delta §2). Only an
		// opaque scalar carries it: a forced tag constructs an ordinary value,
		// which has no tag in its repr.
		inner.Tag = resolved
		// Opaque: ruamel's TaggedScalar, which keeps the scalar's text and
		// nothing else. A tag with no value at all carries the empty string
		// rather than a null — `a: !!str` is `TaggedScalar('')` upstream, and
		// goccy synthesizes a `null` token for it, so the text has to be
		// dropped here or the empty case would read as the four letters
		// `null`. `a: !!str null`, where the user wrote them, keeps them.
		inner.Kind = kind
		if !hasWrittenValue(node) {
			inner.Raw = ""
		}
		return inner
	}

	inner.Kind = kind
	if kind == yamldoc.KindNull {
		// `!!null x` is `None`: the constructor never reads the scalar, so the
		// text is not part of the value and must not reach an error message as
		// if it were.
		inner.Raw = ""
	}
	return inner
}

// hasWrittenValue reports whether a tagged node has a value in the source at
// all, as opposed to the null goccy synthesizes for a bare `a: !!str`.
//
// The discriminator is position: a synthesized token is reported *inside* the
// tag (one column past its start), while anything the user wrote begins after
// the tag ends. Measured on all four spellings — a bare tag, a written `null`,
// a written `~`, and a bare `!!null`.
func hasWrittenValue(node *ast.TagNode) bool {
	tok := node.Value.GetToken()
	if node.Start == nil || node.Start.Position == nil || tok == nil || tok.Position == nil {
		return true
	}
	if tok.Position.Line != node.Start.Position.Line {
		return true
	}
	return tok.Position.Column >= node.Start.Position.Column+len(node.Start.Value)
}

// untagKey strips an explicit tag from a mapping key, reporting whether the tag
// left the key opaque.
//
// A tag that names a constructed type does not: `!!int 1` is the integer 1
// upstream, which is not a string either, but it fails further along in a way
// this port does not reproduce (`RenderCVInternalError: Key '1' not found in the
// YAML file.`, the D-011 class) and so is left where an untagged `1: x` already
// is.
func (b builder) untagKey(key ast.Node) (ast.Node, bool) {
	tag, ok := key.(*ast.TagNode)
	if !ok || tag.Value == nil {
		return key, false
	}
	if _, forced := ResolveTag(ResolveTagText(tagName(tag), b.handles)); forced {
		return tag.Value, false
	}
	return tag.Value, true
}

// tagName is the tag as written — `!!str`, `!unknown`, `!!python/object`.
func tagName(node *ast.TagNode) string {
	if node.Start == nil {
		return ""
	}
	return node.Start.Value
}

func buildPlainScalar(tok *token.Token) *yamldoc.Node {
	raw := scalarRaw(tok)
	style := scalarStyle(tok)
	kind := ResolveScalar(raw, style)
	return &yamldoc.Node{
		Kind:  kind,
		Raw:   raw,
		Style: style,
		Span: yamldoc.Span{
			Start: yamldoc.Position{Line: tok.Position.Line, Column: tok.Position.Column},
			End:   yamldoc.Position{Line: tok.Position.Line, Column: tok.Position.Column},
		},
	}
}

func (b builder) mapping(n *ast.MappingNode) *yamldoc.Node {
	node := &yamldoc.Node{Kind: yamldoc.KindMapping}
	for _, mv := range n.Values {
		// A tagged key is upstream's TaggedScalar again, and a mapping key is
		// the one place its opacity is not the field's problem to report:
		// pydantic never looks the field up, and says `Keys should be strings.`
		// against the enclosing mapping (measured on `cv: {!!str name: Bob}`).
		// The tag itself is not part of the key's text, so the inner scalar is
		// what the span and the Input Value column read.
		keyNode, keyTagged := b.untagKey(mv.Key)
		keyTok := keyNode.GetToken()
		// The token's value, not the node's source form: `mv.Key.String()` keeps
		// the quotes a quoted key was written with, so `"name": John` would bind
		// the field `"name"` and then report it as an unknown key. Upstream's
		// ruamel unquotes it (measured: `{"name": …, 'email': …}` reads back as
		// `['name', 'email']`), and scalar *values* already go through the same
		// token here — only keys were reading the source text.
		key := scalarRaw(keyTok)
		valEnd := valueEndPosition(mv.Value, keyTok)

		keySpan := yamldoc.Span{
			Start: yamldoc.Position{Line: keyTok.Position.Line, Column: keyTok.Position.Column},
			End:   valEnd,
		}

		item := yamldoc.Item{
			Key:     key,
			KeySpan: keySpan,
			Value:   b.node(mv.Value),
			// The key as a node, built from `mv.Key` — the tag still on it,
			// the same path a value takes. That is ruamel's own shape, and it
			// is what makes a key's repr fall out: an unforced tag leaves a
			// KindTagged node and a forced one constructs the value, so
			// `{!!str k: a}` reprs as a TaggedScalar and `{!!int 1: a}` as 1.
			// The untagged `keyNode` above is the *binding* key's text and
			// answers a different question.
			KeyNode:   b.node(mv.Key),
			KeyTagged: keyTagged,
		}
		node.Items = append(node.Items, item)
	}
	return node
}

// isImplicitNull reports whether a null node came from a key written with no
// value, as opposed to an explicit `null` or `~`. The parser marks the two
// differently, which matters because only the implicit one lacks a token to
// end the key's span at.
func isImplicitNull(null *ast.NullNode) bool {
	tok := null.GetToken()
	return tok != nil && tok.Type == token.ImplicitNullType
}

// tokenAfterKey returns the first real token following a key, or nil at the end
// of the document.
//
// Two token types are skipped because neither is content the user wrote: the
// key's own `:`, and the synthetic implicit-null the parser appends when a
// valueless key is the last thing in the document.
func tokenAfterKey(keyTok *token.Token) *token.Token {
	next := keyTok.Next
	for next != nil &&
		(next.Type == token.MappingValueType || next.Type == token.ImplicitNullType) {
		next = next.Next
	}
	return next
}

func valueEndPosition(val ast.Node, keyTok *token.Token) yamldoc.Position {
	if null, ok := val.(*ast.NullNode); ok && isImplicitNull(null) {
		// A key written with **no value at all** has no value token to point at,
		// so ruamel ends its span at the start of the next token in the document,
		// wherever that is — the same indent when the next key is a sibling, a
		// different one when it dedents, and skipping blank lines either way.
		// Measured: `a:\n  b:\nc: 1` gives `a.b` the end `[2, 0]`, and
		// `a:\n  b:\n\n\n  d: 1` gives it `[4, 2]`.
		//
		// This is only for the implicit case. An explicit `null` or `~` has a
		// token of its own and ends at that token, like any other value
		// (measured: `b: null` ends at `[1, 3]`), so it falls through below.
		if next := tokenAfterKey(keyTok); next != nil {
			return yamldoc.Position{Line: next.Position.Line, Column: next.Position.Column}
		}
		// Nothing follows: the span ends at the start of the line after the key.
		return yamldoc.Position{Line: keyTok.Position.Line + 1, Column: 1}
	}
	if seq, ok := val.(*ast.SequenceNode); ok && seq.Start != nil {
		return yamldoc.Position{Line: seq.Start.Position.Line, Column: seq.Start.Position.Column}
	}
	first := firstToken(val)
	return yamldoc.Position{Line: first.Position.Line, Column: first.Position.Column}
}

func (b builder) sequence(n *ast.SequenceNode) *yamldoc.Node {
	node := &yamldoc.Node{Kind: yamldoc.KindSequence}
	for i, entry := range n.Values {
		elem := b.node(entry)
		pos := sequenceEntryPosition(n, i, entry)
		elem.Span = yamldoc.Span{
			Start: pos,
			End:   pos,
		}
		node.Elems = append(node.Elems, elem)
	}
	return node
}

func sequenceEntryPosition(seq *ast.SequenceNode, idx int, entry ast.Node) yamldoc.Position {
	// An element that is itself a flow collection starts at its opening bracket,
	// not at its first value. `yaml_location: [[4, 3], [4, 11]]` puts element 1
	// at the second `[` (measured: column 29, 0-indexed 28), where reading
	// through to the first value would give the `4` one column later.
	//
	// A block element has no bracket, so it keeps starting at its first token —
	// which for a mapping is its first key, the position ruamel reports there.
	if open := flowOpenToken(entry); open != nil {
		return yamldoc.Position{Line: open.Position.Line, Column: open.Position.Column}
	}
	first := firstToken(entry)
	return yamldoc.Position{Line: first.Position.Line, Column: first.Position.Column}
}

// flowOpenToken returns the `[` or `{` a flow collection opens with, or nil if
// the node is not a flow collection.
func flowOpenToken(n ast.Node) *token.Token {
	switch v := n.(type) {
	case *ast.SequenceNode:
		if v.IsFlowStyle {
			return v.Start
		}
	case *ast.MappingNode:
		if v.IsFlowStyle {
			return v.Start
		}
	}
	return nil
}

func firstToken(n ast.Node) *token.Token {
	switch v := n.(type) {
	case *ast.MappingNode:
		if len(v.Values) > 0 {
			return v.Values[0].Key.GetToken()
		}
		return v.Start
	case *ast.SequenceNode:
		if len(v.Values) > 0 {
			return firstToken(v.Values[0])
		}
		return v.Start
	case *ast.AnchorNode:
		return firstToken(v.Value)
	case *ast.StringNode:
		return v.Token
	case *ast.IntegerNode:
		return v.Token
	case *ast.FloatNode:
		return v.Token
	case *ast.BoolNode:
		return v.Token
	case *ast.NullNode:
		return v.Token
	case *ast.InfinityNode:
		return v.Token
	case *ast.NanNode:
		return v.Token
	case *ast.LiteralNode:
		return v.Start
	default:
		return n.GetToken()
	}
}

// buildLiteral reads a block scalar — `key: |` and `key: >` with their chomping
// variants.
//
// **The body lives in `n.Value`, not in `n.Start`.** `n.Start` is the *indicator*
// token, so reading it yielded `" |\n"` — the indicator itself — and the block's
// content never reached the model at all. Every artifact carried a literal `|`
// where the user's text belonged. Found by a fresh-context verifier and present
// since the reader was first written.
//
// goccy has already applied the folding and chomping rules by this point, so
// `n.Value.Value` is the finished string.
func buildLiteral(n *ast.LiteralNode) *yamldoc.Node {
	tok := n.Start
	raw := ""
	if n.Value != nil {
		raw = n.Value.Value
	}
	return &yamldoc.Node{
		Kind:  yamldoc.KindString,
		Raw:   raw,
		Style: scalarStyle(tok),
		Span: yamldoc.Span{
			Start: yamldoc.Position{Line: tok.Position.Line, Column: tok.Position.Column},
			End:   yamldoc.Position{Line: tok.Position.Line, Column: tok.Position.Column},
		},
	}
}

// scalarRaw is the scalar's value without the surrounding layout that goccy
// keeps in Origin: a token origin carries the indentation and line break around
// the value, which is not part of the scalar and must not reach classification
// (resolve.go) or the models.
//
// **A quoted token's Value is authoritative even when it is empty.** Falling
// back to Origin for `degree: ""` returns the two quote characters themselves,
// so the empty string becomes a two-character string that no longer compares
// equal to `""` — it reaches the artifact as a literal `""` and reaches
// `render_entry_templates`' empty-value filter as a non-empty value. The
// fallback exists for tokens goccy leaves without a Value at all, which is not
// the quoted ones.
func scalarRaw(tok *token.Token) string {
	switch tok.Type {
	case token.SingleQuoteType, token.DoubleQuoteType, token.LiteralType, token.FoldedType:
		return tok.Value
	default:
	}
	if tok.Value != "" {
		return tok.Value
	}
	return strings.TrimSpace(tok.Origin)
}

func scalarStyle(tok *token.Token) yamldoc.ScalarStyle {
	switch tok.Type {
	case token.SingleQuoteType:
		return yamldoc.StyleSingleQuoted
	case token.DoubleQuoteType:
		return yamldoc.StyleDoubleQuoted
	case token.LiteralType:
		return yamldoc.StyleLiteral
	case token.FoldedType:
		return yamldoc.StyleFolded
	default:
		return yamldoc.StylePlain
	}
}

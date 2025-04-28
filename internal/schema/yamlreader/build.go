package yamlreader

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

func parse(src string) (*yamldoc.Node, error) {
	tokens := Dealias(lexer.Tokenize(src))
	file, err := parser.Parse(tokens, 0)
	if err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}
	if len(file.Docs) == 0 {
		return nil, nil
	}
	body := file.Docs[0].Body
	if body == nil {
		return nil, nil
	}
	return buildNode(body), nil
}

func buildNode(n ast.Node) *yamldoc.Node {
	switch v := n.(type) {
	case *ast.MappingNode:
		return buildMapping(v)
	case *ast.SequenceNode:
		return buildSequence(v)
	case *ast.AnchorNode:
		return buildNode(v.Value)
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

func buildMapping(n *ast.MappingNode) *yamldoc.Node {
	node := &yamldoc.Node{Kind: yamldoc.KindMapping}
	for _, mv := range n.Values {
		keyTok := mv.Key.GetToken()
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
			Value:   buildNode(mv.Value),
		}
		node.Items = append(node.Items, item)
	}
	return node
}

func valueEndPosition(val ast.Node, keyTok *token.Token) yamldoc.Position {
	if _, ok := val.(*ast.NullNode); ok {
		return yamldoc.Position{Line: keyTok.Position.Line + 1, Column: 1}
	}
	if seq, ok := val.(*ast.SequenceNode); ok && seq.Start != nil {
		return yamldoc.Position{Line: seq.Start.Position.Line, Column: seq.Start.Position.Column}
	}
	first := firstToken(val)
	return yamldoc.Position{Line: first.Position.Line, Column: first.Position.Column}
}

func buildSequence(n *ast.SequenceNode) *yamldoc.Node {
	node := &yamldoc.Node{Kind: yamldoc.KindSequence}
	for i, entry := range n.Values {
		elem := buildNode(entry)
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
	first := firstToken(entry)
	return yamldoc.Position{Line: first.Position.Line, Column: first.Position.Column}
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

func buildLiteral(n *ast.LiteralNode) *yamldoc.Node {
	tok := n.Start
	raw := tok.Origin
	if raw == "" {
		raw = tok.Value
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
func scalarRaw(tok *token.Token) string {
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

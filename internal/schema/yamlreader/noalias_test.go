package yamlreader_test

import (
	"testing"

	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/token"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

func TestDealiasProbeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		checks func(t *testing.T, tokens token.Tokens)
	}{
		{
			name:  "key: *not_an_alias",
			input: "key: *not_an_alias\n",
			checks: func(t *testing.T, tokens token.Tokens) {
				assertNoAlias(t, tokens)
				assertStringValue(t, tokens, "*not_an_alias")
			},
		},
		{
			name:  "mixed: *a and more",
			input: "mixed: *a and more\n",
			checks: func(t *testing.T, tokens token.Tokens) {
				assertNoAlias(t, tokens)
				assertStringValue(t, tokens, "*a and more")
			},
		},
		{
			name:  "multi: - *one / - *two",
			input: "multi:\n  - *one\n  - *two\n",
			checks: func(t *testing.T, tokens token.Tokens) {
				assertNoAlias(t, tokens)
				assertStringValue(t, tokens, "*one")
				assertStringValue(t, tokens, "*two")
			},
		},
		{
			name:  "nested: inner: *deep_value",
			input: "nested:\n  inner: *deep_value\n",
			checks: func(t *testing.T, tokens token.Tokens) {
				assertNoAlias(t, tokens)
				assertStringValue(t, tokens, "*deep_value")
			},
		},
		{
			name:  "real_anchor: &anchor value / use: *anchor",
			input: "real_anchor: &anchor value\nuse: *anchor\n",
			checks: func(t *testing.T, tokens token.Tokens) {
				assertNoAlias(t, tokens)
				assertHasAnchor(t, tokens)
				assertStringValue(t, tokens, "*anchor")
			},
		},
		{
			name:  "highlights: - normal *star* here",
			input: "highlights:\n  - normal *star* here\n",
			checks: func(t *testing.T, tokens token.Tokens) {
				assertNoAlias(t, tokens)
			},
		},
		{
			name:  "b: '*quoted'",
			input: "b: '*quoted'\n",
			checks: func(t *testing.T, tokens token.Tokens) {
				assertNoAlias(t, tokens)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := yamlreader.Dealias(lexer.Tokenize(tt.input))
			tt.checks(t, tokens)
		})
	}
}

func assertNoAlias(t *testing.T, tokens token.Tokens) {
	t.Helper()
	for _, tok := range tokens {
		if tok.Type == token.AliasType {
			t.Errorf("unexpected AliasType token: %+v", tok)
		}
	}
}

func assertHasAnchor(t *testing.T, tokens token.Tokens) {
	t.Helper()
	for _, tok := range tokens {
		if tok.Type == token.AnchorType {
			return
		}
	}
	t.Error("expected AnchorType token not found")
}

func assertStringValue(t *testing.T, tokens token.Tokens, want string) {
	t.Helper()
	for _, tok := range tokens {
		if tok.Type == token.StringType && tok.Value == want {
			return
		}
	}
	t.Errorf("expected StringType token with Value=%q not found", want)
}

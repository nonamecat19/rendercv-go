package yamlreader

import (
	"strings"

	"github.com/goccy/go-yaml/token"
)

// Dealias rewrites alias tokens into plain strings so a `*` that a user meant
// literally is never resolved as an anchor reference (spec §3.10).
func Dealias(tokens token.Tokens) token.Tokens {
	if len(tokens) == 0 {
		return tokens
	}

	var result token.Tokens
	i := 0
	for i < len(tokens) {
		t := tokens[i]
		if t.Type != token.AliasType {
			result = append(result, t)
			i++
			continue
		}

		aliasLine := t.Position.Line
		var origins []string
		origins = append(origins, t.Origin)

		j := i + 1
		for j < len(tokens) {
			next := tokens[j]
			if next.Type == token.StringType && next.Position.Line == aliasLine {
				origins = append(origins, next.Origin)
				j++
			} else {
				break
			}
		}

		merged := strings.TrimLeft(strings.Join(origins, ""), " \t")

		newTok := token.String(merged, merged, t.Position)
		result = append(result, newTok)
		i = j
	}

	return result
}

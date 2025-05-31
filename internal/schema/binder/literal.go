package binder

import "strings"

// LiteralMessage is pydantic's `literal_error` text for a `Literal[...]` union:
// the members in **declaration** order, single-quoted, comma-separated, with
// `or` before the last and no serial comma.
//
// Measured on two unrelated unions — `SocialNetworkName`'s seventeen and
// `PageSize`'s four — which is why it lives in `binder` rather than beside
// either. Building it from the member list rather than writing it out is what
// stops a message naming sixteen names after a seventeenth is added.
//
// No dictionary row matches it, so the pipeline only appends a period.
//
// empty is what to say when there are no members at all. It cannot happen for
// any union in the tree and each caller still has to name one, because a shared
// fallback would be a string no measurement backs.
func LiteralMessage(members []string, empty string) string {
	quoted := make([]string, 0, len(members))
	for _, member := range members {
		quoted = append(quoted, "'"+member+"'")
	}
	switch len(quoted) {
	case 0:
		return empty
	case 1:
		return "Input should be " + quoted[0]
	}
	return "Input should be " +
		strings.Join(quoted[:len(quoted)-1], ", ") + " or " + quoted[len(quoted)-1]
}

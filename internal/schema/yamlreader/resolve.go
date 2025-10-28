package yamlreader

import (
	"errors"
	"strconv"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// ResolveScalar classifies a scalar's text into a node kind. Timestamps are
// deliberately left unresolved: upstream keeps them as strings (spec §3.11).
func ResolveScalar(raw string, style yamldoc.ScalarStyle) yamldoc.Kind {
	if style != yamldoc.StylePlain {
		return yamldoc.KindString
	}

	if raw == "" || raw == "~" || raw == "null" || raw == "Null" || raw == "NULL" {
		return yamldoc.KindNull
	}

	if raw == "true" || raw == "True" || raw == "TRUE" ||
		raw == "false" || raw == "False" || raw == "FALSE" {
		return yamldoc.KindBool
	}

	if raw == ".inf" || raw == ".Inf" || raw == ".INF" ||
		raw == "+.inf" || raw == "+.Inf" || raw == "+.INF" ||
		raw == "-.inf" || raw == "-.Inf" || raw == "-.INF" {
		return yamldoc.KindFloat
	}

	if raw == ".nan" || raw == ".NaN" || raw == ".NAN" {
		return yamldoc.KindFloat
	}

	if isInteger(raw) {
		return yamldoc.KindInt
	}

	if isFloat(raw) {
		return yamldoc.KindFloat
	}

	return yamldoc.KindString
}

func isInteger(s string) bool {
	if s == "" {
		return false
	}

	if s[0] == '+' || s[0] == '-' {
		s = s[1:]
	}
	if s == "" {
		return false
	}

	if s[0] == '0' && len(s) > 1 {
		switch s[1] {
		case 'x', 'X':
			return isHexInteger(s[2:])
		case 'o', 'O':
			return isOctalInteger(s[2:])
		case 'b', 'B':
			// YAML 1.1's binary form, which ruamel still resolves: `0b101` reads
			// back as the integer 5, not as a string (measured).
			return isBinaryInteger(s[2:])
		}
	}

	if strings.Contains(s, "_") {
		s = strings.ReplaceAll(s, "_", "")
	}

	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func isHexInteger(s string) bool {
	if s == "" {
		return false
	}
	s = strings.ReplaceAll(s, "_", "")
	for _, ch := range s {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') && (ch < 'A' || ch > 'F') {
			return false
		}
	}
	return true
}

func isOctalInteger(s string) bool {
	if s == "" {
		return false
	}
	s = strings.ReplaceAll(s, "_", "")
	for _, ch := range s {
		if ch < '0' || ch > '7' {
			return false
		}
	}
	return true
}

func isBinaryInteger(s string) bool {
	if s == "" {
		return false
	}
	s = strings.ReplaceAll(s, "_", "")
	for _, ch := range s {
		if ch != '0' && ch != '1' {
			return false
		}
	}
	return true
}

func isFloat(s string) bool {
	if s == "" {
		return false
	}

	if s[0] == '+' || s[0] == '-' {
		s = s[1:]
	}
	if s == "" {
		return false
	}

	if s == ".inf" || s == ".Inf" || s == ".INF" ||
		s == ".nan" || s == ".NaN" || s == ".NAN" {
		return true
	}

	s = strings.ReplaceAll(s, "_", "")

	if !strings.ContainsAny(s, ".eE") {
		return false
	}

	// **An overflowing float is still a float.** `float("1e400")` is `inf` in
	// Python and ruamel resolves the scalar to one, where Go's `ParseFloat`
	// returns `+Inf` *together with* `ErrRange` — so testing `err == nil`
	// dropped the value to a string, and it reached a bool field carrying the
	// wrong pydantic error. Underflow is the same shape: `1e-400` is `0.0` to
	// both, and `ErrRange` to Go alone.
	_, err := strconv.ParseFloat(s, 64)
	return err == nil || errors.Is(err, strconv.ErrRange)
}

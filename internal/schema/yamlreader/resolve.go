package yamlreader

import (
	"math"
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

	_, err := strconv.ParseFloat(s, 64)
	return err == nil && !math.IsInf(parseFloatRaw(s), 0)
}

func parseFloatRaw(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

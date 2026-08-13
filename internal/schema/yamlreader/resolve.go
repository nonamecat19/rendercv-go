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

// ResolveTagText is ruamel's `Tag.trval` (`ruamel/yaml/tag.py:55-88`): the tag
// a `TaggedScalar`'s `repr()` names, from the handle the author wrote.
//
// The scanner splits the written text into a handle and a suffix, and `trval`
// is `handles[handle] + uri_decode(suffix)` with
// `DEFAULT_TAGS = {'!': '!', '!!': 'tag:yaml.org,2002:'}`
// (`ruamel/yaml/parser.py:106`). Written out, and each row measured:
//
//	!!str        tag:yaml.org,2002:str    the `!!` handle expands
//	!!foo/bar    tag:yaml.org,2002:foo/bar  suffix verbatim, `/` and case kept
//	!unknown     !unknown                 the `!` handle maps to `!`
//	!!           !!                       **not** the `!!` handle — see below
//	!%21         !!                       the suffix is URI-decoded
//	!<!local>    !local                   the verbatim form is its own URI
//
// **`!!` alone is not the `!!` handle.** A handle needs a non-empty suffix, so
// the scanner reads a local tag whose suffix is `!`, and `!` + `!` is `!!`.
// Reading it as the handle would give `tag:yaml.org,2002:` instead.
//
// A bare `!` never reaches here: it is the non-specific tag and `buildTagged`
// returns the plainly resolved node before asking.
//
// **The table is the document's, not a constant**, because a `%TAG` directive
// rebinds a handle for the document that carries it. Measured against upstream:
//
//	%TAG !! tag:example.com,2000:   !!str x   tag:example.com,2000:str
//	%TAG !e! tag:example.com,2000:  !e!x v    tag:example.com,2000:x
//	%TAG ! tag:example.com,2000:    !foo v    tag:example.com,2000:foo
//	%TAG !e! tag:example.com,2000:  !e!a%21b  tag:example.com,2000:a!b
//
// A verbatim `!<…>` tag is its own URI and no handle touches it.
func ResolveTagText(written string, handles TagHandles) string {
	if verbatim, ok := strings.CutPrefix(written, "!<"); ok {
		if uri, closed := strings.CutSuffix(verbatim, ">"); closed {
			return uriDecode(uri)
		}
	}
	if handle, suffix, ok := splitTagHandle(written); ok {
		if prefix, bound := handles[handle]; bound {
			return prefix + uriDecode(suffix)
		}
	}
	return handles["!"] + uriDecode(strings.TrimPrefix(written, "!"))
}

// splitTagHandle splits a written tag into the handle the scanner reads and the
// suffix that follows it — `!!str` into `!!` and `str`, `!e!x` into `!e!` and
// `x`, `!foo` into `!` and `foo`.
//
// **A handle needs a non-empty suffix**, so `!!` alone is not the `!!` handle:
// the scanner reads a local tag whose suffix is `!`, which is why `!!` reprs as
// `!!` and not as `tag:yaml.org,2002:`.
//
// **The search for the handle's closing `!` starts *after* the short handle the
// scanner already consumed** (`ruamel/yaml/scanner.py:1047-1086`): `!!e!x` is
// the handle `!e!` over the suffix `x`, not the handle `!!` over `e!x`.
// Measured under `%TAG !e! tag:example.com,2000:`, where upstream reprs it as
// `Tag('tag:example.com,2000:x')` and this function used to answer
// `tag:yaml.org,2002:e!x`. Only the *first* `!` after that prefix closes the
// handle, so a suffix may carry more of them — `!e!x!y` is `!e!` over `x!y`.
func splitTagHandle(written string) (handle, suffix string, ok bool) {
	if !strings.HasPrefix(written, "!") {
		return "", "", false
	}
	short := 1
	if strings.HasPrefix(written, "!!") {
		short = 2
	}
	if end := strings.IndexByte(written[short:], '!'); end >= 0 {
		handle, suffix = written[short-1:short+end+1], written[short+end+1:]
		if suffix != "" {
			return handle, suffix, true
		}
		return "", "", false
	}
	if suffix = written[short:]; suffix != "" {
		return written[:short], suffix, true
	}
	return "", "", false
}

// uriDecode resolves the `%XX` escapes a tag suffix may carry —
// `Tag.uri_decoded_suffix` (`ruamel/yaml/tag.py:62`). An escape that is not two
// hex digits is left as written, which is what a malformed tag should do here:
// this is a rendering path, not a validator.
func uriDecode(text string) string {
	if !strings.Contains(text, "%") {
		return text
	}
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] != '%' || i+2 >= len(text) {
			out.WriteByte(text[i])
			continue
		}
		decoded, err := strconv.ParseUint(text[i+1:i+3], 16, 8)
		if err != nil {
			out.WriteByte(text[i])
			continue
		}
		out.WriteByte(byte(decoded))
		i += 2
	}
	return out.String()
}

// ResolveTag answers the same question as ResolveScalar — what type is this
// scalar — for a scalar carrying an explicit tag, which supplies the answer
// instead of leaving it to the text.
//
// A tag ruamel has a constructor for **forces** that type, whatever the text
// says and whatever style it was written in: `!!int "200"` is the integer 200
// and `!!bool "yes"` is `True` (measured through upstream's own loader). The
// forcing tags and the kinds they name:
//
//   - `!!int`, `!!float`, `!!bool` — the constructor parses the scalar, and
//     raises if it cannot. A text that does not parse is left with the forced
//     kind here; upstream's `ValueError`/`KeyError` is an unhandled-exception
//     traceback, the D-011 class, recorded rather than reproduced.
//   - `!!null` — `None` regardless of the text, so `!!null x` is a null and the
//     `x` is gone (`ok` is reported with an empty raw by the caller).
//   - `!!timestamp` — a plain string, and **only because upstream replaces that
//     constructor** with `construct_scalar` (`yaml_reader.py:83-86`); ruamel's
//     own would give a `date`.
//
// Every other tag — `!!str` included, and every unknown one — makes the scalar
// opaque, which is KindTagged's job and not this function's.
//
// **The argument is the *resolved* tag, not the written one**, because ruamel
// looks the constructor up by the expanded URI. The two only coincide when the
// handles are the defaults: measured, `!!int 1` under
// `%TAG !! tag:example.com,2000:` is a `TaggedScalar`, not the integer 1, while
// `!e!int 5` under `%TAG !e! tag:yaml.org,2002:` *is* the integer 5 — and so is
// the verbatim `!<tag:yaml.org,2002:int> 7`.
func ResolveTag(resolved string) (yamldoc.Kind, bool) {
	switch resolved {
	case "tag:yaml.org,2002:int":
		return yamldoc.KindInt, true
	case "tag:yaml.org,2002:float":
		return yamldoc.KindFloat, true
	case "tag:yaml.org,2002:bool":
		return yamldoc.KindBool, true
	case "tag:yaml.org,2002:null":
		return yamldoc.KindNull, true
	case "tag:yaml.org,2002:timestamp":
		return yamldoc.KindString, true
	}
	return yamldoc.KindTagged, false
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

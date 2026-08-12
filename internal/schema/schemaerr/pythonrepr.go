package schemaerr

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// PythonText is `str(value)` of a parsed YAML node — what upstream's messages
// quote when they name the value itself rather than the Input Value column.
//
// **It is not RenderInput.** The two agree on every scalar and differ on
// containers, deliberately: the column renders a mapping or a sequence as `...`
// (`pydantic_error_handling.py:126`) while a message quotes Python's own repr,
// so `locale: {language: {a: 1}}` reads `Input tag '{'a': 1}'` and not
// `Input tag '...'`.
//
// **A number is its value, not its token.** `0x1f` is `31`, `1_000` is `1000`
// and `1.50` is `1.5`, at the top level and inside a container alike — the
// scalar arms come from RenderInput so the two cannot drift.
//
// Measured through the vendored `build_rendercv_dictionary_and_model` on the
// `union_tag_invalid` message for thirty-five shapes of `locale.language`.
func PythonText(node *yamldoc.Node) string {
	if node == nil {
		return "None"
	}
	switch node.Kind {
	case yamldoc.KindSequence:
		parts := make([]string, 0, len(node.Elems))
		for _, elem := range node.Elems {
			parts = append(parts, pythonRepr(elem))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case yamldoc.KindMapping:
		parts := make([]string, 0, len(node.Items))
		for _, item := range node.Items {
			// **The key is repr'd like any other value, and this renders every
			// key as a string.** Upstream's `{1: a}` is `{1: 'a'}`, with the
			// integer key unquoted, but `yamldoc.Item` carries the key's text
			// and neither its kind nor its quoting style, so `1:` and `'1':`
			// are the same three bytes here. Guessing from the text would get
			// the quoted spelling wrong instead, so the guess is not made.
			// Telling the two apart needs an `Item` that keeps the key's node
			// — the same change `repr(TaggedScalar)` waits on.
			parts = append(parts, pythonStringRepr(item.Key)+": "+pythonRepr(item.Value))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case yamldoc.KindNull, yamldoc.KindBool, yamldoc.KindInt, yamldoc.KindFloat,
		yamldoc.KindString, yamldoc.KindTagged:
		// The scalars are RenderInput's, unchanged: `str()` of a string is the
		// string, of a null is `None`, of a bool is `True`/`False`, and of a
		// number is its value. Named rather than left to a `default` so a new
		// kind has to decide here (kindguard).
		return RenderInput(node)
	}
	return RenderInput(node)
}

// pythonRepr is `repr(value)` — `str()` for everything except a string, which
// a container quotes. It is what a container's own `str()` uses for its
// members, which is why `[english]` reads `['english']`.
//
// **A `TaggedScalar` is the other kind whose `repr` is not its `str`.** Its
// `__str__` is the bare value (`ruamel/yaml/comments.py:1177-1178`) and its
// `__repr__` is a constructor call (`:1186-1187`), so the two differ exactly
// where a container is involved — which is why this arm lives here and
// `PythonText`'s `KindTagged` arm stays `RenderInput`. `[!!str 31]` was `[31]`
// here, indistinguishable from a plain `[31]`.
func pythonRepr(node *yamldoc.Node) string {
	if node == nil {
		return PythonText(node)
	}
	switch node.Kind {
	case yamldoc.KindString:
		return pythonStringRepr(node.Raw)
	case yamldoc.KindTagged:
		return taggedScalarRepr(node)
	case yamldoc.KindNull, yamldoc.KindBool, yamldoc.KindInt, yamldoc.KindFloat,
		yamldoc.KindMapping, yamldoc.KindSequence:
		// Every other kind reprs as it prints. Named rather than left to a
		// `default` so a new kind has to decide here (kindguard).
	}
	return PythonText(node)
}

// taggedScalarRepr is `TaggedScalar.__repr__`
// (`ruamel/yaml/comments.py:1186-1187`), one f-string:
//
//	f'TaggedScalar(value={self.value!r}, style={self.style!r}, tag={self.tag!r})'
//
// with `Tag.__repr__` supplying `Tag(<repr of trval>)`
// (`ruamel/yaml/tag.py:31-32`). All three interpolations are Python `repr`,
// which is why `[!!str "it's"]` renders `value="it's"` — the quote-selection
// rule reaches inside the constructor call too.
func taggedScalarRepr(node *yamldoc.Node) string {
	return "TaggedScalar(value=" + pythonStringRepr(node.Raw) +
		", style=" + pythonStyleRepr(node.Style) +
		", tag=Tag(" + pythonStringRepr(node.Tag) + "))"
}

// pythonStyleRepr is `repr()` of the scalar style ruamel records —
// `data2.style = node.style` (`ruamel/yaml/constructor.py:1618-1620`), the
// scanner's own style byte, which is `None` for a plain scalar and the marker
// character for every other style. Measured on all five.
func pythonStyleRepr(style yamldoc.ScalarStyle) string {
	switch style {
	case yamldoc.StylePlain:
		return "None"
	case yamldoc.StyleSingleQuoted:
		return pythonStringRepr("'")
	case yamldoc.StyleDoubleQuoted:
		return pythonStringRepr(`"`)
	case yamldoc.StyleLiteral:
		return pythonStringRepr("|")
	case yamldoc.StyleFolded:
		return pythonStringRepr(">")
	}
	return "None"
}

// pythonStringRepr is CPython's `repr(str)` (Objects/unicodeobject.c,
// `unicode_repr`):
//
//   - the quote is `'`, unless the string holds a `'` and no `"`, in which case
//     it is `"` — so `it's` reprs as `"it's"`;
//   - a backslash and the quote in force are escaped;
//   - `\t`, `\n` and `\r` have their own escapes;
//   - anything else unprintable becomes `\xhh`, `\uhhhh` or `\Uhhhhhhhh`, by
//     width, and every printable character — non-ASCII included — is written
//     as itself, because Python 3's repr does not escape those.
func pythonStringRepr(text string) string {
	quote := byte('\'')
	if strings.ContainsRune(text, '\'') && !strings.ContainsRune(text, '"') {
		quote = '"'
	}

	var out strings.Builder
	out.WriteByte(quote)
	for _, character := range text {
		switch {
		case character == rune(quote) || character == '\\':
			out.WriteByte('\\')
			out.WriteRune(character)
		case character == '\t':
			out.WriteString(`\t`)
		case character == '\n':
			out.WriteString(`\n`)
		case character == '\r':
			out.WriteString(`\r`)
		case unicode.IsPrint(character):
			out.WriteRune(character)
		default:
			out.WriteString(pythonEscape(character))
		}
	}
	out.WriteByte(quote)
	return out.String()
}

// pythonEscape is the `\xhh` / `\uhhhh` / `\Uhhhhhhhh` form of one unprintable
// character, chosen by width the way CPython's repr chooses it.
func pythonEscape(character rune) string {
	switch {
	case character < 0x100:
		return `\x` + pythonHex(character, 2)
	case character < 0x10000:
		return `\u` + pythonHex(character, 4)
	default:
		return `\U` + pythonHex(character, 8)
	}
}

func pythonHex(character rune, digits int) string {
	text := strconv.FormatInt(int64(character), 16)
	return strings.Repeat("0", digits-len(text)) + text
}

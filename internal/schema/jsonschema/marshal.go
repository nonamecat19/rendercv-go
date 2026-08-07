package jsonschema

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Marshal serializes a schema node the way upstream does:
// `json.dumps(schema, indent=2, ensure_ascii=False)` (json_schema_generator.py:44).
//
// Hand-written rather than a wrapper around `encoding/json`, because Python's
// output differs in three ways that all reach the bytes (spec 005 plan §4):
//
//   - Go escapes `<`, `>` and `&` by default and Python does not. `schema.json`
//     contains none of the three today, so an encoder that got this wrong would
//     pass the current gate and break the first time a description gains an
//     ampersand.
//   - Python leaves non-ASCII literal under `ensure_ascii=False`; the file
//     carries 1,269 such characters.
//   - There is **no trailing newline**. `write_text` writes exactly what `dumps`
//     returned, and `dumps` does not append one.
//
// An empty object or array is `{}` or `[]` on one line, which is also Python's
// behavior under `indent`.
func Marshal(value any) (string, error) {
	var out strings.Builder
	if err := marshalValue(&out, value, 0); err != nil {
		return "", err
	}
	return out.String(), nil
}

const indentUnit = "  "

func marshalValue(out *strings.Builder, value any, depth int) error {
	switch typed := value.(type) {
	case nil:
		out.WriteString("null")
	case *Object:
		return marshalObject(out, typed, depth)
	case []any:
		return marshalArray(out, typed, depth)
	case string:
		writeString(out, typed)
	case bool:
		out.WriteString(strconv.FormatBool(typed))
	case int:
		out.WriteString(strconv.Itoa(typed))
	default:
		return fmt.Errorf("jsonschema: cannot marshal %T", value)
	}
	return nil
}

func marshalObject(out *strings.Builder, object *Object, depth int) error {
	if object == nil || object.Len() == 0 {
		out.WriteString("{}")
		return nil
	}

	out.WriteString("{\n")
	inner := strings.Repeat(indentUnit, depth+1)
	for i, key := range object.Keys() {
		if i > 0 {
			out.WriteString(",\n")
		}
		out.WriteString(inner)
		writeString(out, key)
		out.WriteString(": ")

		value, _ := object.Get(key)
		if err := marshalValue(out, value, depth+1); err != nil {
			return err
		}
	}
	out.WriteString("\n" + strings.Repeat(indentUnit, depth) + "}")
	return nil
}

func marshalArray(out *strings.Builder, values []any, depth int) error {
	if len(values) == 0 {
		out.WriteString("[]")
		return nil
	}

	out.WriteString("[\n")
	inner := strings.Repeat(indentUnit, depth+1)
	for i, value := range values {
		if i > 0 {
			out.WriteString(",\n")
		}
		out.WriteString(inner)
		if err := marshalValue(out, value, depth+1); err != nil {
			return err
		}
	}
	out.WriteString("\n" + strings.Repeat(indentUnit, depth) + "]")
	return nil
}

// writeString escapes exactly what Python's `json.dumps` escapes with
// `ensure_ascii=False`: the two structural characters, the five short escapes,
// and any other control character as `\uXXXX`. Everything else — including every
// non-ASCII rune and the three characters Go would escape for HTML — is written
// through.
func writeString(out *strings.Builder, value string) {
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(out, `\u%04x`, r)
				continue
			}
			if r == utf8.RuneError {
				// Preserve the bytes rather than substituting; a schema string
				// should never contain one, and silently rewriting it would hide
				// the fact that something upstream did.
				out.WriteRune(r)
				continue
			}
			out.WriteRune(r)
		}
	}
	out.WriteByte('"')
}

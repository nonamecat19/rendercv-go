package sample

import (
	"fmt"
	"regexp"
	"strings"
)

// The starter CV's `cv.name` is the one value in the document that comes from
// the user, and it is emitted by ruamel, not by RenderCV: `dictionary_to_yaml`
// installs a `str` representer that asks for a literal block when the value has
// more than one line and leaves every other decision to the emitter
// (`schema/sample_generator.py:35-44`). The emitter then picks between plain,
// single-quoted, double-quoted and literal styles, and the choice is visible —
// `name: John Doe`, `name: 'A: B'`, `name: yes`, `name: |-`.
//
// This file is that decision, ported from the vendored
// `ruamel/yaml/emitter.py` for the one context the generator uses: the value of
// a mapping key at indent 2, block context, no flow level, no simple-key
// context, `allow_unicode`, `width = 9999`. Everything below cites the upstream
// line it mirrors.
//
// **The width is why there is no line folding here.** All four writers fold at
// `best_width`, and `dictionary_to_yaml` sets it to 9999
// (`schema/sample_generator.py:42`), so a name would have to exceed ~9990
// columns before the first fold — well past the point where the file name
// `new` derives from it (spec 013 §3.1 behavior 20) stops being a legal path.
// A longer name is emitted here without the folds upstream would insert; it is
// the one input in this file that is not parity-checked, and `nameLine`'s
// caller has no way to reach it that produces a writable file.

// nameLine renders `  name: <value>` for the starter CV's `cv` block, newline
// terminated.
//
// The terminating newline is not the scalar's: upstream's emitter writes it
// when the *next* key indents, so a style that already ends at column zero —
// a literal block with `+` chomping, say — gets no second one
// (`emitter.py:1266-1276`, `write_indent`).
func nameLine(name string) string {
	value := emitScalar([]rune(name))
	if !endsAtColumnZero(value) {
		value += "\n"
	}
	return "  name: " + value
}

// nameIndent is the column a literal block's content sits at: the `cv` mapping
// is at indent 2, and a block scalar under it indents by 2 more.
const nameIndent = 4

// emitScalar is `choose_scalar_style` (`emitter.py:853-890`) followed by the
// writer it names, for a `str` node with no explicit tag.
func emitScalar(scalar []rune) string {
	// `dictionary_to_yaml`'s representer asks for `|` when the value has more
	// than one line, and leaves the style unset otherwise
	// (`schema/sample_generator.py:35-38`).
	wantsBlock := len(splitLines(scalar)) > 1
	analysis := analyzeScalar(scalar)

	// `emitter.py:861-871`. `implicit[0]` is true when the plain form would
	// resolve back to a string; `simple_key_context` is false for a value, so
	// the guard in front of the style test is always satisfied here.
	if !wantsBlock && resolvesToString(scalar) && analysis.allowBlockPlain {
		return writePlain(scalar)
	}
	// `emitter.py:874-881`: `allow_block` is forced true before the test, so a
	// requested block style is always granted outside a flow or a key.
	if wantsBlock {
		return writeLiteral(scalar)
	}
	// `emitter.py:882-884`: an apostrophe or a newline in the value sends it to
	// the double-quoted writer before the single-quoted one is ever considered.
	if strings.ContainsAny(string(scalar), "'\n") {
		return writeDoubleQuoted(scalar)
	}
	if analysis.allowSingleQuoted {
		return writeSingleQuoted(scalar)
	}
	return writeDoubleQuoted(scalar)
}

// scalarAnalysis is the subset of `ScalarAnalysis` this context can observe.
// `allow_double_quoted` is never cleared upstream and `allow_block` is
// overwritten by `choose_scalar_style`, so neither is carried.
type scalarAnalysis struct {
	allowBlockPlain   bool
	allowSingleQuoted bool
}

// analyzeScalar is `analyze_scalar` (`emitter.py:1040-1216`) with
// `allow_unicode` on, supplementary planes allowed and the YAML version at 1.2 —
// the settings `dictionary_to_yaml` leaves at their defaults.
func analyzeScalar(scalar []rune) scalarAnalysis {
	if len(scalar) == 0 {
		// `emitter.py:1041-1052`: an empty scalar allows block plain. It never
		// reaches the plain writer anyway, because the empty string resolves to
		// null and so is not implicitly a string.
		return scalarAnalysis{allowBlockPlain: true, allowSingleQuoted: true}
	}

	var (
		blockIndicators   bool
		flowIndicators    bool
		lineBreaks        bool
		specialCharacters bool

		leadingSpace  bool
		leadingBreak  bool
		trailingSpace bool
		trailingBreak bool
		breakSpace    bool
		spaceBreak    bool
	)

	// `emitter.py:1069-1071`.
	if strings.HasPrefix(string(scalar), "---") || strings.HasPrefix(string(scalar), "...") {
		blockIndicators = true
		flowIndicators = true
	}

	precededByWhitespace := true
	followedByWhitespace := len(scalar) == 1 || isEmitterWhitespace(scalar[1])
	previousSpace, previousBreak := false, false

	for index := 0; index < len(scalar); index++ {
		ch := scalar[index]

		if index == 0 {
			// `emitter.py:1090-1104`.
			if strings.ContainsRune("#,[]{}&*!|>'\"%@`", ch) {
				flowIndicators = true
				blockIndicators = true
			}
			if ch == '?' || ch == ':' {
				// The `use_version == (1, 1)` arm of `emitter.py:1096-1097`
				// never runs: ruamel's default is 1.2.
				if len(scalar) == 1 {
					flowIndicators = true
				}
				if followedByWhitespace {
					blockIndicators = true
				}
			}
			if ch == '-' && followedByWhitespace {
				flowIndicators = true
				blockIndicators = true
			}
		} else {
			// `emitter.py:1107-1117`.
			if strings.ContainsRune(",[]{}", ch) {
				flowIndicators = true
			}
			if ch == ':' && followedByWhitespace {
				flowIndicators = true
				blockIndicators = true
			}
			if ch == '#' && precededByWhitespace {
				flowIndicators = true
				blockIndicators = true
			}
		}

		// `emitter.py:1120-1133`.
		if isEmitterBreak(ch) {
			lineBreaks = true
		}
		if ch != '\n' && (ch < 0x20 || ch > 0x7E) {
			printable := ch == 0x85 ||
				(ch >= 0xA0 && ch <= 0xD7FF) ||
				(ch >= 0xE000 && ch <= 0xFFFD) ||
				(ch >= 0x10000 && ch <= 0x10FFFF)
			if !printable || ch == 0xFEFF {
				specialCharacters = true
			}
		}

		// `emitter.py:1136-1156`.
		switch {
		case ch == ' ':
			if index == 0 {
				leadingSpace = true
			}
			if index == len(scalar)-1 {
				trailingSpace = true
			}
			if previousBreak {
				breakSpace = true
			}
			previousSpace, previousBreak = true, false
		case isEmitterBreak(ch):
			if index == 0 {
				leadingBreak = true
			}
			if index == len(scalar)-1 {
				trailingBreak = true
			}
			if previousSpace {
				spaceBreak = true
			}
			previousSpace, previousBreak = false, true
		default:
			previousSpace, previousBreak = false, false
		}

		// `emitter.py:1160-1163`. The lookahead is deliberately two characters
		// past the one just consumed; that is upstream's own arithmetic.
		precededByWhitespace = isEmitterWhitespace(ch)
		followedByWhitespace = index+2 >= len(scalar) || isEmitterWhitespace(scalar[index+2])
	}

	// `emitter.py:1166-1205`.
	allowBlockPlain := true
	allowSingleQuoted := true
	if leadingSpace || leadingBreak || trailingSpace || trailingBreak {
		allowBlockPlain = false
	}
	if breakSpace {
		allowBlockPlain = false
		allowSingleQuoted = false
	}
	if specialCharacters {
		allowBlockPlain = false
		allowSingleQuoted = false
	} else if spaceBreak {
		allowBlockPlain = false
		allowSingleQuoted = false
	}
	if lineBreaks {
		allowBlockPlain = false
	}
	if blockIndicators {
		allowBlockPlain = false
	}
	_ = flowIndicators // only `allow_flow_plain` reads it, and there is no flow here
	return scalarAnalysis{allowBlockPlain: allowBlockPlain, allowSingleQuoted: allowSingleQuoted}
}

// writePlain is `write_plain` (`emitter.py:1642-1712`) without its folding.
func writePlain(scalar []rune) string {
	return string(scalar)
}

// writeSingleQuoted is `write_single_quoted` (`emitter.py:1297-1359`): the
// apostrophe doubles and nothing else changes.
//
// The upstream writer also folds spaces and re-indents across line breaks.
// Neither is reachable: a break sends the value to the literal writer through
// the representer, and folding needs column 9999.
func writeSingleQuoted(scalar []rune) string {
	return "'" + strings.ReplaceAll(string(scalar), "'", "''") + "'"
}

// escapeReplacements is `ESCAPE_REPLACEMENTS` (`emitter.py:1361-1377`).
var escapeReplacements = map[rune]string{
	0x00: "0", 0x07: "a", 0x08: "b", 0x09: "t", 0x0A: "n", 0x0B: "v",
	0x0C: "f", 0x0D: "r", 0x1B: "e", '"': `"`, '\\': `\`,
	0x85: "N", 0xA0: "_", 0x2028: "L", 0x2029: "P",
}

// writeDoubleQuoted is `write_double_quoted` (`emitter.py:1379-1482`) without
// its folding, which `width = 9999` puts out of reach.
func writeDoubleQuoted(scalar []rune) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, ch := range scalar {
		printable := (ch >= 0x20 && ch <= 0x7E) ||
			(ch >= 0xA0 && ch <= 0xD7FF) ||
			(ch >= 0xE000 && ch <= 0xFFFD) ||
			(ch >= 0x10000 && ch <= 0x10FFFF)
		if !strings.ContainsRune("\"\\\u0085\u2028\u2029\ufeff", ch) && printable {
			out.WriteRune(ch)
			continue
		}
		switch replacement, named := escapeReplacements[ch]; {
		case named:
			out.WriteString(`\` + replacement)
		case ch <= 0xFF:
			fmt.Fprintf(&out, `\x%02X`, ch)
		case ch <= 0xFFFF:
			fmt.Fprintf(&out, `\u%04X`, ch)
		default:
			fmt.Fprintf(&out, `\U%08X`, ch)
		}
	}
	out.WriteByte('"')
	return out.String()
}

// writeLiteral is `write_literal` (`emitter.py:1589-1640`) preceded by
// `determine_block_hints` (`:1484-1520`), in block context at indent 4.
//
// The hints are the two characters after the `|`: an explicit indentation
// indicator when the first character is a space or a break — without it a
// reader cannot tell the content's indentation from the block's — and a
// chomping indicator, `-` when the text does not end in a break and `+` when it
// ends in more than one.
func writeLiteral(scalar []rune) string {
	var out strings.Builder
	out.WriteString("|" + blockHints(scalar) + "\n")

	breaks := true
	start := 0
	for end := 0; end <= len(scalar); end++ {
		ch := rune(-1)
		if end < len(scalar) {
			ch = scalar[end]
		}
		if breaks {
			if ch == -1 || !isEmitterBreak(ch) {
				for _, br := range scalar[start:end] {
					out.WriteRune(br)
				}
				if ch != -1 {
					out.WriteString(strings.Repeat(" ", nameIndent))
				}
				start = end
			}
		} else if ch == -1 || isEmitterBreak(ch) {
			out.WriteString(string(scalar[start:end]))
			if ch == -1 {
				out.WriteString("\n")
			}
			start = end
		}
		if ch != -1 {
			breaks = isEmitterBreak(ch)
		}
	}
	return out.String()
}

// blockHints is `determine_block_hints` (`emitter.py:1484-1520`) minus its
// root-context branch, which looks for a `---` or `...` the emitter would have
// to indent past. A mapping value is never in root context.
func blockHints(scalar []rune) string {
	if len(scalar) == 0 {
		return ""
	}
	hints := ""
	if scalar[0] == ' ' || isEmitterBreak(scalar[0]) {
		hints += "2"
	}
	last := scalar[len(scalar)-1]
	switch {
	case !isEmitterBreak(last):
		hints += "-"
	case len(scalar) == 1 || isEmitterBreak(scalar[len(scalar)-2]):
		hints += "+"
	}
	return hints
}

// isEmitterBreak is the emitter's line-break set — narrower than Python's
// `str.splitlines`, which is what the representer uses. A `\r` therefore counts
// as a second line for the style choice and as ordinary text for the writer,
// so `"a\rb"` comes out as a one-line literal block.
func isEmitterBreak(ch rune) bool {
	return ch == '\n' || ch == 0x85 || ch == 0x2028 || ch == 0x2029
}

// isEmitterWhitespace is `'\0 \t\r\n\x85  '` (`emitter.py:1077`).
func isEmitterWhitespace(ch rune) bool {
	return ch == 0 || ch == ' ' || ch == '\t' || ch == '\r' || isEmitterBreak(ch)
}

// endsAtColumnZero reports whether the emitted value left the cursor at the
// start of a line, in which case the next key needs no line break of its own.
func endsAtColumnZero(value string) bool {
	runes := []rune(value)
	return len(runes) > 0 && isEmitterBreak(runes[len(runes)-1])
}

// splitLines is Python's `str.splitlines`, which the representer calls to
// decide whether a value is multi-line (`schema/sample_generator.py:36`). Its
// boundary set is wider than YAML's: it also breaks on `\r`, `\v`, `\f` and the
// three C1 file separators.
func splitLines(scalar []rune) []string {
	isBoundary := func(ch rune) bool {
		switch ch {
		case '\n', '\r', 0x0B, 0x0C, 0x1C, 0x1D, 0x1E, 0x85, 0x2028, 0x2029:
			return true
		}
		return false
	}
	var lines []string
	start := 0
	for i := 0; i < len(scalar); i++ {
		if !isBoundary(scalar[i]) {
			continue
		}
		lines = append(lines, string(scalar[start:i]))
		if scalar[i] == '\r' && i+1 < len(scalar) && scalar[i+1] == '\n' {
			i++
		}
		start = i + 1
	}
	if start < len(scalar) {
		lines = append(lines, string(scalar[start:]))
	}
	return lines
}

// resolvers are ruamel's implicit resolvers for YAML 1.2, in registration
// order, from `ruamel/yaml/resolver.py:24-93`. A plain scalar matching one of
// them would load back as something other than a string, so the emitter may not
// write it plain.
//
// **The `first` column is not decoration.** ruamel indexes its resolvers by the
// scalar's first character and tries only that bucket, so `_` — which the int
// pattern matches — is never offered to the int resolver and stays plain.
var resolvers = []struct {
	first   string
	pattern *regexp.Regexp
}{
	{"tTfF", regexp.MustCompile(`^(?:true|True|TRUE|false|False|FALSE)$`)},
	{"-+0123456789.", regexp.MustCompile(
		`^(?:` +
			`[-+]?(?:[0-9][0-9_]*)\.[0-9_]*(?:[eE][-+]?[0-9]+)?` +
			`|[-+]?(?:[0-9][0-9_]*)(?:[eE][-+]?[0-9]+)` +
			`|[-+]?\.[0-9_]+(?:[eE][-+][0-9]+)?` +
			`|[-+]?\.(?:inf|Inf|INF)` +
			`|\.(?:nan|NaN|NAN))$`)},
	{"-+0123456789", regexp.MustCompile(
		`^(?:[-+]?0b[0-1_]+` +
			`|[-+]?0o?[0-7_]+` +
			`|[-+]?[0-9_]+` +
			`|[-+]?0x[0-9a-fA-F_]+)$`)},
	{"<", regexp.MustCompile(`^(?:<<)$`)},
	{"~nN\x00", regexp.MustCompile(`^(?:~|null|Null|NULL|)$`)},
	{"0123456789", regexp.MustCompile(
		`^(?:[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]` +
			`|[0-9][0-9][0-9][0-9]-[0-9][0-9]?-[0-9][0-9]?` +
			`(?:[Tt]|[ \t]+)[0-9][0-9]?` +
			`:[0-9][0-9]:[0-9][0-9](?:\.[0-9]*)?` +
			`(?:[ \t]*(?:Z|[-+][0-9][0-9]?(?::[0-9][0-9])?))?)$`)},
	{"=", regexp.MustCompile(`^(?:=)$`)},
	{"!&*", regexp.MustCompile(`^(?:!|&|\*)$`)},
}

// resolvesToString reports whether a plain scalar would load back as a string —
// `BaseResolver.resolve`'s answer, and `implicit[0]` in the emitter's style
// choice (`emitter.py:862`).
//
// The empty string gets its own bucket upstream: the null resolver's `first`
// list ends with an empty entry (`resolver.py:71-77`), which is what makes a
// name of `""` come out quoted. The NUL in the null bucket above stands in for
// that entry, since an empty scalar has no first character to look up.
func resolvesToString(scalar []rune) bool {
	var first rune = 0
	if len(scalar) > 0 {
		first = scalar[0]
	}
	text := string(scalar)
	for _, resolver := range resolvers {
		if strings.ContainsRune(resolver.first, first) && resolver.pattern.MatchString(text) {
			return false
		}
	}
	return true
}

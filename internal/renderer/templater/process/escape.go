package process

import (
	"fmt"
	"regexp"
	"strings"
)

// The two patterns that hold Typst source out of the escape
// (markdown_parser.py:74-75).
//
// `typstCommandPattern` is a `#` followed by a letter, then anything that is not
// whitespace, `(` or `[`, then optional `(...)` and `[...]` groups. It is
// deliberately loose: it exists to protect what the port itself emitted — the
// footer's `#str(here().page())` (spec 008 §4D behavior 43) and the connection
// list's `#link(...)` — not to validate Typst.
var (
	typstCommandPattern = regexp.MustCompile(`#([A-Za-z][^\s()\[]*)(\([^)]*\))?(\[[^\]]*\])?`)
	mathPattern         = regexp.MustCompile(`\$\$(?s:.*?)\$\$`)
)

// singleCharacterEscapes is the thirteen characters `str.translate` handles
// (markdown_parser.py:111-124), in upstream's declaration order — which does not
// matter to a translate table and does here, because a reviewer diffs the two
// lists.
var singleCharacterEscapes = []struct{ from, to string }{
	{"[", `\[`},
	{"]", `\]`},
	{`\`, `\\`},
	{`"`, `\"`},
	{"#", `\#`},
	{"$", `\$`},
	{"@", `\@`},
	{"%", `\%`},
	{"~", `\~`},
	{"_", `\_`},
	{"/", `\/`},
	{">", `\>`},
	{"<", `\<`},
}

// EscapeTypstCharacters is `escape_typst_characters`
// (markdown_parser.py:78-142), and the three phases are the behavior.
//
// The order is what makes it work, and each phase depends on the one before:
//
//  1. every math span and Typst command is replaced by a dummy name, so nothing
//     inside them is escaped. The saved text has `$$` collapsed to `$`, which is
//     why `$$x$$` in the source comes out as `$x$`.
//  2. thirteen single characters are escaped through what Python does as one
//     `str.translate` — **simultaneous**, so the `\` rule cannot re-escape a
//     backslash the `[` rule just wrote.
//  3. two **longer** replacements run afterwards, because `translate` is
//     single-character only. They see a string in which `#` is already `\#`, so
//     the `#sym.ast.basic` they insert is written *after* escaping and survives.
//
// Then the dummy names come back.
//
// **A lone newline returns immediately** (`:91-92`), before any of it.
func EscapeTypstCharacters(text string) string {
	if text == "\n" {
		return text
	}

	// Phase 1. **Both patterns scan the *original* string.**
	//
	// `itertools.chain(math_pattern.finditer(string), typst_command_pattern
	// .finditer(string))` (`:97-102`) binds both iterators before the loop
	// mutates `string`, so the command pattern never sees a dummy name. Rescanning
	// the mutated text instead matches `#emph[RENDERCVTYPSTCOMMANDORMATH0]` as one
	// command and leaves dummy 0 unresolved — the literal name reaches the
	// output. Measured on `#emph[$$x$$]`, which upstream escapes entirely.
	original := text
	matches := append(
		mathPattern.FindAllString(original, -1),
		typstCommandPattern.FindAllString(original, -1)...,
	)

	saved := map[string]string{}
	var order []string
	for index, match := range matches {
		name := fmt.Sprintf("RENDERCVTYPSTCOMMANDORMATH%d", index)
		// `str.replace` with no count: **every** occurrence. A repeated command
		// is saved once and restored everywhere — and a later match whose text
		// an earlier replacement already removed simply finds nothing, which is
		// upstream's behavior too.
		text = strings.ReplaceAll(text, match, name)
		saved[name] = strings.ReplaceAll(match, "$$", "$")
		order = append(order, name)
	}

	// Phase 2, simultaneous. `strings.NewReplacer` is Python's `translate`: it
	// scans once and never reconsiders what it wrote.
	pairs := make([]string, 0, len(singleCharacterEscapes)*2)
	for _, escape := range singleCharacterEscapes {
		pairs = append(pairs, escape.from, escape.to)
	}
	text = strings.NewReplacer(pairs...).Replace(text)

	// Phase 3, sequential and after phase 2. `"* "` is tried before `"*"`, which
	// is Python dict order here and is load-bearing: the bare-`*` rule would
	// otherwise consume the one with a trailing space.
	text = strings.ReplaceAll(text, "* ", "#sym.ast.basic ")
	text = strings.ReplaceAll(text, "*", "#sym.ast.basic#h(0pt, weak: true) ")

	for _, name := range order {
		text = strings.ReplaceAll(text, name, saved[name])
	}
	return text
}

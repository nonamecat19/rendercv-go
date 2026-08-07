package process

import (
	"regexp"
	"strings"
)

// horizontalRulePattern is python-markdown's `hr` block processor, which is
// **still registered** — only five were deregistered (spec 008 §4C behavior 35)
// and this is not one of them.
//
// A rule produces an `<hr/>` element, and `to_typst_string`'s default branch
// recurses into a node with no text and no children, so the line renders as the
// **empty string**. Measured on `---`, `***`, `___`, `* * *` and `- - -`; `--`
// and `**` are too short and stay literal.
//
// Go's RE2 has no backreference, so the "same character three times" rule is
// checked in isHorizontalRule rather than in the pattern.
var horizontalRulePattern = regexp.MustCompile(`^ {0,3}[-_*][-_* ]*$`)

// isHorizontalRule is the rest of the rule: at least three of the **same**
// marker, and nothing else but spaces.
func isHorizontalRule(line string) bool {
	if !horizontalRulePattern.MatchString(line) {
		return false
	}
	marker := byte(0)
	count := 0
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case ' ':
		case '-', '_', '*':
			if marker == 0 {
				marker = line[i]
			}
			if line[i] != marker {
				return false
			}
			count++
		default:
			return false
		}
	}
	return count >= 3
}

// MarkdownToTypst is `markdown_to_typst` (markdown_parser.py:158-190).
//
// **Line by line**, and that is not an optimization. Single-newline-separated
// lines are one paragraph to Markdown, so an unmatched `*` on one line would
// pair with one on the next; converting the whole string at once emits emphasis
// across a line boundary. Measured: `*a\nb*` stays two literal asterisks.
//
// The one exception is an admonition block — a line starting with `!!!` plus
// every following line indented by four spaces — which is converted as a unit
// because it is multi-line by design and is what `process_summary` produces.
func MarkdownToTypst(text string) string {
	lines := strings.Split(text, "\n")
	parts := make([]string, 0, len(lines))

	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "!!!") {
			parts = append(parts, convertLine(lines[i]))
			continue
		}

		block := []string{lines[i]}
		i++
		for i < len(lines) && strings.HasPrefix(lines[i], "    ") {
			block = append(block, lines[i])
			i++
		}
		i--
		parts = append(parts, strings.TrimSpace(convertAdmonition(block)))
	}
	return strings.Join(parts, "\n")
}

// convertLine is one `md.convert` call, and the strip at the end is **Python's,
// not a tidy-up**: `Markdown.convert` returns `output.strip()`. It is why a
// four-space-indented line loses its indent and why a trailing
// `#sym.ast.basic#h(0pt, weak: true) ` loses its space — both measured.
func convertLine(line string) string {
	// **`indent` is still registered.** Only five block processors are
	// deregistered (spec 008 §4C behavior 35) and the indented-code one is not
	// among them, so a line starting with four spaces is a code block — and
	// `to_typst_string`'s `code` branch emits its raw text, newline included.
	//
	// It reaches a real document through a highlight that happens to start
	// indented; `process_summary`'s four-space indent does not, because its
	// lines are inside an admonition block that is converted as a unit.
	if isHorizontalRule(line) {
		return ""
	}

	// A tab counts as the same indent.
	for _, indent := range []string{"    ", "\t"} {
		if body, indented := strings.CutPrefix(line, indent); indented {
			return "`" + body + "\n`"
		}
	}
	return strings.TrimSpace(ParseInline(line))
}

// convertAdmonition is the `div` branch of `to_typst_string` (`:52-57`):
// `#summary[…]` with the content stripped of leading and trailing newlines and
// its inner newlines replaced by `" \ "`.
//
// **The admonition's title line is dropped.** `to_typst_string` skips any child
// whose class is `admonition-title` (`:60-62`), so `!!! summary` and `!!! note`
// produce the same wrapper — the keyword selects nothing.
func convertAdmonition(block []string) string {
	body := make([]string, 0, len(block))
	for _, line := range block[1:] {
		body = append(body, ParseInline(strings.TrimPrefix(line, "    ")))
	}

	content := strings.Trim(strings.Join(body, "\n"), "\n")
	return "#summary[" + strings.ReplaceAll(content, "\n", ` \ `) + "]"
}

package process

import (
	"regexp"
	"strings"
	"unicode"
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
	// **The split is over the raw string** (`markdown_parser.py:175`), before any
	// whitespace normalization — `NormalizeWhitespace` is a preprocessor
	// (`preprocessors.py:66-73`) and so runs *inside* each `md.convert`, over one
	// line at a time. Normalizing first and splitting after is not the same
	// function: it turns a trailing `\r` into a top-level line boundary where
	// upstream keeps it inside the `convert` whose `output.strip()` then removes
	// it, so `"    a  \r"` gained a newline it does not have upstream.
	lines := strings.Split(text, "\n")
	parts := make([]string, 0, len(lines))

	for i := 0; i < len(lines); {
		if !strings.HasPrefix(lines[i], "!!!") {
			parts = append(parts, convertChunk(lines[i]))
			i++
			continue
		}

		block, next := admonitionBlock(lines, i)
		parts = append(parts, convertChunk(strings.Join(block, "\n")))
		i = next
	}
	return strings.Join(parts, "\n")
}

// admonitionBlock is the `!!!` line at lines[i] plus every following line
// indented by a full `tab_length` (`markdown_parser.py:178-185`), and the index
// the caller resumes at.
func admonitionBlock(lines []string, i int) ([]string, int) {
	block := []string{lines[i]}
	i++
	for i < len(lines) && strings.HasPrefix(lines[i], indentWidth) {
		block = append(block, lines[i])
		i++
	}
	return block, i
}

// convertChunk is one `md.convert` call: the preprocessors, the parse, and the
// `output.strip()` `Markdown.convert` ends with (`markdown/core.py:329-360`).
//
// The chunk is one raw line, or the raw lines of an admonition block. It is
// **normalized here**, which is the only place a chunk can grow lines: a `\r`
// inside a raw line makes `md.convert` see a multi-line document. What upstream
// then runs over it is the block parser; what runs here is still the line-wise
// conversion, so the newline a `\r` introduces is only reproduced where the two
// agree — a line boundary, a code block of its own, a paragraph of its own. A
// `\r` in the *middle* of a line is still open, and is a different defect: there
// upstream's block parser joins the halves into one block, so `"    a\r    b"`
// is one two-line code block carrying "a\nb\n", and `"*a\rb*"` is one `#emph`
// spanning the break. Reproducing that needs the block parser, not this loop.
//
// The trailing strip is what makes the trailing case exact: `"    a\r"` is one
// code block and then an empty line here as it is upstream, and the empty line
// is stripped off the `convert` output in both.
//
// The admonition grouping runs **again** over the normalized lines, because
// `AdmonitionProcessor` is a block processor and so sees the document after the
// preprocessors: a `\r` in front of a `!!!` opens an admonition upstream, where
// only the top-level grouping — which reads the raw lines — would leave the
// marker literal. `"a\r!!! note"` is `"a\n#summary[]"`, measured.
func convertChunk(chunk string) string {
	lines := strings.Split(normalizeWhitespace(chunk), "\n")
	parts := make([]string, 0, len(lines))

	for i := 0; i < len(lines); {
		if !strings.HasPrefix(lines[i], "!!!") {
			parts = append(parts, convertLine(lines[i]))
			i++
			continue
		}

		block, next := admonitionBlock(lines, i)
		parts = append(parts, convertAdmonition(block))
		i = next
	}
	return trimPythonSpace(strings.Join(parts, "\n"))
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
	//
	// The block's text is HTML-escaped on the way in —
	// `util.code_escape(block.rstrip())` (`markdown/blockprocessors.py:269`
	// and `:276`) — the second of `code_escape`'s two call sites, the other
	// being the inline code span. `to_typst_string`'s `code` branch emits the
	// text verbatim (`markdown_parser.py:42-45`), so an indented `a & b`
	// has to become `` `a &amp; b\n` `` here or nowhere.
	if isHorizontalRule(line) {
		return ""
	}

	// **A blank line is a blank line, however it is spelled.** The block parser
	// splits on blank lines before `indent` ever runs, so a line of nothing but
	// whitespace produces no element at all — `markdown_to_typst("    ")` and
	// `markdown_to_typst("  \t")` are both `""` upstream. Without this the
	// indent branch below reads an empty body and emits an empty code block,
	// which `NormalizeWhitespace` then made reachable from `" \t"` by expanding
	// the tab to the fourth column.
	//
	// The mechanism is `Markdown.convert`'s own first statement — `if not
	// source.strip(): return ''` (`markdown/core.py:279-282`) — which is why the
	// predicate has to be Python's whitespace and not Go's.
	if trimPythonSpace(line) == "" {
		return ""
	}

	// A tab counts as the same indent.
	for _, indent := range []string{"    ", "\t"} {
		if body, indented := strings.CutPrefix(line, indent); indented {
			// **`rstrip` before `code_escape`, not after.** The block processor
			// escapes `block.rstrip()` (`blockprocessors.py:269` and `:276`), so
			// the trailing whitespace of the last line of a code block is gone
			// before any `&` becomes `&amp;` — an indented highlight ending in a
			// space carried it into the `.typ` here.
			//
			// It is the **block** that is stripped, not each line: upstream keeps
			// `x  ` in `    x  \n    y  `. Nothing is needed for that here,
			// because this path converts a line at a time and every code block is
			// therefore one line long.
			return "`" + codeEscape(trimPythonSpaceRight(body)) + "\n`"
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
//
// # The inline pass runs over the whole paragraph, not over each line
//
// `markdown_to_typst` hands the block to one `md.convert("\n".join(block))`
// (`markdown_parser.py:186`) precisely because an admonition "spans multiple
// lines by design" (`:164-167`). Parsing each line on its own instead broke
// every construct that crosses a line boundary — `**bold\nspanning**`, the shape
// `process_summary` produces whenever a bold run wraps, came out as two stray
// `#sym.ast.basic` pairs.
//
// What the block parser does before the inline pass is reproduced here in three
// steps, because each is observable:
//
//  1. `NormalizeWhitespace`'s `re.sub(r'(?<=\n) +\n', '\n', source)`
//     (`markdown/preprocessors.py:74`) empties a whitespace-only line;
//  2. `AdmonitionProcessor` detabs by `tab_length` (`blockprocessors.py:85-98`);
//  3. `BlockParser.parseChunk` splits on a blank line and `ParagraphProcessor`
//     lstrips each block (`:612-640`), and `PrettifyTreeprocessor` puts a single
//     `"\n"` between the resulting paragraphs.
func convertAdmonition(block []string) string {
	body := make([]string, 0, len(block))
	for _, line := range block[1:] {
		if strings.TrimSpace(line) == "" {
			body = append(body, "")
			continue
		}
		body = append(body, strings.TrimPrefix(line, indentWidth))
	}

	paragraphs := make([]string, 0, len(body))
	for _, lines := range splitOnBlank(body) {
		text := lineBreakPattern.ReplaceAllString(strings.Join(lines, "\n"), "\n")
		paragraphs = append(paragraphs, ParseInline(strings.TrimLeft(text, " \t\n\r\f\v")))
	}

	content := strings.Trim(strings.Join(paragraphs, "\n"), "\n")
	return "#summary[" + strings.ReplaceAll(content, "\n", ` \ `) + "]"
}

// indentWidth is python-markdown's `tab_length`, four spaces.
const indentWidth = "    "

// isPythonSpace is `str.isspace()`: the 29 characters CPython's
// `Py_UNICODE_ISSPACE` accepts, which is what every bare `str.strip()` and
// `str.rstrip()` in python-markdown trims.
//
// **Go's set is not Python's.** `unicode.IsSpace` is the 25-rune `White_Space`
// property; Python adds the four C0 separators U+001C–U+001F, so
// `strings.TrimSpace` leaves a trailing file separator where `rstrip` takes it.
// Measured on `markdown_to_typst`: a trailing file separator is gone from the
// code block `"    a\x1c"` produces, and `"  \x1c"` converts to the empty
// string — a whitespace-only line either way.
//
// The same rule the colour parser transcribes for `\s`
// (`internal/schema/models/design/color.go:49`), reached from the other side.
func isPythonSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// trimPythonSpace is Python's `str.strip()` with no argument.
func trimPythonSpace(s string) string {
	return strings.TrimFunc(s, isPythonSpace)
}

// trimPythonSpaceRight is Python's `str.rstrip()` with no argument.
func trimPythonSpaceRight(s string) string {
	return strings.TrimRightFunc(s, isPythonSpace)
}

// lineBreakPattern is `LINE_BREAK_RE` (`markdown/inlinepatterns.py:173`), two
// spaces at the end of a line.
//
// It builds a `br`, which `to_typst_string` renders as nothing, and
// `PrettifyTreeprocessor` then puts a `"\n"` back in front of the element's tail
// (`treeprocessors.py:437-441`). The net effect on the Typst side is that the
// two spaces vanish and the newline stays, which is what dropping them here
// reproduces.
var lineBreakPattern = regexp.MustCompile(`  \n`)

// splitOnBlank groups lines into paragraphs the way `BlockParser.parseChunk`'s
// `text.split('\n\n')` does, dropping the empty blocks `ParagraphProcessor`
// throws away (`blockprocessors.py:612-614`).
func splitOnBlank(lines []string) [][]string {
	var groups [][]string
	var current []string
	for _, line := range lines {
		if line == "" {
			if len(current) > 0 {
				groups = append(groups, current)
				current = nil
			}
			continue
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}
	return groups
}

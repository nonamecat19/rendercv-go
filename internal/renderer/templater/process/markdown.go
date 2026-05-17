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

// admonitionPattern is `AdmonitionProcessor.RE`
// (`markdown/extensions/admonition.py:53`) matched against one line, its
// `(?:^|\n)` and `(?:\n|$)` bounds being the line's own ends.
//
// **The keyword is required and the line may hold nothing else**: `!!! note` and
// `!!! note "T"` open an admonition, `!!!` alone and `!!! note: x` do not — the
// trailing `*(?:\n|$)` leaves no room for the `: x`. The class and the title
// select nothing on this path (see `convertAdmonition`), so neither group is
// captured here.
var admonitionPattern = regexp.MustCompile(`^!!! ?[\w\-]+( +[\w\-]+)*( +"[^"]*")? *$`)

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
	// (`preprocessors.py:66-73`) and so runs *inside* each `md.convert`.
	// Normalizing first and splitting after is not the same function: it turns a
	// `\r` into a top-level line boundary upstream does not have. A trailing one
	// gained a newline `output.strip()` removes upstream — `"    a  \r"` — and a
	// `\r` in the *middle* of a line took every construct spanning it with it:
	//
	//	a `\rx` span  ->  "a `\nx` span"  upstream "a `x` span"
	//	[t `a\rb`](u) ->  "\\[t `a\nb`\\](u)"  upstream "#link(\"u\")[t `a\nb`]"
	//
	// A `\r` is what a CV field pasted from a Windows editor carries, and a
	// chunk that keeps one is a **multi-line** `md.convert` — see `convertChunk`.
	lines := strings.Split(text, "\n")
	parts := make([]string, 0, len(lines))

	for i := 0; i < len(lines); {
		if !strings.HasPrefix(lines[i], "!!!") {
			parts = append(parts, convertChunk(lines[i]))
			i++
			continue
		}

		// **The group goes to `convertAdmonition`, not through `convertChunk`.**
		// `md.convert("\n".join(block))` (`markdown_parser.py:186`) is one
		// convert, and its top-level `parseChunk` splits on a blank line
		// *before* `AdmonitionProcessor` runs — so a second indented block after
		// a blank line is a separate top-level block that `parse_content`
		// re-attaches to the same `div` as a sibling
		// (`extensions/admonition.py:70-120`). Feeding the group to
		// `convertChunk` would split it and lose that: `!!! note\r    a\r\r    b`
		// is `#summary[a \ b]` upstream, one admonition holding two paragraphs.
		block, next := admonitionBlock(lines, i)
		parts = append(parts, trimPythonSpace(strings.Join(
			convertAdmonition(normalizeChunk(strings.Join(block, "\n"))), "\n")))
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

// convertChunk is one `md.convert` call, and the strip at the end is **Python's,
// not a tidy-up**: `Markdown.convert` returns `output.strip()`. It is why a
// four-space-indented line loses its indent and why a trailing
// `#sym.ast.basic#h(0pt, weak: true) ` loses its space — both measured.
//
// A chunk is one raw line of the input, so it is usually one block and one
// element. It is only usually: `NormalizeWhitespace` runs here (see
// `MarkdownToTypst`) and folds a `\r` into a newline, so a chunk can arrive at
// the block parser as several lines and produce several elements, which
// `PrettifyTreeprocessor` and the serializer then join with `"\n"`.
func convertChunk(chunk string) string {
	// `Markdown.convert`'s own first statement — `if not source.strip(): return
	// ''` (`markdown/core.py:279-282`) — which is why the predicate has to be
	// Python's whitespace and not Go's.
	if trimPythonSpace(chunk) == "" {
		return ""
	}

	parts := make([]string, 0, 2)
	for _, block := range splitOnBlank(normalizeChunk(chunk)) {
		parts = append(parts, convertBlock(block)...)
	}
	return trimPythonSpace(strings.Join(parts, "\n"))
}

// normalizeChunk is `NormalizeWhitespace` (`preprocessors.py:66-73`) run over
// one chunk, returning its lines with every whitespace-only one emptied so that
// `splitOnBlank` can read them as block separators.
//
// The emptying is the preprocessor's last step, `re.sub(r'(?<=\n) +\n', '\n',
// source)` (`:74`), and both of its edges are load-bearing. The lookbehind
// spares the **first** line — a chunk opening with a space is not a blank line —
// and the character class is literal spaces, so a line of `\x0b` is not emptied
// and does not separate blocks. RE2 has no lookbehind, so the rule is applied
// per line instead; upstream appends `"\n\n"` before the substitution, which is
// what makes a trailing space-only line match its own trailing newline.
func normalizeChunk(chunk string) []string {
	// `HtmlBlockPreprocessor` is registered at 20 and `NormalizeWhitespace` at 30
	// (`preprocessors.py:40-41`), so the raw-HTML pass sees the normalized text —
	// which is what makes `<div>a</div>\t t` an indented code block, the tab
	// having become the fourth column before the tail is a line of its own.
	lines := strings.Split(splitRawBlockTails(
		normalizeCharRefs(normalizeWhitespace(chunk)), "\n\n"), "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] != "" && strings.Trim(lines[i], " ") == "" {
			lines[i] = ""
		}
	}
	return lines
}

// convertBlock is `BlockParser.parseBlocks` over one block, in the registry's
// priority order (`blockprocessors.py:60-77`) with the five processors spec 008
// §4C behavior 35 names deregistered. It returns one string per element,
// because both of the processors below can turn one block into several.
func convertBlock(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}

	// `AdmonitionProcessor` is registered at 105, above every core processor
	// (`extensions/admonition.py:47`), and it **searches** the block: its `RE`
	// starts `(?:^|\n)`, so a `!!!` line anywhere in the block opens an
	// admonition and the lines before it are re-parsed as their own block
	// (`:131-136`). A chunk reaches this only through a `\r`, since
	// `MarkdownToTypst` already groups a `!!!` line that begins a raw one.
	for i, line := range lines {
		if !admonitionPattern.MatchString(line) {
			continue
		}
		return append(convertBlock(lines[:i]), convertAdmonition(lines[i:])...)
	}

	// **`indent` is still registered.** Only five block processors are
	// deregistered and the indented-code one is not among them, so a block
	// starting with four spaces is a code block — and `to_typst_string`'s `code`
	// branch emits its raw text, newline included.
	//
	// It reaches a real document through a highlight that happens to start
	// indented; `process_summary`'s four-space indent does not, because its
	// lines are inside an admonition block that is converted as a unit.
	//
	// Tabs cannot appear: `normalizeWhitespace` expanded them.
	if strings.HasPrefix(lines[0], indentWidth) {
		body, rest := detab(lines)
		// **`rstrip` before `code_escape`, not after.** The block processor
		// escapes `block.rstrip()` (`blockprocessors.py:269` and `:276`) — the
		// second of `code_escape`'s two call sites, the other being the inline
		// code span — so the trailing whitespace of the last line of a code
		// block is gone before any `&` becomes `&amp;`. It is the **block** that
		// is stripped, not each line: upstream keeps `x  ` in `    x  \n    y  `.
		//
		// `to_typst_string`'s `code` branch emits the text verbatim
		// (`markdown_parser.py:42-45`), so an indented `a & b` has to become
		// `` `a &amp; b\n` `` here or nowhere.
		block := "`" + codeEscape(trimPythonSpaceRight(strings.Join(body, "\n"))) + "\n`"
		return append([]string{block}, convertBlock(rest)...)
	}

	// `HRProcessor` **searches** the block rather than testing its first line
	// (`blockprocessors.py:517-547`): the lines before the rule are re-parsed as
	// a block of their own and the lines after it go back on the queue.
	//
	// The rule itself is an `hr` element, which takes `to_typst_string`'s default
	// branch and recurses into a node with no text and no children — **the empty
	// string, not nothing.** It is still an element, so the serializer puts a
	// newline on each side of it: `a\r---\rb` is `"a\n\nb"` upstream, a blank
	// line between the two paragraphs, where dropping the element gives `"a\nb"`.
	for i, line := range lines {
		if !isHorizontalRule(line) {
			continue
		}
		out := append(convertBlock(lines[:i]), "")
		return append(out, convertBlock(lines[i+1:])...)
	}

	// `ParagraphProcessor` (`:612-640`) lstrips the block — the whole block, not
	// each line, which is why `  a\r  b` keeps the second line's two spaces.
	text := lineBreakPattern.ReplaceAllString(strings.Join(lines, "\n"), "\n")
	return []string{ParseInline(strings.TrimLeft(text, " \t\n\r\f\v"))}
}

// detab is `BlockProcessor.detab` (`blockprocessors.py:100-130`): it removes one
// indent from each line, keeps a blank line as blank, and stops at the first
// line carrying neither — returning that line and everything after it as the
// remainder, which upstream puts back at the head of the block queue.
func detab(lines []string) (body, rest []string) {
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, indentWidth):
			body = append(body, line[len(indentWidth):])
		case trimPythonSpace(line) == "":
			body = append(body, "")
		default:
			return body, lines[i:]
		}
	}
	return body, nil
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
//
// It returns one string per element, because the detab in step 2 has a
// remainder: an unindented line **after** an indented one ends the admonition
// and goes back on the block queue (`extensions/admonition.py:163-167`), where
// it becomes a sibling of the `div` rather than a child of it. Reachable only
// through a `\r`, since `MarkdownToTypst`'s grouping stops at the same line.
func convertAdmonition(block []string) []string {
	body, rest := detab(block[1:])

	paragraphs := make([]string, 0, len(body))
	for _, lines := range splitOnBlank(body) {
		paragraphs = append(paragraphs, convertBlock(lines)...)
	}

	content := strings.Trim(strings.Join(paragraphs, "\n"), "\n")
	summary := "#summary[" + strings.ReplaceAll(content, "\n", ` \ `) + "]"
	return append([]string{summary}, convertBlock(rest)...)
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

package process

import (
	"regexp"
	"strings"
)

// inlineParser is python-markdown's inline pass, emitting Typst directly rather
// than building an element tree.
//
// **Skipping the tree is safe here and only here.** `to_typst_string`
// (markdown_parser.py:9-70) is a total function from the tree to a string with
// no lookahead and no reordering — every node's output depends on itself and its
// children — so composing it into the parse loses nothing. The HTML path, which
// iteration 11 owns, does need a tree and keeps goldmark.
type inlineParser struct{}

// The three patterns that run **before** emphasis, in upstream's registry order
// (markdown/inlinepatterns.py:73-95). Only these three reach the Typst path:
// references, images, autolinks and inline HTML are either unused by RenderCV's
// content or produce tags `to_typst_string` drops.
var (
	// BACKTICK_RE, reduced to the form RenderCV's content reaches: a run of
	// backticks, a body, the same run. **The body is not escaped** —
	// `to_typst_string`'s `code` branch uses the child's raw text (`:38-41`).
	backtickPattern = regexp.MustCompile("(?s)^(`+)(.+?)" + "`")

	// ESCAPE_RE `\\(.)`.
	escapePattern = regexp.MustCompile(`(?s)^\\(.)`)

	// LINK_RE, reduced to `[text](url)` plus python-markdown's optional title.
	//
	// **The title is stripped, and an earlier comment here said it never
	// appeared.** It does: `[t](u "ti")` is ordinary Markdown a user writes in a
	// summary, upstream renders `#link("u")[t]`, and passing the title through
	// emitted `#link("u "ti"")[t]` — an unbalanced Typst string literal, so the
	// document **did not compile at all**. An audit measured that; the corpus has
	// no link title, which is why it shipped.
	linkPattern = regexp.MustCompile(`(?s)^\[([^\]]*)\]\(([^)]*)\)`)

	// linkTitlePattern is the trailing `"title"` python-markdown splits off.
	linkTitlePattern = regexp.MustCompile(`(?s)^(.*?)\s+["'][^"']*["']\s*$`)

	// NOT_STRONG_RE — a lone `*` or `_` surrounded by whitespace is literal
	// text, which is why `a * b` survives unchanged.
	notStrongPattern = regexp.MustCompile(`^((^|\s)(\*|_)(\s|$))`)
)

// ParseInline converts one line of Markdown to Typst.
//
// It is the composition of python-markdown's inline pass and
// `to_typst_string`, so text that matches no pattern goes through
// EscapeTypstCharacters — which is where `#`, `%` and the rest are handled and
// where `*` becomes `#sym.ast.basic`.
func ParseInline(line string) string {
	var parser inlineParser
	return parser.parseFrom(line, -1, 0)
}

// parseFrom is `parse_sub_patterns` (markdown/inlinepatterns.py:589-646).
//
// `from` is the index of the pattern that produced the parent; only patterns
// **after** it are tried, which is what stops `***x***` matching itself forever.
//
// **The cutoff is per processor**, which is why `fromDelim` travels with it.
// Upstream has two processors — `AsteriskProcessor` and `UnderscoreProcessor` —
// and `parse_sub_patterns`' `idx` indexes the one that matched. A `*` inside an
// `_` emphasis is a fresh processor and starts from its first pattern, so
// `_**a**_` is a strong inside an emph. Sharing one cutoff across both makes it
// four literal asterisks instead.
//
// Text between matches is escaped and emitted as it goes.
func (p *inlineParser) parseFrom(data string, from int, fromDelim byte) string {
	var out strings.Builder
	pending := 0
	pos := 0

	flush := func(upto int) {
		if upto > pending {
			out.WriteString(EscapeTypstCharacters(data[pending:upto]))
		}
	}

	for pos < len(data) {
		// **The pre-emphasis patterns run at every depth**, not only at the top.
		// They are separate processors upstream, registered at higher priority,
		// and the tree processor visits the sub-content of an emphasis just as
		// it visits the root — which is why `**[a](u)**` is a link inside a
		// strong and not an escaped bracket. The `from` cutoff applies to the
		// emphasis patterns alone.
		if end, typst, ok := p.matchPrefix(data, pos); ok {
			flush(pos)
			out.WriteString(typst)
			pos, pending = end, end
			continue
		}

		if data[pos] != '*' && data[pos] != '_' {
			pos++
			continue
		}
		if notStrongPattern.MatchString(data[pos:]) && p.isolatedDelimiter(data, pos) {
			pos++
			continue
		}

		patterns := asteriskPatterns
		if data[pos] == '_' {
			patterns = underscorePatterns
		}

		// Only the processor that produced the parent is cut off.
		cutoff := -1
		if data[pos] == fromDelim {
			cutoff = from
		}

		matched := false
		for index, pattern := range patterns {
			if index <= cutoff {
				continue
			}
			end, first, second, ok := pattern.match(data, pos)
			if !ok {
				continue
			}
			flush(pos)
			out.WriteString(pattern.build(p, first, second, index, data[pos]))
			pos, pending = end, end
			matched = true
			break
		}
		if !matched {
			pos++
		}
	}

	flush(len(data))
	return out.String()
}

// matchPrefix runs the backtick, escape and link patterns, which precede
// emphasis in the registry.
func (p *inlineParser) matchPrefix(data string, pos int) (int, string, bool) {
	rest := data[pos:]

	if match := backtickPattern.FindStringSubmatchIndex(rest); match != nil {
		// `to_typst_string`'s `code` branch emits the raw text in backticks and
		// escapes nothing (`:38-41`).
		return pos + match[1], "`" + rest[match[4]:match[5]] + "`", true
	}
	if match := escapePattern.FindStringSubmatch(rest); match != nil {
		return pos + len(match[0]), EscapeTypstCharacters(match[1]), true
	}
	if match := linkPattern.FindStringSubmatchIndex(rest); match != nil {
		text := rest[match[2]:match[3]]
		href := rest[match[4]:match[5]]
		if href == "" {
			// `child.get("href") if child.get("href") else "https://example.com"`
			// (`:46-47`) — an empty URL becomes the example, not an empty link.
			href = "https://example.com"
		}
		if title := linkTitlePattern.FindStringSubmatch(href); title != nil {
			href = title[1]
		}
		return pos + match[1], `#link("` + href + `")[` + p.parseFrom(text, -1, 0) + `]`, true
	}
	return 0, "", false
}

// isolatedDelimiter is NOT_STRONG_RE's condition, checked against the character
// before the position as well as after — the pattern's `(^|\s)` is a real match
// group upstream and consumes the space.
func (p *inlineParser) isolatedDelimiter(data string, pos int) bool {
	before := pos == 0 || isSpaceByte(data[pos-1])
	after := pos+1 >= len(data) || isSpaceByte(data[pos+1])
	return before && after
}

func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f'
}

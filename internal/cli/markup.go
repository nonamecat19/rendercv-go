package cli

import (
	"regexp"
	"slices"
	"strings"
)

// tagPattern is Rich's `RE_TAGS` (`rich/markup.py:12-15`), whose three groups
// are the whole tag, the backslashes escaping it, and the tag's own text.
//
// The character class is deliberately narrow: a tag has to open with a
// lowercase letter, `#`, `/` or `@`, so `[Bold]` and `[1]` are ordinary text.
var tagPattern = regexp.MustCompile(`(\\*)\[([a-z#/@][^\[]*?)\]`)

// Markup renders Rich console markup into styled text — `rich.markup.render`
// (`rich/markup.py:106-231`), the function every `rich.print` of a string goes
// through.
//
// **The port parses markup rather than assembling spans by hand, because the
// span structure is not obvious from the markup and it is what the bytes
// show.** `create_theme_command.py:42-55` opens `[purple]` on one line and
// closes it on the *next* one, so the outer purple keeps running to the end of
// the message and later `[cyan]` tags override its colour without ending it —
// measured, and a hand-built set of spans got it wrong twice.
//
// Three rules of Rich's that the run structure depends on:
//
//  1. **A closing tag closes the nearest matching open tag**, not the innermost
//     one (`pop_style`, `:146-151`), which is what leaves the outer `[purple]`
//     open above.
//  2. **A tag left open at the end covers the rest of the text** (`:224-228`).
//  3. **The spans are reversed and then stably sorted by start** (`:230`), and
//     that order is the colour precedence: the last span covering a character
//     wins it (`Text.render` combines the stack in span order,
//     `rich/text.py:758-766`).
//
// An unknown style name is **silently dropped**, not an error: `Text.render`
// resolves each span's name with `get_style(default=Style.null())`
// (`rich/text.py:736`). Measured end to end — `rendercv new "[qq]John"` prints
// `./John_CV.yaml`, having eaten the tag and styled nothing.
//
// Emoji codes are not replaced. Upstream passes `emoji=True` and no RenderCV
// markup contains one; a `:name:` in user input would differ, which is the one
// known gap in this function.
func Markup(markup string) Text {
	if !strings.Contains(markup, "[") {
		return PlainText(markup)
	}

	var (
		plain strings.Builder
		// length is the plain text's length in **runes**, because a span
		// offset is a rune offset (`style.go`) and Python's `len(text)` counts
		// codepoints.
		length int
		spans  []Span
		// stack holds each open tag with the offset it opened at.
		stack []openTag
	)

	appendText := func(text string) {
		// `plain_text.replace("\\[", "[")` (`:156`): a brace escaped for the
		// parser is written through unescaped.
		text = strings.ReplaceAll(text, `\[`, "[")
		plain.WriteString(text)
		length += len([]rune(text))
	}

	position := 0
	for _, match := range tagPattern.FindAllStringSubmatchIndex(markup, -1) {
		start, end := match[0], match[1]
		escapes := markup[match[2]:match[3]]
		tagText := markup[match[4]:match[5]]

		if start > position {
			appendText(markup[position:start])
		}
		if escapes != "" {
			// `divmod(len(escapes), 2)` (`:89`): each pair of backslashes is one
			// literal backslash, and an odd one escapes the tag itself.
			backslashes, escaped := len(escapes)/2, len(escapes)%2
			if backslashes > 0 {
				appendText(strings.Repeat(`\`, backslashes))
			}
			if escaped != 0 {
				appendText(markup[start+len(escapes) : end])
				position = end
				continue
			}
		}
		position = end

		name, parameters, hasParameters := strings.Cut(tagText, "=")
		if closing, ok := strings.CutPrefix(name, "/"); ok {
			var open openTag
			if open, stack, ok = popTag(stack, normalizeStyleName(closing)); !ok {
				// Rich raises `MarkupError` here (`:167`). Nothing in RenderCV
				// writes an unbalanced closing tag, and a traceback is a worse
				// answer than the text the user asked for, so the tag is simply
				// dropped.
				continue
			}
			spans = append(spans, Span{Start: open.start, End: length, Style: open.style})
			continue
		}
		normalized := normalizeStyleName(name)
		stack = append(stack, openTag{
			start: length,
			name:  normalized,
			style: StyleFromDefinition(tagDefinition(normalized, parameters, hasParameters)),
		})
	}
	if position < len(markup) {
		appendText(markup[position:])
	}

	// `while style_stack: start, tag = style_stack.pop()` (`:224-228`) — the
	// tags still open are closed at the end of the text, innermost first.
	for i := len(stack) - 1; i >= 0; i-- {
		spans = append(spans, Span{Start: stack[i].start, End: length, Style: stack[i].style})
	}

	// `sorted(spans[::-1], key=attrgetter("start"))` (`:230`). The reversal is
	// what puts a tag that was opened later — and therefore closed later —
	// after the one enclosing it, so its colour wins.
	slices.Reverse(spans)
	slices.SortStableFunc(spans, func(a, b Span) int { return a.Start - b.Start })

	// **A span whose style resolved to nothing is kept**, because Rich keeps it:
	// its offsets are still boundaries in `Text.render`, so `[qq]a[/qq]b` is two
	// unstyled runs rather than one.
	return Text{Plain: plain.String(), Spans: spans}
}

// openTag is one entry of Rich's `style_stack`.
type openTag struct {
	start int
	name  string
	style Style
}

// popTag is `pop_style` (`rich/markup.py:146-151`): the **nearest** open tag
// with this name, searched from the top of the stack.
func popTag(stack []openTag, name string) (openTag, []openTag, bool) {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].name == name {
			return stack[i], append(stack[:i:i], stack[i+1:]...), true
		}
	}
	return openTag{}, stack, false
}

// tagDefinition is `str(Tag)` (`rich/markup.py:28-31`): the style definition a
// tag stands for, which is its name, or its name and parameters separated by a
// space — so `[link=https://…]` becomes the definition `link https://…`.
func tagDefinition(name, parameters string, hasParameters bool) string {
	if !hasParameters {
		return name
	}
	return name + " " + parameters
}

// normalizeStyleName is `Style.normalize` (`rich/style.py`) for the names
// RenderCV writes: lowercase, trimmed.
//
// Rich normalizes by round-tripping through `Style.parse`, which also reorders
// the words — `cyan bold` normalizes to `bold cyan`. That reordering only
// matters for a closing tag spelled differently from its opening one, which
// nothing upstream does, so it is not reproduced here.
func normalizeStyleName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// StyleFromDefinition resolves a Rich style definition — the string a markup
// tag carries — to the style it names.
//
// It is `ParseStyle` plus the one definition that is not a colour: `link <url>`,
// which `Style.parse` turns into a hyperlink and no SGR (`rich/style.py:539-556`).
func StyleFromDefinition(definition string) Style {
	if url, ok := strings.CutPrefix(definition, "link "); ok {
		return LinkStyle(url)
	}
	style, _ := ParseStyle(definition)
	return style
}

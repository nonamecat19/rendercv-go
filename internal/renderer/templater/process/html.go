package process

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
)

// listMarker is a list item's indentation and bullet, which is the only place
// python-markdown and CommonMark disagree on the documents this port produces.
var listMarker = regexp.MustCompile(`^( +)([-*+] |\d+\. )`)

// pythonMarkdownTabLength is `markdown.Markdown`'s `tab_length` default, which
// is what decides whether an indented list item is a **child or a sibling**
// (`markdown/blockprocessors.py`, `ListIndentProcessor.tab_length`).
const pythonMarkdownTabLength = 4

// MarkdownToHTML is `markdown_to_html` (markdown_parser.py:193-202), which is
// `markdown.markdown(string)` with no extensions and no configuration.
//
// **The engine is goldmark, and one rule of upstream's is applied to its input
// rather than to its output.** python-markdown nests a list item only when it is
// indented by a full `tab_length` of 4; CommonMark — and so goldmark — nests at
// 2. The entry templates emit their nested highlights at **2** spaces, so
// upstream *flattens* them into siblings and goldmark nests them. Measured over
// the 24 corpus documents: goldmark alone matches 8, and matches all 24 once the
// under-indented markers are moved to column 0 first.
//
// That is a normalization of the *input* to upstream's own list rule, not a
// rewrite of goldmark's output. It is the whole difference between the two
// libraries on this corpus, which is the answer to the question spec 011 §6
// posed — and it is the opposite of the answer this port first recorded, from
// reading the diff rather than reducing it.
func MarkdownToHTML(markdown string) (string, error) {
	var out bytes.Buffer
	if err := goldmark.Convert([]byte(flattenShallowLists(markdown)), &out); err != nil {
		return "", err
	}
	// goldmark ends the document with a newline; upstream's `markdown.markdown`
	// returns the body without one, and `Full.html` supplies the layout.
	return strings.TrimRight(out.String(), "\n"), nil
}

// flattenShallowLists moves every list marker indented by less than a tab length
// to column 0, which is what python-markdown's list processor effectively does
// with it.
func flattenShallowLists(markdown string) string {
	lines := strings.Split(markdown, "\n")
	for i, line := range lines {
		match := listMarker.FindStringSubmatch(line)
		if match == nil || len(match[1]) >= pythonMarkdownTabLength {
			continue
		}
		lines[i] = line[len(match[1]):]
	}
	return strings.Join(lines, "\n")
}

package process_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// TestHTMLHeadingStripsEveryPythonSpace sweeps `HashHeaderProcessor`'s strip
// (`blockprocessors.py:479`) over all 29 characters `str.strip()` removes, at
// every heading level.
//
// The level is not an independent axis of the rule — `h%d` and the header group
// come out of one match of one regex (`:461`, `:478-479`) — but it is free to
// check, and a wrapper that trimmed only what it recognised as an `<h1>` would
// be caught here.
//
// `\n` and `\r` are the two characters that make it a different document: they
// end the heading line, so the text after them is a paragraph of its own, and
// that is upstream's answer too. Each `want` below is measured through the
// vendored `markdown.markdown`.
func TestHTMLHeadingStripsEveryPythonSpace(t *testing.T) {
	for level := 1; level <= 6; level++ {
		t.Run(fmt.Sprintf("h%d", level), func(t *testing.T) {
			hashes := strings.Repeat("#", level)
			for _, r := range pythonSpaces {
				in := fmt.Sprintf("%s %sh%s", hashes, string(r), string(r))
				want := fmt.Sprintf("<h%d>h</h%d>", level, level)
				if r == '\n' || r == '\r' {
					// The heading ends at the line break and `h` is the next
					// block, in both libraries.
					want = fmt.Sprintf("<h%d></h%d>\n<p>h</p>", level, level)
				}
				got, err := process.MarkdownToHTML(in)
				if err != nil {
					t.Fatalf("MarkdownToHTML(%q): %v", in, err)
				}
				if got != want {
					t.Errorf("MarkdownToHTML(%q) = %q, want %q", in, got, want)
				}
			}
		})
	}
}

// TestHTMLHeadingStripsBothEnds is the difference between this rule and the
// paragraph's, which is an `lstrip` (`paragraph.go`).
//
// A heading keeps nothing at either end, and a heading of nothing but
// whitespace is `<h1></h1>` rather than a heading of that whitespace. Every
// `want` is upstream's.
func TestHTMLHeadingStripsBothEnds(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"leading", "# \vh", "<h1>h</h1>"},
		{"trailing", "# h\v", "<h1>h</h1>"},
		{"both", "# \vh\v", "<h1>h</h1>"},
		{"space and more", "# \v h \v", "<h1>h</h1>"},
		{"nothing but whitespace", "# \v", "<h1></h1>"},
		{"interior is kept", "# a\vb", "<h1>a\vb</h1>"},
		{"between two paragraphs", "a\n\n# \vh\n\nb", "<p>a</p>\n<h1>h</h1>\n<p>b</p>"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := process.MarkdownToHTML(test.in)
			if err != nil {
				t.Fatalf("MarkdownToHTML(%q): %v", test.in, err)
			}
			if got != test.want {
				t.Errorf("MarkdownToHTML(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}

// TestHTMLHeadingOpensWithoutASpace sweeps the opening rule of
// `HashHeaderProcessor.RE` (`blockprocessors.py:461`) over levels 1 to 8 and
// seven followers.
//
// Nothing in the expression separates `#{1,6}` from `header`, so a space after
// the hashes is not required, and the run is greedy but capped, so a seventh
// hash is the text's first character rather than a seventh level. Both `want`
// formulas were run against the vendored `markdown.markdown` over all 56
// combinations before being written here.
func TestHTMLHeadingOpensWithoutASpace(t *testing.T) {
	for level := 1; level <= 8; level++ {
		t.Run(fmt.Sprintf("run%d", level), func(t *testing.T) {
			for _, follower := range "h1!-._(" {
				in := strings.Repeat("#", level) + string(follower)
				capped := min(level, 6)
				want := fmt.Sprintf("<h%d>%s%s</h%d>",
					capped, strings.Repeat("#", level-capped), string(follower), capped)
				got, err := process.MarkdownToHTML(in)
				if err != nil {
					t.Fatalf("MarkdownToHTML(%q): %v", in, err)
				}
				if got != want {
					t.Errorf("MarkdownToHTML(%q) = %q, want %q", in, got, want)
				}
			}
		})
	}
}

// TestHTMLHeadingRefusesIndentation is the other half of the same anchor: the
// hash run has to sit immediately after `^` or a `\n`, where CommonMark 0.31
// §4.2 allows three spaces of indentation.
//
// One space is already too many and the line is prose, `#` and all. At four it
// is an indented code block, which is where both libraries agree again. Every
// `want` is measured.
func TestHTMLHeadingRefusesIndentation(t *testing.T) {
	for indent := range 5 {
		t.Run(fmt.Sprintf("indent%d", indent), func(t *testing.T) {
			for _, level := range []int{1, 3} {
				hashes := strings.Repeat("#", level)
				in := strings.Repeat(" ", indent) + hashes + " h"
				var want string
				switch {
				case indent == 0:
					want = fmt.Sprintf("<h%d>h</h%d>", level, level)
				case indent < 4: // a tab length, where the indented code block begins
					want = fmt.Sprintf("<p>%s h</p>", hashes)
				default:
					want = fmt.Sprintf("<pre><code>%s h\n</code></pre>", hashes)
				}
				got, err := process.MarkdownToHTML(in)
				if err != nil {
					t.Fatalf("MarkdownToHTML(%q): %v", in, err)
				}
				if got != want {
					t.Errorf("MarkdownToHTML(%q) = %q, want %q", in, got, want)
				}
			}
		})
	}
}

// TestHTMLHeadingInProse is the reach of the two rules above, in the words a CV
// actually carries (spec-delta-atx §3.2).
//
// **Every one of the first thirteen turns prose into a heading**, and the
// hashes are consumed rather than shown: `#1 in sales` is `<h1>1 in sales</h1>`
// and not the sentence the user wrote. That is upstream's answer, so it is the
// port's; the five after them are the shapes that stay prose, because the hash
// is not at a line start or is escaped.
//
// `#include <stdio.h>` belongs to this set and is not here: it differs on the
// dotted tag name and not on the heading, and is pinned as its own class in
// `html_conformance_test.go`'s `knownRemainder`.
func TestHTMLHeadingInProse(t *testing.T) {
	tests := []struct{ in, want string }{
		{"#1 in sales", "<h1>1 in sales</h1>"},
		{"#1 ranked team", "<h1>1 ranked team</h1>"},
		{"#2 of 300 applicants", "<h1>2 of 300 applicants</h1>"},
		{"#hashtag", "<h1>hashtag</h1>"},
		{"#TeamWork", "<h1>TeamWork</h1>"},
		{"#tag and #tag2", "<h1>tag and #tag2</h1>"},
		{"#!/bin/sh", "<h1>!/bin/sh</h1>"},
		{"#define X 1", "<h1>define X 1</h1>"},
		{"#FF00AA is the brand colour", "<h1>FF00AA is the brand colour</h1>"},
		{"#1 in sales\nand more prose", "<h1>1 in sales</h1>\n<p>and more prose</p>"},
		{
			"#1 in sales -- led the team\n\nnext paragraph",
			"<h1>1 in sales -- led the team</h1>\n<p>next paragraph</p>",
		},
		{"#1 in sales\n===", "<h1>1 in sales</h1>\n<p>===</p>"},
		{"- #1 in sales", "<ul>\n<li>\n<h1>1 in sales</h1>\n</li>\n</ul>"},
		{"Ranked #1 in sales", "<p>Ranked #1 in sales</p>"},
		{"C# and .NET", "<p>C# and .NET</p>"},
		{"issue #42", "<p>issue #42</p>"},
		{"a #b", "<p>a #b</p>"},
		{"\\#1 in sales", "<p>#1 in sales</p>"},
	}
	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			got, err := process.MarkdownToHTML(test.in)
			if err != nil {
				t.Fatalf("MarkdownToHTML(%q): %v", test.in, err)
			}
			if got != test.want {
				t.Errorf("MarkdownToHTML(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}

// TestHTMLSetextHeadingStripsEveryPythonSpace sweeps `SetextHeaderProcessor`'s
// strip (`blockprocessors.py:510`) over all 29 characters, at both bars.
//
// `=` is an `<h1>` and `-` an `<h2>` (`:505-508`), and the bar's own shape is
// not part of this rule — the text comes from the line above it either way.
//
// `\n` is the character that makes it a different document, and it is upstream's
// answer too: the heading's text ends at the line break, so the bar no longer
// follows a paragraph line and stops being a bar. `\t` is the same story one
// step further on, since `expandtabs` turns a leading one into the four spaces
// of an indented code block. Both are measured, not skipped.
func TestHTMLSetextHeadingStripsEveryPythonSpace(t *testing.T) {
	bars := map[string]int{"===": 1, "---": 2}
	for bar, level := range bars {
		t.Run(bar, func(t *testing.T) {
			for _, r := range pythonSpaces {
				in := fmt.Sprintf("%sh%s\n%s", string(r), string(r), bar)
				want := fmt.Sprintf("<h%d>h</h%d>", level, level)
				switch r {
				case '\n':
					want = "<p>h</p>\n<p>===</p>"
					if level == 2 {
						want = "<p>h</p>\n<hr />"
					}
				case '\t':
					want = "<pre><code>h\n</code></pre>\n<p>===</p>"
					if level == 2 {
						want = "<pre><code>h\n</code></pre>\n<hr />"
					}
				}
				got, err := process.MarkdownToHTML(in)
				if err != nil {
					t.Fatalf("MarkdownToHTML(%q): %v", in, err)
				}
				if got != want {
					t.Errorf("MarkdownToHTML(%q) = %q, want %q", in, got, want)
				}
			}
		})
	}
}

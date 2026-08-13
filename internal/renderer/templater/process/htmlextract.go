package process

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
)

// commentCloseRe is `htmlparser.py:37`'s `commentclose`, the only terminator
// python-markdown accepts for a comment.
var commentCloseRe = regexp.MustCompile(`--!?>`)

// rawBlockEnd is `HTMLExtractor`'s tag stack (`htmlparser.py:209-253`): the
// offset just past the closing tag that empties it, or -1 if nothing closes the
// block.
//
// `start` is the `<` of a block-level start tag that has already been accepted
// as opening a raw block. From there every start tag pushes — `:222` appends
// whatever tag it sees while `inraw`, block-level or not — and every end tag
// pops back to its own name (`:235-239`), which is how `<div><br></div>` closes
// on one `</div>` even though nothing closed the `<br>`.
//
// **A tag that pushes nothing ends the block where it ends.** `hr` is upstream's
// one `empty_tags` entry (`:134`) and an XHTML-style `<div/>` is a
// `startendtag`; both go to `handle_empty_tag` (`:265-286`), which stashes the
// tag alone and returns the rest of the document to markdown. The stack is
// therefore still empty after the opening tag, and the block is over.
//
// Comments, processing instructions, declarations and CDATA take the same
// `handle_empty_tag` path when they stand alone; inside a raw block they are
// cached with no effect on the stack (`:267-269`), which is all this needs from
// them — goldmark opens its own block type for each of them, and those types
// already close on a terminator of their own.
func rawBlockEnd(source []byte, start int) int {
	var stack []string
	for i := start; i < len(source); {
		if source[i] != '<' {
			i++
			continue
		}
		rest := source[i:]
		switch {
		case bytes.HasPrefix(rest, []byte("<!--")):
			m := commentCloseRe.FindIndex(rest[4:])
			if m == nil {
				return -1
			}
			i += 4 + m[1]
		case bytes.HasPrefix(rest, []byte("<?")):
			j := bytes.Index(rest[2:], []byte("?>"))
			if j < 0 {
				return -1
			}
			i += 2 + j + 2
		case bytes.HasPrefix(rest, []byte("<!")):
			j := bytes.IndexByte(rest[2:], '>')
			if j < 0 {
				return -1
			}
			i += 2 + j + 1
		case bytes.HasPrefix(rest, []byte("</")):
			name, end, ok := scanTag(source, i)
			if !ok {
				i++
				continue
			}
			if len(stack) == 0 {
				// Not `inraw` upstream: an end tag with nothing open closes
				// nothing (`:235`).
				i = end
				continue
			}
			if slices.Contains(stack, name) {
				for len(stack) > 0 {
					top := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					if top == name {
						break
					}
				}
			}
			if len(stack) == 0 {
				return end
			}
			i = end
		default:
			name, end, ok := scanTag(source, i)
			if !ok {
				i++
				continue
			}
			if selfClosing(source[i:end]) || name == "hr" {
				if len(stack) == 0 {
					return end
				}
				i = end
				continue
			}
			stack = append(stack, name)
			i = end
		}
	}
	return -1
}

// scanTag reads one start or end tag, returning its lowercased name and the
// offset just past its `>`.
//
// The name is `tagfind_tolerant`'s (`html.parser`), a letter followed by
// anything but whitespace, `/` and `>`; the scan to `>` honours quoted
// attribute values, which is what keeps `<div a='>' >x</div>` one tag.
func scanTag(source []byte, i int) (name string, end int, ok bool) {
	j := i + 1
	if j < len(source) && source[j] == '/' {
		j++
	}
	if j >= len(source) || !isASCIILetter(source[j]) {
		return "", 0, false
	}
	nameStart := j
	for j < len(source) && !endsTagName(source[j]) {
		j++
	}
	name = lower(source[nameStart:j])

	quote := byte(0)
	for ; j < len(source); j++ {
		switch {
		case quote != 0:
			if source[j] == quote {
				quote = 0
			}
		case source[j] == '\'' || source[j] == '"':
			quote = source[j]
		case source[j] == '>':
			return name, j + 1, true
		}
	}
	return "", 0, false
}

// selfClosing reports whether a start tag is the XHTML-style `<div/>` upstream
// routes to `handle_startendtag` (`:288-289`).
func selfClosing(tag []byte) bool {
	return len(tag) >= 3 && tag[len(tag)-2] == '/'
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// endsTagName is `tagfind_tolerant`'s character class, the bytes a tag name
// cannot contain.
func endsTagName(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\f', '/', '>', 0:
		return true
	}
	return false
}

// splitRawBlockTails puts the content that follows a raw block's closing tag on
// its own line, which is where `HTMLExtractor` puts it.
//
// When the closing tag is not followed by a blank line, `:246-247` sets `intail`
// rather than extending the stash, and `:250-252` appends the stashed block and
// then `'\n\n'` to `cleandoc` — so the tail is a **new block** of the document,
// re-parsed as ordinary markdown:
//
//	<div>block</div> tail\n\nafter  →  <div>block</div>\n<p>tail</p>\n<p>after</p>
//	<div>x</div>     tail           →  <div>x</div>\n<pre><code> tail\n</code></pre>
//
// The second shape is why the tail keeps its leading whitespace: upstream hands
// the block parser everything after the tag, and five spaces of it is an
// indented code block there as much as anywhere else.
//
// **`separator` is the boundary the caller's own block splitter reads**, and the
// two paths spell it differently because they split blocks differently. The
// Typst path has only the text: `convertChunk` divides on a blank line
// (`markdown.go`'s `splitOnBlank`), so it asks for the `"\n\n"` upstream
// literally appends. The HTML path has a block parser, and `blockLevelHTMLParser.
// Continue` has already ended the raw block at its closing tag; one `"\n"` is
// enough there, and a blank line would be read back by
// `blankLineAfterRawBlock` as the one upstream only writes into the stash when
// the *input* had it (`htmlblock.go`), turning `<p>tail</p>` into a paragraph
// one newline too far down.
//
// Only a tail on the closing tag's own line is moved. A tail that is only
// whitespace is left alone: upstream stashes the tag without it, but inserting a
// separator there would manufacture exactly the blank line the paragraph above
// describes.
func splitRawBlockTails(source, separator string) string {
	var out strings.Builder
	raw := []byte(source)
	written := 0

	for i := 0; i < len(raw); {
		lineEnd := bytes.IndexByte(raw[i:], '\n')
		if lineEnd < 0 {
			lineEnd = len(raw)
		} else {
			lineEnd += i
		}

		tag := i + tagOffset(raw[i:lineEnd])
		end := rawBlockOnLine(raw, tag, lineEnd)
		if end < 0 {
			i = lineEnd + 1
			continue
		}

		tailEnd := bytes.IndexByte(raw[end:], '\n')
		if tailEnd < 0 {
			tailEnd = len(raw)
		} else {
			tailEnd += end
		}
		if len(bytes.Trim(raw[end:tailEnd], " \t")) == 0 {
			i = tailEnd + 1
			continue
		}

		out.Write(raw[written:end])
		out.WriteString(separator)
		written = end
		// The tail now starts a line of its own, so a raw block that begins in
		// it is at a line start too — `<div>a</div> <div>b</div>` is two blocks
		// upstream (`:215`'s `intail`).
		i = end
	}
	if written == 0 {
		return source
	}
	out.Write(raw[written:])
	return out.String()
}

// rawBlockOnLine is `rawBlockEnd` for a raw block that starts at `tag`, guarded
// by the two things `handle_starttag` checks first: the tag is block-level
// (`:215`) and it is at the start of its line (`:181-192`, `atLineStart`).
//
// **The start tag has to be complete on that line.** A tag broken over two lines
// leaves `self.offset` past the start of the line it ends on, so upstream is not
// at a line start when it reads it and the block never opens: `<div\n
// class='x'>a</div> t` is a paragraph upstream, not a raw block.
func rawBlockOnLine(source []byte, tag, lineEnd int) int {
	if tag >= lineEnd || source[tag] != '<' || !atLineStart(source, tag) {
		return -1
	}
	name, end, ok := scanTag(source, tag)
	if !ok || end > lineEnd || !blockLevelElements[name] {
		return -1
	}
	if bytes.HasPrefix(source[tag:], []byte("</")) {
		return -1
	}
	return rawBlockEnd(source, tag)
}

package process

import (
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

// typstReferences is `md.references` on the **module-level** `Markdown`
// instance `markdown_to_typst` converts with (`markdown_parser.py:147`), and
// its lifetime is the reason it lives at package scope rather than inside a
// call.
//
// `Markdown.convert` never calls `Markdown.reset` (`markdown/core.py:263-273`
// says so itself: "Should be called manually between calls to
// `Markdown.convert`"), and `markdown_to_typst` never calls it either. The map
// therefore accumulates across every line of a field **and across every field
// of the document**, which is observable in an ordinary CV. Two entries,
//
//   - "[zq]: https://example.com"
//   - "see [t][zq] here"
//
// render as `see #link("https://example.com")[t] here` under the vendored
// Python — the second entry resolving a definition the first one wrote. A
// per-call map gets that wrong, so this one is per-process, exactly as
// upstream's is.
//
// The HTML path is unaffected and must stay so: `markdown_to_html` calls
// `markdown.markdown` (`markdown_parser.py:202`), which builds a fresh
// `Markdown` per call, so its references are per-document. Nothing in this file
// is reachable from `html.go`, and for the same reason no reference form is
// added to `maskAbove`, which both paths share.
var typstReferences = struct {
	sync.Mutex
	byID map[string]string
}{byID: map[string]string{}}

// storeReference is `self.parser.md.references[id] = (link, title)`
// (`blockprocessors.py:593`).
//
// **The title is parsed and dropped.** `ReferenceInlineProcessor.makeTag` sets
// it as an attribute (`inlinepatterns.py:911-912`), but `to_typst_string`'s `a`
// branch reads only `href` (`markdown_parser.py:47-51`), so a definition with a
// title and one without produce the same Typst. It is still *matched*, because
// a title left unconsumed would be trailing content the block processor puts
// back on the queue.
func storeReference(id, href string) {
	typstReferences.Lock()
	defer typstReferences.Unlock()
	typstReferences.byID[id] = href
}

// lookupReference is `self.md.references[id]`, guarded by `if id not in
// self.md.references: return None` (`inlinepatterns.py:897-900`).
func lookupReference(id string) (string, bool) {
	typstReferences.Lock()
	defer typstReferences.Unlock()
	href, ok := typstReferences.byID[id]
	return href, ok
}

// pythonSpaceClass is the body of `\s` as Python's `re` means it for a `str`
// pattern.
//
// Measured against the vendored interpreter over all 0x110000 codepoints:
// Python's `\s` is exactly `str.isspace()`, so it is `isPythonSpace`'s set. Go's
// `\s` is only `[\t\n\f\r ]` and would have let a vertical tab or a
// non-breaking space into a reference's destination.
const pythonSpaceClass = `\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}` +
	`\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

// referenceDefinitionPattern is `ReferenceProcessor.RE`
// (`blockprocessors.py:579-581`).
//
// Upstream writes the title as `(["\'])(.*)\4` — one quote character and a
// backreference to it. RE2 has no backreference, so the alternation is spelled
// out: `"…"`, `'…'` and the parenthesised form are three branches. The first
// character decides the branch either way, so the two spellings accept the same
// strings; the title itself is never read (see storeReference).
var referenceDefinitionPattern = regexp.MustCompile(
	`(?m)^[ ]{0,3}\[([^\[\]]*)\]:[ ]*\n?[ ]*([^` + pythonSpaceClass + `]+)[ ]*` +
		`(?:\n[ ]*)?(?:".*"[ ]*|'.*'[ ]*|\(.*\)[ ]*)?$`)

// collectReference is `ReferenceProcessor.run` (`blockprocessors.py:586-604`),
// registered at priority 15 — below every other block processor and above
// `paragraph` (`blockprocessors.py:54-55`).
//
// **It produces no element.** The definition is recorded and its text
// disappears, which is the half of this a CV hits first: an entry holding
//
//	[ref][1]
//
//	[1]: https://example.com
//
// renders the second line as a blank line upstream and rendered it as
// `\[1\]: https:\/\/example.com` here.
//
// The processor **searches** the block rather than testing its head, so a
// definition in the middle splits it: the lines before the match and the lines
// after it go back on the block queue in that order (`:594-599`), each parsed
// from the top. That is reachable only through a `\r`, which `MarkdownToTypst`
// does not treat as a line boundary.
func collectReference(lines []string) (before, after []string, ok bool) {
	block := strings.Join(lines, "\n")
	m := referenceDefinitionPattern.FindStringSubmatchIndex(block)
	if m == nil {
		return nil, nil, false
	}

	// `id = m.group(1).strip().lower()` and `link = m.group(2).lstrip('<')
	// .rstrip('>')` (`:591-592`). The id is stripped **here and only here**: the
	// inline side collapses whitespace runs instead of trimming them, so
	// `[  a  ]: u` defines `a` while `[t][  a  ]` looks up `" a "` and misses.
	id := strings.ToLower(trimPythonSpace(block[m[2]:m[3]]))
	href := strings.TrimRight(strings.TrimLeft(block[m[4]:m[5]], "<"), ">")
	storeReference(id, href)

	if tail := block[m[1]:]; trimPythonSpace(tail) != "" {
		after = strings.Split(strings.TrimLeft(tail, "\n"), "\n")
	}
	if head := block[:m[0]]; trimPythonSpace(head) != "" {
		before = strings.Split(strings.TrimRight(head, "\n"), "\n")
	}
	return before, after, true
}

// referenceIDPattern is `ReferenceInlineProcessor.NEWLINE_CLEANUP_RE`,
// `re.compile(r'\s+', re.MULTILINE)` (`inlinepatterns.py:878`).
var referenceIDPattern = regexp.MustCompile(`[` + pythonSpaceClass + `]+`)

// cleanReferenceID is `id.lower()` followed by `NEWLINE_CLEANUP_RE.sub(' ', id)`
// (`inlinepatterns.py:894-896`).
//
// **It collapses, it does not trim**, so `[t][a  b]` finds the definition
// `[a b]: u` and `[t][ a ]` does not find `[a]: u`.
func cleanReferenceID(text string) string {
	return referenceIDPattern.ReplaceAllString(strings.ToLower(text), " ")
}

// matchReference is the four reference inline processors — `reference` (170),
// `image_reference` (140), `short_reference` (130) and `short_image_ref` (125)
// (`inlinepatterns.py:75-86`) — at the head of `data[pos:]`.
//
// They are one function because they differ in exactly two bits: whether a `!`
// precedes the label, and whether the id comes from a second bracket pair or
// from the label itself.
//
// # Declining is not the same as consuming, and here it does not have to be
//
// Upstream returns `(None, m.start(0), end)` for an id it cannot find
// (`inlinepatterns.py:897-898`), which leaves the span literal and resumes
// *that* pattern after it; the patterns below it then get their own full pass
// over the unchanged text. This port tries the patterns in priority order at
// each position instead, so an unresolved reference simply declines and the
// next one is offered the same position. The two agree on the shape that
// distinguishes them — with `t` defined and `9` not, `[t][9]` is
// `#link(…)[t]\[9\]` under both, the full form declining and the short form
// claiming the label. What a decline does *not* do is expose the span to a
// reference pattern again — see `inlineParser.refBarrier`.
//
// minPriority is the lowest priority the caller is offering the position to, so
// that `matchPrefix` can run `reference` (170) ahead of `link` and the other
// three behind `image_link` without splitting this into four functions.
func (p *inlineParser) matchReference(data string, pos, pending, minPriority int) (int, string, bool) {
	if pos < p.refBarrier {
		return 0, "", false
	}
	rest := data[pos:]

	image := len(rest) > 1 && rest[0] == '!' && rest[1] == '['
	labelStart := 2
	if !image {
		// `NOIMG`, the same lookbehind the direct link form carries.
		if len(rest) == 0 || rest[0] != '[' || precededByBang(data, pos, pending) {
			return 0, "", false
		}
		labelStart = 1
	}

	// The same escape-masked scan the direct forms use: `escape` (180) outranks
	// every reference pattern, so a `\]` in the label is not a closing bracket.
	scan := maskAbove(rest, prioReference)
	_, afterLabel := matchBracketed(scan, labelStart, '[', ']')
	if afterLabel < 0 {
		return 0, "", false
	}
	text := rest[labelStart : afterLabel-1]

	// The full form first and the short form after it, because **the full form
	// declining is not the short form declining**: with `t` defined and `9` not,
	// `![t][9]` is `image_reference` giving up on `9` and then `short_image_ref`
	// claiming `![t]`, leaving `[9]` as the tail. Upstream reaches that by
	// giving each pattern its own pass over the whole text; here the two
	// candidates are tried in registry order at the position.
	if idStart, idEnd, after, matched := matchReferenceID(scan, afterLabel); matched {
		id := rest[idStart:idEnd]
		if id == "" {
			id = text // `if not id: id = text` (`inlinepatterns.py:917-918`)
		}
		if typst, ok := p.resolveReference(text, id, image, true, minPriority); ok {
			return pos + after, typst, true
		}
	}
	if typst, ok := p.resolveReference(text, text, image, false, minPriority); ok {
		return pos + afterLabel, typst, true
	}

	// The short form was offered this span and turned it down, so no reference
	// pattern sees inside it again — see `inlineParser.refBarrier`. A decline
	// that came from `minPriority` or `linkFloor` rather than from the map is
	// the pattern never running, and leaves no barrier.
	short := referencePriority(image, false)
	if short >= minPriority && p.linkFloor >= short {
		p.refBarrier = pos + afterLabel
	}
	return 0, "", false
}

// resolveReference is `handleMatch`'s tail: look the id up, and build the `a`
// or the `img` (`inlinepatterns.py:882-901`).
func (p *inlineParser) resolveReference(text, id string, image, full bool, minPriority int) (string, bool) {
	priority := referencePriority(image, full)
	if priority < minPriority || p.linkFloor < priority {
		return "", false
	}
	href, defined := lookupReference(cleanReferenceID(id))
	if !defined {
		return "", false
	}

	if image {
		// `ImageReferenceInlineProcessor.makeTag` builds an `img`
		// (`inlinepatterns.py:940-949`), and `to_typst_string` has no branch for
		// one: the default branch recurses into an element with no text and no
		// children, so **a resolved image reference contributes nothing**. Its
		// tail is emitted as usual.
		return "", true
	}
	if href == "" {
		// `child.get("href") if child.get("href") else "https://example.com"`
		// (`markdown_parser.py:49`).
		href = "https://example.com"
	}

	// `__applyPattern` re-parses a built element's text from `patternIndex + 1`
	// (`treeprocessors.py:313-317`), so the label of a full reference is parsed
	// with `link` (160) still available — `[a [b](c) d][1]` really does nest an
	// inline link — while the label of a short reference starts below
	// `short_image_ref` (125).
	outer := p.linkFloor
	p.linkFloor = referenceLabelFloor(priority)
	inner := p.parseFrom(text, -1, 0)
	p.linkFloor = outer

	return `#link("` + href + `")[` + inner + `]`, true
}

// referencePriority is the registry priority of whichever of the four
// processors a shape belongs to (`inlinepatterns.py:75-86`).
func referencePriority(image, full bool) int {
	switch {
	case image && full:
		return prioImageRef
	case image:
		return prioShortImageRef
	case full:
		return prioReference
	default:
		return prioShortRef
	}
}

// referenceLabelFloor is the priority of the pattern registered directly below
// the one that built the element, which is where its label is re-parsed from.
func referenceLabelFloor(priority int) int {
	if priority == prioReference {
		return prioLink
	}
	return prioShortImageRef
}

// matchReferenceID is `RE_LINK.match(data, pos=index)`,
// `re.compile(r'\s?\[([^\]]*)\]', re.DOTALL | re.UNICODE)`
// (`inlinepatterns.py:880` and `:909`), returning the id's bounds and the index
// just past its closing bracket.
//
// `[^\]]*` under `DOTALL` spans newlines, and the `\s?` is **one** optional
// whitespace character — Python's set, not Go's — which is why `[t] [1]`
// resolves and `[t]  [1]` does not.
func matchReferenceID(scan string, index int) (start, end, after int, ok bool) {
	pos := index
	if pos < len(scan) {
		if r, width := utf8.DecodeRuneInString(scan[pos:]); isPythonSpace(r) {
			pos += width
		}
	}
	if pos >= len(scan) || scan[pos] != '[' {
		return 0, 0, 0, false
	}
	closing := strings.IndexByte(scan[pos+1:], ']')
	if closing < 0 {
		return 0, 0, 0, false
	}
	return pos + 1, pos + 1 + closing, pos + closing + 2, true
}

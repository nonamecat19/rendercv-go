package process

import (
	"bytes"
	"fmt"
	"regexp"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// automailPattern is python-markdown's `AUTOMAIL_RE`
// (`markdown/inlinepatterns.py:158`), used by both halves below: the parser asks
// it what to claim, and the renderer asks it whether an autolink goldmark found
// counts as *mail*.
//
// It is deliberately looser than an email grammar: any run without space, `<`,
// `>` or `!`, an `@`, then a run without those or a second `@`. That is why
// `<mailto:a@b.com>` is a **mail** autolink upstream and not a URL one — automail
// is registered at priority 110 against autolink's 120, and python-markdown runs
// the higher number first (`inlinepatterns.py:88`).
var automailPattern = regexp.MustCompile(`^[^<> !]+@[^@<> ]+$`)

// autolinkPattern is `AUTOLINK_RE` (`inlinepatterns.py:155`), which is registered
// at priority 120 against automail's 110 and so **wins**: an `@` inside a URL —
// `<http://x@y.com>`, a userinfo component — is a plain link upstream, not an
// obfuscated mail one. Only `http`, `https`, `ftp` and `ftps` are spelled out
// there.
var autolinkPattern = regexp.MustCompile(`^(?i:https?|ftps?)://[^<>]*$`)

// isAutomail applies those two patterns in upstream's own order.
func isAutomail(label []byte) bool {
	return !autolinkPattern.Match(label) && automailPattern.Match(label)
}

// autoLinkRenderer is `AutomailInlineProcessor`
// (`markdown/inlinepatterns.py:971-995`): an email autolink is written out
// **entirely as character references**, both in the `href` and in the link text,
// which is python-markdown's spam-harvester obfuscation.
//
//	<a@b.com>  →  <a href="&#109;&#97;&#105;…">&#97;&#64;&#98;&#46;…</a>
//
// goldmark writes a plain `mailto:a@b.com`, so any email address in a CV — a
// contact line is the obvious one — produced a differing `.html`.
//
// URL autolinks are delegated to goldmark's own renderer, which the two
// libraries already agree on.
type autoLinkRenderer struct {
	inner renderer.NodeRendererFunc
}

// RegisterFuncs claims the autolink node, and only that one.
func (r autoLinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindAutoLink, r.renderAutoLink)
}

func (r autoLinkRenderer) renderAutoLink(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	label := node.(*ast.AutoLink).Label(source)
	if !isAutomail(label) {
		return r.inner(w, source, node, entering)
	}

	// python strips a `mailto:` the author wrote and adds its own back, so the
	// two spellings of the same address obfuscate identically
	// (`inlinepatterns.py:977-978`).
	email := bytes.TrimPrefix(label, []byte("mailto:"))

	_, _ = w.WriteString(`<a href="`)
	writeDecimalReferences(w, append([]byte("mailto:"), email...))
	_, _ = w.WriteString(`">`)
	writeNamedReferences(w, email)
	_, _ = w.WriteString(`</a>`)

	return ast.WalkSkipChildren, nil
}

// automailParser widens goldmark's autolink grammar to `AUTOMAIL_RE`'s.
//
// goldmark follows CommonMark, whose email autolink is an ASCII grammar, so
// `<josé@x.com>` and `<a"b@x.com>` were not autolinks at all and came out as
// escaped literal text. python-markdown only asks for the loose regex above, so
// both are `mailto:` links there.
//
// It registers ahead of goldmark's own (300) and takes every address the regex
// accepts, which is a superset — upstream's automail runs ahead of its autolink
// too. Anything it declines falls through to `NewAutoLinkParser` and then
// `NewRawHTMLParser` (400), which is what keeps `<b>` and `<div>` passing through
// untouched: neither is `…@…`.
type automailParser struct{}

// Trigger is `<`, shared with the autolink and raw-HTML parsers.
func (automailParser) Trigger() []byte { return []byte{'<'} }

// Parse claims a `<…@…>` run on the current line, or declines with nil.
func (automailParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, segment := block.PeekLine()
	end := bytes.IndexByte(line, '>')
	if end < 0 || !isAutomail(line[1:end]) {
		return nil
	}
	block.Advance(end + 1)
	value := ast.NewTextSegment(text.NewSegment(segment.Start+1, segment.Start+end))
	return ast.NewAutoLink(ast.AutoLinkEmail, value)
}

// nodeRendererFunc pulls one node kind's function out of a `NodeRenderer` by
// offering it a registerer that keeps rather than installs. It is how a node
// renderer here delegates to goldmark's own for the cases the two libraries
// already agree on, instead of copying it.
type nodeRendererFunc struct {
	kind ast.NodeKind
	got  renderer.NodeRendererFunc
}

// Register keeps the one function asked for and discards the rest.
func (c *nodeRendererFunc) Register(kind ast.NodeKind, fn renderer.NodeRendererFunc) {
	if kind == c.kind {
		c.got = fn
	}
}

func defaultNodeRendererFunc(kind ast.NodeKind, from renderer.NodeRenderer) renderer.NodeRendererFunc {
	capture := nodeRendererFunc{kind: kind}
	from.RegisterFuncs(&capture)
	return capture.got
}

// writeNamedReferences is the **link text** encoding: `&name;` when HTML 4 gives
// the character one and `&#decimal;` otherwise — `codepoint2name(ord(letter))`
// in `inlinepatterns.py:980-990`. It iterates *characters*, so a multi-byte rune
// is one reference and not one per byte.
func writeNamedReferences(w util.BufWriter, text []byte) {
	for _, r := range string(text) {
		if name, named := codepointNames[r]; named {
			_, _ = fmt.Fprintf(w, "&%s;", name)
			continue
		}
		_, _ = fmt.Fprintf(w, "&#%d;", r)
	}
}

// writeDecimalReferences is the **href** encoding, which is not the same one:
// `inlinepatterns.py:992-994` builds the `mailto:` with a bare `'#%d;'` and never
// consults the name table. So a `"` is `&quot;` in the text and `&#34;` in the
// href of the very same link.
func writeDecimalReferences(w util.BufWriter, text []byte) {
	for _, r := range string(text) {
		_, _ = fmt.Fprintf(w, "&#%d;", r)
	}
}

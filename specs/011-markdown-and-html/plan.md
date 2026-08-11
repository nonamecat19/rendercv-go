# Iteration 11 — plan

The Go design for `spec.md`. Wave A shipped; Wave B is measured and scoped, not built.

---

## 1. Wave A — the Markdown document (**done**)

**No new pipeline.** Upstream's `render_full_template` takes `file_type` as a parameter, so the port
took the same shape: `internal/renderer/typstdoc` became `internal/renderer/document` and
`Render` gained a `templater.Format`. The three format-dependent decisions of spec §1 behavior 3
are the only branches:

| Decision | Where |
|---|---|
| template directory and extension | the `Format` passed to `Environment.Render` |
| processor chain | `processFormat`, which maps onto `process.Format` |
| preamble or not | one `if typst` around one call, and `Assemble`'s existing flag |

The one genuinely new piece is `bridge.MarkdownFields`: the Markdown header reads `cv`'s five
contact fields where the Typst header reads the pre-formatted connection list. Two of the five
needed a serialization the raw node does not carry — the phone's stored RFC 3966 form and the
website's `HttpUrl` form — and the phone is the one the corpus caught.

## 2. Wave B — the HTML document (**done**)

`render_html` is trivial; `markdown_to_html` looked as though it was not. The design question
spec §6 refused to answer without measurement got measured — twice, and the first answer was wrong.

### The first measurement, and the wrong conclusion drawn from it

goldmark out of the box, over the 24 `.md` documents this port produces, matched **8**. Reading the
first differing line of two cases, this plan concluded the misses were "loose versus tight lists",
called that a block-layer disagreement no post-pass could fix, and cut the wave.

**That was reading a diff instead of reducing one.** The 16 misses had a single cause, and it is
not loose lists:

> python-markdown nests a list item only when it is indented by a full `tab_length` — **4** spaces
> (`markdown/blockprocessors.py`). CommonMark, and so goldmark, nests at **2**. The entry templates
> emit nested highlights at exactly 2, so upstream *flattens* them into siblings and goldmark nests
> them.

### The second measurement

Move every list marker indented by less than 4 spaces to column 0 before converting, and goldmark
matches **24 of 24**. One rule, applied to the *input*, in upstream's own terms.

### Why this is a library substitution and not a divergence

Same reasoning as `nyaruka/phonenumbers` in plan 009 §3: upstream's choice is a Python package the
port cannot call, and the user-visible output is byte-identical on every case measured. Where the
two libraries disagree on this corpus is now exactly one documented rule, reproduced deliberately.

**What it does not cover**: the constructs the corpus does not contain. `MarkdownToHTML` accepts
arbitrary Markdown and only the shapes the eight entry templates emit (spec §3 behavior 11) are
pinned. A user's `summary` containing a table, a code fence or raw HTML is unmeasured.

### The one upstream oddity reproduced rather than fixed

`Full.html` interpolates a `title` that `render_html` never binds (`templater.py:153`), so every
upstream `.html` has an empty `<title>`. The port binds nothing there either. Passing
`settings.pdf_title` would be an improvement and an artifact diff.

---

## 3. Wave C — the two remaining divergence classes (**designed, not built**)

Covers spec §7 (emphasis) and §8 (spaced link destination). Both are goldmark inline parsers the
port replaces; neither touches the block layer, the writer, or `Full.html`.

Everything lands in the existing `internal/renderer/templater/process` package, next to
`image.go`, `link.go`, `codespan.go` — the same shape those already use, so a reviewer diffs
mentally against a file that is already there.

### 3.1 The asset that makes this cheap: `emphasis.go` already exists

`internal/renderer/templater/process/emphasis.go` is a **faithful, measured port of
`AsteriskProcessor.PATTERNS` and `UnderscoreProcessor.PATTERNS`** written for the Typst path. It
already carries every hard part:

| spec behavior | already in `emphasis.go` |
|---|---|
| 17, the five ordered patterns | `asteriskPatterns`, `underscorePatterns` |
| 20, `EM_STRONG_RE`'s `strong,em` tag order | the `build` closures |
| 22, the index guard | `parseFrom(data, from, delim)`'s `from` |
| 24, the word-boundary guards | `matchSmart`, `isWordByte` |
| the backreference/lookaround problem | hand-written scanners, RE2 avoided |

Evidence it is right: running the Typst path on the divergent shapes today gives
`ParseInline("*a **b** c*") == "#emph[a ]#emph[b]#emph[ c]"` — three sibling spans, upstream's
answer — while `MarkdownToHTML` on the same input gives one nested `<em><strong>`.
`ParseInline("_a __b__ c_")` likewise leaves the inner `__` literal.

**So this is not a new port. It is a second backend for a port that exists.**

### 3.2 The refactor: split matching from emitting

`emphasisPattern.build` currently returns a Typst string and calls `inlineParser.parseFrom`. Make
the emitter a parameter.

- `emphasisPattern.match` is untouched and is the whole value.
- `build` moves behind a small interface — one method to open/close a `strong`, one for an `em`,
  one for literal text, one to recurse with an index — with the existing Typst implementation as
  its first, unchanged-output client.
- The refactor's criterion is that `markdown_to_typst.json`'s 166 rows stay green with **zero**
  diff. It ships as its own commit, before any HTML behavior changes, so that if the HTML work
  regresses the Typst path the bisect is one commit wide.

### 3.3 The HTML backend: a goldmark inline parser, not a post-pass

Add `emphasisParser` to `converter`'s `parser.WithInlineParsers` (`html.go:37-39`) at triggers
`*` and `_`, at a priority **above** goldmark's own emphasis parser so it wins outright. Same
technique as `automailParser` and `imageParser` already registered there.

- `Parse` peeks the line, runs the shared matcher at the trigger offset, and, on a match, builds
  `ast.Emphasis` nodes (level 1 = `<em>`, level 2 = `<strong>`) in **upstream's** order —
  behavior 20 means the outer node for `***a***` is the strong one.
- Bodies are recursed with the matched pattern's index (behavior 22). At index 4 the body carries
  **no** emphasis at all, so it is emitted as a text node whose delimiters are literal; the
  matcher already tells us the index, so this is a parameter, not a special case.
- Non-emphasis inline content inside a body — code spans, links — must still be parsed. Emit the
  body as a source range and let goldmark's remaining inline parsers run over it, exactly as
  `linkTitleSplitter` relies on. `*a [b](c) d*` and ``*a `b` c*`` already match and are the
  regression pins for this.
- **Alternative rejected: a post-pass over goldmark's emphasis AST.** goldmark's delimiter-run
  resolution has already thrown away which delimiters were consumed by the time the AST exists, so
  `*a **b** c*` cannot be re-split into three siblings from the tree. Measured on the shape, not
  assumed.
- **Alternative rejected: disable goldmark emphasis and pre-transform the source string.** That is
  what the Typst path does and it works there because the Typst path is line-at-a-time with no
  AST. Here it would mean emitting HTML into the Markdown source before block parsing, which the
  writer would then escape. Same trap the earlier `htmlescape.go` attempt hit.

### 3.4 The link backend: transcribe `getLink`, share it with the image path

`imageParser` already calls `matchBracketed` + `parseDestination` (`image.go:41-52`) and already
matches `![a](b c)`. Promote that pair into the faithful scanner and have both callers use it.

- New `getLink(data string, index int) (href, title string, end int, handled bool)` in `link.go`,
  a direct transcription of `inlinepatterns.py:716-830`: paren depth from 1, primary and
  secondary quote tracking, the backtrack counter for `[t](a"b)`, `handled = bracket_count == 0`,
  then `strings.TrimSpace` on the href and the title's whitespace-to-space normalization.
- New `linkParser` on trigger `[`, registered alongside `imageParser`, declining when the
  previous byte is `!` (upstream's `NOIMG`, `inlinepatterns.py:140`) so the image parser keeps
  its shapes.
- The label is parsed as inline Markdown (unlike the image's `alt`, spec behavior 33 and
  `image.go`'s comment), so the label range is handed back to goldmark rather than stashed as an
  `ast.String`.
- **The unexported-`linkLabelState` objection in `html_conformance_test.go`'s comment is
  narrower than it reads.** The port does not need to *reuse* goldmark's label handling; it needs
  a balanced-bracket label scan, which `matchBracketed` already is. `getText`
  (`inlinepatterns.py:832+`) is itself just a bracket-balance loop.
- Decline anything crossing a line boundary and record the decline (spec §9.3). `imageParser`
  already sets this precedent.

### 3.5 Dependencies

None added. goldmark stays; both changes are its own extension points
(`parser.WithInlineParsers`), which the file already uses twice.

### 3.6 Hazards from `AGENTS.md` §6

- §6.1 / §6.2 (Jinja semantics, `trim_blocks`) — untouched; this is below the templater.
- The live hazard is **shared-code regression**: `emphasis.go` serves the Typst path, and
  `markdown_to_typst.json` is the only thing standing between a refactor and a silent `.typ`
  change. Hence §3.2's zero-diff commit boundary.
- Second live hazard: **an inverted assertion that starts passing**. `knownRemainder` entries must
  be deleted in the same commit as the fix, or `TestMarkdownToHTMLMatchesPython` fails — which is
  the design working, not a problem to route around.

### 3.7 What is deliberately not designed here

Spec §9.5's block-tag-in-a-list-item. It is a raw-HTML stash in a preprocessor over the source
string — a different layer from both of these — and it gets its own unit when someone takes it.


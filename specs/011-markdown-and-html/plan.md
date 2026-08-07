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

## 2. Wave B — the HTML document

`render_html` is trivial; `markdown_to_html` is not. The design question spec §6 refused to answer
without measurement has now been measured.

### The measurement

goldmark, out of the box, over the 24 `.md` documents this port now produces, compared against the
`<article>` bodies of upstream's own `.html`:

```
same 8    diff 16
```

The differing cases share one cause: **loose versus tight lists**. Where an entry's highlights are
separated by blank lines — which is what every non-minimal corpus case produces — python-markdown
emits `<li>\n<p>…</p>\n</li>` and goldmark structures the list differently. This is a block-layer
disagreement, not a whitespace one, so no post-pass over goldmark's output normalizes it away.

### What that rules out

The "goldmark and normalize" route from spec §6 is the one the measurement kills: normalizing a
structural difference means rewriting the tree, at which point the library is no longer doing the
work it was chosen for.

### The route left

Port python-markdown's block layer, as iteration 8 ported its inline layer. The input is narrow —
spec §3 behavior 11 lists what the templates can actually emit — so the scope is bounded:
paragraphs, ATX headings, unordered lists (loose and tight), and the serializer's tag and newline
placement. Iteration 8's inline port is already there to build on, and it is the same shape of
job: read `markdown/blockprocessors.py` and `markdown/serializers.py`, port, differentially test.

**It is its own iteration's worth of work**, which is why this one stops here rather than shipping
half of it. `AGENTS.md` §10.2 forbids marking an iteration done with a failing conformance case,
so the `.html` comparison is not in the suite at all yet — the fixture carries the expected bytes,
and the test that reads them lands with the implementation.

## 3. What the fixture already gives Wave B

`tools/docprobe` stores `expected.html` beside `expected.md` and `expected.typ` for all 24 cases.
So the gate for Wave B exists and is red-by-absence: adding one loop to
`document_conformance_test.go` turns it on the moment `markdown_to_html` exists.

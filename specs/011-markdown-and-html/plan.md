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


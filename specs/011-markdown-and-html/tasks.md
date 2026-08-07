# Iteration 11 — tasks

| # | Unit | Criterion | State |
|---|---|---|---|
| T1 | rename `typstdoc` → `document`, `Render` takes a format | the Typst suite unchanged | **done** |
| T2 | `tools/docprobe` captures `.md` and `.html` too | 24 cases × 3 artifacts | **done** |
| T3 | the Markdown document | 24/24 `.md` byte-identical | **done** |
| T4 | `bridge.MarkdownFields` — the header's five contact fields | each field's removal breaks 18–21 cases | **done** |
| T5 | `markdown_to_html` — goldmark plus python-markdown's list-indent rule | 24/24 `.html` byte-identical | **done** |
| T6 | `render_html` + `Full.html` | 24/24, and the HTML is built from the `.md` this run produced | **done** |

---

## The cut that should not have been

T5 and T6 were cut once, on this reasoning: goldmark matched 8 of 24, the misses "were loose versus
tight lists", and that made them a block-layer port worth its own iteration.

**The measurement was real; the diagnosis was not.** All 16 misses had one cause —
python-markdown nests a list item at `tab_length` 4 where CommonMark nests at 2, and the entry
templates emit nested highlights at 2. Normalizing that one thing in the input makes goldmark match
**24 of 24**.

The difference between the two conclusions is fifteen minutes of reducing the diff instead of
reading its first line. This is the third time in this port that "only a bigger port can fix this"
turned out to be false — spec 008 §8 said only a corpus `.typ` could check the template transform,
and a fragment differential found a real bug in its first run.

Mutation-checked: without the list-indent rule 16 of 24 fail, with a tab length of 2 the same 16
fail, and keeping goldmark's trailing newline fails all 24.

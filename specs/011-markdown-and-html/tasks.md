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

---

## Wave C — the two remaining divergence classes

Spec §7 and §8, plan §3. **Every unit is `[sequential]`.** This is inside the renderer, i.e. on
the pipeline spine (`AGENTS.md` §5), and T8–T10 additionally share `emphasis.go` with the Typst
path. One owner, in order. Do not fan this out.

Fixture units land **first and red** (`AGENTS.md` §7). Every expected string comes from running
the vendored submodule's `markdown.markdown`; none may be hand-written (`AGENTS.md` §10.1).

| # | Unit | Criterion | Mode |
|---|---|---|---|
| T7 | `html.json` gains the spec §7.2 and §8.1 rows; the four fixed shapes get `knownRemainder` entries so the suite stays green while the rows are red-by-pin | every new row reproduces under the submodule's own `markdown.markdown`; `just test-parity` green | [sequential] |
| T8 | refactor `emphasis.go`: `build` takes an emitter, Typst becomes its first client | `markdown_to_typst.json` 166/166, **zero** output diff; no HTML behavior change | [sequential] |
| T9 | `emphasisParser` — the HTML emitter, registered on `*` and `_` above goldmark's own | spec §7.2's `***a***`/`___a___` tag order and the three-sibling shapes; drop `___strong em___` and `*a **bold** thing*` from `knownRemainder` **in this commit** | [sequential] |
| T10 | the index guard on the HTML path (spec behavior 22) | `_a __b__ c_` leaves the inner `__` literal; `___a_b__`, `___a__b_`, `__a_b___` match; drop `_a __b__ c_` from `knownRemainder` | [sequential] |
| T11 | `getLink` — transcribe `inlinepatterns.py:716-830` into `link.go`; `imageParser` switches to it | `![a](b c)` and every §8.1 `=` row unchanged; no new link shapes yet | [sequential] |
| T12 | `linkParser` on `[`, declining after `!` | every non-`=` row of §8.1; drop `[t](a b)` from `knownRemainder` | [sequential] |
| T13 | `[t](a\nb)` added to `knownRemainder` with an inverted assertion (spec §9.3) | the decline is recorded and still differs | [sequential] |
| T14 | corpus additions (spec §11) regenerated through `tools/docprobe` | the 24 `.md` are **unchanged**; the `.html` change only where the four shapes appear | [sequential] |

### Ordering constraints, and why they are real

- **T7 before T8–T13.** A fixture that lands after its fix cannot fail, and this iteration has
  already been promoted once on a fixture that measured the wrong thing.
- **T8 before T9.** T9 needs the emitter seam. Splitting them is what makes a Typst regression a
  one-commit bisect instead of a two-feature one.
- **T9 before T10.** T10 is a parameter on T9's recursion; there is nothing to guard until the
  HTML emitter recurses.
- **T11 before T12.** T12 is the caller; T11 is the scanner, and landing it under the already-
  passing image path proves it before it is trusted with new shapes.
- **T14 last.** It regenerates artifacts, so it must observe the finished behavior.

T8–T10 and T11–T12 look like two independent chains and are not: both edit
`internal/renderer/templater/process` and both delete keys from the same `knownRemainder` map.
One owner.

### Mutation checks each unit must survive

- T9: reverting the tag order for `EM_STRONG_RE` must fail `***a***`.
- T10: removing the `index <= idx` guard must fail `_a __b__ c_`.
- T11: replacing the paren counter with a first-`)` scan must fail `[t](a(b)c)`.
- T12: adding a "no space in a bare destination" rule must fail `[t](a b)`.
- T7: deleting any one added row must reduce the differential's row count, i.e. the count is
  asserted, not just the rows.

### Not in this wave

Spec §9.5, the block-level tag inside a list item. It stays in `knownRemainder` with its inverted
assertion and is a separate unit for whoever takes it.

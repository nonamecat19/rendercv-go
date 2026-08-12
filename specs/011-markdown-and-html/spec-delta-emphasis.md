# Spec delta 011-E — the emphasis class, re-measured

Extends [`spec.md`](spec.md) §7, §9 and §12. Nothing here supersedes those sections; §7's five
patterns and §12's registry-priority model are the model this delta measures against. Written
against `internal/renderer/templater/process` at `b6dcee0`, python-markdown **3.10.2**, goldmark as
vendored.

Every string in this document was produced by running the vendored library, per `AGENTS.md` §10.1.
The recipes are in §8; nothing below was hand-written or reasoned out.

---

## 0. Summary — of three findings, one reproduces

| Finding as recorded | Status at `b6dcee0` | Evidence |
|---|---|---|
| Emphasis **flanking**: `**\x1cb**` is literal upstream, `<strong>` here | **REPRODUCES**, and is wider than recorded: 96 of 201 probed shapes on the HTML path, 100 on the Typst path | §3 |
| Emphasis **nesting**: `___x___`, an `*em*` reopening around a nested `**strong**`, intraword `_` | **DOES NOT REPRODUCE.** 53 nesting shapes, both backends, 0 mismatches | §4 |
| The two standing `knownRemainder` keys | **NOT EMPHASIS.** One is a link destination spanning a line break (§9.3, declined), one is a block tag in a list item (§9.5) | §4.3 |

The reproducing finding is **not a parser problem**. It is two character-class predicates —
Python's `\s` and `\w` — implemented over bytes where upstream implements them over Unicode. §6
sizes it, with a working prototype: **~40 lines, seven guard sites, no goldmark internals, zero
regressions across the 641-row differential and the whole unit suite.**

> **Recommendation to the merge owner.** Scope §6 as its own code unit and take it now. The rest of
> this document exists to record why the other two findings should not become work.

---

## 1. What upstream actually does

### 1.1 The registry (corrects "two regex tree processors")

The two emphasis processors are **inline** processors, not tree processors. They are registered in
`build_inlinepatterns` and run inside the one `treeprocessors.InlineProcessor` that walks the tree:

| Priority | Name | Class | Cite |
|---|---|---|---|
| 90 | `html` | `HtmlInlineProcessor` | `markdown/inlinepatterns.py:90` |
| 80 | `entity` | `HtmlInlineProcessor` | `:91` |
| **70** | **`not_strong`** | `SimpleTextInlineProcessor(NOT_STRONG_RE)` | `:92` |
| **60** | **`em_strong`** | `AsteriskProcessor(r'\*')` | `:93` |
| **50** | **`em_strong2`** | `UnderscoreProcessor(r'_')` | `:94` |

Registry order is the behavior; §12 of the parent spec establishes that, and the port implements it
as `stash.go`'s `maskAbove`.

### 1.2 The patterns

`AsteriskProcessor.PATTERNS` (`inlinepatterns.py:546-552`) and `UnderscoreProcessor.PATTERNS`
(`:680-686`), each compiled `re.DOTALL | re.UNICODE`:

```
EM_STRONG_RE        = r'(\*)\1{2}(.+?)\1(.*?)\1{2}'                                   :125
STRONG_EM_RE        = r'(\*)\1{2}(.+?)\1{2}(.*?)\1'                                   :131
STRONG_EM3_RE       = r'(\*)\1(?!\1)([^*]+?)\1(?!\1)(.+?)\1{3}'                       :137
STRONG_RE           = r'(\*{2})(.+?)\1'                                               :113
EMPHASIS_RE         = r'(\*)([^\*]+)\1'                                               :110
EM_STRONG2_RE       = r'(_)\1{2}(.+?)\1(.*?)\1{2}'                                    :128
STRONG_EM2_RE       = r'(_)\1{2}(.+?)\1{2}(.*?)\1'                                    :134
SMART_STRONG_EM_RE  = r'(?<!\w)(\_)\1(?!\1)(.+?)(?<!\w)\1(?!\1)(.+?)\1{3}(?!\w)'      :122
SMART_STRONG_RE     = r'(?<!\w)(_{2})(?!_)(.+?)(?<!_)\1(?!\w)'                        :116
SMART_EMPHASIS_RE   = r'(?<!\w)(_)(?!_)(.+?)(?<!_)\1(?!\w)'                           :119
NOT_STRONG_RE       = r'((^|(?<=\s))(\*{1,3}|_{1,3})(?=\s|$))'                        :152
```

**Six occurrences of `\w` and three of `\s` decide the whole reproducing class.** `re.UNICODE` is
the default for `str` patterns and is passed explicitly here, so both are Unicode classes:

- **`\s` is 29 codepoints**, not six:
  `U+0009 U+000A U+000B U+000C U+000D U+001C U+001D U+001E U+001F U+0020 U+0085 U+00A0 U+1680
  U+2000..U+200A U+2028 U+2029 U+202F U+205F U+3000`.
- **`\w` is 137,936 codepoints**: every `L*` (Lu, Ll, Lt, Lm, Lo), every `N*` (Nd, Nl, No), and
  `U+005F`. No `\s` codepoint is a `\w` codepoint.

Both sets were enumerated by sweeping `re.match` over all 1,112,064 non-surrogate codepoints
(§8.1), not read from documentation.

### 1.3 Why `NOT_STRONG_RE` is the one that bites

At priority 70 it runs **before** either emphasis processor, and `SimpleTextInlineProcessor` turns
its match into stashed literal text. So a run of one to three delimiters that *begins the block or
follows `\s`* and is *followed by `\s` or the end* can never open or close anything. With `\s`
counted correctly, `**\xa0b**` has its opening `**` eaten by `not_strong` — because `\xa0` is
whitespace — and the strong never forms.

---

## 2. What the port does today

**goldmark's own emphasis parser is not in the pipeline.** `pythonInlineParsers`
(`internal/renderer/templater/process/html.go:160-175`) removes `parser.NewEmphasisParser()` by
identity and `emphasisParser` (`emphasis_html.go:31`) is registered in its place at priority 450.
The five patterns are hand-written matchers in `emphasis.go`, shared by the Typst and HTML paths;
`NOT_STRONG_RE` is `stash.go`'s `maskNotStrong` (`:263`).

So the recorded framing — "python-markdown's Unicode-aware `\s` (29 chars) **against goldmark's
flanking rule** (4)" — describes a comparison that no longer runs. CommonMark's flanking rule is
not consulted anywhere in this path. What is consulted is:

| Predicate | Where | What it is | What upstream is |
|---|---|---|---|
| `isSpaceByte` | `inline.go:398` | `' ' \t \n \r \v \f` — **6 bytes** | 29 codepoints |
| `isWordByte` | `emphasis.go:244` | ASCII `[A-Za-z0-9_]` **plus every byte ≥ 0x80** | 137,936 codepoints, and `\xa0`, `·`, `—` are *not* among them |

`isWordByte`'s `b >= 0x80` arm is wrong in **both** directions: it makes every UTF-8 continuation
byte a word byte (intended) and also every non-ASCII punctuation, symbol and space rune (not
intended).

The seven guard sites that decide emphasis:

| Site | Guard it implements |
|---|---|
| `emphasis.go:212` `matchSmart` | `(?<!\w)` before an opening `_` run |
| `emphasis.go:231` `matchSmart` | `(?!\w)` after a closing `_` run |
| `emphasis.go:293` `matchStrongEm3` | `(?<!\w)` before the opening pair |
| `emphasis.go:316` `matchStrongEm3` | `(?<!\w)` before the middle delimiter |
| `emphasis.go:326` `matchStrongEm3` | `(?!\w)` after the closing run |
| `stash.go:268`, `:279` `maskNotStrong` | `(^|(?<=\s))` and `(?=\s|$)` |

`isSpaceByte`'s other callers — `link.go`, `softbreak.go`, `emphasis_html.go:273` — implement
*different* upstream rules and are **out of scope**; see §7.3.

---

## 3. Class E — the reproducing divergence, measured

201 shapes, both backends. **HTML: 96 emphasis mismatches** (98 counting the two non-emphasis
`knownRemainder` rows). **Typst: 100.**

### 3.1 E-1, `\s` — a delimiter run beside Unicode whitespace

23 of the 29 `\s` codepoints diverge (the 6 the port already knows are fine), in three shapes each:

| Shape | Upstream | Port | Rule |
|---|---|---|---|
| `**\x1cb**` | `<p>**\x1cb**</p>` | `<p><strong>\x1cb</strong></p>` | `NOT_STRONG_RE` eats the opener |
| `**\xa0b**` | `<p>**\xa0b**</p>` | `<p><strong>\xa0b</strong></p>` | same |
| `**b\xa0** x` | `<p>**b\xa0** x</p>` | `<p><strong>b\xa0</strong> x</p>` | `NOT_STRONG_RE` eats the closer |
| `a **\xa0** b` | `<p>a **\xa0** b</p>` | `<p>a <strong>\xa0</strong> b</p>` | both runs literal |

Typst, same input `**\xa0b**`:

```
upstream  #sym.ast.basic#h(0pt, weak: true) #sym.ast.basic#h(0pt, weak: true) \xa0b#sym.ast.basic#h(0pt, weak: true) #sym.ast.basic#h(0pt, weak: true)
port      #strong[\xa0b]
```

### 3.2 E-2, `\w` — an underscore beside a non-ASCII non-word rune

19 shapes on `a<SPACE>_b_ c` for the 19 `\s` runes above `\x7f`, plus 8 on bare non-word runes:

| Shape | Upstream | Port |
|---|---|---|
| `a\xa0_b_ c` | `<p>a\xa0<em>b</em> c</p>` | `<p>a\xa0_b_ c</p>` |
| `·_x_` | `<p>·<em>x</em></p>` | `<p>·_x_</p>` |
| `_x_—` | `<p><em>x</em>—</p>` | `<p>_x_—</p>` |
| `·__x__` | `<p>·<strong>x</strong></p>` | `<p>·__x__</p>` |

Typst for `·_x_`: upstream `·#emph[x]`, port `·\_x\_`.

Note the two classes push **opposite ways**: E-1 makes the port emphasise where upstream does not,
E-2 makes it refuse where upstream does.

### 3.3 It is reachable from an ordinary CV

A non-breaking space is what a word processor puts in pasted text.

| Input | Upstream (Typst) | Port (Typst) |
|---|---|---|
| `**\xa0Senior Engineer**` | four literal `#sym.ast.basic` runs | `#strong[\xa0Senior Engineer]` |
| `**Team lead\xa0** at ACME` | literal runs | `#strong[Team lead\xa0] at ACME` |

The first is a highlight that reads **bold in our PDF and literal asterisks in upstream's**. Seven
realistic shapes were probed; the two with an NBSP adjacent to a delimiter diverge and the other
five agree (`Led\xa0**a team** of 5`, `Built _the\xa0pipeline_`, `Grew revenue by **20 %**`,
`Wrote *docs*\xa0and tests`, `**Python**\xa0/ **Go**`).

---

## 4. What does not reproduce — do not turn these into work

### 4.1 Nesting: 53 shapes, 0 mismatches, both backends

Including every shape the finding named, and every shape parent §9.1 recorded as divergent when it
was written:

```
___x___   ____x____   _____x_____   ***a***   ***bold italic***   **_bold italic_**
*a **b** c*   **a *b* c**   *a **b***   **a*b***   ***a*b**   *a*b*
_a_b_   a_b_c   snake_case_   _snake_case   __a_b___   __a__b__   _ a _
*a\nb*   **a\nb**   *[link](u)*   **`code`**   *<b>x</b>*   \*a\*   *a\*b*
```

§9.1 of the parent spec ("`___x___` is divergent") was true when written and is **now stale**: Wave
C's `emphasisParser` closed it. §9.2 (intraword `_` is not a divergence) still holds. The finding
appears to have been carried forward from §9.1's era.

### 4.2 The narrowing the finding already suspected

`***bold italic***` and `**_bold italic_**` match — correct, and so does everything else in §4.1.
The reach is not "narrower than it sounds"; on this evidence it is empty.

### 4.3 The two `knownRemainder` keys are not emphasis

Both re-measured at `b6dcee0`; both still differ, neither involves `*` or `_`:

| Key | Upstream | Port | Class |
|---|---|---|---|
| `[t](a\nb)` | `<p><a href="a\nb">t</a></p>` | `<p>[t](a\nb)</p>` | link destination, §9.3, **declined permanently** |
| `- <div>block</div>` | `<ul>\n<li><div>block</div></li>\n</ul>` | `<ul>\n<li>\n<div>block</div></li>\n</ul>` | raw-HTML stash before block parsing, §9.5 |

Fixing the emphasis class will not move either, and neither should be bundled with it.

---

## 5. The earlier sizing is stale

The recorded estimate — "a hand-written replacement `parser.InlineParser`, a new file each,
comparable to `emphasis.go`'s existing Typst-side reimplementation" — describes work that **has
already shipped**: `emphasis_html.go`, `link.go`, `image.go`, `automail.go` are exactly that.

`linkLabelState` / `processLinkLabel` being unexported is a real constraint, but it constrains the
**link-label** work — parent §12.5's `![[b](c)](u)`, re-measured as still divergent — not emphasis.
`parser.WithInlineParsers` is sufficient for emphasis and is already how this port does it: a
parser owns its trigger bytes and returns a node for the whole span, which is what `emphasisParser`
does. No goldmark internal is needed for §6.

---

## 6. The fix, sized against a working prototype

Prototyped in the worktree, measured, then reverted; nothing was committed.

**Change:** add `isPyWordRune`/`isPySpaceRune` over runes, plus `wordBefore`, `wordAt`,
`spaceBefore`, `spaceAt` helpers that decode the adjacent rune with `utf8.DecodeLastRuneInString` /
`utf8.DecodeRuneInString`; point the seven guard sites in §2 at them. `isWordByte` and
`isSpaceByte` stay for the callers that are out of scope. About 40 lines.

**The rule, and its gate:**

```go
func isPyWordRune(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) }
```

`unicode.IsNumber`, **not** `unicode.IsDigit`: Python's `\w` includes `Nl` (236 codepoints) and
`No` (915), which `IsDigit` (Nd only) excludes. Swept against `re.match(r'\w', …)` and
`re.match(r'\s', …)` over all 1,112,064 non-surrogate codepoints: **identical, 0 differences**.

**Result of the prototype:**

| Suite | Before | After |
|---|---|---|
| §3 probe, HTML | 96 emphasis mismatches / 201 | **0** |
| §3 probe, Typst | 100 / 201 | **0** |
| `testdata/html.json`, 641 rows, tagged | green | **green** |
| `go test ./...` | green | **green** |
| `go test -tags conformance ./internal/renderer/...` | green | **green** |

Not one of the 641 rows moved. That is the answer to the regression question: the change is
invisible to every shape whose delimiters are flanked by ASCII, which is every shape the fixture
currently holds.

---

## 7. Regression strategy

### 7.1 The fixture has no generator, and that is the first task

`testdata/html.json` is machine-generated by construction (`AGENTS.md` §10.1) but **no tool in this
repo generates it** — `tools/` has no writer for it, and the parent spec's T7 says only "generated
through the vendored submodule's `markdown.markdown`". Every new row is therefore currently a hand
transcription waiting to happen. Write `tools/mdprobe` first (§9, task E-1), have it emit both
`html.json` and a new `typst.json`, and regenerate the existing 641 rows to prove it reproduces
them byte for byte before adding anything.

### 7.2 Rows to add

Machine-generated, red before the fix:

- 3 shapes × 23 `\s` codepoints = 69 rows (E-1);
- `a<ws>_b_ c` × 19 = 19 rows, plus 8 non-word-rune rows (E-2);
- the 7 realistic §3.3 shapes;
- the 53 §4.1 nesting shapes as **regression pins** — they pass today and this is what proves the
  fix does not disturb them;
- the same shapes on the Typst path, which today has no differential fixture at all.

### 7.3 What must not change

`link.go:267`, `:289`, `:302`, `:325`, `softbreak.go:129` and `emphasis_html.go:273` also call
`isSpaceByte`. They implement upstream rules with **their own** whitespace notions
(`LINK_RE`'s scanner, `\n` handling in `br` detection, the window builder). Changing them is a
separate question with separate evidence; the unit must leave them alone and say so in the commit.

---

## 8. Reproduction recipes

### 8.1 The character classes

```bash
cd third_party/rendercv && uv run python -c '
import re
print(len([c for c in range(0x110000) if re.match(r"\s", chr(c))]))
print(len([c for c in range(0x110000) if re.match(r"\w", chr(c))]))'
```

The sweep gate: emit one line per non-surrogate codepoint from both sides and `diff`.

### 8.2 The shape differential

```python
# upstream, from third_party/rendercv with uv run
import markdown
from rendercv.renderer.templater.markdown_parser import markdown_to_typst
markdown.markdown(shape)        # the HTML path, markdown_parser.py:202
markdown_to_typst(shape)        # the Typst path, markdown_parser.py:158-190
```

```go
// port
process.MarkdownToHTML(shape)   // html.go:100
process.MarkdownToTypst(shape)  // markdown.go:69
```

The Typst path splits on `\n` and converts line by line (`markdown_parser.py:175-189`), so a
multi-line shape must be compared on that path with the same splitting.

---

## 9. Tasks

| # | Unit | Done when | Notes |
|---|---|---|---|
| E-1 | `tools/mdprobe` generates `html.json` | it reproduces the existing 641 rows byte for byte | no behavior change; unblocks every row below |
| E-2 | `mdprobe` also emits `typst.json`; a tagged differential reads it | the Typst path has a fixture, green at its current behavior | the PDF-visible path has none today |
| E-3 | Fixture rows for §3.1, §3.2, §3.3 land red-by-pin | every new row reproduces under the vendored library; suite green via `knownRemainder` | fixtures first, red (`AGENTS.md` §7) |
| E-4 | Rune-aware `\s` and `\w` predicates + the seven guard sites | E-3's rows go green, `knownRemainder` shrinks to the two §4.3 keys, all 641 old rows unmoved | the whole of §6 |
| E-5 | The §4.1 nesting shapes land as regression pins | green before and after E-4 | records that the nesting finding is closed |
| E-6 | Parent §9.1 corrected to say `___x___` now matches, §7/§12 cross-referenced to this delta | the stale sentence is gone | documentation only |

E-1 and E-2 are independent of each other; E-3 needs E-1; E-4 needs E-3; E-5 needs E-1. E-6 is
independent.

---

## 10. Acceptance criteria

- [ ] `tools/mdprobe` regenerates `html.json` and the existing 641 rows are byte-identical.
- [ ] All 23 `\s` codepoints are fixture rows in all three §3.1 shapes, and match.
- [ ] `a\xa0_b_ c`, `·_x_`, `_x_—`, `·__x__` match on both backends.
- [ ] `**\xa0Senior Engineer**` matches on the Typst path.
- [ ] The `\w` and `\s` predicates are swept against Python over every non-surrogate codepoint with
      0 differences, and the sweep is a committed test.
- [ ] All 53 §4.1 shapes are fixture rows and are green before and after E-4.
- [ ] `knownRemainder` holds exactly the two §4.3 keys — no emphasis key is added.
- [ ] `go test ./...`, `go test -tags conformance ./internal/renderer/...` green.

---

## 11. Adjacent findings, not part of this delta

1. **`strings.go:150`'s `isWordRune` uses `unicode.IsDigit`.** It implements Python's `\b` for
   `MakeKeywordsBold`, and `IsDigit` is `Nd` only, so it misses `Nl` and `No` — a keyword adjacent
   to `²` or `Ⅷ` gets a boundary Python does not give it. Same rule as §6, different feature, not
   measured end-to-end here. It is the reason §6 should export one shared predicate rather than add
   a second private one.
2. **Parent §12.5's link-in-image-label** (`![[b](c)](u)`) re-measured at `b6dcee0`: still
   `alt="[b](c)"` here against `alt="b"` upstream. Unchanged, still not corpus-reachable.
3. **`*x`+backtick+`\ny*`** — parent §12.5 lists it as open; it **matches** on both backends now.

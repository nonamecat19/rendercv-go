# Iteration 11 — the Markdown and HTML documents

Behavior of the two remaining text artifacts, extracted from the vendored Python. No Go design
here.

Upstream: `third_party/rendercv` @ `v2.8` (`2eba248`).
Primary sources: `src/rendercv/renderer/{markdown.py,html.py}`,
`src/rendercv/renderer/templater/templater.py:50-155`, and
`src/rendercv/renderer/templater/markdown_parser.py:193-202`.

---

## 0. What this iteration is

**Two artifacts, one already 90% built.** Iteration 9 ported `render_full_template` and every
piece it calls; upstream takes `file_type` as a *parameter* (`templater.py:51`), so the Markdown
document is the same function with a different template directory. What is genuinely new is the
Markdown→HTML conversion.

The two artifacts are not independent: **the HTML is generated from the Markdown file's bytes**,
not from the model (`html.py:31-33`), so a wrong `.md` is a wrong `.html` and the order is fixed.

---

## 1. The Markdown document

1. `generate_markdown` (`markdown.py:9-28`) is `render_full_template(model, "markdown")` written to
   the resolved path, in UTF-8.
2. It returns `None` when `settings.render_command.dont_generate_markdown` is set, and **that
   return value is what disables the HTML** (`html.py:28-30`) — the two flags are not independent.
3. The document differs from the Typst one in exactly three places (`templater.py:76-97`):
   - the templates come from `templates/markdown/` with a `.md` extension;
   - **there is no preamble** — the document opens with the header;
   - the string-processor chain is `make_keywords_bold` **only**; `markdown_to_typst` does not run,
     because the output is already Markdown (spec 008 §3).
4. **The theme's `.j2.md` overrides are not searched.** The theme-qualified candidate path is tried
   only for Typst (`templater.py:197-202`), so a user's `classic/Header.j2.md` is ignored where
   `classic/Header.j2.typ` is honored.

## 2. The Markdown header's context is not the Typst header's

5. The Typst header reads `cv._connections`, a list of pre-formatted strings. The Markdown header
   reads the **fields themselves** — `cv.phone`, `cv.email`, `cv.location`, `cv.website`,
   `cv.social_networks` — and formats them inline (`templates/markdown/Header.j2.md`).
6. `cv.phone` is printed with `tel:` stripped and every `-` turned into a space, by two chained
   `replace` filters in the template rather than by a processor.
7. `cv.website` is printed twice: as link text with `https://` and the trailing `/` removed, and as
   the href unchanged.
8. Each social network contributes `network.network`, `network.username` and `network.url` — the
   generated profile URL of spec 004 §3.13, not a cleaned one.
9. **None of these five fields is processed** by `process_model`, which touches only `name`,
   `headline` and the sections (`model_processor.py:88-95`). So a `_` in an email survives
   unescaped, which is correct for Markdown and would be wrong for Typst.

## 3. Markdown → HTML

10. `markdown_to_html` is `markdown.markdown(markdown_string)` (`markdown_parser.py:202`) —
    python-markdown with **no extensions and no configuration**. Its defaults are therefore the
    whole specification: the default `output_format` (`xhtml`), the default extension set (none),
    and the default `tab_length` (4).
11. The input is not arbitrary Markdown. It is the document §1 produced, so the constructs that can
    actually appear are the ones the eight entry templates and the header emit: ATX headings,
    unordered lists, links, `**bold**`, `*italic*`, and paragraphs.
12. `render_html` (`templater.py:130-155`) renders `html/Full.html` with the converted body bound to
    `html_body`, plus the same four model names every fragment gets.
13. **`html_body` is indented by 8** through Jinja's `indent` filter, which does **not** indent the
    first line (spec 008 §4 pins this) — so the body's opening tag sits where the template put it
    and every later line carries eight spaces.
14. The `<html lang="…">` attribute is `locale.language_iso_639_1`, the same table the Typst
    preamble uses.
15. `Full.html` is a single fragment with no theme lookup and no per-section rendering; the whole
    document is one `render_single_template` call.

## 4. Out of scope

**4.1 The CLI flags** `-nomd` and `-nohtml` are iteration 12's. This iteration ends at two strings;
which of them gets written is the render command's decision.

**4.2 `resolve_rendercv_file_path`** — the output paths and their placeholders — is iteration 12's
as well. The corpus's `.md` and `.html` are compared as content here, not as filenames.

**4.3 PDF and PNG** are iteration 10's.

---

## 5. Acceptance criteria

- [x] §1's Markdown document byte-identical to upstream's `.md` for every corpus case that has an
      input, by the same differential `tools/typprobe` established for the `.typ`.
- [x] §2's five header fields, driven by a document that supplies all of them.
- [x] §3's HTML byte-identical for the same cases.
- [x] §3 behavior 13's indent, which is visible in every case at once and therefore proves nothing
      on its own — it needs a case whose body has more than one line.

## 6. The known hazard

**python-markdown is not goldmark, and this is where that bill comes due.** Iteration 8 ported
python-markdown's *inline* layer for `markdown_to_typst` and recorded five measured divergences in
the process — none of which have a `divergences.md` entry yet, because that file is human-gated.
`markdown_to_html` needs the *block* layer as well: paragraphs, lists, headings, and the
serializer's exact tag and newline placement.

Two routes, and the choice belongs in `plan.md` after measurement, not here:

- port python-markdown's block layer, as iteration 8 did for the inline one;
- use goldmark and normalize, which trades a known-shaped porting job for an unknown-shaped
  difference hunt.

**What decides it is a measurement**: run both over the `.md` documents the corpus actually
produces and count the differing cases. The input is narrow (§3 behavior 11), so the honest answer
may be that neither library's general behavior matters much — but that is a claim to test, not to
assume. Iteration 8's spec §8 made exactly this kind of assumption and hid a real bug behind it.

**Resolved: goldmark, plus one rule.** The measurement said 8 of 24, and the second measurement —
after reducing the diff rather than reading it — said the 16 misses were a single list-indent
difference. Normalizing it in the input gives 24 of 24. `plan.md` §2 carries the reasoning, and the
wrong first conclusion, which was to cut the wave as its own iteration.

**The bill is not fully paid.** §7 and §8 below are the two remaining classes, measured rather
than reasoned about, and neither is parity-*impossible* — see §9.4. They are open work in this
iteration, **not** proposed `divergences.md` entries.

---

## 7. Emphasis: python-markdown resolves `*` and `_` by ordered regex, not by delimiter runs

### 7.1 Where the behavior comes from

Both conversion paths get stock emphasis. `markdown_to_html` is bare `markdown.markdown()`
(`third_party/rendercv/src/rendercv/renderer/templater/markdown_parser.py:202`); the Typst path
constructs `markdown.core.Markdown(extensions=["admonition"])`
(`markdown_parser.py:147`). Neither replaces an inline pattern, so both inherit
`AsteriskProcessor` and `UnderscoreProcessor` unchanged.

Citations below are to the pinned dependency, python-markdown **3.10.2**
(`third_party/rendercv/uv.lock:440-441`), vendored at
`third_party/rendercv/.venv/lib/python3.12/site-packages/markdown/inlinepatterns.py`.
Short form in this section: `inlinepatterns.py:NNN`.

16. There are exactly two emphasis processors, one per delimiter character:
    `AsteriskProcessor` (`inlinepatterns.py:543`) and `UnderscoreProcessor`
    (`inlinepatterns.py:677`), the latter a subclass differing only in its pattern list.
17. Each carries an **ordered list of five patterns** — `AsteriskProcessor.PATTERNS`
    (`inlinepatterns.py:546-552`), `UnderscoreProcessor.PATTERNS` (`inlinepatterns.py:680-686`).
    The index in that list is load-bearing; it is threaded through every recursion.

    | idx | asterisk (`:546-552`) | underscore (`:680-686`) | builder | tags |
    |---|---|---|---|---|
    | 0 | `EM_STRONG_RE` `:125` | `EM_STRONG2_RE` `:128` | `double` | `strong,em` |
    | 1 | `STRONG_EM_RE` `:131` | `STRONG_EM2_RE` `:134` | `double` | `em,strong` |
    | 2 | `STRONG_EM3_RE` `:137` | `SMART_STRONG_EM_RE` `:122` | `double2` | `strong,em` |
    | 3 | `STRONG_RE` `:113` | `SMART_STRONG_RE` `:116` | `single` | `strong` |
    | 4 | `EMPHASIS_RE` `:110` | `SMART_EMPHASIS_RE` `:119` | `single` | `em` |

18. At a trigger character, `handleMatch` (`inlinepatterns.py:660-674`) tries the five patterns
    **in index order** and the **first** one that matches at exactly that offset wins; it `break`s.
    There is no delimiter stack, no left/right-flanking rule, and no "longest match" preference.
19. `EMPHASIS_RE` is `(\*)([^\*]+)\1` (`inlinepatterns.py:110`). Its body character class
    **cannot cross an asterisk**, so `*a **b** c*` does not produce one span covering the whole
    run; it produces `*a *`, then the scan resumes and produces `*b*`, then `* c*` — three
    sibling `<em>` elements.
20. `EM_STRONG_RE` is `(\*)\1{2}(.+?)\1(.*?)\1{2}` at index **0** with tags `strong,em`
    (`inlinepatterns.py:125`, `:547`), so `***a***` is `<strong><em>a</em></strong>`.
    CommonMark — and so goldmark — emits `<em><strong>a</strong></em>`. **The outer tag differs.**
    This is the shape a CV bullet writes for bold-italic, and it is currently a defect: an
    earlier reading of this iteration recorded `***bold italic***` as matching. It does not.
21. `build_single` / `build_double` / `build_double2` (`inlinepatterns.py:557-590`) recurse into
    the matched body through `parse_sub_patterns(text, parent, last, idx)`, passing the index of
    the pattern that matched.
22. **`parse_sub_patterns` only tries patterns strictly after that index**
    (`inlinepatterns.py:613-616`, the `if index <= idx: continue` guard). Consequences, all
    mechanical:
    - a body matched at index 4 (`EMPHASIS_RE` / `SMART_EMPHASIS_RE`) can contain **no**
      emphasis at all — every inner delimiter is literal;
    - a body matched at index 3 (`STRONG_RE` / `SMART_STRONG_RE`) can contain only index 4, i.e.
      `<em>` but never a nested `<strong>`;
    - nesting is therefore **monotone downwards** in the pattern list. CommonMark has no such
      rule.
23. The inner loop at `inlinepatterns.py:613-634` does **not** `break` after a successful match;
    it advances `pos` and keeps trying the remaining, higher-index patterns from the new
    position. More than one element can be appended per pass. Reproduce this, do not tidy it.
24. `SMART_EMPHASIS_RE` `(?<!\w)(_)(?!_)(.+?)(?<!_)\1(?!\w)` (`inlinepatterns.py:119`) and
    `SMART_STRONG_RE` (`:116`) carry word-boundary guards. **This is not a divergence.**
    Measured, goldmark already agrees on every intraword-underscore shape tested; see §9.2.
    It needs regression pinning, not a fix.

### 7.2 Measured shapes

Every row below was produced by running the vendored `markdown.markdown` and the port's current
`MarkdownToHTML` on the same input. `want` is upstream. Rows marked **=** already agree and exist
only to stop a fix from breaking them.

| input | want (python-markdown 3.10.2) | current port (goldmark) |
|---|---|---|
| `***a***` | `<p><strong><em>a</em></strong></p>` | `<p><em><strong>a</strong></em></p>` |
| `___a___` | `<p><strong><em>a</em></strong></p>` | `<p><em><strong>a</strong></em></p>` |
| `*a **b** c*` | `<p><em>a </em><em>b</em><em> c</em></p>` | `<p><em>a <strong>b</strong> c</em></p>` |
| `*a **b***` | `<p><em>a </em><em>b</em>**</p>` | `<p><em>a <strong>b</strong></em></p>` |
| `_a __b__ c_` | `<p><em>a __b__ c</em></p>` | `<p><em>a <strong>b</strong> c</em></p>` |
| `___a_b__` | `<p><strong><em>a</em>b</strong></p>` | `<p>_<strong>a_b</strong></p>` |
| `___a__b_` | `<p><em><strong>a</strong>b</em></p>` | `<p>__<em>a__b</em></p>` |
| `__a_b___` | `<p>__a_b___</p>` | `<p><strong>a_b</strong>_</p>` |
| `***a***b` | `<p><strong><em>a</em></strong>b</p>` | *(follows from `***a***`)* |
| **=** `**a**` | `<p><strong>a</strong></p>` | same |
| **=** `*a*` / `_a_` / `__a__` | *(the obvious)* | same |
| **=** `***a*b**` | `<p><strong><em>a</em>b</strong></p>` | same |
| **=** `***a**b*` | `<p><em><strong>a</strong>b</em></p>` | same |
| **=** `**a*b***` | `<p><strong>a<em>b</em></strong></p>` | same |
| **=** `**a *b* c**` | `<p><strong>a <em>b</em> c</strong></p>` | same |
| **=** `__a _b_ c__` | `<p><strong>a <em>b</em> c</strong></p>` | same |
| **=** `**_a_**` | `<p><strong><em>a</em></strong></p>` | same |
| **=** `**a**b**` | `<p><strong>a</strong>b**</p>` | same |
| **=** `*a*b*` | `<p><em>a</em>b*</p>` | same |
| **=** `*a [b](c) d*` | `<p><em>a <a href="c">b</a> d</em></p>` | same |
| **=** `*a ` + `` `b` `` + ` c*` | `<p><em>a <code>b</code> c</em></p>` | same |
| **=** `foo_bar_baz` | `<p>foo_bar_baz</p>` | same |
| **=** `_foo_bar_baz_` | `<p><em>foo_bar_baz</em></p>` | same |
| **=** `__foo__bar__baz__` | `<p><strong>foo__bar__baz</strong></p>` | same |

Two realistic-CV shapes, for the corpus:

```
*Lead **dev** now*
```
```
<p><em>Lead </em><em>dev</em><em> now</em></p>
```

```
**Lead *dev* now**
```
```
<p><strong>Lead <em>dev</em> now</strong></p>
```

25. Emphasis resolution is **independent of block context**: the same run inside a list item
    yields the same inline tree.

```
- *a **b** c*
- [t](u v)
```
```
<ul>
<li><em>a </em><em>b</em><em> c</em></li>
<li><a href="u v">t</a></li>
</ul>
```

---

## 8. A link destination may contain unbracketed spaces

26. `LinkInlineProcessor.RE_LINK` (`inlinepatterns.py:692`) matches only the angle-bracket form
    `(<dest> "title")` up front. Everything else falls through to the manual scanner in `getLink`
    (`inlinepatterns.py:716-830`).
27. That scanner tracks **only** parenthesis depth (`inlinepatterns.py:749-813`, starting at
    `bracket_count = 1`) and an optional quoted title. Success is `handled = bracket_count == 0`
    (`inlinepatterns.py:824`). **There is no space rule anywhere in it** — a raw space in a bare
    destination is ordinary destination text. CommonMark requires either no space or a `<…>`
    wrapper, so goldmark declines the link and leaves the source literal.
28. Balanced parentheses inside the destination are tolerated because the counter matches them
    (`inlinepatterns.py:738-757`); an unbalanced `)` closes the link.
29. The final destination is `self.unescape(href).strip()` (`inlinepatterns.py:828`), so
    **leading and trailing** whitespace is removed and interior whitespace, including a run of
    more than one space, is kept exactly.
30. A title is taken from the first quote pair inside the parens (`inlinepatterns.py:774-790`,
    `:800-808`); the destination is then the text before the opening quote, right-stripped by
    behavior 29. Either `'` or `"` opens it. The title is normalized by
    `RE_TITLE_CLEAN.sub(' ', dequote(unescape(title.strip())))` (`inlinepatterns.py:826`,
    pattern at `:693`) — every whitespace character becomes a single space, but **runs are not
    collapsed**: `"x y  z"` stays `x y  z`.
31. An unresolvable title backtracks to the last balanced `)` (`inlinepatterns.py:815-821`), so
    `[t](a"b)` is a link whose destination contains the quote.
32. The destination is **not** URL-escaped — already ported, see `link.go`'s `linkRenderer`. This
    section adds only the *matching* rule, not the serialization.
33. `ImageInlineProcessor` inherits the same `getLink`, and the port's `imageParser` already
    reproduces it. `![a](b c)` already matches; the gap is the link path only.

### 8.1 Measured shapes

| input | want (python-markdown 3.10.2) | current port (goldmark) |
|---|---|---|
| `[t](a b)` | `<p><a href="a b">t</a></p>` | `<p>[t](a b)</p>` |
| `[t](a  b)` | `<p><a href="a  b">t</a></p>` | `<p>[t](a  b)</p>` |
| `[t](a b c)` | `<p><a href="a b c">t</a></p>` | `<p>[t](a b c)</p>` |
| `[t](url (p) and s)` | `<p><a href="url (p) and s">t</a></p>` | `<p>[t](url (p) and s)</p>` |
| `[t](a b "ti")` | `<p><a href="a b" title="ti">t</a></p>` | `<p>[t](a b "ti")</p>` |
| `[t](a b 'ti')` | `<p><a href="a b" title="ti">t</a></p>` | `<p>[t](a b 'ti')</p>` |
| `[t](a b "x y  z")` | `<p><a href="a b" title="x y  z">t</a></p>` | *(declined)* |
| `[t](a"b)` | `<p><a href="a&quot;b">t</a></p>` | *(declined)* |
| `[t](a` + TAB + `b)` | `<p><a href="a   b">t</a></p>` | `<p>[t](a   b)</p>` |
| **=** `[t](<a b>)` | `<p><a href="a b">t</a></p>` | same |
| **=** `[t]( a )` | `<p><a href="a">t</a></p>` | same |
| **=** `[t](a)b)` | `<p><a href="a">t</a>b)</p>` | same |
| **=** `[t](a(b)c)` | `<p><a href="a(b)c">t</a></p>` | same |
| **=** `![a](b c)` | `<p><img alt="a" src="b c" /></p>` | same |

The tab row is a consequence of the existing `normalizeWhitespace` pass, not of a new rule: the
tab is expanded to column 4 before parsing (spec 008; `markdown/preprocessors.py:66-73`).

---

## 9. Edge cases and non-goals inside these two classes

**9.1 `___x___` is divergent, and the note claiming otherwise is wrong.** It was argued that
`___x___` matches `EM_STRONG2_RE` and so is CommonMark-identical. Measured: python-markdown emits
`<strong><em>x</em></strong>` and goldmark emits `<em><strong>x</strong></em>`. The pattern
identification was right; the conclusion was not. Same for `***x***`. Both go in §10.

**9.2 Intraword underscore is NOT a divergence.** Measured on the three discriminating shapes —
`foo_bar_baz`, `_foo_bar_baz_`, `__foo__bar__baz__` — python-markdown and the current port agree
byte for byte. CommonMark rule 17 disables intraword `_` emphasis for the same reason
python-markdown's `(?<!\w)`/`(?!\w)` guards do. No fix; behavior 24 asks for regression rows only.
The underscore shapes that *do* differ (`_a __b__ c_`, `___a_b__`, `___a__b_`, `__a_b___`) differ
because of behavior 22's index guard and behavior 20's tag order, not because of word boundaries.

**9.3 A destination spanning a line break is out of scope.** `getLink` scans the block's whole
text, so `[t](a\nb)` upstream is `<a href="a\nb">` with a literal newline in the attribute.
Matching that needs a block-level scanner; decline it, and pin the decline with an inverted
assertion the way `imageParser` already declines its multi-line form. Record it in §11, not in
`divergences.md`.

**9.4 Neither class is parity-impossible, so neither is a `divergences.md` entry.** Emphasis can
be taken over at the same trigger characters through goldmark's inline-parser registration, and
the port **already owns a faithful port of the five-pattern machinery** — the Typst path's
matchers, which measurably produce `*a **b** c*` as three sibling spans today. The link scanner is
a direct transcription of `inlinepatterns.py:716-830`. Both are engineering, and the human gate
(`AGENTS.md` §5) is for impossibility, not for expense.

**9.5 The third `knownRemainder` class stays open.** `- <div>block</div>` — a block-level tag
inside a list item — is not covered by this extension. Upstream solves it with a stash-and-restore
preprocessor pass over the raw string before block parsing; nothing prevents that, and it belongs
in its own unit.

**9.6 No upstream rendercv test covers any of this.** `third_party/rendercv/tests/renderer/
test_markdown.py` and `third_party/rendercv/tests/renderer/templater/test_markdown_parser.py`
were both read in full and neither encodes an emphasis-nesting or spaced-destination case. Every
expected string in §7.2 and §8.1 was measured by running the vendored library, per `AGENTS.md`
§10.1; none may be hand-written.

---

## 10. Acceptance criteria for §7 and §8

Mechanically checkable against `internal/renderer/templater/process/testdata/html.json`, whose
rows are generated through the submodule's own `markdown.markdown`.

- [ ] Every row of §7.2 marked **=** still matches after the emphasis work — added as ordinary
      (non-inverted) fixture rows *before* the fix, green from the start.
- [ ] `***a***` and `___a___` produce `<strong><em>a</em></strong>`.
- [ ] `*a **b** c*` produces three sibling `<em>` elements, exactly as in §7.2.
- [ ] `*a **b***` produces `<p><em>a </em><em>b</em>**</p>` — the trailing `**` survives literally.
- [ ] `_a __b__ c_` leaves the inner `__` literal.
- [ ] `___a_b__`, `___a__b_` and `__a_b___` match §7.2, including the third being wholly literal.
- [ ] The two realistic shapes `*Lead **dev** now*` and `**Lead *dev* now**` match.
- [ ] Behavior 25's list-item case matches, proving block context does not change the inline tree.
- [ ] `foo_bar_baz`, `_foo_bar_baz_`, `__foo__bar__baz__` are fixture rows and stay matching
      (behavior 24 / §9.2) — they are regression pins, not fixes.
- [ ] Every non-`=` row of §8.1 produces the `want` column.
- [ ] Every `=` row of §8.1 still matches, including `![a](b c)`, which proves the shared scanner
      did not regress the image path.
- [ ] `knownRemainder` loses `___strong em___`, `*a **bold** thing*`, `_a __b__ c_` and
      `[t](a b)`; the inverted assertion for each is removed in the same commit that fixes it,
      because an inverted assertion that starts passing is itself a test failure.
- [ ] `- <div>block</div>` stays in `knownRemainder` and still differs (§9.5).
- [ ] `[t](a\nb)` is added to `knownRemainder` with an inverted assertion (§9.3) — the decline is
      recorded, not silent.
- [ ] All 24 corpus `.html` documents stay byte-identical (spec §5), and `just test-parity` is
      green with no new skips.

## 11. Corpus additions

- [ ] A CV whose `summary` contains `***bold italic***` — the highest-reach shape, and one the
      port currently gets wrong end-to-end in a real `.html`.
- [ ] A highlight containing `*a **b** c*`.
- [ ] A highlight containing a link with a spaced destination, e.g.
      `[Best Paper Award](https://example.com/my paper.pdf)`.
- [ ] A `summary` containing `_a __b__ c_`.
- [ ] `tools/docprobe` regenerates all three artifacts for the above through the vendored Python
      (`AGENTS.md` §10.1); the `.md` must be unchanged by this work, only the `.html`.

## 12. Emphasis never sees the source — the pattern registry runs in priority order

§7 and §8 both describe a single pattern in isolation, and that framing is what let Wave C ship
six divergence classes past a green differential. The missing behavior is not in any one
pattern: it is in how python-markdown sequences all of them.

### 12.1 Where the behavior comes from

`build_inlinepatterns` registers seventeen processors with explicit priorities
(`third_party/rendercv/.venv/…/markdown/inlinepatterns.py:73-95`):

| priority | name | priority | name |
|---|---|---|---|
| 190 | `backtick` | 110 | `automail` |
| 180 | `escape` | 100 | `linebreak` |
| 170 | `reference` | 90 | `html` |
| 160 | `link` | 80 | `entity` |
| 150 | `image_link` | 70 | `not_strong` |
| 140 | `image_reference` | 60 | `em_strong` |
| 120 | `autolink` | 50 | `em_strong2` |

`treeprocessors.InlineProcessor.__handleInline` walks that registry **in order**, giving each
pattern a complete left-to-right pass over the whole block before the next pattern begins
(`markdown/treeprocessors.py:120-140`). What a pattern matches is replaced by an opaque stash
placeholder. The module's own docstring states the intent (`inlinepatterns.py:60-70`):

> backticks and escaped characters have to be handled before everything else so that we can
> preempt any markdown patterns by escaping them

**Behavior 34.** By the time `em_strong` (60) runs, a code span, an escape, a link, an image, an
autolink, an inline tag, an entity and a whitespace-flanked delimiter run are no longer the
characters the author wrote. Any port that scans the raw string must reconstruct every one of
those pre-emptions or the emphasis matchers will act on bytes upstream never showed them.

**Behavior 35.** `NOT_STRONG_RE` — `((^|(?<=\s))(\*{1,3}|_{1,3})(?=\s|$))` (`:150`) — is a
pattern at priority 70, not a guard inside emphasis. It claims a whitespace-flanked run of one
to three delimiters as literal text block-wide, so such a run can neither open nor close an
emphasis. A run of four or more matches nothing: `\*{1,3}` cannot reach the closing lookahead
past a fourth delimiter, and no shorter alternative begins at a whitespace boundary.

**Behavior 36.** A built element's text is reprocessed from `patternIndex + 1` — one pattern
*below* the one that built it — while its tail is reprocessed from `patternIndex`
(`treeprocessors.py:315-320`). This is the same cutoff rule `parse_sub_patterns` applies within
one processor (§7.1), extended across the whole registry. It is why a link label cannot contain
a link but can contain an image, and it is the general form of §7's index cutoff.

**Behavior 37.** `EscapeInlineProcessor.handleMatch` returns `None` when the escaped character
is outside `Markdown.ESCAPED_CHARS` (`inlinepatterns.py:350-360`, set at `markdown/core.py:111`
to `` \`*_{}[]()>#+-.! ``). The pattern then declines and the backslash stays literal text.
`ESCAPE_RE` is `\\(.)`, so the regex alone is not the rule.

**Behavior 38.** Every lookbehind is evaluated against the whole block, because `handleMatch`
matches with `pattern.match(data, m.start(0))` (`:663-668`). The three smart underscore patterns
each open with `(?<!\w)`; feeding a matcher a string that begins at the delimiter makes all of
them vacuous.

### 12.2 Measured shapes

Each row is `markdown.markdown` on the vendored 3.10.2. All were byte-identical to upstream
before Wave C and wrong after it, which is why they are stated here rather than in §9.

| input | upstream | class |
|---|---|---|
| `**bold ** text` | `<p>**bold ** text</p>` | 35 |
| `**Skills: ** Go` | `<p>**Skills: ** Go</p>` | 35 |
| `a ** b ** c` | `<p>a ** b ** c</p>` | 35 |
| `_a _` | `<p>_a _</p>` | 35 |
| `snake_case_` | `<p>snake_case_</p>` | 38 |
| `foo_bar_` | `<p>foo_bar_</p>` | 38 |
| ``Used *the `*` operator*`` | `<p>Used <em>the <code>*</code> operator</em></p>` | 34 |
| ``**a `**` b**`` | `<p><strong>a <code>**</code> b</strong></p>` | 34 |
| `*a<!-- * -->b*` | `<p><em>a<!-- * -->b</em></p>` | 34 |
| `**\alpha**` | `<p><strong>\alpha</strong></p>` | 37 |
| `*C:\temp*` | `<p><em>C:\temp</em></p>` | 37 |
| `[a [b](c) d](u)` | `<p><a href="u">a [b](c) d</a></p>` | 36 |
| `[[b](c)](u)` | `<p><a href="u">[b](c)</a></p>` | 36 |
| `[![i](p.png)](u)` | `<p><a href="u"><img alt="i" src="p.png" /></a></p>` | 36 |
| `[t](a\)b)` | `<p><a href="a)b">t</a></p>` | 34 |
| `[a\](b)` | `<p>[a](b)</p>` | 34 |

### 12.3 Acceptance criteria

- [x] Every row of §12.2 is a row of `testdata/html.json`, generated through the vendored
      library, and passes.
- [x] The Typst path resolves the same pre-emptions from the same mechanism, so a shape cannot be
      fixed on one path and left open on the other — the asymmetry that hid MAJOR #5.
- [x] `[![i](p.png)](u)` and `[t](<a b> "t")` are pinned in `testdata/markdown_to_typst.json`,
      closing the gap where a Typst behavior change had no fixture.

### 12.4 Two more classes the same model explains

A second fresh-context verifier swept 34,959 cases and found two more, both
new to the fix above rather than to Wave C:

**Behavior 39.** `BACKTICK_RE`'s opener is `(?<!\\)(`+)` (`inlinepatterns.py:103`) — a
backslash before the run stops it opening a span, even though `escape` (180) has not run yet.
The closing run carries no such guard, so `` `a\`b` `` is `<code>a\</code>b` ``. Ignoring the
opener guard let an escaped backtick open a span that ran to the next one and swallowed whatever
delimiter stood between: `*a \`b* c\`` is `<p><em>a `b</em> c`</p>` upstream.

**Behavior 40.** The escape round trip happens **once**. `EscapeInlineProcessor` stashes the pair
and `UnescapeTreeprocessor` restores the bare character, so `[a](b\\\\c)` — four source
backslashes, two escaped pairs — is `href="b\\c"`, two real backslashes. A port that unescapes at
parse time *and* at serialization time yields one.

**Behavior 41.** Every emphasis pattern is compiled `re.DOTALL` (`:546-552`) and matched against
the whole block, so a body may cross a soft line break: `*a\nb*` is one `<em>` spanning the
newline, in a paragraph, a list item and a blockquote alike.

### 12.5 Still open

- **A link inside an image label.** `![[b](c)](u)` is `alt="b"` upstream — `link` (160) resolves
  before `image_link` (150) and `unescape` flattens the stashed element with
  `''.join(value.itertext())` (`inlinepatterns.py:264-281`). This port keeps `alt="[b](c)"`.
  Pre-existing, unaffected by iteration 11, not reachable from any corpus document.
- **A destination or an escape spanning a line break**, e.g. `*x\`+`\n`+`y*`. Both paths are
  line-bounded; this is the same class §9.3 already declines.

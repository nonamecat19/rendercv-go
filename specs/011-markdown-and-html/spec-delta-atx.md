# Spec delta 011-A — the ATX heading rule

**Status:** proposal · **Extends:** [`spec.md`](spec.md) §6 · **Inherits:**
[`specs/000-parity-contract/spec.md`](../000-parity-contract/spec.md) · **Supersedes:** nothing

python-markdown **3.10.2** as vendored (`third_party/rendercv/.venv/lib/python3.12/site-packages/`),
goldmark **v1.8.5** as required by `go.mod:12`. Citations to `src/...` and `markdown/...` are relative
to `third_party/rendercv/` and to its resolved `.venv`; citations to `parser/...` are relative to the
vendored goldmark module.

Every `upstream` string in this document was produced by running the vendored
`markdown.markdown` and every `port` string by running `process.MarkdownToHTML` over the same input,
in one pass over 194 shapes (`AGENTS.md` §10.1). The recipes are in §8. **It proposes no Go code**
(`AGENTS.md` §4).

---

## 0. Summary

| | |
|---|---|
| **Rule 1** | python-markdown's ATX heading needs **no space after the hashes**. `#h` is `<h1>h</h1>`. |
| **Rule 2** | It also permits **no indentation before them**. ` # h` is `<p># h</p>`, where CommonMark allows up to three spaces. |
| **Reach** | **every `#`-initial line in every CV**, in both directions |
| **Direction** | rule 1: the port under-produces headings. Rule 2: the port **over**-produces them. |
| **Measured** | 194 shapes probed, **127 differ**; 122 belong to this delta, 4 to the setext delta, 1 to an unrelated blockquote finding (§9) |
| **Ordinary prose** | **15 of 15** shapes a user would write as prose change meaning (§3.2) |
| **Artifacts affected** | `.html` only. `.typ` and `.pdf` cannot reach it (§2.3) — the Typst instance deregisters both header processors |
| **Differential blindness** | **0 of the 761 `html.json` rows would change** (§5.1) |
| **Corpus blindness** | **0 of the golden `.md` lines** are in the divergent class (§5.2) |

The shape that motivates the spec is `#1 in sales`. Upstream makes that whole line — and, if the
paragraph continues, the rest of it — an `<h1>`, and drops the `#`. The port renders the prose the
user meant. **The port is right-looking and wrong**, which is the argument for a spec rather than a
patch: the fix is small, and it changes what counts as a heading in every document the tool has
produced.

**The two rules are coupled and must land in the order §7 gives.** ` #h` agrees today only because
both libraries decline it, for different reasons — upstream on the indent, goldmark on the missing
space. Relaxing the space requirement first turns ` #h` into `<h1>h</h1>` and regresses five shapes
that are green now (§3.4).

---

## 1. What upstream actually does

### 1.1 One regex

```python
# markdown/blockprocessors.py:461
RE = re.compile(r'(?:^|\n)(?P<level>#{1,6})(?P<header>(?:\\.|[^\\])*?)#*(?:\n|$)')
```

Five properties, each measured in §3:

1. **No space is required after the hashes.** Nothing separates `#{1,6}` from `header`.
2. **No space is permitted before them.** The hash run must sit immediately after `^` or a `\n`;
   there is no `[ ]{0,3}` in the expression. The anchor is the start of a line **in the block as the
   processor receives it**, so list-marker and blockquote padding — already removed upstream of this
   point — do not count (`- # h` is a heading, `-   # h` is a heading, §3.4).
3. **The level saturates at six and the remainder is text.** `#{1,6}` is greedy but capped, so
   `#######h` is `<h6>#h</h6>` — six hashes of level, one of content.
4. **The closing run `#*` is unconditional.** It needs no space in front of it: `#h#` is
   `<h1>h</h1>`. It is defeated by an escape, because `header` is `(?:\\.|[^\\])*?` and consumes
   `\#` as a unit: `#h\#` is `<h1>h#</h1>`.
5. **A heading is found anywhere in the block**, not only at its head — `test` is
   `bool(self.RE.search(block))` (`:463-464`) and `run` splits the block into `before`, the heading,
   and `after` (`:470-487`).

Property 5 is not a source of divergence on its own (§3.5): goldmark's ATX parser interrupts a
paragraph too, and every `search`-reachable shape agrees once the space and indent rules match.

### 1.2 Where it sits in the registry

```
markdown/blockprocessors.py:48   HashHeaderProcessor    'hashheader'    70
markdown/blockprocessors.py:49   SetextHeaderProcessor  'setextheader'  60
markdown/blockprocessors.py:55   ParagraphProcessor     'paragraph'     10
```

Higher priority runs first, so a chunk that matches the hash regex is a heading before it is a setext
heading and long before it is a paragraph. `#h\n===` is therefore `<h1>h</h1>` followed by a
paragraph of `===`, and not a setext heading whose text is `#h` (§3.6).

### 1.3 The text is then stripped

`h.text = m.group('header').strip()` (`:479`). Already ported — `pythonATXHeadingParser`
(`internal/renderer/templater/process/heading.go`), no delta required. It is named here because it is
why a heading's leading whitespace never shows: `#\vh` is `<h1>h</h1>` and the `\v` is the strip's
doing, not the regex's.

---

## 2. What the port does today

### 2.1 goldmark requires a space and tolerates an indent

```go
// parser/atx_heading.go:97-100
l := util.TrimLeftSpaceLength(line[i:])
if l == 0 {
    return nil, NoChildren
}
```

CommonMark 0.31 §4.2 requires the opening run to be followed by a space, a tab or the end of the
line, and permits up to three spaces of indentation before it. `TrimLeftSpaceLength` counts
goldmark's four space characters, so `#h` declines and the line falls through to the paragraph
parser. The level cap is at `:91-92` and agrees with upstream's `{1,6}`, but declines instead of
saturating: `#######h` is not a heading here at all. A lone `#` at end of input is a heading at
`:94-96`, which agrees. The closing run is handled at `:120-136` and requires a preceding
goldmark-space (`util.IsSpace(line[i])`, `:133`), where upstream requires nothing.

Indentation is not decided inside this parser — `pc.BlockOffset()` (`:83`) has already skipped up to
three spaces — so the two rules are enforced in different places even though one expression states
both upstream.

### 2.2 The port's own additions are not implicated

`pythonBlockParsers` (`html.go:131-176`) wraps six default parsers. Two touch headings
(`pythonATXHeadingParser`, `pythonSetextHeadingParser`, both in `heading.go`) and neither changes
which lines open a heading — both trim an already-parsed heading's text. **This delta's rules are
upstream of all of them.**

Note for whoever implements: `NewATXHeadingParser` **is not a singleton** — it takes options and
returns a fresh `&atxHeadingParser{}` per call (`parser/atx_heading.go:69-75`), so the wrapper is
selected by `Trigger()` and not by identity (`html.go:160-170`). An identity comparison here matched
nothing and the wrapper silently never ran.

### 2.3 The Typst path cannot reach this

```python
# src/rendercv/renderer/templater/markdown_parser.py:150-151
md.parser.blockprocessors.deregister("hashheader")
md.parser.blockprocessors.deregister("setextheader")
```

The Typst `Markdown` instance (`:147`) removes both header processors, so `#h` is prose in the `.typ`
output by construction, and in the PDF and PNG with it. The HTML instance is plain
`markdown.markdown(markdown_string)` with no extensions (`:202`). **So this is an `.html`-only
divergence**, and parity axis 1 is engaged for exactly one artifact.

---

## 3. The divergent set, measured

194 shapes probed (§8.1); **127 differ**, of which 122 are this delta's. Levels 1–8 were swept
against eight following characters, indents 0–4 against levels 1 and 3, and every shape below was run
through both libraries.

| Class | Diverging shapes |
|---|---|
| no space after the run (§3.1, §3.2, §3.5) | 74 |
| level saturation above six (§3.3) | 24 |
| indentation before the run (§3.4) | 12 |
| the closing run and its escape (§3.6) | 8 |
| ATX versus setext priority (§3.7) | 4 |

### 3.1 Rule 1 — no space after the hashes

| Input | upstream | port |
|---|---|---|
| `#h` | `<h1>h</h1>` | `<p>#h</p>` |
| `##h` … `######h` | `<h2>h</h2>` … `<h6>h</h6>` | `<p>##h</p>` … `<p>######h</p>` |
| `#1` | `<h1>1</h1>` | `<p>#1</p>` |
| `#!` | `<h1>!</h1>` | `<p>#!</p>` |
| `#-` | `<h1>-</h1>` | `<p>#-</p>` |
| `#.` | `<h1>.</h1>` | `<p>#.</p>` |
| `#(` | `<h1>(</h1>` | `<p>#(</p>` |
| `#_` | `<h1>_</h1>` | `<p>#_</p>` |
| `#\v` | `<h1></h1>` | `<p>#\v</p>` |
| `- #h` | `<ul>\n<li>\n<h1>h</h1>\n</li>\n</ul>` | `<ul>\n<li>#h</li>\n</ul>` |
| `1. #h` | `<ol>\n<li>\n<h1>h</h1>\n</li>\n</ol>` | `<ol>\n<li>#h</li>\n</ol>` |
| `> #h` | `<blockquote>\n<h1>h</h1>\n</blockquote>` | `<blockquote>\n<p>#h</p>\n</blockquote>` |

The sweep found no following character that separates the two libraries other than by being or not
being one of goldmark's four spaces. `#\th` and `#  h` agree; every non-space follower diverges at
every level 1–6.

**Agreeing controls**, each measured: `#`, `# `, `#\n`, `## `, `###### ` (all `<h_n_></h_n_>`),
`\# h` and `\#h` (escape defeats both), `    #h` and `\t# h` (indented code wins in both).

### 3.2 Ordinary CV prose — the reach question

**15 of 15 measured prose shapes change meaning.** None involves a whitespace character, an escape,
or anything a user would recognise as Markdown.

| Input | upstream | port |
|---|---|---|
| `#1 in sales` | `<h1>1 in sales</h1>` | `<p>#1 in sales</p>` |
| `#1 in sales\n` | `<h1>1 in sales</h1>` | `<p>#1 in sales</p>` |
| `#1 ranked team` | `<h1>1 ranked team</h1>` | `<p>#1 ranked team</p>` |
| `#2 of 300 applicants` | `<h1>2 of 300 applicants</h1>` | `<p>#2 of 300 applicants</p>` |
| `#hashtag` | `<h1>hashtag</h1>` | `<p>#hashtag</p>` |
| `#TeamWork` | `<h1>TeamWork</h1>` | `<p>#TeamWork</p>` |
| `#tag and #tag2` | `<h1>tag and #tag2</h1>` | `<p>#tag and #tag2</p>` |
| `#include <stdio.h>` | `<h1>include <stdio.h></h1>` | `<p>#include &lt;stdio.h&gt;</p>` |
| `#!/bin/sh` | `<h1>!/bin/sh</h1>` | `<p>#!/bin/sh</p>` |
| `#define X 1` | `<h1>define X 1</h1>` | `<p>#define X 1</p>` |
| `#FF00AA is the brand colour` | `<h1>FF00AA is the brand colour</h1>` | `<p>#FF00AA is the brand colour</p>` |
| `#1 in sales\nand more prose` | `<h1>1 in sales</h1>\n<p>and more prose</p>` | `<p>#1 in sales\nand more prose</p>` |
| `#1 in sales -- led the team\n\nnext paragraph` | `<h1>1 in sales -- led the team</h1>\n<p>next paragraph</p>` | `<p>#1 in sales -- led the team</p>\n<p>next paragraph</p>` |
| `#1 in sales\n===` | `<h1>1 in sales</h1>\n<p>===</p>` | `<h1>#1 in sales</h1>` |
| `- #1 in sales` | `<ul>\n<li>\n<h1>1 in sales</h1>\n</li>\n</ul>` | `<ul>\n<li>#1 in sales</li>\n</ul>` |

Three observations, in descending order of how much they matter:

1. **Upstream escapes an entity the port emits.** `#include <stdio.h>` keeps its literal `<stdio.h>`
   in the heading because python-markdown treats it as inline HTML, while the port's paragraph
   escapes it to `&lt;stdio.h&gt;`. So the fix changes more than a tag name on this row.
2. **The hashes are consumed.** `#1 in sales` loses its `#` and becomes the heading `1 in sales`. A
   fix changes the characters, not only the element.
3. **Not affected**, each measured: `Ranked #1 in sales`, `C# and .NET`, `issue #42`, `a #b` — the
   hash is not at a line start; and `\#1 in sales`, where the escape defeats both.

### 3.3 Level saturation

| Input | upstream | port |
|---|---|---|
| `#######h` | `<h6>#h</h6>` | `<p>#######h</p>` |
| `########h` | `<h6>##h</h6>` | `<p>########h</p>` |
| `####### h` | `<h6># h</h6>` | `<p>####### h</p>` |
| `####### ` | `<h6>#</h6>` | `<p>####### </p>` |
| `#######` | `<h6></h6>` | `<p>#######</p>` |
| `####### h #######` | `<h6># h</h6>` | `<p>####### h #######</p>` |
| `########\th` | `<h6>##    h</h6>` | `<p>########    h</p>` |

The last row is `NormalizeWhitespace`'s `source.expandtabs(self.md.tab_length)`
(`markdown/preprocessors.py:73`) showing through, not a heading rule; the port already expands tabs
the same way, which is why only the element differs.

### 3.4 Rule 2 — indentation before the hashes

**This class runs the other way: the port produces headings upstream does not.** It is a divergence
today, independent of rule 1.

| Input | upstream | port |
|---|---|---|
| ` # h` | `<p># h</p>` | `<h1>h</h1>` |
| `  # h` | `<p># h</p>` | `<h1>h</h1>` |
| `   # h` | `<p># h</p>` | `<h1>h</h1>` |
| ` ### h` … `   ### h` | `<p>### h</p>` | `<h3>h</h3>` |
| ` # h #` | `<p># h #</p>` | `<h1>h</h1>` |
| `a\n\n # h` | `<p>a</p>\n<p># h</p>` | `<p>a</p>\n<h1>h</h1>` |
| `a\n\n  ## h` | `<p>a</p>\n<p>## h</p>` | `<p>a</p>\n<h2>h</h2>` |
| `a\n # h\nb` | `<p>a\n # h\nb</p>` | `<p>a</p>\n<h1>h</h1>\n<p>b</p>` |
| `>  # h` | `<blockquote>\n<p># h</p>\n</blockquote>` | `<blockquote>\n<h1>h</h1>\n</blockquote>` |
| `- item\n  # h` | `<ul>\n<li>item\n  # h</li>\n</ul>` | `<ul>\n<li>item\n<h1>h</h1>\n</li>\n</ul>` |
| `- item\n\n  # h` | `<ul>\n<li>item</li>\n</ul>\n<p># h</p>` | `<ul>\n<li>item</li>\n</ul>\n<h1>h</h1>` |
| ` # h\n===` | `<h1># h</h1>` | `<h1>h</h1>\n<p>===</p>` |

The indent is measured **inside the block**, not in the document: `- # h`, `-   # h`, `1. # h` and
`   1. # h` all agree and are headings in both, because the marker and its padding are gone before
the processor sees the line. `> # h` agrees for the same reason; `>  # h` does not, because the
blockquote strips exactly one space (`markdown/blockprocessors.py:287`, `[ ]?`).

**The coupling.** These five shapes are green today and would go red if rule 1 were relaxed alone:

```
 #h        <p>#h</p>
  #h       <p>#h</p>
   #h      <p>#h</p>
 #1 in sales      <p>#1 in sales</p>
  #1 in sales     <p>#1 in sales</p>
```

Both libraries decline them, for opposite reasons. Rule 2 therefore lands first (§7).

### 3.5 Position in the document

| Input | upstream | port |
|---|---|---|
| `a\n#h` | `<p>a</p>\n<h1>h</h1>` | `<p>a\n#h</p>` |
| `a\n\n#h` | `<p>a</p>\n<h1>h</h1>` | `<p>a</p>\n<p>#h</p>` |
| `#h\nb` | `<h1>h</h1>\n<p>b</p>` | `<p>#h\nb</p>` |
| `a\n#h\nb` | `<p>a</p>\n<h1>h</h1>\n<p>b</p>` | `<p>a\n#h\nb</p>` |
| `> q\n#h` | `<blockquote>\n<p>q</p>\n</blockquote>\n<h1>h</h1>` | `<blockquote>\n<p>q\n#h</p>\n</blockquote>` |
| `- item\n#h` | `<ul>\n<li>item</li>\n</ul>\n<h1>h</h1>` | `<ul>\n<li>item\n#h</li>\n</ul>` |

**`search`-reachability is not itself a divergence.** With a space present, every one of these
agrees: `a\n# h\nb`, `- item\n# h`, `> q\n# h`, `a\n\n- item\n# h` are byte-identical between the two
libraries, including goldmark ending the list to start the heading. So a fix that relaxes the space
requirement gets this section for free; nothing here needs the `before`/`after` splitting of
`:470-487` modelled separately.

### 3.6 The closing run and the escape

| Input | upstream | port |
|---|---|---|
| `# h#` | `<h1>h</h1>` | `<h1>h#</h1>` |
| `# h##` | `<h1>h</h1>` | `<h1>h##</h1>` |
| `# h#\n` | `<h1>h</h1>` | `<h1>h#</h1>` |
| `# h\v#` | `<h1>h</h1>` | `<h1>h\v#</h1>` |
| `#h#` | `<h1>h</h1>` | `<p>#h#</p>` |
| `#h #` | `<h1>h</h1>` | `<p>#h #</p>` |
| `#h#h#` | `<h1>h#h</h1>` | `<p>#h#h#</p>` |
| `#h\#` | `<h1>h#</h1>` | `<p>#h#</p>` |

`# h#`, `# h##`, `# h#\n` and `# h\v#` are the **closing-hash-run** item queued separately as 27
shapes. It is the same expression: goldmark needs a goldmark-space before the run
(`parser/atx_heading.go:133`), upstream's `#*` needs only to reach the line end. §9 argues it be
folded in rather than kept apart. `# h #` and `## h ##` agree — the space is present, so both trim.

### 3.7 ATX versus setext priority

| Input | upstream | port |
|---|---|---|
| `#h\n===` | `<h1>h</h1>\n<p>===</p>` | `<h1>#h</h1>` |
| `#h\n---` | `<h1>h</h1>\n<hr />` | `<h2>#h</h2>` |
| `#1 in sales\n===` | `<h1>1 in sales</h1>\n<p>===</p>` | `<h1>#1 in sales</h1>` |
| ` # h\n===` | `<h1># h</h1>` | `<h1>h</h1>\n<p>===</p>` |

All four are decided by the two rules above, not by anything in the setext processor: once the ATX
parser opens (or declines to open) on the first line, the bar has nothing to attach to. `a\n===`
and `a\n\nb\n===` agree already. **These rows belong to this delta**, even though a bar appears in
them.

---

## 4. Reachability from an ordinary CV

Demonstrated on `testdata/golden/theme_classic/files/rendercv_output/John_Doe_CV.md` — the port's own
generated corpus document — by putting the text a user would write into the field that carries it and
running the vendored `markdown_to_html` over the whole document (§8.2).

| Field | The line in the generated `.md` | Upstream `.html` |
|---|---|---|
| a summary (line 11, column 1) | `#1 in sales -- RenderCV reads a CV written in a YAML file, …` | `<h1>1 in sales -- RenderCV reads a CV written in a YAML file, and generates a PDF with professional typography.</h1>` |
| a highlight | `- #1 in sales` | `<li>\n<h1>1 in sales</h1>\n</li>` |

The summary case swallows the **entire paragraph** into the heading, because `header` is
`(?:\\.|[^\\])*?` and `[^\\]` matches a newline; the match ends at the first position where `#*`
meets a line end. This is the largest single artifact difference this delta can produce.

The pipeline is `render_markdown` → `.md` file → `markdown_to_html` → `.html`
(`src/rendercv/renderer/html.py:33`, `src/rendercv/renderer/templater/templater.py:131,153`), so
**any** user string that reaches a line start in the generated Markdown reaches this rule. Section
titles and entry headers are emitted by the templates with a space after the hashes and at column 0,
and are unaffected by either rule.

---

## 5. Blast radius

### 5.1 The differential is blind to it

Of the 761 rows in `internal/renderer/templater/process/testdata/html.json`: 67 contain a `#`, 58 have
one at a line start, and

- **0** have a hash run followed by anything but a space, a hash or a line end;
- **0** have a hash run preceded by one to three spaces;
- **0** have a closing run without a space before it.

**Every existing row would be byte-identical after the fix.** That is a statement about the fixture,
not about the code: the corpus was grown from whitespace and emphasis work and has never probed this
axis.

### 5.2 The golden corpus is blind to it too

Across every `.md` under `testdata/golden/`, **0 lines** are in either divergent class. The 23
`#`-initial lines in a generated `.md` are template-emitted section and entry headers, all at column
0 with a space. `just test-parity` cannot see this class.

**Both gates being green is therefore not evidence.**

### 5.3 What a fix must not disturb

The heading and paragraph unit families in this package — `heading_test.go`'s ATX and setext strips,
`paragraph_test.go`'s throw-away-and-`lstrip` including its setext-bar guard — all assume the current
*set* of headings. Changing which lines open a heading changes their inputs. `"\v\n==="`
(`<h1></h1>`) and `"#h\n==="` (`<h1>h</h1>\n<p>===</p>`) are where the heading rules and the
paragraph rules meet.

---

## 6. Options

| | Approach | Cost | Risk |
|---|---|---|---|
| A | **Wrap goldmark's ATX parser** — decline on an indent, open where goldmark declines for want of a space, saturate the level, and re-cut the closing run | medium: the level cap, the closing run and the escape all have to be reproduced, and `Open` owns the reader position | contained: one parser, and `pythonATXHeadingParser` is already the wrapper |
| B | **Rewrite the source**, inserting a space after a hash run at a line start | small | wrong: it changes the text, and it cannot tell a heading from an indented code block without parsing |
| C | **Port the regex** and replace the ATX parser wholesale with one driven by `RE` | medium-large | inherits `search`-anywhere and the `before`/`after` splitting, which §3.5 shows is not needed |
| D | Record as a divergence in `divergences.md` | none | **not available**: parity is achievable, and §4 shows the divergence is user-visible |

**Recommendation: A.** §3.5 is what makes it sufficient — the shapes that would have forced C all
agree once the space and indent rules match. A keeps the existing strip wrapper as the single owner
of ATX behavior in the port.

Rule 2 (§3.4) may not be expressible inside `Open`, because `pc.BlockOffset()` has already consumed
the indent by then; the implementer should expect to read the raw line to count it. That is a design
question for `plan.md`, not a spec one.

---

## 7. Tasks

Each is one commit, fixtures first and red under
`go test -tags conformance ./internal/renderer/templater/process/` (`AGENTS.md` §7, §10.1). Fixture
rows are generated by `go run ./tools/mdprobe -add … -write`, never typed.

| # | Task | Parallel? | Done when |
|---|---|---|---|
| A1 | `html.json` gains the §3.4 indentation rows | sequential | rows reproduce under `mdprobe`, red |
| A2 | An indented hash run does not open a heading | sequential | §3.4 green; the 5 coupled shapes still green; 0 of the pre-existing 761 rows move |
| A3 | `html.json` gains the §3.1, §3.2 and §3.5 rows | sequential | red |
| A4 | A heading opens with no space after the hashes | sequential | §3.1, §3.2 green; §3.5 and §3.7 green with no further change; 0 of the 761 move |
| A5 | `html.json` gains the §3.3 saturation rows | parallel with A7 | red |
| A6 | The level saturates at six, the remainder is text | sequential after A5 | §3.3 green |
| A7 | `html.json` gains the §3.6 closing-run and escape rows | parallel with A5 | red |
| A8 | The closing run needs no preceding space; an escaped hash is text | sequential after A7 | §3.6 green; **closes the separately queued 27-shape item** |
| A9 | A unit test sweeping levels 1–8, indents 0–4 and the §3.2 prose set | parallel | table-driven, every `want` measured |

**A1–A2 come before A3–A4 and that order is load-bearing** (§3.4): relaxing the space rule while
goldmark still tolerates an indent turns five green shapes red. A6 and A8 are corrections within a
shape that is already a heading and may be dropped without leaving the tree inconsistent; stopping
after A4 is a clean stopping point.

---

## 8. Reproduction recipes

### 8.1 The shape set

Upstream, from `/home/nnc/Projects/rendercv-go/third_party/rendercv`:

```bash
uv run python - <<'EOF'
import json, markdown
shapes = json.load(open("/path/to/shapes.json"))
print(json.dumps({s: markdown.markdown(s) for s in shapes}, ensure_ascii=False))
EOF
```

Port, from the repository root, over the same `shapes.json`, calling
`process.MarkdownToHTML` on each entry. The 194 shapes were levels 1–8 × the followers
`h 1 ! - . ( _ \v \t <space> <none>`, indents 0–4 × levels 1 and 3 × {with space, without}, the ten
position shapes of §3.5, the closing-run set of §3.6, the fifteen prose shapes of §3.2 with their
five controls, and the setext-adjacent shapes of §3.7 and §9.

### 8.2 Reachability

```bash
uv run python - <<'EOF'
import markdown, re
p = "<repo>/testdata/golden/theme_classic/files/rendercv_output/John_Doe_CV.md"
src = open(p).read()
anchor = "RenderCV reads a CV written in a YAML file"
out = markdown.markdown(src.replace(anchor, "#1 in sales -- " + anchor, 1))
print([h for h in re.findall(r"<h[1-6]>.*?</h[1-6]>", out, re.S) if "sales" in h])
EOF
```

### 8.3 The differential's blindness

```bash
python3 - <<'EOF'
import json, re
rows = json.load(open("internal/renderer/templater/process/testdata/html.json"))
print(sum(1 for r in rows if re.search(r"(?:^|\n)#{1,6}(?![ #\n])", r["In"])), "no-space")
print(sum(1 for r in rows if re.search(r"(?:^|\n)[ ]{1,3}#{1,6}", r["In"])), "indented")
print(len(rows), "rows")
EOF
```

---

## 9. Adjacent findings, and what belongs where

**In this delta**, because they are one expression and one parser:

- **The closing-hash run (27 shapes)**, queued separately. §3.6 measures it as part of `RE`; tasks
  A7–A8 fold it in. Keeping it apart would mean two units editing one `Open`.
- **The ATX-versus-setext rows** of §3.7. They read as setext shapes and are decided by ATX.

**Not in this delta** — each is a different processor, a different fixture batch, and shares no code
path with the ATX parser:

- **A multi-line paragraph becoming a setext heading**: `a\nb\n===` is `<p>a\nb\n===</p>` upstream and
  `<h1>a\nb</h1>` here. `SetextHeaderProcessor.RE` is
  `r'^.*?\n[=-]+[ ]*(\n|$)'` matched with `.match` (`markdown/blockprocessors.py:497,501`), so `.`
  never crosses a newline and upstream only ever sees a bar under line 2. **Own delta.**
- **A spaces-only first line under a bar**: `" \n==="` and `"  \n==="` are `<h1></h1>` upstream and
  `<p>===</p>` here. `NormalizeWhitespace`'s `re.sub(r'(?<=\n) +\n', '\n', source)`
  (`markdown/preprocessors.py:74`) has a **lookbehind**, so it never empties a document's *first*
  line; goldmark calls that line blank and opens no paragraph. `"\n==="` and `" \n\n==="` agree.
  **Same delta as the previous item** — both are the setext path, and both change what the paragraph
  parser hands over.
- **The tab-expanded first line** (`"\t\n==="`), already a named skip in `paragraph_test.go`.
- **Blockquote merging**, found while sweeping §3.4: `> q\n\n> # h` is one `<blockquote>` upstream and
  two here. Nothing to do with headings; **needs its own investigation.**

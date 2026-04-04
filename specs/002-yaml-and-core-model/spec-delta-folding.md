# Spec delta 002-F — costing the multi-line plain scalar workaround

Extends [`spec.md`](spec.md). Nothing here supersedes it. Written against
`internal/schema/yamlreader` at `6b284ff`, goccy/go-yaml **v1.19.1** as vendored in `go.mod`,
ruamel via the pinned `third_party/rendercv` submodule.

**This document implements nothing and proposes no code.** It exists because the human has to
choose between three options for a class of documents upstream accepts and the port rejects —
(a) a divergence entry, (b) an upstream issue against goccy with a pin here, (c) a workaround in
the reader — and (c) had never been costed, so the choice was being made blind on its most
important option.

Every string below was produced by running the vendored library, per `AGENTS.md` §10.1. The
recipes are in §7.

---

## 0. Summary, and the recommendation

| Question | Answer |
|---|---|
| Where would a workaround live? | Between `lexer.Tokenize` and `parser.Parse` in `yamlreader/build.go`, the seam `Dealias` already occupies. §2 |
| Is pre-folding the text possible? | **The question is the wrong shape.** goccy's lexer *already folds* these scalars. Two narrower defects sit on top of it. §3 |
| Blast radius? | Every document goes through the path; a wrong fold corrupts a valid CV silently, which is worse than rejecting it. Gateable, but not by the corpus I have. §4 |
| Which mechanisms does it fix? | The folded plain scalar in block **and** flow context — one mechanism, two contexts. **Not** the empty-block-scalar class, **not** collection tags. §5 |

> **Recommendation: (b), an upstream issue against goccy plus a pin here — not (c).**
>
> The reasoning is in §6 and it is not the one I expected going in. A workaround is *cheaper* than
> feared, because goccy's lexer already does the folding correctly. But what is left is two
> position-and-tokenisation defects inside goccy's lexer, and compensating for them from outside
> means re-deriving where each plain scalar truly ends — the same scanner reconstruction that
> `blockscan.go` needed 358 lines, 170,003 measured documents and 26 residual wrong answers to do
> for *error messages*. A fold changes **values**, so "26 residual" is not an acceptable resting
> place. Upstream is also the correct venue on the merits: ruamel is right and goccy is wrong
> against the YAML spec on both defects, and both are small and crisply reproducible (§3.4).

**§3 also records a defect nobody has logged: folded plain scalars already carry the wrong source
coordinates in documents the port parses successfully today.** That is validation-error parity
(axis 4) and it is live now, independent of any rejection.

---

## 1. The class

Two mechanisms reach the same symptom — a document ruamel loads and the port refuses. They are
unrelated and only one is about folding.

### 1.1 Mechanism F — a plain scalar folded across an over-indented dash

Minimal, measured:

```yaml
k: 1
  - item
 q
```

ruamel loads `{"k": "1 - item q"}`. It folds all three lines into **one plain scalar**, sequence
dash and all, because a plain scalar continues onto every following line indented past the block
level. The port rejects the file.

Realistic shape, nested, measured — a user who over-indents a list under a key:

```yaml
cv:
  name: John
    - a
   b
```

ruamel loads `{"cv": {"name": "John - a b"}}` and renders a PDF. The port refuses the document.

**The fold is context-dependent, and one character decides it.** Measured:

| Document | ruamel |
|---|---|
| `k: 1\n  - item\n q\n` | `{"k": "1 - item q"}` — folds |
| `k:\n  - item\n q\n` | **error**, `while parsing a block mapping` |
| `k:\n  - item\n` | `{"k": ["item"]}` — a real sequence |
| `k: 1\n  - item\nq\n` | **error**, `while scanning a simple key` |

Whether `- item` is sequence syntax or scalar text depends on whether line 1 already carried an
inline value; whether the third line continues the scalar or ends the document depends on its
column against the block indent. No lexical rule over those three lines can tell them apart.

### 1.2 Mechanism C — an empty block scalar closed by a comment

Minimal, measured:

```yaml
k: >

# c
```

ruamel loads `{"k": ""}`. The port rejects with `[3:1] non-map value is specified.`

**The blank line is load-bearing.** `k: >\n# c\n` is accepted by the port, `k: >\n\n` is accepted,
`k: >\n\nx: 1\n` is accepted. It needs all three of block header, blank line, comment. Holds for
`>`, `|`, `>-`, `|-` at every indentation probed, comment indented or not.

No plain scalar is involved, so **§3–§6 do not apply to this mechanism at all.**

### 1.3 Sizes

The seven documents originally reported were an artifact: the sweep that found them was built to
enumerate *error* shapes, so valid documents entered it by accident. A targeted 380-document grid
over the two minimal shapes found **160** more:

| | count in the 380-document grid |
|---|---|
| Mechanism F | 120 |
| Mechanism C | 40 |

Neither number is a population estimate. Both mechanisms are parameterised by indentation and
scalar kind and are unbounded in principle.

---

## 2. Where a workaround would have to live

### 2.1 Correcting the premise: `yamlreader` does not absorb the 172

The brief for this investigation said `yamlreader` already absorbs 172 of the 179 valid-but-
rejected documents, and treated that as a precedent. **It is not one.** Measured over those 179:

| parse path | accepts |
|---|---|
| `parser.ParseBytes(src, parser.ParseComments)` | 0 |
| `parser.Parse(lexer.Tokenize(src), 0)` | 172 |
| `parser.Parse(Dealias(lexer.Tokenize(src)), 0)` | 172 |
| `yamlreader.ReadString` | 172 |

The 172 are absorbed by **goccy's own parse mode**, not by anything the port wrote. My probe used
`ParseComments`, the reader does not, and that difference is the entire gap. `Dealias` absorbs
none of them. There is no existing workaround for this class to extend.

### 2.2 The real precedent, and the bar it sets

`parseTolerantOfQuotedTabs` (`build.go:138-168`) is the port's one repair-and-retry workaround. It
parses; on a *specific* recognised goccy message it rewrites the one line goccy named and reparses,
bounded by the line count. It is safe for four reasons worth naming, because they are the bar:

1. it fires only on one recognised error substring;
2. it only rewrites a line goccy itself pointed at;
3. the rewrite is **value-preserving by construction** — leading whitespace inside a quoted scalar
   is folded away, so tab-to-space cannot change the loaded value;
4. it replaces tabs with spaces **one for one, so every later column is unchanged** and error
   coordinates stay exact.

A text-level fold fails (3) and (4) outright: it changes what the value is by deleting newlines,
and it destroys the line/column mapping for everything after the fold. `coords_test.go` and the
whole of parity axis 4 depend on that mapping.

### 2.3 The seam that could work

`Dealias` (`noalias.go`) already rewrites the **token stream** between `lexer.Tokenize` and
`parser.Parse`, and already merges tokens — it joins an alias token with the `String` tokens
following it on the same line. Tokens carry their own positions, so a merge there can keep the
first token's position and leave every other token untouched. That is the only seam where a fold
could preserve coordinates.

---

## 3. Is pre-folding possible? The lexer already folds

This is where the investigation turned over. **goccy's lexer already performs the fold**, correctly,
including the sequence dash. Token dumps, measured:

```
lexer.Tokenize("k: 1\n  - item\n q\n")
  String        "k"          line 1 col 1
  MappingValue  ":"          line 1 col 2
  String        "1 - item"   line 1 col 4     <- folded, correct
  String        "q"          line 3 col 2     <- not folded in

lexer.Tokenize("k:\n  - item\n q\n")
  String        "k"          line 1 col 1
  MappingValue  ":"          line 1 col 2
  SequenceEntry "-"          line 2 col 3     <- correctly NOT folded
  String        "item"       line 2 col 5
  String        "q"          line 3 col 2
```

The lexer makes the fold-versus-sequence distinction of §1.1 correctly, from indentation state it
already tracks. So the expensive part — knowing whether `- item` is syntax or text — **is already
solved inside goccy.** What remains is two narrower defects.

### 3.1 Defect A — the fold stops one line early

In the first dump the third line becomes its own `String` token instead of joining the scalar.
Worse, when the continuation line looks like a quoted scalar, goccy lexes it as one and **strips
the quotes**, while ruamel keeps them as literal characters:

```
lexer.Tokenize("k: ~\n      - item\n    \"quoted\"\n")
  String        "k"              line 1 col 1
  MappingValue  ":"              line 1 col 2
  String        "~ - item"       line 1 col 4
  DoubleQuote   "quoted"         line 3 col 5     <- quotes stripped
```

ruamel loads `{"k": "~ - item \"quoted\""}` — quote characters and all. A token merge therefore
cannot concatenate `Value`s; it has to fall back to each token's `Origin` and reconstruct the raw
text. Doable, but it means the merge is not a merge, it is a re-lex.

### 3.2 Defect B — the folded token is positioned at the end of the fold

```
lexer.Tokenize("k: 1\n  - item\n")
  String        "1 - item"   line 2 col 9      <- end of the fold
lexer.Tokenize("k: 1\n  - item\n\n")
  String        "1 - item"   line 3 col 1      <- the blank line
lexer.Tokenize("k: 1\n  - item\n\n\n")
  String        "1 - item"   line 4 col 1
```

ruamel puts the value at line 1 column 4, the scalar's **start** (`d.lc.data["k"] == [0, 0, 0, 3]`,
0-based). goccy puts it wherever the fold happened to stop, which drifts with trailing blank lines.

This is what rejects one of the seven outright. `   k: 1\n      - item\n\n` puts the folded token at
line 3 column 1, which is left of the key at column 4, and the parser rejects it on column grounds:
`[3:1] value is not allowed in this context`. The identical document with the key at column 1
parses. The rejection is **a positioning bug, not a folding gap.**

### 3.3 A live defect nobody has logged

Defect B is not confined to documents that fail. `k: 1\n  - item\n` **parses** in the port today,
and the value's coordinate is line 2 column 9 where upstream's is line 1 column 4. Any validation
error reported against a folded plain scalar therefore points at the wrong place right now — parity
axis 4, live, in documents that render.

I have not sized this: it needs a differential of *coordinates* over documents that parse, which
neither gate currently runs. It is called out here because it is the same root cause and it should
not be discovered separately later.

### 3.4 Both defects are goccy's, and both are small

ruamel is right and goccy is wrong against the YAML spec in both cases — a plain scalar continues
onto more-indented lines, and its mark is its start. Both reproduce in four lines with no rendercv
context, which makes them good upstream issues:

```go
// defect A
lexer.Tokenize("k: 1\n  - item\n q\n")  // want one scalar "1 - item q"
// defect B
lexer.Tokenize("k: 1\n  - item\n")      // want the token at line 1 col 4
```

---

## 4. Blast radius, and whether it can be gated

**The asymmetry is the whole risk.** A wrong error-phrasing rule produces a wrong message on a
document that was already failing. A wrong fold produces a **wrong value on a document that
succeeds** — a CV that renders with the wrong name in it, at exit 0, with nothing to notice.
`blockscan.go` is allowed 26 residual wrong answers out of 82,418 because they are messages. The
same residual rate on values would be unshippable.

**A naive token merge is not viable, and here is the number.** The obvious predicate — merge
adjacent `String` tokens where the second is not followed by `:` — was measured against the 179
valid documents and the 65 real YAML files in the vendored submodule:

| corpus | documents | with a merge candidate |
|---|---|---|
| valid-but-rejected (179) | 179 | 2 |
| real upstream YAML | 65 | 0 |

It fires on 2 of the 179 documents it is meant to fix, because the continuation token is often
`DoubleQuote`, `SingleQuote` or `Null` rather than `String` (§3.1). Widening the predicate to every
scalar-ish token widens it toward exactly the false positives that corrupt values. The zero on real
upstream YAML is reassuring about the narrow predicate and says nothing about a wide one.

**Gating.** A fold *can* be verified against ruamel across a corpus, and the design is
straightforward: load each document with ruamel and with the port, canonicalise both to JSON, and
diff **values**, not accept/reject. The 170,003 documents enumerated for `blockscan.go` could be
re-run through it.

**But that corpus is the wrong shape for this job, and I want that on the record.** It was generated
to enumerate *error* shapes; 150,791 of the 170,003 fail to parse. A value-corrupting fold does its
damage on documents that **succeed**, which this corpus under-samples by construction. Gating a fold
honestly needs a differently generated corpus of valid documents — round-trip generated CVs, the
golden corpus, permuted real inputs — and nobody has built one. That corpus is a prerequisite for
option (c), not a detail of it, and it is plausibly larger than the fold itself.

---

## 5. What a fold would and would not fix

| Mechanism | Fixed by folding? |
|---|---|
| Folded plain scalar, **block** context (§1.1) | Yes — this is the mechanism |
| Folded plain scalar, **flow** context (the standing blocked class) | Yes — same mechanism, different context |
| Empty block scalar closed by a comment (§1.2) | **No.** No plain scalar is involved; the failure is in block-scalar termination |
| Collection tag on a scalar | **No**, by construction — no plain scalar, no folding. Not re-measured here |
| Sequence keys | **No**, by construction. Not re-measured here |

Confirmed as asked. The two mechanisms in §1 are genuinely independent: they share a symptom and
nothing else, and a fold leaves §1.2 exactly where it is.

The one piece of good news for the human's arithmetic stands: **flow-context and block-context
folded plain scalars are one mechanism seen twice**, so a single decision covers both.

---

## 6. Recommendation

**Take (b): an upstream issue against goccy/go-yaml, with a pinned test here, and a divergence entry
for as long as the pin stands.** Do not take (c).

The reasoning, in the order it changed my mind:

1. **(c) is cheaper than it looked** — goccy already folds, so a workaround does not have to
   reimplement a YAML scanner from scratch. I expected to recommend against it on that ground and
   cannot.
2. **But what is left is inside goccy's lexer, not outside it.** Defects A and B are a
   tokenisation error and a position error. Compensating from the token stream means re-deriving
   the fold's true extent from the source — which is the scanner reconstruction `blockscan.go`
   already did once, at 358 lines and 170,003 measured documents, and which still has 26 residual
   wrong answers. That residual is tolerable for messages and not for values (§4).
3. **The gating corpus does not exist.** Option (c) cannot be verified with what we have; it needs
   a corpus of *valid* documents that nobody has built (§4). That prerequisite is plausibly larger
   than the fold.
4. **Upstream is right on the merits and the repros are four lines** (§3.4). This is not a case
   where the port has to compensate for a design difference; goccy is wrong against the spec in two
   small, isolated ways.
5. **The divergence is narrow and visible.** It needs a plain scalar value followed by a
   more-indented line — accidental over-indentation of a list. The port fails loudly at exit 1. The
   failure mode of (c) done imperfectly is silent value corruption at exit 0.

If the human wants relief before upstream moves, the *narrow* form of (c) is defensible on its own:
Defect B alone, at the `Dealias` seam, resetting a folded scalar token's position to the start of
its fold. It is value-preserving by construction (it moves a mark, it does not change text), it
addresses the §3.3 live coordinate defect, and it fixes the one document of the seven that fails
purely on column comparison. That is a much smaller decision than folding and could be scoped
separately. **I am not proposing it as work here; I am recording that it exists so the human is not
offered a false all-or-nothing.**

---

## 7. Method

Every measurement is reproducible with the vendored submodule.

```bash
# ruamel's verdict and loaded value for a document
cd third_party/rendercv && uv run python -c '
import json, ruamel.yaml
from rendercv.schema.yaml_reader import read_yaml
d = read_yaml("k: 1\n  - item\n q\n"); print(json.dumps(dict(d)), d.lc.data)'

# goccy's token stream at the Dealias seam
lexer.Tokenize(src)                      // github.com/goccy/go-yaml/lexer

# the port's verdict
yamlreader.ReadString(src)
modelbuilder.ReadYamlWithValidationErrors(src, schemaerr.SourceMain)
```

- §1.3's 380-document grid: key at column 1 with values `~`, `1`, `'v'`, empty; a dash at
  indentation 1–6; a third line at every indentation strictly between them, with content `"q"`,
  `q`, `'q'`, `- q`, `q: 1`; plus block headers `>`, `|`, `>-`, `|-` at indentation 0–3 with a
  comment at indentation 0–the header's, with and without an intervening blank line.
- §2.1's four parse paths were run over the 179 valid documents extracted from the
  `blockscan.go` enumeration by the predicate "ruamel accepts".
- §4's merge-candidate counts used the predicate "adjacent `String`,`String` where the second is
  not followed by `MappingValue`", over the 179 and over every `.yaml` in the submodule.

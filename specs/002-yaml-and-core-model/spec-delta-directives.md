# Spec delta 002-D — YAML directives (`%YAML`, `%TAG`), and the closeout of the repr findings

**Status:** draft · **Extends:** [`spec.md`](spec.md) · **Inherits:**
[`specs/000-parity-contract/spec.md`](../000-parity-contract/spec.md)
**Upstream covered:** `src/rendercv/schema/yaml_reader.py`; ruamel's `parser.py`, `resolver.py`,
`main.py`, `tag.py`

Citations to `src/...` are relative to `third_party/rendercv/`. Citations to `ruamel/...` are
relative to `third_party/rendercv/.venv/lib/python3.12/site-packages/`, the resolved dependency the
vendored submodule pins and runs — the same convention
[`specs/015-yaml-tags/spec-delta.md`](../015-yaml-tags/spec-delta.md) uses.

**Every string in this document was measured** by driving the vendored Python
(`read_yaml`, `build_rendercv_dictionary_and_model`) and the port's own binary. Recipes in §9.
**This document implements nothing and proposes no Go code** (`AGENTS.md` §4); §6 is the design
register, not an implementation.

---

## 0. Why this file exists, and what it is *not*

It was commissioned as "the missing spec delta for the mapping-key and `TaggedScalar` repr
findings". **That delta already exists and its work already landed.** Writing a second one would
fork the spec. §1 records the closeout with fresh evidence; §2 onward is the residual the closeout
exposed, which is a different and larger subject with no spec at all.

---

## 1. Closeout — the three findings `specs/STATE.md` records as blocked are closed

`specs/STATE.md:410-416` and `:813` describe three findings blocked on one missing `yamldoc`
capability. That text is stale. The spec was written as
[`specs/015-yaml-tags/spec-delta.md`](../015-yaml-tags/spec-delta.md), and units A, B and C landed
as `952c559`, `24ff903`, `7eea6dc`, `43b0baa`.

The design that closed them, for the record — it is exactly the one the commissioning brief asked
be designed, so it is named here rather than re-derived:

| Field | On | Meaning |
|---|---|---|
| `Tag string` | `yamldoc.Node` | ruamel's resolved `trval`; empty unless `Kind == KindTagged` |
| `KeyNode *Node` | `yamldoc.Item` | the key built by the **same** path as any value; `nil` for a key a CLI overlay synthesizes |

One model change covers all three, as `STATE.md` predicted, and **no new `Kind` was needed**:
`KindTagged` already existed. `Item.Key` was deliberately *not* merged into `KeyNode` — `Key` is the
**binding** key that names a field, read at 45 non-test sites, which is a different question from
how a key is *rendered* (`015-yaml-tags/spec-delta.md` §5).

### 1.1 Independent re-measurement

Re-measured against the vendored Python in this session, not read off the earlier delta. Upstream
column is live output; port column is the live binary's panel:

| `locale.language` | Upstream `Input tag '…'` | Port |
|---|---|---|
| `{1: a}` | `{1: 'a'}` | ✅ same |
| `{'1': a}` / `{"1": a}` | `{'1': 'a'}` | ✅ same |
| `{true: a}` | `{True: 'a'}` | ✅ |
| `{'true': a}` | `{'true': 'a'}` | ✅ |
| `{null: a}` / `{~: a}` | `{None: 'a'}` | ✅ |
| `{1.50: a}` | `{1.5: 'a'}` | ✅ |
| `{0x1f: a}` | `{31: 'a'}` | ✅ |
| `{.inf: a}` | `{inf: 'a'}` | ✅ |
| `{2024-01-01: a}` | `{'2024-01-01': 'a'}` | ✅ |
| `{!!str k: a}` | `{TaggedScalar(value='k', style=None, tag=Tag('tag:yaml.org,2002:str')): 'a'}` | ✅ |
| `{!!int 1: a}` | `{1: 'a'}` | ✅ |
| `{!!bool yes: a}` | `{True: 'a'}` | ✅ |
| `[!!str x]` | `[TaggedScalar(value='x', style=None, tag=Tag('tag:yaml.org,2002:str'))]` | ✅ |
| `[!unknown x]` | `[TaggedScalar(value='x', style=None, tag=Tag('!unknown'))]` | ✅ |
| `[!!str 31]` | `[TaggedScalar(value='31', style=None, tag=Tag('tag:yaml.org,2002:str'))]` | ✅ |
| `{a: !!str x}` | `{'a': TaggedScalar(value='x', style=None, tag=Tag('tag:yaml.org,2002:str'))}` | ✅ |

`go test ./internal/schema/models/locale/` passes with 2 skips in `taggedrepr_test.go` and 3 in
`keyrepr_test.go`, all carrying a measured upstream value and a reason naming goccy.

**Action for the merge owner:** correct `STATE.md:410-416` and the row-15 text at `:813`. No porter
task is needed for these three.

### 1.2 The one thing the closeout left undone

`015-yaml-tags/spec-delta.md` §6.1 defers `%TAG` directives and says, in terms:

> Not measured, because the harness cannot place a directive before the document it builds. **It
> should be measured before the §3.3 table is treated as complete.**

**That premise is wrong** — the harness takes the whole YAML string, so a directive places fine, and
measuring it took one command. §2 onward is that measurement and what it found, which is not a repr
gap but a live rejection of documents upstream renders.

---

## 2. Purpose

A YAML *directive* is a `%`-prefixed line before the `---` document marker. Upstream inherits full
directive support from ruamel: `yaml_reader.py:81` constructs a plain `ruamel.yaml.YAML()` and
`:53` loads with it, so `process_directives` runs on every document
(`ruamel/yaml/parser.py:288-330`). The port refuses **every** document carrying a directive, with an
error that names the wrong cause. This delta specifies the behavior and bounds the fix.

## 3. Inputs / outputs

Input: the bytes of the main YAML file. Output: either a parsed document (with a resolver version
and a tag-handle table selected by the directives) or a validation record. Nothing else in the
pipeline observes directives directly; they act by changing how scalars resolve and how tags expand.

---

## 4. Behavior

### 4.1 A directive-headed document loads normally

4.1.1 A document preceded by one or more directives and a `---` marker loads exactly as the same
document without them. Measured: `%YAML 1.2\n---\ncv:\n  name: John Doe\n` and
`%TAG !e! tag:example.com,2000:\n---\ncv:\n  name: John Doe\n` both load `{'cv': {'name': 'John
Doe'}}` and validate clean (`ruamel/yaml/parser.py:288-330`).

4.1.2 An **unrecognised** directive is ignored, not an error. `%FOO bar\n---\nk: v\n` loads
`{'k': 'v'}`; `process_directives` branches only on `'YAML'` and `'TAG'` and drops anything else
(`ruamel/yaml/parser.py:292-311`).

4.1.3 Multiple directives are allowed. `%YAML 1.2\n%TAG !e! tag:x,1:\n---\nk: v\n` loads
`{'k': 'v'}` (`ruamel/yaml/parser.py:291`, the `while` over `DirectiveToken`).

4.1.4 A directive **without** a `---` marker is an error. `%TAG !e! tag:x,1:\nk: v\n` and
`%YAML 1.2\nk: v\n` both raise `ScannerError` with problem `mapping values are not allowed here` at
line 2, column 2.

### 4.2 `%TAG` rebinds a handle, and the rebinding reaches the tag repr

`trval` is `handles[handle] + uri_decoded_suffix`, and `handles` is per-document
(`ruamel/yaml/tag.py:55-88`), seeded from `DEFAULT_TAGS = {'!': '!', '!!': 'tag:yaml.org,2002:'}`
only for handles the directives did not bind (`ruamel/yaml/parser.py:106`, `:327-329`).

4.2.1 Rebinding the primary handle changes every `!!` expansion. Measured, `locale.language` of
`[!!str x]` under `%TAG !! tag:example.com,2000:`:

```
[TaggedScalar(value='x', style=None, tag=Tag('tag:example.com,2000:str'))]
```

4.2.2 A named handle expands through its binding. `%TAG !e! tag:example.com,2000:` with `[!e!x v]`:

```
[TaggedScalar(value='v', style=None, tag=Tag('tag:example.com,2000:x'))]
```

4.2.3 Rebinding `!` changes local tags. `%TAG ! tag:example.com,2000:` with `[!foo v]`:

```
[TaggedScalar(value='v', style=None, tag=Tag('tag:example.com,2000:foo'))]
```

4.2.4 A named handle used **without** its directive is not a tag resolution at all: `[!e!x v]` with
no `%TAG` line raises a `RenderCVUserValidationError` before any repr.

4.2.5 A duplicate handle is an error: problem `duplicate tag handle '!e!'` at the second directive's
line, column 1 (`ruamel/yaml/parser.py:307-310`).

**This completes the tag table at `015-yaml-tags/spec-delta.md` §3.3**, which is exact only for a
document with no `%TAG` line.

### 4.3 `%YAML` selects the resolver version — the largest behavior here

`processing_version` reads the scanner's `yaml_version` and falls back to the default only when
there is no directive (`ruamel/yaml/resolver.py:377-392`); the implicit-resolver table is keyed by
version (`ruamel/yaml/resolver.py:25-72`).

4.3.1 `%YAML 1.2` is a no-op — it selects what upstream already uses.

4.3.2 `%YAML 1.1` **changes what plain scalars mean.** Measured through `read_yaml` on `k: <token>`:

| Token | `%YAML 1.2` | `%YAML 1.1` |
|---|---|---|
| `yes` | `'yes'` | `True` |
| `no` | `'no'` | `False` |
| `on` | `'on'` | `True` |
| `off` | `'off'` | `False` |
| `y` | `'y'` | `True` |
| `n` | `'n'` | `False` |
| `010` | `10` | `8` (octal without `0o`) |
| `0o10` | `8` | `'0o10'` |
| `1:30` | `'1:30'` | `90` (sexagesimal int) |
| `1:30.5` | `'1:30.5'` | `90.5` (sexagesimal float) |
| `true`, `0x1f`, `.inf`, `1_0`, `~` | unchanged | unchanged |

Citations for each row: the 1.1 bool set (`ruamel/yaml/resolver.py:30-35`), the 1.1 float with
sexagesimal (`:45-53`), the 1.1 int with bare-octal and sexagesimal (`:62-69`).

**Consequence for a CV:** `%YAML 1.1\n---\ncv:\n  name: yes\n` gives upstream a `bool` where the
model wants a string, so upstream *rejects* it — the directive is not merely cosmetic, it changes
validation outcomes.

4.3.3 `%YAML` with major ≠ 1 is a `ParserError`, problem exactly:

```
found incompatible YAML document (version 1.* is required)
```

at the directive's line, column 1 (`ruamel/yaml/parser.py:296-304`).

4.3.4 `%YAML 1.3` — major 1, minor neither 1 nor 2 — is **not** that error. It is a bare Python
`AssertionError` from `ruamel/yaml/main.py:851`:

```
version minor part can only be 2 or 1, got (1, 3)
```

This is not a `MarkedYAMLError`, so upstream does not catch it with the scanner and parser errors:
it escapes as a **rich traceback on stderr, nothing on stdout, exit 1** — the class D-012 §3 already
names (D-011's open question).

4.3.5 A duplicate `%YAML` directive is a `ParserError`, problem exactly:

```
found duplicate YAML directive
```

at the second directive's line, column 1 (`ruamel/yaml/parser.py:293-296`).

4.3.6 A malformed `%YAML` with no version is a `ScannerError` with context `while scanning a
directive` at line 1 column 1 and problem:

```
expected a digit, but found '\n'
```

at line 1, column 6.

### 4.4 What the port does today — the defect

4.4.1 **Any** document carrying **any** directive is rejected with one record whose message is:

```
This is not a valid YAML file. expected a single document in the stream.
```

located `main_yaml_file: line 1 to line 2`, exit 1. Measured on `%YAML 1.2`, `%YAML 1.1`,
`%TAG !! …`, `%TAG !e! …`, `%FOO bar` — including `%FOO bar`, which upstream ignores entirely
(4.1.2), and `%YAML 1.2`, which upstream treats as a no-op (4.3.1).

4.4.2 **Root cause, and it is not multi-document handling.** goccy's parser emits the directive line
as its own `*ast.DocumentNode`, so `checkSingleDocument` sees `len(docs) == 2` and raises
`MultiDocumentError` (`internal/schema/yamlreader/build.go:215-240`). That is the only producer of
the string in 4.4.1 (`build.go:221`), and a plain `---\ncv: …` with no directive parses fine, so the
`---` marker is not the trigger. The check is correct for its own purpose — a genuine two-document
stream must still fail — and wrong only in counting a directive-only pseudo-document.

4.4.3 Two directive shapes fail *before* the count, with goccy's raw text leaking:

| Document | Port message | Upstream |
|---|---|---|
| `%YAML 2.0\n---\n…` | `This is not a valid YAML file. [1:7] unknown YAML version "2.0".` | `found incompatible YAML document (version 1.* is required)` (4.3.3) |
| `%TAG !e! tag:x,1:\ncv: …` | `This is not a valid YAML file. [1:1] unexpected directive value. document not started.` | `mapping values are not allowed here`, line 2 column 2 (4.1.4) |

Both are `ruamelPhrasing` gaps (`internal/schema/modelbuilder/yamlerror.go:138`), the same shape as
the goccy-phrasing rows already tracked in `STATE.md`.

---

## 5. Exact user-visible strings

Upstream problem texts, verbatim. Each is prefixed by `This is not a valid YAML file. ` when it
becomes a record — via `yamlSyntaxValidationError`, which appends **no** trailing period
(`internal/schema/modelbuilder/yamlerror.go:87-88`), unlike the `MultiDocumentError` path, which
does (`:42`).

```
found incompatible YAML document (version 1.* is required)
```
Condition: `%YAML` with a major version other than 1 (4.3.3).

```
found duplicate YAML directive
```
Condition: two `%YAML` directives on one document (4.3.5).

```
duplicate tag handle '!e!'
```
Condition: two `%TAG` directives binding the same handle; the handle is Python-`repr`'d, so the
quoting rule is `pythonStringRepr`'s (4.2.5).

```
mapping values are not allowed here
```
Condition: a directive with no `---` marker before the document body (4.1.4).

```
expected a digit, but found '\n'
```
Condition: `%YAML` with no version, under context `while scanning a directive` (4.3.6).

```
version minor part can only be 2 or 1, got (1, 3)
```
Condition: `%YAML 1.<n>` for n ∉ {1,2}. **Not a record** — an uncaught `AssertionError` printed as a
Python traceback on stderr (4.3.4).

The string the port must **stop** producing for a directive-headed document:

```
This is not a valid YAML file. expected a single document in the stream.
```

---

## 6. Design (plan register)

Combined with the spec because the shape is small: one predicate, one phrasing row pair, and two
proposed divergences. Nothing here is code.

### 6.1 The core change

`checkSingleDocument` (`internal/schema/yamlreader/build.go:227-240`) must count only documents that
carry content. A goccy document produced solely by a directive line has no body; skipping bodyless
documents before the `len(docs) < 2` test restores 4.1.1–4.1.3 without touching the genuine
multi-document case, whose marks are pinned by `yamlerror_test.go:528` and its neighbours.

**Why the predicate must be "has a body", not "is first":** a real two-document stream whose second
document is empty must keep failing, and a directive block can precede either document in a stream.
Anchoring on emptiness rather than position keeps both.

**Open measurement the implementer must make first, because it decides scope.** Once the document
parses, does goccy expand a `%TAG` handle into the node's tag, or hand back the unexpanded handle?
The probe is four lines:

```
lexer/parser over: "%TAG !e! tag:example.com,2000:\n---\nk: !e!x v\n"
   -> read the *ast.TagNode's tag string
```

- If goccy expands, 4.2.1–4.2.3 fall out of the core change with no further code — the same shape
  `015-yaml-tags/spec-delta.md` §8.2 found for tagged keys, and the same reason: the information is
  already in hand at the parse site.
- If it does not, `buildTagged` (`build.go:288-325`) needs a per-document handle table threaded to
  `tagName` (`:365-370`), seeded from `DEFAULT_TAGS`. That is a second unit, not a bigger first one.

### 6.2 Type discipline

The handle table, if 6.1's probe says it is needed, is a named type over the document, not a
`map[string]string` passed loose — `AGENTS.md` §9. It belongs beside the parse state, not on
`yamldoc.Node`: it is a property of the *document*, and putting it on every node would repeat the
mistake §5 of the 015 delta avoids with `Item.Key`.

**No new `Kind`, no new `Node` field for §4.2.** A rebound handle changes the *value* of the
existing `Node.Tag`, which unit A already carries. This is why 4.2 is a completion of the 015 table
rather than a new capability.

### 6.3 Hazards (`AGENTS.md` §6)

- **Blast radius is every document.** `checkSingleDocument` runs on every parse. The predicate must
  be provably inert for a document with no directive — a bodyless-document skip is, since a
  directive-free single document has exactly one document with a body.
- **The gating asymmetry of `spec-delta-folding.md` §4 applies in reverse and is favourable here:**
  today's failure is a *rejection*, so a wrong fix regresses toward accepting something, not toward
  silently rendering a wrong value. `%YAML 1.1` is the one place that inverts — see 6.4.

### 6.4 Proposed divergence — `%YAML 1.1`

Not in `specs/divergences.md`. Proposed entry, for the merge owner to file:

> **D-0xx — `%YAML 1.1` does not switch the scalar resolver**
> **Differs:** a document opening `%YAML 1.1` resolves plain scalars by YAML 1.1 rules upstream —
> `yes`/`no`/`on`/`off`/`y`/`n` become bools, `010` is octal 8, `0o10` is a string, `1:30` is
> sexagesimal 90 (`ruamel/yaml/resolver.py:30-35`, `:45-53`, `:62-69`, selected at `:377-392`). The
> port resolves by 1.2 in all cases.
> **Why not:** it is a second complete resolver table behind a directive no CV in either project
> writes, and no upstream test covers it (`grep '%YAML' third_party/rendercv/tests/` is empty). It
> is the one directive behavior where a partial fix would silently change a *value*.
> **Scope note:** must be decided *with* 6.1, because 6.1 makes `%YAML 1.1` documents parse — today
> they are rejected, which is loud; afterwards they would render with 1.2 values, which is silent.
> Options: implement 1.1, or reject `%YAML 1.1` explicitly with a named error. **Do not land 6.1
> without choosing.**

### 6.5 Proposed divergence — container mapping keys

`015-yaml-tags/spec-delta.md` §4.2 measured this, three rows are `t.Skip`ped in
`internal/schema/models/locale/keyrepr_test.go:168`, and **no entry exists in
`specs/divergences.md`** — grep for `sequence key`, `container key` and `tuple` all return nothing.
It is D-012's family but not covered by D-012's text. Proposed entry:

> **D-0yy — a mapping key that is a sequence or a mapping is refused at parse time**
> **Differs:** upstream constructs a tuple key — `{[1]: a}` is `{(1,): 'a'}`, `{[1, 2]: a}` is
> `{(1, 2): 'a'}`, `{[]: a}` is `{(): 'a'}`, `{? {a: 1}\n : b}` is `{ordereddict({'a': 1}): 'b'}`,
> and `{[[1]]: a}` raises `TypeError: unhashable type: 'CommentedSeq'`. goccy refuses all of them
> with `found an invalid key for this map`.
> **Why not:** the fault is in the parser, before any node exists — D-012 §2's reason exactly. A
> `yamldoc` change cannot reach it.

### 6.6 Tradeoffs considered

| Option | Verdict |
|---|---|
| Strip directive lines from the source before parsing | **No.** It changes coordinates for every later line, which `spec-delta-folding.md` §2.2 rules out as the bar `parseTolerantOfQuotedTabs` sets, and it silently discards `%TAG` semantics. |
| Fix `checkSingleDocument` only, leave 4.4.3's two phrasings | **Acceptable as a first unit.** The two shapes are already-failing documents, so they are a message defect, not a value defect. |
| Do nothing, record the whole class as a divergence | **No.** `%YAML 1.2` and `%FOO bar` are no-ops upstream, so the port rejects documents that are *semantically identical* to ones it accepts. That is the least defensible shape of divergence. |

---

## 7. Out of scope

1. **`%YAML 1.1` resolver semantics** — 6.4, needs a decision before 6.1 lands.
2. **Container mapping keys** — 6.5, parser-level, divergence not implementation.
3. **The `AssertionError` traceback of 4.3.4** — D-011's class, not this delta's.
4. **Directives on the *second* document of a stream.** Upstream fails such a stream anyway
   (`MultiDocumentError`); not measured here.
5. **Directives in a design/locale/override file** rather than the main YAML. The reader is shared,
   so the behavior should follow, but it was measured only on the main file.

---

## 8. Acceptance criteria

- [ ] `%YAML 1.2\n---\n<valid CV>` renders, exit 0, byte-identical `.typ` to the same CV without the
      directive.
- [ ] `%FOO bar\n---\n<valid CV>` renders, exit 0 (4.1.2).
- [ ] `%YAML 1.2\n%TAG !e! tag:x,1:\n---\n<valid CV>` renders, exit 0 (4.1.3).
- [ ] A genuine two-document stream still produces
      `This is not a valid YAML file. expected a single document in the stream.` with the marks
      `yamlerror_test.go:528` pins — the regression this unit most plausibly causes.
- [ ] `locale.language: [!!str x]` under `%TAG !! tag:example.com,2000:` produces the 4.2.1 string
      byte for byte; likewise 4.2.2 and 4.2.3.
- [ ] `%TAG !e! tag:x,1:\ncv: …` (no marker) produces upstream's `mapping values are not allowed
      here` at line 2 column 2, not goccy's `unexpected directive value` (4.4.3).
- [ ] `%YAML 2.0\n---\n…` produces `found incompatible YAML document (version 1.* is required)` at
      line 1 column 1 (4.3.3).
- [ ] `%YAML 1.2\n%YAML 1.2\n---\n…` produces `found duplicate YAML directive` at line 2 column 1.
- [ ] `%TAG !e! a:\n%TAG !e! b:\n---\n…` produces `duplicate tag handle '!e!'` at line 2 column 1.
- [ ] `%YAML 1.1` either resolves by the 4.3.2 table or is rejected by a named error — 6.4's
      decision, whichever it is, is asserted.
- [ ] `just check` clean; `go test ./internal/schema/...` clean.

## 9. Corpus additions

`tools/gengolden` cases, one document each, all through the main YAML file:

| Case | Document |
|---|---|
| `yaml_directive_noop` | `%YAML 1.2` + `---` + the sample CV |
| `unknown_directive` | `%FOO bar` + `---` + the sample CV |
| `tag_directive_primary` | `%TAG !! tag:example.com,2000:` + `locale.language: [!!str x]` |
| `tag_directive_named` | `%TAG !e! tag:example.com,2000:` + `locale.language: [!e!x v]` |
| `tag_directive_default` | `%TAG ! tag:example.com,2000:` + `locale.language: [!foo v]` |
| `directive_no_marker` | `%TAG !e! tag:x,1:` with no `---` |
| `yaml_version_major` | `%YAML 2.0` + `---` |
| `yaml_duplicate_directive` | two `%YAML 1.2` lines |
| `duplicate_tag_handle` | two `%TAG !e!` lines |
| `yaml_11_bools` | `%YAML 1.1` + `cv.name: yes` — **gated on 6.4**; land it only with the decision |

## 10. Recommended breakdown

| Unit | Content | Depends on |
|---|---|---|
| 0 | Correct `STATE.md:410-416` and `:813` (§1). Merge owner, not a porter. | — |
| 1 | The 6.4 and 6.5 divergence decisions. | — |
| 2 | Golden fixtures for §9, landing **red**. | unit 1 for the last row |
| 3 | `checkSingleDocument` counts only documents with a body (4.1.1–4.1.3). | units 1, 2 |
| 4 | 6.1's handle probe, then `%TAG` expansion if the probe says it is needed (4.2). | unit 3 |
| 5 | `ruamelPhrasing` rows for the two 4.4.3 shapes. | unit 3 |

Units 4 and 5 are independent of each other and fan out. Units 0 and 1 are independent of
everything.

---

## 11. Method

```bash
# upstream: what a directive-headed document loads to
cd third_party/rendercv && uv run python -c '
import json
from rendercv.schema.yaml_reader import read_yaml
print(json.dumps(read_yaml("%YAML 1.1\n---\nk: yes\n")))'

# upstream: the repr a message quotes, via locale.language
cd third_party/rendercv && uv run python -c '
import re
from rendercv.schema.rendercv_model_builder import build_rendercv_dictionary_and_model
src = "%TAG !! tag:example.com,2000:\n---\ncv:\n  name: J\nlocale:\n  language: [!!str x]\n"
try: build_rendercv_dictionary_and_model(src)
except Exception as e:
    print(re.search(r"Input tag \x27(.*?)\x27 found using", str(getattr(e, "message", e)), re.S).group(1))'

# the port
go build -o /tmp/rcvgo ./cmd/rendercv-go && /tmp/rcvgo render CV.yaml
```

`grep -rn '%YAML\|%TAG' third_party/rendercv/{tests,src,docs}` is empty: **no upstream test covers
any of this.** The behavior is inherited from ruamel, so every expectation above is measured from
the library rather than mined from a test, and `AGENTS.md` §4's "mine upstream's tests" yields
nothing here.

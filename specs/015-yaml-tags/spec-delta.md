# Spec 015 delta — what `yamldoc` drops, and what three findings need back

**Status:** proposal · **Extends:** [`spec.md`](spec.md) §3.4 · **Blocks:** three recorded findings

**Upstream covered:**

- `src/rendercv/schema/pydantic_error_handling.py` (where a value's `str()` reaches a message)
- `ruamel/yaml/comments.py`, `ruamel/yaml/tag.py`, `ruamel/yaml/constructor.py`,
  `ruamel/yaml/parser.py` (what a tag and a non-string key actually become)

Citations to `src/...` are relative to `third_party/rendercv/`. Citations to `ruamel/...` are
relative to `third_party/rendercv/.venv/lib/python3.12/site-packages/`, the resolved dependency the
vendored submodule pins and runs.

**Every string in this document was measured**, by driving
`build_rendercv_dictionary_and_model` on the vendored Python and reading the `union_tag_invalid`
message for a `locale.language` of the given shape — the same harness
`internal/schema/models/locale/languagerepr_test.go` uses. Nothing here is read off the Python
source alone.

---

## 1. Purpose

Three findings are blocked on one missing capability, and none of them can be closed without a
change to the shape of `yamldoc.Node` / `yamldoc.Item`. This document establishes exactly what is
dropped, what it would cost to keep, and how the three should be split into units. **It proposes no
Go code** (`AGENTS.md` §4).

The three, as recorded:

1. `repr(TaggedScalar)` — the port prints the scalar's text where upstream prints a constructor
   call. `specs/STATE.md` row 15 already calls this "a spec change, not a rename".
2. The same, for a `TaggedScalar` **inside a container** — same cause, and it is the reachable one.
3. A **non-string mapping key** — `{1: a}` is `{1: 'a'}` upstream and `{'1': 'a'}` here.

### 1.1 Where these become visible

Only through a message that quotes a value's `str()` rather than the Input Value column. The column
renders any mapping or sequence as `...` (`pydantic_error_handling.py:122-126`), so a container
never reaches it; a message built by pydantic itself does, and `union_tag_invalid` is the one the
port has a harness for. **This is a narrow surface, and that is a reason to size the work honestly
rather than a reason to skip it** — the same `PythonText` is what any future message quoting a
container will use.

---

## 2. What `yamldoc` keeps and what it drops

`yamldoc.Node` (`internal/schema/yamldoc/node.go:74-81`) keeps `Kind`, `Span`, `Raw`, `Style`,
`Items`, `Elems`. `yamldoc.Item` (`:84-95`) keeps `Key string`, `KeySpan`, `Value`, `KeyTagged bool`.

Two things the parser has and these types discard:

| Dropped | Available at parse time | Needed by |
|---|---|---|
| the **tag** of a tagged scalar | `tagName(node)` already computes it (`yamlreader/build.go:365-370`) and `buildTagged` throws it away after `ResolveTag` (`:302`) | findings 1, 2 |
| the key's **kind and style** | `buildMapping` calls `scalarRaw(keyTok)` for the text (`:404`) and never calls `buildNode` on the key | finding 3 |

The consequence is stated in the code already, in both repr implementations, which are two copies of
the same function: `schemaerr/pythonrepr.go:40-48` and `:66-70`, and
`models/design/design.go:146-153` and `:174-181`.

---

## 3. Behavior — finding 1 and 2, `repr(TaggedScalar)`

### 3.1 The format

`TaggedScalar.__repr__` is one f-string (`ruamel/yaml/comments.py:1186-1187`):

```python
f'TaggedScalar(value={self.value!r}, style={self.style!r}, tag={self.tag!r})'
```

and `Tag.__repr__` is `f'{self.__class__.__name__}({self.trval!r})'` (`ruamel/yaml/tag.py:31-32`).
So the rendered form is, exactly:

```
TaggedScalar(value=<repr of the text>, style=<repr of the style>, tag=Tag(<repr of the tag>))
```

All three interpolations are Python `repr`, which the port already has as
`pythonStringRepr` (`schemaerr/pythonrepr.go:88-115`) — including its quote-selection rule, which
finding 1 exercises: `[!!str "it's"]` is `TaggedScalar(value="it's", style='"', …)`, measured.

**A top-level `TaggedScalar` is not affected.** `__str__` is the bare value
(`ruamel/yaml/comments.py:1177-1178`), so `language: !!str english` is `Input tag 'english'` and the
port is already right. Only a container's members are `repr`'d, which is why finding 2 is the
reachable one and finding 1 is its top-level twin.

### 3.2 The style table — measured, all five

`data2.style = node.style` (`ruamel/yaml/constructor.py:1618-1620`), the scanner's own style byte.

| Written | `style=` | Note |
|---|---|---|
| `- !!str x` | `None` | plain |
| `- !!str 'x'` | `"'"` | Python renders the `'` in double quotes |
| `- !!str "x"` | `'"'` | |
| `- !!str \|` | `'\|'` | value keeps its trailing newline: `value='x\n'` |
| `- !!str >` | `'>'` | same, `value='x\n'` |
| `- !!str` (no value) | `None` | `value=''`, matching `build.go:304-313` |

### 3.3 The tag table — measured

`trval` is `handles[handle] + uri_decoded_suffix` when a handle was scanned, and the URI-decoded
suffix alone when one was not (`ruamel/yaml/tag.py:55-88`), with
`DEFAULT_TAGS = {'!': '!', '!!': 'tag:yaml.org,2002:'}` (`ruamel/yaml/parser.py:106`).

| Written | `tag=Tag(...)` | Rule |
|---|---|---|
| `!!str` | `'tag:yaml.org,2002:str'` | `!!` handle expands |
| `!!foo/bar` | `'tag:yaml.org,2002:foo/bar'` | suffix verbatim, `/` included |
| `!!STR` | `'tag:yaml.org,2002:STR'` | case preserved |
| `!unknown` | `'!unknown'` | local tag, verbatim |
| `!<tag:x.com,1:t>` | `'tag:x.com,1:t'` | verbatim URI form |
| `!<!local>` | `'!local'` | |
| `!!` | `'!!'` | **no expansion** — scanned as a local tag, not as a handle |
| `!%21` | `'!!'` | the suffix is URI-decoded |
| `!` | *not tagged at all* | resolves normally; `[! x]` is `['x']` |

**Two of these rows are unreachable in the port — see §6.5.** `!<tag:x.com,1:t>` and `!!merge` are
refused by goccy at parse time, so no node exists to render. They stay in the table because they are
what upstream does; they are recorded as skipped rows in the test rather than as expectations.

The `!` row is not a rendering rule but a **resolution** one, and it was the first thing the
implementation had to fix: `KindTagged` is the kind every typed field rejects, so reading `!` as a
tag turned a document upstream accepts into a validation error — `locale.language: ! english`
raises nothing upstream and failed here. Fixed in `buildTagged` before this delta's own work, since
without it `! x` would have started rendering as `TaggedScalar(…, tag=Tag('!'))`, a worse answer
than the one it replaced.

**A tagged collection is not a `TaggedScalar`** and needs nothing here: `construct_unknown` branches
on the node's shape (`ruamel/yaml/constructor.py:1598-1640`), so `[!!map {a: 1}]` is `[{'a': 1}]` and
`[!unknown [1]]` is `[[1]]` — both measured, both already right in the port.

### 3.4 Exact strings

| Input (`locale.language`) | Upstream `Input tag '…'` | Port today |
|---|---|---|
| `[!!str x]` | `[TaggedScalar(value='x', style=None, tag=Tag('tag:yaml.org,2002:str'))]` | `[x]` |
| `{a: !!str x}` | `{'a': TaggedScalar(value='x', style=None, tag=Tag('tag:yaml.org,2002:str'))}` | `{'a': x}` |
| `[!unknown x]` | `[TaggedScalar(value='x', style=None, tag=Tag('!unknown'))]` | `[x]` |
| `[!!str 31]` | `[TaggedScalar(value='31', style=None, tag=Tag('tag:yaml.org,2002:str'))]` | `[31]` |
| `[!!str x, !unknown y]` | `[TaggedScalar(value='x', …:str')), TaggedScalar(value='y', …'!unknown'))]` | `[x, y]` |
| `{a: [!!str x]}` | `{'a': [TaggedScalar(value='x', style=None, tag=Tag('tag:yaml.org,2002:str'))]}` | `{'a': [x]}` |

**The port's output is not merely a different rendering — it is not valid Python.** `[x]` and `[31]`
put a bare word and an integer where a repr must appear, so a reader cannot tell `[!!str 31]` from
`[31]`, which upstream renders differently on purpose.

---

## 4. Behavior — finding 3, a non-string mapping key

A key is `repr`'d exactly like any other value, so the *type* decides the spelling. Measured:

| Written | Upstream | Port today |
|---|---|---|
| `{1: a}` | `{1: 'a'}` | `{'1': 'a'}` |
| `{'1': a}` / `{"1": a}` | `{'1': 'a'}` | `{'1': 'a'}` ✓ |
| `{true: a}` | `{True: 'a'}` | `{'true': 'a'}` |
| `{False: a}` | `{False: 'a'}` | `{'False': 'a'}` |
| `{'true': a}` | `{'true': 'a'}` | `{'true': 'a'}` ✓ |
| `{yes: a}` / `{ON: a}` | `{'yes': 'a'}` / `{'ON': 'a'}` | ✓ — YAML 1.2, not a bool |
| `{null: a}` / `{~: a}` | `{None: 'a'}` | `{'null': 'a'}` / `{'~': 'a'}` |
| `{1.50: a}` | `{1.5: 'a'}` | `{'1.50': 'a'}` |
| `{0x1f: a}` | `{31: 'a'}` | `{'0x1f': 'a'}` |
| `{1_000: a}` | `{1000: 'a'}` | `{'1_000': 'a'}` |
| `{1e3: a}` | `{1000.0: 'a'}` | `{'1e3': 'a'}` |
| `{-0: a}` | `{0: 'a'}` | `{'-0': 'a'}` |
| `{.inf: a}` / `{.nan: a}` | `{inf: 'a'}` / `{nan: 'a'}` | `{'.inf': 'a'}` |
| `{2024-01-01: a}` | `{'2024-01-01': 'a'}` | ✓ — a **string**, not a date |
| `{'': a}` | `{'': 'a'}` | ✓ |
| `{a: 1, 2: b}` | `{'a': 1, 2: 'b'}` | `{'a': 1, '2': 'b'}` — mixed keys, input order |

**This is the same value-rendering `RenderInput` already performs**, applied to the key instead of
the value: every right-hand column above is what the port already prints for the same token in value
position. The gap is not the rendering rule; it is that the key never becomes a node.

### 4.1 A tagged key

| Written | Upstream |
|---|---|
| `{!!str k: a}` | `{TaggedScalar(value='k', style=None, tag=Tag('tag:yaml.org,2002:str')): 'a'}` |
| `{!unknown k: a}` | `{TaggedScalar(value='k', style=None, tag=Tag('!unknown')): 'a'}` |
| `{!!int 1: a}` | `{1: 'a'}` — a forced tag constructs the value |
| `{!!bool yes: a}` | `{True: 'a'}` |

So finding 3's tagged half **needs finding 1's capability too**: `Item.KeyTagged bool`
(`node.go:89-94`) records that a key was tagged but not with what.

### 4.2 A container key — blocked in the parser, not in `yamldoc`

| Written | Upstream | Port today |
|---|---|---|
| `{[1]: a}` | `{(1,): 'a'}` — a **tuple** | goccy refuses the document: `found an invalid key for this map` |
| `{[1, 2]: a}` | `{(1, 2): 'a'}` | same |
| `{[]: a}` | `{(): 'a'}` | same |
| `{? {a: 1}\n : b}` | `{ordereddict({'a': 1}): 'b'}` | same |
| `{[[1]]: a}` | `TypeError: unhashable type: 'CommentedSeq'` | same |

**This half is not reachable from a `yamldoc` change at all** — goccy rejects the document before a
node exists. It is a parser-level divergence in the same family as the ones spec 015 §6 already
records, and it needs the human gate (`AGENTS.md` §5), not an implementation unit.

---

## 5. What the change would be

Two fields. **No new `Kind`** — the key is an ordinary node of an existing kind, and `KindTagged`
already exists.

| Field | On | Meaning |
|---|---|---|
| `Tag string` | `Node` | the resolved tag, `trval`; empty unless `Kind == KindTagged` |
| `KeyNode *Node` | `Item` | the key as a node; `nil` for a synthesized key |

`Item.Key` stays as it is. `item.Key` is read at 53 sites, 45 of them outside tests, and it is the
**binding** key — the thing that names a field — which is a different question from how the key is
*rendered*. Merging the two would put all 45 at risk for a rendering fix.

### 5.1 Blast radius — measured, not estimated

Both fields were added to `node.go` on a scratch tree and the tree was rebuilt and re-tested:

| Check | Result |
|---|---|
| `go build ./...` | clean |
| `go test ./...` | no new failure |
| `go test ./internal/kindguard/` | ok — **no new Kind, so nothing to enforce** |
| `golangci-lint run` | clean but for a `gofumpt` complaint about the scratch edit's own formatting |

The scratch edit was reverted; nothing was committed. **Adding the fields costs nothing.** All of
the work is in populating and consuming them:

- **populate:** `buildTagged` (`build.go:288-325`) for `Tag`, `buildMapping` (`:387-421`) for
  `KeyNode`. Both already hold the information — `tagName` computes the tag and discards it, and the
  key token is in hand.
- **synthesized keys:** `modelbuilder/merge.go:228` and `:321` build `Item{Key:, Value:}` for
  CLI-overlay keys with no source node. `KeyNode` is `nil` there and the renderer must fall back to
  `pythonStringRepr(item.Key)` — those keys are always strings, so the fallback is exact.
- **consume:** `schemaerr/pythonrepr.go`, and only it — see §5.2. Within it, the arm to change is
  `pythonRepr` (`:71-76`), the container-member path. **`PythonText`'s own `KindTagged` arm
  (`:51-57`) and `RenderInput` (`schemaerr/error.go:163-164`) must not change**: those are `str()`
  and the Input Value column, where a `TaggedScalar` correctly renders as its bare text
  (`ruamel/yaml/comments.py:1177-1178`). The whole of finding 1/2 is the difference between `str()`
  and `repr()`, and the port already has that split in the right place.
- **tests constructing `Item`:** `binder_test.go`, `renderinput_test.go`, `node_test.go` — unaffected,
  since a new field defaults to its zero value.

### 5.2 There is one renderer, not two

**An earlier draft of this section was wrong and is corrected here.** It described
`PythonText`/`pythonRepr` and `themeNameRepr`/`pythonElemRepr` as two copies of one function and
recommended against unifying them as part of this work. That duplication no longer exists:
`themeNameRepr`, `pythonElemRepr` and `pythonBoolRepr` are deleted and `design.go:88` calls
`schemaerr.PythonText`, landed as its own unit with its own red test over 19 shapes through the
theme path — and it fixed a live defect on the way, since `design.py:57` computes the theme name
once, so upstream's `theme: 007` looks for the folder `7` where the port looked for `007`.

Three consequences for the work this document scopes, all of them reductions:

1. **One consumer to change, not two.** Every "both reprs" in §5.1 and §7 is one function in
   `schemaerr/pythonrepr.go`.
2. **The divergence hazard is gone.** There is no longer a way to fix the tag in one renderer and
   leave the other wrong, which was the reason the earlier draft argued for care here.
3. **No unification unit is needed and none should be scheduled.**

One near-miss for a future reader: `models/design/schema.go:276` still defines a `pythonRepr`, but
it renders **Go values** (`any`) for the JSON-schema description resplice and never sees a
`yamldoc.Node`. It is not a second node renderer and is out of scope.

---

## 6. Out of scope

1. **`%TAG` directives.** A document may rebind a handle, which changes every expansion in §3.3.
   Neither the port nor this delta handles it; ruamel's `handles` table is per-document
   (`ruamel/yaml/tag.py:66-68`). Not measured, because the harness cannot place a directive before
   the document it builds. **It should be measured before the §3.3 table is treated as complete.**
2. **Container keys** — §4.2, blocked in goccy, human gate.
3. **`[!!str]` and `{a: !!str}` in flow style.** Both raise a `RenderCVUserValidationError` whose
   `str()` is **empty** upstream — a different defect, unrelated to repr, and worth its own record.
   The block spelling `- !!str` behaves normally and is in the §3.2 table.
4. ~~Unifying the two repr implementations.~~ **Already done and merged** — §5.2.

### 6.5 Tags goccy refuses on a scalar — found while implementing §3

`buildTagged` never sees these, because the document does not parse. Measured with
`yamlreader.ReadString` over `language: [<tag> x]`, one spelling at a time:

| Spelling | goccy | upstream |
|---|---|---|
| `!!merge` | `could not find merge key` | `TaggedScalar(value='x', …, tag=Tag('tag:yaml.org,2002:merge'))` |
| `!!omap`, `!!set`, `!!seq`, `!!map` | refused | a `TaggedScalar` for each |
| `!<tag:x.com,1:t>` | refused | `TaggedScalar(…, tag=Tag('tag:x.com,1:t'))` |
| `!!str`, `!!binary`, `!!pairs`, `!!python/object`, `!!foo/bar`, `!!STR`, `!!`, `!%21`, `!<!local>`, `!unknown`, `!foo`, `!!value`, `!!yaml` | accepted | — |

The pattern: goccy checks a handful of **standard collection tags** against the node's shape while
parsing, where ruamel defers to the constructor and gets a `TaggedScalar`. Note the asymmetry — the
same `!!seq` and `!!map` on an actual collection parse fine and are already correct
(`[!!seq [1]]` → `[[1]]`), so this is specifically a collection tag on a *scalar*.

**Recorded, not worked around.** Two of them are `t.Skip`ped rows in
`models/locale/taggedrepr_test.go` carrying their measured upstream value, and the reason names
goccy. They belong with §4.2's container keys: a parser-level divergence for the human gate, not an
implementation unit.

---

## 7. Acceptance criteria

**Status: 1–3 and 6–7 are met by units A and the non-specific-tag fix that preceded it; 4 and 5 are
finding 3's and remain open.**

1. `[!!str x]`, `{a: !!str x}`, `[!unknown x]`, `[!!str 31]`, `[!!str x, !unknown y]` and
   `{a: [!!str x]}` produce the §3.4 strings, byte for byte. ✅
2. All five styles of §3.2 and all nine tag spellings of §3.3 are covered by a table-driven test
   whose expectations were measured, not written — **less the two §6.5 makes unreachable**, which
   are present as skipped rows carrying their measured value. ✅ 39 rows, 37 asserted, 2 skipped.
3. `language: !!str english` still yields `Input tag 'english'` — the top-level `__str__` case must
   not regress. ✅ asserted separately, over three tag spellings, so the fix cannot reach it.
4. Every row of §4 passes, and the four rows now `t.Skip`ped in
   `models/locale/languagerepr_test.go:75-90` are unskipped rather than deleted. ⬜ finding 3.
5. A tagged key renders as §4.1, including the two forced-tag rows. ⬜ finding 3.
6. The **theme path** renders the same as the locale path for the same shapes, since both now go
   through `schemaerr.PythonText` (§5.2). This replaces the earlier criterion "both repr
   implementations agree", which no longer has two implementations to compare: the check is now that
   unifying them did not leave the theme path reading a value the locale path renders differently.
   ✅ measured end to end rather than by unit test: a CV whose `design.theme` is `!!str classic`
   produces a **byte-identical** panel from the port and the vendored CLI (1599 bytes each, exit 1
   both), and so does one whose `locale.language` is `[!!str english, !unknown y]` — the
   `TaggedScalar(…)` text now appears inside the panel on both sides.
7. `just check` and `go test -tags conformance ./internal/schema/...` clean. ✅

---

## 8. Recommended breakdown

**They do not close together.** Two independent capabilities, and the dependency runs one way:

| Unit | Content | Depends on |
|---|---|---|
| A | `Node.Tag`, populated in `buildTagged`, consumed by `pythonRepr` — findings 1 and 2 | **done** |
| B | `Item.KeyNode`, populated in `buildMapping`, consumed by `pythonRepr` for untagged keys — finding 3, scalar half | — |
| C | tagged keys — §4.1 | A **and** B, and **may cost nothing** — see below |
| D | container keys — §4.2 | goccy; **human gate**, not an implementation unit |

A and B are independent and can run in parallel; neither reads the other's output. D is not work
until someone decides it is a declared divergence.

### 8.1 Does the unification collapse A and B? No — and why not

The unification (§5.2) halves the *consumer* half of both A and B, but it does not merge them. They
remain two defects with two upstream mechanisms — a tag on a scalar, a kind on a key — two parse
sites that do not touch (`buildTagged` and `buildMapping`), and two disjoint fixture sets: B's is
the four rows already `t.Skip`ped in `languagerepr_test.go:75-90`, A's does not exist yet. Bundling
them is the "add all X" shape `AGENTS.md` §7 forbids, and it would produce one commit that cannot
be reverted in halves. **Keep A and B separate.**

### 8.2 One real simplification: C may be free

`buildMapping` currently strips a key's tag before reading it (`untagKey`, `build.go:353-362`), so
whether C is a unit at all depends on a single decision B's implementer makes:

- if `KeyNode = buildNode(mv.Key)` — the key built by the **same** path as any value, before
  `untagKey` — then a forced tag constructs the value and an unforced one yields a `KindTagged`
  node, which is exactly §4.1's measured behavior: `{!!int 1: a}` → `{1: 'a'}` and `{!!str k: a}`
  → `{TaggedScalar(…): 'a'}`. C then falls out of A + B with no further code.
- if `KeyNode` is built from the *untagged inner* node, C needs its own arm.

**The first is both simpler and upstream's own shape** — ruamel constructs a key with the same
constructor it uses for a value — so I recommend it, and C should then be scheduled as a
**fixture-only unit** that pins §4.1 and confirms it already passes. `Item.KeyTagged` stays as the
binder's signal and is not the renderer's input; the two answer different questions and merging them
would repeat the mistake §5 avoids with `Item.Key`.

**On cost: A and B are each genuinely small** — the information is already in hand at both parse
sites, the field addition breaks nothing, and the rendering rules are ones the port already
implements for values. What is *not* small is the measurement, and that is done: every string in
§3.2, §3.3, §3.4 and §4 is recorded above and can be turned into a fixture directly.

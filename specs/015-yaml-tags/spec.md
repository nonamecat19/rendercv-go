# Spec 015 — Explicit YAML tags

**Status:** draft · **Inherits:** [`specs/000-parity-contract/spec.md`](../000-parity-contract/spec.md)
· **Extends:** [`specs/002-yaml-and-core-model/spec.md`](../002-yaml-and-core-model/spec.md) §3

**Upstream covered:**

- `src/rendercv/schema/yaml_reader.py` (the loader's configuration)
- `ruamel/yaml/constructor.py` (`RoundTripConstructor`, the thing that actually resolves a tag)

Citations to `src/...` are relative to `third_party/rendercv/`. Citations to
`ruamel/...` are relative to `third_party/rendercv/.venv/lib/python3.12/site-packages/`,
the resolved dependency the vendored submodule pins and runs.

---

## 1. Purpose

An **explicit tag** is a `!`-prefixed type annotation on a node: `!!str`, `!!int`, `!!map`,
`!unknown`. YAML allows one on any node, and upstream's round-trip loader gives each one a
defined meaning. The port's reader has no case for a tag node at all
(`internal/schema/yamlreader/build.go:243-269`), so **every tagged node becomes `KindNull`** —
its value is lost.

This breaks parity in both directions, which is why it is its own iteration rather than a patch:

- a tagged **collection** is transparent upstream, so `cv: !!map` renders a document — and the
  port rejects it. **The port refuses a CV upstream accepts.**
- a tagged **scalar** keeps its value upstream in one of two ways, and the port loses it, so the
  validation table's Input Value column reads `None` where upstream echoes the text, and a field
  that should have bound a value binds nothing.

The blocker was recorded in `specs/STATE.md` by iteration 14's 20th verification pass and
measured properly by its 22nd; this spec is the deliberate decision that pass deferred.

## 2. Inputs / Outputs

**Input:** a document whose parse tree contains at least one `*ast.TagNode`
(goccy exposes them directly — measured on ten shapes, including a tag on a mapping *key* and a
tag on the document root).

**Output:** the same `*yamldoc.Node` tree the untagged document would produce, except where the
tag changes the resolved type, plus one new node kind for the opaque case (§3.4).

## 3. Behavior

### 3.1 The mechanism upstream

`yaml_reader.py:53` loads through a plain `ruamel.yaml.YAML()` — the round-trip loader — with
exactly two customisations: `*` is not an alias (`yaml_reader.py:70-79`) and the
`tag:yaml.org,2002:timestamp` constructor is replaced by `construct_scalar`, so an ISO date stays
a string (`yaml_reader.py:83-86`).

Three constructor paths decide everything below:

1. `RoundTripConstructor.construct_unknown` (`ruamel/yaml/constructor.py:1598-1640`) —
   registered for **every** tag with no constructor of its own
   (`constructor.py:1724`, `add_constructor(None, …)`). It branches on the *node shape*, not the
   tag: a `MappingNode` becomes an ordinary `CommentedMap`, a `SequenceNode` an ordinary
   `CommentedSeq`, and only a `ScalarNode` becomes a `TaggedScalar`.
2. `construct_yaml_str` (`constructor.py:1181-1184`) — **if the node carries a tag handle it
   defers to `construct_unknown`.** This is why an explicit `!!str` behaves like an unknown tag
   and a plain string does not.
3. the per-type constructors for `!!int`, `!!float`, `!!bool`, `!!null`, `!!binary`, `!!set`,
   `!!omap`, which parse the scalar and may raise.

### 3.2 The resolution table

Measured through upstream's own configured loader
(`from rendercv.schema.yaml_reader import read_yaml`), never a default `YAML()` — the two
disagree about `!!timestamp` and the difference is upstream's override.

| Document | Python type | Value |
|---|---|---|
| `a: !!str Bob` | `TaggedScalar` | opaque, `str()` is `Bob` |
| `a: !!str 5` | `TaggedScalar` | opaque, `str()` is `5` |
| `a: !!str` (no value) | `TaggedScalar` | opaque, `str()` is `` |
| `a: !unknown b` | `TaggedScalar` | opaque, `str()` is `b` |
| `a: !!merge x` / `!!value x` / `!!yaml x` | `TaggedScalar` | opaque |
| `a: !!int 200` | `int` | `200` |
| `a: !!int 0x10` | `HexInt` | `16` |
| `a: !!int 0b101` | `BinaryInt` | `5` |
| `a: !!int 1_000` | `ScalarInt` | `1000` |
| `a: !!float 0.5` | `ScalarFloat` | `0.5` |
| `a: !!float 1` | `ScalarFloat` | `1.0` |
| `a: !!bool true` | `bool` | `True` |
| `a: !!bool yes` | `bool` | `True` |
| `a: !!null ~` | `NoneType` | `None` |
| `a: !!null x` | `NoneType` | `None` — **the text is discarded** |
| `a: !!timestamp 2001-01-01` | `str` | `2001-01-01` (upstream's override) |
| `a: !!binary aGk=` | `bytes` | `b'hi'` |
| `a: !!map {b: 1}` | `CommentedMap` | `{'b': 1}` — tag has no effect |
| `a: !!seq [1,2]` | `CommentedSeq` | `[1, 2]` — tag has no effect |
| `a: !!str [1,2]` | `CommentedSeq` | `[1, 2]` — **a tag on a collection is transparent even when it names a scalar type** |
| `a: !!str {b: 1}` | `CommentedMap` | `{'b': 1}` |
| `a: !!set {x}` | `CommentedSet` | `set(odict_keys(['x']))` |
| `a: !!omap [{x: 1}]` | `CommentedOrderedMap` | `{'x': 1}` |
| `!!str a: 1` | key is a `TaggedScalar` | the *key* is opaque |
| `!!map` on the document root | `CommentedMap` | transparent |

`!!bool`'s accepted spellings are YAML 1.1's, not 1.2's: `yes`, `no`, `y`, `n`, `true`, `false`,
`on`, `off`, matched case-insensitively (`constructor.py:432-445`).

### 3.3 Constructor failures

These raise, and the exception class decides what the user sees.

| Document | Exception | User-visible |
|---|---|---|
| `a: !!int bogus` | `ValueError` | **traceback on stderr, empty stdout, exit 1** |
| `a: !!float bogus` | `ValueError` | traceback |
| `a: !!bool bogus` | `KeyError` | traceback |
| `a: !!int` (no value) | `IndexError` | traceback |
| `a: !!map [1,2]` | `ConstructorError` | the validation table: `This is not a valid YAML file.` + `expected a mapping node, but found sequence` |
| `a: !!seq {b: 1}` | `ConstructorError` | same shape, `expected a sequence node, but found mapping` |
| `a: !!omap {b: 1}` | `ConstructorError` | same shape, `while constructing an ordered map` |

`ConstructorError` is a `MarkedYAMLError`, so it is caught with the scanner and parser errors and
becomes a validation record. `ValueError`/`KeyError`/`IndexError` are not, so they escape as
an unhandled exception — the D-011 class already recorded for `err_missing_file` and
`err_bad_override_key`.

### 3.4 What a `TaggedScalar` does downstream

A `TaggedScalar` is an ordinary Python object with no relationship to `str`, `int` or `bool`, so
**every** typed field rejects it, and pydantic's message is the field's own. Its `str()` is its
text (`constructor.py:1619-1621`, `data2.value = self.construct_scalar(node)`; measured:
`str(TaggedScalar(value='Bob'))` is `'Bob'`), which is what reaches the Input Value column and
any message that interpolates the value.

Measured end to end against the vendored CLI, `COLUMNS=80 NO_COLOR=1 TERM=dumb`:

| Document | Location | Input Value | Explanation |
|---|---|---|---|
| `cv.name: !!str Bob` | `cv.name` | `Bob` | `Input should be a valid string.` |
| `design.page.top_margin: !!str 3cm` | `design.page.top_margin` | `3cm` | `Input should be a valid string.` |
| `design.theme: !!str classic` | `design` | `...` | ``The custom theme folder `<abs>/classic` does not exist. It should be in the same directory as the input file.`` |

The third is the load-bearing one: the theme is **not** matched against the built-in literal set
(a `TaggedScalar` equals no string), so it falls through to the custom-theme path, passes the
lowercase-name pattern because that runs on `str(theme)`, and reports the *folder* message with
the tag's text interpolated. The port currently reports the *name* message with `None`.

### 3.5 What a resolved value does downstream

Nothing tag-specific: `!!int 200` on a string field is the same rejection an untagged `200`
would get (`Input should be a valid string.`, Input Value `200`), and `!!bool false` on
`design.links.underline` renders a document exactly as `false` does.

## 4. Exact user-visible strings

This spec adds **no new message**. Every string it must produce already exists in the port; the
defect is which one is chosen and what the Input Value column carries. The strings involved are
owned by specs 004 (`Input should be a valid …`), 006 (the two custom-theme-folder messages) and
002 (`This is not a valid YAML file.`).

## 5. Edge cases

1. **A tag on a collection is transparent regardless of the tag's own type** — `!!str [1,2]` is
   a sequence. Branch on the node's shape, never on the tag name (`constructor.py:1598-1610`).
2. **`!!null` discards the scalar's text**: `!!null x` is `None`.
3. **A tag on a mapping key** produces an opaque key. Upstream then binds no field by that name.
4. **A tag on the document root** is transparent; the port currently sees `KindNull` at the root
   and reports the 553-byte "The input file is empty!" panel, because
   `yaml_reader.py:55-57`'s predicate is `is None`.
5. **A tag on a colour-tuple element** (`colors.text: [!!int 10, 20, 30]`) reaches the
   design-tree validators, which have their own `Kind` switches (iteration 14's territory).
6. **An empty tagged scalar** (`a: !!str`) is a `TaggedScalar` whose value is the empty string,
   not a null.
7. `!!timestamp` is a plain string **only because upstream overrides it**; a port that follows
   ruamel's default here would produce a date and diverge.

## 6. Out of scope

Recorded, not implemented — each needs its own unit and none blocks a plausible CV:

- `!!binary` (base64 → `bytes`, a Python type the port's `Kind` set has no member for, which
  pydantic then *coerces to `str`* for a string field);
- `!!set` and `!!omap` (distinct container types with their own Input Value spellings —
  `set(odict_keys(['a']))` — and their own construction errors);
- the four `ValueError`/`KeyError`/`IndexError` constructor crashes of §3.3, which are the D-011
  unhandled-exception class;
- Python's float `repr` in the Input Value column, deferred since iteration 14's pass 13 and
  unchanged by this spec.

## 7. Acceptance criteria

Measured with the differential harness (upstream and the port on the same document, comparing
exit code, stdout with only wall-clock timings normalised, and every text artifact byte for
byte). The 24-case matrix in `§3` is the suite; **23 of 24 diverge today**.

1. Every row of §3.2's table that is *not* listed in §6 resolves to the same value, so the
   document renders identically or fails identically.
2. `cv: !!map`, `cv.sections.experience: !!seq`, a `!!map`-tagged entry, and a `!!map`-tagged
   document root each render **byte-identically** to the same document without the tag.
3. §3.4's three rows are byte-identical to upstream, including the theme-folder message.
4. `!!int bogus` and its three siblings are recorded in `specs/divergences.md` under D-011
   rather than silently rendering at exit 0, which is what the port does today.
5. No corpus case changes: `TestParity` stays at 34/42 and `just schema-diff` stays empty.

# Plan 015 — Explicit YAML tags

Go design for [`spec.md`](spec.md). Spine work (the reader), so **one owner, no fan-out**
(`AGENTS.md` §5).

---

## 1. The representation decision

Spec §3 splits tagged nodes into three behaviors, and only the third needs anything new:

| Behavior | Representation |
|---|---|
| transparent (a tag on a collection; `!!map`/`!!seq`) | none — build the inner node and drop the tag |
| type-forcing (`!!int`, `!!float`, `!!bool`, `!!null`, `!!timestamp`) | none — an existing `Kind`, chosen by the tag instead of by `ResolveScalar` |
| opaque (`!!str`, an unknown tag) | **a new `yamldoc.KindTagged`** |

### Why a new `Kind` and not a `Tagged bool` flag

The alternative — keep the resolved `Kind` and set a flag every consumer must remember to check —
was rejected. A `TaggedScalar` upstream is an object with no relationship to `str`, `int` or
`bool`, so **every** typed field must reject it; a flag makes that the caller's job at 24 `Kind`
switch sites, and the failure mode of forgetting one is a silently accepted value. A new `Kind`
inverts that: the sites that must reject it do so by *not naming it*.

That only works if the existing switches fail safe. Audited, all of them:

- `binder.isTextKind` (`binder.go:528`) returns true for `KindString` alone, so a string field
  rejects `KindTagged` with `Input should be a valid string` — upstream's message exactly.
- `design/validate.go`'s `validBoolNode` (`:386`), `validColorNode` (`:449`) and the literal,
  font-family and model arms (`:305`, `:313`, `:328`) all end in a trailing type-error `return`
  rather than a silent `nil`.
- `schemaerr.RenderInput` (`error.go:116`) ends in `return node.Raw`, which is already the right
  answer for `KindTagged` — `str(TaggedScalar)` is its text — but is made explicit (T5) so the
  next reader of that switch does not have to derive it.
- `entries/dump.go`'s `dumpValue` (`:150`) defaults to `node.Raw`. Unreachable for `KindTagged`,
  since a tagged value never survives validation, and harmless if it ever became reachable.

One site does **not** fail safe and is the reason T6 exists: `design.ValidateTheme`
(`design.go:89-94`) matches the built-in theme set against `themeNameRepr(node)`, a plain string
comparison, so `design.theme: !!str classic` would match `classic` and render. Upstream's match
is pydantic's discriminated union on the *object*, which a `TaggedScalar` fails, sending it to the
custom-theme path (`design.py:57`, `theme_name = str(design["theme"])`, then the folder checks).
The built-in loop therefore gains a `Kind == KindString` guard.

`KindTagged` is appended **after** `KindSequence` so no existing constant's `iota` value moves.

## 2. Tag resolution

New in `internal/schema/yamlreader`, beside `ResolveScalar` (`resolve.go:13`) because it is the
same question — what type is this scalar — asked with an explicit answer supplied:

```go
// tagBehavior is what a tag does to the node it is attached to.
type tagBehavior uint8

const (
    tagTransparent tagBehavior = iota // the tag is dropped; the node resolves normally
    tagOpaque                         // ruamel's TaggedScalar
    tagForceKind                      // the tag names the kind
)

func resolveTag(tag string, inner *yamldoc.Node) (yamldoc.Kind, tagBehavior)
```

`buildNode` gains one case, placed before the scalar cases:

```go
case *ast.TagNode:
    return buildTagged(v)
```

`buildTagged` branches on the **node's shape first, the tag second** — spec §5.1, because
`!!str [1,2]` is a sequence. goccy exposes the tag text as `v.Start.Value` (`!!str`, `!unknown`,
`!!python/object` — measured on ten shapes, including a tag on a mapping key and on the document
root).

The forcing table:

| Tag | Forced kind |
|---|---|
| `!!int` | `KindInt` |
| `!!float` | `KindFloat` |
| `!!bool` | `KindBool` |
| `!!null` | `KindNull` |
| `!!timestamp` | `KindString` (upstream's own override, `yaml_reader.py:83-86`) |

Everything else on a scalar is opaque. `!!bool`'s spellings are YAML 1.1's
(`yes`/`no`/`y`/`n`/`on`/`off` as well as `true`/`false`, case-insensitive,
`constructor.py:432-445`) and are wider than `ResolveScalar`'s plain-scalar set, so the forced
`KindBool` carries its raw text and the existing bool consumers — `dumpValue`, `RenderInput`,
`design.normalizeBools` — must read `yes`/`on` as true. `dumpValue` already does
(`dump.go:162`); `RenderInput` and `pythonBoolRepr` do not and are corrected in T3, where the
divergence is introduced.

`!!null` **discards the scalar's text** (spec §5.2), so the forced node carries an empty `Raw`.

## 3. Tagged mapping keys

`yamldoc.Item.Key` is a `string` and stays one. A tagged key is not a field name upstream — it is
an unhashable-by-name object, and pydantic reports `Keys should be strings.` against the
*enclosing* mapping (measured on `cv: {!!str name: …}`). So `Item` gains:

```go
// KeyTagged marks a key written with an explicit tag, which upstream resolves
// to a TaggedScalar rather than a str (spec 015 §3.4).
KeyTagged bool
```

and the binder's mapping walk reports one record per tagged key, with the message above, the
key's text as the input, and the enclosing mapping's location — never binding the field. This is
a message the port has never emitted; it is pydantic-core's `invalid_key`, and it appears in no
upstream source file because pydantic owns the text.

## 4. Files touched

| File | Change |
|---|---|
| `internal/schema/yamldoc/node.go` | `KindTagged`; `Item.KeyTagged` |
| `internal/schema/yamlreader/resolve.go` | `resolveTag` and the tag table |
| `internal/schema/yamlreader/build.go` | the `*ast.TagNode` case; tagged keys in `buildMapping` |
| `internal/schema/schemaerr/error.go` | `RenderInput`'s explicit `KindTagged` arm; YAML-1.1 bool spellings |
| `internal/schema/models/design/design.go` | the built-in-theme guard; `pythonBoolRepr`'s spellings |
| `internal/schema/binder/binder.go` | the tagged-key record |
| `specs/divergences.md` | the four out-of-scope tags and the constructor crashes |

## 5. What this plan deliberately does not do

Spec §6's out-of-scope list, unchanged: `!!binary`, `!!set`, `!!omap`, the four constructor
crashes, and Python's float `repr`. Each needs its own unit and none blocks a plausible CV. They
are `divergences.md` entries, so the last task writes them directly.

A forced `KindInt` whose text is not a number (`!!int bogus`) is **kept** rather than made
opaque: upstream crashes with a `ValueError` traceback at exit 1 and empty stdout, and a wrong
validation record at exit 1 is closer to that than the exit-0 render the port produces today.
Recorded with the other crashes.

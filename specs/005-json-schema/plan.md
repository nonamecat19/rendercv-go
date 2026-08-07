# Iteration 5 — plan

Go design for `spec.md`. Behavior lives there; this file is packages, types and tradeoffs.

---

## 1. The shape of the problem

Upstream's generator is 45 lines because pydantic does the work: the schema is *reflected* out of
the model classes. The Go port has no such reflection — its validators are hand-written functions
over `yamldoc.Node`, and they carry field names but not types, defaults, examples or titles.

So the schema is **hand-built data, diffed against upstream**, which is what `AGENTS.md` §2 already
says of this subsystem. That makes the design question not "how do we reflect" but "where does the
data live, and what stops it drifting".

Three candidates, with the choice last:

| Where | Cost | Drift risk |
|---|---|---|
| A. One big literal in `internal/schema/jsonschema` | lowest to write | high — the schema lives nowhere near the model it describes |
| B. Struct tags on Go model types | idiomatic-looking | the models are node-holders, not typed structs; there is nothing to tag |
| C. A `Schema()` descriptor beside each model | one function per model | low — it sits in the file it describes, and the per-`$defs` diff catches the rest |

**C.** The deciding argument is the same one iteration 3 used for `Descriptor`: a model's field
order already lives beside the model, and putting its schema anywhere else creates a second place
to update. B is not available — `Cv` is a struct of `*yamldoc.Node`, so a tag would describe the
node, not the field.

---

## 2. Packages

```
internal/schema/jsonschema/
  jsonschema.go        Object, the ordered map, and Generate
  marshal.go           the serializer of spec §3.4
  jsonschema_test.go
  golden_conformance_test.go   the per-$defs diff against the submodule
```

Each model package gains one file:

```
internal/schema/models/cv/schema.go              Cv, SocialNetwork, CustomConnection, Section
internal/schema/models/cv/entries/schema.go      the nine entry types, ListOfEntries
internal/schema/models/schema.go                 RenderCVModel — the top level
```

`jsonschema` imports **nothing** from `models`; the dependency runs the other way, so the
serializer can be tested on hand-built objects.

---

## 3. The ordered object

JSON Schema is order-sensitive here (`spec.md` §6) and Go maps are not ordered, so the type is:

```go
// Object is a JSON object that remembers its key order. Every schema node is
// one, because spec 005 §6 makes order contractual in three different ways at
// once: sorted almost everywhere, declaration order inside `properties`, and a
// fixed non-sorted sequence at the top level.
type Object struct {
    keys   []string
    values map[string]any
}

func (o *Object) Set(key string, value any) *Object   // append or overwrite in place
func (o *Object) Sort() *Object                        // ASCII, for everything but §6 rules 1 and 3
```

**`Set` overwrites in place rather than moving the key to the end.** That is not a convenience: it
is exactly how the top-level `title` keeps its position in the sorted run while `description`,
`$id` and `$schema` land after it (`spec.md` §3.1 behavior 6). Writing `Set` the other way
reproduces every key and gets the order wrong.

Rejected: `encoding/json` with `json.RawMessage` assembled by hand. It would make the ordering
implicit in string concatenation, which is unreviewable, and `ensure_ascii=False` plus the
separators would still need a custom encoder.

---

## 4. Serialization

`json.dumps(schema, indent=2, ensure_ascii=False)` is not `encoding/json`'s output, in three ways
that all matter:

| Python | Go's `json.MarshalIndent` | What the port does |
|---|---|---|
| `": "` after a key | `": "` | same |
| `", "` between items | `",\n"` under indent | same under indent; no compact form is emitted |
| non-ASCII literal | escapes `<`, `>`, `&` by default | `Encoder.SetEscapeHTML(false)`, and no `\uXXXX` |
| no trailing newline | `Encoder` appends one | write through a buffer and trim |

So `marshal.go` is a small hand-written encoder over `Object`, not a wrapper. It handles exactly
the value kinds a schema contains: `*Object`, `[]any`, `string`, `bool`, `int`, and `nil` — the
last because `"description": null` is emitted rather than omitted (`spec.md` §5 behavior 18).

A `nil` that means "absent" is spelled by not calling `Set`. There is no `omitempty`.

---

## 5. `$defs` naming

`spec.md` §3.3's two rules become one function:

```go
// DefName returns the $defs key for a model: its bare class name when unique,
// its module path with `.` → `__` when not.
func DefName(class, module string) string
```

The port cannot *derive* uniqueness — it has no module graph — so each model states its own name
and the conformance test checks it against upstream's key set. That is the same trade as the
field orders of iteration 3: generated fixture, mechanical check.

**The collision suffix is not implemented** (`spec.md` §7.2). `DefName` panics if asked for one,
with a message naming iteration 6, because a silent `__1` would be a wrong answer that looks
right.

---

## 6. What the gate is

`golden_conformance_test.go` reads `third_party/rendercv/schema.json` and, for each of the
eighteen `$defs` of `spec.md` §8:

1. serializes the port's version of that one def;
2. serializes upstream's, through the **same** encoder;
3. compares bytes.

Re-serializing upstream's side is deliberate. Comparing against the raw file substring would make
the test sensitive to where the def sits in the file; comparing parsed values would lose the key
order that is half the contract. Round-tripping both through one encoder tests exactly the two
things that matter — the keys, and their order — and nothing else.

A second test asserts the **absent** set: every `$defs` key upstream has that the port does not,
listed explicitly. It fails if the list shrinks without the models landing, which is what stops
iterations 6 and 7 forgetting to close Axis 3.

---

## 7. Hazards

1. **`Set` semantics.** §3. The top-level key order is the only place it shows, and it shows as a
   three-key difference at the end of a 405 KB file.
2. **Escaping.** Go escapes `<`, `>` and `&` unless told not to. `schema.json` contains none of
   the three today, so a wrong encoder passes the current gate and breaks when a description gains
   an ampersand. Mitigated by a unit test on the encoder itself, not only through the diff.
3. **`description: null` versus absent.** Both are falsy in Go and one is wrong. Mitigated by
   `Set(key, nil)` being the only way to get `null`, and by `BulletEntry`'s def carrying one.
4. **Enum order.** `spec.md` §7.4. Taken from the Go lists that already exist, which are already
   pinned against upstream — so a reordering fails two tests, not none.

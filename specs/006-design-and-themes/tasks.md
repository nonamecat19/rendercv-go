# Iteration 6 — tasks

Commit-sized units. Each leaves `go build ./... && go test ./...` green.

`[parallel]` marks a leaf that reads no other task's output; everything else is the spine and stays
with one owner (`AGENTS.md` §5).

---

## Wave A — the three value types — **done**

Each is a leaf: a pattern or a library rule, its message, and its code. None reads the others.

**Two changes to this wave as written, both recorded rather than edited away:**

1. **T7's tool arrived early.** T2 needs the 147 CSS colour names, which are exactly the kind of
   data `plan.md` §2 says to generate, so `tools/designprobe` landed with its colour mode before
   Wave B rather than after. Its remaining modes — the field tree and the overrides — are still
   T7's and T8's.
2. **A shared unit came first.** `binder.LiteralMessage` was `cv`'s, and T4 is the second caller,
   so it moved to `binder` in its own commit before T4 landed. `AGENTS.md` §7 forbids bundling a
   refactor with the feature that motivates it.

### T1 — `TypstDimension` · `[parallel]` — **done**
`typstdimension.go`: the full match of `-?\d+(?:\.\d+)?(cm|in|pt|mm|em)` and §4A.1's message,
coded `rendercv_other_error` through `schemaerr.Coded`.
Spec §3.1 behavior 9.
Tests: `1cm` and `-0.5in` pass; `1`, `1px` and `1 cm` fail with the literal text.

### T2 — `Color` · `[parallel]` — **done**
`color.go`: the library's `color_error`, §4A.2's raw message, and the `as_rgb()` rendering the
templates need.
Spec §3.1 behaviors 10 and 11.
Tests: `notacolor` and `#gggggg` give the same text; the message reaches spec 004 §4.11's `)".`
ending **through the pipeline**, which is the first live producer for dictionary row 13.

### T3 — `FontFamily` accepts anything · `[parallel]` — **done**
`fontfamily.go`: the seventeen names in `sorted()` order and the free-string arm that makes any
name valid.
Spec §3.1 behaviors 12 and 13.
Tests: an unlisted name validates; the seventeen are in sorted order, not source order.

### T4 — the six `Literal` unions · `[parallel]` — **done**
`literals.go`: `Bullet`, `BodyAlignment`, `Alignment`, `SectionTitleType`,
`PhoneNumberFormatType`, `PageSize`, each in **declaration** order.
Spec §2 behavior 5.
Tests: the orders, and `Bullet`'s non-ASCII members surviving a round trip.

---

## Wave B — the tree

### T5 — the tree shape · `[sequential]`
`tree.go`: `Model`, `Field`, `Kind`. No data.
Plan §3, §5.

### T6 — the 161-row differential, red · `[sequential]`
**Revised from "the submodule diff".** A test comparing the generated tree against the same
introspection that generated it could not fail — `tools/localeprobe`'s stated blind spot, and here
it would be the *whole* check rather than half of it.

The gate is the per-`$defs` differential that already exists. `schema.json` is a **total**
projection of the tree: every field, default, description, example and title appears in it, so a
wrong default is a byte mismatch and there is nothing a separate diff would catch that this does
not. Landing it red means declaring an empty `design.SchemaDefs`, wiring it into `portDefs`, and
moving the absent count from 227−63 to 227−224 — which fails until Wave D lands.

### T7 — `tools/designprobe` and the generated tree · `[sequential]`
Introspects `ClassicTheme` through `uv` and emits `tree_generated.go`: twenty-two models, every
field, default and description. `just designprobe` reruns it.
Plan §2. **The tool's head states what the diff does and does not check**, matching
`tools/localeprobe` — including that it and the test share a parser.

### T8 — the eight override maps · `[sequential]`
`overrides_generated.go` from the same tool, plus `Themes` in union order: `classic` then the eight
sorted stems.
Spec §1 behavior 2, §5 criterion 7.
Test: `Themes` derived from the glob, as `TestLanguagesAreInUnionOrder` does for locale.

---

## Wave C — validation

### T9 — the recursive binder · `[sequential]`
`validate.go`: one walk of the tree, `binder.ForbidExtra` at every level, leaf kinds dispatching to
Wave A.
Spec §3 behavior 4. Plan §5.

### T10 — the effective tree per theme · `[sequential]`
`variants.go`: an override map applied to the base tree, deep-merged, so a theme validates against
its own defaults.
Spec §1 behavior 1.

### T11 — wire `design` into the model · `[sequential]`
`rendercvmodel.go` reaches only `ValidateTheme` today, so nothing can reach the option tree — the
same gap iteration 7's verifier found in `locale` (`STATE.md`, iteration 7).
Spec §3 behavior 6.
Tests: a built-in theme with a bad option reports **that option**, not "unknown theme".

### T12 — the two coercions · `[sequential]`
`validate_font_family`'s string widening and `convert_section_titles_to_snake_case`.
Spec §3.2 behaviors 14 and 15.
Tests: `font_family: Roboto` and the full mapping produce the same model;
`["Work Experience"]` becomes `["work_experience"]`.

---

## Wave D — the schema

### T13 — the ordinal assignment · `[sequential]`
`variants.go`: `(model, theme) → ordinal` by walking the tree depth-first inside the theme loop.
Plan §4. **Two hazards are the unit's whole point**: a model no theme overrides carries **no**
suffix, and a theme that omits a key points back at `__1`.
Tests: `Page` has six, `Links` eight, `OneLineEntry` none; `HarvardTheme.links` refs `__1`.

### T14 — the 161 `$defs` · `[sequential]`
`schema.go`. Turns the 161 rows T6 made red green. The absent count is already 227−224 by then.

### T15 — the three settings `$defs` · `[sequential]`
`Settings`, `RenderCommand`, `PlannedPathRelativeToInput` — unowned by any iteration's spec today
and the last three between the port and a green Axis 3. Small enough to land here rather than
leave Axis 3 open on three entries.
**Update the absent count to 0** and `just schema-diff` exits 0.

### T16 — close the ledger · `[sequential]`
`specs/STATE.md`: iteration 6 green, Axis 3 **closed**, Wave E recorded as cut scope.

---

## Wave E — deferred, with the reason

**Custom themes (D-002's Lua path) and spec §3 behavior 7's second and third messages are not in
this iteration.** The spec places them here; `plan.md` §7 moves them out, because the sandbox is a
subsystem of its own and bundling it with 161 `$defs` makes both unreviewable.

This is a scheduling decision, not a divergence — `divergences.md` already carries D-002 and is
untouched. `STATE.md` records it as cut scope with this reason, and the two messages keep their
`TODO` naming the iteration that owns them.

# Iteration 7 — plan

Go design for `spec.md`. Behavior lives there.

---

## 1. Where the catalog data lives

Twenty-two catalogs × ten fields, of which two are twelve-element lists. Roughly 300 strings, half
of them non-ASCII.

The same three candidates as always, and the same answer as `error_dictionary.yaml`:

| Where | Drift risk |
|---|---|
| A. `go:embed` the submodule's YAML | **not available** — the submodule is absent at runtime |
| B. Copy the YAML into the Go tree and embed that | copy is untested unless something diffs it |
| C. Go source, diffed against the submodule | same as B, but the data is reviewable in the language it is used from |

**C**, matching iteration 4's dictionary (spec 004 plan §3). B and C carry identical transcription
risk and identical mitigation — a submodule-diff test — so the tiebreak is that Go source shows up
in `go doc` and in review diffs as data rather than as an opaque blob.

`require_all_fields=True` (`spec.md` §1 behavior 3) makes the diff strict for free: every variant
must supply every field, so a missing one is a difference rather than an inherited default. The
test does not need to know which fields are optional, because none is.

---

## 2. Packages

`internal/schema/models/locale` already exists — iteration 4 put the twenty-two language names and
the discriminator check there for spec 004 §4.30. It gains:

```
locale.go        (exists) Languages, ValidateLanguage
catalog.go       the ten-field model and its validation
catalogs.go      the twenty-two catalogs as data
schema.go        the 45 $defs
catalogs_conformance_test.go   the submodule diff
```

**`Languages` is not duplicated.** It is already the source for spec 004 §4.30's message, and
`catalogs.go` must derive from it rather than restating it, so a twenty-third language is one
edit. `spec.md` §5's second criterion is that assertion.

---

## 3. The two length messages

`spec.md` §3 behavior 10 has two, distinguished by which bound was violated and interpolating the
actual count:

```go
// tooShort and tooLong are pydantic's own, and there are two because the
// bound that was violated decides the wording. Both interpolate the count,
// so neither is a constant.
fmt.Sprintf("List should have at least %d items after validation, not %d", want, got)
fmt.Sprintf("List should have at most %d items after validation, not %d", want, got)
```

Neither matches a dictionary row, so the pipeline only appends a period. They are the **only** two
strings this iteration adds — the locale package raises nothing of its own (`spec.md` §3
behavior 9).

Codes are `too_short` and `too_long`, which is a third and fourth code for
`binder`'s table; they reach a record through the `schemaerr.Coded` interface iteration 4 added
for URLs.

---

## 4. The `$defs`

45 entries: `Locale`, twenty-two `<Language>Locale` models, and twenty-two `Phrases` models that
collide and therefore carry the `__1`…`__22` suffixes of spec 005 §3.3 behavior 12.

**This iteration must implement the collision numbering** that spec 005 §7.2 deferred — it is the
first place a collision actually occurs, and `jsonschema.DefNameWithSuffix` panics today naming
iteration 6. Two consequences:

- the panic's message needs updating: whichever of 6 and 7 lands first implements it;
- the numbering is **emission order**, not alphabetical, so it needs the order pydantic walks the
  union in — which is `Languages`' order, already ported.

That last point is why this iteration is a better place to build it than iteration 6: the locale
union's emission order is a flat list of twenty-two, while the design union's is nine themes ×
their nested models. Same rule, far easier to get visibly right.

---

## 5. Hazards

1. **Non-ASCII.** `måned`, `nuværende`, `norwegian_bokmål`. Go source is UTF-8 so the data is
   fine; the risk is the schema encoder, which spec 005 already covers with a literal-output test.
2. **`June`, `July`, `Sept`.** `spec.md` §2 behavior 7. A port that generates abbreviations by
   slicing three characters gets three of twelve wrong, and eleven of twenty-two catalogs would
   still look right.
3. **The twelve-element constraint is `min` *and* `max`.** Implementing it as `!= 12` produces one
   message where upstream produces two.
4. **`Languages` drift.** §2. Two consumers, one list.

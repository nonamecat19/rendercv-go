# Plan 003 — Entry types

Go design for [`spec.md`](spec.md). Behavior claims live there; this file decides code.

---

## 1. Dependency decisions

**None added.** Everything in this iteration is stdlib plus what iteration 2 already imports
(`goccy/go-yaml`, transitively, through `yamlreader`).

Two candidates were considered and rejected:

**A regex engine matching pydantic's.** `spec.md` §3.11 requires `\b10\..*` with pydantic's
semantics, and pydantic-core uses the Rust `regex` crate, whose `\b` is Unicode-aware. Go's
`regexp` gives `\b` ASCII semantics only, so the literal pattern accepts `ü10.5` and `ß10.5`, which
upstream rejects (`spec.md` §5.1). The fix is nine lines, not a dependency — see §5.

**A URL library for `PublicationEntry.url`.** Deferred by `spec.md` §7.3 to iteration 4, where the
same decision covers `cv.email`, `cv.phone`, `cv.website` and `cv.photo`. Committing a library here
would pre-empt that decision for three fields this iteration does not own.

---

## 2. The package extraction that unblocks `models.Validate → cv.Validate`

### 2.1 The cycle

Today: `models` owns `ValidationContext` (`models/validationcontext.go`) and the two path types
(`models/path.go`); `models/cv` imports `models` for both (`cv/cv.go:5` for
`Options.Context`, `cv/customconnection.go:5` for `ExistingPathRelativeToInput` and
`ResolveExistingPath`). So `models` cannot import `cv`, and `models.Validate` cannot call
`cv.Validate`. This is carried item 2 of `specs/STATE.md`.

### 2.2 The move

Both types are leaves: neither reads anything from `models`, and `path.go` reads only
`ValidationContext`. They move down.

```
internal/schema/models/
  valctx/                 ← schema/models/validation_context.py
    valctx.go               ValidationContext, InputPath(), Today()
    valctx_test.go
  inputpath/              ← schema/models/path.py
    inputpath.go            ExistingPathRelativeToInput, PlannedPathRelativeToInput,
                            ResolutionBase, ResolveExistingPath, ResolvePlannedPath, Serialize
    inputpath_test.go
  rendercvmodel.go        now imports valctx and cv
  base.go, …              unchanged
  cv/                     now imports valctx and inputpath, no longer imports models
```

Resulting edges: `valctx` → nothing; `inputpath` → `valctx`, `schemaerr`; `cv` → `valctx`,
`inputpath`, `entries`, `binder`, `schemaerr`, `yamldoc`; `models` → `cv`, `valctx`, `entries`.
Acyclic.

**Package names.** `valctx` and `inputpath` rather than `validationcontext` and `path`: the second
of each pair is either unreadable at every call site or shadows a stdlib package name that
`inputpath.go` itself imports (`path/filepath`). Both remain under `models/`, so `AGENTS.md` §9's
mirror-upstream rule still reads cleanly: `schema/models/validation_context.py` →
`internal/schema/models/valctx/valctx.go`.

**No aliases.** `models` will *not* re-export the moved types via `type X = valctx.X`. Aliases
would keep the old call sites compiling and hide which package actually owns what; the move is
mechanical (six non-test references, listed in `tasks.md` T1/T2) and is cheaper done once. The two
test files move with their subjects.

**Two commits, not one.** `inputpath` depends on `valctx`, so `valctx` moves first. Each move is a
pure `refactor:` commit with no behavior change and no test edits beyond the package clause and
imports.

### 2.3 Wiring the call

`models.Validate` gains the `cv` recursion:

```go
func Validate(node *yamldoc.Node, ctx *valctx.ValidationContext, source schemaerr.YamlSource) (*RenderCVModel, []schemaerr.ValidationError)
```

After binding the four top-level keys it validates the `cv` node at schema location `["cv"]` with
`cv.Options{Registry: entries.Default(), Context: ctx}`, appending the resulting errors after the
top-level binder's own. `RenderCVModel` grows a typed member `CvModel *cv.Cv` alongside the raw
`Cv *yamldoc.Node` node it already keeps; the raw node stays because iteration 4 resolves
coordinates against it. `design`, `locale` and `settings` keep their placeholder nodes
(iterations 6, 7).

An **absent** `cv` key is not an error (spec 002 §3.28): the recursion runs only when the key is
present. A present-but-null `cv` binds through `binder.Bind`'s non-mapping branch, which already
produces the model-type error.

---

## 3. Package layout for the entry types

Upstream has one file per type at `schema/models/cv/entries/` plus a `bases/` subpackage, and both
`__init__.py` files are empty (`spec.md` §3.1 behavior 4). The port mirrors that exactly: the eight
concrete types are files in the existing `entries` package, next to `registry.go`.

```
internal/schema/models/cv/entries/
  registry.go              (exists) TypeName, Descriptor, Registry — unchanged
  default.go               (new)    Default() *Registry, in union order; Validate() dispatcher
  bullet.go                (new)    ← entries/bullet.py
  numbered.go              (new)    ← entries/numbered.py
  reversednumbered.go      (new)    ← entries/reversed_numbered.py
  oneline.go               (new)    ← entries/one_line.py
  normal.go                (new)    ← entries/normal.py
  experience.go            (new)    ← entries/experience.py
  education.go             (new)    ← entries/education.py
  publication.go           (new)    ← entries/publication.py
  doipattern.go            (new)    the Unicode word-boundary matcher of §5
  bases/                   (exists) entry.go, entrywithdate.go, entrywithcomplexfields.go,
                                    complexfieldsentry.go
```

`entries` imports `bases`, `binder`, `schemaerr`, `yamldoc`, stdlib. It imports **nothing** from
`cv`, which is what keeps `cv` → `entries` one-directional (spec 002 plan §5).

### 3.1 One type, one shape

Every concrete type follows the same three-part shape. `EducationEntry` in full; the other seven
differ only in their own fields.

```go
// EducationEntry mirrors EducationEntry (entries/education.py:26) and its own-field
// base BaseEducationEntry (entries/education.py:7-22).
type EducationEntry struct {
    bases.BaseEntryWithComplexFields

    Institution *yamldoc.Node
    Area        *yamldoc.Node
    Degree      *yamldoc.Node
}

// educationOwnFields is the own-field order of spec §3.9. It precedes the base's
// fields because upstream declares `class EducationEntry(BaseEntryWithComplexFields,
// BaseEducationEntry)` and pydantic emits the last-listed base first
// (education.py:25-26, spec §3.2).
var educationOwnFields = []binder.Field{
    {Name: "institution", Required: true, Value: binder.ValueString},
    {Name: "area", Required: true, Value: binder.ValueString},
    {Name: "degree", Value: binder.ValueString},
}

func EducationDescriptor() Descriptor
func ValidateEducationEntry(
    node *yamldoc.Node, location []string, source schemaerr.YamlSource, reference time.Time,
) (*EducationEntry, []schemaerr.ValidationError)
```

- **Declared fields are `*yamldoc.Node`, not `string`.** Consistent with iteration 2's `Cv`, and
  required: iteration 4 needs each value's span to place its error, and iteration 8 needs the raw
  text with no normalization (`spec.md` §6.5). A typed accessor per field (`func (e
  *EducationEntry) InstitutionText() string`) is added only when a consumer needs it, which is
  iteration 8, not now.
- **`Descriptor().Fields` is own fields then base fields**, assembled from the base helpers
  iteration 2 already exposes: `bases.DateFieldNames()` and `bases.ComplexFieldNames()`. So
  education is `institution, area, degree` + `date` + `start_date, end_date, location, summary,
  highlights` — exactly the verified runtime order (`spec.md` §3.9). Publication is its six own
  fields + `date` only, because it embeds `BaseEntryWithDate` (`spec.md` §3.10).
  The order is asserted positionally by a test, never recomputed at runtime from a set.
- **Binding** delegates to the base binder that matches the type's upstream base:
  `bases.BindEntry` for the four `BaseEntry` types, `bases.BindEntryWithDate` for
  `PublicationEntry`, `bases.BindEntryWithComplexFields` for the other three. Each already takes
  an `extraFields []binder.Field` argument, so no base changes shape. The own fields are passed
  there, and the *order* of the descriptor is maintained separately — binding order does not
  affect the field-order surface, but error order for missing fields does, so the own fields are
  passed first and iteration 2's binder reports missing fields in the order of `Spec.Fields`
  (`binder.go:144`). That gives `institution` before `area` (`spec.md` §5.8) for free.

### 3.2 `TextEntry` has no type

`TextEntry` is a string with no model (`spec.md` §3.14). It gets **no** Go struct — adding one
would invent a field surface upstream does not have and would leak into iteration 5's schema. It
stays what iteration 2 made it: a `TypeName` constant (`cv.TextEntry`) plus the string branch of
`cv.InferEntryType`. Its validator is the identity: a string node is always valid.

### 3.3 The dispatcher and the default registry

```go
// Default returns the registry in the union order of spec §3.1 behavior 2. The
// order is load-bearing and is written out literally, not sorted or derived.
func Default() *Registry {
    return NewRegistry(
        OneLineDescriptor(), NormalDescriptor(), ExperienceDescriptor(),
        EducationDescriptor(), PublicationDescriptor(), BulletDescriptor(),
        NumberedDescriptor(), ReversedNumberedDescriptor(),
    )
}

// Validate dispatches one entry to its type's validator.
func Validate(
    node *yamldoc.Node, name TypeName, location []string,
    source schemaerr.YamlSource, reference time.Time,
) []schemaerr.ValidationError
```

`Default()` returns a fresh registry per call rather than a package-level singleton, so no test can
mutate another's. `Characteristic()` is recomputed per registry; it is eight small set operations
and this is not a hot path (`registry.go:41`).

`Validate` is a `switch` on `TypeName` with nine arms plus a default. The default arm is an
`InternalError`, not a silent pass: an unregistered name reaching it means the registry and the
dispatcher disagree, which is a defect. A test asserts every `Default().Names()` entry has an arm.

### 3.4 Replacing the stub validator in `cv`

`cv.EntryValidator` (`internal/schema/models/cv/sectionvalidation.go:37-48`) gains the reference
date and defaults to the real dispatcher:

```go
type EntryValidator func(
    node *yamldoc.Node, entryType entries.TypeName, location []string,
    source schemaerr.YamlSource, reference time.Time,
) []schemaerr.ValidationError

var entryValidator EntryValidator = entries.Validate
```

`ValidateSection` grows a `reference time.Time` parameter, threaded from
`Options.Context.Today()` at the one call site in `cv.validateFields`. The
`SetEntryValidatorForTest` seam in `export_test.go` stays — it is how the section rules of spec 002
§3.53–§3.61 are tested in isolation — but no production path uses it (`spec.md` §3.19
behavior 45).

`cv.SectionRecords` needs no change: it infers from the registry, and the registry is now real.

---

## 4. Extending the binder with value types

`spec.md` §3.13 needs three shapes upstream declares and iteration 2's binder does not check.
`binder.Field` grows one member:

```go
// ValueType is the declared shape of a field's value. It is deliberately not a
// general type system: upstream's entry models declare exactly three shapes.
type ValueType uint8

const (
    ValueAny        ValueType = iota // no check — the field is bound as a raw node
    ValueString                      // `str`
    ValueStringList                  // `list[str]`
)

type Field struct {
    Name     string
    Required bool
    Value    ValueType
}
```

`ValueAny` is the zero value, so every existing `Field{Name: …}` literal keeps its current
behavior and the change is additive.

Checking rules, applied in `Bind` after presence resolution and before the missing-field pass:

| Declared | Value | Result |
|---|---|---|
| any | anything | no check |
| `ValueString` | null, field optional | no check (pydantic's `str \| None`) |
| `ValueString` | null, field required | `string_type` at the field (`spec.md` §5.7) |
| `ValueString` | string / int / float / bool | see below |
| `ValueString` | mapping or sequence | `string_type` at the field |
| `ValueStringList` | null, field optional | no check |
| `ValueStringList` | anything not a sequence | `list_type` at the field |
| `ValueStringList` | sequence | each element checked as `ValueString`, error located at the element's index |

**Non-string scalars in a string field are an open sub-case.** Pydantic in strict-off mode still
rejects `int` for `str` (`{'type': 'string_type', 'msg': 'Input should be a valid string'}` —
verified for `summary: 5`), so `ValueString` rejects `KindInt`, `KindFloat` and `KindBool`. This
matters because the YAML reader classifies `2020` as `KindInt` (spec 002 plan §3), so
`location: 2020` must fail. A test pins each of the four scalar kinds.

Element locations use the index as a decimal string, matching upstream's own error fixture, where
an entry index appears as `"1"` in `schema_location`
(`tests/schema/testdata/test_pydantic_error_handling/expected_errors.yaml:56-57`). Iteration 2's
`elemLocation` already does this; the binder reuses the same helper shape.

**Why in the binder and not in each type.** The three shapes recur across eleven fields in eight
types plus three inherited ones. Duplicating the check per field would make the error text, code
and location a per-file decision, which is exactly what `AGENTS.md` §9 forbids for contractual
error output.

---

## 5. The `doi` pattern

`spec.md` §3.11 and §5.1. Go's `regexp` cannot express pydantic's `\b`, so the check is hand
written in `entries/doipattern.go`:

```go
// matchDOIPattern reports whether value matches pydantic's `\b10\..*`
// (publication.py:34). Search semantics, not anchored (spec §3.11 behavior 18).
//
// pydantic-core evaluates the pattern with the Rust `regex` crate, whose `\b` uses
// the Unicode `\w` class — not Go's ASCII-only `\b`. Verified against the vendored
// Python: `①10.5` matches (U+2461 is category No, outside `\w`) while `ü10.5` and
// `ß10.5` do not. Go's `regexp` with the literal pattern accepts both and diverges
// (spec §5.1).
func matchDOIPattern(value string) bool
```

Implementation: scan every byte index where `value[i:]` begins with `"10."`, and accept at the
first such index whose preceding rune and `'1'` straddle a word boundary — i.e. `i == 0`, or the
rune ending at `i` is not a word rune. Word runes follow the Rust crate's `\w` =
`\p{Alphabetic} ∪ \p{M} ∪ \p{Nd} ∪ \p{Pc} ∪ \p{Join_Control}`, expressed with Go's tables as
`unicode.IsLetter(r) || unicode.Is(unicode.Nl, r) || unicode.Is(unicode.Other_Alphabetic, r) ||
unicode.Is(unicode.Nd, r) || unicode.Is(unicode.Pc, r) || r == 0x200C || r == 0x200D`.
`'1'` is always a word rune, so no forward check is needed.

The sixteen-row table of `spec.md` §3.11 behavior 19 is the test, and it is the acceptance gate for
this function. `ü10.5`, `ß10.5`, `_10.5`, `9910.5` (rejections) and `①10.5`, `-10.5`, `.10.5`
(acceptances) are the rows that distinguish a correct implementation from `regexp`.

---

## 6. `PublicationEntry`'s model-level rules

Three rules, in upstream's order (`publication.py:46-96`), run after field binding:

1. `doi` present → `url` set to nil (`spec.md` §3.12 behavior 21). Silent.
2. `DOIURL()` returns `"https://doi.org/" + doi.Raw` or `""` for absent
   (`spec.md` behavior 22). It is a method, not a field, so it cannot appear in
   `Descriptor().Fields`. Upstream caches it (`functools.cached_property`); the Go method is pure
   and recomputes — a two-string concatenation, and caching it would need a mutable member that
   iteration 5 would have to learn to ignore.
3. The `doi_url` length check (`spec.md` behavior 23): if `len(DOIURL()) > 2083`, emit the
   `url_too_long` error of `spec.md` §4.2 with an **empty** schema location — verified as
   `loc: []` upstream, so the error carries no field name. This is the only entry-level error in
   the iteration whose location is not a field path, and it is a deliberate fidelity detail, not a
   bug.

`url` itself is bound as `ValueAny` — a raw node — with a `TODO(iteration-4)` naming `spec.md`
§7.3. No parsing, no normalization, no error. The hook shape iteration 2 established for
`email`/`phone`/`website` (`internal/schema/models/cv/scalarorlist.go`'s `elementValidators`) is
the model: iteration 4 registers one HTTP-URL validator and all four fields get it at once.

---

## 7. Known hazards (`AGENTS.md` §6)

Hazards 2, 3, 4 (whitespace control, custom filters, loader order) are the templater's and land in
iteration 8. Hazard 5 (Lua custom themes) is iteration 6's. Hazard 6 (fonts) is iteration 10's.

**Hazard 1 constrains this iteration's data model.** Templates call Python methods on
renderer-injected attributes — `entry.main_column.splitlines()[:first_row_lines]` — and the port's
answer is pre-split `…Lines []string` fields on the *renderer's* view of an entry, not the
schema's. Two consequences, both in `spec.md` §3.17:

1. No concrete type declares `main_column`, `date_and_location_column`, `degree_column`, or any
   other injected name. A declared field lands in `Descriptor().Fields`, therefore in the JSON
   Schema, therefore in a schema diff failure in iteration 5.
2. The types must still carry a writable by-name surface for them. `bases.BaseEntry` already holds
   `extras []yamldoc.Item` with `Extra(name)` and `ExtraKeys()` readers
   (`bases/entry.go:53-75`). Iteration 3 adds **nothing** here: the write side and the
   template-facing view are iteration 8's, and designing them now would guess at the templater's
   needs. What iteration 3 owns is the negative constraint — the acceptance criterion asserting the
   three names are not fields.

Iteration-local hazards, in descending severity:

1. **The `doi` word boundary** (§5). Cheap to get wrong (`regexp.MustCompile` looks right and
   passes fourteen of sixteen rows), invisible without the two Unicode rows. Mitigated by making
   the table test the gate on the function.
2. **Field order.** `PublicationEntry`'s `date`-last order and the base-reversal rule
   (`spec.md` §3.2) are counter-intuitive and are not exercised by anything until iteration 5's
   schema diff. Mitigated by a positional order test per type, landing before the type.
3. **The package extraction** touches six files it is not otherwise changing. Mitigated by making
   it two pure `refactor:` commits that precede all new code, so a bisect separates the move from
   the feature.
4. **`ValueString` on non-string scalars** (§4). If the reader classifies a scalar as `KindInt`
   and pydantic would have accepted it as a string, every numeric `location:` or `summary:` in a
   real CV starts failing. Verified in the other direction (pydantic rejects `summary: 5`), but
   the interaction with iteration 2's `resolve.go` classification is new. Mitigated by a
   four-scalar-kind test and by edge case 17's requirement that every conftest fixture validate
   clean.
5. **The stub's blast radius.** Iteration 2's accept-everything validator means no existing test
   exercises entry errors. Turning it on can only *add* errors, so it can break iteration 2's
   green suite in ways that look like regressions. Mitigated by ordering: the real dispatcher is
   wired (T13) after all nine types exist and pass their own tests.

---

## 8. Tradeoffs considered and rejected

- **A single generic `Entry` struct with a `map[string]*yamldoc.Node`.** Would collapse eight files
  into one and make the dispatcher trivial. Rejected: field *order* is a parity surface
  (`spec.md` §6.1) and a map cannot hold it; and iteration 8 needs per-type accessors that a
  generic bag cannot type-check. Upstream also has eight classes, and `AGENTS.md` §9 asks for a
  mentally diffable structure.
- **Reflection over struct tags to derive the field list.** Would remove the duplicated own-field
  slices. Rejected for the reason spec 002 plan §9 already gave: error text, order and count are
  contractual, and reflection makes them emergent.
- **Registering descriptors via `init()` in each type's file.** Would let `Default()` be a
  one-liner. Rejected: `init()` order across files within a package is source-name order, which
  is alphabetical, which is *not* the union order (`spec.md` §3.1 behavior 3) — the exact bug the
  spec warns about. `Default()` lists the eight literally.
- **A `TextEntry` struct wrapping a string.** Would make the dispatcher uniform. Rejected:
  upstream has no such class, and inventing one gives iteration 5 a ninth model to accidentally
  emit.
- **Typing `location`/`summary`/`highlights` in iteration 4 with the rest of the error work.**
  Tempting, since the strings are pydantic's. Rejected: without them the mixed-section criteria of
  `spec.md` §5.13 cannot be re-asserted against real types, which is the carried item this
  iteration exists to close.
- **Caching `doi_url`.** Upstream does (`functools.cached_property`). Rejected as noise: two
  concatenated strings, and a cached member is state iteration 5 must be taught to skip.
- **Doing the package extraction as one commit with the wiring.** Rejected: `AGENTS.md` §7 forbids
  refactor+feature bundles, and here the refactor is exactly the risky part.

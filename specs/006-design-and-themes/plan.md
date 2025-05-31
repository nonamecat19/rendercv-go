# Iteration 6 — plan

Go design for `spec.md`. Behavior lives there; this file is packages, types and tradeoffs.

---

## 1. What makes this iteration different from 5 and 7

Iterations 5 and 7 could state their models as Go literals because they were small — eighteen
`$defs` and ten fields respectively. This one is 161 `$defs` over twenty-two nested models and
eight override files, and the volume changes the answer to every design question below.

Three facts drive the whole plan:

1. **There is one tree, not nine.** `ClassicTheme` is the only hand-declared model; the eight
   others are `create_variant_pydantic_model` applied to override dicts (`spec.md` §1 behavior 1).
   The port must mirror that or it multiplies its transcription risk by nine.
2. **The `$defs` are a function of the tree and the overrides**, nothing more. Every one of the 161
   is derivable, so none should be written by hand.
3. **The collision numbering is the only part that is not obvious**, and §4 measures it.

---

## 2. Where the data lives, and why it is generated

Two bodies of data, both from the submodule, both absent at runtime:

| Data | Size | Source |
|---|---|---|
| `ClassicTheme`'s field tree — names, defaults, descriptions | 22 models, ~110 fields, descriptions up to 900 bytes | `classic_theme.py` (857 lines) |
| The eight overrides | 513 lines of YAML | `other_themes/*.yaml` |

Both are **generated into Go source by `tools/designprobe`**, following `tools/localeprobe`
(spec 007 plan §1 and its tool head comment). The argument is the same and stronger here: the
descriptions are multi-paragraph strings listing template placeholders, and a transcription typo
inside one is invisible to review and fatal to the `$defs` differential — which would report it as
one of 161 byte failures.

**What that costs is the same too, and is written at the tool's head**: the submodule-diff test
compares generated data against the files it came from, so it cannot fail at generation time. Its
value is drift detection after a submodule bump. Unlike locale, the field tree half is not read
from YAML but introspected out of pydantic, so the tool needs `uv` — it joins `gengolden` as a
tool that only runs where the submodule is set up, and `just designprobe` gates on that.

**One thing is *not* generated**: the six `Literal` unions of `spec.md` §2 behavior 5 and the
seventeen font families. They are short, order-carrying, and named in acceptance criteria, so they
are written as Go source and diffed — the same split iteration 4 made between the error dictionary
(transcribed, thirteen rows) and the locale catalogs (generated, 210 strings).

---

## 3. Packages

```
internal/schema/models/design/
  design.go        (exists) ThemeName, ValidateTheme — iteration 4's slice for spec 004 §4.27
  tree.go          the option-tree shape: Model, Field, Kind
  tree_generated.go        ClassicTheme's twenty-two models, from designprobe
  overrides.go     the eight override maps, from designprobe
  literals.go      the six Literal unions and the seventeen font families
  typstdimension.go  §3.1 behavior 9 — the pattern and §4A.1's message
  color.go           §3.1 behavior 10 — the library's failure, coded color_error
  fontfamily.go      §3.2 behavior 14's string-widening coercion
  validate.go      the recursive binder over the tree
  variants.go      the override walk that produces a theme's effective tree
  schema.go        the 161 `$defs`
```

`design` already exists and already owns `ThemeName`; `Themes` is its `Languages` — one list, two
consumers, and `available_themes`' order is asserted against the glob exactly as
`TestLanguagesAreInUnionOrder` does for locale.

---

## 4. The collision numbering

`jsonschema.SuffixedNames` handles a flat emission order and is what iteration 7 built on
locale's twenty-two. Design's is not flat, and the difference is the one thing in this iteration
that a plausible implementation gets wrong.

**Measured rule.** For each nested field, walk the nine themes in union order. A theme whose
override dict contains that key gets a **new** number; a theme that does not **reuses the base's**,
which is always `__1`. So the count per model is not nine:

| Model | Suffixes | Which of the eight override files name the key |
|---|---:|---|
| `Page` | 6 | five: ember, engineeringresumes, harvard, ink, opal |
| `Colors` | 7 | six: those five minus harvard's absence — measured, all but engineeringclassic and moderncv |
| `Links` | 8 | seven: all but harvard |
| `Typography`, `Header`, `SectionTitles`, `Sections`, `Entries`, `Templates` | 9 | all eight |
| `SmallCaps` | 4, `Bold` 7, `FontSize` 7, `Connections` 7, `Summary` 7, `Highlights` 8 | nested one level deeper, so counted over the themes that override **both** the parent and the child |
| `OneLineEntry`, `PublicationEntry` | **none** | no theme overrides them, so the bare name is unique and carries **no suffix at all** |

Every count above is `1 + (number of override files naming the key)`, and the `+1` is the base
class — which is also why a theme that omits a key points back at `__1`.

The last row is the trap: an implementation that suffixes unconditionally emits
`…OneLineEntry__1` where upstream emits `…OneLineEntry`. The rule is pydantic's general one —
qualify when the bare name collides, number when the qualified name still collides — so the port
must count first and suffix only when the count exceeds one.

`SuffixedNames` therefore grows a sibling rather than a parameter: `variants.go` produces the
`(model, theme) → ordinal` assignment by walking the tree depth-first inside the theme loop, and
`schema.go` reads it. Deriving it is not optional — a table of the counts above would be a
restatement that drifts from the overrides it is supposed to describe.

**Verification is the same as locale's**: the numbering is invisible in the output because `$defs`
sorts its keys, so only the `$ref` from each theme's field pins it. The differential covers both
because it compares whole objects.

---

## 5. Validation

The tree is data, so the binder is one recursive function over it rather than twenty-two
hand-written validators:

```go
func validateModel(node *yamldoc.Node, model Model, location []string, …) []schemaerr.ValidationError
```

Each `Field` carries a `Kind` — `KindNested`, `KindString`, `KindBool`, `KindTypstDimension`,
`KindColor`, `KindFontFamily`, `KindLiteral`, `KindStringList` — and the leaf kinds dispatch to the
three value types of `spec.md` §3.1. Unknown keys are `binder.ForbidExtra` at every level, which is
behavior 4.

**The two coercions of §3.2 are not validation and do not live here.** `validate_font_family`
widens a string into the full object and `convert_section_titles_to_snake_case` lowercases a list;
both are observable only in rendered output, so they produce values on the model and the tests that
prove them are iteration 8 and 9's as much as this one's. They are implemented here because the
model is here.

**Which theme's tree is validated matters.** `validate_design` picks the union member from `theme`
before any option is read (`spec.md` §3 behavior 6), exactly as locale picks its catalog — and
iteration 7's verifier finding applies directly: the `design` block is wired into
`rendercvmodel.go` today with `ValidateTheme` alone, so nothing reaches the option tree. Closing
that edge is a task here, not an afterthought.

---

## 6. Hazards

1. **`OneLineEntry` and `PublicationEntry` carry no suffix.** §4's last row.
2. **A theme that does not override a block reuses `__1`**, so nine themes do not mean nine
   classes. An implementation that numbers per theme produces the right *count* for the six blocks
   every theme overrides and the wrong one for the other nine models.
3. **`Bullet`'s members are non-ASCII** and reach the schema literally (`spec.md` §2 behavior 5).
   The encoder already handles it; the risk is the generated Go source, which the differential
   catches.
4. **`FontFamily` exists twice** — `design__classic_theme__FontFamily` and
   `design__font_family__FontFamily` are two different models with the same bare name, qualified
   rather than numbered. A port with one `FontFamily` emits one `$defs` entry and misses the other.
5. **The font list is `sorted()`, not source order** (`spec.md` §3.1 behavior 13).
6. **Descriptions resplice per variant**, the same rule locale measured
   (`update_description_with_new_default`), and here they nest: an override three levels deep
   rewrites the description at that level only.

---

## 7. What lands where

Custom themes (D-002's Lua path) and `spec.md` §3 behaviors 7's second and third messages are
**not** in this iteration's tasks. The spec places them here; the plan moves them out, because the
Lua sandbox is a subsystem of its own and bundling it with 161 `$defs` makes both unreviewable.
`tasks.md` §Wave E records the split and `STATE.md` carries it as cut scope with the reason — it is
a scheduling decision, not a divergence, and `divergences.md` is untouched.

# Iteration 6 — design and themes

Behavior of the `design` block, extracted from the vendored Python. No Go design here; that is
`plan.md`.

Upstream: `third_party/rendercv` @ `v2.8` (`2eba248`).
Primary sources: `src/rendercv/schema/models/design/` — `classic_theme.py` (857 lines),
`built_in_design.py` (50), `design.py` (135), `other_themes/*.yaml` (513 across eight files),
`color.py`, `font_family.py`, `typst_dimension.py`.

---

## 1. The shape of the subsystem, and what it decides

**There is one theme model and eight sets of overrides.** `ClassicTheme` is 857 lines of Python
declaring a tree of twenty-two nested option models; the other eight built-in themes are **YAML
files** that a generator turns into variant classes at import time
(`built_in_design.py:13-38`).

1. `discover_other_themes()` globs `other_themes/*.yaml` in **sorted filename order**, and for each
   calls `create_variant_pydantic_model(variant_name=<stem>, defaults=read_yaml(file)["design"],
   base_class=ClassicTheme, discriminator_field="theme", …)`. So a variant is `ClassicTheme` with
   some field defaults replaced and its `theme` literal pinned to the file's stem.
2. `BuiltInDesign` is the discriminated union of `ClassicTheme` and those eight, on `theme`
   (`:41-43`). `available_themes` is the nine names in that order: `classic` first, then the eight
   sorted stems (measured).
3. **This is why the JSON Schema has 161 design `$defs`** (spec 005 §1): each variant re-emits
   every nested model it overrides, which is what produces
   `…classic_theme__Colors__1` through `__7` and the rest of spec 005 §3.3 behavior 12's
   collision suffixes.

### 1.1 The consequence for the port

The Go structure follows upstream's, because `AGENTS.md` §9 asks it to and because here the two
happen to agree on what is least work:

- **one** hand-written option tree, mirroring `classic_theme.py`;
- the eight variants as **override data**, not eight more trees.

The alternative — hand-writing nine trees — was not considered seriously: it is nine times the
transcription risk for a structure upstream itself refuses to duplicate.

The override files live in the submodule, which is not present at runtime, so their content must
be copied into the Go tree. That is a transcription risk with an established answer in this
repository: compiled-in data plus a **submodule-diff test**, exactly as iteration 4 did for
`error_dictionary.yaml` (spec 004 §3.4). `plan.md` §2 sizes it.

---

## 2. The option tree

4. `ClassicTheme` declares twenty-two nested models, all `BaseModelWithoutExtraKeys` — so every
   one rejects unknown keys, and the error is spec 004 §4.10's. In declaration order:

   ```
   Page          Colors        FontFamily    FontSize      SmallCaps
   Bold          Typography    Links         Connections   Header
   SectionTitles Sections      Summary       Highlights    Entries
   OneLineEntry  EducationEntry NormalEntry  ExperienceEntry
   PublicationEntry            Templates     ClassicTheme
   ```

   The last five before `Templates` are the **per-entry-type template blocks**, which share names
   with the CV entry models of spec 003 §3.1 and are unrelated to them. That name collision is
   what forces the qualified `$defs` names of spec 005 §3.3 behavior 11 on *both* sets.

5. Six scalar types are `Literal` unions and carry their members' declaration order into the
   schema's `enum` (spec 005 §6 rule 6):

   | Type | Members | Source |
   |---|---|---|
   | `Bullet` | `●`, `•`, `◦`, `-`, `◆`, `★`, `■`, `—`, `○` | `classic_theme.py:10` |
   | `BodyAlignment` | `left`, `justified`, `justified-with-no-hyphenation` | `:11` |
   | `Alignment` | `left`, `center`, `right` | `:12` |
   | `SectionTitleType` | **eight**, `:13-22` — the earlier "six" was a miscount, corrected here | `:13` |
   | `PhoneNumberFormatType` | `national`, `international`, `E164` | `:23` |
   | `PageSize` | `a4`, `a5`, `us-letter`, `us-executive` | `:24` |

   `Bullet`'s members are non-ASCII and appear literally in the schema (spec 005 §3.4).

   A seventh union is **not** in the table because it is not a named alias: `header.photo_position`
   is an inline `Literal["left", "right"]`, so it has no `$defs` entry and is emitted inline. Two
   of the six are named and reached once each, and pydantic inlines those too — `BodyAlignment` has
   no `$defs` entry either, while `Alignment`, reached twice, does. The rule is usage count, not
   whether the alias has a name.

---

## 3. Validation

6. `validate_design` (`design.py:24-...`) tries the built-in union first and falls through to the
   custom-theme path **only** when the failure carries a `'theme'` discriminator context
   (`:38-52`). Any other failure is re-raised unchanged, so a built-in theme with a bad option
   reports that option and not "unknown theme".
7. The three custom-theme shape messages of spec 004 §3.17 behavior 64 — §4.27's name check,
   §4.28's missing folder, §4.29's folder with no `*.j2.typ` — run in that order, before any theme
   code is loaded. **§4.27 is already ported** (iteration 4, `models/design`); §4.28 and §4.29
   are this iteration's and are only reachable once custom themes can be loaded, which is D-002's
   Lua path.
8. `TypstDimension`, `Color` and `FontFamily` are the three value types with their own
   validation, and they fail in three different ways — §3.1 has the measurements.

### 3.1 The three value types

9. **`TypstDimension`** (`typst_dimension.py:9-30`) is a full match of
   `-?\d+(?:\.\d+)?(cm|in|pt|mm|em)`, raising `PydanticCustomError(other)` with §4.1's text.
   Measured: `1` and `1px` both fail, `1cm` passes. A negative value is legal, and the pattern
   admits no space between number and unit.
10. **`Color`** (`color.py:1-15`) subclasses `pydantic_extra_types.color.Color` and adds **no
    validation** — only a `__str__` that returns `as_rgb()`, because the Typst templates need
    `rgb(r, g, b)`. So the failure is the library's, coded `color_error`, and its message is
    §4.2's. Measured for `notacolor` and `#gggggg`, which give the same text.
11. **`Color`'s failure is already reachable from the port's pipeline.** Its message is dictionary
    row 13's key, so spec 004 §3.4 behavior 14's substitution and the `)".` ending of §4.11 are
    the *final* text a user sees. That row is live today with nothing to fire it; this iteration
    supplies the producer.
12. **`FontFamily`** (`font_family.py:5-30`) is `SkipJsonSchema[str] | Literal[*available_font_families]`
    — a union of a free string with seventeen names, where the string arm is hidden from the
    schema. So **any** font name validates and the seventeen only drive editor completion. There
    is no failure message.
13. `available_font_families` is **`sorted()`** over a source list written in two groups (three
    Typst built-ins, fourteen RenderCV-bundled). The port must carry the sorted order, not the
    source order, because that is what reaches the schema.

### 3.2 `classic_theme.py` has two field validators, and neither raises

*(This section replaces an earlier §4.3 that asserted the file "raises nothing of its own" and
deferred the check. The assertion was wrong — there are two validators — and the check is what
found it. Recorded rather than edited away, because the failure mode is the one this spec was
most at risk of: a plausible claim about 857 lines, cheap to write and expensive to be wrong
about.)*

14. **`validate_font_family`** (`classic_theme.py:280-300`, `mode="plain"`) accepts either a
    string or a `FontFamily` mapping and **expands a string into the full object**, giving every
    element the same font. It raises nothing: a string is widened, a mapping passes through. This
    is a coercion the port must reproduce, not an error to match — `font_family: Roboto` and
    `font_family: {body: Roboto, name: Roboto, …}` must produce the same model.
15. **`convert_section_titles_to_snake_case`** (`:493-500`, `mode="after"`) lowercases each entry
    of `sections.show_time_spans_in` and replaces spaces with underscores. Also non-raising, also
    a coercion: `["Work Experience"]` becomes `["work_experience"]`, which is what the renderer
    matches section titles against.
16. So the original claim survives in the form that matters — **the file adds no error strings** —
    but for the wrong reason. It has validators; they transform rather than reject. Both
    transformations are observable in rendered output, which makes them iteration 8 and 9's
    problem as much as this one's.

---

## 4. Out of scope

**4.1 The templates are strings here and rendering is iteration 8's.** `Templates` and the five
per-entry blocks hold Jinja-ish strings like
`"**INSTITUTION**\n*DEGREE* *in* *AREA*\nSUMMARY\nHIGHLIGHTS"`. This iteration models and validates
them as **data**; interpreting them is the templater's.

**4.2 Custom theme loading is D-002's Lua path**, and §4.28/§4.29 land with it.

**4.3 Superseded by §3.2**, which is the corrected version of what this section claimed.

---

## 4A. Exact strings

### 4.1 Bad Typst dimension — `typst_dimension.py:26-27`

```
The value must be a number followed by a unit (cm, in, pt, mm, em). For example, 0.1cm.
```

### 4.2 Bad colour — `pydantic_extra_types.color`

The raw message. Dictionary row 13's key is a substring of it, so the final text is spec 004
§4.11's and ends `)".`

```
value is not a valid color: string not recognised as a valid color
```

---

## 5. Acceptance criteria

- [ ] `ClassicTheme`'s twenty-two nested models, every field, every default, every constraint.
- [ ] §4A's two messages, verbatim, and `Color`'s reaching §4.11 through dictionary row 13 — the
      first live producer for a row that has been in the table since iteration 4.
- [ ] Any font name accepted, with the seventeen appearing in the schema's `enum` in **sorted**
      order (§3.1 behavior 13).
- [ ] §3.2's two coercions: a string `font_family` expands to every element, and
      `show_time_spans_in` is lowercased and underscored.
- [ ] The six `Literal` unions with their members in declaration order.
- [ ] The eight override files reproduced as data, with a **submodule-diff test** proving each
      matches `other_themes/<stem>.yaml` key for key.
- [ ] `available_themes` is the nine names in upstream's order: `classic` then the eight sorted
      stems.
- [ ] Unknown keys rejected at every level, with spec 004 §4.10's message.
- [ ] A built-in theme with a bad option reports that option, not "unknown theme" (§3 behavior 6).
- [ ] **All 161 design `$defs` byte-identical**, including the `__1`…`__9` collision suffixes —
      which requires implementing the emission-order numbering spec 005 §7.2 deferred here.
- [ ] `just schema-diff` shrinks from 8,621 differing lines to the locale and settings remainder.

---

## 6. Status

**Complete for behavior; `plan.md` and `tasks.md` still to write.** The owed pass is done and is
§3.2 — it disproved the claim it was meant to confirm, which is the argument for having run it.

The iteration adds two error strings (§4A) and two coercions (§3.2). What remains is the volume:
twenty-two nested models and eight override files, all measurable, none surprising.

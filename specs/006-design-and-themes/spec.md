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
   | `SectionTitleType` | (six, `:13-22`) | `:13` |
   | `PhoneNumberFormatType` | `national`, `international`, `E164` | `:23` |
   | `PageSize` | `a4`, `a5`, `us-letter`, `us-executive` | `:24` |

   `Bullet`'s members are non-ASCII and appear literally in the schema (spec 005 §3.4).

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
8. `TypstDimension`, `Color` and `FontFamily` are the three value types with their own validation.
   Their messages are iteration 6's and are **not yet extracted**; §7 scopes that.

---

## 4. Out of scope

**4.1 The templates are strings here and rendering is iteration 8's.** `Templates` and the five
per-entry blocks hold Jinja-ish strings like
`"**INSTITUTION**\n*DEGREE* *in* *AREA*\nSUMMARY\nHIGHLIGHTS"`. This iteration models and validates
them as **data**; interpreting them is the templater's.

**4.2 Custom theme loading is D-002's Lua path**, and §4.28/§4.29 land with it.

**4.3 The individual option messages are not yet extracted.** §3 behavior 8 names three value
types whose failure text this spec does not yet carry. That is the honest state: this spec was
written to establish structure and scope, and a second pass must extract every message before
implementation, the way spec 004 §4 enumerates thirty-four. **`tasks.md` must not be written until
then** — a porter given this file alone would invent error text.

---

## 5. Acceptance criteria

- [ ] `ClassicTheme`'s twenty-two nested models, every field, every default, every constraint.
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

**Incomplete — structure and scope only.** §4.3 says what is missing and why `tasks.md` does not
exist yet. The next step on this iteration is a message-extraction pass over `classic_theme.py`,
`color.py`, `font_family.py` and `typst_dimension.py`, not implementation.

# Iteration 9 — the Typst renderer

Behavior of the step between a validated model and a `.typ` string, extracted from the vendored
Python. No Go design here.

Upstream: `third_party/rendercv` @ `v2.8` (`2eba248`).
Primary sources: `src/rendercv/schema/models/cv/{cv.py,section.py}`,
`src/rendercv/renderer/templater/templater.py`, and the design and locale models iterations 6
and 7 ported.

---

## 0. What this iteration is, and what it inherits

**It is the bridge, not the engine.** Iteration 8 ported everything a `.typ` is made *of* — the
processors, the pongo2 environment, the transformed templates, the assembly — and nothing that
makes one. This iteration is what turns a `RenderCVModel` into the `process.Model` those consume,
plus the top-level orchestration that calls them in order.

It carries three inherited items, each recorded in `STATE.md` before this file existed:

| Inherited | From | Why it landed here |
|---|---|---|
| **T10 — the effective per-theme option tree** | iteration 6 | nothing validates a default, so the renderer is the first consumer |
| **Wave C — the corpus's artifact `.typ` cases** | iteration 8 | they are whole `render` runs and need this bridge |
| `process_date`, `render_entry_templates` | iteration 8 | **already closed** at the head of this iteration; they were under-scoped, not blocked |

---

## 1. Sections

1. **`cv.sections` is a mapping and the templates need a list**, so `get_rendercv_sections`
   (`section.py:320-355`) converts it, in the mapping's own order — which the YAML reader preserves
   and which is therefore the input file's order.
2. Each section gets three things: a `title`, an `entry_type` and its `entries`.
3. **The title is the key put through `dictionary_key_to_proper_section_title`**, which spec 002
   §3.62-§3.64 already ported as `TitleFromKey` — the stop-word rule and Python's
   `str.capitalize()`.
4. **`snake_case_title` is the *formatted* title lowercased with spaces underscored**
   (`section.py:85-87`), not the original key. So a key of `work_experience` becomes the title
   `Work Experience` and then the snake-case title `work_experience` — a round trip that is only
   the identity when the key had no stop words. `Skills and Tools` → `Skills and Tools` →
   `skills_and_tools`.
5. **An empty section's entry type is `TextEntry`** (`:342-344`), chosen before any entry is
   examined because there is none to examine.
6. Otherwise the type comes from the **first** entry (`:345-349`), which is safe because
   `validate_section` already forced every entry in the section to that type.
7. `TextEntry` is the type of a bare string (`:162-165`); anything else is its model's class name.

---

## 2. The connection list

8. **The order is `cv._key_order`** (`cv.py:126`, `:161-173`): the keys as the user wrote them,
   filtered to those whose value is not null. Spec 008 §4E behavior 45 measured the consequence;
   this is where the list comes from.
9. It is captured in a `model_validator(mode="wrap")` **before** pydantic validates, so it is the
   raw input's order rather than the model's field order.
10. A key present with a null value is **dropped** from the order, so it contributes no connection.

---

## 3. The effective option tree — iteration 6's T10

11. A theme's options are `ClassicTheme`'s defaults with the theme's override mapping deep-merged
    over them (spec 006 §1 behavior 1). Validation never needed this because nothing validates a
    default; the renderer needs every value, defaulted or not.
12. The merge is **deep**: `create_nested_model_variant_model`
    (`variant_pydantic_model_generator.py:280-315`) recurses into a nested mapping and replaces only
    the keys the override supplies, so a theme that sets `typography.font_size.body` keeps the
    other four sizes.
13. **A document's own `design` block merges over the theme's**, by the same rule — that is what
    makes `{theme: sb2nov, page: {top_margin: 1cm}}` mean "sb2nov, but".

---

## 4. Orchestration

14. `render_full_template` (`templater.py:50-127`) is the order iteration 8's `Assemble` already
    pins: `download_photo_from_url`, then `process_model`, then the header, then the preamble for
    Typst, then each section's beginning, entries and ending.
15. **`download_photo_from_url` reaches the network** (`model_processor.py:23-58`) when
    `cv.photo` is a URL, writing the file beside the input. It is the only network access in the
    whole pipeline.

---

## 5. Out of scope

**5.1 Compiling the `.typ` is iteration 10's.** This iteration ends at a string.

**5.2 The Markdown and HTML documents are iteration 11's**, though `Assemble` and the processors
already handle both formats — what is missing is only the caller.

**5.3 The CLI is iteration 12's.** The corpus cases are run through a test harness here, not
through `rendercv-go render`.

---

## 6. Acceptance criteria

- [ ] §1's section conversion, including the empty section's `TextEntry` and the title round trip
      that is not the identity.
- [ ] §2's connection order, driven by a document whose keys are deliberately not in field order.
- [ ] §3's deep merge, proven by a theme that overrides one nested key and keeps its siblings.
- [ ] The first corpus case's `.typ` **byte-identical**, which is Axis 1's first passing case.
- [ ] Each remaining entry type, one case at a time.

---

## 7. Status

**Behavior complete for the bridge.** §4 behavior 15's network access needs a decision before it is
ported — the corpus has no photo case, and a renderer that reaches the network in a test is worse
than one that does not.

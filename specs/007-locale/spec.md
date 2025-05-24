# Iteration 7 — locale catalogs

Behavior of the `locale` block, extracted from the vendored Python. No Go design here; that is
`plan.md`.

Upstream: `third_party/rendercv` @ `v2.8` (`2eba248`).
Primary sources: `src/rendercv/schema/models/locale/` — `english_locale.py` (146 lines),
`locale.py` (51), `other_locales/*.yaml` (21 files).

---

## 1. The shape

**One base model and twenty-one YAML overrides** — the same mechanism as the design themes
(spec 006 §1), with one difference that matters.

1. `discover_other_locales()` globs `other_locales/*.yaml` in **sorted filename order** and builds
   a variant of `EnglishLocale` per file, discriminated on `language`
   (`locale.py:13-38`).
2. `Locale` is the discriminated union of `EnglishLocale` and those twenty-one; `available_locales`
   is the twenty-two names in that order — `english` first, then the sorted stems, which is spec
   004 §4.30's enumeration and is **already ported** (`models/locale`, iteration 4).
3. **The difference from design: `require_all_fields=True`** (`locale.py:35`). A theme variant may
   override a subset of `ClassicTheme`'s fields; a locale variant must supply **every** field.
   That is checkable, and §5's first criterion checks it.

---

## 2. The model

4. `EnglishLocale` has ten fields, in declaration order:

   ```
   language  last_updated  month  months  year  years  present
   phrases  month_abbreviations  month_names
   ```

5. Seven are plain strings with defaults, measured:

   | Field | English default |
   |---|---|
   | `last_updated` | `Last updated in` |
   | `month` | `month` |
   | `months` | `months` |
   | `year` | `year` |
   | `years` | `years` |
   | `present` | `present` |

   `language` is the `Literal` discriminator, defaulting to the variant's own name.

6. `phrases` is a one-field nested model, `Phrases.degree_with_area`, defaulting to
   `DEGREE in AREA`. It is a **template**: the placeholders `DEGREE` and `AREA` are substituted by
   the renderer, which is iteration 9's.

7. `month_abbreviations` and `month_names` are `list[str]` **constrained to exactly twelve**
   (`at.Len(min_length=12, max_length=12)`). Measured English values:

   ```
   Jan  Feb  Mar  Apr  May  June  July  Aug  Sept  Oct  Nov  Dec
   January February March April May June July August September October November December
   ```

   **`June`, `July` and `Sept` are not three letters.** The abbreviation list comes from Yale's
   cataloguing table (`english_locale.py:59`, cited in a comment) rather than from truncation, and
   a port that generated abbreviations by slicing would get three of twelve wrong.

---

## 3. Validation

8. `EnglishLocale` is a `BaseModelWithoutExtraKeys`, so an unknown key is rejected with spec 004
   §4.10's message.
9. **The locale package raises no custom failures.** It has neither a field nor a model validator,
   which spec 004 §3.17 behavior 67 already measured — every locale failure is a plain pydantic
   message with the discriminator element dropped by the pipeline's step 2. So this iteration adds
   **no new error strings**, and that is a finding rather than an omission: the only locale message
   in the whole contract is §4.30's, already ported.
10. A twelve-element list of the wrong length fails with pydantic's own length message, and there
    are **two** of them rather than one — the bound that was violated decides which. Measured:

    | Input | Code | Message |
    |---|---|---|
    | 11 items | `too_short` | `List should have at least 12 items after validation, not 11` |
    | 13 items | `too_long` | `List should have at most 12 items after validation, not 13` |
    | 0 items | `too_short` | `List should have at least 12 items after validation, not 0` |

    Both interpolate the actual count, and neither matches a dictionary row, so the pipeline only
    appends a period.

---

## 4. Out of scope

**4.1 Date formatting is iteration 9's.** The catalogs are data here; the code that turns
`2020-09` into `Sept 2020` belongs with the renderer.

**4.2 `phrases.degree_with_area`'s substitution is iteration 9's** for the same reason.

---

## 5. Acceptance criteria

- [ ] All twenty-one override files reproduced as data, each with **every** field, and a
      **submodule-diff test** proving it — `require_all_fields=True` makes a missing field a real
      difference rather than an inherited default (§1 behavior 3).
- [ ] `available_locales` is the twenty-two names in upstream's order, matching the list iteration
      4 already ships for spec 004 §4.30. One list, two consumers.
- [ ] The English defaults of §2 behaviors 5 and 7 verbatim, `June`/`July`/`Sept` included.
- [ ] Both twelve-element lists rejected at eleven and thirteen, with §3 behavior 10's two
      distinct messages and their interpolated counts.
- [ ] All 45 locale `$defs` byte-identical, taking `just schema-diff` to the design remainder.
- [ ] Non-ASCII survives: `måned`, `nuværende`, `norwegian_bokmål` (spec 005 §3.4).

---

## 6. Status

**Complete.** Every behavior here is measured against the vendored Python. The iteration adds two
error strings, both pydantic's length messages of §3 behavior 10, and nothing else — the locale
package raises none of its own.

`tasks.md` can be written from this file.

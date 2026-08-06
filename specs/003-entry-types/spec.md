# Spec 003 — Entry types

**Status:** draft · **Inherits:** [`specs/000-parity-contract/spec.md`](../000-parity-contract/spec.md)

**Upstream covered:**

- `schema/models/cv/entries/bullet.py`
- `schema/models/cv/entries/numbered.py`
- `schema/models/cv/entries/reversed_numbered.py`
- `schema/models/cv/entries/one_line.py`
- `schema/models/cv/entries/normal.py`
- `schema/models/cv/entries/experience.py`
- `schema/models/cv/entries/education.py`
- `schema/models/cv/entries/publication.py`
- `schema/models/cv/entries/__init__.py`, `schema/models/cv/entries/bases/__init__.py` (both 0 bytes)
- `schema/models/cv/section.py:11-42`, `:45-77`, `:90-126`, `:128-178` (the parts that read the
  concrete classes; the rest is spec 002's)
- `schema/models/custom_error_types.py`

Citations are relative to `third_party/rendercv/src/rendercv/`, except those beginning with
`tests/`, which are relative to `third_party/rendercv/`.

---

## 1. Purpose

Give the nine entry types their real shapes, so that section discrimination stops running against
a test fixture and starts running against the types users actually write. This closes the
registry inversion spec 002 §7.1 opened, and it makes entry-level validation errors — missing
required fields, wrong scalar types, bad DOIs — real for the first time. It is the last piece of
the `cv` block before iteration 4 can render errors.

## 2. Inputs / Outputs

### Inputs

One entry, as it appears inside a section list in the parsed document: either a mapping, or a
bare string (`section.py:34`). The mapping's keys are unvalidated user text; unknown keys are
legal and retained (spec 002 §3.67). Entry validation additionally needs a reference date, for
the `present` handling the complex-field base inherits (spec 002 §3.73 case 5).

### Outputs

1. **A typed entry** — one of nine kinds, carrying its declared fields plus every unknown key the
   user wrote.
2. **The entry's type name**, which is the class name verbatim: `BulletEntry`, `EducationEntry`,
   … , `TextEntry` (`section.py:37-39`, `:152`, `:164`, `:175`).
3. **A list of validation-error records** (spec 002 §2) for the entry's own field failures. These
   travel as the children of the section-level §4.12 record of spec 002.

Example input, the education fixture (`tests/schema/models/cv/conftest.py:6-17`):

```yaml
- institution: Boğaziçi University
  location: Istanbul, Turkey
  degree: BS
  area: Mechanical Engineering
  start_date: 2015-09
  end_date: 2020-06
  highlights:
    - "GPA: 3.24/4.00 ([Transcript](https://example.com))"
    - "Awards: Dean's Honor List, Sportsperson of the Year"
```

It resolves to `EducationEntry`, its section model is named `SectionWithEducationEntries`, and it
produces no errors.

Example input, a bare string (`tests/schema/models/cv/conftest.py:122-128`):

```yaml
- This is a *TextEntry*. It is only a text and can be useful for sections like **Summary**.
```

It resolves to `TextEntry` with no model and no fields.

---

## 3. Behavior

### 3.1 The type set and its two orders

1. There are **nine** entry type names and **eight** entry models. `TextEntry` is a bare string,
   not a model, and is therefore absent from the model union and appended to the name list
   afterwards (`section.py:23-24`, `:34`, `:37-39`).
2. The eight models' **discrimination order** is the union's declaration order
   (`section.py:24-33`):

   ```
   OneLineEntry, NormalEntry, ExperienceEntry, EducationEntry,
   PublicationEntry, BulletEntry, NumberedEntry, ReversedNumberedEntry
   ```

   `TextEntry` is appended ninth for enumeration only; it never takes part in characteristic-field
   discrimination (`section.py:37-39`, and the `isinstance(entry, str)` branch at `:162-165` which
   precedes no field lookup).
3. The **import order** (`section.py:11-18`) is alphabetical by filename and is *not* the
   discrimination order. Nothing may be derived from it. Behavior 2's order is the only one that
   is observable, because it decides which type an ambiguous entry resolves to (spec 002 §3.57,
   §6.4).
4. Neither `entries/__init__.py` nor `entries/bases/__init__.py` re-exports anything — both are
   zero bytes. Consumers import leaf modules directly (`section.py:11-18`). There is no upstream
   aggregate surface to mirror.

### 3.2 Field order within a type

5. Every entry model's field order is a parity surface: it drives JSON Schema property order
   (iteration 5) and sample-YAML key order (iteration 13). It is fixed by the declaration and
   must not be re-derived.
6. Four types are declared with **two** bases, in the form
   `class X(BaseWithDates, BaseX)` (`education.py:26`, `experience.py:17`, `normal.py:14`,
   `publication.py:100`). Pydantic emits the **last-listed** base's own fields first, then the
   first-listed base's. The comment above each such class says so explicitly — *"This approach
   ensures …Base keys appear first in the key order"* (`education.py:25`, `experience.py:16`,
   `normal.py:13`, `publication.py:99`).
7. Field order therefore does **not** follow the MRO. For `EducationEntry` the MRO is
   `EducationEntry → BaseEntryWithComplexFields → BaseEntryWithDate → BaseEducationEntry →
   BaseEntry`, while the field order puts `institution`, `area`, `degree` first. Verified at
   runtime against the vendored Python; the eight orders are behaviors 8–15 below.

### 3.3 `OneLineEntry` — `one_line.py:6-12`

8. Two fields, both required text, in this order. Base: `BaseEntry` (`one_line.py:6`), so it has
   no date fields at all.

   | # | Field | Shape | Default | Description metadata |
   |---|---|---|---|---|
   | 1 | `label` | text | none (required) | none |
   | 2 | `details` | text | none (required) | none |

   Examples metadata: `label` → `["Languages", "Citizenship", "Security Clearance"]`;
   `details` → `["English (native), Spanish (fluent)", "US Citizen", "Top Secret"]`
   (`one_line.py:8`, `:11`). Runtime field order: `['label', 'details']`.

### 3.4 `BulletEntry` — `bullet.py:6-9`

9. One field, required text, no description. Base: `BaseEntry`.

   | # | Field | Shape | Default | Description metadata |
   |---|---|---|---|---|
   | 1 | `bullet` | text | none (required) | none |

   Examples: `["Python, JavaScript, C++", "Excellent communication skills"]` (`bullet.py:8`).

### 3.5 `NumberedEntry` — `numbered.py:6-9`

10. One field, required text, no description. Base: `BaseEntry`.

    | # | Field | Shape | Default | Description metadata |
    |---|---|---|---|---|
    | 1 | `number` | text | none (required) | none |

    Examples: `["First publication about XYZ", "Patent for ABC technology"]` (`numbered.py:8`).

### 3.6 `ReversedNumberedEntry` — `reversed_numbered.py:6-13`

11. One field, required text, **with** a description. Base: `BaseEntry`.

    | # | Field | Shape | Default | Description metadata |
    |---|---|---|---|---|
    | 1 | `reversed_number` | text | none (required) | §4.7 |

    Examples: `["Latest research paper", "Recent patent application"]`
    (`reversed_numbered.py:12`).

### 3.7 `NormalEntry` — `normal.py:7-15`

12. One own field plus the complex-field base's six. Runtime field order:
    `['name', 'date', 'start_date', 'end_date', 'location', 'summary', 'highlights']`.

    | # | Field | Shape | Default | Owner |
    |---|---|---|---|---|
    | 1 | `name` | text | none (required) | `BaseNormalEntry` (`normal.py:8`) |
    | 2 | `date` | arbitrary date | absent | `BaseEntryWithDate` |
    | 3 | `start_date` | exact date | absent | `BaseEntryWithComplexFields` |
    | 4 | `end_date` | exact date or `present` | absent | `BaseEntryWithComplexFields` |
    | 5 | `location` | text | absent | `BaseEntryWithComplexFields` |
    | 6 | `summary` | text | absent | `BaseEntryWithComplexFields` |
    | 7 | `highlights` | list of text | absent | `BaseEntryWithComplexFields` |

    `name` examples: `["Some Project", "Some Event", "Some Award"]`, no description
    (`normal.py:9`).

### 3.8 `ExperienceEntry` — `experience.py:7-17`

13. Two own fields plus the same six. Runtime field order:
    `['company', 'position', 'date', 'start_date', 'end_date', 'location', 'summary',
    'highlights']`.

    | # | Field | Shape | Default | Description metadata |
    |---|---|---|---|---|
    | 1 | `company` | text | none (required) | none |
    | 2 | `position` | text | none (required) | none |
    | 3–8 | as behavior 12's rows 2–7 | | | |

    Examples: `company` → `["Microsoft", "Google", "Princeton Plasma Physics Laboratory"]`;
    `position` → `["Software Engineer", "Research Assistant", "Project Manager"]`
    (`experience.py:9`, `:12`).

### 3.9 `EducationEntry` — `education.py:7-27`

14. Three own fields plus the same six. Runtime field order:
    `['institution', 'area', 'degree', 'date', 'start_date', 'end_date', 'location', 'summary',
    'highlights']`.

    | # | Field | Shape | Default | Description metadata |
    |---|---|---|---|---|
    | 1 | `institution` | text | none (required) | none |
    | 2 | `area` | text | none (required) | §4.8 |
    | 3 | `degree` | text | absent (optional) | none |
    | 4–9 | as behavior 12's rows 2–7 | | | |

    Examples: `institution` → `["Boğaziçi University", "MIT", "Harvard University"]`;
    `area` → `["Mechanical Engineering", "Computer Science", "Electrical Engineering"]`;
    `degree` → `["BS", "BA", "PhD", "MS"]` (`education.py:9`, `:13-17`, `:21`).

### 3.10 `PublicationEntry` — `publication.py:12-101`

15. Six own fields plus **`date` only**. Runtime field order:
    `['title', 'authors', 'summary', 'doi', 'url', 'journal', 'date']` — `date` is **last**,
    because it comes from the first-listed base (behavior 6) and the base is
    `BaseEntryWithDate`, not `BaseEntryWithComplexFields` (`publication.py:100`).

    | # | Field | Shape | Default | Description metadata |
    |---|---|---|---|---|
    | 1 | `title` | text | none (required) | none |
    | 2 | `authors` | list of text | none (required) | §4.9 |
    | 3 | `summary` | text | absent | none |
    | 4 | `doi` | text matching a pattern (behavior 17) | absent | §4.10 |
    | 5 | `url` | HTTP URL | absent | §4.11 |
    | 6 | `journal` | text | absent | §4.12 |
    | 7 | `date` | arbitrary date | absent | inherited |

16. `PublicationEntry` has **no** `start_date`, `end_date`, `location` or `highlights`
    (`publication.py:100` inherits `BaseEntryWithDate`, whose only field is `date`). Because
    entries allow extra keys, writing `start_date: 2020` on a publication entry is accepted and
    retained as an unknown key, and **no date validation runs on it** — see §5.6.

    Examples: `title` → `["Deep Learning for Computer Vision", "Advances in Quantum Computing"]`;
    `authors` → `[["John Doe", "**Jane Smith**", "Bob Johnson"]]`; `summary` → `["This paper
    presents a new method for computer vision."]`; `doi` → `["10.48550/arXiv.2310.03138"]`;
    `journal` → `["Nature", "IEEE Conference on Computer Vision", "arXiv preprint"]`. `url` has
    **no** examples metadata (`publication.py:36-39`).

### 3.11 `PublicationEntry` — the `doi` pattern

17. `doi` carries `pattern=r"\b10\..*"` (`publication.py:34`). This is an **enforced constraint**,
    not schema decoration: a non-absent `doi` that does not match fails with §4.1.
18. The pattern is applied with **search** semantics, not anchored match: `prefix 10.5/x` is
    accepted (verified against the vendored Python). Only a `10.` preceded by a word boundary is
    required, anywhere in the value.
19. The word boundary is **Unicode-aware**. Verified against the vendored Python:

    | Value | Result |
    |---|---|
    | `10.1109/TASC.2023.3340648` | accepted |
    | `10.1` | accepted |
    | `10.` | accepted |
    | `prefix 10.5/x` | accepted |
    | `-10.5` | accepted |
    | `①10.5` | accepted |
    | `.10.5` | accepted |
    | `\t10.5` | accepted |
    | `10.5\n` | accepted |
    | `10` | rejected (§4.1) |
    | `notadoi` | rejected |
    | `abc10.5x` | rejected |
    | `9910.5` | rejected |
    | `_10.5` | rejected |
    | `ü10.5` | rejected |
    | `ß10.5` | rejected |

    The last two rows are the load-bearing ones: `ü` and `ß` count as word characters, so no
    boundary exists before `10.` and the value is rejected. An ASCII-only word-boundary
    implementation accepts them and diverges. See §5.1.
20. No trimming, casing or normalization is applied to `doi`. `10.5\n` keeps its newline
    (`publication.py:27-35` declares no such transform).

### 3.12 `PublicationEntry` — the three model-level rules

21. **`doi` beats `url`.** After field validation, if `doi` is not absent, `url` is set to absent
    (`publication.py:46-62`). Silent: no warning, no error. Verified — `{doi: "10.x", url:
    "https://example.com"}` validates to `url = None`.
22. **`doi_url`** is `https://doi.org/` concatenated with the `doi` value verbatim, or absent when
    `doi` is absent (`publication.py:80-96`). No stripping, no escaping:
    `doi = "10. spaced ?"` yields `https://doi.org/10. spaced ?`. It is a derived read-only
    value, not a field, and does not appear in the field order of behavior 15.
23. **`doi_url` is itself validated as an HTTP URL** when non-absent
    (`publication.py:64-78`, using the adapter at `publication.py:9`). This is reachable: a `doi`
    long enough to push `doi_url` past 2083 characters fails with §4.2, at an **empty** schema
    location (the error carries no field name — verified). Shorter pathological DOIs, including
    ones containing spaces, `#`, `\t`, `\n` and NUL, all pass.

### 3.13 Inherited field types

24. The complex-field base declares `location: str | None`, `summary: str | None`,
    `highlights: list[str] | None` (`entry_with_complex_fields.py:106-132`), and
    `BasePublicationEntry` declares `authors: list[str]` and `summary: str | None`
    (`publication.py:19-26`). Every one of these is type-enforced: a mapping supplied where text
    is declared fails with §4.4, a scalar supplied where a list is declared fails with §4.5, and a
    non-text list element fails with §4.4 at the element's own index.
25. Iteration 2 bound `location`, `summary` and `highlights` as raw document nodes without
    checking their types. Enforcing behavior 24 is therefore an **iteration-2 gap this iteration
    closes**, not new upstream behavior. It is required here because it is the shape of the
    child errors the section-level §4.12 record carries, and iteration 4 renders those.
26. `date`, `start_date` and `end_date` keep the semantics spec 002 §3.69–§3.79 already pinned.
    Nothing in this iteration changes them; the eight models only inherit them.
26a. The reference date those semantics need for `present` (spec 002 §3.73 case 5) is
    **existing behavior this iteration depends on, not new work**. Its precedence is: a real date
    in the validation context wins; the literal string `today` resolves to the system's today;
    anything else — including absent and including an invalid value — falls back to today
    silently, deliberately, so that the settings model reports the error itself
    (`schema/models/validation_context.py:36-58`, rationale at `:42-45`; the context is the
    two-member record of `:9-12`). Spec 002 §3.26 pinned it and iteration 2 ported it. This
    iteration only threads it: every concrete type carrying date fields receives the same
    reference date, from the same source.

### 3.14 `TextEntry`

27. `TextEntry` is a bare string. It has no model, no fields, no validators and no extra-key
    surface (`section.py:23`, `:34`).
28. Its detection is positional in `get_entry_type_name_and_section_model`: the mapping branch
    runs first, then the string branch (`section.py:145`, `:162-165`). A string never reaches
    characteristic-field discrimination.
29. Its name is the literal `TextEntry`, produced in two places — the appended ninth name
    (`section.py:38`) and the string branch (`section.py:164`) — and once more as a section model
    name (behavior 31).

### 3.15 Section model names

30. For a model, the section model's name is `"SectionWith"` plus the class name with the first
    occurrence of `Entry` replaced by `Entries` (`section.py:110`). The eight names are therefore
    `SectionWithOneLineEntries`, `SectionWithNormalEntries`, `SectionWithExperienceEntries`,
    `SectionWithEducationEntries`, `SectionWithPublicationEntries`, `SectionWithBulletEntries`,
    `SectionWithNumberedEntries`, `SectionWithReversedNumberedEntries`.
31. For a string the name is the literal `SectionWithTextEntries` (`section.py:107`).
32. The section model declares `entry_type` as a one-value literal and `entries` as a homogeneous
    list of the one type (`section.py:113-118`), which is why a heterogeneous section fails
    through ordinary list-item validation rather than a dedicated check (spec 002 §3.61).
33. These names are user-visible only through the dynamic-model machinery upstream, which the port
    does not reproduce. They are pinned here because upstream's own test asserts them
    (`tests/schema/models/cv/test_section.py:19-60`) and the port must be able to answer the same
    question.

### 3.16 Discrimination against the real types

34. The characteristic-field table (spec 002 §3.55) is now computed from the eight real field
    sets. Verified at runtime against `section.py:77`:

    | Entry type | Characteristic fields |
    |---|---|
    | `OneLineEntry` | `details`, `label` |
    | `NormalEntry` | `name` |
    | `ExperienceEntry` | `company`, `position` |
    | `EducationEntry` | `area`, `degree`, `institution` |
    | `PublicationEntry` | `authors`, `doi`, `journal`, `title`, `url` |
    | `BulletEntry` | `bullet` |
    | `NumberedEntry` | `number` |
    | `ReversedNumberedEntry` | `reversed_number` |

    Common (characteristic of none): `date`, `start_date`, `end_date`, `location`, `summary`,
    `highlights`. `summary` is common because both `BaseEntryWithComplexFields` and
    `BasePublicationEntry` declare it (`entry_with_complex_fields.py:110`, `publication.py:23`).
35. This table is byte-for-byte the fixture table spec 002 §3.56 pinned. Every section test
    iteration 2 wrote must pass **unchanged** against the real registry; that equality is the
    whole point of the inversion (spec 002 §7.1).
36. The registry must be populated in the union order of behavior 2. Populating it in import
    order changes discrimination results and is a defect, not a style choice.

### 3.17 Extra keys and the downstream extension surface

37. All eight models allow extra keys and expose them by name, because every one of them descends
    from `BaseEntry` (`entries/bases/entry.py:11`) which descends from the extra-keys base
    (`base.py:8-9`). Upstream's test parametrizes this over all eight models
    (`tests/schema/models/cv/test_section.py:63-83`).
38. The renderer **writes** to that surface. `renderer/templater/entry_templates_from_input.py:216-221`
    does `setattr(entry, template_name, …)` for names such as `main_column`,
    `date_and_location_column` and `degree_column` — attributes that are not declared fields and
    exist only because extra keys are allowed. Templates then read them, e.g.
    `entry.main_column.splitlines()[:first_row_lines]`
    (`renderer/templater/templates/typst/entries/EducationEntry.j2.typ`), which is
    `AGENTS.md` §6 hazard 1.
39. **Requirement for this iteration:** the eight types must carry an open, writable extension
    surface keyed by name, and must **not** declare `main_column`, `date_and_location_column`,
    `degree_column` or any other renderer-injected name as a field. Declaring them would put them
    in the field order of §3.2 and therefore in the JSON Schema. The mechanism by which the
    renderer writes and templates read is iteration 8's (§7).
40. `entry_type_in_snake_case` (spec 002 §3.68) selects the theme's per-type template sub-model
    (`renderer/templater/entry_templates_from_input.py:120`, `:125`). The nine snake-case names
    must be exactly `one_line_entry`, `normal_entry`, `experience_entry`, `education_entry`,
    `publication_entry`, `bullet_entry`, `numbered_entry`, `reversed_numbered_entry`,
    `text_entry`.
41. The four trivial templates are recorded here so iteration 9 has the field-name contract in
    one place: `BulletEntry.j2.typ` is `- {{entry.bullet}}`, `NumberedEntry.j2.typ` is
    `+ {{entry.number}}`, `ReversedNumberedEntry.j2.typ` is `+ {{entry.reversed_number}}`,
    `TextEntry.j2.typ` is `{{entry}}`. Rendering them is iteration 9's.

### 3.18 Error codes

42. Section-level failures all carry `rendercv_other_error`, except the entry-problems wrapper
    which carries `rendercv_entry_validation_error` (`models/custom_error_types.py:4-6`;
    used at `section.py:158`, `:169`, `:214`, `:230`, `:240`). **Correction, recorded after
    verification:** an earlier draft of this behavior said "Iteration 2 already emits both". It
    did not — iteration 2 emitted only `rendercv_other_error` for all five, and the wrapper's
    type was fixed inside iteration 3 (commit `9ddd896`). Exactly one of the five raise sites,
    `:230`, is `entry_validation`; the other four are `.other`.
43. Entry-level failures carry pydantic's own codes, which iteration 4 uses to look up rewrite
    rules. The five this iteration emits are `missing`, `string_type`, `list_type`,
    `string_pattern_mismatch` and `url_too_long`. Verified by running the vendored Python; the
    matching texts are §4.1–§4.5.

### 3.19 Wiring

44. `models.Validate` must call `cv.Validate` (spec 002 §3.27–§3.31 describe a top-level model
    whose `cv` member is validated; iteration 2 left the call unmade for a package-cycle reason
    recorded in `specs/STATE.md`). After this iteration, validating a document validates its `cv`
    block, its sections, and its entries in one pass, in field-declaration order (pydantic's emission order, which is **not** document order).
45. The entry validator seam iteration 2 injected as a test stub
    (`internal/schema/models/cv/sectionvalidation.go`) must dispatch to the real types. No
    production code path may retain the accept-everything stub.

---

## 4. Exact user-visible strings

Every string is verbatim. `{...}` marks an interpolation. §4.1–§4.5 are pydantic's own text, not
RenderCV's; they fall under the deferred-string decision of spec 002 §7.3 and are pinned here so
that a later decision to diverge shows up as a test diff. §4.6–§4.12 are metadata strings: they
are not error text, but they are emitted verbatim into the JSON Schema in iteration 5.

### 4.1 `doi` does not match the pattern — `publication.py:34`

Code `string_pattern_mismatch`. Location: the `doi` field.

```
String should match pattern '\b10\..*'
```

The pattern appears in the message exactly as written in the source, including the backslashes.

### 4.2 `doi_url` is too long — `publication.py:64-78`

Code `url_too_long`. Location: **empty** — the error carries no field name (verified).

```
URL should have at most 2083 characters
```

### 4.3 A required field is absent

Code `missing`. Location: the field.

```
Field required
```

Already emitted by the binder as of iteration 2.

### 4.4 A value declared as text is not text

Code `string_type`. Location: the field, or the field plus the element index for a list element.

```
Input should be a valid string
```

### 4.5 A value declared as a list is not a list

Code `list_type`. Location: the field.

```
Input should be a valid list
```

### 4.6 `url` fails to parse — `publication.py:36-39`

Code `url_parsing`. Location: the `url` field. Observed for the input `example.com`:

```
Input should be a valid URL, relative URL without a base
```

The trailing clause is the URL library's own reason string and varies with the input. This is why
`url` parsing is deferred with the other HTTP-URL work (§7).

### 4.7 `reversed_number` description — `reversed_numbered.py:9-11`

```
Reverse-numbered list item. Numbering goes in reverse (5, 4, 3, 2, 1), making recent items have higher numbers.
```

### 4.8 `area` description — `education.py:12`

```
Field of study or major.
```

### 4.9 `authors` description — `publication.py:20`

```
You can bold your name with **double asterisks**.
```

### 4.10 `doi` description — `publication.py:30-32`

```
The DOI (Digital Object Identifier). If provided, it will be used as the link instead of the URL.
```

### 4.11 `url` description — `publication.py:38`

```
A URL link to the publication. Ignored if DOI is provided.
```

### 4.12 `journal` description — `publication.py:42`

```
The journal, conference, or venue where it was published.
```

### 4.13 The `doi_url` prefix — `publication.py:94`

```
https://doi.org/
```

---

## 5. Edge cases

1. **The `doi` word boundary is Unicode-aware.** Behavior 19's table. Go's `regexp` implements
   `\b` over ASCII word characters only, so `\b10\..*` compiled as-is accepts `ü10.5` and
   `ß10.5`, which upstream rejects. Measured: upstream rejects both; Go's `regexp` with the
   literal pattern accepts both. Not covered by any upstream test. This is an implementation
   constraint, not a divergence — the boundary must be evaluated against a Unicode word class.
2. **`doi_url` validation is reachable but only through length.** A `doi` of `10.` plus 2100 `a`s
   fails with §4.2; a `doi` of `10. spaced ?`, `10.###`, `10.\x00x` or `10.5\n` all validate and
   produce a `doi_url` containing those bytes verbatim (verified). Upstream's own test covers only
   the happy path and the absent case (`tests/schema/models/cv/entries/test_publication.py:7-17`).
3. **`doi_url` for the pinned DOI.** `10.1109/TASC.2023.3340648` →
   `https://doi.org/10.1109/TASC.2023.3340648`; absent → absent
   (`tests/schema/models/cv/entries/test_publication.py:10-11`).
4. **`url` is silently dropped, not rejected, when `doi` is present.** `{doi: "10.x", url:
   "https://example.com"}` validates and the `url` is gone (behavior 21). A user who supplies both
   gets no diagnostic.
5. **`url` is normalized by the URL library.** `https://example.com` becomes
   `https://example.com/` (trailing slash added) and `HTTPS://Example.COM/Path` becomes
   `https://example.com/Path` (scheme and host lowercased, path preserved). Verified against the
   vendored Python. This normalization is observable in rendered output, and Go's standard library
   does not perform it. **Parity risk — the decision belongs with the shared HTTP-URL work in
   iteration 4** (spec 002 §7 already assigns `cv.website` and `cv.photo` URL semantics there). No
   divergence is proposed here; see §7.3.
6. **A publication entry accepts date fields it does not have.** `{title, authors, start_date:
   2020}` validates, retaining `start_date` as an unknown key, and performs **no** date
   validation on it (behavior 16, verified). A user who writes `start_date: not-a-date` on a
   publication entry gets no error.
7. **A required field written as null is a type error, not a missing field.**
   `{title: null, authors: [a]}` produces §4.4 at `title`, not §4.3 (verified). The key is
   present; only its value is wrong.
8. **Missing-field errors come in declaration order.** `EducationEntry({})` reports `institution`
   then `area`, not alphabetically and not in input order (verified).
9. **List element errors are per index.** `{title: t, authors: [1, 2]}` produces two §4.4 records
   at `authors.0` and `authors.1` (verified).
10. **`authors` as a scalar is one error, and the missing `title` is reported alongside it.**
    `{authors: "notalist"}` produces §4.3 at `title` and §4.5 at `authors`, in that order
    (verified). Upstream's own fixture exercises the scalar-`authors` half
    (`tests/schema/testdata/test_pydantic_error_handling/wrong_input.yaml:54-56`).
11. **Extra keys are readable on all eight models.** Upstream parametrizes
    `entry_contents["extra_attribute"] = "extra value"` over `available_entry_models` and asserts
    `entry.extra_attribute == "extra value"` (`tests/schema/models/cv/test_section.py:63-83`).
12. **All nine types coexist in one CV.** Upstream builds one section per entry-type name — nine
    sections, two entries each, all from the conftest fixtures — and asserts every section has two
    entries (`tests/schema/models/cv/test_cv.py:13-36`, which iterates
    `available_entry_type_names`, i.e. including `TextEntry`).
13. **The mixed-section case, now against real types.** `[education_entry, experience_entry]`
    resolves the section to `EducationEntry` and then fails on the second entry
    (`tests/schema/models/cv/test_cv.py:38-52`). Upstream's error fixture pins exactly what the
    second entry reports: `institution` and `area` both missing, and the `company`/`position` keys
    it *does* carry reported not at all, because extra keys are allowed
    (`tests/schema/testdata/test_pydantic_error_handling/wrong_input.yaml:20-24` →
    `expected_errors.yaml:50-67`). This closes carried item 6 of `specs/STATE.md`.
14. **An arbitrary-date string on a real type still passes.** `{company: "Company C", location:
    …, date: "No."}` reports only the missing `position`; `date: "No."` is a legal arbitrary date
    (`wrong_input.yaml:25-28` → `expected_errors.yaml:69-79`).
15. **The seven-type discrimination table.** Upstream asserts the `(entry_type_name,
    SectionModel.__name__)` pair for `publication`, `experience`, `education`, `normal`,
    `one_line`, `text` and `bullet` fixtures, twice each — once from the raw dict and once from a
    constructed model instance (`tests/schema/models/cv/test_section.py:19-60`). `numbered` and
    `reversed_numbered` are **not** in that table; they are covered only by behavior 11's
    parametrization and by edge case 12.
16. **A constructed model instance reports its own class name.** The already-validated branch
    (`section.py:173-176`) bypasses characteristic fields entirely, so an `EducationEntry`
    instance carrying no `institution` still reports `EducationEntry`.
17. **The conftest fixtures are the canonical corpus.** Their exact contents
    (`tests/schema/models/cv/conftest.py:6-128`) must validate with zero errors for all nine
    types, including the Unicode `Boğaziçi University`, the markdown-bearing highlights, the
    triple-asterisk author `***H. Tom***`, and the `date: "2021-12-08"` publication.
18. **`degree` is the only optional own field among the eight.** Every other own field of every
    type is required (behaviors 8–15). A `degree`-less education entry is valid.
19. **`OneLineEntry`, `BulletEntry`, `NumberedEntry` and `ReversedNumberedEntry` have no date
    fields.** They descend directly from `BaseEntry` (`one_line.py:6`, `bullet.py:6`,
    `numbered.py:6`, `reversed_numbered.py:6`). A `date` written on one is an unknown key and is
    not validated as a date.
20. **`summary` is not characteristic of `PublicationEntry`** even though `PublicationEntry`
    declares it, because the complex-field base declares it too (behavior 34). An entry of only
    `{summary: …}` matches nothing and produces spec 002 §4.9.
21. **The date rejection and acceptance tables of spec 002 §5.23 are inherited unchanged** by
    `NormalEntry`, `ExperienceEntry` and `EducationEntry`, and by `PublicationEntry` for `date`
    alone. Upstream's own date tests construct a bare `BaseEntryWithComplexFields`
    (`tests/schema/models/cv/entries/bases/test_entry_with_complex_fields.py`); this iteration
    must show at least one concrete type reaching the same outcomes.

### 5.1 Optional-field combinations no upstream fixture covers

The existing 42-case corpus already exercises all nine types — one shared `cv:` block does it,
byte-identical across the nine theme example files, reaching 16 golden cases
(`examples/John_Doe_ClassicTheme_CV.yaml:16-183`; section keys at `:17-21` TextEntry, `:22-45`
Education, `:46-101` Experience, `:102-120` Normal, `:121-159` Publication, `:160-165` Bullet,
`:166-174` OneLine, `:175-178` Numbered, `:179-183` ReversedNumbered). `NormalEntry`,
`NumberedEntry` and `ReversedNumberedEntry` appear **only** in that block; the `ats_*` cases repeat
Education, Experience, OneLine, Bullet and Publication.

The combinations below occur in **no YAML file anywhere in the submodule**, so no golden output
exists for them and none can be generated. Each is an acceptance criterion in §8.

22. **`EducationEntry` with `degree` omitted.** Every education entry in the submodule sets it
    (`examples/John_Doe_ClassicTheme_CV.yaml:25`, `:37`;
    `tests/…/minimal.yaml:19`; `standard_full.yaml:51`, `:59`; `diacritics.yaml:35`, `:41`;
    `academic.yaml:21`, `:29`, `:35`). `degree` is the only optional own field in the iteration
    (edge case 18), so its absent branch is entirely unexercised.
23. **A bare `date` on an education or experience entry.** The key is present but always blank
    (`examples/John_Doe_ClassicTheme_CV.yaml:26`, `:38`). Bare `date` with a real value is
    exercised for `NormalEntry` only — the "NeuralPrune" entry, `date: '2021'`
    (`examples/John_Doe_ClassicTheme_CV.yaml:113-115`). So spec 002 §3.77 step 1 (`date` silently
    clears the range fields) is never reached on a type that also carries `start_date`/`end_date`
    in a golden.
24. **`PublicationEntry.url` with a real value.** `doi` is set on every publication in the
    submodule (`examples/John_Doe_ClassicTheme_CV.yaml:128`, `:137`, `:147`, `:156`;
    `academic.yaml:71`, `:79`, `:86`, `:93`, `:99`) and `url` is always blank or absent — the only
    non-blank `url:` values anywhere in the submodule's YAML are a CI workflow,
    `error_dictionary.yaml` and `mkdocs.yaml`. **Consequence: behavior 21
    (`ignore_url_if_doi_is_given`) and behavior 23 (`validate_doi_url`) are exercised by no golden
    case at all**, and neither is the URL normalization of edge case 5. Their parity rests
    entirely on unit tests. See §7.3.
25. **A non-blank `summary` on an education, experience or publication entry.** Always blank or
    absent; only `NormalEntry` carries real summaries
    (`examples/John_Doe_ClassicTheme_CV.yaml:108`, `:117`).
26. **`end_date: present` is covered** and needs nothing added
    (`examples/John_Doe_ClassicTheme_CV.yaml:51`, `:104-106`; `minimal.yaml:12`;
    `diacritics.yaml:20`; `academic.yaml:45`).
27. **Upstream's only mechanism for this matrix is dead code in the pinned tree.**
    `create_combinations_of_entry_type` (`tests/renderer/conftest.py:342-383`) with
    `return_value_for_field` (`:215-339`, which does define both
    `"doi": "10.1007/978-3-319-69626-3_101-1"` and `"url": "https://example.com"` at `:282-283`)
    is consumed only by the `full_rendercv_model` fixture (`:124-212`, calling it at `:174-177`),
    and nothing in the pinned tree consumes that fixture — it matches only its own file. It builds
    Python model instances directly, never serializes them to YAML, and never runs the CLI, so
    there is **no upstream output to lift**. How these combinations are covered instead is §7.4.
28. **A stop-word-free multi-word section key.** `ats_academic` uses `academic_positions` for a
    structurally-`ExperienceEntry` list (`academic.yaml:41-63`), which exercises
    `dictionary_key_to_proper_section_title` (spec 002 §3.62) on a key where no word is in the
    28-word stop list. Already covered by a golden; recorded so it is not re-tested here.
29. **`err_wrong_input` is not type coverage.** `wrong_input.yaml:17-56` is entry-shaped, but every
    entry is deliberately malformed and the case asserts exit 1. It is validation-error-path
    coverage and belongs to iteration 4; this iteration uses it only for the locations-and-codes
    differential of §8's last criterion.

---

## 6. Ordering and whitespace guarantees

1. **Per-type field order** (behaviors 8–15) is contractual and drives JSON Schema property order
   (iteration 5) and sample-YAML key order (iteration 13). The `PublicationEntry` case — `date`
   last — is the one most likely to be got wrong.
2. **Discrimination order** (behavior 2) is the union's, not the import order's. Reordering the
   registry changes which type an ambiguous entry resolves to.
3. **Error order within one entry** is: errors produced while walking the input's keys, in input
   order, then missing-required errors in declaration order (edge cases 8, 10). Iteration 2's
   binder already establishes this shape; nothing here changes it, and spec 002 §6.6's
   `TODO(iteration-4)` about the relative order of the two groups still stands.
4. **Error order across entries** is the section's list order (spec 002 §6.3).
5. **No whitespace is produced.** This iteration writes no bytes. Every scalar it validates is
   returned as written, with no trimming — `doi = "10.5\n"` keeps its newline (behavior 20),
   which matters because `AGENTS.md` §6 hazard 2 makes whitespace observable downstream.

---

## 7. Out of scope

| Deliberately excluded | Owned by |
|---|---|
| Rendering entry-level errors, message rewriting, coordinate resolution | iteration 4 |
| HTTP-URL parsing and normalization, including `PublicationEntry.url` (§5.5) and §4.6's text | iteration 4 |
| The three pydantic-borrowed error texts of §4.1–§4.5 as a *decision* — pinned here, decided there | iteration 4 |
| Emitting the examples/description metadata of §4.7–§4.12 into JSON Schema, and property order | iteration 5 |
| The theme-level `Templates` model that declares `main_column`, `degree_column` etc. (`schema/models/design/classic_theme.py`) | iteration 6 |
| The mechanism by which the renderer writes injected attributes and templates read them (§3.17) | iteration 8 |
| Rendering the four trivial templates of behavior 41 | iteration 9 |
| Sample-entry generation from the examples metadata | iteration 13 |

Carried items from `specs/STATE.md` → *Cut scope → Iteration 2*:

- **Item 2** (`models.Validate` does not call `cv.Validate`) is resolved here — behavior 44, and
  `plan.md` §2 for the package move that unblocks it.
- **Item 6** (§4.12's mixed-section and entry-problems criteria tested through a stub) is resolved
  here — behavior 45, edge case 13, and §8's *Wiring* criteria.
- **Items 1, 3, 4 and 5** — the coordinate-column shapes, `phone` formatting and its spec
  self-contradiction, the missing `dealias` no-op regression test, and the hand-written scalar
  corpus — belong to **iteration 4** and are deliberately not absorbed here. This iteration must
  not touch `internal/schema/yamlreader` or `cv.phone`.

Four decisions recorded so they are not relitigated:

**7.1 The gate is unit tests, not conformance cases, and there is no golden refresh.** As in
spec 002 §7.2: no corpus case can pass until the renderer exists, so the parity suite stays at its
42 red cases and that redness is not a failure of this iteration. Beyond that, **all nine entry
types are already exercised by the existing 42-case corpus** (§5.1's opening paragraph), so
iteration 3 needs no new corpus case and **must not run `just golden`**. There is deliberately no
golden-refresh task in `tasks.md` and no human gate for one; regenerating `testdata/golden/` here
would change the contract for no coverage gain (`AGENTS.md` §5).

**7.2 The registry stays inverted.** Upstream computes the characteristic table at import time
from the classes (`section.py:36-42`, `:77`). The port keeps spec 002 §7.1's registry and merely
populates it (behavior 36). Iteration 2's fixture registry is deleted in the same commit that
adds the real one. **Correction, recorded after verification:** this section originally required
that every iteration-2 section test pass unchanged. Three did not, and could not: their fixtures
were ones the accept-everything stub let pass and that upstream genuinely rejects, so their
expectations were corrected against the vendored Python. See `specs/STATE.md`, iteration 3 cut
scope, item 3. This changes no observable behavior and gets no entry in
`specs/divergences.md`.

**7.3 One parity risk is flagged, not diverged.** `PublicationEntry.url` normalization (§5.5) is
user-visible in rendered output and Go's standard library does not reproduce it. Rather than
propose a divergence, this iteration keeps `url` bound as a raw value behind the same registered
hook pattern iteration 2 used for `email`, `phone` and `website`, so the HTTP-URL decision is made
once, in iteration 4, for all four fields. **If iteration 4 concludes the normalization cannot be
reproduced, that is a divergence entry and a human gate — not something this spec authorizes.**

Note the coverage consequence, which **raises** the bar on that decision rather than lowering it:
`PublicationEntry.url` is exercised by **no golden case whatsoever** (§5.24), so nothing in the
parity suite will ever catch a wrong URL decision. Axis 1 is silent here. Whatever iteration 4
chooses must be justified against upstream directly, and its unit tests are the only gate it will
ever have.

**7.4 The uncovered combinations of §5.1 are covered by differential unit tests, and that is not a
hand-written golden.** Upstream has no fixture for them (§5.27), so there is nothing to generate
from. They are therefore covered the way iteration 2 covered its unpinned cases: a Go unit test
whose expectation was derived by running the vendored Python on the same input and recording what
it produced, with the input and the observed result both written into the test.

The distinction from `AGENTS.md` §10.1 is worth stating plainly, because a later reviewer will
otherwise misread it. §10.1 forbids hand-*editing* a file under `testdata/golden/`: those files are
the artifact-parity contract, they are produced mechanically by `tools/gengolden`, and a hand-edited
one silently invalidates every case that reads it. Nothing here writes to `testdata/golden/`,
nothing here claims artifact parity, and every expectation is a differentially-obtained value in a
unit test that names its provenance. Spec 002 §5.3, §5.6, §5.8, §5.21 and §5.22 are all the same
pattern, and iteration 2 shipped green on them.

---

## 8. Acceptance criteria

Each is a unit test. None requires the conformance suite.

**Field sets and order**

- [ ] Each of the eight models reports its field names in exactly the order of behaviors 8–15, as
      a table test. `PublicationEntry` is `title, authors, summary, doi, url, journal, date`.
- [ ] `OneLineEntry`, `BulletEntry`, `NumberedEntry` and `ReversedNumberedEntry` declare no
      `date`, `start_date`, `end_date`, `location`, `summary` or `highlights` (edge case 19).
- [ ] `PublicationEntry` declares no `start_date`, `end_date`, `location` or `highlights`
      (behavior 16).
- [ ] `degree` is optional; every other own field of every type is required (edge case 18).
- [ ] None of the eight declares `main_column`, `date_and_location_column` or `degree_column`
      (behavior 39).

**Per-type validation**

- [ ] Each of the nine conftest fixtures validates with zero errors against its own type
      (edge case 17), as a table test — the exact fixture bytes, including `Boğaziçi University`
      and `***H. Tom***`.
- [ ] Each of the eight models accepts `extra_attribute: "extra value"` and reads it back
      (edge case 11).
- [ ] Each of the eight models with a required own field reports §4.3 for it when absent, in
      declaration order (edge case 8).
- [ ] A required text field written as null reports §4.4, not §4.3 (edge case 7).
- [ ] A required text field written as a mapping reports §4.4 (edge case 7,
      `wrong_input.yaml:43-45`).
- [ ] `authors: "scalar"` reports §4.5; `authors: [1, 2]` reports §4.4 at index 0 and index 1
      (edge cases 9, 10).
- [ ] `location`, `summary` and `highlights` are type-enforced on a concrete complex-field type:
      a mapping in `summary` gives §4.4, a scalar in `highlights` gives §4.5, a non-text
      `highlights` element gives §4.4 at its index (behaviors 24, 25).
- [ ] A `NormalEntry` reaches spec 002 §5.23's rejection and acceptance outcomes through its
      inherited date fields (edge case 21).
- [ ] A publication entry with `start_date: not-a-date` validates with zero errors and retains
      the key (edge case 6, behavior 16).

**`PublicationEntry` specifics**

- [ ] Behavior 19's sixteen-row `doi` table, verbatim, including `ü10.5` and `ß10.5` as
      rejections (edge case 1). This criterion fails for any implementation using an ASCII-only
      word boundary.
- [ ] `prefix 10.5/x` is accepted — the pattern is a search, not an anchored match (behavior 18).
- [ ] A rejected `doi` produces §4.1 with the pattern text exactly as written in §4.1.
- [ ] `doi_url` is `https://doi.org/10.1109/TASC.2023.3340648` for the pinned DOI and absent for
      an absent DOI (edge case 3).
- [ ] `doi_url` preserves the `doi` bytes verbatim for `10. spaced ?`, `10.###`, `10.5\n`
      (behavior 22, edge case 2).
- [ ] `doi = "10." + 2100×"a"` produces §4.2 with an **empty** schema location (behavior 23).
- [ ] `{doi: "10.x", url: "https://example.com"}` validates with `url` absent and no diagnostic
      (behavior 21, edge case 4).
- [ ] `doi_url` does not appear in the field order (behavior 22).

**Combinations no golden covers** (§5.1; expectations differentially obtained per §7.4, each test
naming its provenance)

- [ ] An `EducationEntry` with `degree` omitted validates with zero errors and reports `degree`
      absent — not empty text (§5.22).
- [ ] An `EducationEntry` and an `ExperienceEntry` each carrying a real bare `date` alongside
      `start_date` and `end_date` reach spec 002 §3.77 step 1: `date` is kept, both range fields
      are cleared, no diagnostic (§5.23).
- [ ] A `NormalEntry`, `ExperienceEntry`, `EducationEntry` and `PublicationEntry` each with a
      non-blank `summary` validate with zero errors and retain the text verbatim (§5.25).
- [ ] The `doi`/`url` interaction across all four states (§5.24, behaviors 21–23), as one table
      test:

      | `doi` | `url` | Expected |
      |---|---|---|
      | absent | absent | both absent, `doi_url` absent |
      | `10.1007/978-3-319-69626-3_101-1` | absent | `doi` kept, `url` absent, `doi_url` set |
      | absent | `https://example.com` | `url` kept, `doi` absent, `doi_url` absent |
      | `10.1007/978-3-319-69626-3_101-1` | `https://example.com` | `doi` kept, `url` silently cleared, `doi_url` set, **no error** |

      The two values are upstream's own (`tests/renderer/conftest.py:282-283`), taken from the dead
      fixture of §5.27 so the port's inputs match the only inputs upstream ever chose.
- [ ] `end_date: present` needs no new test — it is golden-covered (§5.26). A criterion asserting
      it here is redundant and should not be added.

**Discrimination and section wiring**

- [ ] The characteristic table computed from the real registry equals behavior 34 exactly, and the
      common set is exactly `{date, start_date, end_date, location, summary, highlights}`.
- [ ] The fixture registry is gone from the tree and every iteration-2 section test runs against
      the real registry. **Not** "with no edit to the test": three tests asserted outcomes only the
      stub produced and were corrected against upstream (`specs/STATE.md`, iteration 3 cut scope,
      item 3).
- [ ] The registry's order is behavior 2's, asserted positionally, not as a set.
- [ ] The seven `(entry_type_name, section_model_name)` pairs of edge case 15 hold, both from a
      raw mapping and from a constructed entry.
- [ ] `numbered_entry` and `reversed_numbered_entry` fixtures also discriminate to their own
      types, and their section model names are `SectionWithNumberedEntries` and
      `SectionWithReversedNumberedEntries` (behavior 30) — the two rows upstream's table omits.
- [ ] `{summary: "only a summary"}` matches no type and produces spec 002 §4.9 (edge case 20).
- [ ] All nine snake-case names of behavior 40, as a table test.

**Wiring (closes carried items 2 and 6)**

- [ ] `models.Validate` on a document with a `cv` block reports that block's field errors,
      section errors and entry errors in field-declaration order (pydantic's emission order, which is **not** document order) (behavior 44). A stubbed or absent call
      is a failing criterion.
- [ ] No production code path reaches the accept-everything entry validator; the injectable seam
      exists only for tests (behavior 45).
- [ ] `[education_entry, experience_entry]` produces spec 002 §4.12 naming `EducationEntry`, with
      children reporting `institution` and `area` missing at entry index 1 and **nothing** about
      `company` or `position` (edge case 13).
- [ ] `[{x: 1}, <valid BulletEntry>]` produces spec 002 §4.12 naming `BulletEntry` with a child
      error on the first entry (spec 002 §5.8, now against a real type).
- [ ] A CV with nine sections, one per entry-type name, two fixture entries each, validates with
      zero errors and yields nine section records with the right entry types (edge case 12).
- [ ] The five entry-level codes of behavior 43 are all reachable and distinguishable without
      matching on message text.

**Differential fixture**

- [ ] Reading `tests/schema/testdata/test_pydantic_error_handling/wrong_input.yaml` from the
      submodule and validating it produces, for the nine `welcome_to_rendercv_tests*` sections,
      the same entry-level schema locations and error codes upstream's
      `expected_errors.yaml:44-140` records — locations and codes only. The rendered *messages*
      in that file are iteration 4's rewrites and are not compared here.

---

## 9. Corpus additions

**None, and no golden refresh.** Two independent reasons, per §7.1: no corpus case can be *checked*
until the renderer exists, and every one of the nine entry types is already *exercised* by the
existing 42 cases (§5.1). `tools/gengolden` is not run, `testdata/golden/` is not touched, and no
human gate is requested. The uncovered optional-field combinations of §5.1 are covered by the
differential unit tests of §7.4, not by fixtures.

One **read-only submodule fixture** is added instead, per §8's last criterion:
`tests/schema/testdata/test_pydantic_error_handling/wrong_input.yaml` is consumed directly from
`third_party/rendercv` as a differential unit fixture over entry-level locations and codes. It is
not copied into `testdata/golden/`, and the companion `expected_errors.yaml` stays iteration 4's
import (contract §4).

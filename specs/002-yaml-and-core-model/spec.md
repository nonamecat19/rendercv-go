# Spec 002 — YAML reader and core model

**Status:** draft · **Inherits:** [`specs/000-parity-contract/spec.md`](../000-parity-contract/spec.md)

**Upstream covered:**

- `src/rendercv/schema/yaml_reader.py`
- `src/rendercv/schema/rendercv_model_builder.py` (overlay-merge half only — see §7)
- `src/rendercv/schema/models/rendercv_model.py`
- `src/rendercv/schema/models/base.py`
- `src/rendercv/schema/models/validation_context.py`
- `src/rendercv/schema/models/path.py`
- `src/rendercv/exception.py`
- `src/rendercv/schema/models/cv/cv.py`
- `src/rendercv/schema/models/cv/section.py`
- `src/rendercv/schema/models/cv/entries/bases/entry.py`
- `src/rendercv/schema/models/cv/entries/bases/entry_with_date.py`
- `src/rendercv/schema/models/cv/entries/bases/entry_with_complex_fields.py`
- `src/rendercv/schema/models/cv/social_network.py`, `.../custom_connection.py` (typed shells only)

Citations below are relative to `third_party/rendercv/`.

---

## 1. Purpose

Turn a byte stream into a validated in-memory CV model, keeping every source coordinate the
later error renderer needs. This is the first two stages of the pipeline (`AGENTS.md` §2): parse
with a round-trip YAML reader that preserves key order and per-node line/column, then validate
the top-level document, the `cv` block, its sections, and the shared entry bases. It also
performs the overlay merge that folds `--design` / `--locale` / `--settings` files into the main
document before validation.

## 2. Inputs / Outputs

### Inputs

- A filesystem path, or a raw document string. The path form is what the CLI uses; the string
  form is what the model builder uses internally (`schema/yaml_reader.py:33`, `:50-51`).
- Optional overlay document strings for `design`, `locale`, `settings`
  (`schema/rendercv_model_builder.py:120-124`).
- Optional render-command overrides and dotted-key overrides
  (`schema/rendercv_model_builder.py:135-155`).
- A validation context: the input file's path (or absent), and a current date
  (`schema/models/validation_context.py:9-12`).

### Outputs

1. **A parsed document** — an ordered mapping where every node carries `(line, column)`
   coordinates of its start and end, keys keep input order, and no scalar has been coerced to a
   date/time (`schema/yaml_reader.py:11`, `:83-86`).
2. **A validated model** with four top-level members `cv`, `design`, `locale`, `settings`
   (`schema/models/rendercv_model.py:19-42`), plus the input file path recorded out-of-band
   (`schema/models/rendercv_model.py:44`, `:46-62`).
3. **Or** a user error (single message) or a user validation error (an ordered list of
   validation-error records) (`exception.py:20-41`).

Example input:

```yaml
cv:
  name: John Doe
  email: john.doe@example.com
  sections:
    education_and_training:
      - institution: MIT
        area: Computer Science
        start_date: 2018-09
        end_date: 2022-06
design:
  theme: classic
```

After parsing, `cv` is at line 1 column 1; `sections` at line 4 column 3; the single education
entry is a list element at line 5. After validation the section's title is
`Education and Training` and its entry type is `EducationEntry`.

A validation-error record carries five members (`exception.py:20-26`):

| Member | Meaning |
|---|---|
| `schema_location` | dotted path of model keys, or absent for YAML syntax errors |
| `yaml_location` | `((start_line, start_col), (end_line, end_col))`, 1-indexed lines, or absent |
| `yaml_source` | which of the four input documents produced it |
| `message` | the user-visible text |
| `input` | the offending value, echoed as a string |

`yaml_source` is one of exactly four literals (`exception.py:4-9`):

```
main_yaml_file
design_yaml_file
locale_yaml_file
settings_yaml_file
```

---

## 3. Behavior

### 3.1 Reading a document from a path

1. If the path does not exist, fail with the nonexistent-file message (§4.1).
   `schema/yaml_reader.py:34-37`.
2. Otherwise, if the path's final extension is not one of `.yaml`, `.yml`, `.json`, `.json5`,
   fail with the extension message (§4.2). `schema/yaml_reader.py:39-47`.
3. The extension check runs **after** the existence check and **before** the file is read
   (`schema/yaml_reader.py:35`, `:41`, `:49`). Therefore an existing, empty file with a wrong
   extension yields the extension error, not the empty-file error.
4. The file is read as UTF-8 (`schema/yaml_reader.py:49`).
5. The extension comparison is on the last suffix only, exact and case-sensitive:
   `archive.tar.YAML` is rejected, `cv.yaml` is accepted. `schema/yaml_reader.py:41`.

### 3.2 Reading a document from a string

6. A string input skips all four steps above and is parsed directly
   (`schema/yaml_reader.py:50-51`).

### 3.3 Post-parse checks (both forms)

7. If the parsed root is empty/absent, fail with the empty-file message (§4.3).
   `schema/yaml_reader.py:55-57`.
8. If the parsed root is itself a scalar string, fail with the passed-a-string message (§4.4) as
   an **internal** error, not a user error. `schema/yaml_reader.py:59-65`. The message
   interpolates the original input verbatim — for the string form, the string itself.
9. Otherwise the parsed root is returned as an ordered mapping (`schema/yaml_reader.py:67`).

### 3.4 Parser configuration

10. **`*` is not an alias.** The round-trip scanner's alias-fetch is replaced by plain-scalar
    fetch, so a `*` at the start of a token begins an ordinary plain scalar
    (`schema/yaml_reader.py:70-80`). `key: *not_an_alias` parses to the string
    `*not_an_alias` (`tests/schema/test_yaml_reader.py:40-44`).
10a. **Anchors are not patched.** Only alias *consumption* is replaced
    (`schema/yaml_reader.py:73-75` overrides `fetch_alias` and nothing else). An anchor
    definition `&name` is still scanned, still consumed, and still contributes nothing to the
    parsed value — but the alias that would refer to it is now a literal string, so an anchor
    can never be dereferenced. See §5.3 for the verbatim asymmetry.
11. **ISO timestamps are disabled.** The `tag:yaml.org,2002:timestamp` constructor is replaced
    by plain-scalar construction, so `2020-09-24` stays the string `2020-09-24` and never
    becomes a date object (`schema/yaml_reader.py:83-86`). All downstream date validation
    (§3.13–§3.15) is therefore string/int based.
12. Key order of every mapping is preserved (`schema/yaml_reader.py:11-31` — the round-trip
    loader is chosen precisely for this; `schema/models/cv/cv.py:166` relies on it).
13. Per-node source coordinates are retained for every mapping key and every list element
    (`schema/yaml_reader.py:14-18`).

### 3.5 Overlay merge and pre-validation shaping

14. `settings` is force-created if absent, and `settings.render_command` is force-created if
    absent, both as empty mappings, before anything else touches the document
    (`schema/rendercv_model_builder.py:118`;
    `tests/schema/test_rendercv_model_builder.py:36-42`).
15. Overlays are applied in the fixed order `settings`, `design`, `locale`
    (`schema/rendercv_model_builder.py:120-124`). An absent or empty overlay is skipped
    (`:128`; `tests/schema/test_rendercv_model_builder.py:124-135`).
16. An overlay document contributes **its own top-level key of the same name**, not the whole
    document: from a `design` overlay only `overlay["design"]` is taken, and it is assigned to
    `input["design"]` (`schema/rendercv_model_builder.py:132`).
17. The assignment **replaces**, it does not merge: a main document with
    `design: {theme: classic, font_size: 12pt}` plus a `design` overlay of `{theme: sb2nov}`
    yields exactly `{theme: sb2nov}` (`tests/schema/test_rendercv_model_builder.py:357-368`).
18. Each overlay's parsed document is retained alongside the merged result, keyed by overlay
    name, so error coordinates can later be resolved against the file the value actually came
    from (`schema/rendercv_model_builder.py:126`, `:133`).
19. Render-command overrides are written into `settings.render_command.<key>` for the eleven
    keys `output_folder`, `typst_path`, `pdf_path`, `markdown_path`, `html_path`, `png_path`,
    `dont_generate_typst`, `dont_generate_html`, `dont_generate_markdown`, `dont_generate_pdf`,
    `dont_generate_png` (`schema/rendercv_model_builder.py:135-151`).
20. An override is written only when its value is truthy. `false` and `0` and `""` are treated
    as "not supplied" and do not overwrite (`schema/rendercv_model_builder.py:150`).
21. Dotted-key overrides are applied last, after overlays and render-command overrides
    (`schema/rendercv_model_builder.py:153-155`). Their semantics are iteration 12's; only the
    ordering is fixed here.

### 3.6 Validation context

22. The context is threaded to validators under a nested key `"context"` — i.e. the object
    handed to validation is a mapping whose `"context"` member is the context record
    (`schema/rendercv_model_builder.py:176-183`; `schema/models/validation_context.py:29-31`).
23. The context has exactly two members, `input_file_path` and `current_date`, both defaulting
    to absent (`schema/models/validation_context.py:9-12`).
24. `current_date` is taken from `settings.current_date` in the merged document, falling back to
    the literal string `"today"` when `settings` or `settings.current_date` is absent
    (`schema/rendercv_model_builder.py:179-181`).
25. Reading the input file path is defensive: absent context, a context that is not a mapping,
    or an absent path all yield "no path" rather than an error
    (`schema/models/validation_context.py:29-33`;
    `tests/schema/models/test_validation_context.py:47-54`).
26. Reading the current date is defensive in the same way, and additionally: a real date value
    is returned as-is; the exact string `"today"` resolves to the system's today; **anything
    else** — including an invalid date string — falls back to today rather than failing, so that
    the settings model can report the error itself
    (`schema/models/validation_context.py:53-58`, and the rationale at `:44-46`).

### 3.7 Top-level model

27. The top-level model has exactly four fields, in this declaration order: `cv`, `design`,
    `locale`, `settings` (`schema/models/rendercv_model.py:19-42`).
28. Every one of the four has a default, so an empty document `{}` validates
    (`schema/models/rendercv_model.py:19`, `:24`, `:31`, `:38`). Defaults are: an empty `cv`,
    the `classic` theme, the English locale, and default settings.
29. The top-level model forbids unknown keys (§3.8, via
    `schema/models/rendercv_model.py:14`).
30. `cv` is deliberately omitted from the JSON-schema `required` list even though it is
    semantically required, so the same schema validates standalone design/locale/settings files
    (`schema/models/rendercv_model.py:15-18`). Emitting this is iteration 5's; the model must
    carry the marker.
31. After validation, the input file path from the context is recorded on the model out-of-band
    (not as a document field) for later path resolution
    (`schema/models/rendercv_model.py:44`, `:46-62`;
    `tests/schema/test_rendercv_model_builder.py:379-392`).

### 3.8 Base model kinds

32. There are exactly two base kinds (`schema/models/base.py`):
    - **without extra keys** — unknown keys are rejected (`schema/models/base.py:5`);
    - **with extra keys** — unknown keys are kept on the object and readable by name
      (`schema/models/base.py:9`; `tests/schema/models/cv/test_section.py:67-83`).
33. Both kinds validate their defaults, not just supplied values
    (`schema/models/base.py:5`, `:9`). A default that would fail validation fails at
    construction.
34. Neither kind defines field aliases. The YAML key is the field name, exactly.

### 3.9 Path types

35. There are two path types, differing only in whether existence is required
    (`schema/models/path.py:67-80`).
36. Both resolve a relative path against the **parent directory of the input file**, or against
    the process working directory when there is no input file
    (`schema/models/path.py:38-41`; `tests/schema/models/test_path.py:57-67`).
37. An absolute path is left unchanged by both (`schema/models/path.py:40`;
    `tests/schema/models/test_path.py:196-205`).
38. An empty path short-circuits: no resolution and no existence check
    (`schema/models/path.py:37`).
39. The existence-required type fails if the resolved path does not exist (§4.5) and fails if it
    exists but is not a regular file (§4.6) (`schema/models/path.py:43-55`;
    `tests/schema/models/test_path.py:47-55`, `:109-117`). Both messages interpolate the path
    **relative to the resolution base**, not the resolved absolute path
    (`schema/models/path.py:48`, `:54`).
40. The planned type performs no existence check and accepts nonexistent paths
    (`schema/models/path.py:74-76`; `tests/schema/models/test_path.py:130-139`).
41. The planned type serializes as a POSIX path relative to the process working directory when
    that is expressible, and otherwise as the absolute path — it never fails
    (`schema/models/path.py:60-64`; `tests/schema/models/test_path.py:241-259`). The
    existence-required type has no such serializer.

### 3.10 Exception taxonomy

42. Four distinct kinds (`exception.py`):
    - **validation-error record** — a plain data record with the five members of §2, *not* a
      raised error (`exception.py:20-26`);
    - **user error** — a single optional message; the CLI renders it in an `Error` panel
      (`exception.py:29-31`);
    - **user validation error** — carries an ordered list of validation-error records
      (`exception.py:34-36`);
    - **internal error** — a required message, for conditions that indicate a defect rather than
      bad user input (`exception.py:39-41`).
43. The overlay-name → source-literal mapping is fixed:
    `design → design_yaml_file`, `locale → locale_yaml_file`, `settings → settings_yaml_file`
    (`exception.py:13-17`).

### 3.11 The `cv` block

44. Fields, in declaration order, all optional and all defaulting to absent
    (`schema/models/cv/cv.py:32-118`):

    | # | Field | Shape |
    |---|---|---|
    | 1 | `name` | text |
    | 2 | `headline` | text |
    | 3 | `location` | text |
    | 4 | `email` | one email address, or a list of them |
    | 5 | `photo` | an existing file path relative to the input, **or** an HTTP URL |
    | 6 | `phone` | one phone number, or a list of them |
    | 7 | `website` | one HTTP URL, or a list of them |
    | 8 | `social_networks` | list of network records |
    | 9 | `custom_connections` | list of custom-connection records |
    | 10 | `sections` | mapping from section title to list of entries |

45. `cv` forbids unknown keys (`schema/models/cv/cv.py:31`).
46. `photo` resolves its union **left to right**: the file-path interpretation is attempted
    first, and only if it fails is the URL interpretation tried
    (`schema/models/cv/cv.py:52-57`). This is the only field in this iteration with a
    non-default union resolution order.
47. `email`, `phone` and `website` are validated by a single shared rule that inspects the value
    *before* any type coercion (`schema/models/cv/cv.py:177-229`):
    - absent → absent (`:205-206`);
    - a list → validated as a list of that field's element type (`:226-227`);
    - anything else → validated as one value of that field's element type (`:229`).
    Deciding list-vs-scalar first is what makes the resulting error message name the element
    type rather than reporting a union failure (`:190-196`).
48. If that shared rule is ever invoked without a field name it raises an internal error (§4.7)
    (`schema/models/cv/cv.py:208-209`; `tests/schema/models/cv/test_cv.py:93-98`).
49. `phone` serializes with any leading `tel:` removed (`schema/models/cv/cv.py:231-250`).
    `+905419999999` validates and serializes to `+90-541-999-99-99`
    (`tests/schema/models/cv/test_cv.py:85-91`). Phone normalization itself is not specified
    here — see §7.
50. **Key order is captured.** Before validation, the input mapping's key order is recorded; if
    the input is not a mapping, the recorded order is empty
    (`schema/models/cv/cv.py:166`). After validation, keys whose input value was null are
    dropped from the recorded order (`schema/models/cv/cv.py:173`). Keys absent from the input
    never appear. The order is used to render header connections in the user's order
    (`schema/models/cv/cv.py:124-126`).
51. If the value being validated is already a validated `cv` object, it is returned untouched
    and its recorded order is preserved (`schema/models/cv/cv.py:162-163`).
52. The typed section list is derived from `sections` on demand and cached
    (`schema/models/cv/cv.py:128-140`).

### 3.12 Sections

53. A section value must be a list. Anything else fails with the not-a-list message (§4.8)
    (`schema/models/cv/section.py:238-242`; `tests/schema/models/cv/test_cv.py:67-74`).
54. An empty list is returned unchanged, with **no** type inference
    (`schema/models/cv/section.py:196-197`; `tests/schema/models/cv/test_section.py:108-111`).
55. **Characteristic fields.** For the set of entry types, count how many types declare each
    field name (including inherited names). A field declared by exactly one type is
    *characteristic* of that type; a field declared by two or more is common to all and
    characteristic of none (`schema/models/cv/section.py:61-74`).
56. For upstream's eight entry types the characteristic sets are exactly
    (verified against `schema/models/cv/section.py:77` at runtime):

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

    The common (non-characteristic) fields are `date`, `start_date`, `end_date`, `location`,
    `summary`, `highlights`.
57. **Priority order is load-bearing.** Candidate types are tested in the union's declaration
    order, and the **first** whose characteristic set intersects the entry's keys wins
    (`schema/models/cv/section.py:24-33` for the order, `:148-154` for the first-match break).
    The order is:

    ```
    OneLineEntry, NormalEntry, ExperienceEntry, EducationEntry,
    PublicationEntry, BulletEntry, NumberedEntry, ReversedNumberedEntry
    ```

    A ninth name, `TextEntry`, exists as an entry type but not as a model, and is appended after
    the eight for enumeration purposes (`schema/models/cv/section.py:37-39`).
58. Entry-type inference, per entry (`schema/models/cv/section.py:128-178`):
    - a mapping → first type whose characteristic fields intersect its keys (`:145-155`);
      no intersection → the no-match error (§4.9) (`:156-160`);
    - a bare string → `TextEntry` (`:162-165`);
    - null → the null-entry error (§4.10) (`:167-171`);
    - an already-validated entry object → its own type (`:173-176`).
59. **Section type is decided by the first resolvable entry.** Entries are walked in order; an
    entry that raises during inference is skipped and the next is tried
    (`schema/models/cv/section.py:201-210`). Note the consequence: because the null-entry error
    of §4.10 is one of the skipped errors, a null entry never surfaces its own message here —
    see §5.6.
60. If no entry in a non-empty list resolves, the section fails with the no-entry-types message
    (§4.11) (`schema/models/cv/section.py:212-217`).
61. Once decided, **all** entries in the section are validated against that one type. Any
    failure is re-raised as the entry-problems message (§4.12), which names the detected type
    and carries the nested failures as a sub-list
    (`schema/models/cv/section.py:225-236`; `tests/schema/models/cv/test_cv.py:38-52`). The
    nested failures must be preserved for iteration 4's renderer, not flattened to text.
62. Section titles: if the key contains a space **or** any uppercase letter, it is used
    unchanged (`schema/models/cv/section.py:277-278`). Otherwise `_` becomes a space, the
    result is split on spaces, and each word is capitalized unless it is in the stop list
    (`:280-317`).
63. The stop list has **28** words, in this order (`schema/models/cv/section.py:283-312`):

    ```
    a, and, as, at, but, by, for, from, if, in, into, like, near, nor, of, off, on,
    onto, or, over, so, than, that, to, upon, when, with, yet
    ```

    The stop test applies to every word including the first: a key beginning with a stop word
    keeps it lowercase.
64. Capitalization is Python's `str.capitalize()`: title-case the first character and lowercase
    the rest (`schema/models/cv/section.py:315`). See §5.9 for the Unicode consequences.
65. The typed section list (`schema/models/cv/section.py:320-359`): for each `title → entries`
    pair, in input order, produce a record of `title` (formatted per §3.62), `entry_type`, and
    `entries` (unvalidated a second time). An **empty** entry list forces `entry_type` to
    `TextEntry` (`:342-343`; `tests/schema/models/cv/test_cv.py:76-83`); otherwise the type is
    inferred from the first entry only, which is safe because the whole list has already been
    validated to one type (`:344-349`).
66. A section record exposes a snake-case form of its title: lowercased with spaces replaced by
    underscores (`schema/models/cv/section.py:85-87`).

### 3.13 Entry bases — extra keys and type name

67. The entry base **allows extra keys**, and this is the arbitrary-keys feature, not an
    oversight: any unknown key a user writes on an entry is retained and readable
    (`schema/models/cv/entries/bases/entry.py:11` inheriting
    `schema/models/base.py:9`; `tests/schema/models/cv/test_section.py:67-83`).
68. Every entry exposes its type name in snake case, derived by inserting `_` before each
    uppercase letter that is not the first character, then lowercasing
    (`schema/models/cv/entries/bases/entry.py:8`, `:14-18`). So `ReversedNumberedEntry` →
    `reversed_numbered_entry`.

### 3.14 Entry bases — dates

69. **Arbitrary date** (the `date` field). The value may be an integer or text. It is converted
    to text and then (`schema/models/cv/entries/bases/entry_with_date.py:10-31`):
    - matching `\d{4}-\d{2}-\d{2}` in full → must parse as a real ISO date, else fail (§4.13);
    - else matching `\d{4}-\d{2}` in full → `-01` is appended and must parse as a real ISO date,
      else fail (§4.13);
    - else → accepted unchanged. `Fall 2023`, `Summer 2020`, `2020` all pass through.

    The original value is returned; no coercion happens (`:31`).
70. `date` defaults to absent (`schema/models/cv/entries/bases/entry_with_date.py:42-50`).
71. **Exact date** (the `start_date` and `end_date` fields). The value must be parseable by the
    date-object rule of §3.15. A structural failure yields the not-a-valid-date message
    (§4.14); a range failure (month 99, day 99) propagates the underlying date-library message
    (§4.13) (`schema/models/cv/entries/bases/entry_with_complex_fields.py:15-37` — note the
    handler catches only the internal error, so range errors pass through unwrapped).
72. `end_date` additionally accepts the exact literal `present`
    (`schema/models/cv/entries/bases/entry_with_complex_fields.py:98-105`).

### 3.15 Date-object conversion

73. Ordered format handling
    (`schema/models/cv/entries/bases/entry_with_complex_fields.py:43-87`):

    | # | Condition | Result |
    |---|---|---|
    | 1 | value is an integer | January 1 of that year (`:67-68`) |
    | 2 | full match `\d{4}-\d{2}-\d{2}` | that ISO date (`:69-71`) |
    | 3 | full match `\d{4}-\d{2}` | the 1st of that month (`:72-74`) |
    | 4 | full match `\d{4}` | January 1 of that year (`:75-77`) |
    | 5 | value is exactly `present` | the reference date (`:78-83`) |
    | 6 | anything else | internal error `This is not a valid date!` (`:84-85`) |

    The conditions are tried strictly in this order and the first match wins.
74. Case 5 with no reference date supplied is an internal error (§4.15)
    (`schema/models/cv/entries/bases/entry_with_complex_fields.py:79-82`;
    `tests/.../test_entry_with_complex_fields.py:21`, `:38-40`).
75. Cases 2–4 use full-string matching, so `20222` and `202222-12-20` reach case 6 and fail
    (`tests/.../test_entry_with_complex_fields.py:23-24`).
76. Cases 2 and 3 can match structurally but still fail on range: `2022-20-20` matches case 2
    and then fails with a range message (§4.13)
    (`tests/.../test_entry_with_complex_fields.py:26`).

### 3.16 Date precedence and ordering check

77. After an entry with complex fields is validated, four steps run in this order
    (`schema/models/cv/entries/bases/entry_with_complex_fields.py:134-171`):
    1. If `date` is present, `start_date` and `end_date` are **silently discarded**
       (set to absent) — no warning, no error (`:140-143`).
    2. Else if `end_date` is present and `start_date` is absent, `date` is set to the
       `end_date` value and both `start_date` and `end_date` are cleared (`:144-149`).
    3. Else if `start_date` is present and `end_date` is absent, `end_date` becomes the literal
       `present` (`:150-153`).
    4. If, after the above, both `start_date` and `end_date` are set, they are converted to date
       objects using the context's current date as the reference for `present`, and
       `start_date` after `end_date` fails with the ordering message (§4.16) (`:155-169`).
78. The ordering check is the only one of the four that can fail. Steps 1–3 are silent
    rewrites.
79. Fields of an entry with complex fields, in declaration order after the inherited `date`:
    `start_date`, `end_date`, `location`, `summary`, `highlights`; all optional, all defaulting
    to absent (`schema/models/cv/entries/bases/entry_with_complex_fields.py:93-132`).

### 3.17 Typed shells

80. A social-network record has exactly two required fields, `network` and `username`, and
    forbids unknown keys (`schema/models/cv/social_network.py:53-57`). `network` is one of
    seventeen literals, in this order (`schema/models/cv/social_network.py:13-31`):

    ```
    LinkedIn, GitHub, GitLab, IMDB, Instagram, ORCID, Mastodon, StackOverflow,
    ResearchGate, YouTube, Google Scholar, Telegram, WhatsApp, Leetcode, X, Bluesky, Reddit
    ```

    Per-network username pattern validation and URL generation are **out of scope** (§7).
81. A custom-connection record has exactly three fields in declaration order —
    `fontawesome_icon` (required text), `placeholder` (required text), `url` (an HTTP URL or
    null, with no default, therefore required-but-nullable) — and forbids unknown keys
    (`schema/models/cv/custom_connection.py:6-9`).

### 3.18 YAML syntax errors

82. A YAML parse failure is converted into a single user validation error rather than escaping
    as a parser exception (`schema/rendercv_model_builder.py:84-101`).
83. The record has no `schema_location`, a `yaml_location` derived from the parser's marks, the
    `yaml_source` of the document being parsed, the message of §4.17, and the literal input echo
    `...` (`schema/rendercv_model_builder.py:92-100`;
    `tests/schema/test_rendercv_model_builder.py:137-160`).
84. The parser message used in §4.17 is the **first line** of the parser's own error text,
    stripped, with a `.` appended if it does not already end in one
    (`schema/rendercv_model_builder.py:87-89`).
85. Parser marks: the start mark is the context mark if present else the problem mark; the end
    mark is the problem mark if present else the context mark; if either is still absent the
    location is absent. Both line and column are converted from 0-indexed to 1-indexed
    (`schema/rendercv_model_builder.py:42-62`;
    `tests/schema/test_rendercv_model_builder.py:759-765`).

---

## 4. Exact user-visible strings

Every string below is verbatim. `{...}` marks an interpolation.

### 4.1 Input file does not exist — `schema/yaml_reader.py:36`

```
The input file `{file_path}` doesn't exist!
```

`{file_path}` is the path exactly as given by the caller, not resolved.

### 4.2 Wrong extension — `schema/yaml_reader.py:42-46`

```
The input file should have one of the following extensions: .yaml, .yml, .json, .json5. The input file is {file_name}.
```

`{file_name}` is the final path component. The extension list is joined with `, ` in the
declared order.

### 4.3 Empty input — `schema/yaml_reader.py:56`

```
The input file is empty!
```

### 4.4 A string was passed instead of a path — `schema/yaml_reader.py:60-64`

Internal error.

```
You probably meant to pass a path to the YAML file, but you passed as a string and RenderCV interpreted it as the contents of the YAML file. Pass the path using `pathlib.Path({file_path_or_contents})`.
```

### 4.5 Required file missing — `schema/models/path.py:47`

```
The file `{file_path}` does not exist.
```

`{file_path}` is relative to the resolution base (`schema/models/path.py:48`).

### 4.6 Required file is not a file — `schema/models/path.py:53`

```
The path `{path}` is not a file.
```

`{path}` is relative to the resolution base (`schema/models/path.py:54`).

### 4.7 Missing field name in the scalar-or-list validator — `schema/models/cv/cv.py:209`

Internal error.

```
field_name is None in validator
```

### 4.8 Section is not a list — `schema/models/cv/section.py:241`

```
Each section should be a list of entries! This is not a list.
```

### 4.9 Entry matches no type — `schema/models/cv/section.py:159`

```
The entry does not match any entry type.
```

### 4.10 Entry is null — `schema/models/cv/section.py:170`

```
The entry cannot be None.
```

Not reachable through normal section validation — see §5.6.

### 4.11 Section matches no entry type — `schema/models/cv/section.py:215-217`

```
RenderCV couldn't match this section with any entry types. Please check the entries and make sure they are provided correctly.
```

### 4.12 Entries failed validation — `schema/models/cv/section.py:231-233`

```
There are problems with the entries. RenderCV detected the entry type of this section to be {entry_type_name}. The problems are shown below.
```

`{entry_type_name}` is one of the nine names of §3.57. The nested failures travel with the
record.

### 4.13 Date is structurally well-formed but out of range

Produced by the underlying date library, surfaced unwrapped. Observed values (CPython 3.12,
verified by running upstream):

```
month must be in 1..12
day is out of range for month
year 0 is out of range
```

Rendered by the error layer with a `Value error, ` prefix. See §5.10 — this is a parity risk.

### 4.14 Exact date is malformed — `schema/models/cv/entries/bases/entry_with_complex_fields.py:34-35`

```
This is not a valid date! Please use either YYYY-MM-DD, YYYY-MM, or YYYY format.
```

### 4.15 `present` with no reference date — `schema/models/cv/entries/bases/entry_with_complex_fields.py:81`

Internal error.

```
current_date is None when processing 'present' date
```

### 4.16 Start date after end date — `schema/models/cv/entries/bases/entry_with_complex_fields.py:163-164`

```
`start_date` cannot be after `end_date`. The `start_date` is {start_date} and the `end_date` is {end_date}.
```

Both interpolations are the values **as the user wrote them** (string or integer), not the
parsed date objects (`:165-168`).

### 4.17 YAML syntax error — `schema/rendercv_model_builder.py:97`

```
This is not a valid YAML file. {parser_message}
```

`{parser_message}` is defined by §3.84 and is the YAML library's own text. See §5.11 — this is
a parity risk.

### 4.18 Internal date failure — `schema/models/cv/entries/bases/entry_with_complex_fields.py:85`

Internal error, never surfaced to a user through normal validation.

```
This is not a valid date!
```

---

## 5. Edge cases

1. **Wrong extension beats empty file.** An existing zero-byte file named `x.txt` yields §4.2,
   not §4.3, because the extension check precedes the read
   (`schema/yaml_reader.py:41` vs `:49`, `:55`). Upstream's tests cover each in isolation
   (`tests/schema/test_yaml_reader.py:22-27`, `:33-38`) but not the combination.
2. **A path-looking string is a document, not a path.** `read_yaml("plain_string.yaml")` parses
   the text `plain_string.yaml` as a document, gets a scalar string root, and raises the
   internal error of §4.4 (`tests/schema/test_yaml_reader.py:29-31`).
3. **The anchor/alias asymmetry.** Upstream keeps anchors working and makes aliases inert. Only
   the first of these is covered by an upstream test
   (`tests/schema/test_yaml_reader.py:40-44`); the rest were obtained by running `read_yaml`
   against the vendored Python and are recorded here verbatim as the port's reference:

   ```
   'key: *not_an_alias\n'                        -> {'key': '*not_an_alias'}
   'mixed: *a and more\n'                        -> {'mixed': '*a and more'}
   'multi:\n  - *one\n  - *two\n'                -> {'multi': ['*one', '*two']}
   'nested:\n  inner: *deep_value\n'             -> {'nested': {'inner': '*deep_value'}}
   'real_anchor: &anchor value\nuse: *anchor\n'  -> {'real_anchor': 'value', 'use': '*anchor'}
   'highlights:\n  - normal *star* here\n'       -> {'highlights': ['normal *star* here']}
   "b: '*quoted'\n"                              -> {'b': '*quoted'}
   ```

   The fifth line is the load-bearing one and has **no upstream test**: `&anchor value` is
   consumed normally and yields the plain value `value`, while `*anchor` on the next line stays
   the literal string `*anchor` rather than resolving to it. A port that suppresses aliases by
   suppressing anchors too would pass the other six cases and fail this one.

   `mixed: *a and more` is the second load-bearing case: the whole remainder of the line is one
   plain scalar, not a `*a` token followed by trailing text.

   Cases six and seven show the boundaries: a `*` that is not at token start is ordinary text
   already, and quoting already suppresses aliasing, so neither is affected by the patch. Every
   other YAML feature is unmodified.
4. **A date-looking scalar stays a string.** `2020-09-24` under any key is the six-plus-four
   character string, never a date (`schema/yaml_reader.py:83-86`).
5. **Empty section list.** `References: []` validates to `[]` and produces a section record with
   `entry_type` = `TextEntry` (`tests/schema/models/cv/test_cv.py:76-83`,
   `tests/schema/models/cv/test_section.py:108-111`).
6. **A section that is `[null]`** reports §4.11, *not* §4.10. The null-entry error is raised
   during inference, and the inference loop swallows every such error and moves to the next
   entry (`schema/models/cv/section.py:206-210`); with no entries left, §4.11 fires. Verified by
   running upstream. `tests/schema/models/cv/test_section.py:102-105` asserts only that it
   fails, so the message is not pinned by upstream's own suite.
7. **Mixed entry types in a section.** `[education_entry, experience_entry]` resolves the
   section to `EducationEntry` from the first entry, then fails validating the second, producing
   §4.12 with `EducationEntry` (`tests/schema/models/cv/test_cv.py:38-52`).
8. **An unresolvable entry followed by a resolvable one.** `[{x: 1}, {bullet: b}]` resolves the
   section to `BulletEntry` (from the second entry) and then fails on the first with §4.12.
   Verified by running upstream. Not covered by upstream's tests.
9. **Section-title casing.** Upstream's table
   (`tests/schema/models/cv/test_section.py:86-99`), reproduced exactly:

   | Key | Title |
   |---|---|
   | `this_is_a_test` | `This Is a Test` |
   | `welcome_to_rendercv!` | `Welcome to Rendercv!` |
   | `Welcome to RenderCV!` | `Welcome to RenderCV!` |
   | `\faGraduationCap_education` | `\faGraduationCap_education` |
   | `\faGraduationCap Education` | `\faGraduationCap Education` |
   | `Hello_World` | `Hello_World` |
   | `Hello World` | `Hello World` |

   Row 4 is the uppercase guard firing (`C` in `faGraduationCap`), which is why the underscore
   survives. Row 2 shows `rendercv` capitalizing to `Rendercv`, and `to` staying lowercase.
10. **Unicode capitalization.** Python's `str.capitalize()` applies the **full** title-case
    mapping to the first character and lowercases the remainder. Verified: `ßeta` → `Sseta`,
    `ﬁle` → `File`, `ǆab` → `ǅab`, `çay` → `Çay`. A single-rune uppercase mapping is not
    equivalent. Additionally, title-case-category characters such as `ǅ` are not "uppercase", so
    a key containing one passes the §3.62 guard and is then lowercased in non-first positions.
    No upstream test covers any of this.
11. **Range-error text is not RenderCV's.** §4.13's three strings come from CPython's
    `date.fromisoformat`. `tests/.../test_entry_with_complex_fields.py:52-55` exercises
    `2020-99-99` and `2020-10-12`/`2020-99-99` but asserts only that validation fails, so the
    text is unpinned upstream while still being user-visible. **Parity risk — needs a decision
    in iteration 4.** No divergence entry is proposed here.
12. **YAML parser message is not RenderCV's.** §4.17 interpolates ruamel's own first error line.
    A Go YAML parser will not emit the same text.
    `tests/schema/test_rendercv_model_builder.py:137-160` asserts only that the record exists
    with the right source and a non-null location. **Parity risk — needs a decision in
    iteration 4.** No divergence entry is proposed here.
13. **BOM handling is unspecified.** The file is read as UTF-8
    (`schema/yaml_reader.py:49`); nothing strips a byte-order mark, and no upstream test covers
    a BOM'd input. **Open question — do not invent behavior.** Resolve by generating a corpus
    case when the renderer exists.
14. **Non-mapping, non-string, non-null entries crash upstream.** A section of `[1]` or `[[]]`
    reaches the already-validated-object branch (`schema/models/cv/section.py:173-176`) and
    fails with an unhandled key lookup rather than a validation error. Verified by running
    upstream. **Open question:** reproduce the crash, or produce §4.9? Not covered by any
    upstream test. Flag for the iteration-4 gate.
15. **`_key_order` and nulls.** `cv: {name: John, email: null}` records only `name`
    (`schema/models/cv/cv.py:173`). This makes "absent" and "present but null" indistinguishable
    in the recorded order — but *not* in extra-key rejection, where a null-valued unknown key is
    still rejected (`schema/models/base.py:5`).
16. **Non-mapping input to `cv`.** The recorded key order is empty rather than an error
    (`schema/models/cv/cv.py:166`); the subsequent validation reports the type failure.
17. **Falsy render-command overrides are dropped.** Passing "don't generate PDF = false"
    produces no key at all, so a `false` in the YAML is not overwritten
    (`schema/rendercv_model_builder.py:150`). Not covered by upstream's tests, which only pass
    `True` (`tests/schema/test_rendercv_model_builder.py:162-201`).
18. **Overlay of a document that lacks its own key.** A `design` overlay whose document has no
    `design` key fails with a key lookup, not a validation error
    (`schema/rendercv_model_builder.py:132`). No upstream test.
19. **`current_date` fallback is silent.** `settings.current_date: yesterday` still lets every
    date validator run against today, and the failure is reported by the settings model
    (`schema/models/validation_context.py:53-58`;
    `tests/schema/test_rendercv_model_builder.py:434-459`). Settings is iteration 7's; the
    fallback behavior is this iteration's.
20. **`"today"` survives to the model.** `settings.current_date: today` validates and the model
    holds the string `today`, not a date
    (`tests/schema/test_rendercv_model_builder.py:470-477`).
21. **Lone `end_date` migrates.** An entry with only `end_date: 2021-02-03` ends up with
    `date = 2021-02-03` and both range fields cleared. Verified by running upstream
    (`schema/models/cv/entries/bases/entry_with_complex_fields.py:144-149`).
22. **`date` silently wins.** An entry with `date`, `start_date` and `end_date` all set keeps
    only `date`; the discarded values produce no diagnostic
    (`schema/models/cv/entries/bases/entry_with_complex_fields.py:140-143`).
23. **Date rejection table**, from `tests/.../test_entry_with_complex_fields.py:14-27` and
    `:44-58`. All must fail:

    | Input | Where |
    |---|---|
    | `present` with no reference date | §3.74 |
    | `invalid`, `20222`, `202222-20200`, `202222-12-20` | §3.75 |
    | `2022-20-20` | §3.76 |
    | `start_date: aaa` | §4.14 |
    | `end_date: aaa` / `invalid_end_date` | §4.14 |
    | `start_date: 2023-01-01`, `end_date: 2021-01-01` | §4.16 |
    | `start_date: 2022`, `end_date: 2021` | §4.16 |
    | `start_date: 2025`, `end_date: 2021` | §4.16 |
    | `start_date: 2020-99-99` | §4.13 |
    | `end_date: 2020-99-99` | §4.13 |
    | `date: 2020-20-20` | §4.13 |

    And these must pass: `2020-01-01`, `2020-01`, `2020`, the integer `2020` — all resolving to
    January 1, 2020 (`tests/.../test_entry_with_complex_fields.py:16-20`).
24. **Extra keys on entries are readable.** Every one of the eight entry types accepts an
    unknown key and exposes its value (`tests/schema/models/cv/test_section.py:63-83`). The
    shells in this iteration must already permit this.
25. **Path resolution formats.** `subdir/file.txt`, `../sibling/file.txt`, `./same_dir/file.txt`
    all resolve against the input file's parent
    (`tests/schema/models/test_path.py:80-100`, `:165-181`).
26. **Path given as text.** A path supplied as a string validates identically to one supplied as
    a path value (`tests/schema/models/test_path.py:102-107`, `:183-189`).
27. **Overlay round-trip through the model.** A `design` overlay must reach the model *and* the
    merged dictionary, and must not leave a file path anywhere in
    `settings.render_command` (`tests/schema/test_rendercv_model_builder.py:632-646`).
28. **Overlays are independent.** A `design` overlay must not disturb a `locale` present in the
    main document (`tests/schema/test_rendercv_model_builder.py:712-727`).
29. **Empty document string.** `build_rendercv_dictionary_and_model("")` raises the plain user
    error of §4.3, not a validation error
    (`tests/schema/test_rendercv_model_builder.py:624-626`).

---

## 6. Ordering and whitespace guarantees

1. **Mapping key order is preserved** end to end. `cv`'s recorded order (§3.50) is a direct
   observable in the rendered header, and `sections` iteration order (§3.65) is the order
   sections appear in every output artifact.
2. **Section order** is the input mapping's order, unfiltered
   (`schema/models/cv/section.py:339`).
3. **Entry order within a section** is the input list order, unfiltered
   (`schema/models/cv/section.py:222`).
4. **Entry-type priority order** (§3.57) is fixed and must not be reordered; a different order
   changes which type an ambiguous entry resolves to.
5. **Overlay application order** is `settings`, `design`, `locale` (§3.15). Only observable if
   an overlay document carries more than one top-level key, which upstream never reads.
6. **Error order** within a user validation error is the order the validator produced them.
   Preserving it is an Axis 4 requirement (contract §4).
7. **Source coordinates.** Coordinate *capture* is in scope; the *rules that turn a schema path
   into a coordinate pair* are not. Those rules differ for mapping keys and list indices
   (`schema/pydantic_error_handling.py:198-217`): for a list index the pair is
   `((line+1, col-1), (line+1, col))`; for a mapping key it is
   `((line+1, col+1), (line+1_end, col_end))`. Note the asymmetric `col-1` versus `col+1` —
   it is not a typo in this spec. `expected_errors.yaml` records the resulting pairs. Only the
   1-indexed **line** numbers are surfaced to users, as `{source}: line N` or
   `{source}: line N to line M` (`src/rendercv/cli/render_command/progress_panel.py:14-36`).
   Consuming coordinates is iteration 4's.
8. **No whitespace is produced** by this iteration. Nothing here writes bytes.

---

## 7. Out of scope

| Deliberately excluded | Owned by |
|---|---|
| The eight concrete entry types (`entries/{bullet,education,experience,normal,numbered,one_line,publication,reversed_numbered}.py`) | iteration 3 |
| Per-network username patterns and URL generation (`social_network.py:59-184`) | iteration 4 |
| `pydantic_error_handling.py`, `error_dictionary.yaml`, message rewriting, coordinate resolution, error rendering | iteration 4 |
| Phone-number parsing and formatting (upstream delegates to a phone library) | iteration 4 |
| Email and HTTP-URL validation semantics | iteration 4 |
| JSON-schema emission, including the `required: []` marker of §3.30 | iteration 5 |
| `design` models and themes | iteration 6 |
| `locale` models and catalogs | iteration 7 |
| `settings` models, including `current_date` validation | iteration 7 |
| `override_dictionary.py` (dotted-key override semantics) — only its position in the pipeline is fixed here (§3.21) | iteration 12 |

Three decisions recorded so they are not relitigated:

**7.1 Entry-registry inversion (a design note, not a divergence).** Upstream computes the
characteristic-field table at import time from the concrete entry classes
(`schema/models/cv/section.py:36-42`, `:77`), so section discrimination structurally depends on
entry types this iteration does not port. This iteration therefore defines an **entry-type
registry**: a type registers its name and its full field-name set, and the characteristic-field
computation (§3.55) and discrimination (§3.57–3.58) run against the registry. The registry is
populated with a fixture reproducing upstream's eight field sets (§3.56) for this iteration's
tests; iteration 3 populates the real registry and the same tests must still pass unchanged.
This changes no observable behavior, so it is not a divergence and gets no entry in
`specs/divergences.md`.

**7.2 The gate for this iteration is unit tests, not conformance cases.** No corpus case can
pass until the renderer exists (iteration 9 and later). Nothing here adds a corpus case and
nothing here regenerates goldens. The parity suite stays red for its existing 42 cases
throughout, and that redness is **not** a failure of this iteration. §8's criteria are unit
tests mirroring upstream's pytest cases; §9 is empty by design.

**7.3 Two strings are not reproducible verbatim and are deferred.** §4.13 (CPython date range
errors) and §4.17's `{parser_message}` (ruamel's parser text) are user-visible but originate
outside RenderCV. This spec records the observed values and flags both as parity risks
(§5.11, §5.12). Deciding what `rendercv-go` emits is iteration 4's, and it must pass the human
gate if the answer is a divergence.

---

## 8. Acceptance criteria

Each is a unit test. None requires the conformance suite.

**Reading**

- [ ] Nonexistent path produces §4.1 with the path interpolated exactly as supplied.
- [ ] Each of `.txt`, `.YAML`, `.yamls`, no extension produces §4.2 with the file's final
      component interpolated; each of `.yaml`, `.yml`, `.json`, `.json5` is accepted.
- [ ] An existing zero-byte `x.txt` produces §4.2, not §4.3 (§5.1).
- [ ] An existing zero-byte `x.yaml` produces §4.3.
- [ ] A document string whose root is a scalar string produces the internal error §4.4 with the
      string interpolated.
- [ ] All seven cases of §5.3 parse to their recorded values, as a table test. The fifth —
      `real_anchor: &anchor value` / `use: *anchor` → `{real_anchor: "value", use: "*anchor"}` —
      is a separate criterion because it is the only one that constrains the anchor half, and it
      has no upstream test (§3.10a, §5.3).
- [ ] `d: 2020-09-24` parses to the string `2020-09-24`; likewise `2020-09-24T10:00:00Z` (§5.4).
- [ ] A document with keys in a non-alphabetical order round-trips that order.
- [ ] Every mapping key and list element in a fixture document reports the line and column
      upstream's reader reports, as a table test.
- [ ] A malformed document produces exactly one validation-error record with no schema
      location, the correct source, a non-absent YAML location, and input echo `...` (§3.83).
- [ ] A parser error with neither mark yields an absent YAML location (§3.85).

**Merging**

- [ ] `settings` and `settings.render_command` exist after merge even for a document containing
      neither (§3.14).
- [ ] Each of the three overlays replaces its key wholesale; `{theme: classic, font_size: 12pt}`
      + `{theme: sb2nov}` = `{theme: sb2nov}` (§5, `test_overlay_replaces_not_merges`).
- [ ] Absent/empty overlays are no-ops.
- [ ] A `design` overlay leaves a `locale` in the main document untouched (§5.28).
- [ ] Each of the eleven render-command override keys lands at
      `settings.render_command.<key>`; a falsy value lands nowhere (§3.20, §5.17).
- [ ] Overlay documents are retained per overlay name for later coordinate resolution (§3.18).

**Context, paths, exceptions**

- [ ] With no context, the input file path reads as absent and the current date as today
      (§3.25, §3.26).
- [ ] `current_date` of `"today"` resolves to today; a real date resolves to itself; `yesterday`
      falls back to today without erroring (§3.26, §5.19).
- [ ] Both path types resolve `subdir/f`, `../sib/f`, `./same/f` against the input file's parent
      and leave absolute paths alone (§5.25).
- [ ] With no input file path, both resolve against the working directory (§3.36).
- [ ] The existence-required type produces §4.5 for a missing file and §4.6 for a directory,
      each interpolating the path relative to the resolution base (§3.39).
- [ ] The planned type accepts a nonexistent path (§3.40) and serializes relative-to-cwd when
      expressible, absolute otherwise, never failing (§3.41).
- [ ] The overlay-name → source-literal map has the three entries of §3.43 and no others.

**Top-level and `cv`**

- [ ] `{}` validates, yielding the four defaults of §3.28.
- [ ] An unknown top-level key is rejected; an unknown top-level key whose value is null is also
      rejected (§5.15).
- [ ] Field order is `cv`, `design`, `locale`, `settings` (§3.27).
- [ ] The input file path is recorded on the model when supplied and absent otherwise (§3.31).
- [ ] `cv` accepts each of the ten fields of §3.44 and rejects an eleventh.
- [ ] `email`, `phone`, `website` each accept a scalar and a list, and route to the list
      validator only when the input is a list (§3.47).
- [ ] The scalar-or-list rule invoked with no field name raises the internal error §4.7 (§3.48).
- [ ] `phone: "+905419999999"` serializes to `+90-541-999-99-99` with no `tel:` (§3.49).
- [ ] `photo` tries the path interpretation before the URL interpretation (§3.46) — assert by
      supplying a value valid as both and checking which won.
- [ ] `_key_order` for `{name, email: null, location}` is `[name, location]` (§3.50, §5.15).
- [ ] `_key_order` for a non-mapping input is empty (§5.16).

**Sections**

- [ ] Given the fixture registry of §7.1, the computed characteristic-field table equals §3.56
      exactly, and the common set is exactly
      `{date, start_date, end_date, location, summary, highlights}`.
- [ ] Discrimination priority follows §3.57: a constructed entry carrying characteristic fields
      of two types resolves to the earlier one in the declared order.
- [ ] A bare string resolves to `TextEntry`; a mapping with no characteristic field produces
      §4.9; null produces §4.10 when inference is invoked directly (§3.58).
- [ ] A non-list section produces §4.8 (§3.53).
- [ ] `[]` returns `[]` with no inference (§3.54).
- [ ] `[null]` produces §4.11, not §4.10 (§5.6).
- [ ] `[{x: 1}]` produces §4.11 (§3.60).
- [ ] `[{x: 1}, <valid BulletEntry>]` produces §4.12 naming `BulletEntry` (§5.8).
- [ ] A mixed education/experience section produces §4.12 naming `EducationEntry` (§5.7).
- [ ] §4.12's nested failures are retained as structured children, not flattened (§3.61).
- [ ] The seven title-casing rows of §5.9 pass as a table test.
- [ ] All 28 stop words stay lowercase in a non-first position and in the first position
      (§3.63).
- [ ] `str.capitalize()` equivalence for `ßeta`, `ﬁle`, `ǆab`, `çay` (§5.10).
- [ ] `References: []` yields one section record titled `References` with entry type
      `TextEntry` and no entries (§3.65).
- [ ] Section records preserve input order; `snake_case_title` of `Education and Training` is
      `education_and_training` (§3.66).

**Entry bases**

- [ ] An unknown key on an entry base is retained and readable (§3.67).
- [ ] `ReversedNumberedEntry` → `reversed_numbered_entry`; `OneLineEntry` →
      `one_line_entry` (§3.68).
- [ ] The date-object table of §3.73 as a table test, including the integer case and `present`
      with an explicit reference date.
- [ ] `present` with no reference date raises the internal error §4.15 (§3.74).
- [ ] `20222`, `202222-20200`, `202222-12-20`, `invalid` all reach case 6 (§3.75).
- [ ] `2022-20-20`, `2020-99-99`, `2020-01-99`, `0000-01-01` fail with §4.13's respective texts
      (§3.76, §5.11) — pinned by a table test so a later decision to diverge is visible.
- [ ] Arbitrary dates: `Fall 2023`, `Summer 2020`, `2020`, `2020-09`, `2020-09-24`, the integer
      `2020` all pass; `2020-20-20` and `2020-09-99` fail (§3.69).
- [ ] Exact dates reject `aaa` with §4.14 (§3.71); `end_date: present` is accepted (§3.72).
- [ ] The four-step precedence of §3.77 as a table test: `date` present clears both range
      fields; lone `end_date` becomes `date`; lone `start_date` implies `present`; ordering
      violation produces §4.16 with the user's original spellings interpolated.
- [ ] The full rejection table of §5.23 fails, and its four accept cases pass.

**Shells**

- [ ] A social-network record requires both fields, rejects an unknown key, and accepts exactly
      the seventeen names of §3.80 and no others.
- [ ] A custom-connection record requires `fontawesome_icon` and `placeholder`, requires `url`
      to be present but permits it to be null, and rejects an unknown key (§3.81).

---

## 9. Corpus additions

**None.** Per §7.2, no corpus case can exercise this iteration until the renderer exists, and
`tools/gengolden` is not run. Iteration 4 adds the first cases that depend on this subsystem, by
importing `tests/schema/testdata/test_pydantic_error_handling/{wrong_input,expected_errors}.yaml`
(contract §4).

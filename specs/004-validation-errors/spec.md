# Spec 004 — Validation-error parity

**Status:** draft · **Inherits:** [`specs/000-parity-contract/spec.md`](../000-parity-contract/spec.md)

**Upstream covered:**

- `schema/pydantic_error_handling.py` (all 258 lines)
- `schema/error_dictionary.yaml` (all 13 rows)
- `schema/rendercv_model_builder.py:42-101` (the YAML-syntax error half; spec 002 covered the
  overlay-merge half)
- `schema/models/cv/social_network.py:12`, `:59-184` (per-network username rules and the
  generated URL)
- `schema/models/cv/cv.py:15-28`, `:44`, `:52-57`, `:58-74`, `:75-82`, `:231-250` (email, photo,
  phone, website semantics and phone serialization)
- `schema/models/cv/custom_connection.py:9`, `schema/models/cv/entries/publication.py:9`, `:36`
  (the other two `pydantic.HttpUrl` sites)
- `schema/models/path.py:43-55` (the two path messages)
- `schema/models/design/design.py:60-88` (the three custom-theme-shape messages)
- `schema/models/locale/locale.py:38-41` (the discriminated union whose branch element is dropped)
- `cli/render_command/progress_panel.py:14-36`, `:137-169` (recorded here, implemented in
  iteration 12 — §7)

Citations are relative to `third_party/rendercv/src/rendercv/`, except those beginning with
`tests/`, `examples/` or `scripts/`, which are relative to `third_party/rendercv/`.

Every claim marked **measured** was obtained by running the vendored Python at
`third_party/rendercv` @ `v2.8` (`2eba248`) and recording its output.

---

## 1. Purpose

Turn the pile of raw validator failures iterations 2 and 3 accumulate into the exact
human-readable rows upstream prints: the same messages, at the same schema locations, pointing at
the same YAML coordinates, in the same order, with the same duplicates removed. This is parity
axis 4 (contract §4) and it is the last piece of the schema half of the pipeline. It also settles
the four borrowed-library surfaces — email, phone, HTTP URL and the CPython date texts — that
iterations 2 and 3 deliberately left as pass-through seams.

## 2. Inputs / Outputs

### Inputs

1. **An ordered list of raw validator failures**, each carrying a kind (pydantic's `type`), a raw
   location (a tuple whose elements are field names, list indices, or pydantic-core's synthetic
   schema-branch tags), a raw message, the offending input value, and an optional context
   dictionary (`pydantic_error_handling.py:35-39`).
2. **The parsed main document**, for coordinate lookup (`:37`, `:100`).
3. **Optionally, the parsed overlay documents**, keyed by their top-level key
   (`:38`, `:101-104`).

### Outputs

An ordered list of **validation-error records**, each with five members
(`exception.py:21-27`):

| Member | Meaning |
|---|---|
| `schema_location` | dotted path elements, all stringified, or absent for a YAML-syntax error |
| `yaml_location` | `((start_line, start_col), (end_line, end_col))`, 1-indexed, or absent |
| `yaml_source` | one of `main_yaml_file`, `design_yaml_file`, `locale_yaml_file`, `settings_yaml_file` |
| `message` | the rewritten, period-terminated human text |
| `input` | the offending value as text, or the literal `...` for a mapping or a sequence |

Example input document (`tests/schema/testdata/test_pydantic_error_handling/wrong_input.yaml:2-9`):

```yaml
cv:
  name: John Doe
  website: not_a_valid_url
  phone: not_a_valid_phone_number
  email:
    - not_a_valid_email
    - not_a_valid_email_2
  photo: photo_doesnt_exist.jpg
```

Example output, the first record
(`tests/schema/testdata/test_pydantic_error_handling/expected_errors.yaml:2-6`):

```yaml
schema_location: ["cv", "email", "0"]
message: An email address must have an @-sign.
input: not_a_valid_email
yaml_location: [[7, 5], [7, 6]]
yaml_source: main_yaml_file
```

That whole file — 25 records for that one input — is the normative fixture for this iteration.

---

## 3. Behavior

### 3.1 The two producers

1. There are exactly **two** producers of validation-error records, and they never mix in one
   run. A **YAML-syntax** failure produces one record with **no** schema location, built directly
   in `read_yaml_with_validation_errors` (`rendercv_model_builder.py:84-101`). Everything else
   produces records through `parse_validation_errors`
   (`pydantic_error_handling.py:130-176`), every one of which **always** has a schema location.
   §3.20 depends on this being an absolute split.
2. The syntax producer runs before the schema producer and short-circuits it: a document that
   fails to parse never reaches validation (`rendercv_model_builder.py:117`, `:129-131`).

### 3.2 The eleven-step per-error transform

3. Every raw failure goes through `parse_plain_pydantic_error` (`:35-127`) in this **exact
   order**. Reordering any two steps changes observable output; the steps that prove it are named
   in each row.

   | # | Step | Citation | Why the position matters |
   |---|---|---|---|
   | 1 | strip unwanted message prefixes | `:23`, `:50-51` | upstream's order; **unobservable** on every measured message — see behavior 3a |
   | 2 | drop the discriminator element for `design` / `locale` | `:53-55` | must precede the context override, which re-pins `design.theme` (§3.5) |
   | 3 | context overrides `input`, then `loc` | `:57-62` | must precede location building (§3.5) |
   | 4 | build `location`, dropping synthetic elements | `:64-68` | must precede the two special cases, which read `location[-1]` |
   | 5 | `end_date` message override | `:71-75` | must precede the dictionary, which is why the dictionary's own `end_date` row is dead (§3.4 row 1) |
   | 6 | `current_date` suffix strip, then message override | `:81-87` | the strip must precede the containment test on `location[-1]` |
   | 7 | dictionary substitution, first match wins | `:89-92` | must precede the period rule |
   | 8 | append `.` if absent | `:94-95` | **strictly last** message step; applies to matched and unmatched messages alike (§3.6) |
   | 9 | choose `yaml_source` and the coordinate document | `:97-104` | reads `location[0]`, so it follows step 4 |
   | 10 | resolve coordinates, truncating the path for a missing key | `:106-119` | §3.10 |
   | 11 | render `input` as text | `:122-126` | §3.11 |

3a. **Correction to row 1 of the table above.** It previously read "must precede the dictionary, or
   `value is not a valid phone number` never matches (§3.4 row 7)". That reason does not hold, and
   the correction is recorded rather than silently edited because a test was written against the
   wrong claim before it was caught.

   Two independent reasons the ordering is unobservable:

   - The phone message carries **no prefix at all**. Measured: `phone: bad` reports
     `value is not a valid phone number`, bare, as a `PydanticCustomError`. There is nothing for
     step 1 to strip, so row 7 matches with or without it.
   - More generally, substitution matches by **containment** and replaces the **whole** message,
     so a prefix can only ever *add* a match, never remove one. Neither prefix contains a
     dictionary key, so it adds none either. Measured across all five prefixed messages the port
     can produce: the matching row is identical before and after the strip.

   The port still runs step 1 first, because the order is upstream's and a future dictionary row
   could depend on it. What changes is only the justification — and
   `errorpipeline.TestPrefixStripDoesNotChangeWhichRowMatches` now asserts the unobservability, so
   if a later row makes the order matter, that test fails and this behavior is re-measured rather
   than quietly wrong.

4. **Step 1 strips two prefixes, unconditionally and by substring replacement, not by prefix
   test** (`:23`, `:50-51`):

   ```
   value is not a valid email address: 
   Value error, 
   ```

   Both include a trailing space. `str.replace` removes **every** occurrence anywhere in the
   message, not just a leading one. Measured: `value is not a valid email address: An email
   address must have an @-sign.` becomes `An email address must have an @-sign.`, and
   `Value error, month must be in 1..12` becomes `month must be in 1..12`.

4a. **The two prefixes have different provenance, and the port reproduces one and not the other.**
   This is a scoping decision, recorded here because §8's criteria depend on it.

   | Prefix | Where it comes from | In the port's raw records? |
   |---|---|---|
   | `value is not a valid email address: ` | an explicit message **template** in pydantic's source, `'value is not a valid email address: {reason}'`, used at two sites in `validate_email` | **yes** — reproduced verbatim |
   | `Value error, ` | pydantic-core wrapping a bare exception that **escaped** a validator | **no** — the port has no such mechanism |

   The first is source text the port can cite and reproduce; the second is a framework artifact of a
   framework the port does not have. Reproducing the second would mean fabricating a prefix purely
   so the strip could remove it. So `emailaddr`'s caller builds the prefixed message and the strip
   is **exercised by production data** — records 1 and 2 of the 25-record fixture are email failures
   whose final text is `An email address must have an @-sign.` (`expected_errors.yaml:3`, `:9`), so
   the email half of the strip is gated by the strongest test in the iteration. The
   `Value error, ` half is implemented, verified on synthetic records, and **inert** for every
   message the port itself produces. §8 says so rather than asserting a criterion that cannot fail.

4b. **The prefix is not a function of the error code.** Measured, three failures all coded
   `value_error`, only one of which carries it:

   | Failure | Code | Carries `Value error, `? |
   |---|---|---|
   | `email: bad` | `value_error` | no — it is a `PydanticCustomError` with the template of behavior 4a |
   | `phone: bad` | `value_error` | no — a `PydanticCustomError` with a fixed message |
   | `date: 2020-13-01` | `value_error` | **yes** — `Date.fromisoformat`'s `ValueError` escapes `validate_arbitrary_date` uncaught (`models/cv/entries/bases/entry_with_date.py:26-29`) |

   A `PydanticCustomError` is never wrapped, whatever code it declares. Only two upstream sites
   produce a bare exception that escapes a validator: `entry_with_date.py:26-29` and
   `models/design/design.py:132`. Both are inert for the port per behavior 4a.

### 3.3 The location builder

5. `location` is built by stringifying every element of the raw location and **dropping** any
   element whose string **contains** one of seven substrings (`:24-32`, `:64-68`):

   ```
   tagged-union
   list
   literal
   int
   str
   constrained-str
   function-
   ```

   The test is `item in str(location_element)` — substring containment on the element, in either
   direction from the reader's intuition: the *table entry* must appear *inside* the element.
   `constrained-str` is redundant, being itself a superstring of `str`.
6. The intent is to remove pydantic-core's synthetic union and schema-branch tags. Measured
   examples of what it removes:

   ```
   function-after[validate_exact_date(), union[str,int]]
   function-wrap[wrap_val()]
   function-after[<lambda>(), lax-or-strict[lax=union[json-or-python[json=function-after[path_validator(), str],python=is-instance[Path]],function-after[path_validator(), str]],strict=json-or-python[json=function-after[path_validator(), str],python=is-instance[Path]]]]
   literal['present']
   literal['today']
   ```

7. **The filter is not restricted to synthetic elements and this is user-visible.** Any real
   mapping key containing one of the seven substrings is silently deleted from the location.
   Measured, with four sections that each fail to match an entry type:

   | Section key | Contains | Resulting location |
   |---|---|---|
   | `interests` | `int` | `("cv", "sections")` |
   | `my_list` | `list` | `("cv", "sections")` |
   | `strengths` | `str` | `("cv", "sections")` |
   | `literally_fine` | `literal` | `("cv", "sections")` |
   | `normal_key` | — | `("cv", "sections", "normal_key")` |

   All four truncated locations are equal, so §3.8's deduplication collapses them to **one**
   record, whose coordinates point at the `sections` mapping rather than at any section. Four
   errors are reported as one, naming no section. `interests` and `strengths` are ordinary CV
   section names; this is reachable by an unmodified user. It is upstream behavior and the port
   reproduces it exactly.
8. A **list index** is stringified before the test, so index `0` becomes `"0"`. No decimal
   integer string contains any of the seven substrings, so indices always survive
   (`:64-68`; measured at `expected_errors.yaml:2`, `:56-57`).
9. When the raw location's **first** element is `design` or `locale`, the **second** element is
   dropped before anything else happens (`:53-55`). That element is the discriminated union's
   branch value, not a user-written key. Measured:

   | Raw location | After step 2 |
   |---|---|
   | `("design", "classic", "nope")` | `("design", "nope")` |
   | `("design", "classic", "page", "top_margin")` | `("design", "page", "top_margin")` |
   | `("locale", "english", "month")` | `("locale", "month")` |
   | `("design",)` | `("design",)` |

   The slice is `loc[:1] + loc[2:]`, so a one-element location is unchanged. `settings` is **not**
   in the list and keeps every element (`:53`).
10. An **empty** raw location makes `plain_error["loc"][0]` raise upstream (`:53`). Measured: an
    `IndexError` escapes `parse_plain_pydantic_error` uncaught. It is unreachable through the
    document pipeline, because the only failure with an empty location is `PublicationEntry`'s
    generated-DOI-URL length check (spec 003 §4.2) and §3.7's splice always prepends the
    wrapper's location to it. The port must guard the lookup rather than crash; see §5.19.

### 3.4 The message dictionary

11. `error_dictionary.yaml` is read once at import time (`:19-22`). Substitution is
    **substring containment on the message, in file order, first match wins, then `break`**
    (`:89-92`):

    ```python
    for old_error_message, new_error_message in error_dictionary.items():
        if old_error_message in plain_error["msg"]:
            plain_error["msg"] = new_error_message
            break
    ```

    It is **not** equality. `Input should be a valid URL` therefore also matches
    `Input should be a valid URL, relative URL without a base` and every other parse-failure
    reason, flattening them all to one message (§3.15 behavior 40).
12. The thirteen rows, in file order, with their reachability **measured** against the pinned
    tree. Verbatim text is §4.1–§4.11.

    | # | Key (raw message substring) | Live? |
    |---|---|---|
    | 1 | `Input should be 'present'` | **dead** — always pre-empted by §3.5's `end_date` override |
    | 2 | `Input should be a valid integer, unable to parse string as an integer` | **dead** — no int-only field exists; every int-typed field is `int \| str` |
    | 3 | `String should match pattern '\\d{4}-\\d{2}(-\\d{2})?'` | **dead** — twice over, §3.4 behavior 13 |
    | 4 | `String should match pattern '\\b10\\..*'` | **dead** — §3.4 behavior 13 |
    | 5 | `Input should be a valid URL` | live |
    | 6 | `Field required` | live |
    | 7 | `value is not a valid phone number` | live |
    | 8 | `month must be in 1..12` | live |
    | 9 | `day is out of range for month` | live |
    | 10 | `must be in range` | **dead** — no measured input produces it; maps to row 9's value anyway |
    | 11 | `Extra inputs are not permitted` | live |
    | 12 | `Input should be a valid list` | live |
    | 13 | `value is not a valid color` | live |

13. **Rows 3 and 4 are dead because their keys contain doubled backslashes and pydantic's
    messages contain single ones.** The YAML scalars are plain (unquoted), so YAML performs no
    escape processing and the keys literally read `'\\d{4}-\\d{2}(-\\d{2})?'` and `'\\b10\\..*'`
    (`error_dictionary.yaml:4-5`). Measured, an invalid `doi`:

    - raw message: `String should match pattern '\b10\..*'`
    - final message: `String should match pattern '\b10\..*'.`

    Row 4's replacement text never appears in any output. Row 3 is dead for a second, independent
    reason: no field anywhere declares a pydantic `pattern=` of `\d{4}-\d{2}(-\d{2})?`; the date
    formats are checked with `re.fullmatch` inside hand-written validators
    (`models/cv/entries/bases/entry_with_date.py:26`, `:28`;
    `models/cv/entries/bases/entry_with_complex_fields.py:69`, `:72`, `:75`) which raise their own
    messages. A port that "fixes" the backslashes or the pattern breaks parity in both rows.
14. **Row 13 matches a longer message.** Measured: `design.colors.body: notacolor` raises
    `value is not a valid color: string not recognised as a valid color`, row 13's key is a
    substring, and the whole message is replaced. The replacement ends in `"` so §3.6 appends a
    period, giving a final `)".` (§4.11).

### 3.5 The two special cases and the `!.` artifact

15. **`end_date`** (`:71-75`): if `location` is non-empty and `location[-1]` **contains** the
    substring `end_date`, the message is replaced with a fixed literal, **before** the dictionary
    runs. Containment, not equality — a field named `my_end_date` would also match.
16. That literal ends in `!` (`:74`, a stray `!` after the closing quote of `"present"`), the
    dictionary finds no substring match, and §3.6 appends a period. The final message therefore
    ends in **`!.`** — see §4.12. Measured end to end for `end_date: invalid_date`. **This looks
    like a typo and it is not the port's to fix**; there is no exemption from §3.6 for either
    special case.
17. Two raw failures arrive for one bad `end_date` — the exact-date branch at
    `("end_date", "function-after[validate_exact_date(), union[str,int]]")` and the literal branch
    at `("end_date", "literal['present']")` (measured). Both reduce to `("end_date",)` under §3.3,
    both get the same forced message, and §3.8 keeps only the first. The comment at `:69-70` says
    this is the whole reason the override exists.
18. **`current_date`** (`:81-87`): two steps. First, if `location` has at least two elements and
    `location[-1] == "date"` and `location[-2] == "current_date"`, the trailing `"date"` is
    dropped (`:81-82`). Then, if `location` is non-empty and `location[-1]` **contains**
    `current_date`, the message is replaced with a fixed literal (`:83-87`). Measured for
    `settings: {current_date: todady}`: raw locations `("settings", "current_date", "date")` and
    `("settings", "current_date", "literal['today']")` reduce to `("settings", "current_date")`,
    the first survives dedup, and the message is §4.13.
19. §4.13's literal already ends in a period (`:87`), so it comes out with a single `.`. That is
    luck, not an exemption; §3.6 still runs on it.

### 3.6 The unconditional trailing period

20. **Last** (`:94-95`):

    ```python
    if not plain_error["msg"].endswith("."):
        plain_error["msg"] += "."
    ```

    It applies to **every** message — dictionary-matched, specially-overridden, or untouched. Four
    measured consequences, each an acceptance criterion:

    | Raw or overridden message | Final |
    |---|---|
    | `Input should be a valid string` (no dictionary row) | `Input should be a valid string.` (`expected_errors.yaml:112`) |
    | §4.12's `…or "present"!` | §4.12 + `.` → ends `!.` |
    | §4.7's YouTube text, ending `username."` | ends `.".` |
    | §4.11's color text, ending `50%)"` | ends `)".` |

### 3.7 Unpacking a wrapped entry failure

21. A failure whose kind is `rendercv_entry_validation_error`
    (`models/custom_error_types.py:5`) carries its child failures in
    `ctx["caused_by"]` (`:153-165`). The wrapper's **own** record is kept (`:149-151`) **and**
    each child becomes its own record, appended immediately after it, in the child list's order.
22. Each child's location is rebuilt as: **drop the child's first element** — the literal
    `entries`, which exists only because `section.py:219-222` validates the shape
    `{"entries": [...]}` — then **prepend the wrapper's raw location** (`:159-160`). Measured:
    child `("entries", 1, "institution")` under wrapper `("cv", "sections",
    "welcome_to_rendercv_tests_2")` becomes `["cv", "sections", "welcome_to_rendercv_tests_2",
    "1", "institution"]` (`expected_errors.yaml:56-57`).
23. The prepended part is the wrapper's **raw** location, before §3.3's filtering; the whole
    spliced location is then filtered as one, in the child's own pass through §3.2.
24. A child whose location is empty splices to the wrapper's location plus nothing, so its record
    lands one level shallower. Measured: a `PublicationEntry` whose generated DOI URL exceeds the
    length limit reports at `("cv", "sections", "s", "0")` — the entry, not a field
    (`expected_errors.yaml:123` shows the same shape for the start-after-end rule).
25. A `rendercv_entry_validation_error` **without** `ctx`, or with a `ctx` lacking `caused_by`,
    is an internal failure with the message of §4.16 (`:153-157`), pinned by
    `tests/schema/test_pydantic_error_handling.py:190-206`.
26. Nested wrappers do not occur: only `section.py` raises this kind, and it is never raised from
    inside a `caused_by` child. The unpacking is one level deep and the port must not recurse.

### 3.8 Deduplication

27. After every record is built, records are filtered by **schema location** (`:167-176`): a set
    of locations is accumulated in order, and a record whose location is already present is
    dropped. **First occurrence wins**; relative order of survivors is unchanged.
28. Deduplication is on the location **only** — not on the message, not on the kind. Two
    genuinely different failures at one location collapse to the first. Measured, all three:

    | Input | Locations before dedup | Records after |
    |---|---|---|
    | `end_date: invalid_date` | `("end_date",)` twice | 1 (the forced message of §4.12) |
    | `settings: {current_date: todady}` | `("settings","current_date")` twice | 1 (§4.13) |
    | `photo: photo_doesnt_exist.jpg` | `("cv","photo")` twice — the path branch, then a URL parse failure | 1 (§4.14; `expected_errors.yaml:14-18`) |

29. The `photo` case is the load-bearing one: the second record would say §4.9's URL message, and
    dedup is the only thing that suppresses it. `cv.photo` is declared
    `ExistingPathRelativeToInput | pydantic.HttpUrl` with left-to-right union resolution
    (`models/cv/cv.py:52-57`), so both branches always fail together.

### 3.9 Order

30. **Records are never sorted** (`:145-176` contains no sort). The order is the raw failure
    order, which is **model field declaration order**, not document order.
31. Measured, and this is the evidence: `expected_errors.yaml` runs `email`, `photo`, `phone`,
    `website`, `social_networks`, `sections`, matching `Cv`'s declaration order
    (`models/cv/cv.py:44`, `:52`, `:58`, `:75`, `:83`, `:103`), while the document writes them as
    `website`, `phone`, `email`, `photo`
    (`tests/schema/testdata/test_pydantic_error_handling/wrong_input.yaml:4-9`).
32. The full ordering rule, measured:

    1. Top-level keys in `RenderCVModel` declaration order: `cv`, `design`, `locale`, `settings`
       (`models/rendercv_model.py`; verified at runtime as
       `['cv', 'design', 'locale', 'settings']`, and visible at `expected_errors.yaml:44`,
       `:141`, `:147`).
    2. Within a model, **declared-field** failures in declaration order.
    3. **Then** extra-key failures, in **input** order. Measured with a `cv` block writing
       `zzz_extra`, `phone`, `aaa_extra`, `email`: the emitted order is `email`, `phone`,
       `social_networks.0.network`, `social_networks.0.username`,
       `social_networks.0.extra_here`, `cv.zzz_extra`, `cv.aaa_extra`. This resolves the
       `TODO(iteration-4)` at `internal/schema/binder/binder.go:177`.
    4. Within a sequence, by ascending index.
    5. A nested model's failures appear at the position of the field that holds it, fully
       expanded, before the next sibling field's.
    6. A `caused_by` child appears immediately after its wrapper (§3.7 behavior 21), in child
       order — so a section's own record precedes its entries' records.
33. `sections` is a mapping and its failures follow **document** order within the field, because
    a mapping has no declaration order (`expected_errors.yaml:44-139` follows
    `wrong_input.yaml:17-56`).

### 3.9a Order *inside* one entry — an iteration-3 defect this iteration must fix first

This subsection exists because iteration 3 shipped the order backwards
(`specs/STATE.md` → *Cut scope* → *Iteration 3*, item 1). It is not new upstream behavior; it is
behavior 32 part 2 applied to the eight entry types, and the port currently violates it.

33a. **An entry's failures come in its own declared field order**, which is the order spec 003 §3.2
    fixed: own fields first, then the base's, because upstream declares
    `class X(BaseWithDates, BaseX)` and pydantic emits the last-listed base's fields first
    (`models/cv/entries/education.py:25-26`). Measured, four cases:

    | Input | Upstream order |
    |---|---|
    | `ExperienceEntry{company, summary: {a: 1}}` | `position` missing, then `summary` string_type |
    | `ExperienceEntry{position, highlights: "x"}` | `company` missing, then `highlights` list_type |
    | `ExperienceEntry{company, date: 2020-13-01, location: {a: 1}}` | `position`, then `date`, then `location` |
    | `PublicationEntry{title, authors, doi: bad, journal: {a: 1}}` | `doi`, then `journal` |

    All four are the declared orders of spec 003 §3.8 and §3.10 —
    `company, position, date, start_date, end_date, location, summary, highlights` and
    `title, authors, summary, doi, url, journal, date`.
33b. **A date-field failure is emitted at the date field's declared position, not appended.** Row 3
    is the load-bearing one: `date` sits between `position` and `location` in
    `ExperienceEntry`, and its failure appears there.
33c. **Order here is a correctness prerequisite for §3.8, not a cosmetic property.** Deduplication
    keeps the **first** record at a location (§3.8 behavior 27), so a wrongly-ordered list makes
    dedup keep the wrong row. Every mechanism downstream of the raw list assumes the list is
    already in upstream's order.

### 3.9b A non-scalar `date`, `start_date` or `end_date` — the worked example for §3.3 and §3.8

Iteration 3 produced **no** error at all for these inputs
(`specs/STATE.md` → *Cut scope* → *Iteration 3*, item 2). Upstream produces two or three, which
§3.3's filter and §3.8's dedup then reduce to exactly one. The pair of mechanisms is what makes the
observable result correct, which is why neither may be simplified away.

33d. `date` is declared `int | str` (`models/cv/entries/bases/entry_with_date.py:35`) and
    `start_date` is declared `str | int`
    (`models/cv/entries/bases/entry_with_complex_fields.py:40`). **The union arms are in the
    opposite order**, and it is observable. Measured raw failures for a mapping value:

    | Field | Raw failures, in order |
    |---|---|
    | `date` | `("date", "int")` int_type, then `("date", "str")` string_type |
    | `start_date` | `("start_date", "str")` string_type, then `("start_date", "int")` int_type |
    | `end_date` | `("end_date", "function-after[…]", "str")` string_type, then `(…, "int")` int_type, then `("end_date", "literal['present']")` literal_error |

33e. §3.3's filter drops `int`, `str`, `function-` and `literal` elements, so every row of
    behavior 33d collapses onto the bare field location. §3.8's dedup then keeps the first. The
    single surviving record, **measured end to end**:

    | Input | Records | Surviving message |
    |---|---|---|
    | `date: {a: 1}` | 1 | `Input should be a valid integer.` |
    | `date: [1]` | 1 | `Input should be a valid integer.` |
    | `start_date: {a: 1}` | 1 | `Input should be a valid string.` |
    | `end_date: {a: 1}` | 1 | §4.12, because §3.5's override fires regardless of the message |

33f. The `date` and `start_date` rows differ **only** in which message survives, and the difference
    comes entirely from the declared union order of behavior 33d. A port that emits one record per
    field with a hand-chosen message will get one of the two wrong; a port that emits the branch
    pair in the declared order and lets §3.3 and §3.8 run gets both right for free. This is the
    argument for implementing both mechanisms faithfully rather than short-cutting to "one error
    per field".
33g. `date: true` is **accepted** and **coerced to `1`**; `date: false` becomes `0` (measured). A
    bool satisfies the `int` arm and pydantic's lax mode converts it. The stored value is an
    integer, not a bool, which matters downstream: iteration 9 renders `date` and would print
    `1`, not `true`. Iteration 5 also sees `int | str`, never `bool`.

### 3.9c Error codes, corrected

33h. Codes are the pipeline's dispatch key — §3.7 unpacks on one of them and §3.10 truncates on
    another — so they are load-bearing here even though they are not user-visible. Iteration 3
    shipped three wrong and fixed them in `9ddd896`; **any code asserted in the existing Go tests
    is suspect until measured.** The measured values this iteration depends on:

    | Failure | Code |
    |---|---|
    | The entry-problems section wrapper (`models/cv/section.py:230`) | `rendercv_entry_validation_error` |
    | Every other section failure (`section.py:158`, `:169`, `:214`, `:240`) | `rendercv_other_error` |
    | `start_date` / `end_date` malformed (`entry_with_complex_fields.py:31-36`) | `rendercv_other_error` |
    | Start after end (`entry_with_complex_fields.py:161-169`) | `rendercv_other_error` |
    | The arbitrary `date` out of range (`entry_with_date.py:26-29`, a bare `ValueError`) | `value_error` |
    | A required key absent | `missing` |

33i. Only the wrapper's code triggers §3.7's unpacking, and it is the **one** section failure raised
    with a different type than the other four. Only the literal code `missing` triggers §3.10's
    path truncation (§3.10 behavior 36).
33j. **The code does not tell you whether a message carries a §3.2 prefix.** §3.2 behaviors 4a
    and 4b hold the rule and the measurements. The arbitrary-date row of behavior 33h is the one
    `value_error` in the tree whose message *is* wrapped, and it is wrapped because
    `Date.fromisoformat`'s exception escapes uncaught, not because of its code. A porter who infers
    "code `value_error` ⇒ strip" would double-strip the email and phone messages, both of which are
    `value_error` and neither of which is wrapped.

### 3.10 Coordinate resolution

34. `yaml_location` is resolved by walking the coordinate document one location element at a time
    (`:222-257`, per-step at `:179-219`). Spec 002 §6.7 already pinned the two coordinate
    formulas and iteration 2 implemented them; nothing here changes them.
35. **The path used for the walk is `location[:-1]` when the failure kind is exactly `missing`,
    and `location` otherwise** (`:106-108`). A missing key has no node of its own, so the
    coordinates point at the parent container. Measured: `expected_errors.yaml:56-61` and `:63-67`
    both report `[[23, 7], [23, 8]]` — the second entry's mapping — for the two missing
    `EducationEntry` fields.
36. The comparison is against the literal `"missing"`, not a class of absence failures. No other
    kind gets the truncation.
37. Coordinates are resolved against the document chosen in §3.12, which is not always the main
    document.
38. A walk that runs off the end of a sequence, or names a key the coordinate document does not
    have, is an internal failure with the messages of §4.17 and §4.18 (`:206-208`, `:211-213`),
    pinned by `tests/schema/test_pydantic_error_handling.py:233-246`.

### 3.11 Rendering the input value

39. `input` is the offending value converted to text, except that a **mapping or a sequence**
    renders as the literal three-dot string of §4.15 (`:122-126`). Measured: a missing field's
    input is the whole enclosing mapping, so it renders `...`
    (`expected_errors.yaml:59`, `:65`, `:77`); a null value renders `None`; the integer `5`
    renders `5`. When step 3 replaced `input` from the context, it is the replacement that is
    rendered (`:58-59`) — which is how §4.5 shows the theme name rather than the whole `design`
    mapping (`expected_errors.yaml:143`).

### 3.12 Choosing the source document

40. `yaml_source` starts as `main_yaml_file` and the coordinate document starts as the main
    document (`:99-100`). If overlay documents were supplied **and** `location` is non-empty
    **and** `location[0]` is one of the overlay keys, both are replaced: the source becomes the
    overlay's source literal and coordinates are resolved against the overlay document
    (`:101-104`, mapping at `exception.py:13-17`). Pinned by
    `tests/schema/test_pydantic_error_handling.py:96-186`, including a mixed case that asserts
    per-record by `schema_location[0]` (`:151-186`). Iteration 2 already ported the mapping and
    the overlay merge.

### 3.13 `pydantic.HttpUrl`, at four sites

41. Four fields are declared bare `pydantic.HttpUrl` with no RenderCV override, and all four
    behave identically: `cv.website` (`models/cv/cv.py:75`, list form through the adapter at
    `:20-22`), `CustomConnection.url` (`models/cv/custom_connection.py:9`),
    `PublicationEntry.url` (`models/cv/entries/publication.py:9`, `:36`), and the URL
    `SocialNetwork` generates from its network and username
    (`models/cv/social_network.py:12`, `:164`).
41a. A **fifth** site reaches the same type as the right arm of a union: `cv.photo` is
    `ExistingPathRelativeToInput | pydantic.HttpUrl` (`models/cv/cv.py:52-57`). It is listed
    separately because its URL record is **never observable** — §3.8 behavior 29's dedup always
    keeps the path record — so the port emits no URL record there at all. Behavior 41's four sites
    are the ones whose URL behavior is observable.

42. **Normalization**, measured, in full:

    | Input | Result |
    |---|---|
    | `https://example.com` | `https://example.com/` |
    | `HTTPS://Example.COM/Path` | `https://example.com/Path` |
    | `https://example.com:443/a/b?x=1#frag` | `https://example.com/a/b?x=1#frag` |
    | `http://example.com:80` | `http://example.com/` |
    | `https://user:pw@ex.com/p` | `https://user:pw@ex.com/p` |
    | `https://example.com/a%20b` | `https://example.com/a%20b` |
    | `https://xn--80ak6aa92e.com` | `https://xn--80ak6aa92e.com/` |
    | `https://ünicode.de/ünï` | `https://xn--nicode-2ya.de/%C3%BCn%C3%AF` |
    | `https://[::1]:8080/x` | `https://[::1]:8080/x` |
    | `https://example.com?` | `https://example.com/?` |
    | `https://example.com#` | `https://example.com/#` |

    In words: scheme and host are lowercased, an international host is punycoded, a default port
    for the scheme is dropped, an empty path becomes `/`, non-ASCII path bytes are percent-encoded
    as UTF-8, a present-but-empty query or fragment is preserved as a bare `?` or `#`, and case
    outside scheme and host is preserved. This is the WHATWG URL Standard's serialization, which
    is what pydantic-core implements.
43. **The normalization is visible in golden output.** `scripts/ats_proof/corpus/baseline/standard_full.yaml:9`
    writes `website: https://alicechen.dev` and
    `testdata/golden/ats_standard_full/files/rendercv_output/Alice_Chen_CV.typ` links
    `https://alicechen.dev/`; `scripts/ats_proof/corpus/stress_tests/academic.yaml:9` and
    `testdata/golden/ats_academic/…` do the same for `https://zhenwei.ch`. Axis 1 gates the
    trailing slash. It does **not** gate anything else in behavior 42's table.
44. **The `SocialNetwork` URL is validated but not normalized.** `validate_generated_url`
    discards the adapter's return value (`models/cv/social_network.py:153-165`), so the raw
    concatenation of prefix and username is what renders. Measured: a LinkedIn username of
    `not a valid %%^&*()` validates with **no** error — the generated URL parses — and the spaces
    survive into output. `wrong_input.yaml:11-12` writes exactly that username and
    `expected_errors.yaml` has no record for `cv.social_networks.0`.
45. **Three distinct failure kinds**, measured:

    | Kind | Raw message | Final |
    |---|---|---|
    | `url_parsing` | `Input should be a valid URL, ` + a varying reason | §4.9 — the dictionary flattens every reason |
    | `url_scheme` | §4.19 | §4.19 + `.` — no dictionary row |
    | `url_too_long` | §4.20 | §4.20 + `.` — no dictionary row |

    Measured reason clauses for `url_parsing`, all of which become §4.9: `relative URL without a
    base` (for `example.com`, `not a url`, `//example.com`), `empty host` (for `https://`),
    `invalid international domain name` (for `https://exa mple.com`). **The reason text is
    therefore unobservable** and the port need not reproduce it.
46. **The length limit is checked on the input string, before parsing, at 2083 **UTF-8 bytes**
    inclusive.** *(Corrected: this behavior previously read "characters". The check lives in
    pydantic-core, which is Rust and counts bytes; Python's `len()` on a `str` counts characters,
    which is where the wrong reading came from. The two coincide for ASCII, so every measurement
    below is unaffected. Measured by bisection on a non-ASCII path: 1051 characters — 2082 bytes —
    pass, and 1052 — 2084 bytes — fail. A port using a rune count would accept URLs upstream
    rejects.)* Measured: a 2083-character input passes; 2084 fails; a 420-character input whose
    serialized form is 2420 characters passes; and `https://exa mple.com/` + 3000 characters
    reports `url_too_long`, not `url_parsing`, proving the length check runs first.

### 3.14 `phone`

47. `cv.phone` accepts one value or a list (`models/cv/cv.py:58-74`, routed by the shared
    scalar-or-list validator at `:177-229` that spec 002 §3.47 pinned). Each element is validated
    by a vendored dependency, `pydantic_extra_types.phone_numbers.PhoneNumber`, which parses with
    libphonenumber, requires a valid number, and **stores the RFC 3966 form**
    (`models/cv/cv.py:5`, `:23-25`). RenderCV then strips the `tel:` prefix at serialization time
    (`:231-250`).
48. **The stored value is re-grouped, not passed through.** Measured:

    | Input | Stored | Serialized |
    |---|---|---|
    | `+905419999999` | `tel:+90-541-999-99-99` | `+90-541-999-99-99` |
    | `+34-612-345-678` | `tel:+34-612-34-56-78` | `+34-612-34-56-78` |
    | `+1-415-555-0142` | `tel:+1-415-555-0142` | `+1-415-555-0142` |
    | `+44 20 1234 5678` | `tel:+44-20-1234-5678` | `+44-20-1234-5678` |
    | `+493012345678` | `tel:+49-30-12345678` | `+49-30-12345678` |

    The second row is decisive: input grouping `612-345-678` becomes `612-34-56-78`. A `tel:`
    strip alone — which is all iteration 2 implemented (spec 002 cut-scope item 3) — produces the
    input grouping and is wrong.
49. **This is Axis-1 gated, in two golden cases.**
    `testdata/golden/ats_diacritics/files/rendercv_output/Jose_Garcia-Lopez_CV.typ:96` contains
    `tel:+34-612-34-56-78` from `scripts/ats_proof/corpus/edge_cases/diacritics.yaml:9`, and
    `testdata/golden/ats_standard_full/…:94` contains `tel:+1-415-555-0142` from
    `scripts/ats_proof/corpus/baseline/standard_full.yaml:8`. The same two lines also contain the
    **national** display forms `612 34 56 78` and `(415) 555-0142`, which are
    `design.header.connections.phone_number_format`'s and belong to iteration 9 — recorded here
    because they come from the same library.
50. Failure is one kind: raw message `value is not a valid phone number`, dictionary row 7, final
    §4.8. Measured for `not_a_valid_phone_number` (`expected_errors.yaml:20-24`).
51. **A list-valued `phone` crashes upstream at serialization time.** `serialize_phone` is typed
    for a single value and calls `.replace` on it (`models/cv/cv.py:231-250`); given a list this
    raises `AttributeError: 'list' object has no attribute 'replace'` wrapped in a
    `PydanticSerializationError`. Measured. Validation succeeds; only serialization fails. It is
    not a validation error and §7 assigns the decision to iteration 12.

### 3.15 `email`

52. `cv.email` accepts one value or a list (`models/cv/cv.py:44`, adapters at `:15-18`), routed
    the same way as `phone`. Each element is validated by pydantic's `validate_email`, which
    wraps a vendored dependency, the `email-validator` library. Neither the wrapper's nor the
    library's text is RenderCV's, and **no dictionary row applies** — step 1 strips the
    `value is not a valid email address: ` prefix and the library's own reason passes through
    verbatim, already period-terminated (`:23`, `:50-51`).
53. The wrapper contributes three behaviors before the library sees the value, measured: an input
    longer than 2048 characters fails with §4.21; a `Name <local@domain>` form is unwrapped and
    only the address part validated; and surrounding whitespace is stripped with no error.
54. **Normalization**: the domain is lowercased, the local part is not, and a non-ASCII domain is
    left in its Unicode form. Measured: `JOHN.DOE@Example.COM` → `JOHN.DOE@example.com`;
    `a@ünicode.de` → `a@ünicode.de`; `düsseldorf@example.com` unchanged.
55. **The reachable message set, measured.** These are the required set for this iteration; each
    is verbatim in §4.22. `An email address must have an @-sign.` is the one Axis 4 pins
    (`expected_errors.yaml:3`, `:9`).

    | Input | Message |
    |---|---|
    | `not_a_valid_email`, `` (empty) | `An email address must have an @-sign.` |
    | `a@` | `There must be something after the @-sign.` |
    | `@b.com` | `There must be something before the @-sign.` |
    | `a@b` | `The part after the @-sign is not valid. It should have a period.` |
    | `a b@c.com` | `The email address contains invalid characters before the @-sign: SPACE.` |
    | `a@@b.com` | `The part after the @-sign contains invalid characters: '@'.` |
    | `a@b..com` | `An email address cannot have two periods in a row.` |
    | `.a@b.com` | `An email address cannot start with a period.` |
    | `a.@b.com` | `An email address cannot have a period immediately before the @-sign.` |
    | `a@-b.com` | `An email address cannot have a hyphen immediately after the @-sign.` |
    | `a@[1.2.3.4]` | `A bracketed IP address after the @-sign is not allowed here.` |
    | 300 `a`s + `@b.com` | `The email address is too long (52 characters too many).` |

56. Accepted, measured, and required: `a@b.c`, `john.doe+tag@example.co.uk`, and the two
    normalization rows of behavior 54.
57. The library's message catalogue is larger than behavior 55 and this spec does not enumerate
    all of it. §7.4 states the scope decision and the residual risk.

### 3.16 Per-network username rules

58. `SocialNetwork.check_username` (`models/cv/social_network.py:59-151`) runs **only** when a
    valid `network` is already present; when `network` is absent or outside the seventeen names it
    returns the username unchecked and lets the `network` failure stand alone (`:76-80`).
59. Eight of the seventeen networks have a rule; the other nine accept any username. All eight
    raise the RenderCV custom kind `rendercv_other_error`
    (`models/custom_error_types.py:6`) at location `username`, and none is touched by the
    dictionary. Measured for each:

    | Network | Rule | Message |
    |---|---|---|
    | `Mastodon` | full match of `@[^@]+@[^@]+` (`:86-87`) | §4.1 |
    | `StackOverflow` | full match of `\d+\/[^\/]+` (`:93-94`) | §4.2 |
    | `YouTube` | must not start with `@` (`:101`) | §4.3 |
    | `ORCID` | full match of `\d{4}-\d{4}-\d{4}-\d{3}[\dX]` (`:108-109`) | §4.4 |
    | `IMDB` | full match of `nm\d{7}` (`:115-116`) | §4.5 |
    | `Bluesky` | full match of the anchored handle pattern at `:122` | §4.6 |
    | `WhatsApp` | must validate as a phone number (`:129-140`) | §4.7 |
    | `Reddit` | full match of `^[a-zA-Z0-9_-]{3,23}$` (`:142-143`) | §4.24 |

60. Every pattern is applied with **full-match** semantics — `re.fullmatch`, not `search`. The
    `Bluesky` and `Reddit` patterns additionally carry redundant `^`/`$` anchors (`:122`, `:142`).
61. **§4.3's message ends with a stray `"` after its final period**, verbatim as written at
    `:104-105`. §3.6 then appends another period, so the final text ends `username.".`. Measured.
    Like §3.5's `!.`, this is not the port's to fix.
62. The `network` value itself is a literal union of seventeen names (`:13-31`). An unknown value
    fails with pydantic's `literal_error` and the enumerating message of §4.23, which no
    dictionary row matches, so §3.6 appends a period. Measured. Iteration 2 shipped the
    placeholder text `Input should be one of the supported social networks`
    (`internal/schema/models/cv/socialnetwork.go:123`), which this iteration replaces.

### 3.17 Path, design and locale strings

63. `resolve_relative_path` raises §4.25 when a required path does not exist and §4.26 when it
    exists but is not a file (`models/path.py:43-55`). Both interpolate the path **relative to
    the resolution base**, not the absolute path (`:48`, `:54`). Measured at
    `expected_errors.yaml:15`: `The file \`photo_doesnt_exist.jpg\` does not exist.`
64. `validate_design` raises three shape messages before it would load any theme code: §4.27 for
    a theme name outside `^[a-z0-9]+$` (`models/design/design.py:60-70`), §4.28 for a missing
    theme folder (`:74-80`), and §4.29 for a folder with no `*.j2.typ` file (`:82-88`).
65. **§4.27's failure re-pins its own location and input** through the context keys `loc` and
    `input` (`design.py:67-68`), which §3.2 step 3 applies. Its raw location is `("design",)`;
    the final location is `("design", "theme")`. Measured, and pinned at
    `expected_errors.yaml:141-145`. §4.28 and §4.29 set no such keys and report at `("design",)`
    — visible in `testdata/golden/err_unknown_theme/stdout.txt`, whose Location column reads
    `design`.
66. A `design` block whose theme is built-in but whose keys are wrong reports through ordinary
    field validation with the discriminator element dropped (§3.3 behavior 9). Measured:
    `{theme: classic, nope: 1}` → `("design", "nope")` with §4.10;
    `{theme: classic, page: {top_margin: notadim}}` → `("design", "page", "top_margin")` with the
    dimension message, which is iteration 6's string.
67. `Locale` is a discriminated union on `language` (`models/locale/locale.py:38-41`). The locale
    package raises **no** custom failures — it contains no field or model validator — so locale
    failures are plain pydantic messages through the ordinary path, with the discriminator element
    dropped. Measured: `{language: english, month: 123}` → `("locale", "month")` with
    `Input should be a valid string.`; `{language: klingon}` → `("locale",)` with §4.30.

### 3.18 The YAML-syntax producer

68. A parser failure produces exactly one record (`rendercv_model_builder.py:84-101`) with: no
    schema location; coordinates from the parser's own marks, 1-indexed in both line and column
    (`:42-62`); the source of the document being parsed; the input echo of §4.15; and the message
    of §4.31, whose interpolation is the **first line** of the parser's exception text, stripped,
    with a period appended when absent (`:87-89`).
69. Only `context_mark` and `problem_mark` are read, `context_mark` preferred for the start and
    `problem_mark` for the end; when both are absent, coordinates are absent (`:51-57`).
70. Measured through the CLI: `testdata/golden/err_not_yaml/stdout.txt` shows
    `This is not a valid YAML file. while parsing a flow sequence.` at
    `main_yaml_file: line 1 to line 2`.

### 3.19 Placeholder strings iterations 2 and 3 left behind

71. Four strings currently in the Go tree are placeholders, not upstream text, and this iteration
    replaces each with the measured value:

    | Site | Current | Required |
    |---|---|---|
    | `internal/schema/binder/binder.go:45` (`messageModelType`) | `Input should be a valid dictionary` | §4.32 — the model name is part of the text |
    | `internal/schema/models/cv/socialnetwork.go:123` | `Input should be one of the supported social networks` | §4.23 |
    | `internal/schema/models/cv/entries/bases/entrywithcomplexfields.go:100` | routed through the not-a-valid-date path | §4.33 |
    | `internal/schema/modelbuilder/yamlerror.go:53` | the Go YAML library's own first line | §7.5 — a decision, not a fix |

72. §4.32 is model-specific: measured, `cv: null` and `cv: 5` both give
    `Input should be a valid dictionary or instance of Cv.` The suffix is the model's class name,
    so every model that can appear as a mapping value needs its own. The names this iteration must
    emit are `Cv`, `SocialNetwork`, `CustomConnection`, and the eight entry class names of
    spec 003 §3.1.

### 3.20 Presentation, recorded not implemented

73. `print_validation_errors` (`cli/render_command/progress_panel.py:137-169`) renders the
    records as a bordered table with three columns, exactly `Location`, `Input Value`,
    `Explanation`, the first two with wrapping disabled, one row per record in
    `parse_validation_errors` order, wrapped in a panel titled
    `There are validation errors!`, then exits with code **1** (`:148-151`, `:160-167`, `:169`).
74. `format_validation_error_location` (`:14-36`) returns the location elements joined with `.`
    whenever the schema location is present, which per §3.1 is **always** for a schema failure.
    The YAML coordinates are consulted **only** for a syntax failure, and even then only the line
    numbers: both branches destructure as `(start_line, _), (end_line, _)` and discard the
    columns unconditionally (`:33`).
75. **Columns are therefore never user-visible, and there is no machine-readable error mode.**
    `src/rendercv/cli/` contains no `--json` or `--format` flag; its only JSON is the version-check
    cache (`cli/app.py:60-113`). This resolves spec 002's cut-scope item 1 as *not a user-visible
    bug*; §7.2 states what follows for the port.
76. **The panel goes to stdout, stderr stays empty, and the exit code is 1.** Measured, and pinned
    by our own fixture: `testdata/golden/err_wrong_input/` has 9257 bytes of stdout and 0 of
    stderr. The current Go side does the reverse. This is an iteration-12 defect with a golden
    already pinning the correct behavior (§7.6).
77. The box-drawing characters are **not** downgraded when output is piped, and with no terminal
    and no width hint the layout is **80 columns**. At 80 columns the `err_wrong_input` table
    squeezes `Explanation` to nothing — its header text is absent from the golden — and truncates
    long locations with a mid-string ellipsis, e.g.
    `cv.sections.welcome_to_rendercv_tests_…`. **The CLI goldens therefore pin the table layout
    algorithm, not the message text**, and Axis 4 must be verified through
    `expected_errors.yaml`. §9 restates this.

---

## 4. Exact user-visible strings

Verbatim. `{...}` marks an interpolation. Nothing here may be reflowed, re-punctuated or
"corrected".

### 4.1 Mastodon username — `models/cv/social_network.py:90`

```
Mastodon username should be in the format "@username@domain".
```

### 4.2 StackOverflow username — `models/cv/social_network.py:97-98`

```
StackOverflow username should be in the format "user_id/username".
```

### 4.3 YouTube username — `models/cv/social_network.py:104-105`

Note the trailing `"` after the final period. It is in the source and it is not a transcription
error here.

```
YouTube username should not start with "@". Remove "@" from the beginning of the username."
```

After §3.6 the emitted text is:

```
YouTube username should not start with "@". Remove "@" from the beginning of the username.".
```

### 4.4 ORCID username — `models/cv/social_network.py:112`

```
ORCID username should be in the format 'XXXX-XXXX-XXXX-XXX'.
```

### 4.5 IMDB username — `models/cv/social_network.py:119`

```
IMDB name should be in the format 'nmXXXXXXX'.
```

### 4.6 Bluesky username — `models/cv/social_network.py:126-127`

```
Bluesky username should be a valid handle with no '@' (e.g., 'username.bsky.social' or 'domain.com').
```

### 4.7 WhatsApp username — `models/cv/social_network.py:138-139`

```
WhatsApp username should be your phone number with country code in international format (e.g., +1 for USA, +44 for UK).
```

### 4.8 Invalid phone number — `error_dictionary.yaml:8`

```
This is not a valid phone number.
```

### 4.9 Invalid URL — `error_dictionary.yaml:6`

```
This is not a valid URL.
```

### 4.10 Unknown key — `error_dictionary.yaml:12`

```
This field is unknown for this object. Please remove it.
```

### 4.11 Invalid color — `error_dictionary.yaml:14`

The dictionary value, which ends without a period:

```
This is not a valid color. Here are some examples of valid colors: "red", "#ff0000", "rgb(255, 0, 0)", "hsl(0, 100%, 50%)"
```

After §3.6 the emitted text is:

```
This is not a valid color. Here are some examples of valid colors: "red", "#ff0000", "rgb(255, 0, 0)", "hsl(0, 100%, 50%)".
```

### 4.12 Invalid `end_date` — `pydantic_error_handling.py:72-75`

The literal, which ends in `!`:

```
This is not a valid `end_date`! Please use either YYYY-MM-DD, YYYY-MM, or YYYY format or "present"!
```

After §3.6 the emitted text ends in `!.`:

```
This is not a valid `end_date`! Please use either YYYY-MM-DD, YYYY-MM, or YYYY format or "present"!.
```

Pinned by `tests/schema/test_pydantic_error_handling.py:56-93`, which asserts the substrings
`YYYY-MM-DD, YYYY-MM` and `or "present"`.

### 4.13 Invalid `current_date` — `pydantic_error_handling.py:84-87`

Already period-terminated, so §3.6 changes nothing:

```
This is not a valid `current_date`! Please use YYYY-MM-DD format or "today".
```

Pinned at `tests/schema/testdata/test_pydantic_error_handling/expected_errors.yaml:148`.

### 4.14 Required field is absent — `error_dictionary.yaml:7`

```
This field is required.
```

### 4.15 The input echo for a mapping or a sequence — `pydantic_error_handling.py:126`

```
...
```

### 4.16 A wrapped entry failure lost its children — `pydantic_error_handling.py:156`

Internal, not a validation record.

```
entry_validation error missing ctx or caused_by
```

### 4.17 Sequence index out of range — `pydantic_error_handling.py:207`

Internal, not a validation record.

```
Index {index} is out of range in the YAML file.
```

### 4.18 Key not found — `pydantic_error_handling.py:212`

Internal, not a validation record.

```
Key '{location_key}' not found in the YAML file.
```

### 4.19 Wrong URL scheme

Pydantic's `url_scheme` text; no dictionary row, so §3.6 appends a period. Measured for
`ftp://example.com`.

```
URL scheme should be 'http' or 'https'
```

### 4.20 URL too long

Pydantic's `url_too_long` text; no dictionary row. Also reached through the generated DOI URL
(spec 003 §4.2).

```
URL should have at most 2083 characters
```

### 4.21 Email too long before validation

Pydantic's own pre-check, measured for a 2049-character input.

```
Length must not exceed 2048 characters
```

### 4.22 Email syntax messages

The `email-validator` library's own text, passed through verbatim after §3.2 step 1 strips the
prefix. Each is already period-terminated. The triggering inputs are §3.15 behavior 55's table.

```
An email address must have an @-sign.
There must be something after the @-sign.
There must be something before the @-sign.
The part after the @-sign is not valid. It should have a period.
The email address contains invalid characters before the @-sign: SPACE.
The part after the @-sign contains invalid characters: '@'.
An email address cannot have two periods in a row.
An email address cannot start with a period.
An email address cannot have a period immediately before the @-sign.
An email address cannot have a hyphen immediately after the @-sign.
A bracketed IP address after the @-sign is not allowed here.
The email address is too long ({n} characters too many).
```

### 4.23 Unknown social network — `models/cv/social_network.py:13-31`

Pydantic's `literal_error` enumeration, in the literal type's declared order, with `or` before
the last. No dictionary row, so §3.6 appends a period.

```
Input should be 'LinkedIn', 'GitHub', 'GitLab', 'IMDB', 'Instagram', 'ORCID', 'Mastodon', 'StackOverflow', 'ResearchGate', 'YouTube', 'Google Scholar', 'Telegram', 'WhatsApp', 'Leetcode', 'X', 'Bluesky' or 'Reddit'
```

### 4.24 Reddit username — `models/cv/social_network.py:146-148`

```
Reddit username should be made up of uppercase/lowercase letters, numbers, underscores, and hyphens between 3 and 23 characters.
```

### 4.25 Required file does not exist — `models/path.py:47`

```
The file `{file_path}` does not exist.
```

### 4.26 Required path is not a file — `models/path.py:53`

```
The path `{path}` is not a file.
```

### 4.27 Custom theme name is not lowercase alphanumeric — `models/design/design.py:63-64`

```
The custom theme name should only contain lowercase letters and digits. The provided value is `{theme_name}`.
```

### 4.28 Custom theme folder missing — `models/design/design.py:77-78`

```
The custom theme folder `{custom_theme_folder}` does not exist. It should be in the same directory as the input file.
```

### 4.29 Custom theme folder has no template — `models/design/design.py:85-86`

```
The custom theme folder `{custom_theme_folder}` does not contain any *.j2.typ files. It should contain at least one *.j2.typ file.
```

### 4.30 Unknown locale language — `models/locale/locale.py:38-41`

Pydantic's `union_tag_invalid` text, enumerating the twenty-two discovered languages in sorted
filename order with `english` first. No dictionary row. Note the non-ASCII `norwegian_bokmål`.

```
Input tag 'klingon' found using 'language' does not match any of the expected tags: 'english', 'arabic', 'danish', 'dutch', 'french', 'german', 'hebrew', 'hindi', 'hungarian', 'indonesian', 'italian', 'japanese', 'korean', 'mandarin_chinese', 'norwegian_bokmål', 'norwegian_nynorsk', 'persian', 'portuguese', 'russian', 'spanish', 'turkish', 'vietnamese'
```

### 4.31 YAML syntax error — `rendercv_model_builder.py:97`

```
This is not a valid YAML file. {parser_message}
```

### 4.32 Value is not a mapping

Pydantic's `model_type` text. The suffix is the target model's class name (§3.19 behavior 72).

```
Input should be a valid dictionary or instance of {ModelName}
```

### 4.33 Year outside four digits

CPython's `date.fromisoformat` text, reached through an integer date year such as `10000`.
Measured.

```
Invalid isoformat string: '{value}'
```

### 4.34 Not-a-valid-date, for reference — `models/cv/entries/bases/entry_with_complex_fields.py:34-35`

RenderCV's own, already pinned as spec 002 §4.14. Restated here only to show it is
period-terminated and therefore untouched by §3.6, and that it is **not** the `end_date` literal
of §4.12.

```
This is not a valid date! Please use either YYYY-MM-DD, YYYY-MM, or YYYY format.
```

---

## 5. Edge cases

1. **A section key containing a filter substring loses its name.** §3.3 behavior 7's five-row
   table. Four such sections collapse to one record at `("cv", "sections")`.
2. **Two failures at one location become one, with the first message.** §3.8 behavior 28's
   three-row table.
3. **`cv.photo` always fails twice and the URL half is always suppressed.** §3.8 behavior 29.
   Pinned by `expected_errors.yaml:14-18`.
4. **A bad `end_date` reports `!.`** — §3.5 behavior 16, §4.12. Also pinned by
   `tests/schema/test_pydantic_error_handling.py:56-93`.
5. **A bad `current_date` reports a single period.** §3.5 behavior 19, §4.13,
   `expected_errors.yaml:147-151`.
6. **The `design.theme` failure reports at a location its raw form does not have.** §3.17
   behavior 65, `expected_errors.yaml:141-145`.
7. **`design.extra_field_in_design` produces no record at all.** `wrong_input.yaml:59` writes an
   unknown design key, and `expected_errors.yaml` has no record for it: the wrap validator raises
   §4.27 and never reaches per-field validation. Measured — the raw failure list contains one
   `design` entry, not two.
8. **A LinkedIn username of `not a valid %%^&*()` is valid.** §3.13 behavior 44.
   `wrong_input.yaml:11-12` → no record.
9. **A missing key's coordinates point at its parent.** §3.10 behavior 35. Two different missing
   fields of one entry report the same coordinates (`expected_errors.yaml:56-67`).
10. **`Input should be a valid string` has no dictionary row and still gets a period.**
    `expected_errors.yaml:112`.
11. **An arbitrary date given a mapping or a sequence reports an integer message.** Measured:
    `date: {a: 1}` and `date: [1]` on a `NormalEntry` both give `Input should be a valid
    integer.` — the two union branch locations `("date", "int")` and `("date", "str")` are both
    filtered away by §3.3, they dedup to one, and the surviving message matches no dictionary row
    (row 2's key is longer than the message, and containment runs the other way). `date: true`
    is **accepted**.
10a. **The email prefix reaches the pipeline; the `Value error, ` prefix does not.** §3.2
    behaviors 4a and 4b. `email: not_a_valid_email` produces a raw message of
    `value is not a valid email address: An email address must have an @-sign.` and a final message
    of `An email address must have an @-sign.` — so the strip is gated by `expected_errors.yaml:3`
    and `:9`. No message the port produces contains `Value error, `, because the port never wraps an
    escaping exception. Both halves are implemented; only one is exercised by production data.
11a. **A non-scalar `date` and a non-scalar `start_date` produce different surviving messages.**
    §3.9b behaviors 33d–33f. `date: {a: 1}` reports `Input should be a valid integer.` and
    `start_date: {a: 1}` reports `Input should be a valid string.`, from a two-record collapse in
    each case, because the two fields declare their union arms in opposite orders. This is the
    single best test that §3.3's filter and §3.8's dedup are both implemented: any port that
    short-cuts to one record per field gets one of the two wrong.
11b. **A non-scalar `end_date` produces three raw failures and one record.** §3.9b behavior 33d.
    All three collapse to `("end_date",)` and §3.5's override replaces the message with §4.12, so
    the union order is unobservable here.
11c. **Entry failures come in the entry's declared field order, with date failures interleaved.**
    §3.9a behavior 33a's four-row table. Iteration 3 emitted all four backwards
    (`specs/STATE.md` → *Cut scope* → *Iteration 3*, item 1) and §3.9a behavior 33c is why that is
    a correctness bug rather than a presentation one.
12. **`highlights` given a scalar reports the list message.** Measured: `highlights: nope` →
    §5's row for `Input should be a valid list` → `This field should contain a list of items but
    it doesn't.` Same as `expected_errors.yaml:135-139` for `authors`.
13. **The whole 25-record fixture.** `wrong_input.yaml` → `expected_errors.yaml`, compared
    field-by-field and in order with equal length asserted, is upstream's own test
    (`tests/schema/test_pydantic_error_handling.py:19-54`). It is this iteration's primary gate.
14. **Overlay source selection, three cases.**
    `tests/schema/test_pydantic_error_handling.py:96-127` asserts a `design` overlay failure
    reports `design_yaml_file` with non-absent coordinates; `:128-150` asserts every failure of a
    single-file run reports `main_yaml_file`; `:151-186` asserts a mixed run reports per record by
    the first location element.
15. **A wrapped entry failure with no `caused_by` is an internal failure.**
    `tests/schema/test_pydantic_error_handling.py:190-206`, §4.16.
16. **Coordinate-walk failures are internal failures.**
    `tests/schema/test_pydantic_error_handling.py:233-246` pins the substrings
    `Index 10 is out of range` and `Key 'nonexistent' not found`; §4.17, §4.18.
17. **A URL of exactly 2083 characters is accepted; 2084 is not; and the check precedes
    parsing.** §3.13 behavior 46.
18. **A `ftp://` URL fails on the scheme, not the parse.** §4.19. Measured — the underlying
    WHATWG parser accepts it, so the scheme restriction is a separate check.
19. **An empty raw location crashes upstream and must not crash the port.** §3.3 behavior 10.
    Measured: validating a bare `PublicationEntry` with an over-long generated DOI URL and passing
    the result through `parse_validation_errors` raises `IndexError`. Through `RenderCVModel` the
    same input reports cleanly at `("cv", "sections", "s", "0")`, because §3.7's splice supplies a
    prefix. The port treats an empty location as "not `design`, not `locale`, no special case" and
    proceeds.
20. **A list-valued `phone` crashes upstream at serialization.** §3.14 behavior 51.
21. **`+34-612-345-678` is re-grouped to `+34-612-34-56-78`.** §3.14 behavior 48, and it is the
    row that distinguishes a real implementation from a `tel:` strip.
22. **`https://ünicode.de/ünï` becomes `https://xn--nicode-2ya.de/%C3%BCn%C3%AF`.** §3.13
    behavior 42. Go's standard library performs neither the punycoding nor the path encoding.
23. **The `doi` pattern message keeps its single backslashes.** §3.4 behavior 13. Final text:
    `String should match pattern '\b10\..*'.` Spec 003 §4.1 pinned the pre-period form; this
    iteration pins the emitted form and records that the dictionary row for it is dead.
24. **`err_unknown_theme`'s golden contains an absolute filesystem path.**
    `testdata/golden/err_unknown_theme/stdout.txt` embeds
    `/home/nnc/Projects/rendercv-go/testdata/.work…` because §4.28 interpolates
    `custom_theme_folder.absolute()` (`models/design/design.py:79`). The golden is not portable
    across machines. Recorded as a corpus defect, assigned in §7.7.

---

## 6. Ordering and whitespace guarantees

1. **Record order is the raw failure order, unsorted.** §3.9 behaviors 30–33. Any sort, stable or
   not, is a defect.
2. **Declared-field failures precede extra-key failures within one model.** §3.9 behavior 32
   step 3. This is the answer to the open `TODO(iteration-4)` at
   `internal/schema/binder/binder.go:177`, and it is measured, not inferred.
3. **A wrapper's record precedes its children's, and its children keep their own order.** §3.7
   behavior 21.
3a. **Within one entry, failures follow the entry's declared field order, with date failures
   interleaved at their declared position.** §3.9a. This is the order iteration 3 got backwards,
   and §3.9a behavior 33c makes it a prerequisite of rule 4 rather than a sibling of it.
3b. **Within one field's union, branch failures follow the declared arm order** — `int` then `str`
   for `date`, `str` then `int` for `start_date`. §3.9b behavior 33d. Observable only through
   rule 4, which is the point.
4. **Deduplication is order-preserving and keeps the first.** §3.8 behavior 27.
5. **The dictionary is scanned in file order** and the first containment match wins. Two rows map
   to the same replacement (rows 9 and 10), so their relative order is unobservable; every other
   pair is disjoint on every measured message, so file order is currently unobservable too — but
   it is contractual, because a future upstream row could overlap.
6. **The strip of §3.2 step 1 removes every occurrence, not the first.** A message containing
   either prefix twice loses both copies. Per §3.2 behavior 4a only the email prefix reaches this
   rule from the port's own records; the `Value error, ` half is exercised only by synthetic input,
   and §8's criterion says that in as many words rather than implying coverage the port does not
   have.
7. **Messages carry exactly one trailing period, unless the pre-period text already ended in
   punctuation**, in which case the period is appended after it: `!.`, `.".`, `)".`. §3.6.
8. **Whitespace inside a message is never normalized.** The literals of §4 are emitted byte for
   byte, including the double spaces that do not occur and the single spaces that do.
9. **Coordinates are 1-indexed in both line and column, in both producers** (`:205`, `:217`;
   `rendercv_model_builder.py:60-61`).

---

## 7. Out of scope

| Deliberately excluded | Owned by |
|---|---|
| Emitting descriptions and examples into JSON Schema | iteration 5 |
| The `design` models themselves, and every message they raise (dimensions, page sizes, font names) — **except** the theme-name check of §4.27, which §7.9 pulls in | iteration 6 |
| §4.28 and §4.29 are *pinned* here but only reachable once custom themes exist; the two `__init__.py` messages (`models/design/design.py:108-113`, `:115-120`) and the missing-theme-class message (`:129-132`) are Lua-theme territory (D-002) | iteration 6 |
| The `locale` catalogs and the twenty-two language names of §4.30 as *data* | iteration 7 |
| The `settings` model — **except** the `current_date` shape check §7.9 pulls in | iteration 7 |
| The national and other phone display formats of §3.14 behavior 49 | iteration 9 |
| Rendering the table of §3.20, the stdout/stderr fix of behavior 76, and the exit code | iteration 12 |
| Deciding what a list-valued `phone` does, given upstream crashes (§3.14 behavior 51) | iteration 12 |
| Deciding what a non-mapping, non-string entry does, given upstream crashes (the `TODO(iteration-4)` at `internal/schema/models/cv/sectionvalidation.go:110`) | iteration 12 — see §7.8 |

Carried items from `specs/STATE.md` → *Cut scope → Iteration 2*, resolved here:

- **Item 1** (coordinate columns diverge in two shapes) — resolved as a scoping decision in §7.2,
  with work assigned.
- **Item 3** (`phone` formatting, and the spec 002 self-contradiction) — resolved in §3.14 and
  §7.1. **Spec 002 §3.49 listed phone formatting as an iteration-2 acceptance criterion while
  spec 002 §7 assigned it to iteration 4. §7 was right and §3.49's criterion was misplaced.**
  Phone formatting is this iteration's, it is Axis-1 gated, and §7.1 settles the dependency.
- **Item 4** (the `dealias` no-op regression test over the submodule corpus) — assigned here as a
  test-only task; nothing about it is validation-error work, but it is the last unguarded
  transform in the reader and this iteration is the one that starts reading
  `expected_errors.yaml` seriously.
- **Item 5** (`tools/yamlprobe` emits only five coordinate documents, so `resolve_test.go` states
  Go-side expectations) — assigned here, because §7.2's decision makes the probe's output the
  source of the coordinate fixture.

Carried items from `specs/STATE.md` → *Cut scope* → *Iteration 3*, resolved here:

- **Item 1 — entry error ordering.** The composed binder field order is `[base fields] ++ [own
  fields]`, the reverse of what upstream produces, and date failures are appended after all bind
  failures rather than emitted at their declared position. §3.9a is the normative behavior and
  §3.9a behavior 33c is why it must land **before** anything consumes the error list: §3.8's dedup
  keeps the first record at a location, so a wrongly-ordered list makes it keep the wrong one. The
  descriptors already carry the correct order (spec 003 §3.2), so part of the fix is feeding the
  binder the descriptor order; interleaving the date failures touches all three base binders plus
  `publication.go`.
- **Item 2 — a non-scalar `date` / `start_date` / `end_date` produces no failure at all.** §3.9b is
  the normative behavior. It is cheaper here than in iteration 3 because the correct end state is
  *one* record per field, reached by emitting the union-branch pair in declared order and letting
  §3.3's filter and §3.8's dedup run — not by hand-picking a message.
- **`PublicationEntry.url` normalization** was flagged, not diverged (spec 003 §7.3). §7.1 settles
  it: it is reproducible, no divergence is needed, and the registered seam at
  `internal/schema/models/cv/entries/publication.go:53` gets its one real implementation.
- **The three error codes iteration 3 shipped wrong** were fixed in `9ddd896`. §3.9c records the
  measured values this iteration dispatches on and warns that **any code asserted in the existing
  Go tests is suspect until measured** — three of iteration 3's own tests asserted the port's codes
  rather than upstream's, which is how the defect stayed green.

Nine decisions recorded so they are not relitigated:

**7.1 The three borrowed libraries are reproducible in Go and no divergence is proposed.**
Measured against the vendored Python, not argued:

- **Phone.** `github.com/nyaruka/phonenumbers` v1.8.1 reproduces `tel:+90-541-999-99-99`,
  `tel:+34-612-34-56-78`, `tel:+1-415-555-0142`, `tel:+44-20-1234-5678`, `tel:+49-30-12345678`,
  `tel:+81-90-1234-5678` and `tel:+61-412-345-678` byte for byte, and also reproduces the two
  national forms the goldens contain. It rejects `not_a_valid_phone_number`.
- **HTTP URL.** `github.com/nlnwa/whatwg-url` v0.6.2 reproduces **every** row of §3.13
  behavior 42, including `https://xn--nicode-2ya.de/%C3%BCn%C3%AF`, and rejects every input
  pydantic rejects. Its only difference is that it accepts `ftp://`, which §4.19's separate
  scheme check covers.
- **Email.** No Go port of `email-validator` exists; §7.4 states the bounded scope.

Because parity is achievable, **no entry is added to `specs/divergences.md` and no human gate is
requested** for any of the three. `plan.md` §1 records the alternatives and the residual risks.

**7.2 Coordinate columns: the port fixes them, even though users never see them.** The facts are
settled — §3.20 behaviors 74–75 prove columns are discarded in every code path and that no
machine-readable mode exists — so iteration 2's two column shapes (a key with a null or empty
value reporting column 1 instead of the key's indent; a flow-sequence element reporting the first
value token instead of the `[`) are **not a user-visible bug**. The recommendation is nonetheless
to fix them, for three reasons:

1. Upstream's own test compares `yaml_location` field-by-field with an equal-length assertion
   (`tests/schema/test_pydantic_error_handling.py:43-54`). Porting that test as a **full-structure
   differential** is the strongest mechanical Axis-4 gate available, and it is the *only* one:
   §3.20 behavior 77 shows the CLI goldens pin the table layout, not the messages. Weakening the
   comparison to locations, codes and messages leaves 50 of 388 coordinate pairs on
   `expected_errors.yaml` permanently unchecked, in the one file that is the contract.
2. The cost is bounded and already measured: two shapes, in one file
   (`internal/schema/yamlreader/build.go`), with the failing paths enumerated in
   `specs/STATE.md`.
3. A knowingly-wrong coordinate is a latent divergence with no owner. Fixing it now costs one
   commit; discovering it in iteration 12 costs an investigation.

So: the differential test compares **all five members**, the columns are fixed first, and the
coordinate fixture is generated by `tools/yamlprobe` rather than hand-stated (carried item 5).

**7.3 The gate is unit tests plus one differential fixture, and there is no golden refresh.** As
in spec 002 §7.2 and spec 003 §7.1: no corpus case can pass until the renderer exists, so the
parity suite stays at its 42 red cases and that redness is not this iteration's failure. This
iteration additionally does **not** run `just golden`. The 25-record `expected_errors.yaml`
comparison is read directly from the submodule as a read-only fixture; nothing is copied into
`testdata/golden/` and no human gate is requested (`AGENTS.md` §5).

**7.4 Email is scoped to a measured message set, and the residual is an open risk with a named
owner.** The `email-validator` library's syntax module is 822 lines and its message catalogue is
larger than §3.15 behavior 55. This iteration's contract is: pydantic's wrapper behaviors
(behavior 53), the normalization of behavior 54, the twelve messages of §4.22, and the accepted
inputs of behavior 56 — with the one Axis-4-pinned message,
`An email address must have an @-sign.`, non-negotiable. Any input producing a message outside
that set is an **open parity risk carried to iteration 13** (parity closeout). It is *not* written
into `specs/divergences.md` by this iteration, because no divergence has been demonstrated — only
an unenumerated surface. Iteration 13 either enumerates the remainder or proposes the divergence
and takes the human gate.

**7.5 The YAML parser's own text is the one place a divergence may be unavoidable, and this
iteration does not write it.** §4.31 interpolates ruamel's exception text; the Go reader uses
`goccy/go-yaml`, whose text differs. Measured example from our own golden:
`This is not a valid YAML file. while parsing a flow sequence.` The options, their costs and a
recommendation are in `plan.md` §6. **If the conclusion is that the text cannot be reproduced,
that is a `specs/divergences.md` entry and a human gate, and this spec does not authorize
writing it.** `tasks.md` places the investigation before the decision and makes the decision a
stop point.

**7.6 The stdout/stderr inversion is recorded, not fixed here.** §3.20 behavior 76. The correct
behavior is already pinned by `testdata/golden/err_wrong_input/` (9257 bytes stdout, 0 stderr) and
`testdata/golden/err_missing_file/` (0 stdout, 5287 stderr, so the two streams genuinely differ by
case). Fixing it is iteration 12's, together with the table renderer. This iteration produces the
records; it prints nothing.

**7.7 `err_unknown_theme`'s machine-specific golden is a corpus defect, assigned to iteration 12.**
§5.24. It is not fixable by changing Go code — the message interpolates an absolute path — so it
needs either a path-normalizing comparison in `internal/conformance` or a corpus case that runs
from a fixed working directory. Recorded so it is not mistaken for a message-parity failure when
iteration 12 turns the renderer on.

**7.8 Two upstream crashes stay crashes-in-spec and are not resolved here.** A non-mapping,
non-string entry (`internal/schema/models/cv/sectionvalidation.go:110`, spec 002 §5.14) and a
list-valued `phone` (§3.14 behavior 51) both raise unhandled Python exceptions rather than
validation errors. Neither is a validation-error-parity question — there is no message to match —
so both move to iteration 12, where the CLI's top-level error handler decides what an unhandled
failure looks like. This iteration only records that they are not §4 strings.

**7.9 Two thin slices of `design` and `settings` are pulled forward, because the 25-record fixture
needs them.** `expected_errors.yaml:141-145` is a `design.theme` record and `:147-151` is a
`settings.current_date` record, so the differential of §8 cannot reach 25 records without both.
This iteration therefore adds **only**:

- the theme-name shape check of §4.27, including its location re-pinning (§3.17 behavior 65) and
  the condition that reaches it — a `theme` value that is not one of the built-in names
  (`models/design/design.py:35-52`, `:60-70`);
- a `settings.current_date` shape check that fails for a value which is neither a `YYYY-MM-DD`
  date nor the literal `today`, so §4.13's override has something to fire on.

Everything else in `design` and `settings` — the theme models, every dimension and colour and
font option, §4.28 and §4.29's folder checks, the rest of the settings fields — stays with
iterations 6 and 7. A porter who finds themselves adding a second design field has left scope.

---

## 8. Acceptance criteria

Each is mechanically checkable. Criteria marked **[diff]** are the `expected_errors.yaml`
differential; everything else is a unit test.

**The transform**

- [ ] The eleven steps of §3.2 run in that order, asserted by a test per adjacent pair that would
      change output if swapped: strip-before-dictionary (using `value is not a valid phone
      number`), skip-before-context-override (using `design.theme`), filter-before-special-case
      (using `end_date` with a synthetic branch element), override-before-dictionary (using
      `end_date`), dictionary-before-period (using the color message).
- [ ] **The email prefix is stripped on production data.** `email: not_a_valid_email` yields a raw
      message carrying `value is not a valid email address: ` and a final message of
      `An email address must have an @-sign.` This half of §3.2 step 1 is additionally gated by the
      **[diff]** criterion, whose first two records are exactly this (§3.2 behavior 4a, §5.10a).
- [ ] **The `Value error, ` prefix is stripped on synthetic records, and is inert on production
      data.** Two tests, and the second is as important as the first: one feeding `parseOne` a
      hand-built record whose message contains the prefix **twice** and asserting both copies go
      (§6.6), and one asserting that **no** message produced anywhere under
      `internal/schema/models/**` contains the substring `Value error, `. Without the second test
      the first proves nothing about the port, because the port never emits the prefix — §3.2
      behavior 4a records why it does not fabricate one, and this criterion is written to be
      falsifiable rather than to imply coverage that does not exist.
- [ ] Neither prefix is stripped twice, and the strip is not conditioned on the error code: the
      `value_error`-coded email and phone messages are handled correctly and the `value_error`-coded
      arbitrary-date message is the only wrapped one (§3.2 behavior 4b, §3.9c behavior 33j).
- [ ] Each of the seven filter substrings of §3.3 behavior 5 removes an element, as a table test
      over the five measured synthetic elements of behavior 6.
- [ ] §3.3 behavior 7's five-row section-key table, end to end: four sections collapse to one
      record at `("cv", "sections")` whose coordinates are the `sections` mapping's, and
      `normal_key` survives intact.
- [ ] A list index survives the filter (behavior 8) and appears as a decimal string.
- [ ] §3.3 behavior 9's four-row discriminator table, including the one-element case.
- [ ] An empty location neither crashes nor takes the `design`/`locale` branch (behavior 10).

**The dictionary**

- [ ] All thirteen rows of §3.4 behavior 12 are present, in file order, with their keys and values
      byte-identical to §4 and to `error_dictionary.yaml:2-14`.
- [ ] Matching is substring containment on the message, first match wins: `Input should be a
      valid URL, relative URL without a base` becomes §4.9, and `value is not a valid color:
      string not recognised as a valid color` becomes §4.11.
- [ ] Rows 1, 2, 3, 4 and 10 are unreachable, asserted as such: an invalid `doi` produces
      `String should match pattern '\b10\..*'.` and **not** row 4's replacement (behavior 13); a
      bad `end_date` produces §4.12 and **not** row 1's replacement; `date: {a: 1}` produces
      `Input should be a valid integer.` and **not** row 2's replacement (§5.11).

**The special cases and the period**

- [ ] `end_date: invalid_date` produces §4.12 with a terminal `!.`, from **one** record, not two
      (§3.5 behaviors 15–17, §5.4).
- [ ] The `end_date` override fires on containment, not equality: a location ending
      `my_end_date` also gets §4.12.
- [ ] `settings: {current_date: todady}` produces §4.13 with a single period, from one record
      (§3.5 behaviors 18–19).
- [ ] The `current_date` suffix strip fires only when both `location[-1] == "date"` and
      `location[-2] == "current_date"`.
- [ ] §3.6's four-row table: an unmatched message, §4.12, §4.3 and §4.11 all end correctly,
      including `!.`, `.".` and `)".`.

**Unpacking, dedup, order**

- [ ] A wrapped entry failure keeps its own record and appends one per child, in child order,
      with the leading `entries` element dropped and the wrapper's raw location prepended
      (§3.7 behaviors 21–23). The measured case is `("entries", 1, "institution")` →
      `["cv","sections","welcome_to_rendercv_tests_2","1","institution"]`.
- [ ] A child with an empty location lands at the wrapper's location plus nothing
      (behavior 24).
- [ ] Unpacking does not recurse (behavior 26).
- [ ] A wrapped failure with no `ctx`, and one with a `ctx` lacking `caused_by`, both produce the
      internal failure of §4.16.
- [ ] §3.8 behavior 28's three-row dedup table.
- [ ] Dedup keeps the first and preserves relative order, asserted with three records at two
      locations.
- [ ] §3.9 behavior 32's six-part ordering rule, with the measured seven-record sequence of
      step 3 as a table test. No sort appears anywhere in the implementation.
- [ ] `sections` failures follow document order within the field (behavior 33).

**Entry-internal ordering, non-scalar dates, and codes** (the two iteration-3 cut items)

- [ ] §3.9a behavior 33a's four-row table, as a table test on the raw list, **before** the pipeline
      runs. All four currently fail.
- [ ] A date failure appears at the date field's declared position, not appended
      (behavior 33b) — asserted with the three-error `ExperienceEntry` row.
- [ ] Every one of the eight entry types reports its failures in the declared order of spec 003
      §3.3–§3.10, as one table test over the eight, so the composition fix cannot be applied to
      some types and not others.
- [ ] §3.9b behavior 33d's three-row raw-branch table: `date` emits int-then-str, `start_date`
      emits str-then-int, `end_date` emits three.
- [ ] §3.9b behavior 33e's four-row end-to-end table: exactly one record per field, with
      `Input should be a valid integer.` for `date`, `Input should be a valid string.` for
      `start_date`, and §4.12 for `end_date` (§5.11a, §5.11b).
- [ ] `date: true` is accepted (behavior 33g).
- [ ] §3.9c behavior 33h's six-row code table, measured against the vendored Python and not copied
      from the existing Go tests.
- [ ] Only `rendercv_entry_validation_error` triggers §3.7's unpacking, and only the literal
      `missing` triggers §3.10's truncation (behavior 33i).
- [ ] The prefix rule is not keyed on the code (behavior 33j) — covered by the third criterion of
      *The transform* above, and cross-listed here because that is the mistake this table's codes
      invite.

**Coordinates and input**

- [ ] A `missing` failure resolves coordinates from `location[:-1]`; every other kind uses the
      full location (§3.10 behaviors 35–36).
- [ ] Two missing fields of one entry report identical coordinates (§5.9).
- [ ] The two coordinate-walk internal failures of §4.17 and §4.18 (§5.16).
- [ ] `input` renders a mapping and a sequence as §4.15, a null as `None`, and an integer as its
      digits (§3.11).
- [ ] A context-supplied `input` wins over the raw one (§3.11, §3.17 behavior 65).
- [ ] Overlay source selection, all three cases of §5.14.
- [ ] The two column shapes of §7.2 are fixed, and `tools/yamlprobe` generates the coordinate
      fixture rather than the test stating it.

**Phone**

- [ ] §3.14 behavior 48's five-row table, stored and serialized forms both.
- [ ] `not_a_valid_phone_number` produces §4.8.
- [ ] A list-valued `phone` validates every element and produces one record per bad element, at
      the element's index.

**HTTP URL**

- [ ] §3.13 behavior 42's eleven-row normalization table.
- [ ] The three failure kinds of behavior 45, with every measured `url_parsing` reason producing
      §4.9 and the two non-dictionary kinds producing §4.19 and §4.20 with a period.
- [ ] The length semantics of behavior 46: 2083 accepted, 2084 rejected, a 420-character input
      serializing to 2420 accepted, and a malformed-but-long input reporting `url_too_long`.
- [ ] One registered validator serves all four sites of behavior 41, asserted by checking that
      `cv.website`, `CustomConnection.url`, `PublicationEntry.url` and the generated social URL
      all reject `example.com` with §4.9.
- [ ] The generated social URL is validated but **not** normalized, and a LinkedIn username of
      `not a valid %%^&*()` produces no record (behavior 44, §5.8).

**Email**

- [ ] §3.15 behavior 55's twelve-row message table.
- [ ] Behavior 54's normalization: domain lowercased, local part not, non-ASCII domain left
      as written.
- [ ] Behavior 53: a 2049-character input gives §4.21; `Name <a@b.com>` validates the address
      part; surrounding whitespace is stripped silently.
- [ ] Behavior 56's accepted inputs.

**Per-network usernames**

- [ ] All eight rules of §3.16 behavior 59, one test each, message verbatim from §4.
- [ ] §4.3's emitted form ends `username.".` (behavior 61).
- [ ] Every rule is a full match, not a search: `x@a@b` fails Mastodon, `12/name/extra` fails
      StackOverflow, `nm12345678` fails IMDB (behavior 60).
- [ ] The nine networks with no rule accept any username.
- [ ] An absent or unknown `network` suppresses the username check and reports only the network
      failure with §4.23 (behaviors 58, 62).

**Placeholders replaced**

- [ ] §3.19 behavior 71's four rows. `cv: null` and `cv: 5` both give
      `Input should be a valid dictionary or instance of Cv.`
- [ ] §4.32's suffix is per-model, asserted for `Cv`, `SocialNetwork`, `CustomConnection` and at
      least one entry class (behavior 72).
- [ ] The remaining strings pinned by iterations 2 and 3 as "pydantic's, decided in iteration 4"
      are now decided: §4.14, §4.10, `Input should be a valid string.`,
      `This field should contain a list of items but it doesn't.`, §4.20, and the CPython date
      texts of spec 002 §4.13 plus §4.33.

**The YAML-syntax producer**

- [ ] A syntax failure produces exactly one record, with no schema location, 1-indexed
      coordinates from the parser's marks, the source of the failing document, §4.15 as input,
      and §4.31 as the message (§3.18 behaviors 68–69).
- [ ] Coordinates are absent when the parser supplies no marks.
- [ ] The interpolation is the first line, stripped, with a period appended when absent.

**The differential**

- [ ] **[diff]** Reading `tests/schema/testdata/test_pydantic_error_handling/wrong_input.yaml`
      from the submodule and running the full pipeline produces **25** records that equal
      `expected_errors.yaml`'s, compared **member by member on all five members** and **in
      order**, with equal length asserted. This is upstream's own
      `tests/schema/test_pydantic_error_handling.py:19-54`, and it replaces the
      locations-and-codes-only comparison at
      `internal/schema/models/wronginput_conformance_test.go:82`.
- [ ] **[diff]** The same fixture, run with a `design` overlay, reports `design_yaml_file` for
      `design`-rooted records and `main_yaml_file` for the rest (§5.14).

---

## 9. Corpus additions

**None, and no golden refresh** (§7.3). `tools/gengolden` is not run and `testdata/golden/` is not
touched.

Two **read-only submodule fixtures** are consumed instead:

1. `tests/schema/testdata/test_pydantic_error_handling/wrong_input.yaml` and
   `tests/schema/testdata/test_pydantic_error_handling/expected_errors.yaml`, read directly from
   `third_party/rendercv`, as the full-structure differential of §8. Iteration 3 already reads the
   first for locations and codes; this iteration widens the comparison and keeps the file where it
   is.
2. `src/rendercv/schema/error_dictionary.yaml`, read directly, so the thirteen rows cannot drift
   from upstream by transcription. `plan.md` §3 decides whether it is embedded at build time or
   compared in a test.

**Axis 4 is verified through `expected_errors.yaml`, not through the CLI goldens.** §3.20
behavior 77 is the reason: at 80 columns the `err_wrong_input` golden squeezes the `Explanation`
column to nothing, so it pins the table layout algorithm and the record *order*, but contains
almost none of the message text this iteration produces. The seven `err_*` goldens remain
iteration 12's gate for presentation.

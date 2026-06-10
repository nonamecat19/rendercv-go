# Spec 006 delta — the `settings` tree is unvalidated

**Status:** proposal · **Extends:** [`settings.md`](settings.md) §4.1 and §5 · **Inherits:**
[`../000-parity-contract/spec.md`](../000-parity-contract/spec.md)

**Unblocks:** the 88-vector type-mismatch sweep that stopped because `settings.md` §4.1 assigns
`render_command`'s behavior to iteration 12 and §5 lists it as an open criterion. This file removes
that blocker: it specifies the behavior, so the sweep's 41 open vectors become implementable units.

**Upstream covered**

- `src/rendercv/schema/models/settings/settings.py` — the four `Settings` fields
- `src/rendercv/schema/models/settings/render_command.py` — the thirteen `RenderCommand` fields
- `src/rendercv/schema/models/path.py` — the two path aliases
- `src/rendercv/schema/pydantic_error_handling.py` — message, location and Input Value derivation
- `src/rendercv/schema/rendercv_model_builder.py` — the CLI-flag overwrite and its `if value:` gate
- `src/rendercv/cli/render_command/run_rendercv.py`, `.../render_command.py` — mechanism F's crash

Citations to `src/...` are relative to `third_party/rendercv/`.

**Everything in this document was measured**, not read off the Python source: 126 CLI vectors run
through `third_party/rendercv/.venv/bin/rendercv` and through `bin/rendercv-go` at `5fb1a5f`, plus
model-layer probes that call `RenderCVModel.model_validate` and
`build_rendercv_dictionary_and_model` directly to reach the records the CLI crashes before printing
(§4). Where source and measurement disagree, the measurement is what is written down — §3.4's lax
boolean set and §5.4's epoch dates are both cases where reading the type alone gives the wrong rule.

---

## 1. Purpose

The `settings` block accepts a `current_date`, a `pdf_title`, a `bold_keywords` list, and a
`render_command` sub-block of eight paths and five booleans. Upstream validates every one of them
and exits 1 on a type mismatch; the port validates `current_date` thinly, rejects unknown keys, and
accepts everything else — it renders a complete CV at exit 0 where upstream refuses. This delta
specifies the refusals, one mechanism at a time, so that the 41 open vectors of the sweep close
against Axis 2 (exit code and output shape) and Axis 4 (message and location).

---

## 2. The exact upstream type of every field

Declaration order is contractual: it is the JSON-schema property order and the **error-row order**
(§6.1). Column *Measured* records the CLI probe that confirms the type, not the annotation.

### 2.1 `Settings` — `settings.py:10-51`

| # | Field | Upstream type | Cite | Default | Measured |
|---|---|---|---|---|---|
| 1 | `current_date` | `datetime.date \| Literal["today"]` — a two-arm union, **no null arm** | `settings.py:11` | `"today"` | `current_date: null` → exit 1 (§3.5) |
| 2 | `render_command` | `RenderCommand` (nested model) | `settings.py:19-20` | `RenderCommand()` via `default_factory` | `render_command: abc` → exit 1 (§3.6) |
| 3 | `bold_keywords` | `list[str]` | `settings.py:27` | `[]` | already ported, commit `cba5b8e`; all 7 kinds agree |
| 4 | `pdf_title` | `str` | `settings.py:32` | `"NAME - CV"` | `pdf_title: 42` → exit 1 (§3.2) |

`_resolved_current_date` (`settings.py:52`) is a `PrivateAttr`, not a field — a document that writes
it is an unknown key, which the port already reports (`settings.go:84-111`).

### 2.2 `RenderCommand` — `render_command.py:29-117`

| # | Field | Upstream type | Cite | Default | Null legal? |
|---|---|---|---|---|---|
| 1 | `output_folder` | `PlannedPathRelativeToInput` | `render_command.py:30-37` | `rendercv_output` | no |
| 2 | `design` | `ExistingPathRelativeToInput \| None` | `render_command.py:38-41` | `None` | **yes** |
| 3 | `locale` | `ExistingPathRelativeToInput \| None` | `render_command.py:42-45` | `None` | **yes** |
| 4 | `typst_path` | `PlannedPathRelativeToInput` | `render_command.py:46-53` | `OUTPUT_FOLDER/NAME_IN_SNAKE_CASE_CV.typ` | no |
| 5 | `pdf_path` | `PlannedPathRelativeToInput` | `render_command.py:54-61` | `…_CV.pdf` | no |
| 6 | `markdown_path` | `PlannedPathRelativeToInput` | `render_command.py:62-70` | `…_CV.md` | no |
| 7 | `html_path` | `PlannedPathRelativeToInput` | `render_command.py:71-78` | `…_CV.html` | no |
| 8 | `png_path` | `PlannedPathRelativeToInput` | `render_command.py:79-86` | `…_CV.png` | no |
| 9 | `dont_generate_markdown` | `bool` | `render_command.py:87-94` | `false` | no |
| 10 | `dont_generate_html` | `bool` | `render_command.py:95-99` | `false` | no |
| 11 | `dont_generate_typst` | `bool` | `render_command.py:100-107` | `false` | no |
| 12 | `dont_generate_pdf` | `bool` | `render_command.py:108-112` | `false` | no |
| 13 | `dont_generate_png` | `bool` | `render_command.py:113-117` | `false` | no |

Both path aliases are `pathlib.Path` with an `AfterValidator`
(`path.py:67-72` existing, `path.py:74-80` planned). **The `must_exist` difference is invisible to
the type check** — a wrong *type* produces the same `path_type` failure for all eight fields
(`design: [a]` measured identical to `output_folder: [a]`, §3.3). The only observable difference is
the null arm: `design: null` and `locale: null` render at exit 0; `output_folder: null` does not.

### 2.3 Absent is always legal

`settings` absent, `settings: {}`, `render_command` absent and `render_command: {}` all render at
exit 0 in both implementations — every field has a default (`settings.py:11-51`,
`render_command.py:30-117`). Measured, four vectors, no divergence.

---

## 3. The five open mechanisms, measured

Rows are `<yaml value> → exit code · Location cell · Input Value cell · Explanation cell`. The port
column is `bin/rendercv-go` at `5fb1a5f`. Every table cell below was read out of a real CLI run.

### 3.1 How the three columns are derived — applies to B, C, D, F

- **Explanation** is pydantic's `msg`, after `unwanted_texts` stripping
  (`pydantic_error_handling.py:50-51`), after the `error_dictionary.yaml` lookup
  (`:89-92`), then with a trailing `.` appended if absent (`:94-95`). **None of B, C or D has an
  `error_dictionary.yaml` row** — the file's 13 rows are listed at
  `src/rendercv/schema/error_dictionary.yaml:2-14` and none matches `string_type`, `path_type`,
  `bool_type` or `bool_parsing` — so the visible text is pydantic's own plus the period.
- **Input Value** is `str(input)` unless the input is a `dict` or a `list`, in which case it is the
  literal `...` (`pydantic_error_handling.py:122-126`). Hence `True`, `None`, `42`, `3.14`, the raw
  string, and `...`.
- **Location** is the dotted schema path, unchanged for this subtree (no discriminator stripping:
  `settings` is not in `discriminatedRoots`, `settings.md` §1 behavior 3).

### 3.2 Mechanism B — `pdf_title: str`

| Input | Upstream | Location | Input Value | Explanation | Port today |
|---|---|---|---|---|---|
| `"abc"` | 0 | — | — | — | 0 ✓ |
| `""` | 0 | — | — | — | 0 ✓ |
| `42` | 1 | `settings.pdf_title` | `42` | `Input should be a valid string.` | **0** |
| `3.14` | 1 | `settings.pdf_title` | `3.14` | `Input should be a valid string.` | **0** |
| `true` | 1 | `settings.pdf_title` | `True` | `Input should be a valid string.` | **0** |
| `null` | 1 | `settings.pdf_title` | `None` | `Input should be a valid string.` | **0** |
| `[a, b]` | 1 | `settings.pdf_title` | `...` | `Input should be a valid string.` | **0** |
| `{a: 1}` | 1 | `settings.pdf_title` | `...` | `Input should be a valid string.` | **0** |
| absent | 0 | — | — | — | 0 ✓ |

Raw pydantic type: `string_type`. **No coercion in either direction** — `"42"` passes, `42` does
not.

### 3.3 Mechanism C — the eight path fields

Applies identically to `output_folder`, `typst_path`, `pdf_path`, `markdown_path`, `html_path`,
`png_path`, `design`, `locale`. Below, `<f>` is any of the eight and `<loc>` is
`settings.render_command.<f>`.

| Input | Upstream | Location | Input Value | Explanation | Port today |
|---|---|---|---|---|---|
| `"abc"` | 0 (see note) | — | — | — | see §10.1 |
| `""` | 0 | — | — | — | 0 ✓ |
| `42` | 1 | `<loc>` | `42` | `Input is not a valid path for <class 'pathlib.Path'>.` | **0** |
| `3.14` | 1 | `<loc>` | `3.14` | same | **0** |
| `true` | 1 | `<loc>` | `True` | same | **0** |
| `[a, b]` | 1 | `<loc>` | `...` | same | **0** |
| `{a: 1}` | 1 | `<loc>` | `...` | same | **0** |
| `null`, six planned paths | 1 | `<loc>` | `None` | same | **0** |
| `null`, `design`/`locale` | **0** | — | — | — | 0 ✓ |
| absent | 0 | — | — | — | 0 ✓ |

Raw pydantic type: `path_type`. `""` is accepted and then short-circuits `resolve_relative_path`'s
`if path:` guard (`path.py:37`), so no resolution happens and the default filename logic downstream
still fires; both implementations render at exit 0.

**`design` and `locale` are measured through the model layer, not the CLI**, because upstream's CLI
crashes before validation for those two (§3.6, F3). The record above is what upstream's own model
produces and what §7's recommendation makes reachable in the port.

### 3.4 Mechanism D — the five `dont_generate_*` booleans

**Upstream's boolean is lax, and there are two different messages.** A rule of "anything not
`true`/`false` is refused" is wrong in both directions.

Accepted (exit 0, value coerced, and the flag then *takes effect* — `dont_generate_html: "yes"`
suppresses HTML upstream):

```
true  false  True  False  y  n  yes  no  on  off
0  1  0.0  1.0  1e0
"true" "True" "TRUE" "TrUe" "false" "yes" "no" "on" "off" "t" "f" "y" "n" "0" "1"
```

Refused, with the message depending on the *kind*:

| Input | Upstream | Location | Input Value | Explanation | pydantic type |
|---|---|---|---|---|---|
| `2`, `-1`, `2.0` | 1 | `settings.render_command.<f>` | `2` / `-1` / `2.0` | `Input should be a valid boolean, unable to interpret input.` | `bool_parsing` |
| `"abc"`, `""`, `"  true  "` | 1 | same | `abc` / `` / `  true  ` | `Input should be a valid boolean, unable to interpret input.` | `bool_parsing` |
| `3.14`, `0.5` | 1 | same | `3.14` / `0.5` | `Input should be a valid boolean.` | `bool_type` |
| `null` | 1 | same | `None` | `Input should be a valid boolean.` | `bool_type` |
| `[a, b]` | 1 | same | `...` | `Input should be a valid boolean.` | `bool_type` |
| `{a: 1}` | 1 | same | `...` | `Input should be a valid boolean.` | `bool_type` |

The split: a **string or an int** that is not in the accepted set is `bool_parsing` (the "unable to
interpret input" suffix); a **float that is not exactly 0 or 1**, a null, a list and a mapping are
`bool_type` (the bare message). Whitespace is *not* stripped — `"  true  "` is refused.

The port today: exit 0 for every refused row, and `dont_generate_html: "yes"` is *not* honored
(HTML is still generated) — so D has an accept-side defect as well as a refuse-side one.

**`dont_generate_pdf` and `dont_generate_png` are not special.** With no CLI flags they behave
exactly like the other three, measured. They look special only under `-nopdf -nopng`, which is
§9's subject.

### 3.5 Mechanism E — `current_date: null`

| Input | Upstream | Location | Input Value | Explanation | Port today |
|---|---|---|---|---|---|
| `null` | 1 | `settings.current_date` | `None` | ``This is not a valid `current_date`! Please use YYYY-MM-DD format or "today".`` | **0** |

The union has no null arm (`settings.py:11`), so a null produces `date_type` on the date branch and
`literal_error` on the `"today"` branch; the location-based override at
`pydantic_error_handling.py:83-87` replaces the message, and the dedupe at `:168-175` keeps only the
first record per location, so **one row**, not two. Every other kind of `current_date` already
agrees between the two implementations; `null` is the single open vector, and it is open because
`ValidateCurrentDate` returns nil for `KindNull` (`internal/schema/models/settings/settings.go:39-41`).

### 3.6 Mechanism F — the shapes that crash upstream

Upstream prints a **Rich traceback on stderr and exits 1**; no validation panel is produced. Three
distinct crash sites:

| # | Input | Crash | Frames | Port today |
|---|---|---|---|---|
| F1 | `settings:` is a string / int / float / bool / null / list | `AttributeError: '<T>' object has no attribute 'get'` | `render_command.py:205` → `run_rendercv.py:118` | **0** |
| F2 | `settings.render_command:` is a string / int / float / bool / null / list | `AttributeError: '<T>' object has no attribute 'get'` | `render_command.py:205` → `run_rendercv.py:119` | **0** |
| F3 | `render_command.design` / `.locale` is an int / float / bool / list / mapping | `TypeError: unsupported operand type(s) for /: 'PosixPath' and '<T>'` | `render_command.py:205` → `run_rendercv.py:120` (design) / `:122` (locale) | **0** |

`<T>` is the ruamel runtime class: `str`, `int`, `ScalarFloat`, `bool`, `NoneType`, `CommentedSeq`,
`CommentedMap`. The `contextlib.suppress` at `run_rendercv.py:113` catches only
`RenderCVUserValidationError`, so none of these is caught.

**The clean records exist; the CLI just never reaches them.** Driving the model directly gives:

| Input | Location | Input Value | Explanation |
|---|---|---|---|
| `settings: abc` | `settings` | `abc` | `Input should be a valid dictionary or instance of Settings.` |
| `settings: 42` | `settings` | `42` | same |
| `settings: 3.14` | `settings` | `3.14` | same |
| `settings: true` | `settings` | `True` | same |
| `settings: null` | `settings` | `None` | same |
| `settings: [a]` | `settings` | `...` | same |
| `render_command: abc` | `settings.render_command` | `abc` | `Input should be a valid dictionary or instance of RenderCommand.` |
| `render_command: 42` | `settings.render_command` | `42` | same |
| `render_command: 3.14` | `settings.render_command` | `3.14` | same |
| `render_command: true` | `settings.render_command` | `True` | same |
| `render_command: null` | `settings.render_command` | `None` | same |
| `render_command: [a]` | `settings.render_command` | `...` | same |
| `design: [a]` | `settings.render_command.design` | `...` | `Input is not a valid path for <class 'pathlib.Path'>.` |

F1 additionally crashes the *library* entry point, not only the CLI:
`build_rendercv_dictionary` does `input_dict.setdefault("settings", {}).setdefault("render_command",
{})` at `rendercv_model_builder.py:118`, which raises `AttributeError: '<T>' object has no attribute
'setdefault'` for a non-mapping `settings`. F2 and F3 do **not** crash the library entry point —
`build_rendercv_dictionary_and_model` returns the clean records above. So F1's record is reachable
only through `RenderCVModel.model_validate`; F2's and F3's are reachable one layer up.

---

## 4. Exact user-visible strings

Verbatim, one fenced block per string. All are Explanation-column text; all end in the period that
`pydantic_error_handling.py:94-95` appends.

Mechanism B — `settings.pdf_title` is not a string:

```
Input should be a valid string.
```

Mechanism C — any of the eight path fields is not a string (and, for the six planned paths, when it
is null):

```
Input is not a valid path for <class 'pathlib.Path'>.
```

Mechanism D — a `dont_generate_*` value is a float other than 0/1, a null, a list or a mapping:

```
Input should be a valid boolean.
```

Mechanism D — a `dont_generate_*` value is a string or an int outside the accepted set:

```
Input should be a valid boolean, unable to interpret input.
```

Mechanism E — `settings.current_date` is null (the same string every other bad `current_date`
already produces):

```
This is not a valid `current_date`! Please use YYYY-MM-DD format or "today".
```

Mechanism F1 — `settings` is not a mapping:

```
Input should be a valid dictionary or instance of Settings.
```

Mechanism F2 — `settings.render_command` is not a mapping:

```
Input should be a valid dictionary or instance of RenderCommand.
```

F3 reuses mechanism C's string with location `settings.render_command.design` or
`settings.render_command.locale`.

No new `error_dictionary.yaml` row is involved in any of the seven: the file has 13 rows
(`error_dictionary.yaml:2-14`) and none is keyed on `string_type`, `path_type`, `bool_type`,
`bool_parsing` or `model_type`. The one dictionary row this subtree already uses is
`Extra inputs are not permitted` → `This field is unknown for this object. Please remove it.`
(`error_dictionary.yaml:12`), for unknown keys, which the port already emits at both levels
(measured: `settings.no_such_key` and `settings.render_command.no_such_key`, byte-identical).

---

## 5. Edge cases

**5.1 An empty string is a legal path and a legal title.** `output_folder: ""` and `pdf_title: ""`
both render at exit 0 (`path.py:37`'s `if path:` guard). Measured.

**5.2 `design: null` and `locale: null` are legal; the other six paths' nulls are not.** §2.2. A
single "paths reject null" rule is wrong for two of eight fields.

**5.3 A string `design`/`locale` naming a file that does not exist never reaches the
`The file `{file_path}` does not exist.` validator** (`path.py:45-49`) through the `render`
command: `render_command.py:212` calls `design.read_text()` first and raises `FileNotFoundError`.
Upstream prints a traceback; the port already prints `The file abc does not exist!` at exit 1. Both
refuse; the shapes differ, and that difference is D-014's, not this delta's.

**5.4 `current_date` accepts integers as Unix timestamps.** `current_date: 0` → 1970-01-01 and
`current_date: 86400` → 1970-01-02 both render at exit 0 upstream; the port refuses both with the
`current_date` message. Timestamps that are not exactly midnight (`42`, `-1`) are refused upstream
with `date_from_datetime_inexact` on the date branch, which the location override replaces with the
same `current_date` string. This is a **port-is-stricter** divergence in the opposite direction from
B–F; it is scoped as unit E2 in §8.

**5.5 An unquoted YAML date works, quoted or not.** `current_date: 2024-01-02` and
`current_date: "2024-01-02"` both render at exit 0 in both implementations. `2024-01`, `"2024"` and
`"2024-1-2"` are refused by both.

**5.6 `"Today"` and `"TODAY"` are refused.** The literal arm is case-sensitive
(`settings.py:11`). Both implementations agree.

**5.7 Multiple bad fields report multiple rows in one panel.** Measured with
`pdf_title: [a]` + `output_folder: 1` + `dont_generate_html: {a: 1}`: three rows, exit 1.

**5.8 Upstream's own tests cover almost none of this.**
`tests/schema/models/settings/test_settings.py:9-57` has four `current_date` cases (`"today"`, a
`date` object, `"2024-06-15"`, and a rejected `"not-a-date"`), three `bold_keywords` cases and two
`pdf_title` cases — **no type-mismatch case for any `RenderCommand` field, and no null case for
`current_date`**. `tests/schema/test_rendercv_model_builder.py:36-42` pins that
`build_rendercv_dictionary` creates `settings` and `settings.render_command` when absent;
`:176-217` pins that a CLI override lands in `settings.render_command` and that unrelated settings
keys survive. `tests/cli/render_command/test_run_rendercv.py:187-273` pins
`collect_input_file_paths` — including `test_invalid_yaml_still_returns_input_file`, which shows the
author considered malformed input at this exact call site and covered only the YAML-parse arm, not
the wrong-shape arm F crashes on. Every row in §3 therefore comes from measurement, and each becomes
an acceptance criterion in §7 or a corpus case in §9.

---

## 6. Ordering and whitespace guarantees

**6.1 Error rows follow declaration order, not document order.** Measured: a document writing
`pdf_title` before `render_command.output_folder` reports `settings.render_command.output_folder`,
then `settings.render_command.dont_generate_html`, then `settings.pdf_title` — `render_command` is
field 2 and `pdf_title` is field 4 (`settings.py:11-51`), and within `RenderCommand`,
`output_folder` is field 1 and `dont_generate_html` field 10 (`render_command.py:30-99`). A second
measurement (`current_date: null` + `pdf_title: 1`) gives `current_date` first, same rule.

**6.2 One row per location.** `pydantic_error_handling.py:168-175` dedupes on the schema location,
first record wins. This is why `current_date`'s two union failures collapse to one row (§3.5).

**6.3 Unknown-key rows and type rows share one panel** and are ordered by the same rule.

**6.4 No whitespace claim is new here.** The panel and table layout are `internal/cli`'s and are
already under test; this delta adds rows, not a renderer.

---

## 7. Recommendation for mechanism F

**F should follow the D-012/D-014 precedent: the port exits 1 with its own validation panel, and no
new divergence entry is needed for the class — only an amendment naming these vectors.**

Reasoning, in the order it matters:

1. **F is exactly D-014's declared class.** D-014's title is "Any upstream unhandled exception is
   reported as a clean error, not a traceback", and its scope is every path where
   `cli/error_handler.py:38-49`'s `handle_user_errors` does not catch. F1, F2 and F3 raise
   `AttributeError`/`TypeError` outside that decorator's `RenderCVUserError` net, so they are
   inside D-014 by construction. D-012 §3 already recorded four ruamel constructor crashes the same
   way — "recorded here because this is where they were measured; each is D-011's class, not this
   entry's" — which is the precedent for adding vectors to an existing entry rather than opening a
   new one.
2. **The port is not fabricating a string.** This is the part that distinguishes F from earlier
   traceback cases and makes the recommendation cheap: upstream's *own model* produces
   `Input should be a valid dictionary or instance of Settings.` and
   `Input should be a valid dictionary or instance of RenderCommand.` for exactly these inputs
   (§3.6, measured). The CLI crashes on the way to them. So the port's panel can carry upstream's
   message at upstream's location and upstream's exit code; the only thing that differs is that
   upstream never gets there. That is the smallest possible divergence, and it is the same
   "nearest clean error" D-014 mandates.
3. **The message shape already exists in the port.** `Input should be a valid dictionary or
   instance of {Model}` is the binder's `model_type` text (spec 004 §4.32; `internal/schema/binder`
   already emits it for `cv`, `locale.phrases`, `design.page` and others). F1 and F2 need a model
   declaration for `settings` and `render_command`, not a new string.
4. **F3 collapses into mechanism C.** Once `design` and `locale` are typed as path fields, the
   record for `design: [a]` is C's `Input is not a valid path for <class 'pathlib.Path'>.` at
   `settings.render_command.design` — again upstream's own model text. F3 needs no panel of its
   own; it needs C plus the ordering guarantee in behavior 7.5 below.

**What the panel says** — three cases, all upstream's own text, all exit 1, on the stream the port's
validation panel already uses:

```
Input should be a valid dictionary or instance of Settings.
```

```
Input should be a valid dictionary or instance of RenderCommand.
```

```
Input is not a valid path for <class 'pathlib.Path'>.
```

**Behavior 7.5 — order of operations.** The port must validate the `settings` tree **before** it
resolves or reads the `design`/`locale` overlay files. Upstream's crash is precisely the
consequence of doing it in the other order (`render_command.py:205` runs before validation), and if
the port resolves first it will report a file-not-found for a *list-valued* `design` instead of
C's path record. This is a sequencing requirement on the CLI, not a message.

**Divergence bookkeeping.** Proposed amendment to `specs/divergences.md`, appended to D-014's
entry rather than filed as a new ID:

> **Also in this class, measured against the `settings` tree (spec 006
> `spec-delta-settings-validation.md` §3.6):** a non-mapping `settings` or
> `settings.render_command`, and a non-string `render_command.design`/`.locale`, crash upstream's
> CLI with `AttributeError`/`TypeError` at `render_command.py:205` → `run_rendercv.py:118-122`,
> before any validation runs. `rendercv-go` reports the record upstream's own model would have
> produced had the CLI reached it — `Input should be a valid dictionary or instance of Settings.`,
> `Input should be a valid dictionary or instance of RenderCommand.`, and
> `Input is not a valid path for <class 'pathlib.Path'>.` — at the same locations and the same
> exit code. Unlike D-014's `create-theme` pair there is no stream inversion: both sides use exit
> 1, and the port uses the validation panel it uses for every other record.

If the project prefers one entry per measurement site, file it under the next free ID instead; at
`5fb1a5f` the highest entry in `specs/divergences.md` is **D-020**, so "D-021" as used in the
briefing is not yet present on this branch and the ID must be checked at merge time rather than
assumed.

---

## 8. Acceptance criteria

One block per mechanism. Each line is mechanically checkable by a table-driven unit test on the
validator plus one conformance vector from §9.

**B — `pdf_title`**

- [ ] `pdf_title` of int, float, bool, null, list, mapping → exit 1, one row,
      Location `settings.pdf_title`, Explanation `Input should be a valid string.`
- [ ] Input Value cell is `42` / `3.14` / `True` / `None` / `...` / `...` respectively
- [ ] `pdf_title` of `""`, `"42"`, any string, or absent → exit 0

**C — the eight path fields**

- [ ] For each of `output_folder`, `typst_path`, `pdf_path`, `markdown_path`, `html_path`,
      `png_path`, `design`, `locale`: int, float, bool, list, mapping → exit 1, Location
      `settings.render_command.<field>`, Explanation
      `Input is not a valid path for <class 'pathlib.Path'>.`
- [ ] `null` → exit 1 for the six planned paths, **exit 0** for `design` and `locale`
- [ ] `""` and any string → exit 0 for all eight (subject to §5.3 for `design`/`locale`)
- [ ] A list-valued `design` reports the path record, **not** a file-not-found — the ordering
      guarantee of behavior 7.5

**D — the five `dont_generate_*` flags**

- [ ] Every literal in §3.4's accepted set → exit 0 **and the flag takes effect**
      (`dont_generate_html: "yes"` suppresses HTML; `dont_generate_pdf: 1` suppresses the PDF)
- [ ] `2`, `-1`, `2.0`, `"abc"`, `""`, `"  true  "` → exit 1 with
      `Input should be a valid boolean, unable to interpret input.`
- [ ] `3.14`, `0.5`, `null`, list, mapping → exit 1 with `Input should be a valid boolean.`
- [ ] All five fields behave identically when no CLI flag is given — including `dont_generate_pdf`
      and `dont_generate_png`

**E — `current_date`**

- [ ] `current_date: null` → exit 1, Location `settings.current_date`, Input Value `None`,
      Explanation ``This is not a valid `current_date`! Please use YYYY-MM-DD format or "today".``
- [ ] Exactly one row, not two (§6.2)
- [ ] E2: `current_date: 0` and `current_date: 86400` → exit 0; `42` and `-1` → exit 1 with the
      same message

**F — shapes**

- [ ] `settings` of string/int/float/bool/null/list → exit 1, Location `settings`, Input Value per
      §3.6, Explanation `Input should be a valid dictionary or instance of Settings.`
- [ ] `render_command` of string/int/float/bool/null/list → exit 1, Location
      `settings.render_command`, Explanation
      `Input should be a valid dictionary or instance of RenderCommand.`
- [ ] Neither case crashes, panics, or produces a Go stack trace — the port's panel, exit 1
- [ ] `settings: {}` and `render_command: {}` still render at exit 0

**Cross-cutting**

- [ ] Multi-error documents report in declaration order (§6.1), one row per location (§6.2)
- [ ] The unknown-key rows already produced at both levels are unchanged
- [ ] Nothing above changes the exit-0 output bytes of any existing golden

---

## 9. The CLI-flag precedence question, answered plainly

**Yes — the port's `dont_generate_pdf` / `dont_generate_png` precedence is already correct, and
here is the measurement.**

| Vector | Upstream | Port | Agree |
|---|---|---|---|
| `dont_generate_pdf: {a: 1}` + `-nopdf -nopng` | 0 | 0 | ✓ |
| `dont_generate_png: {a: 1}` + `-nopdf -nopng` | 0 | 0 | ✓ |
| `dont_generate_pdf: [a, b]` + `-nopdf -nopng` | 0 | 0 | ✓ |
| `dont_generate_markdown: {a: 1}` + `-nopdf -nopng` | 1 | 0 | ✗ (mechanism D) |
| `dont_generate_typst: {a: 1}` + `-nopdf -nopng` | 1 | 0 | ✗ (mechanism D) |
| `dont_generate_pdf: true`, no flags | 0, PDF skipped | 0, PDF skipped | ✓ |
| `dont_generate_pdf: {a: 1}`, **no flags** | 1 | 0 | ✗ (mechanism D) |

The mechanism is `rendercv_model_builder.py:149-151`: the CLI override dictionary is written into
`input_dict["settings"]["render_command"][key]` **only `if value:`** — truthy-only. `-nopdf` and
`-nopng` (`render_command.py:153-168`) therefore replace whatever the YAML held, mapping included,
*before* pydantic sees it; the other three flags were not passed in the sweep's harness and so did
not. **That is the whole reason the two look exempt: the sweep ran with `-nopdf -nopng`.** With no
flags at all, all five fields are identical (§3.4, measured). A rule of "all `dont_generate_*`
fields behave alike" is correct at the model layer and wrong at the CLI layer, and the porter's
counter-intuitive observation is the CLI layer showing through.

**The caveat that keeps this from being a free pass.** The three ✓ rows agree *vacuously* today:
the port exits 0 for every value of every `dont_generate_*` field, so it would agree with a `0`
whatever it did. The agreement becomes load-bearing only after mechanism D lands, at which point
those same three vectors must **still** exit 0 — i.e. the flag overwrite must happen *before*
validation and only for truthy values. Keep them as regression vectors (§10, `set_nopdf_mapping`).

**One place where the port's `if value:` equivalence is measurably wrong, outside B–F.**
`--typst-path=` (empty value) with `typst_path: from_yaml.typ` in the YAML: upstream exits **1**
with `OS Error: [Errno 21] Is a directory: '<cwd>'` — because typer turns `""` into
`pathlib.Path('.')`, which is truthy, so it overwrites and then the write fails — while the port
exits 0 and keeps the YAML value. This is the path-flag layer, not the boolean one, and it is a
separate defect from B–F; §10.2.

---

## 10. Corpus additions

New cases for `tools/gengolden`, in `testdata/corpus.json`'s `inline_files` form (the shape
`err_unknown_theme` uses), all with `"axis": "errors"`, all invoked as
`render cv.yaml --settings.current_date 2025-03-05`, all `"expect_exit": 1` unless noted. Each
document is the minimal CV plus the named `settings` fragment.

| Case | Fragment | Covers |
|---|---|---|
| `err_settings_pdf_title_list` | `settings: {pdf_title: [a, b]}` | B, `...` input cell |
| `err_settings_pdf_title_int` | `settings: {pdf_title: 42}` | B, scalar input cell |
| `err_settings_output_folder_map` | `render_command: {output_folder: {a: 1}}` | C, planned path |
| `err_settings_png_path_null` | `render_command: {png_path: null}` | C, the null arm |
| `err_settings_design_list` | `render_command: {design: [a]}` | C + F3 + behavior 7.5 |
| `err_settings_dont_generate_typst_map` | `render_command: {dont_generate_typst: {a: 1}}` | D, `bool_type` |
| `err_settings_dont_generate_html_str` | `render_command: {dont_generate_html: abc}` | D, `bool_parsing` |
| `err_settings_current_date_null` | `settings: {current_date: null}` | E |
| `err_settings_not_a_mapping` | `settings: abc` | F1 — **golden is a traceback; unreachable, see below** |
| `err_settings_render_command_null` | `render_command: null` | F2 — same |
| `set_bool_words` | `render_command: {dont_generate_html: "yes"}`, `expect_exit: 0` | D accept-side; the HTML file must be absent from `files.txt` |
| `set_current_date_epoch` | `settings: {current_date: 0}`, `expect_exit: 0` | E2 (§5.4) |
| `set_nopdf_mapping` | `render_command: {dont_generate_pdf: {a: 1}}` + args `-nopdf -nopng`, `expect_exit: 0` | §9's precedence regression |

The two F cases generate a Rich traceback as their golden `stderr.txt`, with this machine's absolute
paths baked in — the exact property that made `err_missing_file` and `err_bad_override_key`
unreachable under D-011/D-014. They are still worth generating: `TestParity` runs both invocations
and catches an exit-code or stream regression even when the byte comparison can never pass. Mark
them in `STATE.md` the way those two are marked, and pin the port's own panel with a unit test
instead.

---

## 11. Out of scope

**11.1 The `pdf_path`/`png_path` write defect.** With `pdf_path: abc` (a bare filename outside the
output folder) upstream renders at exit 0 and the port fails with
`typstc: "<abs>/abc" is outside the document's directory`. Found while measuring C's string row;
it is a compile/write-path defect in `internal/renderer/typstc`, not a validation one, and it is the
only vector in the whole 126-vector sweep where the port refuses a document upstream accepts for a
*path* reason. Needs its own unit; not this delta's.

**11.2 `--typst-path=` and the empty-path flag** (§9's last paragraph) — CLI flag parsing,
iteration 12's.

**11.3 Placeholder expansion** (`OUTPUT_FOLDER`, `NAME_IN_SNAKE_CASE`, `MONTH_NAME`, …,
`render_command.py:8-26`) and `pdf_title`'s own placeholder set (`settings.py:36-49`). This delta
specifies only whether a value is *accepted*, never what it expands to.

**11.4 `bold_keywords`** — mechanism A, closed by commit `cba5b8e`; its dedupe-and-reorder
semantics (`settings.py:54-69`, `list(set(...))`) are the renderer's, iteration 8's.

**11.5 `design`/`locale` overlay *content*** — that a named file exists and parses is spec 002's;
this delta covers only the type of the reference.

---

## 12. Recommended breakdown

Six implementation units. The fan-out is wide because the four value mechanisms are four disjoint
field sets with four disjoint messages, and none reads another's output.

| Unit | Content | Fields | Depends on |
|---|---|---|---|
| B | `pdf_title` is a string | 1 | — · **[parallel]** |
| C1 | the six planned path fields | 6 | — · **[parallel]** |
| D | the five `dont_generate_*` booleans, both messages and the lax accept set | 5 | — · **[parallel]** |
| E | `current_date: null` | 1 | — · **[parallel]** |
| F1 | `settings` must be a mapping | — | — · **[sequential]**, root field declaration |
| F2 | `render_command` must be a mapping | — | F1 · **[sequential]**, same declaration site |
| C2 | `design`/`locale` as nullable path fields | 2 | F2 + behavior 7.5 · **[sequential]** |
| E2 | `current_date` integer timestamps (§5.4) | 1 | E · **[parallel]** after E |

**B, C1, D and E are genuinely independent** — four validators over four disjoint key sets under one
mapping, four disjoint fixture sets, four disjoint strings. They fan out to four porters and never
read each other's output. Each is one commit.

**F1 and F2 are one file's declaration list and must not be parallelized** — they touch the same
root field table, and F2's location (`settings.render_command`) only exists once F1 has declared
`settings` as a model rather than a raw node. Two commits, one owner. Both are small: the
`model_type` message machinery already exists in the binder (§7 reasoning 3), so each is a
declaration plus a fixture.

**C2 depends on F2**, not on C1. C1 is a pure validator over six keys; C2 additionally requires the
CLI to stop resolving overlay files before validation (behavior 7.5), which is a change to the
command's order of operations and therefore on the pipeline spine (`AGENTS.md` §5). It stays with
the owner who lands F1/F2. Splitting C into C1 and C2 is what lets six of the eight path fields fan
out.

**E2 is separate from E and lower priority**: E closes a port-is-laxer defect (the contract's usual
direction); E2 closes a port-is-*stricter* one, which no vector in the sweep flagged because the
sweep only looked for exit-0-where-upstream-exits-1. Landing E2 inside E would bundle two opposite
defects into one commit.

**Fixtures land first and red** (`AGENTS.md` §7): §10's thirteen corpus cases are one commit, before
any of B/C1/D/E. They are generated by `tools/gengolden` from the vendored Python; nothing in §10 is
hand-written.

**Cheapest ordering if run serially:** fixtures → E (one line, one field) → B (one field) → D (the
largest table, but self-contained) → C1 → F1 → F2 → C2 → E2.

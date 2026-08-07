# Iteration 8 — the templater

Behavior of the template engine and its processors, extracted from the vendored Python. No Go
design here; that is `plan.md`.

Upstream: `third_party/rendercv` @ `v2.8` (`2eba248`).
Primary sources: `src/rendercv/renderer/templater/` — `templater.py` (215),
`entry_templates_from_input.py` (514), `date.py` (298), `connections.py` (244),
`markdown_parser.py` (202), `model_processor.py` (189), `string_processor.py` (153),
`footer_and_top_note.py` (123), and `templates/` (384 lines across 25 template files).

---

## 0. Status of this file, stated first

**Incomplete, and it says which parts.** §1–§4 are measured from source I have read end to end.
§6 is a list of what has not been investigated yet, module by module, with what each needs.

Writing the whole thing from a skim was the alternative and it is the failure mode spec 006 §3.2
already recorded once in this project: a plausible claim about a large Python file, cheap to write
and expensive to be wrong about. `tasks.md` cannot be written from this file until §6 is empty.

---

## 1. The environment

1. **One cached environment per input path** (`templater.py:17-47`). `get_jinja2_environment` is
   `functools.lru_cache(maxsize=1)`, so the cache holds exactly one entry and a second input path
   evicts the first. Nothing in a single render changes the path, so this is a performance detail
   — but it means the loader search path is fixed for the whole run.
2. **The loader is a two-element `FileSystemLoader`**, in this order (`:34-41`):
   1. the **input file's directory**, or the process's working directory when there is no input
      file;
   2. the built-in `templates/` directory.

   First hit wins, which is what makes user overrides work.
3. **`trim_blocks` and `lstrip_blocks` are both on** (`:42-43`). This is the largest single source
   of byte differences the port will face, and `AGENTS.md` §6.2 already flags it. Jinja's meaning:
   `trim_blocks` removes the first newline **after** a block tag; `lstrip_blocks` strips whitespace
   from the start of a line **to** a block tag.
4. **Exactly two custom filters** (`:45-46`): `clean_url` and `strip`. `strip` is
   `lambda s: s.strip()`, i.e. Python's whitespace strip on both ends. Everything else a template
   uses — `indent`, `length` — is a Jinja builtin and must match Jinja's behavior, not pongo2's.
5. `clean_url` (`string_processor.py:130-152`) removes `https://` and `http://` **anywhere in the
   string**, not only as a prefix — it is two `str.replace` calls — and then removes **one**
   trailing slash. `https://a/https://b/` becomes `a/b`.

---

## 2. Template resolution

6. **A Typst template is looked up twice** (`templater.py:160-172`). First
   `<theme>/<relative_path>`, then `<file_type>/<relative_path>`; `jinja2.TemplateNotFound` on the
   first is suppressed. Markdown and HTML skip the first lookup entirely.
7. Combined with the loader order of behavior 2, that gives four candidate paths for a Typst
   template and two for the others. The theme-qualified lookup is what lets a user override one
   entry type for one theme.
8. **Every render gets the same four context names** (`:174-180`): `cv`, `design`, `locale`,
   `settings`, plus whatever keyword arguments the caller adds. The callers add
   `section_title`, `snake_case_section_title`, `entry_type`, `entry`, and `html_body`.

---

## 3. How a document is assembled

Measured from `render_full_template` (`templater.py:50-127`). **The separators are as much of the
contract as the templates are**, and they differ between the two formats.

9. The extension is `typ` for `typst` and `md` for `markdown` (`:76-79`).
10. `download_photo_from_url` runs first, then `process_model` — so **the model the templates see
    is a processed copy**, not the validated one. §6.3 owes that processing.
11. Typst gets a preamble and Markdown does not:

    | Format | Opening code |
    |---|---|
    | `typst` | `f"{preamble}\n\n{header}\n"` |
    | `markdown` | `f"{header}\n"` |

12. Each section is `f"{section_beginning}\n{entries_code}\n{section_ending}"`, where
    `entries_code` is the entries joined by **`"\n\n"`** (`:118-121`).
13. Each section is appended as `f"\n{section_code}"` — a leading newline per section, and **no
    trailing newline at the end of the document** beyond whatever the last section ends with
    (`:123`).
14. `render_html` is a different shape: it converts the finished Markdown to HTML and renders a
    single `Full.html` with `html_body` in context (`:129-155`). It is iteration 11's, and it is
    recorded here because it is the third caller of `render_single_template` and the only one that
    is not a fragment.

---

## 4. Two string processors that are measured

15. **`substitute_placeholders`** (`string_processor.py:100-127`) builds one alternation of all
    placeholder names **sorted longest first**, so `YEAR_IN_TWO_DIGITS` matches before `YEAR`. It
    also **`.strip()`s the result**, which a caller substituting into the middle of a longer string
    would not expect. An empty placeholder map returns the string unchanged, without the strip.
16. **`escape_typst_characters`** (`markdown_parser.py:78-142`) is a three-phase transform and the
    order is the whole behavior:
    1. every math span and Typst command is replaced by a dummy name
       `RENDERCVTYPSTCOMMANDORMATH<i>`, so nothing inside them is escaped, and `$$` inside the
       saved text collapses to `$`;
    2. thirteen single characters are escaped through `str.translate` —
       `[ ] \ " # $ @ % ~ _ / > <`;
    3. two **longer** replacements run afterwards, because `translate` is single-character only:
       `"* "` becomes `"#sym.ast.basic "` and a bare `"*"` becomes
       `"#sym.ast.basic#h(0pt, weak: true) "`;
    then the dummy names are put back.

    A lone `"\n"` returns immediately, before any of it.

---

## 4A. `process_model` — what the templates actually see

Measured from `model_processor.py:61-189`. Behavior 10 said the model is a processed copy; this is
what "processed" means.

17. **A deep copy, always** (`:78`). The validated model is never mutated, so the same model can be
    rendered to Typst and then to Markdown and the second render is not compounded on the first.
    **A port that processed in place would produce a correct `.typ` and a doubly-escaped `.md`**,
    and the corpus renders both.
18. **The processor chain is format-dependent and ordered** (`:80-85`):

    | Format | Chain |
    |---|---|
    | both | `make_keywords_bold(_, settings.bold_keywords)` |
    | `typst` only | then `markdown_to_typst` |

    So Markdown output is **not** markdown-parsed — it is already Markdown — and bolding runs
    **before** the Typst conversion in the Typst case, which means the bold markers it inserts are
    Markdown and are converted by the next stage rather than emitted raw.
19. **`cv._plain_name` is captured before `cv.name` is processed** (`:87-90`), and it is what the
    `pdf_title` placeholder `NAME` uses (`:120`) — so the PDF's title carries the **unprocessed**
    name while the header carries the processed one. A port that used one for both would put Typst
    markup in a PDF metadata field.
20. Five things are computed onto the model in this order (`:87-113`): `name`, `headline`,
    `_connections`, `_top_note`, `_footer`. Then `settings.pdf_title` has its placeholders
    substituted (`:115-126`).
21. **An absent `cv.sections` returns early** (`:128-129`), before any section or entry is touched.
22. Per section (`:131-146`): the **title is processed**, then `show_time_span` is computed as
    `section.snake_case_title in design.sections.show_time_spans_in` — which is why spec 006 §3.2
    behavior 15's snake-case coercion has to have run, and where its effect is finally observable.
23. Per entry, **two steps in order**: `render_entry_templates` first, then `process_fields`. So
    the theme's template strings are expanded **before** the string processors run over the result,
    not after.
24. **`process_fields` skips exactly four fields** (`:166`): `start_date`, `end_date`, `doi`,
    `url` — plus anything whose name starts with `_`. Everything else is processed, including
    fields the port has no special knowledge of.
25. `process_fields` reads the field list from `model_dump(exclude_none=True)` and writes back with
    `setattr`, so a `None` field is left alone, a list is processed element-wise, and **a
    non-string non-list value is `str()`-ed first** (`:180-187`) — an integer field comes back as a
    string.
26. A bare string entry — `TextEntry` — is processed directly rather than field-wise (`:168-169`).

---

## 4B. Dates

Measured from `date.py`. This is the half of iteration 7 that spec 007 §4.1 deferred, and it lands
here rather than with the renderer because `process_model` calls it (§4A behavior 23).

27. **Eight placeholders, built from one date and one catalog** (`:12-39`):
    `MONTH_NAME`, `MONTH_ABBREVIATION`, `MONTH`, `MONTH_IN_TWO_DIGITS`, `DAY`,
    `DAY_IN_TWO_DIGITS`, `YEAR`, `YEAR_IN_TWO_DIGITS`. The month lookups are
    `locale.month_names[month - 1]` and `locale.month_abbreviations[month - 1]`, which is the only
    consumer of spec 007's twelve-element lists and the reason their **order** is contractual.
    `YEAR_IN_TWO_DIGITS` is `str(year)[-2:]` — a slice, so year 7 would give `"7"` rather than
    `"07"`.
28. `date_object_to_string` is exactly `substitute_placeholders(single_date_template, …)`, so §4's
    behavior 15 applies: longest-first, and **the result is stripped**.

### The three formatters differ in how they treat a value they cannot parse

29. **`format_date_range`** (`:74-140`): an `int` start or end is stringified as a bare year and
    **not** run through the template — `2020` stays `2020` and does not become `Jan 2020`. An
    `end_date` of `"present"` becomes `locale.present`. Anything else is parsed and formatted.
    There is **no** custom-string fallback here, so an unparseable value raises.
30. **`format_single_date`** (`:143-189`) has one: `get_date_object` failing is caught and the
    value passes through **unchanged**, which is what makes `"Spring 2024"` legal in a publication
    date. The same string in a range would raise.
31. `format_single_date` checks `"present"` **before** parsing, and an `int` before that.

### Time spans, where the arithmetic is upstream's own

32. **`compute_time_span_string`** (`:192-298`) has two branches, and the year-only one is not a
    special case of the other:
    - **Either endpoint an `int`** (`:232-254`): the span is `end_year - start_year`, and a span
      of **less than two years is reported as `1`** — including a span of zero. `MONTHS` and
      `HOW_MANY_MONTHS` are set to the empty string, so the template's month half collapses.
    - **Both full dates** (`:256-298`): `days // 365` years and `(days % 365) // 30 + 1` months.
      **The `+ 1` is unconditional**, so a one-day span is one month. Overflow is then folded —
      `years += months // 12; months %= 12` — which is what stops `1 year 12 months`.
33. Zero years or zero months set **both** the count and the locale word to the empty string, so
    the template collapses rather than printing `0 years`. One uses the **singular**
    `locale.year` / `locale.month`; anything else uses the plural. This is the only consumer of
    spec 007's four singular/plural fields.
34. Every branch ends in `substitute_placeholders`, so the emptied halves leave the surrounding
    template text behind and the final `.strip()` removes only the outer edges — a template of
    `HOW_MANY_YEARS YEARS HOW_MANY_MONTHS MONTHS` with no months yields a **trailing double
    space** collapsed to nothing only at the very end. A port that trimmed each placeholder
    instead would differ by an interior space.

---

## 4C. Markdown → Typst

Measured from `markdown_parser.py`. §4 behavior 16 already covered `escape_typst_characters`; this
is the tree walk around it, and it is where `AGENTS.md`'s choice of `goldmark` needs re-examining.

35. **The parser is python-markdown with five block processors removed** (`:144-156`):
    `hashheader`, `setextheader`, `olist`, `ulist`, `quote` are deregistered, `admonition` is the
    one extension enabled, and `stripTopLevelTags` is `False`.

    So `# Heading`, `1. item`, `- item` and `> quote` are **not** block constructs here — they
    reach `escape_typst_characters` as ordinary text. Any Go Markdown library used as-is would
    parse all four and produce structure upstream does not emit. **This is a spec-level finding
    about the plan**: `AGENTS.md` names `goldmark` for the HTML side, and the Typst side needs a
    parser with the same five constructs off, or a hand-written one.
36. **Every line is converted separately** (`:174-190`), except an admonition block: a line
    starting with `!!!` collects itself plus every following line starting with four spaces, and
    the block is converted as one. The results are rejoined with `"\n"`.

    The reason is in the docstring and it is a real constraint: single-newline-separated lines are
    one paragraph to Markdown, so an unmatched `*` on one line would pair with one on the next. A
    port that converted the whole string at once would emit emphasis across line boundaries.
37. **The element walk** (`:9-70`) maps five tags and ignores the rest:

    | Tag | Output |
    |---|---|
    | `strong` | `#strong[…]` |
    | `em` | `#emph[…]` |
    | `code` | `` `…` `` — the child's **raw text**, not the recursive walk, and not escaped |
    | `a` | `#link("href")[…]`, with `https://example.com` substituted when `href` is missing |
    | `div` | `#summary[…]` with the content `.strip("\n")`ed and inner newlines turned into `" \\ "` |

    Anything else recurses without a wrapper, and a child whose `class` is `admonition-title` is
    **dropped entirely**.
38. Text and **tail** text are both escaped, separately (`:26-27`, `:67-68`). The tail is the text
    after a child's closing tag, which is what keeps `**a** b` from losing its `b`.
39. The two patterns that protect Typst commands from escaping (`:74-75`):
    `#([A-Za-z][^\s()\[]*)(\([^)]*\))?(\[[^\]]*\])?` and `(\$\$.*?\$\$)`. Math is matched
    **first** (§4 behavior 16 phase 1 chains `math_pattern` before `typst_command_pattern`), and
    the saved text has `$$` replaced by `$` — so `$$x$$` in the source becomes `$x$` in the output.

---

## 4D. The footer and the top note

Measured from `footer_and_top_note.py`. Two functions of the same shape with three differences that
all reach the bytes.

40. **Both substitute first and process second**: `apply_string_processors(substitute_placeholders(
    template, placeholders), string_processors)`. So a placeholder's **value** goes through the
    Markdown-to-Typst conversion, not only the template text around it. A port that processed the
    template first would escape the placeholder names.
41. **Both include all eight date placeholders** of §4B behavior 27, plus their own:

    | | Extra placeholders |
    |---|---|
    | top note | `CURRENT_DATE`, `LAST_UPDATED` (which is `locale.last_updated`), `NAME` |
    | footer | `CURRENT_DATE`, `NAME`, `PAGE_NUMBER`, `TOTAL_PAGES` |

    `LAST_UPDATED` exists only in the top note and the two page counters only in the footer — a
    shared placeholder map would make each work in both, which upstream's descriptions do not
    promise.
42. **`NAME` is `name or ""`**, so an absent `cv.name` substitutes the empty string rather than
    leaving the placeholder in place.
43. **The two page placeholders are Typst source, not values**: `PAGE_NUMBER` is
    `#str(here().page())` and `TOTAL_PAGES` is `#str(counter(page).final().first())`. They are
    substituted **before** the string processors run, so `escape_typst_characters` sees them — and
    they survive only because §4 behavior 16's first phase recognizes `#`-commands and holds them
    out. **That is a load-bearing interaction between two modules**, and a port that escaped them
    would emit `\#str(here().page())` into the footer.
44. **The footer is wrapped and the top note is not**: the result is
    `"context {" + " [" + rendered + "] }"`. Note the space after `{` and before `]` — the literal
    is `f"context {{ [{…}] }}"`.

---

## 5. Out of scope

**5.1 The HTML wrapper is iteration 11's** (behavior 14), as is `markdown_to_html`.

**5.2 Compiling the Typst to PDF is iteration 10's.** This iteration ends at a `.typ` string.

---

## 6. Still to investigate — this file is not finished until this list is empty

Each row names the module, its size, and what specifically has to be measured. Nothing here has
been read closely enough to write behavior from.

| Module | Lines | What it owes |
|---|---:|---|
| `entry_templates_from_input.py` | 514 | The largest single unknown. How a theme's template strings become the `main_column` / `date_and_location_column` an entry template reads, and what the UPPERCASE placeholder substitution does with an arbitrary user key. |
| `connections.py` | 244 | How `cv`'s email, phone, website and social networks become the header's connection list, including the `phone_number_format` options and `display_urls_instead_of_usernames`. |
| `templates/**` | 384 | All 25 files, and for each the Jinja constructs it uses — this is what decides how much of `AGENTS.md` §6.1's mechanical transform is actually needed. `EducationEntry.j2.typ` alone uses `splitlines()`, a slice with a computed bound, `|length` and `|indent`. |

---

## 7. Acceptance criteria

Provisional, and they will grow as §6 empties.

- [ ] The loader order of behavior 2 and the double lookup of behavior 6, with a user override of
      one entry type for one theme actually taking effect.
- [ ] `trim_blocks` and `lstrip_blocks` reproduced, proven by byte-identical `.typ` output rather
      than by a unit test of the flags.
- [ ] `clean_url` and `strip`, including behavior 5's two surprises.
- [ ] The assembly separators of behaviors 11–13, which are testable before any template is.
- [ ] `escape_typst_characters`'s three phases in order, including the `$$` collapse and the two
      longer replacements running after `translate`.
- [ ] `substitute_placeholders`' longest-first ordering and its `.strip()`.
- [ ] §4A's ordering: bolding before Typst conversion, entry templates before field processing, and
      `_plain_name` reaching `pdf_title` while the processed name reaches the header.
- [ ] The four skipped fields, and the `str()` of a non-string value.
- [ ] §4B's three formatters, including that only `format_single_date` falls back to the raw
      string, and that a bare year is never run through `single_date`.
- [ ] The time-span arithmetic exactly: `< 2 years` reported as `1`, the unconditional `+ 1`
      month, the overflow fold, and the empty-string collapse for a zero count.
- [ ] §4C's five disabled block processors, proven by a `# Heading` surviving as literal text.
- [ ] Line-by-line conversion, proven by an unmatched `*` on adjacent lines not pairing.
- [ ] The five mapped tags, the dropped `admonition-title`, and tail text surviving.
- [ ] §4D's two placeholder maps, which are **not** the same map, and the `context { [ … ] }`
      wrapper with its exact spacing.
- [ ] The footer's two Typst-source placeholders surviving `escape_typst_characters` — the
      cross-module interaction, tested end to end rather than in either module.

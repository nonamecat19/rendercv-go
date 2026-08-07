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
| `date.py` | 298 | Date formatting end to end — the `single_date`, `date_range` and `time_span` templates, month names and abbreviations from the locale catalog (spec 007 §4.1 sent it here), and the `present` case. |
| `connections.py` | 244 | How `cv`'s email, phone, website and social networks become the header's connection list, including the `phone_number_format` options and `display_urls_instead_of_usernames`. |
| `model_processor.py` | 189 | `process_model` and `process_fields` — which fields are processed, in what order, and how the processed copy differs from the validated model. Behavior 10 depends on it. |
| `markdown_parser.py` | 202 | `to_typst_string`, and the four block processors upstream **deregisters** (`hashheader`, `setextheader`, `olist`, `ulist`, `quote`) plus `stripTopLevelTags = False`. The deregistrations are a strong hint that Go's goldmark cannot be used as-is. |
| `footer_and_top_note.py` | 123 | The two templates that carry `CURRENT_DATE` and `PAGE_NUMBER`. |
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

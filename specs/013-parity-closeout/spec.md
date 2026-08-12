# Spec 013 — Parity closeout

**Status:** draft · **Inherits:** [`specs/000-parity-contract/spec.md`](../000-parity-contract/spec.md)

**Upstream:**
`src/rendercv/schema/sample_generator.py`,
`src/rendercv/schema/sample_content.yaml`,
`src/rendercv/cli/app.py`,
`src/rendercv/cli/entry_point.py`,
`src/rendercv/cli/error_handler.py`,
`src/rendercv/cli/new_command/new_command.py`,
`src/rendercv/cli/new_command/print_welcome.py`,
`src/rendercv/cli/create_theme_command/*`,
`src/rendercv/cli/copy_templates.py`,
`src/rendercv/cli/render_command/progress_panel.py`,
`src/rendercv/cli/render_command/run_rendercv.py`,
`src/rendercv/cli/render_command/render_command.py`,
`src/rendercv/cli/render_command/watcher.py`,
`src/rendercv/exception.py`,
`src/rendercv/__init__.py`,
`pyproject.toml`.

All paths below are relative to `third_party/rendercv/` unless stated. Every measured number in
this spec was taken by running `third_party/rendercv/.venv/bin/rendercv` with cwd in a scratch
directory outside the repository, stdout and stderr redirected to separate files (so the streams
are distinguishable and rich is in its non-tty, 80-column mode — the same mode `tools/gengolden`
captures in).

---

## 1. Purpose

This iteration closes the four surfaces the port either fakes, hard-codes or has never
specified: the **starter-CV generator** behind `new` (currently seven captured fixtures and a
single supported name), the **version-reporting path** (three user-visible places, one of them
inside the starter CV), the **CLI error handler** (two distinct panel paths whose difference is a
single trailing byte, plus five panel bodies no spec names), and **packaging** (what the upstream
distribution declares, and which of those declarations are behavior rather than metadata). It
adds no new pipeline stage; it makes the last hand-waved parts of iterations 12 and 4
mechanically checkable.

It is the iteration that either enumerates or gates the remaining open items iterations 4 and 12
deferred here by name.

---

## 2. Inputs / outputs

**Sample generator.** In: a full name (any string), a theme name, a locale name, and optionally a
destination path. Out: one UTF-8 YAML document, LF line endings, no BOM, ending in a newline. The
document has exactly four top-level keys in the order `cv`, `design`, `locale`, `settings`
(`schema/models/rendercv_model.py:19-42`), preceded by one comment line. The `design` and
`locale` blocks are emitted with their discriminator line live and **every other line commented
out**; `cv` and `settings` are emitted live.

**Version.** In: nothing. Out: one line on stdout.

**Error handler.** In: an exception escaping a command function. Out: bytes on stdout or stderr,
plus a process exit code.

**Packaging.** In: nothing at runtime. Out: the set of files and dependencies the shipped artifact
must contain for the other three to behave. Not directly user-visible; observable through
failures when a data file or a font is missing.

---

## 3. Behavior

### 3.1 The sample generator

1. `create_sample_rendercv_pydantic_model` reads `schema/sample_content.yaml`, takes its `cv`
   key, and constructs a `Cv` from it (`schema/sample_generator.py:69-71`). The sample content is
   a fixed 178-line document; it is not parameterized by anything.
   *Citation:* `schema/sample_generator.py:69-71`, `schema/sample_content.yaml`.

2. **The name substitutes into exactly one place: `cv.name`** (`schema/sample_generator.py:73`).
   It is assigned to the already-constructed model, so no other occurrence of `John Doe`,
   `John_Doe` or `johndoe` in the sample content changes. Measured across four names (`John Doe`,
   `Jane Roe`, `Zoë  Ölçer`, `a`): the rendered documents differ on **line 3 only**.
   *Citation:* `schema/sample_generator.py:73`.

3. The design is `built_in_design_adapter.validate_python({"theme": theme})` and the locale is
   `locale_adapter.validate_python({"language": locale})` — i.e. the theme's and locale's full
   default trees, with nothing from the document (`schema/sample_generator.py:75-77`).
   *Citation:* `schema/sample_generator.py:75-77`.

4. The model is serialized with `model_dump_json(exclude_none=False, by_alias=True)` and reloaded
   through `json.loads` (`schema/sample_generator.py:217-221`). Consequences that are observable:
   `None` fields are **present** (as `key:` with no value), aliases are used as keys, and every
   value has passed through JSON — so a `pathlib.Path` is a string, a colour is its serialized
   `rgb(r, g, b)` form, and an empty list is `[]`.
   *Citation:* `schema/sample_generator.py:217-221`.

5. Key order within each block is the pydantic **field declaration order** of the corresponding
   model, not alphabetical. This is a consequence of behavior 4 and is observable in every line of
   the output.
   *Citation:* `schema/sample_generator.py:217-221`.

6. `dictionary_to_yaml` dumps with ruamel configured as: `encoding = "utf-8"`, `width = 9999`,
   `indent(mapping=2, sequence=4, offset=2)`, and a custom `str` representer that emits a **block
   scalar (`|`)** for any string containing a newline and a plain scalar otherwise
   (`schema/sample_generator.py:35-44`). `width = 9999` means no line in the dump is wrapped;
   `sequence=4, offset=2` means a list item under a key at indent *n* is written with its `-` at
   column *n+2* and its content at column *n+4*.
   *Citation:* `schema/sample_generator.py:35-48`.

7. Scalar quoting is ruamel's, not a rule of RenderCV's. Measured on `cv.name`:

   | name value | emitted as |
   |---|---|
   | `John Doe` | `  name: John Doe` |
   | `A: B` | `  name: 'A: B'` |
   | `*Star*` | `  name: '*Star*'` |
   | `#hash` | `  name: '#hash'` |
   | `  pad  ` | `  name: '  pad  '` |
   | `` (empty) | `  name: ''` |
   | `yes` | `  name: yes` |
   | `line1\nline2` | `  name: \|-` then `    line1`, `    line2` |

   Note the last two rows: `yes` is emitted **unquoted** (ruamel's dumper is YAML 1.2 for
   resolution here), and a multi-line name takes the block-scalar branch of behavior 6.
   *Citation:* `schema/sample_generator.py:35-38`; measured against the vendored library.

8. **The nested-bullet transform.** After dumping, every line matching `^\s+- ` has each occurrence
   of `" - "` **not preceded and not followed by a space** replaced with a newline plus twelve
   spaces plus `- ` (`schema/sample_generator.py:151-159`). The indent is the literal constant 12
   regardless of the line's own indentation. This exists because `sample_content.yaml:75-78` writes
   a nested bullet list as a *plain multi-line scalar* that YAML folds into one line, and
   `width = 9999` keeps it on one line; the regex splits it back apart.
   *Citation:* `schema/sample_generator.py:151-159`, `schema/sample_content.yaml:75-78`.

9. A schema hint is prepended as the first line, terminated by `\n`
   (`schema/sample_generator.py:161-166`). It interpolates `rendercv.__version__`
   (`src/rendercv/__init__.py:3`).
   *Citation:* `schema/sample_generator.py:161-166`.

10. **The design block is commented by string surgery, not by re-serialization.** The dump is split
    on the literal `f"design:\n  theme: {theme}\n"`; everything before it is the `cv` field,
    everything after it up to the literal `f"locale:\n  language: {locale}\n"` is the design body
    (`schema/sample_generator.py:168-179`). Each body line is transformed by
    `f"  {line.replace('  ', '# ', 1)}"` — replace the **first** two-space run with `# `, then
    prefix two spaces. So `  page:` becomes `  # page:` and `    size: us-letter` becomes
    `  #   size: us-letter`. The transformed lines are joined with `\n` and one `\n` is appended.
    *Citation:* `schema/sample_generator.py:168-179`.

11. The locale block is commented by the same rule, bounded below by the literal `"settings:\n"`
    (`schema/sample_generator.py:181-190`).
    *Citation:* `schema/sample_generator.py:181-190`.

12. **The settings block is not commented.** It is `"settings:\n"` plus the remainder of the dump
    verbatim (`schema/sample_generator.py:191`).
    *Citation:* `schema/sample_generator.py:191`.

13. The four pieces are concatenated `cv + design + locale + settings`
    (`schema/sample_generator.py:193-195`) and, if a path was given, written with
    `write_text(..., encoding="utf-8")` (`schema/sample_generator.py:197-198`). The function
    returns the string in both cases.
    *Citation:* `schema/sample_generator.py:193-200`.

14. **Theme and locale are the only two axes, and they are independent.** Measured across the
    nine themes and twenty-two locales: the bytes above `design:` are identical for all nine
    themes, the bytes above `locale:` are identical for all twenty-two locales, and the `settings`
    block is identical for all thirty-one documents (md5 `9cc3cc1a…` on every one).
    *Citation:* measured; `schema/sample_generator.py:75-77` is the reason.

15. **A theme changes the design block's line count, not only its values.** Measured design-block
    line counts: classic 133, ember 128, engineeringclassic 126, engineeringresumes 126,
    harvard 126, ink 128, moderncv 126, opal 127, sb2nov 134. A list-valued option that is empty
    in one theme and one-element in another (`show_time_spans_in: []` versus
    `show_time_spans_in:` + `  - experience`) is one such difference. A generator that only
    substitutes values is wrong.
    *Citation:* measured over all nine themes.

16. `available_themes` and `available_locales` are derived from the discriminated unions, in the
    order the unions are built — `classic` first, then the `other_themes/*.yaml` stems sorted;
    `english` first, then the `other_locales/*.yaml` stems sorted
    (`schema/models/design/built_in_design.py:27-49`, `schema/models/locale/locale.py:27-50`).
    The order is user-visible: it is the order in the two error messages of §4.1 and §4.2 and in
    `new --help`'s option prose (`cli/new_command/new_command.py:33-48`).
    *Citation:* `schema/models/design/built_in_design.py:27-49`,
    `schema/models/locale/locale.py:27-50`.

17. Four sibling generators exist and are **not reachable from the CLI**:
    `create_sample_cv_file`, `create_sample_design_file`, `create_sample_locale_file`,
    `create_sample_settings_file` (`schema/sample_generator.py:285-435`). They share
    `create_sample_yaml_file` (`:236-270`), which applies behaviors 6 and 8 but **not** 9, 10, 11
    or 12 — no schema comment, nothing commented out. `create_sample_settings_file` additionally
    takes `omitted_fields`, popping named keys from the settings dictionary (`:430-432`).
    *Citation:* `schema/sample_generator.py:236-435`.

### 3.2 `new`

18. `new` validates theme, then locale, **before printing anything** — the welcome banner
    included (`cli/new_command/new_command.py:65-79`). Each failure is a `RenderCVUserError`,
    which takes the §3.4 decorator path.
    *Citation:* `cli/new_command/new_command.py:65-77`.

19. Defaults are `theme="classic"`, `locale="english"`
    (`cli/new_command/new_command.py:38`, `:49`).
    *Citation:* `cli/new_command/new_command.py:30-49`.

20. The output path is `pathlib.Path(f"{full_name.replace(' ', '_')}_CV.yaml")` — every space
    becomes an underscore, nothing else is sanitized (`cli/new_command/new_command.py:81`).
    *Citation:* `cli/new_command/new_command.py:81`.

21. The three creatable items are the YAML input file, the Typst templates folder (named after the
    **theme**, so `./classic/` by default) and the Markdown templates folder (always `./markdown/`)
    (`cli/new_command/new_command.py:81-107`). Each is skipped when its path already exists, and
    the YAML file is one item of that same loop — it is **not** overwritten
    (`cli/new_command/new_command.py:112-119`). This is already fixed in the port (`1b4360f`) and
    is restated here because it is the sample generator's own call site.
    *Citation:* `cli/new_command/new_command.py:110-119`.

22. `copy_templates(kind, dest)` copies the whole `renderer/templater/templates/<kind>/` tree,
    excluding `__init__.py` and `__pycache__`, then adds user-write permission to every copied
    file and directory (`cli/copy_templates.py:20-53`). The `chmod` pass exists for immutable
    distributions; it is behavior, not a detail — a copied template must be writable.
    *Citation:* `cli/copy_templates.py:29-53`.

### 3.3 The version-reporting path

23. `__version__` is the literal `"2.8"` (`src/rendercv/__init__.py:3`) and matches
    `pyproject.toml:32`.
    *Citation:* `src/rendercv/__init__.py:3`, `pyproject.toml:32`.

24. `--version` / `-v` is an option on the **root callback**, not a command
    (`cli/app.py:31-33`). It prints `f"RenderCV v{__version__}"` through `rich.print`
    (`cli/app.py:41`). Measured: 14 bytes on stdout, `RenderCV v2.8\n`, zero bytes on stderr,
    exit 0.
    *Citation:* `cli/app.py:31-33`, `:41`; measured.

25. With no `--version` and no subcommand, the callback prints the root help and raises
    `typer.Exit()` — exit 0 (`cli/app.py:42-45`). Measured: 2433 bytes stdout, exit 0. Same bytes
    as `-h`. This is iteration 12's help renderer; only the exit code and the stream belong here.
    *Citation:* `cli/app.py:42-45`; measured.

26. **`__version__` reaches the user in three places, and the port must keep all three in one
    place:** `--version` (behavior 24), the `new` welcome banner
    (`cli/new_command/print_welcome.py:14`), and the schema-hint comment on line 1 of every
    generated starter CV (behavior 9). A version bump that misses any of the three is a byte
    divergence in a golden.
    *Citation:* `cli/app.py:41`, `cli/new_command/print_welcome.py:14`,
    `schema/sample_generator.py:161-166`.

27. `warn_if_new_version_is_available()` runs on **every** invocation, before the version check or
    the subcommand dispatch (`cli/app.py:38`, `:114-139`). It reads a JSON cache under a
    platform-dependent directory (`cli/app.py:48-65`), starts a daemon thread to refresh it when
    absent or older than 86400 s (`cli/app.py:18`, `:124-126`), and prints a yellow notice when
    the cached version is newer than `__version__` (`cli/app.py:133-137`). **`rendercv-go` does
    none of this — D-003, approved.** The user-visible consequence is that upstream's stdout can
    carry a leading notice block that the port never emits; every golden was captured with no
    cache present, which is the branch that prints nothing (`cli/app.py:128`).
    *Citation:* `cli/app.py:18`, `:38`, `:114-139`; `specs/divergences.md` D-003.

### 3.4 The error handler — two paths, one differing byte

28. There is exactly **one** exception class the CLI treats as a clean user error:
    `RenderCVUserError` (`src/rendercv/exception.py:29-31`). `RenderCVUserValidationError`
    (`:34-36`) is handled only inside `run_rendercv`, and `RenderCVInternalError` (`:39-41`) is
    handled nowhere.
    *Citation:* `src/rendercv/exception.py:20-41`.

29. **Path A — the decorator.** `@handle_user_errors` wraps the command function
    (`cli/error_handler.py:11-51`). On `RenderCVUserError` it prints a `rich.panel.Panel` with the
    error's message, title `[bold red]Error[/bold red]`, `title_align="left"`,
    `border_style="bold red"`, through `rich.print` — **which terminates the output with a
    newline** — then raises `typer.Exit(code=1)` (`cli/error_handler.py:39-49`).
    *Citation:* `cli/error_handler.py:39-49`.

30. **Path B — the Live panel.** `ProgressPanel` is a `rich.live.Live`
    (`cli/render_command/progress_panel.py:39`, `:54-65`). `print_user_error` and
    `print_validation_errors` render their panel through `self.update(...)`
    (`:127-134`, `:160-167`); the Live's final render **does not end with a newline**.
    Both then raise `typer.Exit(code=1)` (`:135`, `:169`).
    *Citation:* `cli/render_command/progress_panel.py:120-169`.

31. **Position decides the path, not the error.** Everything raised before
    `with ProgressPanel(...)` at `cli/render_command/render_command.py:231` escapes to the
    decorator; everything raised inside `run_rendercv` is caught by its own handlers and goes to
    the Live. Measured, in bytes on stdout:

    | invocation | bytes | last byte | path |
    |---|---:|---|---|
    | `render empty.yaml` (0-byte file) | 553 | `0a` | A |
    | `render empty.yaml --cv.name=Jane` (odd override, empty file) | 553 | `0a` | A |
    | `new "John Doe" --theme nope` | 638 | `0a` | A |
    | `new "John Doe" --locale nope` | 894 | `0a` | A |
    | `render bad.yaml` (`cv: [`) | 1411 | `af` | B |
    | `render cv.yaml -o <read-only dir>` | 722 | `af` | B |
    | `render cv.yaml` (success) | 880 | `af` | B |

    `af` is the last byte of `╯` (`e2 95 af`) — i.e. the panel's closing corner with nothing after
    it.
    *Citation:* measured; `cli/render_command/render_command.py:231`,
    `cli/error_handler.py:41-48`, `cli/render_command/progress_panel.py:111-118`.

32. **The asymmetry that makes the two paths reachable from adjacent inputs** is
    `collect_input_file_paths`' `contextlib.suppress(RenderCVUserValidationError)`
    (`cli/render_command/run_rendercv.py:113`). It swallows **only** that class, so a YAML
    *syntax* error raised at `:114` falls through into the Live phase and comes out as a
    validation table (path B), while an *empty* file raises `RenderCVUserError`, is not
    suppressed, and reaches the decorator (path A). Any Go equivalent must mirror the clause
    exactly — a broader catch moves errors between paths and changes their last byte.
    *Citation:* `cli/render_command/run_rendercv.py:113-122`.

33. **`new` builds no `ProgressPanel` at all**, so every one of its errors takes path A
    unconditionally (`cli/new_command/new_command.py` contains no `Live`).
    *Citation:* `cli/new_command/new_command.py:64-178`.

34. **`create-theme` is not decorated at all** (`cli/create_theme_command/create_theme_command.py:24`
    has no `@handle_user_errors`), so its `RenderCVUserError`s are unhandled exceptions and take
    §3.5's traceback path. Measured: `create-theme <existing folder>` is **0 bytes stdout,
    1348 bytes stderr, exit 1**, ending in
    `RenderCVUserError: The theme folder "existingtheme" already exists!`;
    `create-theme "Bad Name"` is 0/2369/1.
    *Citation:* `cli/create_theme_command/create_theme_command.py:24`, `:32-34`,
    `cli/create_theme_command/create_init_file_for_theme.py:19-24`; measured.

35. `run_rendercv` catches four things, in this order
    (`cli/render_command/run_rendercv.py:184-198`):

    | caught | becomes |
    |---|---|
    | `RenderCVUserError` | `print_user_error(e)` — path B, message verbatim |
    | `jinja2.exceptions.TemplateSyntaxError` | `print_user_error` with §4.6's composed message |
    | `OSError` | `print_user_error` with `f"OS Error: {e}"` |
    | `RenderCVUserValidationError` | `print_validation_errors(e.validation_errors)` |

    Order matters: `RenderCVUserError` is a `ValueError` and `RenderCVUserValidationError` is a
    different `ValueError`, so neither shadows the other, but `OSError` sits above the validation
    clause and would win for a class that were both.
    *Citation:* `cli/render_command/run_rendercv.py:184-198`.

36. **`--quiet` silences path B and not path A.** `ProgressPanel` is constructed on
    `rich.console.Console(quiet=quiet)` (`cli/render_command/progress_panel.py:63`), so under
    `-q` a validation failure emits **0 bytes** and still exits 1; the decorator's `rich.print`
    uses a different console and is unaffected — `render empty.yaml -q` is still 553 bytes.
    Measured. Already fixed in the port (`cb56ddd`); restated because it is the one behavior that
    distinguishes the two paths without touching an error message.
    *Citation:* `cli/render_command/progress_panel.py:54-65`; measured.

37. `print_user_error` calls `self.clear()` first (`:126`), which empties the completed-step list
    and updates the Live to the empty string; `print_validation_errors` clears only the step list
    (`:147`). Net effect is the same — the subsequent `update` replaces the renderable — so no
    progress row survives either error.
    *Citation:* `cli/render_command/progress_panel.py:126-127`, `:147`.

38. `handle_user_errors` prints **nothing** when `e.message` is falsy, and still exits 1
    (`cli/error_handler.py:40-49`). `print_user_error` substitutes `"An unknown error occurred."`
    in the same situation (`cli/render_command/progress_panel.py:129`). **No upstream raise site
    constructs a `RenderCVUserError` without a message**, so both branches are unreachable from a
    CLI invocation; they are specified so that a Go port does not invent a third behavior.
    *Citation:* `cli/error_handler.py:40`, `cli/render_command/progress_panel.py:129`; raise-site
    inventory in §5.2.

39. A completed run with **no** steps still has a panel body: `print_progress_panel` ends with
    `content = "\n".join(lines) if lines else "Rendering..."`
    (`cli/render_command/progress_panel.py:109`). Already fixed in the port (`5363dc7`).
    *Citation:* `cli/render_command/progress_panel.py:109`.

### 3.5 The traceback path

40. Anything that is not a `RenderCVUserError` escaping a decorated command, and not one of
    `run_rendercv`'s four clauses, reaches typer's `rich.traceback` excepthook: a box-drawn
    traceback with source snippets and absolute filesystem paths, on **stderr**, **0 bytes on
    stdout**, exit **1**. Measured on four vectors:

    | invocation | stdout | stderr | exit |
    |---|---:|---:|---:|
    | `render nope.yaml` (missing input) | 0 | 5312 | 1 |
    | `render .` (directory as input) | 0 | 5292 | 1 |
    | `create-theme <existing>` | 0 | 1348 | 1 |
    | `create-theme "Bad Name"` | 0 | 2369 | 1 |

    *Citation:* measured; `cli/render_command/render_command.py:205` is the read that raises for
    the first two, `cli/create_theme_command/create_theme_command.py:24` the missing decorator for
    the last two.

41. **The whole `RenderCVInternalError` class lands here** — 22 raise sites across the renderer and
    schema layers, none of them caught anywhere. `specs/divergences.md` D-012's closing table lists
    four such documents reachable from ordinary YAML.
    *Citation:* `src/rendercv/exception.py:39-41`; raise-site inventory in §5.2.

42. Usage errors are click's, not RenderCV's, and are the only exit code other than 0 and 1:
    **exit 2**, usage line plus an `Error` panel on **stderr**, 0 bytes stdout. Measured:
    `bogus` → 625 B, `render` (missing argument) → 637 B, `new` → 625 B,
    `create-theme` → 644 B. Iteration 12 owns these; they are listed so §8's exit-code criterion
    is complete.
    *Citation:* measured.

### 3.6 Reachability findings that change what must be implemented

43. **`read_yaml`'s two file-level messages are unreachable from the CLI.** `read_yaml` takes a
    `pathlib.Path | str` (`schema/yaml_reader.py:11`), and only the `Path` branch checks existence
    (`:33-37`) and the extension whitelist (`:39-47`). The only caller in the render path is
    `schema/rendercv_model_builder.py:85`, which passes the **string** contents; the remaining
    callers are internal fixtures (`schema/models/design/built_in_design.py:30`,
    `schema/models/locale/locale.py:30`, `schema/sample_generator.py:70`,
    `schema/pydantic_error_handling.py:21`). Measured consequence: `render ok.txt` on a valid CV
    named with a `.txt` extension **renders at exit 0**, 880 bytes of success panel — the
    extension check never runs, and the missing-file case reaches the traceback of behavior 40
    instead of the `doesn't exist!` message. A port that implements either message reachably is
    *stricter than upstream* and diverges.
    *Citation:* `schema/yaml_reader.py:11`, `:33-47`; `schema/rendercv_model_builder.py:85`;
    measured.

44. The corpus case named `err_not_yaml` does **not** exercise behavior 43 — its `case.json` is
    `render cv.yaml --settings.current_date 2025-03-05` over a malformed document, and its golden
    is a validation table. The name is misleading; no case covers the extension check, because
    none can.
    *Citation:* `testdata/golden/err_not_yaml/case.json`.

### 3.7 The watcher

Iteration 12 §6.2 assigned `--watch` here by name. It is specified, and §7 records the one open
question about it.

45. `--watch` / `-w` is a `bool | None` option (`cli/render_command/render_command.py:169-179`).
    When set, `run_function_if_files_change` is called **inside** the `ProgressPanel` context with
    the resolved file list and a closure over `run_rendercv`
    (`cli/render_command/render_command.py:232-236`).
    *Citation:* `cli/render_command/render_command.py:169-179`, `:232-236`.

46. The watched set is `{str(fp.absolute()) for fp in file_paths}`, and the **parent directories**
    are what is actually scheduled, non-recursively — file-level watching is stated as unreliable
    (`cli/render_command/watcher.py:49-58`).
    *Citation:* `cli/render_command/watcher.py:49-58`.

47. The file list is `collect_input_file_paths`' values: the input file always, plus any of
    `design`, `locale`, `settings` given on the command line, plus `settings.render_command.design`
    and `.locale` resolved relative to the input file's parent when not given on the command line
    (`cli/render_command/run_rendercv.py:99-124`). CLI flags win.
    *Citation:* `cli/render_command/run_rendercv.py:99-124`.

48. The first render happens immediately, before the watch loop
    (`cli/render_command/watcher.py:62-63`), and both it and every subsequent render are wrapped
    in `contextlib.suppress(typer.Exit)` (`:30-31`, `:62`) — **a failing render does not stop the
    watcher and does not set an exit code**.
    *Citation:* `cli/render_command/watcher.py:30-31`, `:62-63`.

49. Only `on_modified` events whose `src_path` is in the watched set trigger a re-run
    (`cli/render_command/watcher.py:24-31`). The loop is `while True: time.sleep(1)` and exits on
    `KeyboardInterrupt`, then stops and joins the observer (`:65-70`). There is no timeout and no
    other exit.
    *Citation:* `cli/render_command/watcher.py:24-31`, `:65-70`.

### 3.8 Packaging

50. The distribution declares exactly one console script:
    `rendercv = "rendercv.cli.entry_point:entry_point"` (`pyproject.toml:84`).
    *Citation:* `pyproject.toml:76-84`.

51. `entry_point` catches `ImportError` on `from .app import app` and writes reinstall guidance to
    **stderr**, then `SystemExit(1)` (`cli/entry_point.py:14-29`). The failure mode is a
    `pip install rendercv` without the `full` extra. **D-004, approved: omitted.**
    *Citation:* `cli/entry_point.py:14-29`; `specs/divergences.md` D-004.

52. Required dependencies: `Jinja2>=3.1.6`, `markdown>=3.10.2`, `phonenumbers>=9.0.24`,
    `pydantic-extra-types>=2.11.0`, `pydantic[email]>=2.12.5`, `ruamel.yaml>=0.19.1`
    (`pyproject.toml:51-58`). The `full` extra adds `typer>=0.24.1`, `watchdog>=6.0.0`,
    `typst>=0.14.8`, **`rendercv-fonts>=0.5.1`** and `packaging>=26.0` (`pyproject.toml:60-67`).
    Everything the CLI does — including the whole of §3.4 — is in the `full` extra.
    *Citation:* `pyproject.toml:51-67`.

53. **`rendercv-fonts` is the only dependency that changes an artifact's bytes.** It supplies the
    font files typst resolves; without the exact set, PDF metrics drift and Axis 1 §1.2 fails.
    The port vendors it under D-007, approved.
    *Citation:* `pyproject.toml:65`; `specs/divergences.md` D-006, D-007.

54. Non-Python data files inside the wheel, by directory:
    21 files under `schema/models/locale/other_locales/`,
    8 under `schema/models/design/other_themes/`,
    2 under `schema/` (`sample_content.yaml`, `error_dictionary.yaml`),
    4 + 9 under `renderer/templater/templates/typst/`,
    3 + 9 under `renderer/templater/templates/markdown/`,
    1 under `renderer/templater/templates/html/`,
    and `renderer/rendercv_typst/{lib.typ,typst.toml}`. The wheel **excludes** the typst package's
    `examples/`, `template/`, `README.md`, `CHANGELOG.md`, `LICENSE` and `thumbnail.png`
    (`pyproject.toml:123-133`) — so the shipped Typst package is two files.
    *Citation:* `pyproject.toml:123-133`; file inventory of `src/rendercv/`.

55. `renderer/rendercv_typst/typst.toml` declares `name = "rendercv"`, `version = "0.3.0"`,
    `entrypoint = "lib.typ"`, `compiler = "0.14.0"`. The version and name are what a generated
    `.typ`'s `#import "@preview/rendercv:0.3.0"` line must agree with — packaging metadata that is
    directly observable in an artifact.
    *Citation:* `src/rendercv/renderer/rendercv_typst/typst.toml`.

56. Commands are registered by **auto-import**: `app.py` globs `*_command.py` under the `cli`
    folder and imports each as `<pkg>.<folder>.<stem>` (`cli/app.py:142-152`). Upstream's own test
    asserts `len(app.registered_commands) == len(command_files)`
    (`tests/cli/test_app.py:24-32`). The observable contract is **exactly three commands**:
    `render`, `new`, `create-theme`. Axis 2 forbids a fourth.
    *Citation:* `cli/app.py:142-152`, `tests/cli/test_app.py:24-32`.

57. `scripts/create_executable.py` (PyInstaller onefile, collecting `rendercv` and
    `rendercv_fonts`) and the `Dockerfile` (`ENTRYPOINT ["rendercv"]`, `CMD ["--help"]`) are
    distribution mechanisms with no CLI-observable behavior of their own. They are named here only
    so §7 can put them out of scope explicitly.
    *Citation:* `scripts/create_executable.py:1-50`, `Dockerfile`.

---

## 4. Exact user-visible strings

Every block below is verbatim. `{…}` marks an interpolation.

### 4.1 Unknown theme to `new` — path A, exit 1

Condition: `theme not in available_themes` (`cli/new_command/new_command.py:65`).

```
Theme {theme} is not available. Available themes are: {", ".join(available_themes)}
```

Rendered in full for `--theme nope` (638 bytes, stdout, trailing newline):

```
╭─ Error ──────────────────────────────────────────────────────────────────────╮
│ Theme nope is not available. Available themes are: classic, ember,           │
│ engineeringclassic, engineeringresumes, harvard, ink, moderncv, opal, sb2nov │
╰──────────────────────────────────────────────────────────────────────────────╯
```

### 4.2 Unknown locale to `new` — path A, exit 1

Condition: `locale not in available_locales` (`cli/new_command/new_command.py:72`).

```
Locale {locale} is not available. Available locales are: {", ".join(available_locales)}
```

Rendered in full for `--locale nope` (894 bytes, stdout, trailing newline):

```
╭─ Error ──────────────────────────────────────────────────────────────────────╮
│ Locale nope is not available. Available locales are: english, arabic,        │
│ danish, dutch, french, german, hebrew, hindi, hungarian, indonesian,         │
│ italian, japanese, korean, mandarin_chinese, norwegian_bokmål,               │
│ norwegian_nynorsk, persian, portuguese, russian, spanish, turkish,           │
│ vietnamese                                                                   │
╰──────────────────────────────────────────────────────────────────────────────╯
```

### 4.3 Unknown theme / locale inside the generator — different wording, unreachable from `new`

`create_sample_yaml_input_file` performs its **own** checks with **different text**
(`schema/sample_generator.py:128-141`). `new` checks first, so these never fire through the CLI;
they fire for a library caller and for `create_sample_design_file` / `create_sample_locale_file`.
Note `{available_themes}` here is a Python **list repr**, not a joined string.

```
The theme {theme} is not available. The available themes are: {available_themes}
```

```
The locale {locale} is not available. The available locales are: {available_locales}. 

But you can continue with `English`, and then write your own `locale` field in the input file.
```

(The second string contains a trailing space after the period on its first line, then `\n\n` —
`schema/sample_generator.py:138-140`.)

### 4.4 The schema hint — line 1 of every starter CV

```
# yaml-language-server: $schema=https://raw.githubusercontent.com/rendercv/rendercv/refs/tags/v2.8/schema.json
```

The `v2.8` is `rendercv.__version__`. **The URL keeps `rendercv/rendercv` — it is an upstream
repository address, not the binary name, so D-001 does not license changing it.**

### 4.5 The `new` welcome banner and links panel — stdout, before the "Get started" panel

```

Welcome to RenderCV v2.8!

╭─ Useful Links ───────────────────────────────────────────────────────────────╮
│ RenderCV App:   https://rendercv.com                                         │
│ Documentation:  https://docs.rendercv.com                                    │
│ Source code:    https://github.com/rendercv/rendercv/                        │
│ Bug reports:    https://github.com/rendercv/rendercv/issues/                 │
╰──────────────────────────────────────────────────────────────────────────────╯
```

The leading blank line is part of it (`cli/new_command/print_welcome.py:14` starts with `\n`).
The label column is `f"{title + ':':<15}"` — the title, a colon, padded to 15
(`cli/new_command/print_welcome.py:22`).

### 4.6 Template syntax error — path B

Condition: a `jinja2.TemplateSyntaxError` escapes any generation step
(`cli/render_command/run_rendercv.py:186-194`).

```
There is a problem with the template ({filename}) at line {lineno}!

{exception}
```

**Not measured end to end.** Reaching it needs a user-supplied broken template, and the port's
templates are pongo2 (D-005), so `{exception}` cannot be Jinja's text. Flagged in §7.

### 4.7 OS error — path B

Condition: any `OSError` inside `run_rendercv` (`cli/render_command/run_rendercv.py:195-196`).

```
OS Error: {exception}
```

Measured with a read-only output folder (722 bytes stdout, no trailing newline):

```
╭─ Error ──────────────────────────────────────────────────────────────────────╮
│ OS Error: [Errno 13] Permission denied:                                      │
│ '{absolute path of the output folder}'                                       │
╰──────────────────────────────────────────────────────────────────────────────╯
```

The message embeds the OS's own `strerror` text and an absolute path. Go's `syscall.Errno` strings
differ from Python's (`permission denied` versus `[Errno 13] Permission denied`), which is a
divergence proposal — §7.

### 4.8 The empty-message fallbacks

Path B (`cli/render_command/progress_panel.py:129`):

```
An unknown error occurred.
```

Path A prints **nothing at all** and exits 1 (`cli/error_handler.py:40`). Both unreachable —
behavior 38.

### 4.9 The empty progress body

`cli/render_command/progress_panel.py:109`:

```
Rendering...
```

### 4.10 The empty-input-file message — path A, 553 bytes, trailing newline

`schema/yaml_reader.py:56`:

```
The input file is empty!
```

### 4.11 The two unreachable file messages

`schema/yaml_reader.py:36` and `:42-46` — behavior 43. Recorded so a port does not implement them.

```
The input file `{path}` doesn't exist!
```

```
The input file should have one of the following extensions: .yaml, .yml, .json, .json5. The input file is {name}.
```

### 4.12 `create-theme`'s two messages — stderr traceback, exit 1

`cli/create_theme_command/create_theme_command.py:33`:

```
The theme folder "{theme_name}" already exists!
```

`cli/create_theme_command/create_init_file_for_theme.py:20-23`:

```
The custom theme name should only contain lowercase letters and digits. The provided value is `{theme_name}`.
```

Both reach the user as the last line of a Rich traceback prefixed `RenderCVUserError: `, on
stderr — not as a panel. Behavior 34.

### 4.13 `entry_point`'s reinstall message — stderr, exit 1, omitted by D-004

`cli/entry_point.py:17-27`, written with `sys.stderr.write` (no panel, no rich):

```

It looks like you installed RenderCV with:

    pip install rendercv

But RenderCV needs to be installed with:

    pip install "rendercv[full]"

Please reinstall with the correct command above.
```

### 4.14 The upgrade notice — omitted by D-003

`cli/app.py:134-136`:

```

A new version of RenderCV is available! You are using v{__version__}, and the latest version is v{latest}.

```

### 4.15 The version line

`cli/app.py:41`:

```
RenderCV v2.8
```

Exactly 14 bytes including the trailing `\n`. **`RenderCV` stays — D-001's own "not licensed by
this" clause.**

### 4.16 The photo-download failure — path B

`renderer/templater/model_processor.py:53-55`. Listed for completeness of the
`RenderCVUserError` inventory; it requires network I/O and is not measured.

```
Failed to download photo from {url}: {exception}
```

---

## 5. Edge cases

### 5.1 From upstream's tests

| Case | Upstream test | Requirement here |
|---|---|---|
| Every theme × every locale builds a valid model | `tests/schema/test_sample_generator.py:20-33` (9 × 22 = 198 parameterizations) | 198 generated documents must each be byte-identical to upstream's |
| Invalid theme and invalid locale both raise | `tests/schema/test_sample_generator.py:35-43`, `:71-77` | §4.1–§4.3 |
| A unicode name survives | `tests/schema/test_sample_generator.py:45-48` (`Matías`) | behavior 2; and the file is UTF-8 with no escaping |
| The returned string equals the written file | `tests/schema/test_sample_generator.py:60-69` | behavior 13 |
| `create_sample_cv_file` yields exactly `["cv"]` and honours the name | `:97-108` | behavior 17 |
| `create_sample_design_file` yields exactly `["design"]` with the right discriminator, for all 9 themes | `:111-131` | behavior 17 |
| `create_sample_locale_file` yields exactly `["locale"]`, for all 22 locales | `:133-152` | behavior 17 |
| `create_sample_settings_file` yields exactly `["settings"]`; `omitted_fields` drops a key | `:155-174` | behavior 17 |
| `dictionary_to_yaml` round-trips a nested dict/list | `:177-194` | behavior 6 |
| `new` × 32 combinations of the five booleans; a pre-existing input file keeps its content; a pre-existing folder is not populated | `tests/cli/new_command/test_new_command.py:12-84` | behavior 21 |
| `new` reports "Available themes are:" / "Available locales are:" on stdout | `tests/cli/new_command/test_new_command.py:86-108` | §4.1, §4.2 |
| `handle_user_errors` returns normally on success, catches `RenderCVUserError`, **re-raises anything else** | `tests/cli/test_error_handler.py:8-30` | behaviors 28–29, 40 |
| `--version` and `-v` both print the version, exit 0 | `tests/cli/test_app.py:36-52` | behavior 24 |
| No arguments prints help, exit 0 | `tests/cli/test_app.py:54-61` | behavior 25 |
| Registered command count equals `*_command.py` file count | `tests/cli/test_app.py:24-32` | behavior 56 |
| `entry_point` exits 1 and mentions `pip install "rendercv[full]"` on `ImportError` | `tests/cli/test_entry_point.py:20-50` | D-004; §4.13 |
| The version cache: missing / corrupt / incomplete / valid / stale, and an unparseable version | `tests/cli/test_app.py:88-319` | D-003 — none of it is ported |
| `copy_templates` excludes `__init__.py` | `tests/cli/test_copy_templates.py` | behavior 22 |

### 5.2 Raise-site inventory (for behaviors 28, 38, 41)

`RenderCVUserError`, 13 sites: `cli/render_command/parse_override_arguments.py:42`, `:49`;
`cli/new_command/new_command.py:70`, `:77`;
`cli/create_theme_command/create_theme_command.py:34`;
`cli/create_theme_command/create_init_file_for_theme.py:24`;
`schema/override_dictionary.py:59`, `:65`, `:71`;
`schema/yaml_reader.py:37`, `:47`, `:57`;
`schema/sample_generator.py:133`, `:141`, `:345`, `:390`;
`renderer/templater/model_processor.py:54`.
**Every one passes a non-empty message**, which is why §4.8's two fallbacks are unreachable.

`RenderCVInternalError`, 22 sites across `renderer/templater/*`, `renderer/pdf_png.py`,
`schema/pydantic_error_handling.py`, `schema/variant_pydantic_model_generator.py`,
`schema/models/design/design.py`, `schema/models/cv/*`, `schema/yaml_reader.py:65`. **Zero catch
sites.** All are behavior 40's traceback.

### 5.3 Sample-generator edge cases not covered by upstream's tests

1. A name that YAML must quote (`A: B`, `#hash`, `*Star*`, leading/trailing spaces, empty) —
   behavior 7's table.
2. A name containing a newline takes the block-scalar branch and produces a **four-line** `cv.name`
   region; the file name still comes from `replace(' ', '_')` and keeps the newline.
3. A name whose text happens to contain `design:\n  theme: {theme}\n` or `settings:\n` would
   corrupt the split at `schema/sample_generator.py:169`, `:184`. **`str.split` is unanchored** and
   the code takes `[0]` and `[1]` unconditionally. This is a latent upstream defect; the port
   should reproduce the split rule rather than a "correct" one, and §7 records it.
4. `below_design = split_yaml_string[0].replace(yaml_design_theme_part, "")` at
   `schema/sample_generator.py:174` is dead — the split already removed the separator.
5. The nested-bullet regex uses a **fixed 12-space** indent (behavior 8), so a bullet at any other
   nesting depth is re-indented to 12 regardless. Reproduce the constant.
6. A design or locale body line that is empty becomes two spaces (`"".replace("  ", "# ", 1)` is
   `""`, prefixed with `"  "`). No current theme or locale produces one; the rule still has to be
   the rule.

### 5.4 Error-handler edge cases

7. Empty file **plus** an odd override count reports the empty file, because
   `collect_input_file_paths` runs at `render_command.py:205` and `parse_override_arguments` at
   `:228`. Measured 553 bytes both ways. Already fixed in the port (`fa12ea5`); it is an ordering
   requirement, not an incidental one.
8. `render <valid CV with a .txt extension>` **succeeds** — behavior 43.
9. `render <directory>` is a traceback, not the `OS Error:` panel: `render_command.py:205` reads
   the file before the `try` in `run_rendercv` ever starts. So the `OSError` clause at
   `run_rendercv.py:195` is **not** reachable through the input file; it is reachable through the
   output tree (§4.7's measurement).
10. `-q` and a validation failure: 0 bytes, exit 1. `-q` and an empty file: 553 bytes, exit 1.
11. A `create-theme` failure writes **nothing to stdout** — the port's clean panel is a divergence,
    §7.

---

## 6. Ordering and whitespace guarantees

1. **Starter CV.** Exactly one `\n` at end of file. LF only. UTF-8, no BOM, no escaping of
   non-ASCII (measured: `norwegian_bokmål` and `—` appear raw). No line is wrapped (`width = 9999`).
   Block order `cv`, `design`, `locale`, `settings`. Comment prefix is exactly `  # ` for a
   depth-1 key and `  # ` + (2 × (depth − 1)) spaces below that — the mechanical consequence of
   behavior 10, not an independent rule.

2. **Path A output** ends with exactly one `\n` after the panel's `╯`. **Path B output ends with
   `╯` and nothing else.** This is one byte and it is the whole of §3.4's contract.

3. `new` writes, in order: the banner's leading `\n`, the welcome line, a blank line, the Useful
   Links panel, then the Get started panel — each `rich.print`, so each newline-terminated.

4. Traceback output is stderr-only and stdout is **exactly 0 bytes** for those vectors; the
   validation and error panels are stdout-only and stderr is **exactly 0 bytes**. There is no
   vector in the measured set that writes to both.

5. Exit codes: 0 success / `--version` / help; 1 every `RenderCVUserError`, every validation
   failure and every unhandled exception; 2 every click usage error. **There is no other code.**

6. The commented design and locale blocks preserve the dump's own key order; commenting is a
   per-line text transform and never re-orders or re-serializes.

---

## 7. Out of scope

**7.1 D-003 and D-004 stay as approved.** The PyPI version check and the `[full]`-extra guidance
are not implemented. `tests/cli/test_app.py:64-319` — 255 lines, the largest single test class in
upstream's CLI suite — is deliberately unported.

**7.2 Distribution mechanisms.** PyInstaller (`scripts/create_executable.py`) and the Dockerfile
have no CLI-observable behavior. Cross-compiled release artifacts are already a `STATE.md` stretch
goal, not a gate.

**7.3 The remaining `email-validator` message surface** that spec 004 §7.4 carried here is **not
enumerated by this iteration.** 004 §7.4 offers two options — enumerate, or propose the divergence
and take the human gate. Enumerating 822 lines of a third-party library's message catalogue is a
subsystem, not a sub-task of parity closeout, and it is orthogonal to everything else here.
**Recommendation: it becomes its own iteration.** This spec does not silently drop it and does not
claim it closed; §10 lists it as the one inherited item this iteration explicitly declines.

**7.4 The trailing-newline blindness in the harness** (`internal/conformance/conformance.go:241-248`
and `tools/gengolden/main.go:317-324` both append `\n` to *both* sides) is **not** fixed here.
Un-blinding it means regenerating `testdata/golden`, which is human-gated (AGENTS.md §5).
`internal/cli/panelnewline_test.go` remains the only thing holding §6.2. This iteration's
differential gate (§8) sees the byte regardless, because it compares raw process output.

**7.5 Tagged YAML** is D-012 and iteration 15's; **Lua custom themes** are D-002 and iteration
14's. Neither is touched.

**7.6 The `caseWorkDir` shared-path fragility** and the operational rule that the parity suite must
not run concurrently with anything else are recorded in `STATE.md` and gated. This iteration's
differential harness must therefore run **outside** `testdata/`, in a scratch directory, the way
iteration 15's did.

---

## 8. Acceptance criteria

Each is mechanically checkable. "Differential" means: run the vendored
`third_party/rendercv/.venv/bin/rendercv` and `bin/rendercv-go` on the same input, in two separate
scratch directories outside the repository, and compare raw stdout, raw stderr, the exit code and
the written file tree — **with no trailing-newline normalization** (§7.4).

**A differential compares the port against a process, never against a digest of one.** A captured
fixture — an md5 table, a recorded output — is not a differential: it moves only when someone
reruns the generator that wrote it, so it cannot notice upstream changing under a submodule bump.
Where an invocation-per-case is too slow to be a routine gate, the vendored library may be driven
once per test run and asked for every case at once, and one process-level invocation then bridges
the two levels by showing the CLI writes what the library returned. Live is the requirement;
one-process-per-case is not.

### Sample generator

- [ ] A 198-case **live** differential over every (theme, locale) pair: the document the port
      generates is **byte-identical** to the one the vendored Python generates on that run,
      including the final byte, and the whole text crosses the boundary so a mismatch names the
      line. `internal/cli/sample/upstream_conformance_test.go`, driving
      `internal/cli/sample/testdata/upstream.py`; 198 cases in ~5s of one interpreter.
- [ ] A live name differential over at least the eight rows of behavior 7's table plus `Matías`
      (`tests/schema/test_sample_generator.py:46`): the `cv.name` region byte-identical to what
      ruamel emits on that run, **and** a byte-identical file name.
- [ ] One process-level case runs upstream's `new` in a scratch directory and asserts the file it
      writes — name and bytes — is the document the 198-case battery claims for that pair, so the
      library-level battery stands for what the CLI does.
- [ ] `internal/cli/samples/*.yaml` is **deleted**, and no *fixture* in the port holds a captured
      starter CV or a digest of one — the two that did, `internal/cli/sample/testdata/matrix.json`
      and `names.json`, are gone with the criteria they served.
      `internal/cli/sample/blocks/**` **stays and is not a fixture**: it is the pydantic dump the
      generator runs on (§3.1 behavior 14's 33 blocks), embedded *data* rather than an
      expectation, and the live differential above is what holds it to upstream.
      The earlier form of this bullet asked for `tools/sampleprobe` to be deleted too, on the
      reading that it existed only to write those fixtures. It does not: it is also the sole
      generator of `blocks/**`, so deleting it would remove the regeneration path for embedded
      production data after a submodule bump. **The tool stays; whether that regeneration path is
      worth keeping now that the differential is live is a human gate, unresolved.**
- [ ] `ErrSampleNameUnsupported` (`internal/cli/new.go`) is deleted and no invocation of `new`
      can produce it.
- [ ] A unit test asserts the block order `cv`, `design`, `locale`, `settings` and that exactly
      one `\n` terminates the file.
- [ ] A unit test asserts behavior 8's fixed 12-space nested-bullet indent on a synthetic line at
      a different nesting depth.
- [ ] A unit test asserts behavior 10's comment transform on the six shapes of §5.3.6 —
      including the empty line.

### Version

- [ ] `--version` and `-v` each produce exactly `RenderCV v2.8\n` on stdout, 0 bytes on stderr,
      exit 0. Differential.
- [ ] A single Go constant feeds all three sites of behavior 26; a test asserts the constant
      appears in the `new` banner and on line 1 of a generated starter CV.
- [ ] The bare invocation exits 0 with 0 bytes on stderr (behavior 25).

### Error handler

- [ ] The seven-row table of behavior 31 reproduces: byte count, last byte, stream and exit code,
      differential, for all seven.
- [ ] `-q` differential: 0 bytes for a validation failure, 553 bytes for an empty file, exit 1
      both (behavior 36).
- [ ] A test enumerates the port's `RenderCVUserError` equivalents and asserts each carries a
      non-empty message (behavior 38 / §5.2), so the two fallback strings stay unreachable.
- [ ] `Rendering...` appears for a run that completes no steps (behavior 39). Differential on
      `-notyp -nopdf -nopng -nomd -nohtml`.
- [ ] `render <valid CV>.txt` exits **0** and renders (behavior 43). Differential. This criterion
      fails if the port implements §4.11's messages.
- [ ] Neither of §4.11's two strings appears anywhere in the port's source.
- [ ] Exit-code inventory test: no code other than 0, 1, 2 is producible (behavior 42 / §6.5),
      including the current `70` sentinel being unreachable.

### Watcher

- [ ] `--watch` performs the first render and then blocks; the first render's stdout is
      byte-identical to the same invocation without `--watch`.
- [ ] A failing first render under `--watch` does **not** exit (behavior 48) — checked by killing
      the process after a bounded wait and asserting it was still running.
- [ ] The watched set equals `collect_input_file_paths`' values (behavior 47), asserted as a unit
      test on the resolver, not on the observer.

### Packaging

- [ ] Exactly three commands are registered (behavior 56); a test asserts the count and the names.
- [ ] The embedded data-file inventory matches behavior 54's counts: 21 locales, 8 themes,
      13 typst templates, 12 markdown templates, 1 html template, `sample_content.yaml`,
      `error_dictionary.yaml`.
- [ ] The Typst package name/version the emitter writes agrees with
      `renderer/rendercv_typst/typst.toml` (behavior 55).
- [ ] `copy_templates`' write-permission pass is reproduced: every file `new
      --create-typst-templates` writes is user-writable (behavior 22).

---

## 9. Corpus additions

**Recommended gate is the differential harness of §8, not new goldens.** Adding 198 `new_*` cases
to `testdata/golden/` is a golden regeneration and therefore human-gated (AGENTS.md §5), and the
goldens already bake absolute paths and a generation month (iteration 1's audit). The differential
compares live processes and sees the trailing byte §7.4 blinds the goldens to.

If the human gate is taken anyway, the minimal useful set is 31 cases — one per theme and one per
locale — replacing the current seven:

| case | invocation |
|---|---|
| `new_theme_<t>` × 9 | `new "John Doe" --theme <t>` |
| `new_locale_<l>` × 22 | `new "John Doe" --locale <l>` |
| `new_name_quoted` | `new "A: B"` |
| `new_name_unicode` | `new "Matías"` |
| `cli_render_txt_extension` | `render cv.txt` over a valid CV — pins behavior 43 |
| `cli_render_all_disabled` | `render cv.yaml -notyp -nopdf -nopng -nomd -nohtml` — pins §4.9 |

`tools/gengolden` would need `new`'s working directory captured (it writes into cwd) and the
generated file added to `files.txt`; today only `render` cases capture a file tree
(`tools/gengolden/main.go:508`).

---

## 10. Open findings — what this iteration closes and what it does not

### Closes

| Finding | Source | How |
|---|---|---|
| `new` accepts only the literal name `"John Doe"` | STATE pass 22, human-gated divergence proposal | §3.1 behavior 2 — a real generator; the proposal is withdrawn, not approved |
| Only 7 of 198 theme/locale variants exist | `tools/sampleprobe/main.go` header | §8's 198-case live differential |
| The two `RenderCVUserError` panel paths were three ad-hoc fixes with no spec | STATE 2026-08-11 (`fa12ea5`, `504c91a`, `cb56ddd`) | §3.4 behaviors 28–39, §6.2 |
| Five panel bodies no spec named (`OS Error:`, template syntax, two empty-message fallbacks, `Rendering...`) | none | §4.6–§4.9 |
| The `--watch` stub returning "not implemented" | STATE G-10, spec 012 §6.2 | §3.7 |
| The version string had no specified source of truth | none | §3.3 behavior 26 |

### Does not close, and why

| Finding | Why not |
|---|---|
| The email-validator message residual | §7.3 — recommended as its own iteration; explicitly declined here, not dropped |
| Golden trailing-newline blindness | §7.4 — human gate on regenerating `testdata/golden` |
| `caseWorkDir` shared-path fragility | §7.6 — same gate |
| D-011's tracebacks | Generalized, not closed — see the proposal below; still a divergence forever |
| Tagged YAML (D-012), Lua themes (D-002) | §7.5 |
| The Python numeric-`repr` gap for floats | Iteration 15's open item; no contact with this subsystem |

### Divergence proposals — **each needs the human gate before any code lands**

`specs/divergences.md` is human-gated (AGENTS.md §5). The following are *proposed*, not written.

**P-1 — Generalize D-011 from two goldens to the class.** D-011 names `err_missing_file` and
`err_bad_override_key`. Behavior 40 measures four more vectors and §5.2 counts 22 uncatchable
`RenderCVInternalError` sites plus every undecorated `create-theme` failure. The entry should say:
*any* upstream unhandled exception produces a Rich traceback on stderr with this machine's absolute
paths, which a Go binary cannot reproduce; `rendercv-go` reports the nearest clean error at the
same exit code and the same stream where it can. Naming two cases understates the surface, and the
current text reads as though the class were bounded.

**P-2 — `create-theme` prints a panel where upstream prints a traceback.** Behavior 34, measured:
upstream is 0 bytes stdout / 1348 bytes stderr; the port prints a clean stdout panel. Already
flagged human-gated in STATE pass 22 and still unwritten. It is a special case of P-1 but has its
own two exact messages (§4.12) and its own stream inversion, so it is proposed separately.

**P-3 — `OS Error:` interpolates the OS's own `strerror`.** §4.7. Python writes
`[Errno 13] Permission denied: '<path>'`; Go's `os.PathError` writes
`<op> <path>: permission denied`. Byte parity is not achievable without hand-mapping errno text
per platform. Proposal: reproduce the `OS Error: ` prefix and the panel, accept the message body's
divergence, record it.

**P-4 — §4.6's template-syntax message cannot carry Jinja's text.** D-005 already says template
*source* diverges; it does not say a template *error* does. The port's templates are pongo2, so
`{filename}` and `{lineno}` can match but `{exception}` cannot. Proposal: extend D-005 with the
error-text consequence rather than open a new number.

---

## 11. What could not be determined without running the suite

Stated rather than guessed, per the brief.

1. **§4.6's rendered panel.** Reaching a `TemplateSyntaxError` needs a deliberately broken
   user-supplied template; it was not constructed. The message *template* is read off
   `run_rendercv.py:188-193`; the *rendered bytes* are unmeasured.

2. **§4.16's photo-download failure.** Requires network I/O. Unmeasured.

3. **The exact stderr bytes of any traceback.** Behavior 40's byte counts are this machine's, and
   D-011 already records that they are non-reproducible even for upstream. Only the *stream*, the
   *empty stdout* and the *exit code* are portable facts.

4. **Whether the 198-case differential currently passes for any pair beyond the 7 captured.** The
   port's `new` errors out for anything else, so the comparison has never been run. §8's first
   criterion is expected to land red.

5. **Whether `write_text` on a name whose `replace(' ', '_')` result is not a legal filename**
   (an embedded `/`, a NUL, a name longer than `NAME_MAX`) produces the same failure on both
   sides. `new` performs no sanitization (behavior 20) and the resulting `OSError` is **not**
   inside any handler in `new`'s call path, so it is behavior 40's traceback upstream — but this
   was not measured, and STATE pass 18 already recorded a related ENAMETOOLONG asymmetry in
   `design.py`'s theme-folder path. Needs measurement before the port picks an answer.

6. **`rich`'s behavior under a real TTY.** Every measurement here is non-tty, 80 columns, no
   colour — the mode `tools/gengolden` captures. Whether path B emits a trailing newline when
   attached to a terminal is unmeasured and, since no golden is captured that way, out of the
   contract.

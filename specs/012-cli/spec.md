# Iteration 12 — the CLI

Behavior of `rendercv`'s command-line surface, extracted from the vendored Python and from the
corpus goldens it generated. No Go design here.

Upstream: `third_party/rendercv` @ `v2.8` (`2eba248`).
Primary sources: `src/rendercv/cli/**`, `src/rendercv/renderer/path_resolver.py`.

---

## 0. What this iteration is, and why it is the gate for 42 cases

**Every red case in the parity suite is red because this does not exist.** The suite shells
`rendercv-go` and compares exit code, stdout, stderr and the output tree; with no binary behavior
behind those commands, all 42 fail identically. Iterations 9 and 11 made the *artifacts*
byte-identical — 72 comparisons — through a test harness that calls the renderer directly. This
iteration is what puts those artifacts behind the command that produces them.

The 42 split three ways:

| Axis | Count | What they need |
|---|---|---|
| `artifacts` | 14 | this iteration **and** iteration 10 — each writes a PDF and PNGs |
| `cli` | 21 | this iteration alone, except where a PDF is written |
| `errors` | 7 | this iteration plus the error-pipeline work of iteration 4, which is done |

**Six of the 21 `cli` cases are help and version text** and are the hazard of §5.

---

## 1. The command surface

1. Three subcommands: `render`, `new`, `create-theme` (`cli/__init__.py`, and the `--help` golden).
2. The root takes `--version` / `-v` and `--help` / `-h`. Typer's own
   `--install-completion` / `--show-completion` appear in the help panel.
3. **`-h` is a real alias for `--help`** — `cli_help_short` and `cli_help` have identical stdout.
4. The binary name is the one sanctioned divergence (`AGENTS.md` §1), so every golden's
   `rendercv` becomes `rendercv-go` in the port's own output. That substitution is the *only*
   permitted textual difference in this iteration's stdout.

## 2. `render`

5. `render <input.yaml>` writes, by default, `.typ`, `.pdf`, `.png`(s), `.md` and `.html` under
   `./rendercv_output/`, named from the CV's name — `John_Doe_CV.typ`.
6. **Every option has both a long and a short spelling, and the corpus only ever uses the short
   one.** The inventory below is read off the signature rather than off the goldens, because
   `render_typst_only` and `render_custom_paths` between them exercise ten of the seventeen and
   name no long form at all. Each row cites
   `third_party/rendercv/src/rendercv/cli/render_command/render_command.py`.

   | Long | Short | Kind | Line |
   |---|---|---|---|
   | `--output-folder` | `-o` | path | 33 |
   | `--design` | `-d` | path | 44 |
   | `--locale-catalog` | `-lc` | path | 52 |
   | `--settings` | `-s` | path | 60 |
   | `--typst-path` | `-typ` | path | 68 |
   | `--pdf-path` | `-pdf` | path | 79 |
   | `--markdown-path` | `-md` | path | 90 |
   | `--html-path` | `-html` | path | 101 |
   | `--png-path` | `-png` | path | 112 |
   | `--dont-generate-markdown` | `-nomd` | bool | 123 |
   | `--dont-generate-html` | `-nohtml` | bool | 134 |
   | `--dont-generate-typst` | `-notyp` | bool | 142 |
   | `--dont-generate-pdf` | `-nopdf` | bool | 153 |
   | `--dont-generate-png` | `-nopng` | bool | 161 |
   | `--watch` | `-w` | bool | 169 |
   | `--quiet` | `-q` | bool | 180 |
   | `--YAMLLOCATION` | — | dummy | 190 |

   **The short forms are whole words, not GNU shorthands** — `-typ`, `-nopdf`, `-lc`. Neither
   pflag nor getopt accepts that spelling, and it is the reason args.go exists.

   **`--YAMLLOCATION` is never read.** It is a parameter declared solely so the help panel has a
   row describing the dotted-override mechanism of behavior 9; the function binds it to `_`.

7. **Three of the options name overlay files** — `--design`, `--locale-catalog` and `--settings`
   (`render_command.py:205-215`). Each is read and merged ahead of the main document in the fixed
   order settings, design, locale. No corpus case passes one, which is why they can be absent from
   a port that passes every corpus case.
8. `--quiet` suppresses the progress output but not the result panel (`render_quiet`).
9. **Arbitrary dotted overrides**: any `--<dotted.path> <value>` sets that field in the parsed
   document before validation. Three shapes appear in the corpus:
   - scalar: `--cv.phone +1-555-555-5555`
   - indexed: `--cv.sections.education.0.institution MIT`
   - discriminator: `--design.theme moderncv`
10. **An unknown override key is a validation error, not a CLI error** (`err_bad_override_key`):
    the value is set, and the model then rejects the extra key with the error pipeline of
    iteration 4.
11. `--settings.current_date` is an ordinary override by rule 9 — `tools/docprobe` already relies
    on it, so the mechanism is exercised before this iteration begins.

**11a. The override collector is not a dotted-key filter, and this is the behavior the port got
wrong.** `render` is declared `context_settings={"allow_extra_args": True,
"ignore_unknown_options": True}` (`render_command.py:26`), so *everything* click does not
recognize — unknown flags, stray positionals, single-dash tokens — lands in `ctx.args` in order,
and `parse_override_arguments` (`parse_override_arguments.py:26-55`) reads that list as
alternating keys and values. Three consequences, each measured against the vendored CLI:

| Input | Upstream |
|---|---|
| `render cv.yaml --nope value` | accepted as the override key `nope`, then rejected by the model as an unknown field |
| `render cv.yaml a b c` | `There is a problem with the extra arguments (a,b,c)! Each key should have a corresponding value.` |
| `render cv.yaml -x value` | `The key (-x) should start with double dashes!` |

Both messages are an `Error` panel on **stdout** with exit 1, not a usage error. The
odd-count message joins the arguments with `,` and **no space** (`:39`).

**11b. The key strips every `--`, not the prefix.** `key.replace("--", "")`
(`parse_override_arguments.py:51`) is unanchored, so `--cv--name` becomes `cvname`. Measured:
upstream accepts the argument and the model then rejects `cvname` as an unknown field.

**11c. A missing required argument is a *usage* error, not a RenderCV error.** `rendercv render`
with no input file, `rendercv new` with no name, `rendercv create-theme` with no theme name and
`rendercv bogus` all write to **stderr** and exit **2** — a different stream, panel and code from
every `err_*` golden. The shape is three parts:

```
Usage: rendercv render [OPTIONS] INPUT_FILE_NAME
Try 'rendercv render -h' for help.
╭─ Error ──────────────────────────────────────────────────────────────────────╮
│ Missing argument 'INPUT_FILE_NAME'.                                          │
╰──────────────────────────────────────────────────────────────────────────────╯
```

with `No such command 'bogus'.` and the root's own usage line for an unknown command. **`rendercv`
with no arguments at all is different again**: the full help on stdout, exit **0**, which is
behavior 3's help renderer and therefore blocked on §5.

### 2.1 Every path parameter is checked for readability at parse time

Added 2026-08-11, from measurement. The spec was silent on this and the port does not implement it.

11a. **Typer's default conversion for a `pathlib.Path` annotation is
`click.Path(exists=False, readable=True, dir_okay=True)`.** So every path parameter `render` declares
is checked for **readability** at parse time and **none** is checked for existence. `render` declares
ten: the `INPUT_FILE_NAME` argument, `--design`/`-d`, `--locale-catalog`/`-lc`, `--settings`/`-s`,
`--output-folder`/`-o`, and the five output paths `--typst-path`/`-typ`, `--pdf-path`/`-pdf`,
`--markdown-path`/`-md`, `--html-path`/`-html`, `--png-path`/`-png`.

An unreadable path — mode 000, file or directory — is a **click usage error**: exit **2**, usage line
plus `Try 'rendercv render -h' for help.` plus the `Error` panel, all on **stderr**, stdout 0 bytes.
Measured on all ten. `Invalid value for '--design' / '-d': Path 'unreadable.yaml' is not readable.`
is 637 B; the longer option names wrap to two panel lines at 722 B.

11b. **Missing is not checked**, because `exists=False`. `render cv.yaml -d nosuch.yaml` is exit **1**
with a 4255 B `FileNotFoundError` traceback — click does nothing and the unguarded `read_text` fails
later. **A uniform "validate the path" rule is therefore wrong**, and this is the vector that proves
it.

11c. **The message names both spellings, long then short, whichever the user typed** — including the
`=` form and inside a short cluster (`-dunreadable.yaml`). The argument has its own shape with no
slash: `Invalid value for 'INPUT_FILE_NAME':`.

11d. **Order: options in the order they were typed, then positional arguments**, because
`_process_args_for_args` runs after the option loop. Measured, and this is the part an implementation
is most likely to get wrong by validating the input file first because it feels primary:

| invocation | reports |
|---|---|
| `render unreadable.yaml -d u2.yaml` | `--design`, though the input was typed first |
| `render cv.yaml -s u2.yaml -d unreadable.yaml` | `--settings`; reverse the two and it reports `--design` |
| `render -d unreadable.yaml` (no input file at all) | the readability error, **not** `Missing argument 'INPUT_FILE_NAME'.` |
| `render --nope -d unreadable.yaml` | the readability error — so this precedes the leftover-token routing |

11e. **Consequence for the input file's failure taxonomy**: unreadable is a usage error on stderr at
exit 2, while missing stays the panel on stdout at exit 1. That split is upstream's own, so
reproducing it is parity rather than divergence. It does not disturb `err_missing_file` or
`err_bad_override_key`, whose files are ordinary readable ones.

*Citation:* measured against `third_party/rendercv/.venv/bin/rendercv` at `COLUMNS=80`, uid 1000,
mode-000 targets; typer's `Path` conversion; click's `_process_args_for_args`.

## 3. `new` and `create-theme`

12. `new "John Doe"` writes a starter `John_Doe_CV.yaml` in the working directory.
13. `--theme <name>` and `--locale <name>` pick the sample's theme and locale; an unknown locale
    is an error (`err_unknown_locale`), so the name is validated against the 22-member union.
14. `--create-typst-templates` additionally writes the theme's Typst templates beside the input, so
    a user can edit them — the override path spec 008 §2 already loads from.
    **`--create-markdown-templates` is its companion** (`new_command.py:57`) and does the same for
    the Markdown template set. It has no corpus case and the port does not declare it, so a user
    passing it gets a parser error where upstream writes files.
15. `create-theme <name>` writes a theme folder of Typst templates to customize — fourteen files:
    the four top-level fragments, the nine entry templates, and `__init__.py`.

**This command cannot be byte-identical, and the reason is structural rather than a defect.**
Two of its outputs are things the port deliberately does not have:

- **`__init__.py` is Python.** A custom theme executes it at validation time, which is D-002 in
  `specs/divergences.md`: the port scripts custom themes in Lua instead. So the file this command
  writes must be `init.lua`, or the feature it exists for does not work.
- **The `.j2.typ` files are Jinja source.** The port ships the pongo2 transform of them
  (`AGENTS.md` §6.1 sanctions the source diverging while the *output* must not), and the loader
  reads that form. Measured on `Header.j2.typ`: upstream's has a newline after `{% macro image() %}`
  that `trim_blocks` removes at parse time, and the port's transform has already removed it.
  Emitting upstream's bytes would produce a theme this binary renders **differently** from the
  theme it just wrote.

So `create_theme`'s golden is unreachable by construction, and the honest resolution is a
`divergences.md` entry naming both files — which is human-gated, so this spec records it and stops.
The *panel* is reachable and is a second shape beyond the result panel: multi-line body text with
blank rows.

## 4. Exit codes and the error surface

16. A successful render exits **0**; the seven `errors` cases exit **non-zero** with their message
    on **stderr** and nothing on stdout.
17. A missing input file (`err_missing_file`) and a non-YAML input (`err_not_yaml`) are
    distinguishable messages, not one generic failure.
18. The validation-error text is iteration 4's, already byte-compared by the 25-record
    differential. What is new here is only *where* it goes and what the process exits with.

## 5. The hazard: the help text is Rich's, not a flag list

19. Every help golden is a **Rich-rendered panel**: box-drawing characters, a fixed 80-column
    width, centred titles filled with `─`, and text wrapped to the panel's inner width.
    `cli_help`'s stdout is 2433 bytes of it.
20. The result panel of a successful render is the same machinery (`render_typst_only`): a
    `╭─ Your CV is ready ─…─╮` header, one `│ ✓ <duration> Generated Typst: … │` row per artifact,
    and a closing `╰─…─╯`. **The duration is normalized by the harness**, so the timing is not
    part of the contract but the column it sits in is.
21. Typer's option table has its own column layout: flag, short flag, then help text wrapped in the
    remaining width, with continuation lines aligned under the help column.

**This is the iteration's real work**, and it is a rendering problem rather than a CLI problem.
cobra will not produce it; no Go CLI library will. What produces it is a small panel renderer that
takes a title, rows and a width, and reproduces Rich's box drawing — which is deterministic and
therefore portable, unlike anything about Rich's *styling*.

**Measured, for the result panel at least**: it is *one* shape — a title, a row per artifact, a
fixed 80 columns. Its geometry is now pinned by a unit test, including the duration column that the
harness erases and that therefore cannot be read off the golden directly. The six help panels are
unmeasured; the note below still applies to them.

**The measurement that must precede the design** — this is spec 011 §6's lesson, applied before
the fact rather than after: count how many distinct panel shapes the six help goldens and the
result panels actually use. If it is two or three, this is a small renderer with a fixture per
shape. If it is a general layout engine, that is a different estimate, and it must come from
counting rather than from reading one golden and extrapolating.

## 6. Out of scope

**6.1 PDF and PNG** are iteration 10's. The 14 `artifacts` cases stay red until both land, and
`render` must still accept and honor `-nopdf`/`-nopng` before then.

**6.2 The watcher** (`--watch`) has no corpus case and is iteration 13's.

**6.3 `rendercv-go schema`** does not exist upstream and must not be added: axis 2 forbids new
commands. `TestSchemaParity` shells it and will stay red forever; Axis 3 is closed by
`just schema-diff` instead, which `STATE.md` already records.

---

## 7. Acceptance criteria

- [~] `render_typst_only`: exit code, stdout, stderr and file list **all match**; the `.typ`
      differs on one line, `day: 6` versus `day: 7`. That is the corpus defect recorded in
      `STATE.md` — the goldens bake their generation date — and not something this iteration can
      fix without the human gate on regenerating them.
- [ ] The four other `render` flag cases: custom paths, the three override shapes, `--quiet`.
- [ ] The seven `errors` cases: exit code, stderr text, empty stdout.
- [ ] `new` and `create-theme`, including the four locale variants.
- [ ] The six help and version cases, which are §5's problem and should be attempted last.
- [ ] The 14 `artifacts` cases, jointly with iteration 10.

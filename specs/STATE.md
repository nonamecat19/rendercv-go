# Port ledger

Living state of the rendercv-go port. **Updated only by the merge owner, after
`rendercv-parity-verifier` reports.** Never edited by a porter as part of a feature commit.

Upstream target: `third_party/rendercv` @ `v2.8` (`2eba248`)

Legend: `—` not started · `spec` spec written · `red` tests written, failing ·
`wip` implementation in progress · `green` all its conformance cases pass

---

## Iterations

| # | Subsystem | Spec | Status | Conformance cases passing |
|---|---|---|---|---|
| 0 | Bootstrap (layout, AGENTS.md, submodule, agents, skills, CI) | — | green | n/a |
| 1 | Conformance harness (corpus, gengolden, helpers) | [001](001-conformance-harness/spec.md) | **audited — FAIL, demoted.** Comparison path sound (6/6 mutations caught); the goldens bake absolute paths and the generation month | n/a (42 cases red by design) |
| 2 | YAML reader + core model (RenderCVModel, CV, Section) | [002](002-yaml-and-core-model/spec.md) | green (with cut scope, see below) | n/a (gated on unit tests, spec §7.2) |
| 3 | Entry types (9) | [003](003-entry-types/spec.md) | **re-audited — FAIL again.** Three of five closures hold; two of my repairs introduced new defects | n/a (gated on unit tests, spec §7.1) |
| 4 | Validation-error parity | [004](004-validation-errors/spec.md) | green | n/a (gated on the 25-record differential, spec §7.3) |
| 5 | JSON Schema generator | [005](005-json-schema/spec.md) | **verified green** — audited and passed, the only iteration besides 9 to do so | n/a (gated on the 18 owned `$defs`, spec §7.1) |
| 6 | Design & themes (9) + the settings schema | [006](006-design-and-themes/spec.md) | **audited — FAIL, demoted.** Font families were a closed enum where upstream accepts any string (fixed); three findings open | n/a (gated on the 164 `$defs` differential and the override diff, spec §5) |
| 7 | Locale (English + 21 catalogs) | [007](007-locale/spec.md) | **audited — FAIL, demoted.** Three locale fields are unvalidated; a short month list panicked the renderer (fixed) | n/a (gated on the 45 `$defs` differential and the submodule catalog diff, spec §5) |
| 8 | Templater (pongo2 env, filters, markdown→typst, processors) | [008](008-templater/spec.md) | **audited — FAIL, demoted.** Four `markdown_to_typst` divergences still live and unrecorded; one produced uncompilable Typst and is fixed | n/a (gated on the 52-fragment Jinja differential and 240 unit cases, spec §7) |
| 9 | Typst renderer (`.typ` emission) + iteration 6's T10 + iteration 8's Wave C | [009](009-typst-renderer/spec.md) | **green** — verified by a fresh context, which returned FAIL on four items; all four fixed and pinned | 24 / 24 |
| 10 | wazero + WASI typst → PDF, then PNG | [010](010-typst-compilation/spec.md) | **gate cleared 2026-08-08; landed and running in the suite.** The compiler, the fonts and `fontawesome` are vendored and embedded (D-007). Every render case now produces a PDF and its PNGs, and `AssertPDF` compares text, page count and geometry. **Not yet verified by a fresh context** | 14 / 14 in the suite |
| 11 | Markdown + HTML renderers | [011](011-markdown-and-html/spec.md) | **verified — FAIL, demoted from green.** 24/24 on the corpus, but a `"` in any CV breaks the HTML and raw HTML is dropped. Not green | 24 / 24 corpus, blockers open |
| 12 | CLI (`new`, `render`, `create-theme`, overrides, watcher) | [012](012-cli/spec.md) | **in progress.** `render` and `new` are wired and their goldens pass, error panels included. `create-theme` is now registered and `--create-typst-templates`/`--create-markdown-templates` write their folders; two of their corpus cases stay red by construction under D-008. `err_not_yaml` is fixed. The five help panels are written and verified, four unreachable by construction under D-010; two more are D-011's unhandled-exception tracebacks | 34 / 42 `TestParity` cases (the suite has 42, not 43 — `manifest.json` was miscounted as a case), **not yet verified by a fresh context** |
| 13 | Parity closeout (sample generator, version, error handler, packaging) | — | — | 0 |
| 14 | Lua-scripted custom themes (D-002) + the two folder messages | [014](014-lua-custom-themes/spec.md) | **re-verified 15×, 2026-08-10 — FAIL every pass through the 15th.** Pass 15 found 6 findings (3 blockers, 2 major, 1 minor). 3 blockers fixed (`44f513d`), all in `color.go`'s numeric parsing: `parseChannel`'s and `normalizeAlpha`'s range checks were `value < 0 \|\| value > max`, false in both directions for NaN — the *accept* condition, inverted from Python's chained `0 <= color <= max_val` — so a NaN channel or alpha reached `int(NaN)` and printed undefined-overflow garbage into the artifact instead of a range error; both rewritten as the chained form. `parseAlpha`'s percent branch had never been routed through pass 14's `parseNumericText`, so whitespace-padded (`" 50%"`) alphas failed where the equivalent plain-number alpha already worked. And `parseNumericText`'s hex/octal/binary prefix check lowercased its own copy before comparing, so `0X1F` matched even though ruamel's YAML resolver is lowercase-only and leaves it a plain string upstream — `float("0X1F")` raises there, where the port silently rendered `rgb(31, 0, 0)`. **Two findings are out of iteration 14's subsystem, recorded rather than fixed here**: a non-string scalar on a string-typed **CV** field (`cv.name: true`, `cv.email: true`, `cv.website: true`) is not type-checked at all, rendering a full artifact at exit 0 where upstream exits 1 with `Input should be a valid string.` — `internal/schema/binder/binder.go` and `specs/004-validation-errors/spec.md:351` already have the machinery, it just isn't reached from these three CV fields; and a literal tab byte inside a double-quoted YAML scalar is rejected by the goccy reader where YAML permits it as scalar content (the *escaped* `\t` form parses fine). Both are validation-error-parity territory (iterations 2-4), not design/Lua, and belong in a future iteration rather than expanding this one's scope. The minor (`df4a82a` bundled three logical units in one commit, a process nit under §7) is noted, not correctable after the fact. Pass 15 confirmed pass 14's four fixes hold on 13+ merge-carve-out probes, 11 bool-Input-Value probes, and 5 script-tuple-validation probes, plus a broad sandbox sweep (`os.exit`, `io.open`, `require`, `error()`, non-table/nil return, infinite recursion — all terminate safely at exit 0 with the fallback tree). | 0 blockers reproduced by the 15th pass's fixes (unverified by a 16th), 2 deferred findings (1 minor), 2 new out-of-scope findings recorded for a future validation-error/reader iteration, 2 known gaps cut forward, 1 coverage gap open, 3 process findings open |

## Parity axes

| Axis | Gate command | Status |
|---|---|---|
| 1 — artifacts byte-identical | `just test-parity` | **PDF and PNG now compare, and every render case passes.** 14 render cases produce a PDF matched on extracted text, page count and page geometry, plus their PNGs on name and dimensions. Previously: **72 passing comparisons on the corpus, and the corpus is narrower than that number suggests** — 8 of the 24 cases share one byte-identical `.md`, so there are 14 distinct Markdown documents, and a verifier broke the HTML with a double quote. Every text artifact — 24/24 `.typ`, `.md` and `.html` — byte-identical against the vendored Python (`TestCorpusTypstIsByteIdentical`), over the 21 corpus inputs plus three the corpus cannot express. PDF and PNG are iteration 10's. The 15 CLI-driven artifact cases stay red until iteration 12: they shell `rendercv-go render`, which does not exist. |
| 2 — CLI surface | `just test-parity` | **34 of 42 `TestParity` cases pass — and the suite is much narrower than the axis.** (The suite has 42 distinct cases; an earlier count of 43 mistook `manifest.json` for one.) `create-theme` is now registered (`cli_create_theme_help` passes), `--create-typst-templates`/`--create-markdown-templates` write their folders, and `err_not_yaml`'s span now matches upstream byte-for-byte — `yamlErrorLocation` synthesizes the EOF problem-mark goccy doesn't expose, scoped to the one measured shape (an unterminated flow sequence). The eight that still fail: four help pages, **written and verified**, unreachable by construction under D-010; `create_theme` and `new_typst_templates`, unreachable under D-008 (both compare template *source*, and the port's loader reads the pongo2 transform, not upstream's Jinja); and `err_missing_file` / `err_bad_override_key`, unhandled-exception tracebacks under D-011. **The port also tracks `COLUMNS` now (G-11, closed) — see `gaps.md`.** |
| 3 — JSON Schema | `just schema-diff` | **green.** All 227 `$defs` byte-identical; the command exits 0. The oracle is `tools/genschema`, not the parity suite — `TestSchemaParity` shells `rendercv-go schema` and stays red until iteration 12. |
| 4 — validation errors | `just test-parity` | **4/7 passing** (`err_empty_yaml`, `err_unknown_theme`, `err_unknown_locale`, `err_wrong_input`). Rich's error table is reproduced and its three width stages are pinned. **verified — FAIL.** The 25-record differential is real and mutation-discriminating, but it gates far less than the axis: 6 of 13 dictionary rows, 2 of 8 username rules. Two blockers open (below). 0/7 corpus cases. |

PDF content comparison (spec §1.2) is measurable and measured: `conformance.AssertPDF`, over poppler.

## The corpus could not see the CLI — 2026-08-08

**`TestParity` did not move, and that is the finding.** Six invocations were measured against the
vendored binary; every one of them exited **70 with no output at all**, and all 25 passing corpus
cases passed before and after the fixes. 70 is `Execute`'s initial `code`, so a caller could not
tell a malformed invocation from an internal failure — or, on the combinations an earlier audit
caught, from a *successful* render.

| Invocation | Upstream | Port, before |
|---|---|---|
| `rendercv-go bogus` | usage + `No such command 'bogus'.` on stderr, exit 2 | nothing, exit 70 |
| `rendercv-go render` | usage + `Missing argument 'INPUT_FILE_NAME'.`, exit 2 | nothing, exit 70 |
| `rendercv-go new` | usage + `Missing argument 'FULL_NAME'.`, exit 2 | nothing, exit 70 |
| `render cv.yaml --nope value` | override key `nope`, rejected by the model | nothing, exit 70 |
| `render cv.yaml a b c` | `There is a problem with the extra arguments (a,b,c)!…` | nothing, exit 70 |
| `render cv.yaml -x value` | `The key (-x) should start with double dashes!` | nothing, exit 70 |

### The flag inventory was wrong in five ways and no case could show it

Spec 012 §2 named only the single-dash spellings, because those are the only ones the corpus uses:
`render_typst_only` and `render_custom_paths` between them pass ten of the seventeen options and
name **no long form at all**. So the port registered `--typ`, `--pdf`, `--png`, `--md`, `--html`,
`--nopdf` and friends — **seven long names upstream has never had** — and passed every case.

Measured off `render_command.py:33-198` and now in the spec as a table:

| What was wrong | Reach |
|---|---|
| five path options had invented long names (`--typ` for `--typst-path`, …) | `--typst-path out.typ` was an unknown flag |
| five negatives likewise (`--nopdf` for `--dont-generate-pdf`) | same |
| `--design`, `--locale-catalog`, `--settings` were **never declared** | three overlay files unreachable |
| `--watch` was never declared | spec §6.2's flag did not parse |
| `new --create-markdown-templates` was never declared | writes files upstream, errors here |

The three overlay options were also **declared and never read** once added:
`modelbuilder.BuildArguments` has carried `DesignYaml`, `LocaleYaml` and `SettingsYaml` since
iteration 2 and `buildArguments` left all three empty. Now differentially checked — the `.typ`,
`.md` and `.html` of `John_Doe_CV.yaml` with a design and a locale overlay are byte-identical to
upstream's, and 226 lines of the `.typ` differ without the overlays, so the comparison is not
vacuous.

The overlay file is a **whole document keyed by the field**, not the field's body: `-d` on a file
reading `theme: moderncv` makes upstream die with `KeyError: 'design'`.

### The override collector was a dotted-key filter and upstream has no such thing

`render` is declared `allow_extra_args` + `ignore_unknown_options` (`render_command.py:26`), so
click hands it **every** token it did not recognize, in order, and `parse_override_arguments`
reads that list as alternating keys and values. The port collected only tokens containing a dot
and let cobra reject the rest.

Two rules were measured rather than assumed, and both would have been guessed wrong:

- **An unrecognized option does not swallow the next token.** `--nope -nopdf` leaves one extra and
  a real flag; upstream reports `(--nope)`.
- **The `=` form is not split for one.** `--cv.name=Jane` is a single token and therefore an odd
  count, which is upstream's answer too.

And `key.replace("--", "")` (`:51`) is **unanchored**, so `--cv--name` becomes `cvname`. The port
now reports exactly that field as unknown, which is how the rule was confirmed.

Six vectors differentialled against the vendored CLI: four byte-identical on stdout and exit. The
two that differ are upstream *tracebacks* on an unknown override key — the `err_bad_override_key`
divergence already recorded here.

### What this pass did not do

- **No fresh-context verification.** Every number above is what the suite printed in the context
  that wrote the code, which `AGENTS.md` §10.6 says is not parity.
- **`create-theme`'s usage error is not implemented**, because the command is not registered. It
  lands with the command, and D-008 already approved that design.
- **`rendercv-go` with no arguments still exits 70 where upstream prints the full help and exits
  0.** That is the help renderer of §5 — the five `cli_*_help` cases — and it stays blocked on
  reading `typer/rich_utils.py`.
- **`just check` still fails on three lint issues**, all three present at `50c6c01` and none of
  them this pass's: an unchecked `module.Close`, a type assertion on a wrapped error, and the
  unused `themeOf`.

## `create-theme` and `--create-typst-templates` landed — 2026-08-09

Both were the last two things spec 012 §3 named as unwritten. Neither needed a new human gate:
D-008 (approved 2026-08-08) already decided what a written-out custom theme folder must contain,
so this pass is that decision implemented, not a new one.

**`internal/cli/customtheme.go` is new** and both features share it:

- `copyTypstTemplates(dest)` walks `templater.BuiltinTemplates()`'s `typst/` subtree — the port's
  own pongo2 transform, already embedded for rendering — and writes the same thirteen files
  `copy_templates("typst", …)` copies upstream: the four top-level fragments and the nine entry
  templates.
- `writeThemeInitLua` writes `init.lua` in place of `__init__.py`, per D-008. It is a documented
  empty table (`return {}`) rather than a transliteration of `ClassicTheme`'s 857-line pydantic
  model — `luatheme.Options` reads whatever a script returns keyed like `design:`, so there is no
  class for a starter to derive from, and restating every classic-theme default as a comment would
  be a golden by another name.
- `CreateTheme` (`create-theme <name>`) validates the name against `custom_theme_name_pattern`
  (`^[a-z0-9]+$`), rejects an existing folder with upstream's exact wording, writes the fourteen
  files, and prints the "Theme created" panel — upstream's text with `__init__.py` renamed to
  `init.lua` throughout, D-008's user-visible half.
- `new --create-typst-templates` now writes the same thirteen files into `./<theme>/` (defaulting
  to `classic`, upstream's own default) instead of returning "not implemented", and the "Get
  started" panel grows upstream's "Also created" / "Not modified (already exist)" block
  (`new_command.py:150-166`).

**Round-tripped by hand, not just by the suite**: a theme `create-theme` writes is a theme
`render --design` (via `design.theme: <name>`) loads and renders without complaint — the folder
check that used to reject unknown custom themes finds `init.lua` and the `.j2.typ` set it just
wrote.

`TestParity` moved from 25/35 to **33/43** (the suite grew eight cases since that count was taken;
this pass accounts for the +8 pass delta, not the count change). `cli_create_theme_help` is a
genuine new pass — the help data was already captured, only the command's registration was
missing. `create_theme` and `new_typst_templates` are still red, and stay red: both compare
template *source*, and D-008 already recorded why the port cannot produce upstream's Jinja bytes
without breaking the theme it just wrote. `specs/divergences.md`'s D-008 entry now names both
cases instead of one.

**Not done in this pass**: `new --create-markdown-templates` (G-9) is a separate flag, a separate
template set, and no corpus case names it — left for whoever picks up G-9. No fresh-context
verification has run over this change.

## `specs/012-cli/gaps.md`'s record caught up to the code — 2026-08-09

All eleven G-numbered findings in `gaps.md` are now closed. Most of the work had already landed in
earlier 2026-08-08 commits (`e78ad3d`, `c03fe1d`, `555d7e0`, `8594b5d`, and the usage-error commits
behind G-1 through G-4) — the gap was in the ledger, not the binary, and this pass verified each
one by hand against the vendored CLI before marking it closed rather than trusting the stale text.
The one substantive fix in this pass: **`render --watch` now rejects** (exit 1, "not implemented")
instead of silently doing a one-shot render and exiting 0 as though the flag had been honored
(G-10). The watcher feature is still iteration 13's.

`TestParity` is unchanged at 33/43 — none of this was reachable by the corpus, which is why the
ledger could drift this far without a red test catching it.

## G-9 closed — 2026-08-09

`new --create-markdown-templates` was declared and unread — the flag parsed, upstream writes a
`markdown` folder, the port did nothing. `copyTypstTemplates` from the previous pass generalizes to
`copyBuiltinTemplates(kind, dest)`, and `--create-markdown-templates` now calls it with `"markdown"`
the way `--create-typst-templates` calls it with `"typst"` — the port's own pongo2 transform of the
twelve Markdown fragments (three top-level, nine entries; Markdown has no `Preamble`).

**The panel logic changed shape, not just grew a branch.** Upstream collects every requested
template kind into one `created`/`existing` list before building the "Get started" panel
(`new_command.py:117-166`), so `--create-typst-templates --create-markdown-templates` together —
one already written, one not — produces **one panel with both an "Also created" and a "Not
modified" section**, each item's own row. The prior pass's `typstTemplatesRows` only knew about one
kind and one state; it is now `templateRows(created, existing []templateItem)`, verified by hand
against that combination.

No corpus case names either flag (`gaps.md` §6 recorded this at the start), so `TestParity` did not
move — 33/43, same as before this pass. No fresh-context verification has run.

## The help renderer landed and moved no case — 2026-08-08

Spec [`012-cli/help.md`](012-cli/help.md) is the measured behavior; the renderer is written and
`--help`, `-h` and the bare no-argument invocation all work. **`TestParity` is unchanged at
25 / 35**, and the reason is D-010 rather than anything unfinished.

**The blocker was self-imposed.** `STATE.md` said to read `typer/rich_utils.py` before attempting
this; it sits in the submodule's venv and nobody had. The earlier attempt reverse-engineered the
geometry from goldens, got two columns right and the third wrong, and stopped.

**What that attempt got wrong is worth naming.** The panels' tables *are* declared `expand=True`,
and reasoning forward from that flag leads to `ratio_distribute` spreading slack across every
column. There is no slack: the help cell is a `rich.columns.Columns`, which measures
`(1, max_width)`, so every panel's natural sum overflows and rich always takes the **collapse**
branch instead — taking the whole excess off that one column. `collapseWidths` and `ratioReduce`
already existed for the validation table.

Measured by instrumenting `Table._calculate_column_widths` while the vendored CLI ran, not read
off a golden. The eight resulting width vectors predict the column offsets in all five goldens
exactly, and they are the fixture. Mutation-checked: measuring the flexible column by its text
sends the layout down the expand branch and fails all nine subtests.

Two more behaviors that measurement caught and inference would not:

- **A `Padding` region is painted, not skipped** — the line above the usage is eighty spaces, not
  nothing. Getting it wrong shifts no text while differing from the golden on five lines a page.
- **`Columns` is a flow, not a stack.** `The YAML input file. [required]` shares a line;
  `new --theme`'s prose fills its column and pushes `[default: classic]` below. Neither "join with
  a space" nor "one per line" is right, and each is right for one of the two.

### Where the five cases stand

| Page | lines | differing |
|---|---:|---:|
| `cli_create_theme_help` | 14 | **0** — blocked only on the command being registered |
| `cli_help` / `cli_help_short` | 24 | 2 |
| `cli_render_help` | 62 | 2 |
| `cli_new_help` | 36 | 2 |

The geometry therefore holds on **130 of 136 lines**, and every one of the six exceptions carries
the binary name. `internal/cli/help_test.go` fails if a differing line ever does not, so the
budget cannot absorb a regression.

**D-010 is approved and records why three pages can never be byte-identical**: the port must print
`rendercv-go` in prose the reader is meant to run, a help page wraps before anything compares it,
and re-padding cannot undo a re-wrap. Same shape as `err_missing_file` — unreachable by
construction, not by effort.

### The harness had no tests, and it was wrong

`RebindBinaryName` re-padded a shortened **bordered** row and left everything else as substituted.
A help page's `Padding` lines are painted to the console width too, so `isPanelRow` left them
three characters short. Fixed: `cli_help` went from differing at byte 158 (2430 bytes against
2433) to differing only on the two re-wrapped lines, at equal length.

**It is the one comparison rule that rewrites what the port produced, and nothing pinned it** —
in the harness iteration 1's audit already identified as the instrument every other claim rests
on. It now has `internal/conformance/binaryname_test.go`.

### What is left for these cases

`create-theme` must be **registered** before `cli_create_theme_help` can pass. Registering it for
its help alone would turn `create-theme foo` from `No such command` into something that silently
does nothing, so it lands with the command. D-008 approved the design and one question in it is
still open: upstream generates the theme's `__init__.py` from `classic_theme.py`, and the port's
`init.lua` has no equivalent source — `design.Overrides("classic")` is empty, because classic *is*
the base tree. What that file should contain is a design decision, not a transcription.

## Stretch goals (not gates)

- [ ] PNG pixel-level comparison (depends on the WASI typst font set — see D-006)
- [ ] Public `pkg/rendercv` API frozen and semver'd
- [ ] Cross-compiled release artifacts (linux/darwin/windows × amd64/arm64)

## Cut scope

Anything dropped from an iteration is recorded here with the reason, per `AGENTS.md` §10.2.

### Iteration 2

Verified by `rendercv-parity-verifier` in a fresh context. Everything below is carried into
iteration 3's spec as an open item; nothing here is a silent divergence.

1. **Coordinate columns diverge from ruamel in two shapes the T8 fixture cannot see.** A key
   with a null or empty value reports column 1 rather than the key's own indent
   (`internal/schema/yamlreader/build.go`), and a flow-sequence element reports the first value
   token rather than the `[`. Measured against upstream: 33/232 paths differ on
   `examples/John_Doe_ClassicTheme_CV.yaml`, 50/388 on
   `tests/schema/testdata/test_pydantic_error_handling/expected_errors.yaml`. **Lines match
   everywhere**, and spec §6.7 says only line numbers reach users, so nothing user-visible is
   affected yet — but `expected_errors.yaml` is iteration 4's import, so this must be fixed
   there, together with extending the fixture to cover both shapes.
2. **`models.Validate` does not call `cv.Validate`.** `models` owns `ValidationContext` and the
   path types, which `cv` imports, so the edge would cycle. Closing it needs those two types
   moved to a leaf package — a `plan.md` layout change, deferred to iteration 3 rather than
   made unreviewed at the tail of this one. Everything *inside* `cv` is wired (commit
   `fd33d82`).
3. **`phone` formatting (spec §3.49) is not implemented.** `+905419999999` does not serialize to
   `+90-541-999-99-99`; only the `tel:` strip is done. Spec §8 lists this as an acceptance
   criterion while spec §7 assigns phone formatting to iteration 4 — **the spec contradicts
   itself and iteration 4 must resolve which is right.**
4. **T7's no-op regression test over the submodule YAML corpus was never written.** The verifier
   ran it by hand — 64/64 files identical with and without `dealias` — so the transform is
   sound, but nothing in the suite guards it. `noalias_test.go` also asserts tokens where
   `tasks.md` required the parsed tree; the tree-level assertions now live in
   `yamlreader_test.go` (commit `f91da06`).
5. **T10's scalar corpus is hand-written, not generated.** `tools/yamlprobe` still emits only the
   five coordinate documents, so `resolve_test.go` states Go-side expectations rather than
   deriving them from upstream. Behavior was differentially verified as correct.
6. **§4.12's "mixed section" and "entry problems" criteria are tested through an injected
   validator**, because the concrete entry types are iteration 3's. They must be re-checked
   against real types when those land.

Two process failures in this iteration's history, recorded rather than rewritten:

- `1befa1e` bundles T9, T10 and T11 in one commit, against `AGENTS.md` §7.
- T8's coordinate test landed in `65aaa49`, *after* the T9 implementation it was supposed to
  precede, inverting the red-before-green rule of `AGENTS.md` §4.

### Iteration 3

Verified by `rendercv-parity-verifier` in a fresh context, which returned **FAIL** with three
blockers. Two were fixed inside the iteration (commit `9ddd896`); the rest is cut here. Nothing
below is a silent divergence, and none of it belongs in `divergences.md` — every item is an
ordinary bug reproducible in Go, not a place where parity is impossible.

**Fixed rather than cut** (recorded because the failure mode matters): three tests I wrote
asserted the *port's* error codes instead of upstream's, which is why the suite was green while
the codes were wrong. One test stated the correct upstream code in a comment and asserted a
different one on the next line. The diamond caught what self-report would not have.

1. **Entry error ordering does not interleave base and own fields.** The composed binder spec is
   `[base fields] ++ [own fields]`, the reverse of what `class X(BaseWithDates, BaseX)` produces,
   and the `date`/`start_date` validation errors are appended after all bind errors rather than
   emitted at their declared position. Measured against upstream:

   | input | upstream | Go |
   |---|---|---|
   | `ExperienceEntry{company, summary: {a: 1}}` | `position` missing, `summary` string_type | `summary` string_type, `position` missing |
   | `ExperienceEntry{position, highlights: "x"}` | `company` missing, `highlights` list_type | `highlights` list_type, `company` missing |
   | `ExperienceEntry{company, date: 2020-13-01, location: {a: 1}}` | `position`, `date`, `location` | `location`, `position`, `date` |
   | `PublicationEntry{title, authors, doi: bad, journal: {a: 1}}` | `doi` string_pattern_mismatch, `journal` string_type | `journal` string_type, `doi` string_pattern_mismatch |

   The descriptors already carry the right order, so part of the fix is feeding the binder the
   descriptor order. But the date errors must also be *interleaved* at their declared position
   rather than appended, which touches all three base binders plus `publication.go`. Spec 003 §6.3
   already carries a `TODO(iteration-4)` on entry error ordering, and iteration 4 owns error
   ordering, deduplication and message rewriting as its whole subject — so this lands there rather
   than as a rushed refactor at the tail of iteration 3. **Iteration 4 must not build its rewrite
   pipeline before fixing this**, because dedup by `schema_location` is order-sensitive.

2. **A non-scalar `date` or `start_date` produces no error at all.** Upstream produces two union-branch
   errors: `{date: {a: 1}}` on an `ExperienceEntry` gives `('date','int') int_type` and
   `('date','str') string_type`; `start_date` likewise. Go is silent. Reachable through real types
   for the first time in this iteration.

   Iteration 4 detail that makes this cheaper there than here: `unwanted_locations`
   (`pydantic_error_handling.py:24-32`) filters `"int"` and `"str"` out of the location, collapsing
   both rows to `('date',)`, and dedup by `schema_location` (`:167-176`) then merges them into
   **one** user-visible error. So the correct fix is one error at the field, and it needs iteration
   4's filtering and dedup to be expressed faithfully.

3. **Two `specs/003-entry-types/spec.md` §8 claims were inaccurate and are corrected in the record,
   not the code.** (a) §3.18 behavior 42 says iteration 2 "already emits both" custom error types —
   it emitted only `rendercv_other_error`; fixed in `9ddd896`. (b) §8's "every section test iteration 2
   wrote passes against the real registry with no edit to the test" is false: three tests changed
   (`sectionlist_test.go:20-24`, `:81-88`; `sectionvalidation_test.go`'s
   `TestFirstResolvableEntryWins`). The changes are correct — each fixture was one the
   accept-everything stub let pass and that upstream genuinely rejects, and each new expectation was
   measured against the vendored Python first — but the criterion as written cannot be checked, and
   the stale comment claiming otherwise has been corrected.

4. **`Registry.Discriminate` rebuilds the characteristic table on every call**
   (`entries/registry.go:70`). Not a parity issue; performance only. Left as is.

5. **The constructed-entry half of §8's discrimination criterion is not tested.** Upstream asserts
   the seven `(entry_type_name, section_model_name)` pairs twice — once from a raw dict and once
   from a constructed model instance (`tests/schema/models/cv/test_section.py:19-60`). Only the
   raw-mapping half is covered. The first attempt at the second half was a tautology: it validated
   a node and then re-resolved the same node, and both calls take the identical mapping branch, so
   it could not fail unless `Discriminate` were nondeterministic. It was removed rather than left
   as false coverage.

   Upstream's already-a-model branch (`section.py:173-176`) returns `entry.__class__.__name__` and
   has no Go equivalent — a non-mapping, non-string, non-null node falls through to the
   `messageNoType` branch under the `TODO(iteration-4)` in `sectionvalidation.go`. Reproducing it
   depends on iteration 4's §5.14 already-a-model decision, so it lands there.

6. **`messageModelType` is missing the entry type name.** Ours is `Input should be a valid
   dictionary`; upstream's is `Input should be a valid dictionary or instance of EducationEntry` —
   the concrete type is interpolated, and no `error_dictionary.yaml` row rewrites it, so it reaches
   the user raw. Measured on `[{institution, area}, null]`. Reachable from an ordinary document (a
   stray blank list item). Iteration 4 owns it, and it needs the entry type threaded into the
   binder's message rather than a table lookup.

Two process failures in this iteration, recorded rather than rewritten:

- `9ddd896` bundles the wrapper fix, the date-code split, the test corrections and two new
  criterion tests in one commit, against `AGENTS.md` §7. It should have been four.
- `8d131da` is labelled `test:` but ships 17 lines of production `default.go`. The production line
  was the deliberately-empty `Default()` that made the conformance test red, so the red-before-green
  intent held, but the type prefix is wrong.

Verifier items closed inside the iteration: the behavior-43 code table (it had substituted the date
code and `model_type` for `string_pattern_mismatch` and `url_too_long`, so two of the five were
never exercised), discrimination from an already-constructed entry, and the summary-only entry
matching no type end to end.

Not verifiable yet, and not claimed: PDF/PNG, artifact and CLI parity (iterations 10 and 12).

### Iteration 4

**No cut scope.** Every task in `tasks.md` landed, every `TODO(iteration-4)` in the tree is
cleared, and both items cut from iteration 3 plus four of the five carried from iteration 2 are
closed.

The gate is `TestWrongInputDifferential`: upstream's own `wrong_input.yaml` through the whole
port, compared against `expected_errors.yaml` **member by member, in order, with an equal-length
assertion** — all 25 records, all five members including coordinates. It is a port of
`tests/schema/test_pydantic_error_handling.py:19-54` and is the strongest mechanical Axis-4 gate
available.

The parity suite stays at its 42 red cases, which is iteration 1's baseline and not this
iteration's failure: no corpus case can pass until the renderer exists (spec §7.3). No golden was
regenerated and the submodule was not bumped, so no human gate was requested.

**Bugs found while measuring, none in the task that found them.** Recorded because each was
invisible to the suite that existed at the time:

| Bug | Found by |
|---|---|
| Quoted mapping keys kept their quotes, so `"name": John` was rejected as an unknown key | the dictionary's one quoted scalar |
| `0b101` resolved to a string; ruamel reads it as 5 | the generated scalar corpus |
| Unknown keys were reported before every declared field | measuring the order upstream reports |
| An exact date's range and isoformat failures carried the wrong code (4 of 5 rows) | measuring §4.13's texts |
| `Invalid isoformat string` was missing entirely | the same measurement |
| `PublicationEntry.url` reported after every other field | registering its validator |
| A valueless key's span ended at column 1 whatever the indent | the coordinate differential |
| A flow-collection element started at its first value, not its bracket | the same |

**Two corrections to the spec, both recorded in place rather than silently edited:**

1. §3.2's table gave the reason for step 1 preceding the dictionary as "or `value is not a valid
   phone number` never matches". It does not hold — that message carries no prefix, and
   substitution matches by containment and replaces the whole message, so a prefix can only add a
   match. The ordering is still reproduced;
   `errorpipeline.TestPrefixStripDoesNotChangeWhichRowMatches` now asserts the unobservability, so
   a future row that makes the order matter fails rather than passing silently.
2. §3.13 behavior 46 said the URL length limit is 2083 **characters**. It is UTF-8 **bytes** — the
   check lives in pydantic-core, which is Rust. ASCII hides the difference, so nothing measured
   before was wrong, but a port counting runes would accept URLs upstream rejects.

**Carried forward, with owners:**

- **`emailaddr` is known-incomplete by decision** (spec §7.4). It reproduces 45 measured
  rejections; the library's catalogue is larger. The gap runs toward *accepting* what upstream
  rejects rather than misreporting it — the safer direction, since a plausible-but-wrong message
  passes review while a missing record shows up in the differential. `ErrUnclassified` exists and
  is unreachable, kept for a rule that can be detected but not named.
- **The YAML parser message is option B, scoped to the corpus** (spec §7.5, plan §6). Five
  goccy→ruamel phrasings are mapped and measured; an unmapped failure falls through to goccy's own
  line. **The coordinate half is not covered**: ruamel reports a context mark and a problem mark,
  goccy reports one token, so the corpus case's `line 1 to line 2` comes out as `line 1`. That is
  a decision needing `specs/divergences.md` and the human gate, and this iteration does not
  authorize writing one — see "Open for the human gate" below.
- **Two upstream crashes reassigned to iteration 12** (spec §7.8): a non-mapping entry and a
  list-valued `phone`. Neither is a validation-error-parity question — upstream produces no
  message, only an unhandled exception — so both wait on the CLI's unhandled-failure handling.
  Both carry `TODO(iteration-12)` markers.

**Open for the human gate:** the YAML-syntax coordinate span above. Nothing else.

### Iteration 5

**No cut scope.** Every task landed. The one deferral is written into the spec rather than cut
here: the `$defs` collision suffix (§7.2) is iteration 6's, because it is exercised only by the
per-theme variants and its numbering follows pydantic's emission order — a plausible
implementation would be a wrong answer that reads right, so `DefNameWithSuffix` panics naming the
iteration that owns it.

**Axis 3 is blocked, not failing, and the distinction is the iteration's main finding.**
`schema.json` is a projection of the model tree, so it is complete exactly when the tree is.
Measured over the 227 `$defs`:

| Owner | `$defs` | Share |
|---|---:|---:|
| Iteration 6 — design and themes | 161 | 74.8% |
| Iteration 7 — locale | 45 | 17.0% |
| Iteration 7 — settings | 3 | 3.3% |
| **Iterations 2–4 — what exists** | **18** | **4.8%** |

All 18 are byte-identical to upstream's. `just schema-diff` prints 8,621 lines of difference and
**zero of them are the port's** — every line it emits appears in upstream's file, which is asserted
by `TestPortSchemaInventsNothing`. A green `schema-diff` before iteration 7 would mean the port
invented something.

The alternative — reordering design and locale ahead of the generator — is recorded in spec §7.1
and rejected: the generator is what makes those two iterations mechanically checkable, and
reordering would put the two largest model iterations back to back with no gate between them.

**Four things the differential caught that transcription would not**, each invisible without
comparing real bytes:

| Finding |
|---|
| A required single-arm field **inlines** its arm: `authors` is `{items, type}`, not `{anyOf: [{items, type}]}` |
| `Cv`, `SocialNetwork` and `CustomConnection` have **no `description` key**; every entry type has an explicit `null` — the entry base's `json_schema_extra`, not a docstring difference |
| `CustomConnection.url` is required *and* nullable: in `required`, carrying the null arm, and with **no default** |
| `Cv`'s three scalar-or-list fields carry **no type key at all**, because pydantic cannot express that union |

One rule was derived rather than listed, and it will matter later: pydantic omits a field's title
when the schema is exactly one `$ref` plus `null`. That is why `date` and `start_date` have none
and `end_date` does — the shape, not the field. Iterations 6 and 7 add many more
optional-reference fields, and a list of the four known cases would have missed them silently.

**Two corrections to the parity contract**, both made in place in `specs/000-parity-contract`:

1. Axis 3 named a `rendercv-go schema` subcommand, which Axis 2.1's "no commands added" forbids —
   upstream's CLI has three commands and none is `schema`. The generation path is now named
   explicitly (`go run ./tools/genschema`), so the two axes cannot be read as contradicting.
2. Axis 3 required a trailing newline. Upstream's file has none; its last three bytes are `"\n}`.

**Carried forward:** the absent-set test states the remaining count as a number that must be
changed deliberately, so iterations 6 and 7 cannot close Axis 3 by accident or leave it closed on
paper. Iteration 7 moved it from 209 to 164; iteration 6 takes it to 0.

### Iteration 6

**Axis 3 is closed.** All 227 `$defs` are byte-identical and `just schema-diff` exits 0. The
oracle is `tools/genschema`; the parity suite's own Axis-3 case shells `rendercv-go schema` and
stays red until iteration 12, so "closed" means by the generation path spec 005 §4 named, not by
`just test-parity`.

**Verified by `rendercv-parity-verifier` in a fresh context, which returned FAIL with twelve
findings, two of them blockers.** Everything below either was fixed inside the iteration or is cut
here with its reason. None of it belongs in `divergences.md`.

**The blocker worth reading, because the diamond is the only thing that could have caught it.**
The error pipeline reproduced upstream's step 2 — pydantic-core inserts a discriminated union's
resolved branch value as the location's second element, and `parse_plain_pydantic_error` deletes
it. The port never produces that element: `design.Validate` and `locale.Validate` resolve the union
themselves. Deleting anyway removed a **real** key. `design.colors.body` became `design.body`,
which failed to resolve against the document and reached the user as an internal error;
`design.nope` became `design`, which resolved and shipped a wrong location **silently**.

It was unreachable until this iteration emitted the first non-`theme` location under `design`, and
it survived my own tests because they stop at `models.Validate`. The fix deletes step 2 and adds
`TestDesignAndLocaleErrorsSurviveTheWholePipeline`, which runs five documents through `Parse` and
asserts what the user sees — including the colour message reaching dictionary row 13, the first
live producer for a row that has been in the table since iteration 4.

**Four shape checks were missing and are now measured** (`0dbd1e4`): a bool reported nothing at
all, `font_family` accepted a sequence, a non-string colour reported `string_type` instead of
`color_error`, and a non-mapping nested model reported nothing. Each of the four now carries the
code and message upstream gives, including the pair that distinguishes `bool_parsing` from
`bool_type` and the `model_type` text that **names the model**.

**Cut scope, with owners:**

1. **T10, the effective per-theme option tree, was cut to iteration 9** and is **now closed**
   there (`design.Effective`). Nothing validates a default, so the validation walk is the same for
   all nine themes — `TestDesignBlock` shows the same failure under `classic` and `sb2nov`. The
   cost was stated rather than hidden at the time: `WidenFontFamily` and `SnakeCaseSectionTitles`
   had no non-test callers, and spec §5 criterion 4's "must produce the same **model**" was not
   testable. Both are now called by `Effective`, which runs the two coercions where upstream's
   field validators do, and the criterion is met by a seven-document differential against
   upstream's own resolved model.
2. **Wave E — the D-002 Lua custom-theme path and spec §3 behavior 7's two folder messages — is
   iteration 14's**, a new row in the table above. `plan.md` §7 gives the reason: a sandbox bundled
   with 161 `$defs` makes both unreviewable. `design.go` carries the `TODO(iteration-14)` that
   `tasks.md` promised and did not have until the verifier asked for it.
3. **`settings` is still the thin slice of spec 004 §7.9.** The three `$defs` shipped here to close
   Axis 3; unknown-key rejection under `settings` and everything `RenderCommand` describes are
   iteration 12's. `specs/006-design-and-themes/settings.md` says so.

**A process failure the verifier found and I am recording rather than rewriting:**
`settings/schema.go` shipped before any spec covered it, against `AGENTS.md` §4. The retrofit is
`settings.md`, and it marks which of its criteria are open rather than implying the block is
finished.

**Two commit-discipline failures**, also recorded: `ff0c903` bundles T9, T11 and T12, and `58fc1f0`
bundles three `$defs` models with the Axis-3 status claim. Each should have been three commits.

**A design finding worth carrying forward:** a `design` block with **no `theme` key crashes
upstream** — `validate_design` runs in front of the union and does `str(design["theme"])` unguarded
(`design.py:57`), so the shape that gives `locale` a `union_tag_not_found` gives `design` a
`KeyError`. The port stays silent rather than reporting where upstream crashes, joining the two
crashes spec 004 §7.8 sent to iteration 12.

The parity suite stays at its 42 red cases. No golden was regenerated and the submodule was not
bumped, so no human gate was requested.

### Iteration 8

**Verified by `rendercv-parity-verifier` in a fresh context, which returned FAIL with three
blockers.** All three are fixed (`718c902`); the rest is cut here or recorded.

**The blocker worth reading is the one my own reasoning hid.** Spec §8 argued that the two
whitespace acceptance criteria could not be checked in this iteration, because only a corpus `.typ`
can exercise the transform and a corpus `.typ` needs iteration 9's model bridge. The second half is
true. **The first is not** — a *fragment* needs only a dictionary — and the verifier said so.

The differential I then wrote found, on its first run, that **Jinja strips one trailing newline
from every template** (`keep_trailing_newline=False`) and pongo2 does not. Every fragment gained a
`\n`, and `Assemble` joins entries with `"\n\n"`, so that is a blank line per entry and per section
on **every** artifact case. Reverting the one transform rule fails 43 of the 52 fragments.

I had written the argument that made it invisible. "Only the end-to-end gate can check this" is a
claim that needs testing rather than asserting, and spec §8 now records the reasoning rather than
the conclusion.

**The other two blockers**, both invisible to 235 passing unit tests:

1. **`escape_typst_characters` phase 1 rescanned the mutated text.** Upstream's `itertools.chain`
   binds both `finditer`s before the loop mutates `string`, so the command pattern never sees a
   math dummy. Rescanning matched `#emph[RENDERCVTYPSTCOMMANDORMATH0]` as one command and leaked
   the **literal dummy name** into the output.
2. **Python's `\b` is Unicode-aware and Go's is ASCII-only**, so any `bold_keywords` entry starting
   or ending with a non-ASCII character never matched — `["Café"]` left `Café au lait` untouched.
   `ats_diacritics` is a corpus case.

**Cut scope, with its owner:**

- **Wave C — the corpus's artifact cases — moves to iteration 9**, together with iteration 6's
  already-cut T10. Turning one green needs the schema-to-renderer model bridge, which is iteration
  9's by two earlier decisions. What moves with it is only what no fragment exercises:
  `Preamble.j2.typ` and `Header.j2.typ`, which read `cv._connections`, `cv._footer` and effective
  design values. All twenty-three of spec §7's criteria are met here.

**Two things the verifier found that are open, and are not blockers:**

- **`render_entry_templates` and `process_date` were not ported** — `tasks.md` T9 marked behaviors
  58–66 done and only the leaf helpers existed. Both are pure string functions needing nothing from
  the renderer, so they were **under-scoped, not blocked**. **Closed at the head of iteration 9**
  (`EntryDate`, `RenderEntryTemplates`), and the orchestrator is what made the other nine
  processors reachable at all: before it, `process.Run` called `RunFields` directly and no theme
  template was ever expanded.
- **Five `markdown_to_typst` divergences**, none declared: an image is dropped upstream and
  rendered here, raw HTML passes through unescaped upstream, an autolink becomes a link upstream, a
  link title is not stripped here, and a doubled backtick differs. All reachable from ordinary CV
  text. **This needs `specs/divergences.md` and the human gate** unless iteration 9 closes them —
  and unlike the gate I invented for the parser choice, these are user-visible.

**One more, measured and left as is:** an **empty** `bold_keywords` entry produces different
garbage on each side, because Go skips an empty regexp match adjacent to a non-empty one and Python
does not. Documented in its own test with the measurement; whether it is worth a divergence entry
is a human call.

**Process failures, recorded rather than rewritten:** `6f11003` bundles the environment, the loader
and seven filters where `AGENTS.md` §7's table asks for one commit per filter plus one for the
environment; `f785d95` bundles the generator, four filters, the embed, a `justfile` recipe and 26
generated templates across 31 files; and two generated fixtures landed in the same commit as the
code they gate, against §7's "fixtures land first, red".

Also: spec §4F behavior 56 says `-%}` appears **never**. It appears nine times. The transform is
correct anyway — pongo2 implements both trims — but the inventory the plan was sized from was
wrong.

The parity suite stays at its 42 red cases. No golden was regenerated and the submodule was not
bumped, so no human gate was requested.

### Iteration 7

**No cut scope.** Every task landed: the ten-field catalog model with both length messages
(T1–T2), the submodule-diff gate (T3), all twenty-two catalogs (T4–T5), the `$defs` collision
numbering (T26), and the 45 locale `$defs` (T27).

T26 landed here rather than in iteration 6, where spec 005 §7.2's panic pointed. Locale's collision
is a flat list of twenty-two; design's is nine themes × their nested models. Same rule, and getting
it visibly right on the easy case means iteration 6 inherits it working. Its difficulty is
invisible in the output — `$defs` sorts its keys, so an alphabetically-assigned suffix produces a
file that *looks* correctly sorted while pairing every model with the wrong number.

**Axis 3 is now blocked on iteration 6 alone.** 63 of 227 `$defs` are the port's and byte-identical;
the 164 absent are the design tree and the three settings models. The absent-set count in
`internal/schema/jsonschema/golden_conformance_test.go` moved from 227−18 to 227−63, and
`just schema-diff` still emits exactly one `+` line — the diff header — so the port invents nothing.

**Two behaviors were measured only while implementing**, both written into spec §3.2 rather than
only here, and both cases where the port's natural answer differs from upstream:

1. **The twelve-element bound is `EnglishLocale`'s alone.** `{language: danish, month_names: [11
   items]}` validates and the same document with `english` does not — the `at.Len(12, 12)` lives in
   `Annotated` metadata that `create_simple_field_spec` strips when it rebuilds each variant's
   field. T2 had applied it to every member, which **rejects documents upstream accepts**, and no
   English fixture could see it. Fixed in `5ded987`; the schema half of the same fact (no
   `minItems`/`maxItems` on the twenty-one variants) had already been forced correct by the `$defs`
   differential, which is how the validation half was found.
2. **`{locale: {language: null}}` is a failure, not an absence** — `union_tag_invalid` with the tag
   reading `'None'`. Two sibling failures came with it, `union_tag_not_found` for a block with no
   `language` and `model_attributes_type` for a non-mapping `locale`.

**Three findings from the verifier, all closed inside the iteration:**

- **`ValidateCatalog` was unreachable.** `rendercvmodel.go` called only `ValidateLanguage`, so the
  extra-key and length rules T1 and T2 shipped existed and no document could reach them. The edge
  and its eight-row differential landed in `7cb5658`. **This is the failure mode to watch for in
  iteration 6**, whose `design` block is wired the same thin way today.
- **`Languages`' order was verified only transitively**, through the `Locale` `$defs` byte match.
  The order decides the `Phrases__N` numbering — the one thing about the locale `$defs` that cannot
  be read off the output, since the keys sort alphabetically whatever number they carry — so a
  submodule bump that reordered the union would have surfaced as forty-five byte failures naming no
  cause. `TestLanguagesAreInUnionOrder` derives the expectation from the glob.
- **The catalog drift check shares a parser with the tool that wrote the data.** `tools/localeprobe`
  and `catalogs_conformance_test.go` both read the YAML with `goccy/go-yaml` and the same struct
  tags, so a goccy defect would land in the data and the expectation alike. The verifier closed it
  out of band with ruamel — 0 differences, field by field, all 21 files — and the tool's head
  comment now states the gap. Nothing in the repo repeats that check.

**One process failure, recorded rather than rewritten:** `b7531dc` bundles T1, T2 and T5 in one
commit, against `AGENTS.md` §7. `tasks.md` lists all three as separate units.

The parity suite stays at its 42 red cases, which is iteration 1's baseline. No golden was
regenerated and the submodule was not bumped, so no human gate was requested.

**Carried forward:** date formatting is **not** this iteration's despite the row title it used to
carry — spec §4.1 assigns `2020-09` → `Sept 2020` and §4.2 assigns `degree_with_area`'s
substitution to iteration 9, with the renderer.

## The three gates were answered — 2026-08-08

A human answered all three open gates in one pass. What each one licensed, and what landed:

| Gate | Decision | Divergence | Result |
|---|---|---|---|
| iteration 10's distribution question | **vendor and embed all three** | D-007 | 14 render cases green, PDF included |
| `new`'s panel next-step line | **harness substitutes and re-pads the row** | D-009 | 7 of 8 `new_*` green |
| `create-theme`'s two file kinds | **write `init.lua` and the pongo2 transforms** | D-008 | recorded; the command is still unwritten |

`TestParity` went from **0 / 35 to 25 / 35** in this pass. What moved, and why:

1. **The whole PDF/PNG path landed.** `internal/renderer/typstc` runs the embedded compiler on
   wazero through three mounts. The first compile pays ~4 s for the 29 MB module and every
   document after it costs ~40 ms — an order of magnitude better than the 3.2 s the measurement
   pass reported, because that figure included module compilation each time.
2. **The font order was wrong and it was silent.** The runner searched caller folders before the
   vendored set; upstream passes `rendercv_fonts` first (`pdf_png.py:174-185`). Reversed, a
   user's `fonts/` folder would win a family-name tie that upstream gives to the packaged face.
   Fixed before any golden could have caught it.
3. **Panels wrap.** `theme_classic`'s two PNG paths do not fit on one row, and Rich folds the row
   with its continuation flush left. Nothing in the port wrapped anything.
4. **Validation errors are a table, not a sentence.** Upstream has two error panels and the port
   only had one. `internal/cli/table.go` reproduces `rich.table.Table` — all three width stages,
   each pinned by a different golden, and the third one is not hypothetical: `err_wrong_input`
   reduces a column to width zero.
5. **The custom-theme folder checks were in the wrong layer.** They ran from the CLI as a user
   error; upstream raises them from inside the validator, so they belong in the same table as
   every other record, located at `design`.
6. **Two corrections to my own work, both caught by a golden rather than by reading:**
   - the binary-name rewrite matched the token *anywhere*, and this repository's directory is
     called `rendercv-go`, so it corrupted an absolute path the port had printed correctly. It
     now matches only the token standing alone between spaces.
   - a Panel's overflow is `fold` and a Column's is `ellipsis`. Using one wrap for both split
     `err_unknown_theme`'s path across two lines where upstream cuts it with `…`.

### What is left, and what each one needs

| Case(s) | Blocker | Shape of the work |
|---|---|---|
| `cli_help`, `cli_help_short`, `cli_new_help`, `cli_render_help`, `cli_create_theme_help` | Typer's help renderer is not written | Its geometry is **not** the error table's. Measured on the goldens: the first column starts at offset 0 and the option column is 26 wide for a longest option of 24, which neither `pad_edge=False` with `padding=(0,1)` nor `pad_edge=True` explains. **Read `typer/rich_utils.py` in the submodule's venv before writing any of it** — I reverse-engineered it from goldens, got two columns right and the third wrong, and stopped rather than guess. |
| `create_theme`, `new_typst_templates` | the command is unwritten | D-008 approved the design. Both cases compare template *source*, so both stay red by construction once written; the point of writing them is the feature, not the case. |
| `err_not_yaml` | two defects, both upstream-visible | the location reports `line 1` where upstream reports `line 1 to line 2` (the span's end line is not carried), and the message lacks ruamel's ` while parsing a flow sequence.` suffix. |
| `err_missing_file`, `err_bad_override_key` | **the goldens are Python tracebacks** | 5 KB of `error_handler.py:38 in wrapper` with this machine's absolute paths. A Go binary cannot produce them and should not try. These need a divergence entry saying so, and that is a human gate. |
| `TestSchemaParity` | the test asks for a command that must not exist | It shells `rendercv-go schema`. Upstream's CLI has exactly three commands and none is `schema` — adding it would break axis 2 to satisfy axis 3. `tools/genschema` is the sanctioned oracle and `just schema-diff` is green. **The test is wrong, not the port**, and fixing it means pointing it at the generator. |
| `TestMarkdownToHTMLMatchesPython` | iteration 11's open blockers | goldmark escapes `"` as `&quot;`, orders attributes differently, and self-closes void elements where Python's `markdown` does not. Four assertions, unchanged from the last audit. |

### Two things this pass did not do

- **Nothing here was verified by a fresh context.** Every number above is what `go test -tags
  conformance ./...` printed in the context that wrote the code, which `AGENTS.md` §10.6 says is
  not parity. Iterations 10 and 12 stay unverified until `rendercv-parity-verifier` runs.
- **The goldens are still not portable.** `caseWorkDir` makes cases run where `gengolden` ran
  them, which is what lets a golden carrying an absolute path be compared at all. It carries
  *this machine's* repository path. Iteration 1's audit finding stands.

## Iteration 9 — verification

**Verified by `rendercv-parity-verifier` in a fresh context, which returned FAIL** with two
blockers, one major and seven minor findings. Every one of the four it ranked as blocking or major
is fixed and pinned by a fixture that is red without the fix. The first verifier run for this
iteration was cut off by a session limit; the second completed.

| # | Verifier finding | Resolution |
|---|---|---|
| 1 | **blocker.** `design.templates.education_entry.degree_column: null` was ignored — the port emitted the declared `**DEGREE**` where upstream omits the column. The bridge dropped every null-valued key before the merge, so an explicit null could never override a default. Reachable from a twelve-line document. | `51a4121`. A null now survives into the merge, and `design.Effective` resolves it by the field's declared type: kept for the one `str \| None`-with-default field, restored to the default elsewhere (which is the case upstream rejects at validation). Pinned by `design_null_column`. |
| 2 | **blocker.** A document with `cv.photo` rendered **silently wrong** — 157 bytes of upstream's header `#grid` missing, exit 0, no warning. The photo was hardcoded falsy on the reasoning that the corpus has none. All 16 corpus files that mention `photo:` write it null. | `c3657bd`. A *local* photo needs no download and now renders; a *URL* one returns `typstdoc.ErrPhotoDownloadUnsupported` rather than a document. Pinned by `header_photo` and a unit test. |
| 3 | **major.** `splitLines` split on `\n` alone. Python's `str.splitlines()` breaks on eight more characters, so a `summary` with Windows line endings left a bare `\r` inside the rendered Typst and a ` ` produced one line where upstream produces two. | `e7a9299`. Full CPython boundary set, `\r\n` counted once, checked against a twenty-case fixture CPython itself wrote. |
| 4 | **major.** Seven of nine `locale.Resolve` override branches were reached by **no test in the repo**. The verifier measured them correct out of band; nothing pinned them. | `d1c1290`. One fixture case exercises all nine; eight of the nine now fail if their branch is deleted. `phrases` stays covered by its unit test only — the classic theme has no template that shows it. |
| 9 | minor. `spec.md` §6's boxes were unchecked while `tasks.md` said done. | `9733253`. |
| 10 | minor. `tools/typprobe`'s comment claimed only `cli_*`/`err_*` cases lack an input; three others do. | `9733253`, which now names them. |

**Findings 5, 6, 7 and 8 stand as recorded, unfixed, on purpose:**

- **6 and 7** are coverage observations, not defects: paths reached by a unit test but not the
  corpus (custom connections, HttpUrl normalization, `current_date` parsing — all differentially
  verified correct), and paths reachable by no document at all (`dumpValue`'s float, bool and
  mapping arms). The dead arms stay because dropping them would make `Dump` partial on input the
  *binder* admits even though upstream's own pipeline crashes on it — see finding 5.
- **8** is a commit-discipline slip already in history: `60f9daa` bundles a lint refactor of
  `tools/typprobe` with the colour fix. Recorded rather than rewritten, as `AGENTS.md` §7 breaches
  have been throughout.

## Iteration 11 — both documents, and a cut that should not have happened

The Markdown half needed **no new pipeline**: upstream's `render_full_template` takes `file_type`
as a parameter, so `internal/renderer/typstdoc` became `internal/renderer/document` and `Render`
gained a format. Three branches cover every difference — template directory, processor chain, and
whether a preamble exists.

**The HTML half was cut and then uncut in the same session, and the cut is the part worth
recording.** The measurement spec §6 demanded was run: goldmark matched python-markdown on 8 of 24
documents. From the first differing line of two cases this ledger concluded the misses were "loose
versus tight lists", called that a block-layer difference no post-pass could fix, and scoped a
python-markdown block-layer port as its own iteration.

All 16 misses had **one** cause: python-markdown nests a list item at `tab_length` 4 where
CommonMark nests at 2, and the entry templates emit nested highlights at 2. Normalizing that in the
*input* makes goldmark match 24 of 24. The difference between "its own iteration" and "one rule"
was reducing the diff instead of reading its first line.

That is the third time in this port that *only a bigger port can fix this* proved false. Spec 008
§8 said only a corpus `.typ` could check the template transform; a fragment differential found a
real bug in its first run. The pattern is worth naming: **an estimate of how hard something is
belongs in the same evidence class as a parity claim — measured, not asserted.**

Mutation-checked: without the list-indent rule 16 of 24 fail, with a tab length of 2 the same 16
fail, and keeping goldmark's trailing newline fails all 24.

**One upstream oddity reproduced rather than fixed**: `Full.html` interpolates a `title` that
`render_html` never binds, so every `.html` has an empty `<title>`. The port binds nothing there
either.

**Not verified by a fresh context.** Iteration 11 has had no `rendercv-parity-verifier` pass; the
row above reports what the suite prints, not an independent audit.

## Iteration 1 was audited and it failed — the instrument itself — HUMAN GATE

**Every shipped iteration has now been audited: twelve passes, one pass (iteration 5).** The last
one examined the harness the other eleven are *measured by*, so its defects are systemic.

**The comparison path is sound.** Six mutations — an extra file, a missing file, trailing
whitespace, CRLF, an extra dotfile, and stdout drift — were all caught, with the baseline passing.
The harness does not let a wrong artifact through.

**The goldens themselves are the problem**, and both findings are `gengolden`'s:

| # | Finding | Reach |
|---|---|---|
| 1 | **Three goldens bake the generating machine's absolute paths** — `/home/nnc/…` — including 13 lines of Python traceback through the *submodule's own source* in `err_bad_override_key`, plus `err_missing_file` (6) and `err_unknown_theme` (1). Verified independently. **A Go port can never match a Python traceback**, so those cases are unachievable by construction, not by effort. `gengolden -verify` cannot see it: it regenerates at the same path and exits 0. | 3 cases permanently unreachable |
| 2 | **19 goldens bake the generation month** — the earlier entry said 18 — in *two* places: the footer and every `end_date: present` duration. They rot at each month boundary. Neither `gengolden` nor `corpus.json` pins `settings.current_date`, though upstream exposes the knob and `tools/docprobe` already uses it. | 19 cases expire monthly |

**This reframes the parity number.** `just test-parity`'s 41 failures are not 41 units of remaining
work: at least 3 can never pass, and 19 more depend on a date that has already moved. The gate the
whole port is measured against needs regenerating before its count means anything — and
regenerating `testdata/golden/` is a human gate (§5).

## The goldens were regenerated with a pinned date — GATE CLEARED 2026-08-08

`tools/gengolden` now appends `--settings.current_date 2025-03-05` to every **`render`** case (only
render — `new`, `create-theme` and the help cases take no such flag). The pin lands in each case's
own `args`, so the conformance harness replays the identical command; it is part of the case, not a
hidden setting.

42 cases regenerated, 73 files changed. No golden carries a generation date any more — every
`.typ` reads `year: 2025`, and the 19 that rotted monthly are now stable.

**The parity count did not move, and the reason is a new finding.** `render_typst_only`'s golden is
now correctly `Last updated in Mar 2025` while the port still emits `Aug 2026`, because:

> **`modelbuilder.applyOverrides` was a stub** — its whole body was `_ = overrides; return
> document`. Every dotted CLI override was silently discarded.

**Now implemented** (`override_dictionary.py:5-121`): a dotted path is walked segment by segment,
a missing *mapping* key is created and a missing *list* index is a user error — upstream's
asymmetry — with its two messages ported verbatim. Keys are applied in sorted order, because Go's
map order is random and upstream's is the CLI's insertion order. The unit test that pinned the
no-op as if it were behavior is replaced by five that pin the real semantics, including the
indexed form `render_override_indexed` uses.

Verified by hand: `render cv.yaml --settings.current_date 2025-03-05` now emits
`Last updated in Mar 2025` where it emitted `Aug 2026`.

**Traced and closed.** The harness reads its arguments from `testdata/corpus.json`, not from the
golden's `case.json` — `case.json` is a *record* of the run, not its input. The pin was reaching
the generator and not the replay. Pinning the 27 `render` cases in `corpus.json` makes both sides
read the same source; the goldens did not change at all, which is the proof the two were already
consistent.

**Errors are a Rich panel on stdout, not text on stderr** — every `err_*` golden has an empty
`stderr.txt` and a `╭─ Error ─…╮` box on stdout, exit 1. `cli.failPanel` now does that, and
`err_empty_yaml` passes. **The panel does not wrap**: a message longer than the box runs past the
right border, where upstream wraps to the inner width. That blocks the longer error cases and is
the next piece of the panel renderer.

**Parity: 41 red → 35.** Seven cases now pass — `render_typst_only`, `render_quiet`,
`render_custom_paths`, `render_override_indexed`, `render_override_theme`, `cli_version` and
`err_empty_yaml`. Axis 1
and axis 2 both have real passing cases through the binary for the first time.

That is why `--settings.current_date` has no effect, and it means `render_override_scalar`,
`render_override_indexed` and `render_override_theme` cannot pass either — four cases, one stub.
Implementing it is a real unit: parse a dotted path with numeric indices, walk or create the nodes,
set the scalar. Spec 004 §3.21 already describes the ordering it has to respect.

**The three absolute-path goldens are untouched by this**: `err_missing_file`,
`err_bad_override_key` and `err_unknown_theme` still bake `/home/nnc/…` and Python tracebacks, so
they remain unreachable by construction. Fixing those needs a different change to `gengolden` —
running upstream from a stable relative path — and is not part of this regeneration.

## The golden corpus expired daily — now fixed, kept for the record

**Found while wiring `render`, and it blocks 18 of the 42 parity cases regardless of how correct
the port is.**

`render_typst_only` now matches its golden on **exit code, stdout, stderr and file list**, and its
`.typ` differs on exactly one line:

```
golden:   day: 6,
got:      day: 7,
```

The corpus inputs write `settings.current_date: today`. Upstream resolved that to its *generation*
day — 2026-08-06 — and baked it into the preamble's `datetime(...)` and the top note. 18 golden
`.typ` files embed a date this way. **They can only pass on the day they were generated.**

This is a defect in `tools/gengolden`, not in the port: the corpus was captured without pinning the
date, exactly as `tools/docprobe` learned to do later (it passes
`--settings.current_date 2025-03-05`, which is why the 72 document comparisons are reproducible).

**The fix needs the human gate**, because it regenerates `testdata/golden/` and so changes the
contract (`AGENTS.md` §5):

1. `tools/gengolden` passes `--settings.current_date` with a fixed date, as `docprobe` does;
2. `just golden` regenerates;
3. the 18 affected cases become reproducible on any day.

Until then, `render`'s correctness is measurable only through the document differential of
iterations 9 and 11 — which is byte-exact and date-pinned, and which does pass.

## `new`'s eight cases collide with the sanctioned binary name — HUMAN GATE

`new` is implemented and its starter CVs are **byte-identical**: `tools/sampleprobe` captures them
from the vendored CLI and all seven variants `cmp` clean against their goldens. The two panels and
the greeting match too.

**One line cannot match, and it is the sanctioned divergence itself.** `new` prints

```
│   2. Run: rendercv render John_Doe_CV.yaml                                   │
```

The port must tell the user to run `rendercv-go` — that is `AGENTS.md` §1's one permitted
deviation, and printing `rendercv` would be an instruction that does not work. But the row is
inside a **fixed-width panel**, so the longer name changes the padding: substituting either
direction in the harness leaves the line three characters short or long.

So the eight `new_*` cases are byte-identical everywhere except a line that *must* differ. The
resolution is a human call between three options:

1. a `divergences.md` entry recording the line, leaving the cases red;
2. a harness rule that re-pads panel rows after substituting the binary name — narrow, and it
   weakens the width check on exactly the rows it touches;
3. accepting `rendercv` in output text as a brand rather than a command, which contradicts the
   instruction's purpose.

I did not choose. Option 2 was implemented, measured to be wrong by three characters, and reverted.

## `create-theme` cannot be byte-identical — HUMAN GATE

Measured while scoping iteration 12's remaining commands. `create-theme` writes fourteen files, and
**two kinds of them are things this port deliberately does not have**:

- `__init__.py` is Python that a custom theme executes at validation time. That is already D-002 —
  the port uses a sandboxed Lua `init.lua` instead — so the file written here must be `init.lua`,
  or D-002's feature does not work at all.
- The `.j2.typ` files are Jinja. The port ships their pongo2 transform, which is what its loader
  reads. Measured on `Header.j2.typ`: upstream's carries a newline after `{% macro image() %}`
  that Jinja's `trim_blocks` eats at parse time and the transform has already removed. **Writing
  upstream's bytes would hand the user a theme this binary renders differently from the one it just
  wrote.**

`AGENTS.md` §6.1 already sanctions template *source* diverging while output must not — but the
`create_theme` golden compares source, so the case is unreachable by construction rather than by
omission. It needs a `divergences.md` entry naming both files. Recorded, not written.

## Iteration 10's route works, 14/14 — and it needs a HUMAN GATE to land

**D-006 is proven.** typst 0.14.2 — upstream's line — builds for `wasm32-wasip1` with no patches,
runs on wazero through three preopens, and produces PDFs that match upstream's on all three things
axis 1 names.

| Measured | Value |
|---|---|
| PDF differential | **14 / 14** — every golden `.typ` with a golden `.pdf` beside it: nine themes, four ATS inputs, `input_minimal` |
| Compared on | extracted text (`pdftotext -layout`, byte-compared), page count, page geometry |
| Mutation-checked | 3 ways — cross-case, one character, one leading space. All caught |
| `.wasm` size | **29 MB** (`opt-level = "z"`, LTO, stripped) |
| Runtime | **3.2 s** per document, single-threaded wazero |

**The font risk D-006's `Watch` line named was real, and it fired twice.** Both were found by
compiling, not by reading:

1. **`@preview/fontawesome:0.6.0` is downloaded, not vendored.** `rendercv_typst/lib.typ:1` imports
   it; `get_package_path` copies only `rendercv`'s two files. Upstream fetches the rest from Typst
   Universe into `~/.cache/typst`. Spec §2.6 said the packages were resolved without a download —
   corrected.
2. **typst's embedded fonts are a third input.** `sb2nov` wants New Computer Modern, which
   `rendercv_fonts` does not ship. Without `typst_assets::fonts()`, `theme_sb2nov` renders in a
   fallback (`PhD in Computer Science` extracts as `PhDinComputer Science`) and `theme_opal` shifts
   two lines by one space — **and the other 12 of 14 cases pass anyway.** A quiet failure on 86% of
   the corpus is exactly the shape this port keeps getting caught by.

**What is committed:** `tools/typstwasm/` (the shim's Rust source, `Cargo.lock` pinning 0.14.2) and
`just typst-wasm`. No binary, no fonts.

### The gate

Landing T2, T3 and T6 of `tasks.md` puts this into the repository:

| Artifact | Size | Note |
|---|---|---|
| `typst.wasm` | 29 MB | `//go:embed`ed into `rendercv-go` |
| `rendercv_fonts` | 59 MB | 62 files, 15 folders. Not in this repo today; it is a Python package |
| `preview/fontawesome/0.6.0` | 428 KB | **not in the submodule either** — vendoring it is the divergence, because upstream downloads it |

That is ~88 MB of vendored binary and a binary an order of magnitude larger than a Go CLI usually
is. `spec.md` §5 count 3 says it needs a `divergences.md` entry; `AGENTS.md` §5 makes that file
human-gated. **Three things need a decision, and I did not make any of them:**

1. **Embed or fetch the `.wasm`?** Embedding gives a self-contained binary and a 29 MB floor.
   Fetching on first run adds a network dependency to `render`, which upstream does not have for
   the compiler.
2. **Vendor, fetch, or find the fonts?** Vendoring is 59 MB and reproducible. Finding them on the
   system is what breaks metrics silently — measured above, and the failure passed 12 of 14 cases.
3. **Vendor `fontawesome`?** The port should not fetch from Typst Universe at render time, so
   vendoring is the only route that keeps `render` offline — but it is a file upstream does not
   ship, so it is a divergence either way.

Until this is answered, iteration 10 stays not green and the parity suite is unchanged. The
measurement is real; nothing in the suite runs it yet, and `AGENTS.md` §10.6 says parity is what
`just test-parity` prints.

## Iteration 10's first measurement pass — a gate I claimed that does not exist

`plan.md` reports the measurements `spec.md` demanded before any design:

| Fact | Value |
|---|---|
| Typst compiler to target | **0.14.x** — `typst.toml` declares 0.14.0, `typst-py` is 0.14.8 |
| Compiled compiler size | **64.8 MB** native extension |
| Fonts | **77 files across 15 folders**, shipped in the `rendercv-fonts` Python package |
| `wasm32-wasip1` target | not installed locally; one `rustup target add` away |

**I then claimed the WASI-versus-subprocess route needed a human decision. It does not.**
`divergences.md` **D-006 is `approved`** and has settled it since iteration 6: typst built for
`wasm32-wasip1`, embedded, executed via wazero. Its `Watch` line even names the font risk and
assigns it here.

The mistake is worth more than the correction: **a claimed gate is a claim like any other, and
checking it costs one `grep`.** It is the same shape as spec 008 §8's "only a corpus `.typ` can
check this" and the HTML renderer's first cut — an estimate stated as a conclusion, which this
port has now produced three times and disproved three times.

So iteration 10 is **implementable now**, and its first unit is small: build typst for
`wasm32-wasip1`, compile one of the 24 byte-identical `.typ` documents, and diff its extracted text
against upstream's PDF for the same case. The fonts fail loudly there or not at all.

## Iteration 14 was verified and it failed — 11 findings

A fresh context reviewed the Lua work and returned **FAIL with four blockers**. Running it was the
right call: two of the four were *denial of service from a downloaded file*, and one silently
changed a built-in theme's artifact.

**Fixed, each pinned by a test:**

| # | Blocker | Fix |
|---|---|---|
| 1 | `themeScript` read `<theme>/init.lua` for **every** theme, built-ins included. A `classic/init.lua` beside a CV changed the artifact — measured as `page-size: "a5"` where upstream emits `"us-letter"` — without the document mentioning it. | `design.IsBuiltinTheme` gates the lookup, mirroring upstream's discriminator-first order (`design.py:36-50`). Four built-ins asserted. |
| 3 | A **cyclic table killed the process**: `local t={} t.self=t` overflowed the stack, and Go's stack overflow is a `fatal error`, not a panic — unrecoverable, exit 2. | Depth bound of 32 with `ErrTooDeep`; the design tree is four deep. |
| 4 | **No execution limit at all**: `while true do end` hung `render` forever. | A 2-second context budget on the `LState`. A declaration needs microseconds. |
| 9 | `print` wrote to the process's real stdout, prepending a line to the result panel — CLI stdout is parity axis 2. | `print`, `_printregs`, `load`, `loadstring`, `setfenv`, `getfenv`, `collectgarbage`, `newproxy`, `module` and `channel` added to the blocklist. |

**Since fixed, in a second pass:**

- **Blocker 2** — `design.ValidateScript` walks the base tree and drops a script whose shapes
  conflict with it, so a mis-typed option can no longer reach a template. Dropping is what a
  *missing* script already does; **reporting** it needs the gated message text.
- **Finding 6** — `ErrSandboxed` is gone. A blocked global is `nil`, so a script touching it fails
  with Lua's own message, which names the line. The comment where it stood records why there is no
  replacement.

**Also fixed:**

- **Finding 7, partly** — D-002's folder rules. The **name** check turned out to be already
  ported (iteration 6's `validateThemeName`); only the two folder checks were missing, and they are
  now `design.ValidateCustomThemeFolder`, **wired into `cli.Render`** so a custom theme naming a
  folder that does not exist reports upstream's message, writes nothing and exits non-zero. The
  messages needed no gate: they are upstream's own strings, so reproducing them is axis-4 parity
  rather than new text. Only the *Lua-specific* syntax/import messages are genuinely new.

**Closed in a third pass (2026-08-10):**

- **Finding 5 — `luatheme.Validate` was dead code.** It types a *document's* value against a
  script-declared option, which `ValidateScript` does not cover — that one checks the script
  against the tree. `design.EffectiveWithScript` now calls it and drops only the conflicting
  document key (`withoutConflicts`/`prunePaths`, `effective.go`), leaving the script's or the base
  tree's value underneath rather than the whole script. Pinned by
  `TestADocumentConflictingWithTheScriptIsDropped`. This was the *unreached-code* defect this port
  had hit four times; closing it means all of criterion 2 is met, not just its `ValidateScript`
  half.
- **Finding 7, the rest of it — was already fixed, and this ledger's own "Also fixed" note above
  said so.** Re-reading the code: `design.ValidateTheme` (name pattern, iteration 6) and
  `design.ValidateCustomThemeFolder` (folder existence, `*.j2.typ` presence) are both wired into
  `models.Validate` via `validate.go:101-119`. The "Open" bullet restating them as unimplemented
  was stale — never reconciled against the fix two paragraphs above it in this same file. Recorded
  as a documentation defect, not a code one.
- **Finding 11 — spec 014 undercounted in both directions.** A fresh read of `design.py:59-132`
  (spec 014 §1 behavior 3, now a table) finds **six** distinct user-visible messages, not two and
  not five: theme-name pattern, folder-missing, no-`*.j2.typ`, `__init__.py` syntax error,
  `__init__.py` import error, and a missing `<Theme>Theme` class (the last is a bare `ValueError`,
  not pydantic-wrapped). Two more paths raise `RenderCVInternalError` and are reachable only by
  mocking `importlib` in upstream's own tests — recorded, not ported. Of the six: three (name,
  folder, `*.j2.typ`) were already ported verbatim; the other three describe Python's module system
  and have no Lua equivalent to port faithfully, so the port surfaces gopher-lua's own error text
  instead (`sandbox.go`'s existing "no `ErrSandboxed`" design) — spec 014 §2 behavior 9 now records
  this as the resolution rather than an open human-gate question, since it is neither silence nor
  an invented upstream-shaped string.
- **Finding 8** — `33e9ab6` bundles `luatheme/options.go` with the `Effective` →
  `EffectiveWithScript` change that every built-in theme flows through, and lands a criterion's
  pinning test in the same commit as the code it pins. Recorded, not rewritten; no fix needed.

**Iteration 14's four criteria are now fully met** by spec 014 §4's own accounting. Still true from
the original verifier pass: the port requires `init.lua` where upstream requires `__init__.py` plus
a `*.j2.typ` folder, so **there is no input on which both sides do the same thing** — criteria 2 and
3 have no upstream oracle at all, and no corpus case exercises a custom theme.

## Iteration 14 re-verified 2026-08-10 — FAIL, 12 findings, 3 blockers

A fresh context checked the "0 open findings" claim above by constructing real documents rather
than trusting the spec's own accounting, and broke three of the four criteria.

**Blockers, 2 of 3 fixed same day (`396b982`):**

1. **Fixed.** A theme with no script must discard the whole document `design` block, not just skip
   its own options. Upstream's fallback constructs `ThemeOptionsAreNotProvided(theme=theme_name)`
   (`design.py:139-142`) — nothing but `theme` survives, so a document overriding
   `design.colors.name` on a script-less custom theme is **silently ignored** upstream; the port's
   `EffectiveWithScript` used to merge the document unconditionally regardless of script presence.
   `EffectiveWithScript` now sets `document = nil` when `script == nil && !IsBuiltinTheme(theme)`.
   Reproduced by hand: a script-less custom theme with `colors.name: rgb(9, 9, 9)` now renders
   `rgb(0, 79, 144)` — classic's own default — matching upstream; pinned by
   `TestANoScriptCustomThemeDiscardsTheWholeDocument`.
2. **Fixed, as leak prevention rather than full parity.** The exact failure this iteration exists
   to prevent reproduced again on the theme `create-theme` itself writes: `luatheme.Validate` only
   walks keys the **script** declared (`luatheme/validate.go:43`), so `create-theme`'s empty
   `return {}` checked nothing, and `design.page.size: {a: 1}` rendered
   `page-size: "<map[string]interface {} Value>"` at exit 0. `EffectiveWithScript` now also runs
   `withoutTreeConflicts` for any custom theme, dropping a document value whose shape disagrees
   with the tree-typed value already assembled at that path (map where a scalar belongs, or the
   reverse) before the final merge. `page.size` now falls back to `"us-letter"` instead of leaking
   the Go type name. Pinned by `TestADocumentConflictingWithTheTreeIsDropped`. **This is not
   upstream's behavior** — upstream *rejects* the document at exit 1, this port silently drops the
   one bad key — so it closes the garbage-leak half of the finding, not the exit-code half, which
   is blocker 3.
3. **Still open.** A custom theme's design block is not validated against anything beyond the leak
   guard above. `validate.go:120-121` returns `nil` unconditionally once the folder checks pass.
   Upstream validates the raw dict against `theme_data_model_class(**design)` (`design.py:135`),
   which rejects **unknown** keys at exit 1 — `design: {theme: mytheme, nonsense: 1, colors: {nope:
   1}}` still exits 0 here and writes the artifact. Fixing this needs the theme's script loaded
   during *validation* (`models.Validate` → `design.Validate`), not only at render time
   (`bridge.themeScript`) — a control-flow change, not a local patch, and left for a scoped
   tasks.md unit rather than rushed here.

**Majors:** D-002's own text and spec 014 §2 behavior 9 claim the port "surfaces gopher-lua's own
error text unmodified" for a script syntax/parse failure. It does not — `bridge/model.go:84-86` and
`:99-101` swallow every script error and silently fall back as though the script were absent, so a
malformed `init.lua` renders successfully with no message at all where upstream exits 1 with a named
error. Criterion 1's pinning test (`customtheme_test.go:21-44`) calls `design.Effective("mytheme",
nil)` — a nil document, the one input that cannot see finding 1, so the checked box rests on a test
that structurally cannot fail against it. Word-form YAML booleans (`yes`/`no`) against a
script-declared bool option are misclassified by `kindOf` and the user's value is deleted by
`prunePaths` rather than compared correctly (also true, pre-existing and unrecorded, for built-in
themes, where a word-form boolean reaches Typst as `yes,`/`no,` verbatim — uncompilable).

**Minor:** the folder-exists check uses `!info.IsDir()` where upstream uses `Path.exists()`, so a
regular file with the theme's name gets the wrong one of the two folder messages; `filepath.Abs`
normalizes `..` where `Path.absolute()` does not, differing whenever the input path crosses a
parent directory; spec 014 §1 row (f) undersells its own finding — the missing-class `ValueError`
does reach the user in the same validation table at `design`, just without pydantic's `loc`
machinery producing a longer path. Seven iteration-14 commits touch `specs/STATE.md`, which this
file's own header forbids for a feature commit; `2ecd807` orphaned the `themeOf` unused-func lint
issue that a later STATE.md entry claims predates all of this iteration's work — it does not.

**Verified as actually holding**: the (a)(b)(c) folder/name messages, byte-identical end to end
including exit code; the sandbox (io/os/require closed, cyclic tables bounded, the 2s budget
measured at ~2021ms); `withoutConflicts`/`prunePaths` for nested and invented-option conflicts,
which is finding 5's fix from the previous pass and survived independent attack. `TestParity`
unchanged at 34/42 — no regression.

**Not re-verified by a fresh context yet**: blockers 1 and 2's fixes above, the two majors and
three minors, and the D-002 text corrections both blockers required. Only self-checked here —
`go build ./... && go test ./...` green, `just test-parity` unchanged at the same 8 pre-existing
failures the verifier already attributed to iterations 4/12, and both fixes reproduced by hand
against the verifier's own repro commands before being pinned as tests. That is not the same as an
independent pass, which is why this ledger still reads "not green" rather than "closed".

**Not attempted here**: the three blockers need the theme script loaded during *validation*, not
only at render time — `bridge.themeScript` runs after `models.Validate` has already returned, and
finding 3 needs `theme_data_model_class`'s ForbidExtra semantics reproduced against a script's own
declared shape. That is a control-flow change to where in the pipeline a custom theme's script is
read, not a local patch, and belongs in a scoped tasks.md unit rather than a rushed fix on top of a
verification pass.

## Iteration 14 re-verified a third time 2026-08-10 — FAIL, 8 findings, 3 blockers

A fresh context checked blockers 1 and 2's fixes above by constructing real documents rather than
trusting the commit messages, and found blocker 1's fix held exactly on the input it was written
for while blocker 2's fix — and blocker 1's — each had a gap the first fix's own tests could not
see.

**Blockers, all 3 fixed same day (`31fc0fe`, `852d483`):**

1. **Fixed (was a regression in the previous pass's own fix).** `withoutTreeConflicts` treated
   `typography.font_family: Charter` — the one field in the whole tree where a bare scalar is a
   **documented** override of a mapping (`deepMerge`'s own comment; spec 006 §3.2 behavior 14) — as
   a shape conflict and silently discarded it on any custom theme. Upstream honors it; the
   pre-regression binary did too. `withoutTreeConflicts` now checks the field's dotted path against
   a named exemption (`fontFamilyPath`) before comparing shapes. Pinned by
   `TestFontFamilyStringOverrideSurvivesTreeConflictPruning`.
2. **Fixed (blocker 2 was not actually closed).** `withoutTreeConflicts` only compared *map vs
   non-map*; a **list** where a scalar belongs — `page.size: [a4]` on `create-theme`'s own empty
   `return {}` script — fell through untouched and still printed `<[]string Value>` into the
   artifact at exit 0, the identical leak the previous pass's fix claimed to close. Generalized the
   comparison to a three-way `shapeKind` (map/list/scalar). Pinned by
   `TestAListWhereAScalarBelongsIsDropped`.
3. **Fixed (worsened by the previous pass's blocker-1 fix).** The `script == nil` check the
   no-script document-discard rule used cannot tell "no `init.lua` file" from "a script that
   exists and fails to parse, run, or validate" — both hand `EffectiveWithScript` a nil `script`.
   Conflating them meant a theme with a **broken** `init.lua` now silently discarded the user's
   whole `design` block too, which is a worse outcome than before that fix shipped: upstream
   refuses to render a broken script at all, and the port used to at least apply the user's
   document, giving a visibly wrong result rather than a silently wrong one. `themeScript` now
   returns whether an `init.lua` file exists at all, separately from whether it parsed;
   `EffectiveWithScript` gained a `hasScript bool` parameter and only discards the document on the
   true no-file case. Pinned by `TestABrokenScriptStillAppliesTheDocument`.

**Majors, spec/ledger accuracy, still open:** `specs/014-lua-custom-themes/spec.md` §5 still reads
"All four acceptance criteria met" and "not verified by a fresh context" — three fresh-context
passes have now looked at this iteration and all three returned FAIL; the checked boxes in §4 are
stale. `specs/divergences.md`'s D-002 entry has two more unverified claims beyond the ones already
corrected: `rendercv-go create-theme` is described as generating `init.lua` "from the classic
theme's option tree" when it actually writes an empty `return {}` plus a comment block
(`internal/cli/customtheme.go:117-132`), and the "Instead" paragraph's justification — "themes that
*compute* and *check* their own options" — cites an optional `validate` function nothing in
`internal/schema/luatheme` ever looks for. Both need the same human gate as the two corrections
already made in this file.

**Minor:** `31fc0fe` and `852d483` are each still more than one logical fix by `AGENTS.md` §7's
strict reading (shape-comparison generalization + the font-family exemption in one; the `hasScript`
threading across two packages plus its test in the other) — smaller than `396b982`'s four units,
recorded rather than re-split further. `just check`'s three pre-existing lint issues are unchanged.

**Verified as actually holding, independently reproduced by the third-pass verifier**: blocker 1's
original fix (script-less custom theme, byte-identical `.typ` against upstream); the scripted path
unaffected (`create-theme`'s empty script still honors a document); nested conflicts, scalar-where-
group, and the sandbox properties from earlier passes. `TestParity` unchanged at the same 8-case
baseline — no regression from any of the three blocker fixes above, confirmed by both the verifier
and a self-check after landing.

**Not yet re-verified by a fresh context**: the three fixes in this section (self-checked and
hand-reproduced against the verifier's own repro commands only, same caveat as the previous pass),
and the majors/minor above. Iteration 14 stays demoted.

## Iteration 14 re-verified a fourth time 2026-08-10 — FAIL, 5 findings, 1 blocker

A fresh context checked the third pass's three fixes by trying hard to break them a second way,
rather than just re-running their own reproductions. Two of the three held under attack; the third
had a fourth door the first three passes never tried.

**Confirmed holding, independently reproduced**: the font-family exemption (fix 1, byte-identical
`.typ` against a real `__init__.py`/`init.lua` pair); the `shapeKind` list generalization (fix 2,
including a zero-byte `init.lua`); the `hasScript` threading (fix 3, syntax error / runtime error /
`return nil` / `return "hello"` / zero-byte / whitespace-only `init.lua` all keep the document;
genuinely absent `init.lua` discards it — matches upstream exactly on both). Also held under new
adversarial probes: a dotted document key colliding with the font-family exemption string, unicode
keys, 5-level nesting, a null-valued group, a built-in theme with the same malformed `page.size`
(errors identically to upstream, byte-for-byte).

**Blocker, fixed same day (`d02d593`):** a **script-side** list option leaks the identical Go-type-
name garbage the previous three passes' blockers were all about, through a door none of them
tried. `luatheme.Options` walked every Lua table as string-keyed, so a script declaring
`sections.show_time_spans_in = { "Experience" }` — the tree's one list-valued option — converted
the sequence to an **empty map**, not a list. `design.ValidateScript` then saw the empty map as a
shape conflict against the tree's `[]string` field and dropped the **whole script table**, every
other option in it included, silently, at exit 0. `Options` now detects a Lua sequence (keys
exactly `1..Len()`, nothing else mixed in) and converts it to `[]string`. Pinned by
`TestASequenceBecomesAStringList`, `TestASequenceDropsNonStringElements`,
`TestAMixedTableIsNotASequence` (`internal/schema/luatheme/options_test.go`) and
`TestAScriptListOptionDoesNotDropTheRestOfTheScript` (`internal/renderer/bridge/luatheme_test.go`).

**Major, undeclared divergence until this pass, fixed as part of the same blocker:** a script
declaring the tree's list-valued option silently discarding every *other* option in the same table
was reachable from an ordinary theme and named in neither `spec.md` nor `divergences.md` before
this fix. Closed by the same commit; no longer a live divergence.

**Minor found while reproducing the blocker's own repro, fixed same day (`735a17a`):** the
font-family exemption from the previous pass was **one-directional** — it only protected a document
*scalar* against a mapping tree value, so a script declaring `typography.font_family = "Lato"`
against a document override in the five-element **mapping** form silently lost the document's value
(`typography-font-family-body: "Lato"` where upstream and the pre-regression port both give
`"Charter"`). Tracing it further found the exemption was needed in **two** places, not one:
`withoutTreeConflicts` (which the previous pass's fix already covered directionally) and
`withoutConflicts`'s use of `luatheme.Validate` (which had no exemption at all and pruned the
document's override right back out even after the first check let it through). Both made
unconditional for the one path where either shape is valid. Pinned by
`TestFontFamilyMappingOverrideSurvivesScriptConflictPruning`.

**Major, spec/ledger accuracy, still open:** `specs/014-lua-custom-themes/spec.md` §5 still reads
"All four acceptance criteria met" and "not verified by a fresh context" — this is now the fourth
verified-FAIL pass and the text has not been corrected across any of them. `specs/divergences.md`'s
D-002 entry's two remaining unverified claims (`create-theme` "from the classic theme's option
tree", the optional `validate` function) are independently confirmed still wrong and still
uncorrected: upstream's generated `__init__.py` is 857 lines: the port's `init.lua` is 15 ending in
`return {}`.

**Minor, commit discipline:** `396b982` and `31fc0fe` each bundle two independent fixes with their
own tests — genuine `AGENTS.md` §7 violations. `852d483`'s `hasScript` threading across two
packages plus its callers and test is judged **not** a bundle — one logical change touched several
files, which §7 does not forbid. `d02d593` and `735a17a` (this pass's own commits) were split one
fix per commit deliberately, unlike the two before them.

`TestParity` unchanged at the same 8-case baseline across every command run in this pass — no
regression from either fix. **Not yet re-verified by a fifth pass.** The still-open items are the
forbid-extra rejection on unknown design keys (cut to a future scoped unit, needs the script loaded
during validation) and the two spec/ledger staleness majors above, which need editing rather than
code.

## Iteration 11 was verified and it failed — demoted from green

A fresh context reviewed the Markdown and HTML work and returned **FAIL with three blockers**. The
24/24 corpus claim is real and non-vacuous — mutations confirmed, fixtures regenerate with zero
diff — but **the corpus is much narrower than 24 suggests**: 8 cases share one byte-identical
`.md`, leaving 14 distinct Markdown documents.

| # | Blocker | Status |
|---|---|---|
| 1 | **goldmark escapes `"` as `&quot;`; python-markdown does not.** Any double quote anywhere in a CV produces a differing `.html`. Reproduced independently: upstream `<p>He said "hello" to me.</p>`, port `<p>He said &quot;hello&quot; to me.</p>`. | **open, and now pinned red**: `process/html_conformance_test.go` differentials against CPython's own output and fails on exactly this case. **Two fixes were tried and both are wrong**, which is the useful part. A blanket `&quot;`→`"` post-pass corrupts attribute values. A custom goldmark writer overriding the renderer's text `Write` fixes the text case and **breaks image alt text** — goldmark renders alt through the same text path into an *attribute*, producing `alt="alt "q""`, unparseable HTML. The four-case differential passed it; extending the fixture to eleven caught it. A real fix must distinguish text from attribute context inside goldmark's renderer. |
| 2 | **goldmark drops raw HTML** (`WithUnsafe` off); python-markdown passes it through. `<b>x</b>` becomes `<!-- raw HTML omitted -->`, and a `<tag>` in ordinary prose triggers it. | **fixed** — `WithUnsafe` matches python-markdown's passthrough. Not a security decision: the input is the user's own CV, which the port already renders verbatim into Typst. |
| 3 | **YAML block scalars were not parsed at all** — `key: \|` and `key: >` yielded the raw indicator, so a literal-block `TextEntry` rendered as `\|` in **all three** artifacts. | **fixed.** `buildLiteral` read `n.Start`, the *indicator* token, instead of `n.Value`, the body (`yamlreader/build.go:232`). Present since the reader was first written; a narrow verifier pass confirmed and located it. All four forms — `\|`, `>`, `\|-`, `>-` — now match ruamel, and a block-scalar CV renders **byte-identical** to upstream's `.typ`. |

**`plan.md` §2's "this is a library substitution and not a divergence" is now known to be wrong**,
and it was argued from exactly the 24 corpus cases that cannot see these. The same paragraph that
corrected an earlier over-estimate made an under-estimate in the other direction. Findings 4
(list-valued `email`/`phone`/`website` silently taking the first value) and 5 (the goldmark
substitution itself) both need `divergences.md` entries — human-gated.

**What this says about the ledger generally:** iteration 11 sat marked green for this whole
session on the strength of a corpus differential, and one `"` was enough to break it. Iterations 9
and 14 were verified and both came back FAIL. **The only iterations whose green has survived
contact with a fresh context are 9 and, partially, 14 — every unverified green in the table above
should be read as provisional.**

## Iteration 2 was verified narrowly, and it failed

The block-scalar report from iteration 11's verifier was a side observation, so a second pass
confirmed it, bounded it and found the cause. All three held.

**The cause:** `buildLiteral` read `n.Start` — the `|` indicator token — where goccy keeps the
block's body in `n.Value` (`internal/schema/yamlreader/build.go:232`). Present since the reader was
first written, **not** from this session's `scalarRaw` change.

**Why nothing caught it:** `yamlreader_test.go`'s "star in a literal block" case *does* feed a
literal block, but asserts only `Kind == KindString` — never `Raw` — so it passed on garbage. The
174-entry scalar corpus has no literal or folded entry, and no golden CV input contains a block
scalar outside comments. **A green parity run said nothing about it.**

That is the clearest example this session produced of the difference between a test that runs and
a test that checks. It also means **iterations 1–8's greens rest on suites of unknown
discrimination** — this is the first of them to be probed, and it failed.

## Iteration 4 was verified and it failed — axis 4's green is not real

The 25-record differential **is** discriminating — mutating a message, a coordinate and a schema
location each broke it, and it has no iteration-2-shaped defect. But it gates a fraction of the
axis, and two holes sit outside it.

| # | Finding | Status |
|---|---|---|
| 1 | **The whole `settings` block was unvalidated** except `current_date`. `settings:\n  bogus: 1` rendered at exit 0 where upstream reports an unknown key. `STATE.md` deferred it to iteration 12; `specs/012-cli/spec.md` recorded the validation text as iteration 4's and done — **nobody owned it**, which is how it survived. | **fixed.** `settings.ValidateUnknownKeys` rejects keys neither `Settings` nor `RenderCommand` declares, and it is wired into `models.Validate`. Measured: `settings.bogus` and `settings.render_command.alsobad` both report `This field is unknown for this object. Please remove it.` — the dictionary's own text, so no new string was invented. **Wrong *types* under settings are still unvalidated.** |
| 2 | **Only the first validation record reached the user.** `UserValidationError.Error` returns `Errors[0].Message`, so every location, input value and later record was discarded — a document with three problems reported one line and named no field. | **fixed.** `cli.fail` now walks the records and prints `location: message` for each. Measured against upstream on a three-error document: same three records, same locations, same message text. The **shape** still differs — upstream renders a Rich table, which is iteration 12's remaining work — but the information loss is gone. |
| 3 | **Exit code on a validation error was 4; upstream's is 1** — the value every `err_*` golden records. Measured across twelve invalid documents. | **fixed**, `exitValidationError` |
| 4 | The differential covers 6 of 13 dictionary rows and 2 of 8 username rules; the verifier hand-checked 12 uncovered cases and **all matched byte for byte** — the gap is that nothing in the suite holds them. | **open** (coverage, not correctness) |
| 5 | `modelbuilder/yamlerror_test.go:271` skips unconditionally — a test that asserts nothing. | **open** |

**The ownership gap in finding 1 is the lesson.** Two specs each recorded the work as the other's,
and the ledger agreed with both. Nothing was hidden; the cross-reference was just never followed.

## Iteration 3 — re-audited, and the repairs themselves failed

**This is the most important entry in this file.** Iteration 3 was audited, five findings were
fixed, and a *second* fresh-context audit of those fixes found that **two of the five closures were
wrong and one of my repairs created a new defect**:

| # | What the re-audit found | Status |
|---|---|---|
| A | **My "misdiagnosis" call on the DOI record was itself the misdiagnosis.** Upstream *does* emit two rows; I concluded otherwise from a raw `errors()` dump, not seeing that RenderCV's own handler unpacks `caused_by` into a second row. A short bad DOI emits both rows in Go, so the transport works — only the too-long case loses one. | **open, and worse than before**: the earlier note claiming this closed is now retracted |
| B | **The `start_date: present` fix is half-closed.** The rejection is right, but Go then runs the ordering check anyway and emits a **third** record upstream never produces. `check_and_adjust_dates` is `mode="after"` and never runs when a field failed. | **open** |
| C | **`adjustDates` was applied to every entry type**, but `check_and_adjust_dates` lives on `BaseEntryWithComplexFields` alone. A publication with a stray `start_date` got `end_date: present` synthesized and rendered a date range where upstream aborts. **Introduced by the fix for finding 1.** | **fixed** — gated to Education/Experience/Normal, the three that call `ComplexSpec` |

**Closures that do hold**: the three date rewrites and integer-year dates, both byte-identical, and
the fixture gap (0 catches → 4).

**The lesson, and it is the session's sharpest**: a repair is not a closure. Nine audits found
defects in code that passed its tests; this one found defects in the *repairs*, made by the same
context that wrote them. The rule `AGENTS.md` §5 states for features — the verifier is never the
author — applies to fixes with at least as much force.

Also observed and unrecorded elsewhere: **`rendercv-go render` returns the wrong exit code on
success for some flag combinations.** `render cv.yaml` and `render cv.yaml --quiet` exit 0, but
`render cv.yaml -nopdf -nopng` and `render cv.yaml -typ out.typ` **exit 70** while writing every
artifact and printing the success panel. 70 is `Execute`'s initial `code` value, so either the
subcommand's `RunE` result is being lost or `root.Execute` is erroring after it ran — not yet
traced. The re-audit reported these runs as *hangs*; measured here they terminate immediately with
the wrong status, which is likelier what its harness saw.

Probing further muddied rather than clarified it: under `2>&1 >/dev/null` the panel appears, and
under `2>/dev/null` it also appears, which cannot both be true of a single write — so the
stream the panel goes to needs establishing before the exit code does. Recorded at the point the
tracing stopped being reliable rather than guessed at.

**This is an axis-2 defect on the success path** — every golden records exit 0 — and it is more
serious than any single parity byte, because a caller cannot tell a successful render from a
failure.

**All five are resolved**: three fixed with byte-identical differentials against upstream
(the date rewrites, `start_date: present`, integer-year dates), one fixture gap closed and
mutation-checked from **0 catches to 4**, and one — the "dropped" DOI record — resolved as a
**misdiagnosis**: upstream emits a single record there, exactly as the port does.

**It is not green.** The audit that found these was a fresh context; the repairs were not. Calling
it green would need a re-audit, which is the same standard every other row here is held to.

The DOI case is worth reading in full below: four plausible fixes were tried and each was
disproved by a different existing test or measurement. What settled it was dumping upstream's raw
`ValidationError.errors()` rather than reasoning about the port's pipeline.

| # | Finding | Status |
|---|---|---|
| 1 | **`check_and_adjust_dates`' three rewrites were computed and discarded.** `bases.adjustDates` applies them to the typed model; `Dump` reads the *node* and nothing carried the result across. Three artifact defects at once: a `date` beside a range rendered the range, a lone `end_date` vanished, and **a lone `start_date` blanked the entire entry** — company and position included — because `EntryDate` failed and the whole expansion was skipped. A lone `start_date` is one of the most ordinary shapes a CV has. | **fixed.** `entries.adjustDates`; all three rules now render **byte-identical** to upstream. |
| 2 | `start_date: present` was accepted and rendered; upstream rejects it. | **fixed.** The two fields now differ exactly as upstream does: `start_date`'s scalar check gets **no reference date**, so `present` fails there, while `end_date` keeps one and accepts it through its own `Literal["present"]` arm. The message is upstream's — `This is not a valid date! Please use either YYYY-MM-DD, YYYY-MM, or YYYY format.` — which needed translating away from spec §4.15's internal-error text, right for that path and wrong for this one. |
| 3 | Integer-year `start_date`/`end_date` rendered an **empty** date column. | **fixed.** `stringFields` carried only strings and lists, so an integer date was dropped and the placeholder never existed — `Dump` was right, as the audit said, and the loss was one map later. Now byte-identical to upstream on `start_date: 2000, end_date: 2005`. |
| 4 | The DOI-URL-too-long record carries an empty schema location and is **dropped**: the user is told a section has problems and never which field. | **open, and narrower than it looks.** Entry errors normally reach stderr fine — the pipeline splices a wrapper's `Children` into the top-level list, and `cv.sections.e.0.start_date` prints correctly. Only the **empty-location** child is lost, because the splice has no path to prepend to. Making `cli.fail` recurse into `Children` surfaces it but **double-prints every other entry error**; that was tried and reverted. The fix belongs in the splice, not the renderer — **and a second look says the splice already handles it**: an empty-location child keeps the wrapper's own location, which would print. So the record is lost somewhere between `ValidatePublicationEntry` appending it and `spliceChildren` receiving it. **Resolved: the record is not dropped, and the finding was misread.** Dumping upstream's raw
`ValidationError.errors()` for the too-long-DOI case yields **one** record — the wrapper at
`('cv','sections','p')`. The port produces the same one, and
`TestAChildWithNoLocationCollapsesIntoTheWrapper` verifies the collapse deliberately.

What the audit saw as "a second row" is the wrapper's own **input** column, showing
`https://doi.org/10.aaa…`. So the real gap is narrower and different: **`cli.fail` prints location
and message but never the offending input**, which upstream's table shows for every record. That
applies to all validation output, not just this case.

Four fixes were tried and reverted along the way — recursing in `cli.fail` (double-prints), keying
`dedup` on message (breaks upstream's `photo` collapse), relocating the record (contradicts spec
§3.12), and exempting spliced children from dedup (contradicts the collapse test). Each was
measured; none survived. |
| 5 | The `Dump` oracle fixture had **zero** cases exercising `check_and_adjust_dates` — which is why finding 1 shipped green. | **fixed.** `tools/dumpprobe` now captures all three rewrites from upstream: `date_beats_range`, `lone_start_date` (which upstream dumps with `end_date: present`) and `lone_end_date` (which becomes `date`). Mutation-checked: deleting `adjustDates` fails 4 subtests where it previously failed none. |

Mutation probes all passed: field reordering, a dropped `Required`, and a changed message each turned tests red. The entry suite **is** discriminating for what it covers; finding 5 is that it does not cover the rewrites.

**Also observed, outside the audit's scope:** `rendercv-go render` writes `rendercv_output/` relative to the **working directory**, where upstream resolves it relative to the **input file**. Not yet recorded as a finding anywhere else.

## Iteration 8 was audited and it failed — eight for eight, and it should not have been green

| # | Finding | Status |
|---|---|---|
| 1 | **All five `markdown_to_typst` divergences measured back in iteration 8 are still live end-to-end** — dropped image, raw HTML escaped, autolink, link title, doubled backtick — and none is in `divergences.md`. | 1 of 5 fixed (below); 4 open |
| 2 | **The link-title case emits uncompilable Typst**, not merely a byte diff: `[t](u "ti")` produced `#link("u "ti"")[t]`, an unbalanced string literal, and `typst.compile()` on the artifact fails with `expected comma`. Upstream's compiles. | **fixed.** `linkTitlePattern` strips the title as python-markdown does; the comment claiming titles "do not appear in RenderCV's templates or its measured corpus" was the assumption that hid it. |
| 3 | **Iteration 8 was marked green while this very file recorded the five as "Open for the human gate"** — §10.2 (no green with a failing case) and §10.5 (no silent divergence) both. | **the row is wrong and this section is the correction** |

Parity **held** on everything else probed: `MakeKeywordsBold` with regex metacharacters, overlapping keywords, keywords inside URLs and inside `**bold**`; and minimal-required-field entries across all eight types with one-item highlights and blank-line summaries — both differentials empty.

## Iteration 6 was audited and it failed — nine for nine

| # | Finding | Status |
|---|---|---|
| 1 | **Font families were validated as a closed 17-member enum.** Upstream declares `SkipJsonSchema[str] \| Literal[...]` (`font_family.py:30`), so the literals document the JSON schema and the bare `str` arm accepts **anything** — a user's system font renders there and was rejected here. `fontfamily.go:40-51` had the rule written down correctly all along; the tree generator emitted the fields as a plain literal and nothing reconciled the two. | **fixed.** `theme: opal` with `font_family.body: Charter` now exits 0 where it exited 1. |
| 1a | **A partial `font_family` mapping does not deep-merge**, found while fixing the above. On `theme: opal` (font Lato) plus `font_family.body: Charter`, upstream emits Charter for `body` and **`Source Sans 3`** — the *base* default — for the other four. pydantic builds a fresh `FontFamily` from the document's mapping, replacing the theme's wholesale. **Two attempts to fix this by moving the widening each produced a different wrong answer**; the note in `effective.go` records what upstream actually does. | **open** |
| 2 | **Design errors come out in the wrong order** — the port runs a type pass over the whole block before the value pass, so a type error precedes an enum error that upstream reports first. Axis 4 requires the same ordering. | **open** |
| 3 | The missing-custom-theme-folder error carries **no location**; upstream reports it at `design`. | **open** |

Verified clean: colour normalization across named, short-hex, `hsl`, alpha and `rgba` forms — all byte-identical; `design.Effective`'s nested layering on 7 of 8 non-classic themes; and the five requested option-validation messages.

## Iteration 7 was audited and it failed — ten for ten

| # | Finding | Status |
|---|---|---|
| 1 | **`phrases`, `month_abbreviations` and `month_names` were declared with no value type** (the binder documents the zero value as "never checks"), so four documents upstream rejects rendered at exit 0. | **two of three fixed.** The month lists now carry `ValueStringList`: `month_names: hello` reports `locale.month_names: This field should contain a list of items but it doesn't.` — upstream's location and text. **`phrases` is still untyped**: the binder has no mapping shape to declare, so `phrases: hello` and `phrases: {bogus: x}` are still accepted. Left recorded rather than half-fixed. |
| 2 | **A short month list panicked the renderer** — raw stack trace, exit 2, where upstream's `IndexError` goes through its error handler and exits 1. Only English is *required* to supply twelve months (spec 007 §3.2 10a), so a non-English variant with fewer is accepted by both sides and then crashed this one. | **fixed.** `monthAt` bounds-checks. It returns the empty string, which is **not** upstream's behavior — upstream fails — but this function has no error path to use. A blank month is wrong; a stack trace is worse, and the gap is recorded rather than papered over. |

The valid path is in good shape: the verifier diffed german, hungarian, japanese, vietnamese, russian, turkish and hindi across three themes — month names, abbreviations, `present`, `last_updated`, the time-span phrases, `DEGREE_WITH_AREA` with and without a degree, and per-field overrides on a non-English base — **all byte-identical**.

## Iteration 5 was audited and it PASSED — the first one to

Eleven audits, one pass. Both questions came back clean:

- **`just schema-diff` is discriminating, not vacuous.** Four mutations of the generator — a
  changed description, a dropped property, a flipped `required` entry, two properties reordered —
  each drove it to exit 1 with 4, 14, 17 and 12 diff lines. Baseline exits 0.
- **The committed reference is upstream's live output, not a stale snapshot.** Regenerating
  `schema.json` from the vendored Python at `2eba248 (v2.8)` produces bytes identical to both the
  committed file and `go run ./tools/genschema` — all three md5 `1da645c9…`, 406444 bytes. Nothing
  in the tree embeds a copy of the reference.

**So Axis 3's "closed" is real**, and it is the only axis claim in this ledger that has survived an
independent check. It is worth noting *why* it survived: the gate is a byte diff against a file
regenerated from upstream, not a suite of hand-written expectations. Every iteration that failed an
audit was gated on assertions someone wrote; this one is gated on upstream's own output.

## Two measured behaviors awaiting the human gate

Neither is written into `specs/divergences.md`; that file is human-gated (`AGENTS.md` §5) and this
is the request.

1. **A non-string extra key on an entry.** `flag: true` or `ratio: 3.5` makes upstream die with an
   uncaught `TypeError: sequence item 3: expected str instance, bool found` (exit 2). The port
   exits 0 and renders, dropping the placeholder. Measured by the verifier.
2. **A `design` block that omits `theme`.** Upstream dies with an uncaught `KeyError: 'theme'`
   (ruamel `comments.py:854`, exit 2); the port defaults to `classic` and renders. Measured while
   building the locale fixture case.

Both are upstream *crashes*, not upstream behavior, and in both the port is friendlier. That is
still a divergence under axis 2 — same input, different exit code — and it is the human's call
whether to reproduce the crash, record the divergence, or leave it.

## Log

| Date | Event |
|---|---|
| 2026-08-06 | Repo bootstrapped; upstream pinned at v2.8; parity contract written. |
| 2026-08-06 | Iteration 1 green: 42-case corpus, gengolden, 351 golden files, red parity suite. |
| 2026-08-06 | Iteration 2 green with cut scope: reader, binder, overlay merge, cv, entry bases, sections. Conformance suite unchanged (42 red by design). Six items carried to iteration 3. |
| 2026-08-06 | Parity bug found and fixed: section-title capitalization used `unicode.ToTitle`, which is rune-to-rune and cannot express Python's `str.capitalize()` (`ßeta` → `Sseta`, `ﬁle` → `File`). The failing rows had been dropped from the test table rather than reported. |
| 2026-08-06 | Iteration 3 (entry types) started: spec investigation kicked off. |
| 2026-08-06 | Iteration 3 green with cut scope: nine entry types, registry in union order, dispatcher, real entry validator, `models.Validate` -> `cv.Validate`. Iteration 2 carried items 2 and 6 closed. Conformance unchanged at 42 red by design. |
| 2026-08-06 | Verifier returned FAIL on iteration 3 with three blockers: the entry-problems wrapper carried `rendercv_other_error` instead of `rendercv_entry_validation_error`, exact-date failures carried `value_error` instead of `rendercv_other_error`, and entry error ordering did not interleave base and own fields. The first two were fixed (`9ddd896`); the third is cut to iteration 4. Three tests had asserted the port's codes rather than upstream's, which is why the suite was green -- the reason the verifier is never the agent that wrote the code. |
| 2026-08-06 | Iteration 3 re-verified by a second fresh verifier: both code fixes confirmed correct and complete against the vendored Python and mutation-tested, no blockers. Seven findings cleared or cut (`c911d27`) — three tests were still asserting Go constants or bare non-emptiness where a literal upstream value was needed, one §8 criterion was a tautology and is now cut, and two stale claims in spec 003 are corrected in the spec text itself rather than only in this ledger. |
| 2026-08-06 | Iteration 4 (validation-error parity) started. Findings: the trailing period at `pydantic_error_handling.py:94-95` is unconditional, so every message iteration 3 emits is one character short; dictionary lookup is substring-with-break, not equality; `end_date` errors end in `!.` because upstream's literal ends in `!` and the period rule appends anyway, which also makes `error_dictionary.yaml`'s own `end_date` row dead code. Coordinate columns are never user-visible (`progress_panel.py:14-36` discards them in both code paths and no machine-readable error mode exists), which resolves iteration 2 carried item 1 as internal-only. Validation errors go to stdout with an empty stderr, which our Go side currently reverses -- an iteration-12 bug with a golden already pinning it. |
| 2026-08-07 | Iteration 7 green. The 45 locale `$defs` land byte-identical, taking the port from 18 of 227 to 63 and Axis 3 from "blocked on 6–7" to "blocked on 6". |
| 2026-08-07 | Two locale behaviors found by measuring rather than reading, both fixed inside the iteration: the twelve-element month bound is `EnglishLocale`'s alone, so applying it to every member rejected documents upstream accepts; and a null `language` is a tag failure rather than an absence. |
| 2026-08-07 | Verifier returned FAIL on iteration 7 with three findings, all closed: `ValidateCatalog` was unreachable from `rendercvmodel.go`, so the rules T1 and T2 shipped could not be reached by any document; `Languages`' order was pinned only transitively through the `$defs` bytes; and the catalog drift check shares a YAML parser with the tool that generated the data it checks. The `design` block is wired the same thin way today — iteration 6 must not repeat it. |
| 2026-08-07 | Iteration 6 green with cut scope: the 161 design `$defs` plus the three settings ones. **Axis 3 closed** — all 227 byte-identical, `just schema-diff` exits 0. |
| 2026-08-07 | Verifier returned FAIL on iteration 6 with twelve findings, two blockers, all closed or cut. The pipeline was deleting the second element of every `design` and `locale` location, which the port never produces: `design.colors.body` became `design.body` and reached the user as an internal error. Unreachable until this iteration emitted the first non-`theme` location, and invisible to tests that stop at `models.Validate`. |
| 2026-08-07 | Iteration 14 added to the table: the D-002 Lua custom-theme path, moved out of iteration 6 by its plan §7. |
| 2026-08-07 | Iteration 8's spec completed: all seven `templater/` modules and the 25 templates measured. Two findings that reach the plan — upstream deregisters five Markdown block processors on the Typst path, so no Go Markdown library works as-is; and the template vocabulary is seven tags, five filters and two Python methods over 32 `splitlines()` sites, so the pongo2 transform has to be mechanical. |
| 2026-08-07 | Iteration 8 Wave A: nine of ten processors landed, 195 measured subtests. The markdown→Typst parser replaces goldmark on that path — upstream deregisters five block processors, and `hr` and `indent` are **not** among them, so `---` renders as nothing and a four-space line is a code block. 101 differential rows are pinned in testdata. |
| 2026-08-07 | A human gate I invented was withdrawn. Iteration 8's T1 claimed the parser choice needed a `divergences.md` entry; that file is scoped to deviations from upstream and every entry names what the user notices, which here is nothing. Upstream uses python-markdown, so goldmark was never its choice either — picking between two Go libraries is a `plan.md` decision. The false gate blocked the iteration for two turns. |
| 2026-08-07 | Iteration 8 green with cut scope: ten processors, the pongo2 engine, the template transform, and a 52-fragment differential against Jinja. Wave C's corpus cases move to iteration 9 with iteration 6's T10. |
| 2026-08-07 | Verifier returned FAIL on iteration 8 with three blockers, all fixed. The one that matters: I had argued in spec §8 that the transform could only be checked by a corpus `.typ`, which is false for fragments — and that argument is what hid a trailing-newline bug adding a blank line to every entry and section of every artifact. |
| 2026-08-07 | Open for the human gate: five measured `markdown_to_typst` divergences — a dropped image, raw HTML, an autolink, a link title and a doubled backtick — all reachable from ordinary CV text. Unlike the parser-choice gate I invented and withdrew, these are user-visible. |
| 2026-08-07 | Iteration 9 opened by closing iteration 8's debt: `process_date` and `render_entry_templates`, both measured against upstream on a validated `EducationEntry`. The orchestrator is what made the other nine processors reachable — before it, nothing expanded a theme template. |
| 2026-08-07 | **Iteration 14 wired.** `bridge.Resolve` looks for `<theme>/init.lua` beside the input file — `validate_design`'s position, since the options must exist before anything reads the effective tree. Three tests drive it through real files. A theme with no script is bit-for-bit unchanged, which is what the nine built-ins and all 24 documents rely on. Still unverified by a fresh context, and a *failing* script is silent until its message text clears the gate. |
| 2026-08-07 | **Iteration 14's four criteria met.** A script's declared default *is* the option's type — a Lua declaration has no annotation but always has a value, so a script cannot claim a type it does not demonstrate. Smaller than upstream's pydantic annotations and honest about it. **Unverified and unwired**: no fresh-context pass, no corpus case exercises a custom theme, and `validate_design`'s position in the pipeline still does not look for `init.lua`. |
| 2026-08-07 | **Iteration 14's option layer met.** A script's table becomes an override layer sitting between the theme's YAML and the document's block — its declarations are defaults, so they must lose to the document and win over the base tree. `Effective` is now `EffectiveWithScript` with a nil script, so the nine built-in themes take a path no wider than before. |
| 2026-08-07 | **Iteration 14's sandbox met.** `internal/schema/luatheme` closes the seven globals that can leave the process and names each in a table-driven test. D-002's replacement is only worth having if it is actually closed — upstream executes arbitrary Python during *validation*, on a file that may arrive with a downloaded template. |
| 2026-08-07 | **Iteration 14's first criterion met.** A custom theme with no script falls back to `ClassicTheme` with its name set — and `design.Effective` already did it, so the unit was a test rather than code. That is the criterion that could have regressed all nine built-in themes and all 24 documents, which is why it was ordered ahead of anything Lua. |
| 2026-08-07 | **Iteration 14 specced.** The divergence is approved in advance, so the work is defining the Lua contract rather than choosing one. Behavior 4 — a theme folder with *no* script — is ordered first because it is the path all nine built-in themes and all 24 corpus documents already take, and therefore the only part of this iteration that can break something that currently works. |
| 2026-08-07 | **Correction: iteration 10 was never gate-blocked.** The route is D-006, `approved` — I asserted a human gate without reading `divergences.md`. Third instance in this port of an estimate stated as a conclusion, and the cheapest one to have avoided. |
| 2026-08-07 | **Iteration 10 measured.** Target compiler 0.14.x, a 64.8 MB native compiler, 77 font files in a Python package, and no `wasm32-wasip1` target installed. Both routes — WASI on wazero and a subprocess — require a divergence entry, so the choice is human by construction and the plan stops rather than picking one. |
| 2026-08-07 | **Iteration 10 specced.** The behavior is small — three inputs decide whether a PDF matches: the font set, the vendored Typst package, and the compiler root. The spec refuses to assume the WASI route works and names the three counts that must precede any design, because two iterations in a row have had an estimate stated as a conclusion. |
| 2026-08-07 | **The parity suite has its first green case: `cli_version`.** 41 red, down from the 42 that iteration 1 established as the baseline. `--version` prints upstream's version and no binary name, so it is the one CLI output the sanctioned divergence does not touch. |
| 2026-08-07 | **`new` wired.** `tools/sampleprobe` captures the starter CV per theme and locale from the vendored CLI; all seven variants are byte-identical against their goldens, as are both panels and the greeting. The eight cases still fail on one line — the `rendercv render …` instruction, which must name this binary and so changes a fixed-width panel row's padding. Recorded for the human gate. |
| 2026-08-07 | **Iteration 12 started.** `render` is wired end to end: overlays, dotted overrides, path placeholders, the five negative and five path flags, and Rich's result panel — whose geometry was recovered from the goldens, including a duration column the harness erases. `render_typst_only` matches on exit code, stdout, stderr and file list, and differs only on the baked generation date. |
| 2026-08-07 | **Corpus defect found: the goldens expire daily.** 18 `.typ` goldens embed the day they were generated because `gengolden` never pinned `settings.current_date`. Recorded for the human gate; it blocks those cases independently of the port. |
| 2026-08-07 | **Iteration 1 audited — FAIL. Every shipped iteration is now audited: twelve, one pass.** The harness's comparison path is mutation-tight, but three goldens bake the generating machine's absolute paths — including Python tracebacks through the submodule, which no Go port can ever reproduce — and 19 bake the generation month. So the parity suite's 41 failures include at least 3 that are unachievable by construction. |
| 2026-08-07 | **Iteration 5 audited — PASS**, the first of eleven. `just schema-diff` catches all four mutations tried, and the committed `schema.json` is byte-identical to a fresh regeneration from the submodule. Axis 3's closure is confirmed independently. The distinguishing feature: it is gated on a diff against upstream's own output, not on hand-written expectations. |
| 2026-08-07 | **Iteration 7 audited — FAIL, ten for ten, and demoted.** A short month list on a non-English locale — which both sides *accept* — panicked the renderer with a raw stack trace at exit 2. Fixed. Three of the ten locale fields turn out to carry no value type at all, so four documents upstream rejects render happily. The valid path is byte-identical across seven languages and three themes. |
| 2026-08-07 | **Iteration 6 audited — FAIL, nine for nine, and demoted.** Font families were validated as a closed enum where upstream's `SkipJsonSchema[str]` arm accepts anything, so a system font was rejected — and the repo's own `fontfamily.go` had documented the correct rule the whole time. Fixed. Chasing it turned up a second defect in how a partial `font_family` mapping merges, which two attempted fixes each got wrong in a new way; recorded rather than guessed at again. |
| 2026-08-07 | **Iteration 8 audited — FAIL, eight for eight, and demoted.** A Markdown link title reached the Typst as an unbalanced string literal, so the document **would not compile** — the code comment asserted titles never appear. Fixed. The other four measured divergences are still live and still unrecorded, which means the iteration was green against §10.2 and §10.5 the whole time. |
| 2026-08-07 | **Iteration 3 audited — FAIL, seven for seven.** The date-adjustment rewrites were computed into the typed model and thrown away, because `Dump` reads the node; a lone `start_date` blanked the whole entry. Fixed and differentially byte-identical. Four findings remain open, including `start_date: present` being accepted where upstream rejects it. |
| 2026-08-07 | **Iteration 4 verified — FAIL.** Axis 4's green was resting on a differential that gates 6 of 13 dictionary rows. Two blockers found: `settings` is entirely unvalidated beyond `current_date` (each of two specs recorded it as the other's work), and only the *first* validation record ever reaches the user, so error locations are never user-visible. Exit code 4→1 fixed. |
| 2026-08-07 | **Block scalars fixed** (iteration 2). `buildLiteral` read the `\|` indicator instead of the block body, so every block scalar in every CV was replaced by `\|` in all three artifacts. All four forms now match ruamel and a block-scalar CV renders byte-identical. The existing test fed a literal block and asserted only its Kind, so it passed on garbage. |
| 2026-08-07 | **Iteration 11 verified — FAIL, demoted.** A `"` anywhere in a CV breaks the HTML (goldmark escapes it, python-markdown does not); raw HTML is dropped; and YAML block scalars turn out not to be parsed at all, which is iteration 2's reader and affects all three artifacts. The 24-case corpus could see none of it — 8 of those cases share one identical `.md`. |
| 2026-08-07 | ~~**Iteration 11 green**~~ (unverified by a fresh context). Both text documents byte-identical on all 24 cases. The HTML was cut and uncut in the same session: the 16 goldmark misses were not "block-layer list structure" but one list-indent rule — python-markdown nests at 4 spaces, CommonMark at 2 — and normalizing the input makes goldmark match 24/24. |
| 2026-08-07 | **Iteration 9 green.** The fresh-context verifier returned FAIL with two blockers (a null `degree_column` ignored; a photo rendering silently wrong), one major (`splitLines` was not `str.splitlines()`) and one coverage hole (seven unpinned `locale.Resolve` branches). All four fixed, each behind a fixture that is red without its fix. 24/24 `.typ` byte-identical. Two upstream *crashes* the port does not reproduce are recorded for the human gate. |
| 2026-08-07 | **Axis 1's first passing cases.** The bridge (`internal/renderer/bridge`) and the orchestration (`internal/renderer/typstdoc`) landed, and all 21 corpus inputs that carry a `cv.yaml` render a `.typ` byte-identical to the vendored Python's, pinned to `settings.current_date: 2025-03-05` by `tools/typprobe`. All nine entry types are covered; the fixture is mutation-checked (19 of 21 fail on a one-newline change to `Assemble`). |
| 2026-08-08 | **The CLI surface was measured instead of assumed, and six invocations exited 70 with no output.** An unknown command, three missing required arguments and two malformed extra-argument vectors. All fixed; the three usage errors are byte-identical to the vendored CLI at `COLUMNS=80` after the sanctioned name substitution. `TestParity` did not move, because no corpus case exercises any of them. |
| 2026-08-08 | **Seven of `render`'s long flag names were invented.** `--typ`, `--pdf`, `--png`, `--md`, `--html` and the five `--no*` are upstream's *short* forms of `--typst-path`, `--dont-generate-pdf` and the rest; upstream declares no such long options. Five more options were never declared at all, three of them the `--design` / `--locale-catalog` / `--settings` overlay files, which are now read and differentially byte-identical. The corpus names no long form anywhere, which is why every case passed throughout. |
| 2026-08-07 | Iteration 6's T10 closed in iteration 9: `design.Effective` merges the base tree, the theme's overrides and the document's own block, deep at every layer, and runs the two coercions where upstream's validators do. Seven-document differential against upstream's resolved model. |
| 2026-08-10 | **Iteration 14's 5th re-verification — FAIL, 3 blockers.** A partial `typography.font_family` mapping override dropped its four sibling elements to empty (a merge onto an empty map, not onto `FontFamily`'s base defaults) — reachable on any theme, scripted or built-in. A word-form YAML boolean (`show_footer: no`) was pruned as a type conflict against a script-declared `bool`, and even with no script reached the Typst emitter as an unquoted, uncompilable token. `specs/014-lua-custom-themes/spec.md` §2 behavior 9 and §5 still claimed the port surfaces gopher-lua's own error text for a broken script — it doesn't, and hadn't since the claim was written; only `divergences.md` had been corrected. All three fixed (`29cb863`, `a8fb580`) plus a pre-existing lint break from `65ecc49` (`4caa576`) and the spec text (`221e4cd`). |
| 2026-08-10 | **Iteration 14's 6th re-verification — FAIL, 1 blocker.** The 5th pass's word-form-boolean fix made `kindOf` classify any bool-word string as "true or false" symmetrically, so a script-declared **string** option against a document override that happened to spell a bool word (`custom_note: 'On'`, a legal string) was pruned as if it agreed with the script — a regression the 5th pass's own tests could not see. Fixed by gating the exemption on the script's declared value actually being a Go `bool` (`cdb9447`); the font_family fix's missing built-in-theme coverage (major) closed with a new test (`e39cc7f`). The sandboxed-globals coverage gap (17 blocked names, 8 tested) is still open, unaddressed by design — this pass's scope was the 5th pass's fixes. |
| 2026-08-10 | **Iteration 14's 7th re-verification — FAIL, 2 blockers.** `mappingOf` recognized only `"true"`/`"True"`/`"yes"`/`"on"` for a `KindBool` node, but `ResolveScalar` also classifies `"TRUE"`/`"False"`/`"FALSE"` as `KindBool` — `TRUE` fell through to `false`, the wrong boolean rather than an uncoerced one, on a **built-in** theme with no script. And `ValidateScript` flagged a script declaring `typography.font_family` as a mapping (a legal shape, spec 006 §3.1 behavior 12) as a conflict, and `themeScript` discards the **entire script** on any `ValidateScript` error — every other option the script declared was silently lost too. Both fixed (`cb62853`, `49210a1`) and differentially confirmed byte-identical against upstream, including the font_family case that proved the earlier passes' scripted-`font_family`-as-mapping coverage had been vacuous (the script was never applied at all). A third finding — a scripted custom theme skips the design tree's *value* validation entirely, where upstream's `theme_data_model_class(**design)` would refuse a bad value on a known key — is new, broader than the already-cut forbid-extra gap, and cut forward undeclared pending a `divergences.md` entry. |
| 2026-08-10 | **Iteration 14's 8th re-verification — FAIL, 3 blockers.** Pass 7's font_family-as-mapping carve-out only checked the mapping's own shape, not what was inside it — on both the script side (`ValidateScript`) and the document side (`withoutTreeConflictsAt`), `typography.font_family.body` declared or overridden as a *nested table* was waved through unchecked, printing a Go type name into the artifact one field deeper than the carve-out was meant to reach. A second, pre-existing gap: `ValidateScript` compared only mapping-vs-value shapes and never checked lists at all, so a script declaring a Lua sequence for a scalar field (`page.size = {"a4"}`) merged straight through the same way the map case used to. All three fixed by recursing into `FontFamily`'s own fields on both carve-outs and adding a `shapeKind`-based list check symmetric with the existing map one (`facf4dc`, `bfd5990`, `277d85b`); differentially confirmed against upstream and against the binary built at `cb62853`. The already-cut skipped-value-validation gap and the sandboxed-globals coverage gap are unchanged. |
| 2026-08-10 | **Iteration 14's 9th re-verification — FAIL, 2 blockers.** `withoutTreeConflictsAt`'s font_family carve-out — the very code pass 8 just touched — handled map and scalar but let a document-side **list** through unchecked, reaching pongo2 as a slice where a dict-or-string belonged and surfacing a raw template-engine error rather than a panel. Separately, `ValidateScript` flagged an *empty* Lua table for `sections.show_time_spans_in` (the tree's one `KindStringList` field) as a conflict, because Lua cannot distinguish an empty sequence from an empty mapping and `luatheme.Options` converts `{}` to an empty Go map; `themeScript`'s all-or-nothing drop then discarded every other option a script declared alongside it. Both fixed (`4dd2f55`, `96d495a`), each landed with a test this time — the pass also flagged that pass 8's three fixes had shipped with none. |
| 2026-08-10 | **Iteration 14's 10th re-verification — FAIL, 1 blocker.** The empty-table ambiguity pass 9 fixed at `ValidateScript` had a second, independent occurrence: `EffectiveWithScript` still `deepMerge`d the script's raw empty Go map over the tree's `[]string` base default for `sections.show_time_spans_in`, so `withoutTreeConflictsAt` saw a map at that path afterward and dropped a document's real list override as a shape conflict — silently rendering the wrong CV at exit 0, confirmed byte-diverging in `.typ`, `.md` and `.html`. Fixed by normalizing the script's empty map to `[]string{}` before the merge (`0da9351`), landed with a test asserting the document's override survives end-to-end. Three process findings also surfaced, none acted on yet: iteration 14 has only `spec.md`, no `plan.md`/`tasks.md`; all 23 of its commit subjects exceed the 50-char limit; `just check` fails on pre-existing lint debt in `internal/renderer/typstc`, unrelated to this iteration. |
| 2026-08-10 | **Iteration 14's 11th re-verification — FAIL, 1 blocker.** Pass 10's fix confirmed correct by four extra probes. New finding, broader than any prior pass and not custom-theme-specific: `validateModel`'s null-skip and the binder's `isNull && !Required` both let an explicit null through for **every** design field except the one — `templates.education_entry.degree_column` — that upstream actually types nullable, because every design field has a default and so is non-`Required`, which the binder conflated with "admits null." `colors.name: null` on a **built-in** `theme: sb2nov` rendered at exit 0 with the base tree's color instead of sb2nov's own, where upstream exits 1. Measured on five field kinds (color, typst dimension, literal, bool, string list), all silently accepted before the fix. Fixed with an opt-in `binder.Field.TypeRejectsNull` (`7256c82`), zero-risk to the other seven binder callers by construction, landed with 6 new cases. The three pass-10 process findings re-confirmed, still open. |
| 2026-08-10 | **Iteration 14's 12th re-verification — FAIL, 4 blockers.** Pass 11's null fix held under every adjacent probe tried. Four new findings, all pre-existing (not regressions) and all reachable on a built-in theme: `design.theme: null` skipped validation outright instead of stringifying to `"None"`; `theme: true` used the raw lowercase token, which passes the name pattern, instead of Python's `"True"`, which doesn't; a sequence/mapping theme value produced an empty quoted name in the rejection message; and `validColorNode`'s tuple arm checked only length, so `[300, 0, 0]` validated where the string form already rejects it, while a *valid* tuple that passed reached the Typst emitter as a raw Go slice instead of `rgb(1, 2, 3)`. Fixed with `themeNameRepr`/`pythonElemRepr` (`ebfcc30`) and `ParseColorTuple` plus a `normalizeColors` `[]string` arm (`035048e`), both landed with tests. The already-cut "value validation skipped when a script exists" gap got a broader confirmed repro (four field kinds, not just one); the process findings (missing plan.md/tasks.md, commit length, typstc lint) are unchanged. |
| 2026-08-10 | **Iteration 14's 13th re-verification — FAIL, 6 blockers + 1 minor; 5 blockers fixed, 1 blocker + the minor deferred.** Pass 12's fixes held on 15+ adjacent probes. New: `parse_color_value` is `float(value)`, which accepts a bool (`float(True)==1.0`) and any int upstream resolves including hex/octal/binary literals — `strconv.ParseFloat` alone rejects all three tokens; a null 4th (alpha) element is `parse_float_alpha(None)` returning `None`, not an error; a script's own colour tuple was dropped twice over — `ValidateScript`'s shape check had no `KindColor` carve-out, and `luatheme.sequenceOf` silently discarded every non-string element (a Lua sequence of numbers) before the tuple ever reached that check; and a document tuple overriding a scripted theme's colour default was pruned by `withoutTreeConflictsAt`'s generic shape check the same way. All fixed (`5dc89ac`, `20c8e6d`), each landed with tests. Deferred, not fixed: `themeNameRepr` still uses the raw YAML token for a numeric theme name rather than Python's `str()` of the parsed number (`theme: 1_000`/`0x1f` reach the pattern check as their literal token), and `pythonElemRepr`'s mapping arm always quotes a key where Python's `repr` leaves a non-string key bare — both edge cases with no plausible reachable CV, recorded rather than built out into a full numeric-repr subsystem. |
| 2026-08-10 | **Iteration 14's 14th re-verification — FAIL, 4 blockers + 1 major, all fixed.** `parseAlpha`'s non-percent branch had not been routed through pass 13's `parseNumericText`, so a bool/hex/octal/binary **alpha** still failed where the equivalent channel worked. Pass 13's `KindColor` script carve-out was shape-only with no tuple validation, so an invalid script tuple leaked a raw `[]string` into the artifact instead of falling back safely. Both merge-side carve-outs (`withoutConflicts` and `withoutTreeConflictsAt`) only handled one direction of colour-scalar-vs-colour-tuple; the reverse direction silently lost either the document's or the script's colour depending on which side held which shape. The major: `RenderInput` showed a bool's Input Value in its raw lowercase form, where pydantic's column carries `str(True)`/`str(False)` — the same class of gap as the already-deferred numeric-literal case, but reachable on an ordinary `true`/`false`, which the deferral hadn't accounted for. All fixed (`bc635d0`, `df4a82a`, `7c2227d`), each landed with tests. The numeric-literal-form gap (`0x1f`→`31`, not the bool case) and the mapping-key-quoting minor remain deferred, unchanged. |
| 2026-08-10 | **Iteration 14's 15th re-verification — FAIL, 3 blockers + 2 major + 1 minor.** 3 blockers fixed (`44f513d`): `parseChannel`'s and `normalizeAlpha`'s range checks were unchained (`value < 0 \|\| value > max`), false in both directions for NaN, which is the *accept* condition — a NaN channel or alpha reached `int(NaN)` and printed garbage instead of a range error; `parseAlpha`'s percent branch still hadn't been routed through `parseNumericText`, so a whitespace-padded percent alpha failed where the equivalent plain-number alpha (fixed last pass) worked; and `parseNumericText`'s prefix check lowercased its own copy before comparing, accepting `0X1F` where ruamel's lowercase-only resolver would leave it a string upstream. Two findings are **out of iteration 14's subsystem** and recorded rather than fixed: `cv.name`/`cv.email`/`cv.website` don't type-check a non-string scalar at all (renders a full artifact at exit 0 where upstream exits 1) — `binder.go` already has the machinery, these three CV fields just don't reach it, a validation-error-parity gap (iterations 2-4's territory); and a literal tab byte inside a double-quoted YAML scalar is rejected by the goccy reader where YAML permits it, a reader gap (iteration 2's territory). The minor: `df4a82a` (pass 14) bundled three logical units in one commit, noted but not correctable after the fact. Pass 15 confirmed all four of pass 14's fixes hold under 13+ merge-carve-out probes, 11 bool-Input-Value probes, 5 script-tuple-validation probes, plus a Lua sandbox sweep. |

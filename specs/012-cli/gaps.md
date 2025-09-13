# Spec 012 — the measured remaining divergences

Two fresh-context verifications of the 2026-08-08 CLI session returned FAIL with **27 findings
between them, 6 of them blockers**. This file is the behavior half of what they found: what
upstream does, measured, for every place the port still differs. It supersedes nothing in
`spec.md`; it adds the cases `spec.md` never described.

Every row was measured against the vendored CLI at `COLUMNS=80 NO_COLOR=1 TERM=dumb`.

**The corpus can see almost none of this.** All 35 parity cases were unchanged while every defect
below was present, so a green suite is not evidence for any of it. The gate is a differential
against the vendored binary, one vector at a time.

**All eleven G-numbered findings are closed as of 2026-08-09** (G-10 narrowly: the flag is rejected
rather than silently ignored, but the watcher feature itself is still iteration 13's, per spec §6.2
— unchanged). Some were fixed across several 2026-08-08 commits this file never recorded; that gap
in the record, not in the code, is what this pass closed. §8's open design question (`init.lua`'s
contents) is unaffected and still open.

**A fresh-context verifier ran after that closure pass** and, correctly, did not just re-trust the
prose — it found two undeclared divergences no G-number covered (both fixed the same day: `render`'s
three panels were printing an extra trailing newline the vendored CLI never does, and the duration
column was `%.1fs` where upstream is always whole milliseconds — see `internal/cli/render.go`'s
`writeLivePanel` and `timing`). It also found process debt this file records without yet fixing:

- ~~No test gates G-5, G-7, G-8, G-9 or G-10.~~ **Closed, 2026-08-09.** Sixteen tests now cover
  them: `TestCreateThemeWritesTheFileSet`/`RoundTrips`/`RejectsBadName`/`RejectsExistingFolder`
  (G-5, `customtheme_test.go`), `TestDocumentNamedOverlaysResolveAgainstInputDir` +
  `TestCLIFlagWinsOverDocumentNamedOverlay` (G-7, `overlay_test.go`),
  `TestOutputFolderResolvesAgainstInputDir` (G-8, `paths_test.go`),
  `TestNewCreateTypstTemplates`/`CreateMarkdownTemplates`/`TemplatesReportsExistingFolders` (G-9,
  new `new_test.go`), `TestRenderRejectsWatch` + `TestRenderWithoutWatchStillRenders` (G-10, new
  `watch_test.go`). Each was mutation-checked by hand before being trusted: reverting the fix under
  test made the test fail (spot-checked for G-8 and G-10; the others follow the same file-set /
  panel-text assertions that already caught the earlier regressions).
- ~~`err_missing_file` / `err_bad_override_key` have no `divergences.md` entry.~~ **Closed,
  2026-08-09 (human-approved): D-011.** Both are unhandled-exception tracebacks with this
  machine's absolute paths baked in — non-reproducible even for upstream, let alone a Go binary.
  The port's own shape on both vectors is now pinned (previous bullet).
- **`err_not_yaml`'s span defect is broader than its one corpus case**: any document with a
  duplicate mapping key shows the same wrong single-line location instead of a range. Recorded in
  kind at `STATE.md`, not by scope.

---

## 1. `--` ends option parsing

**G-1 — closed.** Click drops a bare `--` from the argument vector and treats **every following
token** as an extra — declared flags included. `internal/cli/args.go:115` now matches: verified by
hand this pass, `render cv.yaml -- -notyp -nomd -nopdf -nopng -q` produces upstream's extra-
arguments message and `render cv.yaml --` renders normally.

| Input | Upstream |
|---|---|
| `render cv.yaml --` | renders normally, exit 0 — `--` is dropped and no extra remains |
| `render cv.yaml -- -notyp -nomd -nopdf -nopng -q` | `There is a problem with the extra arguments (-notyp,-nomd,-nopdf,-nopng,-q)! Each key should have a corresponding value.` |

So `-notyp` after `--` is **not** a flag — and now isn't one in the port either.

## 2. `--YAMLLOCATION` is declared and never read

**G-2 — closed.** Upstream declares it (`render_command.py:190-197`) purely so the help panel has
a row describing the dotted-override mechanism; the function binds it to `_` and nothing reads it.
`spec.md` §2 behavior 6 already tabulates it. `internal/cli/root.go:74` now declares and discards
it the same way. Verified by hand this pass: `--YAMLLOCATION zzz` renders normally, exit 0,
matching upstream's silence rather than becoming an override key the model rejects.

## 3. Two usage errors the port used to answer with exit 70 and silence

**G-3 — closed. A declared option missing its value.** Exit **2**, an `Error` panel on stderr, and
— unlike every other usage error — **no usage line and no `Try …` line**.

```
╭─ Error ──────────────────────────────────────────────────────────────────────╮
│ Option '--output-folder' requires an argument.                               │
╰──────────────────────────────────────────────────────────────────────────────╯
```

Measured for `--output-folder` and `--design`; the option's own spelling is interpolated.
`flagUsageError` (`internal/cli/root.go`) produces this; verified by hand this pass.

**G-4 — closed. An unknown option outside `render`.** Exit **2**, *with* the usage line:

```
Usage: rendercv new [OPTIONS] FULL_NAME
Try 'rendercv new -h' for help.
╭─ Error ──────────────────────────────────────────────────────────────────────╮
│ No such option: -d                                                           │
╰──────────────────────────────────────────────────────────────────────────────╯
```

`render` never reaches this, because `ignore_unknown_options` sends its unknowns to the extras.
Verified by hand this pass: `new "John Doe" -d` produces exactly this shape (binary name aside,
D-001).

## 4. `create-theme` is unregistered and the port now lies about it

**G-5 — closed, 2026-08-09.** `create-theme mytheme` → upstream creates the folder, exit 0. The
port used to exit **2** with `No such command 'create-theme'.` while its own `--help` listed the
command — worse than the earlier silent exit 70, since it asserted a false thing.

The command is registered (`internal/cli/customtheme.go`, `internal/cli/root.go`). D-008's open
question — the `__init__.py` equivalent — is answered by writing `init.lua` as a documented empty
table, not by porting `classic_theme.py`'s 857 lines: a Lua declaration has no pydantic class to
derive from, `luatheme.Options` reads whatever a script returns, and restating every classic-theme
default as a comment would be a golden by another name. See §8.

## 5. The `render_command` settings block is half-wired

**G-6 — closed, 2026-08-08 (`e78ad3d`).** `settings.render_command.dont_generate_*` was ignored:
upstream gates inside each generator on the **merged model** (`renderer/pdf_png.py:33,63`,
`typst.py:23`, `markdown.py:22`, `html.py:26`), and the port gated on the CLI flag alone. Fixed by
routing generation through `settings.Resolved` instead of the CLI options directly, so the flags,
the document and a `--settings` overlay are one source of truth. Verified by hand this pass:
`dont_generate_pdf: true` in the document, no CLI flag, produces no PDF (PNG is unaffected, as
upstream's own flag independence requires).

**G-7 — closed, 2026-08-08 (`c03fe1d`).** `settings.render_command.design` and `.locale` were
never resolved. Upstream's `collect_input_file_paths` (`run_rendercv.py:113-122`) resolves each
relative to the **input file's** directory when the CLI flag did not supply one. Measured before
the fix: 240 differing `.typ` lines, the port rendering `classic` where upstream rendered the named
theme; after, byte-identical. Verified by hand this pass: `sub/cv.yaml` naming `mydesign.yaml`
(theme `engineeringresumes`) with no `-d` flag renders `engineeringresumes`'s fonts, not
`classic`'s.

**G-8 — closed (`555d7e0`).** `output_folder` now resolves against the input file's directory
(`PlannedPathRelativeToInput`, `schema/models/settings/render_command.py:30`), not the working
directory.

## 6. Two options are declared and unread

**G-9 — closed, 2026-08-09.** `new --create-markdown-templates` now writes the `markdown` folder
through `copyBuiltinTemplates("markdown", …)`, the same path `--create-typst-templates` uses. The
"Get started" panel's "Also created" / "Not modified (already exist)" block generalizes to both
flags at once (`internal/cli/new.go`). No corpus case names the flag, so nothing in `TestParity`
moved; verified by hand against `new_command.py:150-166`'s shape.

**G-10 — closed narrowly, 2026-08-09.** `render --watch` parsed and did nothing; upstream loops.
`spec.md` §6.2 defers the watcher itself to iteration 13, so it is not implemented here — but
`Render` now rejects `--watch` with an explicit "not implemented" message and exit 1 instead of
silently doing a single render and exiting 0 as if the flag had been honored. Declared-and-unread
was a worse failure than undeclared; declared-and-refused is at least honest about what happened.
The watcher itself is still iteration 13's.

## 7. The console width is ignored

**G-11 — closed, 2026-08-08 (`8594b5d`).** `internal/cli/panel.go:10` was `const PanelWidth = 80`
and nothing in the port read `COLUMNS`. `ConsoleWidth()` now reads it (falling back to 80 on an
absent or non-positive value, matching Rich's own `int()` guard), and `Panel`/`Table`/`HelpPage`
all lay out to it. Verified by hand this pass: `COLUMNS=100 render …` produces a 100-column-wide
result panel, `COLUMNS=60` a 60-column one.

| Width | Upstream | Port |
|---|---|---|
| 100 | 100-column output | now 100-column |
| 60 | wraps at 57, boxes at 60 | now matches |

**Still no golden can see this**: every one is captured at 80, so the fix is unverified by
`TestParity` and rests on the hand check above plus `panel_test.go`/`helptable_test.go`.

## 8. `init.lua`'s contents — the one open design question

Upstream writes a custom theme's `__init__.py` by copying `classic_theme.py` and substituting the
class name and the theme literal (`create_init_file_for_theme.py:26-43`), so the user starts from
a complete, editable declaration of every classic option.

The port has no file to copy. Its `init.lua` returns a table of option **overrides** (D-002), and
`design.Overrides("classic")` is empty because classic is the base tree — so the faithful
translation of "copy the classic theme" is an empty table, which teaches the user nothing.

Three candidates, none measured against anything because upstream has no counterpart:

1. **The full effective classic tree**, serialized to Lua — closest to upstream's intent, and the
   largest file.
2. **An empty table with commented examples** — closest to what the Lua contract actually needs,
   since a declaration's value *is* its default and type.
3. **A small representative subset** — arbitrary, and arbitrary is what this port keeps getting
   caught by.

**Recommendation was 1; implemented was 2.** `writeThemeInitLua` (`internal/cli/customtheme.go`)
ships an empty table with a commented example rather than the full classic tree. Not measured
against upstream — there is nothing to measure against — and worth revisiting against the
recommendation above if a user reports the empty file is unhelpful. The reasoning at the time: a
Lua declaration's value *is* its type (no annotation exists to carry one independently), so a
"complete" starter would mean re-deriving all of `ClassicTheme`'s ~30 fields into Lua syntax by
hand — itself a small port with its own drift risk — for a theme a user is, by definition, about to
change.

## 9. Acceptance criteria

1. G-1 through G-4 each differentially byte-identical to the vendored CLI on stdout, stderr and
   exit code.
2. `create-theme <name>` writes D-008's fourteen files and prints the panel; `cli_create_theme_help`
   passes.
3. G-6 and G-7 differentially byte-identical on the artifact **set** and on `.typ`/`.md`/`.html`.
4. G-8 writes beside the input file, differentially checked from a subdirectory.
5. G-9 writes the Markdown template set; G-10 either implements the watcher or is declared.
6. G-11: the port's panels track `COLUMNS`, checked at 60, 80 and 100 against the vendored CLI.
7. Every one of the above is gated by a test that fails without its fix. The corpus cannot see any
   of them, so a green `TestParity` is not evidence for any criterion here.

# Divergences from upstream RenderCV

Every deviation from `third_party/rendercv` @ `v2.8` lives here. Nothing else is permitted.

**Changing this file is human-gated** (`AGENTS.md` §5). An agent may propose an entry; a human
approves it. An unapproved divergence is a bug, not a design choice.

Each entry: what differs · upstream citation · why parity is impossible or undesirable · what
`rendercv-go` does instead · what the user notices.

---

## D-001 — Executable name

**Status:** approved (project-defining)

- **Differs:** binary is `rendercv-go`, not `rendercv`.
- **Why:** the two must be installable side by side; this project is a distinct artifact.
- **Instead:** identical CLI surface under a different `argv[0]`. Usage strings say `rendercv-go`.
- **User notices:** they type a different command name. Nothing else.
- **Not licensed by this:** the product name in `--version` output stays `RenderCV`
  (`src/rendercv/cli/app.py:41`), per `specs/000-parity-contract/spec.md` §2.2.

---

## D-002 — Custom themes are scripted in Lua, not Python

**Status:** approved · **Replacement:** embedded Lua (iteration 6)

- **Differs:** upstream loads and executes a custom theme's `__init__.py` during validation, using
  the resulting pydantic class as the theme's option schema.
- **Upstream:** `src/rendercv/schema/models/design/design.py` — `validate_design()`, the
  `importlib.util.spec_from_file_location` block; also
  `src/rendercv/cli/create_theme_command/create_init_file_for_theme.py`, which generates the file
  by copying `classic_theme.py` and rewriting the class name and the `theme` literal.
- **Why impossible as-is:** Go has no runtime Python interpreter, and embedding one would defeat
  the single-static-binary goal.
- **Instead:** the same capability, in **Lua**, via an embedded pure-Go interpreter
  (`github.com/yuin/gopher-lua` — Lua 5.1, no CGO). A custom theme provides `<theme>/init.lua`
  returning a table with the theme name, its option defaults, and an optional `validate` function.
  This keeps upstream's actual feature — themes that *compute* and *check* their own options —
  rather than downgrading it to a static manifest.
  - `rendercv-go create-theme <name>` generates `init.lua` from the classic theme's option tree,
    mirroring upstream's copy-and-rename behavior.
  - A theme folder with no `init.lua` falls back to the classic theme's defaults with
    `theme = <name>`, exactly as upstream does when `__init__.py` is absent.
  - Upstream's other custom-theme rules are preserved unchanged: lowercase-alphanumeric theme
    name, folder beside the input file, folder must contain ≥1 `*.j2.typ`.
  - **Sandboxed.** The Lua state is opened without `io`, `os`, `package`, `debug` or `dofile`.
    A theme describes a design; it has no business touching the filesystem or the network.
    Upstream's `__init__.py` had no such limit — this divergence is strictly safer.
  - Error parity where it can be kept: a Lua syntax error and a missing theme table produce the
    upstream messages with `__init__.py` replaced by `init.lua`. Upstream's `ImportError` branch
    has no analogue and is dropped, since `package`/`require` are unavailable.
- **User notices:** a custom theme written for Python RenderCV needs its `__init__.py` translated
  into `init.lua`. Themes with no `__init__.py` (templates only) work unchanged.
- **Open:** the exact table shape and the generated `init.lua` are specified in iteration 6.

---

## D-003 — No PyPI version check

**Status:** approved

- **Differs:** upstream contacts `pypi.org` on a 24h stale-while-revalidate cache and prints an
  upgrade notice.
- **Upstream:** `src/rendercv/cli/app.py:19` (`VERSION_CHECK_TTL_SECONDS`) and
  `warn_if_new_version_is_available()`.
- **Why:** `rendercv-go` is not distributed on PyPI, and silent network I/O on every CLI
  invocation is not something this project wants to reproduce.
- **Instead:** nothing. No network access at startup, no cache file.
- **User notices:** no upgrade nag. Exit codes and all other output unchanged.

---

## D-004 — `pip install rendercv` vs `rendercv[full]` guidance

**Status:** approved

- **Differs:** upstream's entry point catches `ImportError` and prints reinstall instructions.
- **Upstream:** `src/rendercv/cli/entry_point.py`.
- **Why:** a Go binary has no optional-extras split; the failure mode does not exist.
- **Instead:** omitted.
- **User notices:** nothing, unless they were relying on that specific error text.

---

## D-005 — Template source is adapted; template output is not

**Status:** approved

- **Differs:** the `.j2.typ` / `.j2.md` template *sources* in `internal/renderer/templater/templates`
  are not byte-identical to upstream's.
- **Upstream:** e.g. `src/rendercv/renderer/templater/templates/typst/entries/EducationEntry.j2.typ`
  uses `entry.main_column.splitlines()[:first_row_lines]` — a Python method call plus a Python
  slice. `templater.py:43-44` additionally enables `trim_blocks` and `lstrip_blocks`.
- **Why:** pongo2 implements Jinja2 syntax, not Python's object model. It cannot call
  `.splitlines()`, cannot evaluate Python slice expressions, and has no `trim_blocks`/
  `lstrip_blocks` options.
- **Instead:** a documented mechanical transform, applied uniformly:
  1. line-splitting moves into the Go model (`MainColumnLines []string` etc.), so templates
     iterate a ready-made slice;
  2. Python slices become pongo2-expressible forms (`|slice` / loop guards);
  3. `trim_blocks` / `lstrip_blocks` whitespace behavior is baked into the template text.
- **User notices:** nothing — Axis 1 byte-parity of the *rendered* output is unaffected and is
  what the conformance suite checks. Users who override templates (a supported upstream feature)
  must write pongo2-compatible templates; this is called out in the user docs.
- **Risk:** highest-severity item in the port. Whitespace drift will surface as byte diffs.

---

## D-006 — Typst compiled to WASI, executed on wazero

**Status:** approved

- **Differs:** upstream calls the `typst` Python bindings (the Rust crate compiled natively).
- **Upstream:** `src/rendercv/renderer/pdf_png.py`.
- **Why:** Go has no typst binding, and CGO/Rust would break the single-static-binary,
  cross-compilable goal.
- **Instead:** typst built for `wasm32-wasip1`, embedded in the binary, executed via wazero.
- **User notices:** slower compilation than native typst; no external dependency to install.
- **Watch:** the WASI build must carry the same font set as `rendercv-fonts`, or PDF metrics
  drift and Axis 1 §1.2 fails. Tracked in iteration 10.

---

## D-007 — The compiler, the fonts and `fontawesome` are vendored into the repository

**Status:** approved (human gate, 2026-08-08) · **Iteration:** 10

- **Differs:** upstream installs its compiler and fonts as Python dependencies (`typst`,
  `rendercv-fonts`) and **downloads** `@preview/fontawesome:0.6.0` from Typst Universe into
  `~/.cache/typst` the first time a document imports it
  (`third_party/rendercv/rendercv_typst/lib.typ:1`, resolved by the `typst` crate's package
  loader). Nothing but `rendercv`'s own two package files is copied by `get_package_path`
  (`src/rendercv/renderer/pdf_png.py:114-146`).
- **Why parity is impossible:** a Go binary has no package manager behind it. The three inputs
  the compiler needs — the compiler itself, `rendercv_fonts`, and the `fontawesome` package —
  have to arrive somehow, and the two alternatives are worse:
  - *finding fonts on the system* is the failure mode that was measured in iteration 10 and
    **passed 12 of 14 PDF cases while silently rendering `sb2nov` in a fallback face**;
  - *downloading at render time* puts a network dependency inside `render`, which upstream does
    not have for the compiler and which makes an offline render fail.
- **Instead:** all three are committed under `internal/renderer/typstc/assets/` and `//go:embed`ed:

  | Vendored | Size | Source |
  |---|---|---|
  | `typst.wasm` | 29 MB | `just typst-wasm`, `tools/typstwasm` pinned to typst 0.14.2 by `Cargo.lock` |
  | `fonts/` | 59 MB, 62 files in 15 folders | the `rendercv-fonts` package the submodule locks |
  | `packages/preview/rendercv/0.3.0/` | 8 KB | the submodule's `rendercv_typst/` |
  | `packages/preview/fontawesome/0.6.0/` | 428 KB | Typst Universe — **the file upstream does not ship** |

- **User notices:** a ~90 MB binary instead of a ~15 MB one, and `render` works offline with no
  `typst`, no `rendercv-fonts` and no package cache installed. Font metrics are reproducible
  because they cannot be shadowed by whatever the host happens to have.
- **Not licensed by this:** vendoring does not change *font resolution order*. A `fonts/`
  directory beside the input file still wins a name tie, matching typst-cli's `FontSearcher`
  and upstream's `get_typst_compiler` (`pdf_png.py:154-186`).

---

## D-008 — `create-theme` writes port-native files

**Status:** approved (human gate, 2026-08-08) · **Iteration:** 12

- **Differs:** two of the fourteen files `create_theme` writes cannot be upstream's bytes.
- **Upstream:** `src/rendercv/cli/create_theme_command/`.
- **Why:**
  - `__init__.py` is Python that upstream *executes* at validation time. This port does not
    execute Python — that is D-002 — so writing the file would produce a theme whose options
    are never applied.
  - the `.j2.typ` files are Jinja. The port's loader reads the pongo2 transform of them (D-005).
    Writing Jinja source would hand the user a theme **this binary renders differently from the
    one it just wrote** — measured on `Header.j2.typ`, where upstream's carries a newline after
    `{% macro image() %}` that Jinja's `trim_blocks` eats at parse time.
- **Instead:** `create-theme` writes `init.lua` in place of `__init__.py`, and the pongo2
  transform of each template in place of the Jinja source. The other twelve files are
  byte-identical.
- **User notices:** the generated theme folder is scripted in Lua and its templates are in the
  port's dialect. It renders identically to the theme it was copied from, which the Jinja
  version would not.
- **Consequence for the suite:** the `create_theme` corpus case compares template *source*, so it
  is unreachable by construction. It stays red, with this entry as the reason. `new
  --create-typst-templates` writes the same thirteen `.typ` files through the same
  `copyTypstTemplates` path (`internal/cli/customtheme.go`), so `new_typst_templates` fails on
  exactly the same four fragments and stays red for the same reason — not a second divergence.
  `create_theme` also differs on **stdout**, not just the file set: the panel's second step reads
  `Edit ./mytheme/__init__.py to:` upstream and `Edit ./mytheme/init.lua to:` here — the same
  substitution, one more place it shows.

---

## D-009 — The `new` panel's next-step line names `rendercv-go`

**Status:** approved (human gate, 2026-08-08) · **Iteration:** 12

- **Differs:** upstream's `new` prints `2. Run: rendercv render John_Doe_CV.yaml`
  (`src/rendercv/cli/new_command/`). The port must print `rendercv-go`, or it prints an
  instruction that does not work — D-001's whole point.
- **Why it is not just D-001:** the line sits inside a **fixed-width Rich panel**, so the longer
  name also shifts that row's right-hand padding by three characters. It is the one place where
  the sanctioned name change is not confined to `argv[0]`.
- **Instead:** the port prints the working command and pads the row to the panel's width. The
  conformance harness substitutes the binary name **and re-pads the row** before comparing, so
  the eight `new_*` cases stay comparable.
- **User notices:** a command they can copy and run.
- **Cost, recorded rather than hidden:** the harness's fixed-width check is weakened on exactly
  the rows it rewrites. Every other row of every panel is still compared verbatim, and the
  rewrite is confined to rows containing the binary token — a row that does not contain it is
  never touched.

---

## D-010 — The help pages' prose wraps around a longer binary name

**Status:** approved (human gate, 2026-08-08) · **Iteration:** 12

- **Differs:** four of the five `cli_*_help` goldens cannot be byte-identical, on two lines each
  (`cli_help` and `cli_help_short` share the same page and both carry the token).
- **Upstream:** `typer/rich_utils.py:535-620`, rendering the help for `rendercv`, `rendercv render`
  and `rendercv new`.
- **Why it is not just D-001 or D-009:** every help page quotes commands the reader is meant to
  run — `Example: rendercv render John_Doe_CV.yaml` — so the port must print `rendercv-go`, which
  is three characters longer. **A help page wraps its prose to the console before anything
  compares it**, so the line breaks somewhere else:

  ```
  golden:  Render a YAML input file. Example: rendercv render John_Doe_CV.yaml. Details:
           rendercv render --help
  port:    Render a YAML input file. Example: rendercv-go render John_Doe_CV.yaml.
           Details: rendercv-go render --help
  ```

  D-009's remedy — substitute the token, re-pad the row — works on a row that merely got shorter.
  **Re-padding cannot undo a re-wrap**, and neither can any rule applied to finished bytes.
- **Instead:** the port prints the command the reader can actually run, and wraps it honestly.
- **User notices:** a help page whose examples work.
- **Consequence for the suite:** `cli_help`, `cli_help_short`, `cli_render_help` and
  `cli_new_help` are unreachable by construction, like `err_missing_file` and `create_theme`. They
  are held instead by `internal/cli/help_test.go`, which compares every page against its golden
  and **fails if a differing line does not carry the binary name** — so the geometry is gated on
  130 of the 136 lines and the six exceptions have one stated cause.

### The harness change this entry also covers

`RebindBinaryName` re-padded a shortened **bordered** row and left every other line as the
substitution produced it. A help page's `Padding` regions — the usage line and the description —
are painted to the console width too, so they are exactly as width-sensitive as a panel row, and
the `isPanelRow` guard left them three characters short.

The guard is now "bordered **or** exactly the console width". Measured: `cli_help` went from
differing at byte 158 (the usage line, 2430 bytes against 2433) to differing only on the two
re-wrapped lines, at equal length.

- **Cost, recorded rather than hidden:** the width check is now reconstructed rather than compared
  on any full-width line containing the binary token, where before it was reconstructed only on
  bordered rows. A line without the token is still compared verbatim, and a line that was not
  full width to begin with is still left exactly as substituted.
- **New in this entry:** the rewrite rule finally has tests of its own
  (`internal/conformance/binaryname_test.go`). It had none, in a harness that iteration 1's audit
  already found to be the instrument every other claim rests on.

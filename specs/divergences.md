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
  returning a table with the theme name and its option defaults. **Not yet accurate**: this
  paragraph used to also claim "and an optional `validate` function" and that this "keeps upstream's
  actual feature — themes that *compute* and *check* their own options" — nothing in
  `internal/schema/luatheme` looks for a `validate` key or calls back into a script after loading
  it, so a script is a **static** declaration only, closer to the "downgrading it to a static
  manifest" this paragraph said it avoided. `luatheme.Validate` (the Go function of that name) types
  a *document's* value against what the script declared; it is not the script computing or checking
  anything itself. Found by a fresh-context verifier, twice (`specs/STATE.md`, iteration 14's third
  and fourth re-verifications); open.
  - `rendercv-go create-theme <name>` generates `init.lua` as an empty `return {}` behind a short
    comment block (`internal/cli/customtheme.go:117-132`), **not** "from the classic theme's option
    tree" as this line used to claim — upstream's generated `__init__.py` is 857 lines transcribing
    `ClassicTheme`'s full pydantic model; the port's `init.lua` declares nothing at all. Found by
    the same verifier pass; open.
  - A theme folder with no `init.lua` falls back to the classic theme's defaults with
    `theme = <name>`, and now **exactly as upstream**: upstream's fallback
    (`ThemeOptionsAreNotProvided(theme=theme_name)`, `design.py:139-142`) carries only `theme`, so
    the document's own `design` block is discarded entirely, and `design.EffectiveWithScript` does
    the same for any script-less non-built-in theme. **Still open**: a document value that
    conflicts with a *base-tree-typed* field (`page.size: {a: 1}`) on a theme whose script exists
    but never mentions that field — `create-theme`'s own generated `init.lua` is an empty
    `return {}` — is dropped rather than merged (leak prevention, `effective.go`'s
    `withoutTreeConflicts`), but a document setting an **unknown** key on a scripted custom theme
    is still silently accepted where upstream's `theme_data_model_class(**design)` rejects it at
    exit 1 (forbid-extra). That needs the script loaded during *validation*, not only at render
    time, and is unfixed. Found by a fresh-context verifier 2026-08-10 (`specs/STATE.md`, iteration
    14's second re-verification).
  - Upstream's other custom-theme rules are preserved unchanged: lowercase-alphanumeric theme
    name, folder beside the input file, folder must contain ≥1 `*.j2.typ`.
  - **Sandboxed.** The Lua state is opened without `io`, `os`, `package`, `debug` or `dofile`.
    A theme describes a design; it has no business touching the filesystem or the network.
    Upstream's `__init__.py` had no such limit — this divergence is strictly safer.
  - Error parity where it can be kept: **not landed yet.** The intent is a Lua syntax error and a
    missing theme table producing the upstream messages with `__init__.py` replaced by `init.lua`;
    the actual behavior today (`internal/renderer/bridge/model.go:84-86`, `:99-101`) is that every
    script failure — a parse error, a non-table return, a shape conflict — is swallowed and the
    theme silently falls back as though no script existed at all, at exit 0 with no message.
    Upstream exits 1 and names the theme. Found by the same 2026-08-10 verifier pass; open.
    Upstream's `ImportError` branch has no analogue and would stay dropped even once the rest is
    fixed, since `package`/`require` are unavailable.
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

---

## D-011 — `err_missing_file` and `err_bad_override_key` are Python tracebacks

**Status:** approved (human gate, 2026-08-09) · **Iteration:** 12

- **Differs:** both goldens are Rich-rendered Python tracebacks (5,287 B for `err_missing_file`,
  13,732 B for `err_bad_override_key`): box-drawn frames, source snippets, and absolute filesystem
  paths baked in twice over — once for every stack frame's file location
  (`/home/nnc/Projects/rendercv-go/third_party/rendercv/src/rendercv/...`, this machine's
  checkout) and once in the final exception message (a `testdata/.work/run/...` path from the run
  that generated the golden). `rendercv-go` writes neither of these documents.
- **Upstream:** in both cases the failure is an **unhandled exception**, not a `RenderCVUserError`
  — the one path `error_handler.py:38-49`'s `handle_user_errors` decorator catches and prints as a
  clean panel. Typer's default `sys.excepthook` (via `rich.traceback`) takes over instead:
  - `err_missing_file` (`render does_not_exist.yaml --settings.current_date …`): a
    `FileNotFoundError` from `pathlib.Path.read_text` inside `collect_input_file_paths`
    (`render_command.py:205`) — the CLI resolves overlay files (design/locale named by a
    `--settings` overlay, if any) before it opens or checks the *input* file, so a missing input
    combined with certain flag shapes reaches an unguarded read rather than the `err_missing_file`
    the corpus name implies exists as a clean message.
  - `err_bad_override_key` (`render cv.yaml --cv.no_such_field x`, the corpus vector): a
    `KeyError` from `pydantic_error_handling.py:216`'s `get_inner_yaml_object_from_its_key`,
    reached while building a validation error's YAML-span coordinates for a key the document does
    not have.
- **Why parity is impossible:** a Go binary has no CPython call stack, no `pathlib` frames, and no
  way to print source lines from `third_party/rendercv`'s `.py` files — reproducing the traceback
  would mean shipping a Python interpreter and this repository's own source tree inside
  `rendercv-go`, which defeats the port. The absolute paths make the goldens **non-reproducible
  even for upstream**: regenerating them on a different checkout or machine produces different
  bytes, which is not true of any other golden in the corpus.
- **Instead:** `rendercv-go` reports both as ordinary validation/user errors — the same `Error`
  panel shape every other `err_*` case gets, on stdout, exit 1 — rather than attempting to
  fabricate a traceback. This is arguably a *user-facing improvement* over upstream (a clean
  message instead of a stack trace an end user cannot act on), but it is still a divergence: the
  bytes do not match, by construction, forever.
- **User notices:** a real error message instead of a Python stack trace with someone else's
  filesystem paths in it.
- **Consequence for the suite:** `err_missing_file` and `err_bad_override_key` are unreachable by
  construction, like `create_theme` and the four `cli_*_help` cases under D-010. They stay red in
  `TestParity` with this entry as the reason — `TestParity` still runs both invocations every
  time the suite runs, so a crash or a wrong stream/exit code would be caught even though the byte
  comparison never passes; only the exact panel *shape* for these two vectors has its own unit
  test (`TestRenderReportsAMissingInputFile`, `TestRenderReportsAnUnknownOverrideKey`, both in
  `internal/cli/customtheme_test.go`).

---

## D-012 — Six explicit YAML tags do not resolve the way ruamel resolves them

**Status:** approved · **Iteration:** 15

- **Differs:** six tag spellings out of the twenty-four measured in
  [`specs/015-yaml-tags/spec.md`](015-yaml-tags/spec.md) §3.2 do not reach upstream's answer. The
  other eighteen do, byte for byte.
- **Upstream:** `ruamel/yaml/constructor.py:1598-1640` (`construct_unknown`), `:1181-1184`
  (`construct_yaml_str` deferring to it whenever a node carries a tag handle), `:432-445` (the
  YAML 1.1 bool table), `:1724` (`add_constructor(None, …)`);
  `src/rendercv/schema/yaml_reader.py:53`, `:83-86`. `ruamel/…` paths are relative to
  `third_party/rendercv/.venv/lib/python3.12/site-packages/`, the dependency the submodule pins.

Three groups, three different reasons.

### 1. Three Python types the port's `Kind` set has no member for

| Document | Upstream | `rendercv-go` |
|---|---|---|
| `cv.name: !!binary aGk=` | `bytes`, which pydantic coerces to `str` — the CV renders as `hi` | opaque, exit 1 |
| `!!set {x}` | `CommentedSet`; the Input Value column reads `set(odict_keys(['x']))` | a sequence-shaped message, exit 1 both, different bytes |
| `!!omap [{x: 1}]` | `CommentedOrderedMap`, with its own construction error | a different YAML-error phrasing, exit 1 both |

**Why not:** each needs a new node kind carrying a Python container's own `str()` spelling, for a
tag no CV has a reason to use. `!!binary` is the only one of the three where upstream *renders* and
the port does not.

### 2. Three shapes goccy's parser refuses outright

| Document | Upstream | `rendercv-go` |
|---|---|---|
| `a: !!str [1, 2]` — any *known scalar* tag over a collection | transparent; a plain sequence | `unexpected scalar value type`, a YAML-error table |
| `a: !!str` with a sibling key on the next line | `TaggedScalar('')` | `unexpected scalar value`, a YAML-error table |
| `a: !!merge x` | an ordinary `TaggedScalar` | `could not find merge key` |

**Why not:** the fault is in the parser, before any node exists to reinterpret. Fixing it means the
retry-and-reparse technique `parseTolerantOfQuotedTabs` uses
(`internal/schema/yamlreader/build.go:151`), and that technique is only safe when the substitution
provably cannot change a value — true of a tab folded away inside a quoted scalar, not obviously
true of deleting a tag token. A bare `a: !!str` **alone on its line does** work; only the
sibling-key form fails.

### 3. Four constructor crashes, which are D-011's class

`!!int bogus`, `!!float bogus`, `!!bool bogus` and a valueless `!!int` raise
`ValueError`/`KeyError`/`IndexError` inside ruamel — not `MarkedYAMLError`, so they are not caught
with the scanner and parser errors. Upstream prints a **rich traceback on stderr, nothing on
stdout, exit 1**.

**Why not:** reproducing a Python traceback is D-011's open question, not this iteration's. The
port keeps the forced kind and reports an ordinary validation record at exit 1, which is
deliberately the closest available behavior — before iteration 15 these documents *rendered* at
exit 0.

### What the user notices

Nothing, unless they write an explicit tag — which no example, no template and no generated file in
either project does. Where the port diverges it is stricter in five of the six cases and laxer in
one (`!!binary`).

### Four upstream internal errors found while measuring this, **not caused by tags**

Recorded here because this is where they were measured; each is D-011's class, not this entry's.

| Document | Upstream | `rendercv-go` |
|---|---|---|
| `cv: {1: x}`, and any other non-string key | `RenderCVInternalError: Key '1' not found in the YAML file.` | a validation record |
| `cv: {true: x}` | same | a validation record |
| `cv: {!!int 1: x}` | same | a validation record |
| `cv.website: []` | `RenderCVInternalError: website key present but value is None` | renders at exit 0 |

The last is the only case in this entry where the port renders and upstream does not.

---

## D-013 — A broken theme script's reason string is Lua's, not Python's

**Status:** approved (human gate, 2026-08-11) · **Iteration:** 14

- **Differs:** the *text* of the message shown when a custom theme's script fails to load, for
  three of the port's four failure modes. Nothing else about the failure differs.
- **Upstream:** `src/rendercv/schema/models/design/design.py:89-133` — `validate_design`'s
  `exec_module` block raises a `PydanticCustomError` for a `SyntaxError` and for an `ImportError`,
  a `ValueError` for a missing `{Theme}Theme` class, and lets pydantic's own message through when
  `theme_data_model_class(**design)` rejects a declared default.
- **What matches** — measured against the vendored binary, not asserted: the exit code (1), the
  refusal to render (no `rendercv_output/` is created), the `There are validation errors!` panel
  rather than the one-message `Error` box, the `design` location column, the Input Value column
  (`...`, or the offending value for an illegal declared default — upstream prints `bogus`, `True`,
  `10`, `1.5`), and the stream and trailing byte (stdout, empty stderr, last byte the panel's `╯`,
  1411 B for the syntax vector at `COLUMNS=80`).
- **What does not:** the Explanation column, for three modes.
- **Why it cannot be closed:** the two sides' failure modes do not correspond.
  - upstream `SyntaxError` ↔ port **parse or run failure**. Same mode, different text: Python's
    message is fixed and detail-free ("… has a syntax error. Please fix it."), gopher-lua's names
    where the parser stopped (`<string> at EOF:   syntax error`). Discarding that to imitate a
    sentence about a file the port does not read would be worse, not closer.
  - upstream missing `{Theme}Theme` class ↔ port **non-table return**. Analogous, not the same
    thing: a Lua declaration is a table, so there is no class to be missing and no class name to
    put in the message.
  - upstream `ImportError` ↔ **nothing**. D-002's sandbox removes `package` and `require`, so a
    script has nothing to import and this mode is unreachable here.
  - port **shape conflict with the design tree** ↔ **nothing**. Upstream's pydantic annotations do
    this typing when the class is defined; the port has to check it after the fact, so it is a mode
    upstream cannot produce.
  - port **illegal declared value** ↔ upstream's pydantic message, **exactly**: `ScriptValueError`
    carries upstream's own sentence, produced by the same `validateField` rules, and the port emits
    it unchanged and unprefixed at the same `design` location — the two stdouts for that vector are
    byte-identical, 1411 B, diffed. Not a divergence; listed here only so the count is complete. A
    boolean declared in Lua is echoed in the Input Value column as Lua spells it (`true`), where
    upstream prints Python's `True`, which is D-002's substitution showing through rather than a
    second gap.
- **Not licensed by this — the boundary of the entry:** a theme folder with **no script at all** is
  not a failure. Measured on both sides: exit 0, the CV renders, the theme falls back to its base
  defaults. That path is unchanged.
- **What this closes:** all four modes used to fail *silently* — `bridge.themeScript` returned the
  same `nil` for a broken script as for an absent one, and the document rendered with base defaults
  at exit 0. That was the divergence that mattered. It supersedes D-002's "Error parity where it
  can be kept: not landed yet" paragraph.
- **User notices:** a broken theme script is reported instead of ignored, naming the script and the
  reason, in Lua's terms rather than Python's.

---

## D-018 — The `Commands` help panel is captured in a canonical order

**Status:** approved (agent-executable per `specs/STATE.md`, policy change 2026-08-12) ·
**Iteration:** 16

- **Differs:** the order of the three entries in the `╭─ Commands ─╮` panel of `--help` is
  **normalized to ascending command name** on both sides of every golden comparison — in
  `tools/gengolden`'s capture of upstream, and in `internal/conformance.Normalize`'s treatment of
  the port's output. Nothing else in the panel is touched: the same rows, the same column widths,
  the same byte count.
- **Upstream:** `src/rendercv/cli/app.py:142-151` registers the commands by walking its own package
  directory —

  ```python
  cli_folder_path = pathlib.Path(__file__).parent
  for file in cli_folder_path.rglob("*_command.py"):
      ...
      module = importlib.import_module(full_module)
  ```

  `pathlib.Path.rglob` yields raw `os.scandir` order; it is not sorted, unlike
  `sorted(rglob(...))`. Typer then lists subcommands in **registration** order rather than click's
  sorted one (`typer/core.py:816-820`: *"Note that in Click's Group class, these are sorted. In
  Typer, we wish to maintain the original order of creation (cf Issue #933)"* — click's own
  `Group.list_commands` is `sorted(self.commands)`, `click/core.py:1784-1786`).
- **Why parity is not the right target here:** there is no upstream order to be identical to. The
  order is a property of the directory the interpreter happens to read, not of the release.
  Measured on the pinned submodule `2eba248`, one interpreter, one command
  (`COLUMNS=80 NO_COLOR=1 TERM=dumb rendercv --help`):

  ```
  the submodule checkout : create-theme, new, render
  a plain `cp -r` of src/: render, new, create-theme
  ```

  Both are 2,433 bytes and differ **only** in which of the three entries comes first — the column
  width is set by the longest command name, which reordering does not change. So the committed
  golden records a coin flip, `gengolden -verify` fails on any checkout whose readdir order landed
  the other way (this is what broke `cli_help` and `cli_help_short` off the generating machine),
  and no port behavior could fix it: matching a value that is not a function of upstream's source
  is not achievable, and would not mean anything if it were.
- **Instead:** `rendercv-go` lists `create-theme, new, render`, fixed, from
  `internal/cli/helpdata/help.json` — the same order the pinned checkout happens to print, which is
  also the ascending one. `internal/conformance/cmdpanel` puts both sides into that order before
  they are compared. It is deliberately narrow: only a `╭─ Commands` panel, only whole entries
  (an entry's wrapped continuation rows move with it), and any panel shape it does not recognize is
  passed through untouched. `Options` and `Arguments` panels are **not** reordered — their order is
  written in upstream's source, so a difference there is a real defect and stays one.
- **User notices:** nothing. The port's own listing is unchanged; only what the fixtures compare is.
- **Consequence for the suite:** `cli_help` and `cli_help_short` stay unreachable under D-010 (the
  binary name re-wraps their prose), and are unaffected by this entry in `TestParity`. What changes
  is `gengolden -verify`: it now passes from a foreign checkout of the pinned upstream.
  **Measured, both directions**: with the flipped copy on the child's `PYTHONPATH`,
  `gengolden -verify -case cli_help` and `-case cli_help_short` report *goldens match the pinned
  upstream 2eba2481*; with `cmdpanel.Sort` removed and nothing else changed, the same command
  reports *golden differs from the committed bytes: cli_help/stdout.txt*.
- **Not licensed by this:** any other reordering of upstream output. Ordering upstream states in
  its source — sections, entries, options, the file list in `files.txt` — is contractual and is
  compared as-is. This entry covers exactly the one panel upstream fills from a directory listing.

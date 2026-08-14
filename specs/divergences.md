# Divergences from upstream RenderCV

Every deviation from `third_party/rendercv` @ `v2.8` lives here. Nothing else is permitted.

An agent may write entries directly. An unrecorded divergence is a bug, not a design choice.

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
    `withoutTreeConflicts`).
  - **Closed, corrected**: this paragraph used to claim a document setting an unknown key on a
    scripted custom theme is silently accepted. It is not. An unrecognised key is rejected the same
    way upstream's `extra="forbid"` rejects it, over the union of the built-in tree's fields and the
    script's own declared keys; the script is loaded during validation, not only at render time
    (`internal/schema/models/design/scriptextra_test.go`, `TestValidateScriptedThemeUnknownKeys`).
    **Re-measured end-to-end 2026-08-14**, not taken from the ledger: a `create-theme mytheme`
    folder on each side (upstream's 857-line `__init__.py`, the port's `return {}` `init.lua`), a
    generated CV with `design: {theme: mytheme, unknown_key_xyz: 5}`, rendered at `COLUMNS=80` with
    the vendored `third_party/rendercv/.venv/bin/rendercv` and with `bin/rendercv-go`. Both are
    **exit 1**, both write **1411 B to stdout and 0 B to stderr**, and the two stdouts are
    **byte-identical** (`cmp` clean): one row, `design` / `5` / `This field is unknown for this
    object. Please remove it.` Neither side creates `rendercv_output/`. Fixed since the 2026-08-10
    finding; the stale claim was found false by pass 24's fresh-context verifier and is now
    confirmed false by measurement.
  - Upstream's other custom-theme rules are preserved unchanged: lowercase-alphanumeric theme
    name, folder beside the input file, folder must contain ≥1 `*.j2.typ`.
  - **Sandboxed.** The Lua state is opened without `io`, `os`, `package`, `debug` or `dofile`.
    A theme describes a design; it has no business touching the filesystem or the network.
    Upstream's `__init__.py` had no such limit — this divergence is strictly safer.
  - Error parity for a **broken** script: **landed, and recorded in D-013** — this bullet used to
    say every script failure was swallowed at exit 0. It is not. Re-measured 2026-08-14 with a
    truncated `return {` in a `create-theme` folder's `init.lua`: the port exits **1**, writes
    **1411 B to stdout, 0 B to stderr**, and prints the validation panel with `design` / `...` /
    `The custom theme mytheme's init.lua file could not be run: <string> at EOF:   syntax error.`
    What still differs is the *reason string*, Lua's rather than Python's, which is D-013's subject
    and is not restated here. Upstream's `ImportError` branch has no analogue and stays dropped,
    since `package`/`require` are unavailable (D-013 again).
  - The **panel-vs-traceback** shape reached from this entry — `create-theme`'s two refusals are a
    stdout panel here and a stderr Rich traceback upstream — is **D-011's class**, generalised and
    bounded by **D-014**, which already names these two messages and the stream inversion. Not a
    D-002 item; see those entries rather than a second description here. (Spot-checked 2026-08-14:
    `create-theme "My Theme"` is exit 1 on both sides — port 637 B stdout / 0 B stderr, vendored
    upstream 0 B stdout / 2369 B stderr ending `RenderCVUserError: …`.)
  - **Not this entry's, filed where it belongs**: `specs/STATE.md`'s iteration-14 row attributes a
    `!!binary` contradiction to D-002. It is not in D-002 — the line numbers it cites are D-012's,
    and the sentence has been corrected there.
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

**Status:** approved · **Iteration:** 10

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

**Status:** approved · **Iteration:** 12

- **Differs:** **all fourteen** files `create_theme` writes differ from upstream's — one by name
  (`__init__.py` → `init.lua`) and **all thirteen** templates by bytes. This entry used to say
  "two of the fourteen", which measured false; see *Instead* for the split.
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
  transform of each template in place of the Jinja source.

  **Correction (measured 2026-08-13, `create-theme mytheme` on both sides, file by file):
  ZERO of the thirteen templates are byte-identical.** The earlier claim that "the other twelve
  files are byte-identical" was wrong on the count, not on the reasoning. The split is:
  - **5 differ by a missing final newline only** — `entries/{Bullet,Numbered,OneLine,
    ReversedNumbered,Text}Entry.j2.typ`, each exactly one byte shorter here.
  - **8 differ in content** — `Header.j2.typ` (935 → 913 B), `Preamble.j2.typ` (5944 → 5871 B),
    `SectionBeginning.j2.typ` (115 → 111 B), `SectionEnding.j2.typ` (68 → 64 B) and
    `entries/{Education,Experience,Normal,Publication}Entry.j2.typ`.

  Both classes are D-005 consequences, not new divergences: the trailing newline goes because
  Jinja's `keep_trailing_newline` defaults to False and `templater.py:34-44` does not set it, so
  the transform bakes in what Jinja would have stripped at parse time.
- **User notices:** the generated theme folder is scripted in Lua and its templates are in the
  port's dialect. It renders identically to the theme it was copied from, which the Jinja
  version would not.
- **Consequence for the suite:** the `create_theme` corpus case compares template *source*, so it
  is unreachable by construction. It stays red, with this entry as the reason. `new
  --create-typst-templates` writes the same thirteen `.typ` files through the same
  `copyTypstTemplates` path (`internal/cli/customtheme.go`), so `new_typst_templates` fails on
  exactly the same thirteen files — the 5/8 split above, not "four fragments" as this entry
  previously said — and stays red for the same reason, not a second divergence.
  `create_theme` also differs on **stdout**, not just the file set: the panel's second step reads
  `Edit ./mytheme/__init__.py to:` upstream and `Edit ./mytheme/init.lua to:` here — the same
  substitution, one more place it shows.

---

## D-009 — The `new` panel's next-step line names `rendercv-go`

**Status:** approved · **Iteration:** 12

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

**Status:** approved · **Iteration:** 12

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

**Status:** approved · **Iteration:** 12

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
either project does. Where the port diverges it is **stricter or equally strict in all six cases,
and laxer in none**. This sentence used to read "laxer in one (`!!binary`)", which contradicted the
table above it: re-measured 2026-08-14 at `COLUMNS=80` on a generated CV whose `cv.name` is
`!!binary aGk=`, the vendored `third_party/rendercv/.venv/bin/rendercv` **renders** — exit 0, 965 B
success panel, `rendercv_output/hi_CV.pdf` and five siblings — while `bin/rendercv-go` **refuses**:
exit 1, 1318 B stdout, 0 B stderr, one row reading `cv.name` / `aGk=` / `Input should be a valid
string.` Upstream renders and the port does not, which is the port being stricter. The measurement
wins over the old summary; the table's `!!binary` row was right all along.

### Four upstream internal errors found while measuring this, **not caused by tags**

Recorded here because this is where they were measured; each is D-011's class, not this entry's.

| Document | Upstream | `rendercv-go` |
|---|---|---|
| `cv: {1: x}`, and any other non-string key | `RenderCVInternalError: Key '1' not found in the YAML file.` | a validation record |
| `cv: {true: x}` | same | a validation record |
| `cv: {!!int 1: x}` | same | a validation record |
| `cv.website: []` | `RenderCVInternalError: website key present but value is None` | a validation record carrying that same sentence |

All four now differ only in the **shape** of the refusal — a validation table on stdout here, a
Rich traceback on stderr there — and agree on the exit code, which is what axis 2 binds. The
`cv.website: []` row did not: it rendered a complete CV at exit 0 until iteration 15's follow-up.
The cause is that `parse_connections` tests `website` for **falsiness**
(`src/rendercv/renderer/templater/connections.py:117-118`, `if not websites:`) where it tests
`phone`, `social_networks` and `custom_connections` for `is None` (`:95`, `:141`, `:164`), and an
empty sequence passes every validator while staying in `_key_order` —
`src/rendercv/schema/models/cv/cv.py:173` drops only the keys whose value **is None**. The port
now raises there too and reports it the way this class is reported, at `cv.website`, exit 1.

Measured on both sides, `cv:\n  name: A\n  website: <value>\n`, `NO_COLOR=1 TERM=dumb COLUMNS=80`:

| Value | Upstream | `rendercv-go` |
|---|---|---|
| `[]` | exit 1, traceback, `RenderCVInternalError: website key present but value is None` | exit 1, validation table, same sentence at `cv.website` |
| `{}` | exit 1, `URL input should be a string or URL.` | identical bytes |
| `null` | exit 0, renders — the key never enters `_key_order` | identical |
| `""` | exit 1, `This is not a valid URL.` | identical bytes |
| `0` | exit 1, `URL input should be a string or URL.` | identical bytes |
| `false` | exit 1, `URL input should be a string or URL.` | identical bytes |
| `https://example.com` | exit 0, renders | identical |

Pinned by `TestWebsiteShapesMatchUpstreamExitCodes` (`internal/cli/websitefalsiness_test.go`) and
`TestWebsiteFalsinessIsARecordNotARender` (`internal/renderer/bridge/connections_test.go`).

---

## D-013 — A broken theme script's reason string is Lua's, not Python's

**Status:** approved · **Iteration:** 14

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

## D-014 — Any upstream unhandled exception is reported as a clean error, not a traceback

**Status:** approved · **Iteration:** 13 (spec 013 §10, proposals P-1 and P-2)

- **Differs:** D-011 named exactly two vectors (`err_missing_file`, `err_bad_override_key`). Spec
  013 §5.2 counts twenty-two more `RenderCVInternalError` raise sites plus every undecorated
  `create-theme` failure that are the same class: upstream has no `RenderCVUserError` wrapper on
  the path, so Typer's default `sys.excepthook` prints a Rich traceback with this machine's
  absolute paths and this checkout's Python source lines baked in.
- **Upstream:** `cli/error_handler.py:38-49`'s `handle_user_errors` decorator only catches
  `RenderCVUserError`; every other exception reaches Typer's own handler. `create_theme_command.py`
  raises its two messages (`"The theme folder \"{name}\" already exists!"`,
  `"The custom theme name should only contain lowercase letters and digits. The provided value is
  \`{name}\`."`) undecorated, so both surface as the last line of a traceback on **stderr**, prefixed
  `RenderCVUserError: `, at 0 bytes stdout / 1348 bytes stderr (spec 013 §4.12, behavior 34) — not
  as the stdout panel every other `err_*` case gets.
- **Why parity is impossible:** identical to D-011 — a Go binary has no CPython call stack and no
  way to print source lines from `third_party/rendercv`'s `.py` files, and the paths baked into such
  a traceback are non-reproducible even for upstream on a different machine.
- **Instead:** `rendercv-go` reports the nearest clean error at the same exit code and, where
  determinable, the same stream — the `create-theme` messages already print as a stdout panel
  (`internal/cli/customtheme.go:44,59-60`), which for these two vectors is a **stream inversion**
  as well as a shape change (upstream: stderr traceback; port: stdout panel).
- **User notices:** a real error message instead of a Python stack trace with someone else's
  filesystem paths in it — arguably an improvement, never byte-identical.
- **Supersedes:** D-011's framing of "two goldens"; D-011 stays as the historical record of where
  this was first measured but is no longer the boundary of the class. `create-theme`'s stream
  inversion is the one instance with its own exact message pair and is called out by name so a
  future audit does not have to re-derive it.
- **Named vector — a `%YAML 1.<n>` whose minor part is neither 1 nor 2** (`1.0`, `1.3`, `1.9`,
  `1.10`, `1.20`, …). Not a `RenderCVInternalError` raise site and not a `RenderCVUserError`: an
  `AssertionError` out of ruamel itself. `process_directives` checks only the *major* part and
  raises a clean `ParserError` when it is not 1 (`ruamel/yaml/parser.py:296-304`); one line later it
  writes the version onto the loader (`:321`) and the property setter asserts
  `version minor part can only be 2 or 1, got (1, 3)` (`ruamel/yaml/main.py:849-851`). Measured on
  the vendored CLI: exit 1, **0 bytes stdout**, 12958 bytes stderr ending on that line. The port
  reports it as a stdout validation record carrying upstream's own assertion sentence — the one
  line of that traceback with no machine paths in it — at the same exit 1
  (`internal/schema/yamlreader/directive.go`'s `UnsupportedVersionError`). Same stream inversion as
  `create-theme`'s. A *major* part other than 1 is **not** in this class: it is ruamel's
  `ParserError`, reached first, and the port matches its panel byte for byte.
- **Also in this class, measured against the `settings` tree** (spec 006
  `spec-delta-settings-validation.md` §3.6): a non-mapping `settings` or
  `settings.render_command` (`err_settings_not_a_mapping`, `err_settings_render_command_null`)
  crashes upstream's CLI with `AttributeError`/`TypeError` at `render_command.py:205` →
  `run_rendercv.py:118-122`, before any validation runs. `rendercv-go` reports the record
  upstream's own model would have produced had the CLI reached it — `Input should be a valid
  dictionary or instance of Settings.` / `...or instance of RenderCommand.` — at the same
  location and the same exit code. Unlike `create-theme`'s pair there is no stream inversion:
  both sides use exit 1, and the port uses the validation panel it uses for every other record.
  **Mechanism F3 is in the same class**: a list-valued `render_command.design` (`err_settings_design_list`)
  crashes at the identical call site (`render_command.py:205` → `run_rendercv.py:120`) before
  validation runs, so its CLI-captured golden is also a non-reproducible traceback. Only *measured
  through the model layer directly* — spec 006 delta §3.3's footnote, not the CLI — does `design:
  [a]` give mechanism C's clean `path_type` record at `settings.render_command.design`; the port
  emits exactly that record, which is what a unit test pins instead of the CLI-level byte comparison.

---

## D-015 — `OS Error:` carries Go's `strerror` text, not Python's

**Status:** approved · **Iteration:** 13 (spec 013 §10, proposal P-3)

- **Differs:** upstream's path-B `OS Error: {exception}` panel embeds Python's `OSError.__str__`,
  which is `[Errno 13] Permission denied: '/abs/path'`. The port's `syscall.Errno`/`fs.PathError`
  produce Go's fixed shape, `<op> <path>: permission denied`.
- **Upstream:** `cli/render_command/run_rendercv.py:195-196` — `except OSError as e: raise
  RenderCVUserError(message=f"OS Error: {e}")`. Measured: `OS Error: [Errno 13] Permission denied:
  '{absolute path}'`, 637 bytes at `COLUMNS=80`.
- **Port:** `internal/cli/oserror.go` — `osErrorMessage` prefixes `OS Error: ` (matching that much)
  and absolutizes the path (matching upstream's `OSError.filename` semantics), but the body is
  `reported.Error()` / `errno.Error()`, e.g. `open /abs/out/John_Doe_CV.typ: permission denied` —
  552 bytes for the same vector.
- **Why parity is impossible:** `[Errno N] <Capitalized strerror>` is CPython's `OSError.__str__`
  formatting, keyed off `errno.errorcode` and `os.strerror`. Go's `syscall.Errno.Error()` returns
  the platform C library's `strerror` in a different shape (`<op> <path>: <lowercase strerror>`)
  with no public seam to reformat it as Python's without hand-mapping every errno the target
  platform can raise, per platform — the exact thing D-011 already rejects doing for a full
  traceback, at smaller scale but the same shape of cost.
- **Instead:** the port keeps `OS Error: ` and the absolute path (both already match), and accepts
  the message *body* diverging — Go's own `<op>: <strerror>` phrasing.
- **User notices:** the error class (`OS Error:`) and the failing path are identical; the exact
  wording of the underlying OS complaint is Go's, not Python's.

---

## D-016 — A pongo2 template-syntax error cannot carry Jinja's exception text

**Status:** approved · **Iteration:** 13 (spec 013 §10, proposal P-4) · **Extends D-005**

- **Differs:** D-005 already establishes that template *source* diverges (pongo2 syntax, not
  Jinja2) and that rendered *output* does not. It does not cover what happens when a user-supplied
  override template fails to *parse* — that error's text is Jinja's on the reference side and
  pongo2's here, by the same root cause as D-005.
- **Upstream:** `cli/render_command/run_rendercv.py:188-193` builds `Template syntax error in
  {filename} on line {lineno}: {exception}` from a caught `jinja2.TemplateSyntaxError`.
- **Why parity is impossible:** `{filename}` and `{lineno}` are reproducible (the port knows which
  override file it was reading and can track a line number), but `{exception}` is Jinja2's own
  parser diagnostic text, which has no pongo2 analogue — the two libraries do not fail on the same
  malformed input in the same place with the same words, for the reason D-005 already gives.
- **Instead:** the port reproduces the message *shape* (`Template syntax error in {filename} on
  line {lineno}: {pongo2's own message}`) and accepts the final clause diverging.
- **User notices:** a template syntax error names the right file and, where determinable, the right
  line; the diagnostic sentence explaining *what* is wrong is pongo2's phrasing, not Jinja2's.
- **Not yet measured against the rendered panel bytes** — spec 013 §11 flags this as unmeasured
  without a deliberately broken override template constructed and run; the message *template* above
  is read off upstream source, not observed.

---

## D-017 — `goccy/go-yaml` rejects some documents `ruamel` accepts

**Status:** approved · **Axis 1 (artifact parity)** · Costing:
[`specs/002-yaml-and-core-model/spec-delta-folding.md`](002-yaml-and-core-model/spec-delta-folding.md)

- **Differs:** a small number of YAML shapes that `ruamel` (upstream's parser) loads successfully
  make `goccy/go-yaml` v1.19.1 (the port's parser) refuse the document outright — `rendercv-go`
  cannot render a CV upstream renders. This is the **only axis-1 finding in the port**: every other
  divergence is a message, a stream, or a byte the user does not act on; this one is a document the
  user cannot render at all.
- **Measured, five classes:**

  | Class | Minimal shape | ruamel | port |
  |---|---|---|---|
  | Folded plain scalar, **flow mapping** context | `cv: {name: John⏎  Doe}` | renders (`John Doe`) | rejects: `',' or '}' must be specified` |
  | Folded plain scalar, **block** context | `k: 1⏎  - item⏎ q` → `{"k": "1 - item q"}` | renders | rejects |
  | Empty block scalar, blank line, comment | `k: >⏎⏎# c` → `{"k": ""}` | renders | rejects |
  | Collection tags on a scalar | `!!merge`, `!!seq`, `!!map`, `!!set`, `!!omap` | node exists | refused at parse |
  | Sequence keys | a sequence used as a mapping key | loads, then RenderCV raises | refused at parse |

  **Reach, measured 2026-08-14 — the block-folded row is reachable from an ordinary typo.** A
  randomised differential fuzzer over mutations of a real generated CV hit this class on its own,
  and it reduces to seven lines:

  ```yaml
  cv:
    name: A
    sections:
      s:
        - one
            - two
          - three
  ```

  Upstream renders it at **exit 0**; the port refuses at exit 1 with `while parsing a block
  collection.` The trigger is a **mis-indented nested bullet** — a plain scalar followed by a
  more-indented dash — which is a normal thing to typo in a CV, not a synthetic shape. Note the
  same document with `- bullet: one` instead of `- one` is exit 1 on **both** sides, so the class
  needs the entry to be a bare string.

  This does not change the costing spec's recommendation, which stands: option **(b)**, an upstream
  issue against goccy plus a pin here, **not** a local workaround. §6's fifth point is exactly this
  trade — the port fails loudly at exit 1, while an imperfect fold would corrupt values silently at
  exit 0. It does raise the priority of the upstream report, which is the one action still
  outstanding on this entry.

  **The sequence-key row is not an axis-1 loss, re-measured 2026-08-14.** ruamel *loads* a container
  key — `{[1]: a}` becomes `{(1,): 'a'}` — but nothing downstream can consume it, so upstream exits 1
  with an unhandled traceback. Measured on five shapes (`{[1]: a}`, `? [1]`, `? []`, `? {a: 1}`,
  `? [[1]]`): **both sides exit 1 on all five**, upstream printing a Python traceback and the port a
  one-row panel — which is D-011's class, not a document the user loses. So no CV is renderable
  upstream and unrenderable here on account of a container key, and this row costs the user nothing
  beyond the message shape. The other four classes above are the real axis-1 exposure.

  The flow and block folded-scalar classes share one mechanism — goccy's lexer already performs the
  fold correctly, but carries it forward with two narrow, crisply reproducible defects: the fold
  stops one line early in some shapes, and the folded token's position drifts with trailing blank
  lines. Fixing "goccy does not fold plain scalars the way ruamel does" settles both rows at once.
- **Why parity is impossible as-is:** goccy is a different parser implementation with its own
  strictness, not a configurable dial. Three options were weighed, in order of how directly each
  attacks the cause:
  1. **A reader-side workaround** (fold the text before goccy sees it) — costed in the linked spec
     delta and rejected. goccy's lexer already does the hard part (distinguishing a fold from a
     sequence needs the same indent-stack tracking the workaround would have to reimplement), so it
     looked cheap going in. What is actually needed is fixing two *position/tokenisation* bugs
     inside goccy's own lexer from the outside — the same class of problem `blockscan.go` needed
     358 lines and tolerates 26 wrong answers out of 82,418 to solve for *error messages*, where
     "26 wrong" is an acceptable defect rate. **A fold changes values, not messages** — a
     wrong fold silently corrupts a document that currently fails loudly into one that renders
     wrong, which is a strictly worse failure mode than refusing it. No corpus exists to gate a
     value-preserving proof at the scale this would need (the 170,003-document corpus is 88% parse
     failures; a fold's damage shows only on documents that succeed).
  2. **Fix it in goccy itself, upstream, with a pin here** — the chosen path. Both defects are on
     goccy's side of a YAML-spec disagreement (ruamel is spec-correct on both; goccy is not), small,
     and independently reproducible (spec-delta-folding.md §3.4), which makes them a normal upstream
     bug report rather than a project-specific request.
  3. **Do nothing, record only** — the fallback captured by this entry itself; always true
     regardless of 1 or 2, since either fix takes time to land and this entry documents the gap in
     the meantime.
- **Instead:** `rendercv-go` refuses these documents (`This is not a valid YAML file...`) where
  upstream renders them. **Recommendation, not yet executed:** file an issue against
  `github.com/goccy/go-yaml` describing the two lexer defects (spec-delta-folding.md §3.4 has
  minimal repros for both) and pin the current behavior here until a fix lands — filing that issue
  is an action on a third-party public tracker and is being left for explicit confirmation rather
  than taken as part of this pass.
- **User notices:** a hand-written CV using an unquoted value that continues on the next line inside
  `{ }`, a multi-line plain scalar folded across an over-indented sequence marker, or (separately)
  an empty block scalar followed by a blank line and a comment, an explicit YAML 1.1 collection tag
  on a scalar, or a sequence used as a mapping key — refuses to render here and renders upstream.
  Rare in practice: none of these shapes appear in any example, template, or generated file in
  either project.
- **Pinned by:** `TestGoccyRejectsAFoldedFlowScalar`
  (`internal/schema/modelbuilder/yamlerror_test.go`), written to `t.Skip` with a re-measure
  instruction so the evidence survives either way — a fix upstream, or a decision to workaround
  after all.

---

## D-018 — `%YAML 1.1` does not switch the scalar resolver

**Status:** approved · Costing: [`specs/002-yaml-and-core-model/spec-delta-directives.md`](002-yaml-and-core-model/spec-delta-directives.md) §6.4

- **Differs:** a document opening with a `%YAML 1.1` directive resolves plain scalars by YAML 1.1
  rules upstream — `yes`/`no`/`on`/`off`/`y`/`n` become booleans, a leading-zero integer like `010`
  is octal (`8`, not `10`), `0o10` is a plain **string** (1.1 has no `0o` octal prefix), and
  `1:30`/`1:30.5` are sexagesimal (base-60) numbers (`90`, `90.5`). `rendercv-go` resolves plain
  scalars by YAML 1.2 rules regardless of any `%YAML` directive present.
- **Upstream:** `ruamel/yaml/resolver.py:377-392` selects the resolver version from the scanner's
  directive-derived `yaml_version`, falling back to 1.2 only absent one; the 1.1 tables are
  `:30-35` (bool), `:45-53` (float, incl. sexagesimal), `:62-69` (int, incl. bare octal and
  sexagesimal).
- **Why not implemented:** a second, complete scalar-resolution table behind a directive that no CV
  in either project writes and that no upstream test covers (`grep '%YAML'
  third_party/rendercv/tests/` is empty) — pure speculative surface, the same shape of cost this
  project has repeatedly declined elsewhere (D-002's Lua `validate` callback, D-017's fold
  workaround) in favor of the smaller, honest option. **The decision is forced by D-017's own
  directive-parsing fix** (spec-delta-directives.md §6.1, tracked as its own implementation unit,
  not yet landed): once directive-headed documents parse at all, a `%YAML 1.1` document stops being
  loudly rejected — silently rendering it with 1.2 values instead would be strictly worse, a message
  defect turning into a value defect, exactly the asymmetry D-017 already weighs against.
- **Instead:** once the directive-parsing fix lands, a document opening `%YAML 1.1` is rejected with
  a named error identifying the unsupported directive, rather than silently resolved by 1.2 rules or
  left to fail with an unrelated message. The exact error text is that fix's to choose.
- **User notices:** nothing, unless they hand-write a `%YAML 1.1` directive — a construct absent from
  every example, template, and generated file in both projects.

---

## D-019 — The `Commands` help panel is captured in a canonical order

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

---

## D-020 — A document with more than one YAML directive is refused

**Status:** approved · **Axis 1 (artifact parity)** · Costing:
[`specs/002-yaml-and-core-model/spec-delta-directives.md`](002-yaml-and-core-model/spec-delta-directives.md)

- **Differs:** `%YAML 1.2\n%TAG !e! tag:x,1:\n---\n<CV>` — a `%YAML` directive **and** a `%TAG`
  directive on the same document, each legal alone (D-017/D-019's directive-parsing fix landed both)
  — renders upstream. The port refuses the whole document.
- **Upstream:** `ruamel/yaml/parser.py:288-330`'s `process_directives` loops over every
  `DirectiveToken` before the document body, so any number of distinct directives compose freely;
  `specs/002-yaml-and-core-model/spec-delta-directives.md` §4.1.3 measured this directly (multiple
  directives are allowed, cited there).
- **Why parity is impossible as found:** `goccy/go-yaml` accepts **at most one** directive per
  stream — a second directive line makes goccy treat the stream as carrying more than one document
  and fail the whole parse with its own `[1:1] unexpected directive value. document not started`,
  before any of the port's directive-handling code (D-017/D-019's fix) ever runs. This is a parser
  limitation of the same shape and in the same dependency as D-017 (`goccy/go-yaml` rejects some
  documents `ruamel` accepts) — not a gap in the port's directive logic, which was built and tested
  against exactly this shape and correctly declines to fabricate a phrasing goccy's parse state
  doesn't support (`internal/schema/modelbuilder/directivescan.go`).
- **Instead:** `rendercv-go` reports goccy's own parse error rather than upstream's phrasing or a
  fabricated one — the same choice D-017 already documents for a stricter parser: the leaked
  goccy-native text is the honest failure until the strictness itself is decided.
- **User notices:** a hand-written CV combining a `%YAML` version directive with a `%TAG` handle
  directive on the same document fails to render here and renders upstream. Rare: no example,
  template, or generated file in either project uses even one directive, let alone two together.
- **Recommendation:** the same as D-017's — an upstream issue against `goccy/go-yaml`, since this is
  the same "at most one directive" limitation as the class D-017 already tracks, filed together if
  D-017's issue is ever opened. Not filed this pass, for the same reason D-017 gives: it is an action
  on a third-party public tracker, left for explicit confirmation.

---

## D-021 — A `design` mapping with no `theme` key is a panel, not a `KeyError` traceback

**Status:** approved · **Iteration:** 6 (STATE.md's open finding) · **An instance of D-011's
class**, superseded in scope by D-014 — see both; the reasoning is not restated here.

- **Differs:** upstream prints a Rich traceback on stderr (`KeyError: 'theme'`, 9,611 B) with the
  progress box alone on stdout (522 B); `rendercv-go` prints its own validation panel on stdout
  (1,411 B) and nothing on stderr. **Both exit 1** — that is the part this entry closes.
- **Upstream:** `schema/models/design/design.py:35-57`. `validate_design` tries the discriminated
  union (`:36`), catches its `ValidationError`, finds `ctx['discriminator'] == "'theme'"` in it
  (`:38-47`), concludes the block must name a custom theme, and then reads `design["theme"]`
  unguarded at `:57`. There is no key, so it raises — and `handle_user_errors`
  (`cli/error_handler.py:38-49`) only catches `RenderCVUserError`, so Typer's excepthook prints
  the traceback. Measured against the vendored binary, non-tty, `NO_COLOR=1 TERM=dumb COLUMNS=80`:

  | Document | Upstream | `rendercv-go` |
  |---|---|---|
  | *(no `design` key)* | renders, exit 0 | renders, exit 0 |
  | `design: {theme: classic}` | renders, exit 0 | renders, exit 0 |
  | `design: {page: {size: us-letter}}` | `KeyError`, 522 B out / 9,611 B err, **exit 1** | panel, 1,411 B out, **exit 1** |
  | `design: {}` | `KeyError`, 522 B / 9,611 B, **exit 1** | panel, 1,411 B, **exit 1** |
  | `design: {bogus: 1}` | `KeyError`, 522 B / 9,611 B, **exit 1** | panel, 1,411 B, **exit 1** |
  | `design: {page: {size: bogus}, zzz: 1}` | `KeyError`, 522 B / 9,611 B, **exit 1** | panel, 1,411 B, **exit 1** |
  | `design: [1]` | `TypeError`, 522 B / 10,600 B, exit 1 | panel, 1,411 B, exit 1 |
  | `design: null`, `design:` *(no value)*, `design: hello` | panel, exit 1 | **byte-identical** panel, exit 1 |

  Only a `design` **mapping** reaches `:57`; a non-mapping fails the union with
  `model_attributes_type` instead, which is not a discriminator failure, so `:43`'s re-raise sends
  it to the ordinary table — those three rows already matched and still do.
- **Why parity is impossible:** D-011 and D-014. A Go binary has no CPython call stack and cannot
  print source lines out of `third_party/rendercv`.
- **Instead:** the port reports `union_tag_not_found` at `design` with pydantic's own message,
  `Unable to extract tag using discriminator 'theme'`. **The message is not invented**: it is the
  error `design.py:36` builds, `:38-47` inspects and then discards, verified by calling
  `built_in_design_adapter.validate_python({})` in the vendored venv; and it is exactly what
  `locale` — the same construct without a wrap validator in front of it — prints to a user for the
  same shape, down to the `...` in the Input Value column. This follows D-012's stated precedent
  for this class: "The port keeps the forced kind and reports an ordinary validation record at
  exit 1, which is deliberately the closest available behavior."
- **What changed:** before this entry the port **exited 0 and wrote five artifacts** for all four
  crashing shapes, silently defaulting the theme — a violation of contract axis 2 (exit codes) on
  top of the shape difference, and the only remaining case where the port rendered a document
  upstream refuses outright. Axis 2 is now satisfied for every `design` shape measured; what
  remains is the traceback text, which is permanent under D-011.
- **User notices:** a one-row error table naming `design` instead of a Python stack trace with
  someone else's filesystem paths in it.

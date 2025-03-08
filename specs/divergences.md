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

## D-002 — Custom themes cannot execute user Python

**Status:** approved in principle, replacement design pending (iteration 6)

- **Differs:** upstream loads and executes a custom theme's `__init__.py` during validation, using
  the resulting pydantic class as the theme's option schema.
- **Upstream:** `src/rendercv/schema/models/design/design.py` — `validate_design()`, the
  `importlib.util.spec_from_file_location` block; also
  `src/rendercv/cli/create_theme_command/create_init_file_for_theme.py`.
- **Why impossible:** Go has no runtime Python interpreter, and embedding one would defeat the
  single-static-binary goal. There is no equivalent of importing arbitrary user code.
- **Instead:** a declarative theme manifest — `<theme>/options.yaml` — describing option names,
  types, defaults, and constraints. `rendercv-go create-theme` emits it; validation reads it.
  Upstream's other custom-theme requirements are preserved unchanged: lowercase-alphanumeric
  theme name, folder must sit beside the input file, folder must contain ≥1 `*.j2.typ`.
- **User notices:** a custom theme written for Python RenderCV needs its `__init__.py` translated
  into `options.yaml`. Themes with no `__init__.py` (templates only) work unchanged.
- **Open:** the manifest schema is specified in iteration 6, not before.

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

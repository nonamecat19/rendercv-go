# Spec 016 — The public Go API (`pkg/rendercv`)

**Status:** draft · **Inherits:** [`specs/000-parity-contract/spec.md`](../000-parity-contract/spec.md)
· **Depends on:** every iteration 2–15, since this is a facade over them

**Upstream covered:**

- `src/rendercv/__init__.py` (the whole of the package's declared surface)
- `src/rendercv/cli/render_command/run_rendercv.py` (the orchestrator, and the only caller that
  shows what the de-facto API is)
- `src/rendercv/schema/rendercv_model_builder.py` (the two entry points into the schema half)
- `src/rendercv/renderer/{typst,pdf_png,markdown,html}.py` (the five generators)
- `src/rendercv/exception.py` (the whole error surface)
- `docs/api_reference/api_reference.py` (evidence about what upstream considers public)

Citations to `src/...` and `docs/...` are relative to `third_party/rendercv/`.

---

## 1. Purpose

`AGENTS.md` §3 lists `pkg/rendercv` as "the public Go API — stable surface, documented, semver'd",
and §9 requires every exported symbol in it to carry a doc comment naming the upstream construct it
mirrors. **The directory does not exist.** This spec defines what goes in it.

It is the only iteration in this port with **no parity axis**. The four axes — artifact, CLI, JSON
Schema, validation-error — are all about what the *binary* produces. None of them constrains a Go
library surface, because upstream has no Go library. That absence is the central fact this spec has
to handle honestly rather than paper over.

### 1.1 The finding that shapes everything below

**Upstream declares no public API.** Measured, not inferred:

| Fact | Evidence |
|---|---|
| The top-level `__init__.py` binds only `__version__` and `__description__`, and filters a Pydantic warning. No `__all__`, no re-exports | `src/rendercv/__init__.py:1-8` |
| Every other `__init__.py` in the tree is **0 bytes** — `cli/`, `renderer/`, `schema/`, `schema/models/`, `cv/`, `design/`, `locale/`, `settings/`, `cv/entries/`, `cv/entries/bases/`, and each `cli/*_command/` | `wc -l` over all `__init__.py` under `src/rendercv` |
| No `py.typed` marker anywhere under `src/` | `find src -iname py.typed` → empty |
| The docs autogenerate an mkdocstrings page for **every** `.py` file except `__init__.py`/`__main__.py` — the entire module tree is reference material, undifferentiated | `docs/api_reference/api_reference.py:13-29` |
| `pyproject.toml` declares only the `rendercv` console script; no extras distinguish a library subset | `pyproject.toml:76-81` |
| Every internal import is fully module-qualified — `from rendercv.renderer.html import generate_html`, never `from rendercv import ...` | `src/rendercv/cli/render_command/run_rendercv.py:9-18` |

So there is no upstream contract to transcribe. The de-facto API is *whatever a caller imports from
internal module paths*, which is what the CLI itself does.

### 1.2 The scope decision

Three readings were possible: mirror the seven functions the CLI calls; add a one-call convenience
entry point; or re-export the whole module tree the way upstream's docs treat it.

**Decision (human, 2026-08-12): mirror the seven CLI-called functions**, plus the model roots and
the error types they traffic in. Rationale, recorded because a later reader will ask:

- Every exported symbol then names a real upstream function, which is exactly what §9 requires. The
  other two readings both contain symbols that mirror nothing.
- Re-exporting the whole tree would freeze every type in `internal/` as semver'd public surface, so
  any future refactor of the port's internals becomes a breaking change — a permanent tax paid to
  match a "boundary" upstream never actually declared.
- A one-call `Render` was considered and **cut**: it mirrors no single upstream function, so it
  would need a `divergences.md` entry, and that file is human-gated (`AGENTS.md` §5). It is recorded
  in §7 as a candidate for a later iteration rather than smuggled in here.

### 1.3 What is deliberately *not* mirrored

`run_rendercv` (`run_rendercv.py:127-131`) is the obvious candidate for "the" entry point and is
**not library-usable**: its second parameter is a `ProgressPanel`
(`cli/render_command/progress_panel.py`), a Rich `Live` console wrapper, so calling it headlessly
means constructing a terminal UI object. Its *composition* is worth mirroring; its signature is not.
The port already reimplements that composition in `internal/cli`, and `pkg/rendercv` exposes the
same seven pieces it composes from.

## 2. Inputs / Outputs

**Input:** a Go program importing `pkg/rendercv`, with no dependency on `internal/`.

**Output:** the same artifacts the CLI produces, byte-for-byte, from equivalent calls. The parity
axes bind the *artifacts* this API produces exactly as they bind the CLI's — §5.1 makes that
mechanical.

## 3. Behavior

### 3.1 The seven functions

Traced from `run_rendercv.py:127-198`, in the order that function calls them. Each row is one
symbol `pkg/rendercv` must expose, and its upstream doc-comment referent under §9.

| # | Upstream | Citation | Signature |
|---|---|---|---|
| 1 | `read_yaml_with_validation_errors` | `schema/rendercv_model_builder.py:65-84` | `(yaml_content: str, yaml_source: YamlSource) -> CommentedMap` — parses YAML, converting ruamel errors into `RenderCVUserValidationError` |
| 2 | `build_rendercv_dictionary_and_model` | `schema/rendercv_model_builder.py:192-210` | `(main_yaml_file: str, *, input_file_path: pathlib.Path \| None = None, **kwargs: Unpack[BuildRendercvModelArguments]) -> tuple[CommentedMap, RenderCVModel]` — the full pipeline: YAML string → merged dict + validated model |
| 3 | `generate_typst` | `renderer/typst.py:9-29` | `(rendercv_model: RenderCVModel) -> pathlib.Path \| None` |
| 4 | `generate_pdf` | `renderer/pdf_png.py:16-40` | `(rendercv_model: RenderCVModel, typst_path: pathlib.Path \| None) -> pathlib.Path \| None` |
| 5 | `generate_png` | `renderer/pdf_png.py:47-91` | `(rendercv_model: RenderCVModel, typst_path: pathlib.Path \| None) -> list[pathlib.Path] \| None` |
| 6 | `generate_markdown` | `renderer/markdown.py:9-29` | `(rendercv_model: RenderCVModel) -> pathlib.Path \| None` |
| 7 | `generate_html` | `renderer/html.py:9-33` | `(rendercv_model: RenderCVModel, markdown_path: pathlib.Path \| None) -> pathlib.Path \| None` |

### 3.2 The `None` return is a behavior, not an absence

Each of the five generators returns `None` — not an error — when its corresponding
`settings.render_command.dont_generate_*` flag is set (`renderer/typst.py:9-29` and the parallel
guards in the other four). This is a **normal, successful outcome**, and it is the single most
likely thing for a Go port to get wrong by collapsing it into an error or an empty string.

The Go surface must keep "was not generated because it was switched off" distinguishable from both
"generated at this path" and "failed". Which Go shape expresses that is `plan.md`'s decision, not
this file's.

### 3.3 The options struct

`BuildRendercvModelArguments` is a `TypedDict(total=False)` — every key optional
(`schema/rendercv_model_builder.py:24-39`):

| Key(s) | Type |
|---|---|
| `design_yaml_file`, `locale_yaml_file`, `settings_yaml_file` | `str \| None` |
| `output_folder`, `typst_path`, `pdf_path`, `markdown_path`, `html_path`, `png_path` | `pathlib.Path \| str \| None` |
| `dont_generate_typst`, `dont_generate_html`, `dont_generate_markdown`, `dont_generate_pdf`, `dont_generate_png` | `bool \| None` |
| `overrides` | `dict[str, str] \| None` |

`total=False` means an absent key and an explicitly-`None` key are the same thing upstream. Note the
tri-state on the `dont_generate_*` booleans: `None` (absent), `True`, `False` are three distinct
inputs to the merge, and whether `False` differs from absent must be measured against the settings
merge before the Go type is chosen.

### 3.4 The model types

All are pydantic models reachable only by full internal module path — there is no short alias for
any of them, so the Go names are the port's own and the doc comment carries the upstream path.

| Type | Citation | Construction contract |
|---|---|---|
| `RenderCVModel` | `schema/models/rendercv_model.py:14-61` | Every field optional with a `default_factory` (`cv`, `design`, `locale`, `settings`); `model_config` even removes `cv` from the JSON-Schema `required` list (`:18`) |
| `Cv` | `schema/models/cv/cv.py:31` | Carries `PrivateAttr`s `_plain_name`, `_connections`, `_top_note`, `_footer` with **no defaults** — required to be set post-validation — plus `_key_order` (`cv.py:120-126`) |
| `Design`, `Locale`, `Settings` | `schema/models/design/design.py`, `.../locale/locale.py`, `.../settings/settings.py` | `Settings` carries `_resolved_current_date: PrivateAttr()` |
| `Section` and the 9 entry types | `schema/models/cv/section.py:80,90,128,181,320`; `cv/entries/*.py` | Sections are **dynamically generated** pydantic models, not static classes — `create_section_models` builds them |

**A hazard for the facade.** `RenderCVModel._input_file_path` is a `PrivateAttr` set by an
`after` model validator (`rendercv_model.py:44-61`) and then **read directly by other modules** —
`renderer/pdf_png.py:36` and `:73` reach into `rendercv_model._input_file_path`. So it is private by
naming convention and public by use. Any Go model type that omits it will fail to drive
`GeneratePDF`/`GeneratePNG` correctly. Upstream's own convention doc states "Never use
underscore-prefixed names… All names are public", so these `PrivateAttr`s are pydantic mechanics
rather than a privacy boundary — which matches `AGENTS.md` §9's rule and confirms it.

### 3.5 The error surface

All four live in `src/rendercv/exception.py`, and a library caller must be able to distinguish them:

| Upstream | Citation | Shape | Raised by |
|---|---|---|---|
| `RenderCVUserError(ValueError)` | `exception.py:22-24` | `@dataclass`, `message: str \| None = None` | `run_rendercv.py:196` (wrapping `OSError`), `:187-194` (wrapping Jinja `TemplateSyntaxError`) |
| `RenderCVUserValidationError(ValueError)` | `exception.py:27-29` | `@dataclass`, `validation_errors: list[RenderCVValidationError]` | `rendercv_model_builder.py:82,91-99` and through the validation pipeline |
| `RenderCVInternalError(RuntimeError)` | `exception.py:32-34` | `@dataclass`, `message: str` | `generate_png` at `pdf_png.py:83` |
| `RenderCVValidationError` | `exception.py:20-25` | **Not an exception** — a plain `@dataclass` payload: `schema_location`, `yaml_location`, `yaml_source`, `message`, `input` | carried inside the above |

`YamlSource = Literal["main_yaml_file", "design_yaml_file", "locale_yaml_file",
"settings_yaml_file"]` (`exception.py:5-10`) — a closed set, and parameter 2 of function 1.

No other exception types exist under `src/rendercv/**`.

The distinction that must survive into Go: `RenderCVUserError` and `RenderCVUserValidationError`
both descend from `ValueError` and mean "the user's input is wrong"; `RenderCVInternalError`
descends from `RuntimeError` and means "the port broke". The port already types this tree
(`schema.ValidationError`, `schemaerr.UserValidationError`), because its rendering is part of
Axis 4 — `pkg/rendercv` re-exports that tree rather than defining a second one.

## 4. Exact user-visible strings

This iteration introduces none of its own. Two exist in the covered upstream and belong to whoever
ports the surrounding behavior:

- `"Typst compiler returned None for PNG bytes"` — `renderer/pdf_png.py:83`, the message of the one
  `RenderCVInternalError` in the generator path.
- The `[full]` reinstall panel — `cli/entry_point.py:17-27` — which is CLI-only and out of scope
  here, noted so a later reader does not go looking for it in the library surface.

## 5. Acceptance criteria

1. **`pkg/rendercv` compiles with no import of `internal/`** — enforced by a test, not by review.
   (Go permits `pkg/` → `internal/` within the same module; the constraint is architectural, so it
   needs a real check.)
2. **The artifact differential runs through the library, not the binary.** At least one corpus case
   is rendered by calling `pkg/rendercv` directly and its `.typ`/`.md`/`.html` compared to the same
   case's golden. This is what makes §2's claim mechanical: if the facade drifts from the CLI, a
   golden catches it.
3. **All seven functions are exercised**, including each generator's `dont_generate_*` path, with
   the "switched off" outcome asserted as distinct from both success and failure (§3.2).
4. **Every exported symbol has a doc comment naming its upstream construct** with a
   `src/rendercv/...:LINE` citation — `AGENTS.md` §9. Enforced by a test that parses the package and
   fails on an exported symbol whose doc comment lacks a citation, in the shape of
   `internal/kindguard`. A convention that is only a comment is not a constraint; this port has
   already learned that once (iteration 15, open item 7).
5. **Each of the four error types is reachable and distinguishable** from a library call, with a
   test that asserts `errors.Is`/`errors.As` behavior rather than string matching.
6. **`go vet` and `golangci-lint` clean**, and every commit leaves `go build ./... && go test ./...`
   green (`AGENTS.md` §7).

## 6. Edge cases

1. **`png_path` numbering.** `generate_png` writes `{stem}_{i+1}.png` and returns a *list*
   (`pdf_png.py:47-91`). The list is the return value, so a Go caller must get the same ordering and
   the same names, and the empty-vs-`None` distinction of §3.2 applies to a slice here.
2. **A model constructed in Go rather than parsed.** Every `RenderCVModel` field has a
   `default_factory`, so `RenderCVModel()` is legal upstream and produces a document. The
   `PrivateAttr`s of §3.4 are *not* set on such a model until a validator runs — so a Go caller
   building a model by hand and passing it to `GeneratePDF` hits the `_input_file_path` hazard
   directly. The facade must define what happens; the options are to require construction through
   `Build`, or to give the hand-built path a defined default.
3. **`overrides` is `dict[str, str]`** — the same string-keyed override map the CLI parses from
   `--key=value`. A library caller supplies it already parsed, so the CLI's parse-error surface
   (iteration 12) is not reachable through this API, but the *apply*-time errors are.
4. **Relative paths.** Every path option is `Path | str | None` and is resolved against
   `input_file_path`, not the process working directory. A library caller may have no input file at
   all (function 2's `input_file_path` defaults to `None`), and what the generators then resolve
   against must be measured rather than assumed.

## 7. Out of scope

- **A one-call `Render`** composing all seven — cut in §1.2. It mirrors no upstream function, so it
  needs a `divergences.md` entry and the human gate. Candidate for a later iteration.
- **`run_rendercv` itself**, for the reason in §1.3.
- **Re-exporting the theme and locale catalogs** as public types. The analyst did not enumerate
  `design/other_themes/` or `locale/other_locales/` class-by-class, and this surface does not need
  them: themes and locales enter through YAML, not through Go identifiers.
- **Semver freezing.** `AGENTS.md` §3 calls this surface semver'd, and the stretch-goal list in
  `STATE.md` carries "Public `pkg/rendercv` API frozen and semver'd" as a separate item. This
  iteration builds the surface; declaring it frozen is a release decision behind the human gate.
- **A `py.typed`-equivalent stability claim.** Upstream ships none (§1.1), so the port claims none
  beyond what §5 tests.

## 8. Open questions for `plan.md`

1. How the tri-state `dont_generate_*` (`None`/`True`/`False`, §3.3) maps to Go — pointer-to-bool, a
   dedicated tri-state type, or a functional-option pattern where absence is expressible.
2. How "not generated because it was switched off" is expressed in a Go return (§3.2).
3. Whether the model types are re-exported aliases of the `internal/` types or a distinct
   representation with a conversion boundary. Aliases are cheaper and make criterion 1 harder to
   satisfy honestly; a conversion boundary is real work and real drift risk.
4. Whether `Build` returns the `CommentedMap` equivalent at all. Upstream returns it as tuple
   element 0 and the CLI uses it; a Go caller may not need it, but omitting it diverges from the
   mirrored signature.

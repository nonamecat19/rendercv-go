# Plan 016 — The public Go API (`pkg/rendercv`)

**Spec:** [`spec.md`](spec.md) · **Status:** draft

This file is the Go design. The behavior it implements is spec 016; where this file and the spec
disagree, the spec wins.

---

## 1. What the survey of `internal/` changed

The spec was written from upstream. Three facts about the port's own shape, measured before
designing, change what the facade has to be.

### 1.1 There are no `Generate*` functions to wrap

Upstream's five generators each **write a file and return its path**
(`renderer/typst.py:9-29` and parallels). The port split those two jobs:

| Port | Signature | Returns |
|---|---|---|
| `document.Render` | `(bridge.Document, templater.Format, document.Options) (string, error)` | rendered **content** |
| `document.RenderHTML` | `(bridge.Document, markdown string, document.Options) (string, error)` | rendered **content** |
| `typstc.Compile` | `(typstc.Request) (…, error)` | writes PDF/PNG via `Request.OutputPath` |

`internal/cli/render.go` owns the writing, the path derivation and the `dont_generate_*` gates —
`document.Render` at `:210`, `RenderHTML` at `:289`, `typstc.Compile` at `:373` and `:415`.

**Consequence:** `pkg/rendercv`'s generators cannot be thin re-exports. Each one must compose
render + path-derive + write to mirror upstream's path-returning contract. That logic exists once, in
`internal/cli`, and duplicating it into `pkg/` would create exactly the drift spec §5 criterion 2 is
meant to catch.

**Decision:** extract the compose-and-write step out of `internal/cli/render.go` into a new
`internal/renderer/generate` package with one function per upstream generator, and have **both**
`internal/cli` and `pkg/rendercv` call it. The CLI keeps its progress panel, its error handling and
its flag parsing; it loses only the five bodies it currently inlines. `pkg/rendercv` then *is* a thin
re-export, and there is one implementation, not two.

This is a refactor of working, parity-green code, so it lands as its own commits **before** any
`pkg/` file exists, each one leaving the suite green (`AGENTS.md` §7). It is the largest risk in this
plan; §6 covers it.

### 1.2 The error tree already mirrors upstream 1:1

`internal/schema/schemaerr` already defines `UserError`, `UserValidationError`, `InternalError` and
`ValidationError` with doc comments naming their upstream counterparts (`error.go:33-95`), because
Axis 4 required it. Spec §3.5 is therefore already satisfied inside `internal/`.

**Decision:** `pkg/rendercv` re-exports these as **type aliases** (`type UserError =
schemaerr.UserError`), not as new types. An alias is identical to the aliased type, so
`errors.As(err, &rendercv.UserValidationError{})` matches an error produced deep inside `internal/`
with no conversion and no wrapping. Defining parallel types would require translating every error at
the boundary and would silently break `errors.As` for anyone mixing the two.

### 1.3 `RenderCVModel` already has upstream's `_input_file_path` hazard, in the same shape

Spec §3.4 flags `RenderCVModel._input_file_path` as private-by-name and public-by-use. The port
carries the identical arrangement: `rendercvmodel.go:47-62` has an **unexported** `inputFilePath`
field, set out-of-band after validation, with a comment citing `rendercv_model.py:44-62`.

Because it is unexported, a caller outside `internal/schema/models` **cannot set it**. So a Go
program cannot hand-build a `RenderCVModel` and render it — the hazard spec §6.2 raises is, in this
port, a hard wall rather than a footgun.

**Decision:** that wall is the answer to spec §6.2. `pkg/rendercv` documents that a `Model` is
obtained from `Build` and from nowhere else; there is no exported constructor. This diverges from
upstream, where `RenderCVModel()` is legal — but it diverges toward refusing what upstream would
mis-handle, and it needs no `divergences.md` entry because it is a property of the Go API surface,
which has no parity axis (spec §1). Recorded in §5 below.

## 2. Package layout

```
pkg/rendercv/
  doc.go          package doc: what this mirrors, and the "obtained from Build" rule
  rendercv.go     the seven functions
  options.go      BuildOptions and its tri-state (§3.1)
  types.go        aliases: Model, Document, and the four error types
  api_test.go     the exported-symbol doc-comment check (spec §5.4)
  example_test.go runnable examples, which double as the API's documentation

internal/renderer/generate/    NEW, extracted from internal/cli/render.go (§1.1)
  generate.go     Typst, PDF, PNG, Markdown, HTML — compose render + path + write
```

No subpackages under `pkg/`. Spec §7 cut the theme and locale catalogs, and a flat package keeps the
surface countable — a reviewer can read `rendercv.go` and see the whole API.

## 3. The four open questions from spec §8

### 3.1 Q1 — the tri-state `dont_generate_*`

**This is the one question with a latent defect behind it.**

Upstream's `BuildRendercvModelArguments` types these as `bool | None` with `total=False`
(`rendercv_model_builder.py:24-39`). The port's `modelbuilder.BuildArguments` (`merge.go:17-35`)
types them as **plain `bool`**.

Absent and `False` are not the same input. They merge into `settings.render_command`, so:

- **absent** → whatever `settings.yaml` says, which may be `true`
- **`False`** → explicitly force generation on, overriding a `settings.yaml` `true`

The port's collapse to `bool` is safe **only because of how the CLI reaches it**: the flags are
`--nopdf`-style switches that can only be set true, so from the CLI absent and false are genuinely
the same. A library caller has no such restriction, and `BuildOptions{DontGeneratePDF: false}` — the
zero value, which every Go caller gets by default — would silently mean "absent" when the caller may
have meant "force on".

**Decision:** `*bool` in `BuildOptions`, with `nil` meaning absent. Not a functional-option pattern:
options-as-functions hide the field set from `go doc`, and this struct's whole job is to mirror a
documented upstream key set field-for-field. Not a custom tri-state type either — `*bool` is the
idiom a Go reader already knows, and the spec's job here is mirroring, not inventing vocabulary.

`internal/schema/modelbuilder.BuildArguments` gains the `*bool` too, and `internal/cli` passes
`nil`/`&trueValue`, which is exactly what it means today.

**T-1 must measure this before it is built.** If a `settings.yaml` saying `dont_generate_pdf: true`
plus an explicit `False` really does render the PDF upstream, this is also a live CLI-side question
worth its own ledger entry. If it does not, the `*bool` is still correct for the library and the
measurement is recorded either way.

### 3.2 Q2 — how "switched off" is returned

Spec §3.2: the generators return `None` on success when the corresponding flag is set. Three
outcomes must stay distinguishable: generated at a path, deliberately not generated, failed.

**Decision:** `(string, error)` with `("", nil)` for "switched off". Rejected alternatives:

- `(*string, error)` — mirrors `Path | None` most literally, but makes the common path
  `*p` with a nil check for a case most callers do not have, and Go's zero string is already a
  perfectly good "no path".
- a `Generated bool` in a result struct — more explicit, but four extra types for five functions.

`GeneratePNG` returns `([]string, error)` and uses `nil` for switched-off, which Go already
distinguishes from an empty non-nil slice; the doc comment says so, and a test asserts it (spec
§6.1).

The risk is that `("", nil)` reads as an oversight. The doc comment on each generator states the
three outcomes explicitly, and `example_test.go` shows the check.

### 3.3 Q3 — aliases or a conversion boundary

**Decision: aliases**, for the error types (§1.2) and for `Model` and `Document`.

The honest tension: spec §5 criterion 1 says `pkg/rendercv` must not import `internal/`, and an alias
*is* an import. The criterion's intent is that no `internal/` type appears in the **exported
signatures** — that a caller never has to name an `internal/` path. An alias satisfies that (callers
write `rendercv.Model`) while an alias declaration is the only place the internal path is mentioned.

**Criterion 1 is therefore restated for `plan.md`, and `spec.md` §5.1 should be amended to match:**
the test asserts that no exported function's parameters or results name a type whose package path
contains `/internal/`, *except* through an alias declared in `pkg/rendercv/types.go`. That is
checkable with `go/types` and is what T-8 builds.

A conversion boundary was rejected: it means maintaining a parallel `Model` that must track every
schema change, and any drift becomes a silent artifact divergence — the exact failure this port has
spent fifteen iterations eliminating.

### 3.4 Q4 — does `Build` return the document?

Upstream returns `tuple[CommentedMap, RenderCVModel]` and the CLI uses element 0. The port's
`BuildDictionary` returns `*BuildResult{Document, OverlaySources}` (`merge.go:40-42`).

**Decision:** return `(*Document, *Model, error)`, where `Document` aliases `*yamldoc.Node` — the
merged document, mirroring the tuple. `OverlaySources` is **not** exposed: it exists so error
coordinates can be resolved against the file a value came from, which is the error pipeline's
business, and it has no upstream counterpart in the returned tuple.

## 4. Signatures

```go
func ReadYAML(content string, source YamlSource) (*Document, error)
func Build(mainYAML string, opts BuildOptions) (*Document, *Model, error)

func GenerateTypst(m *Model) (string, error)
func GeneratePDF(m *Model, typstPath string) (string, error)
func GeneratePNG(m *Model, typstPath string) ([]string, error)
func GenerateMarkdown(m *Model) (string, error)
func GenerateHTML(m *Model, markdownPath string) (string, error)
```

`BuildOptions` mirrors `BuildRendercvModelArguments` field-for-field (spec §3.3), with
`InputFilePath string` for the keyword-only argument upstream declares separately
(`rendercv_model_builder.py:192-210`).

Every exported symbol carries a doc comment naming its upstream construct with a
`src/rendercv/...:LINE` citation — enforced, not reviewed (spec §5.4).

## 5. Divergences this plan creates

None requiring `divergences.md`, and the reasoning is worth stating because the gate is narrow.
`divergences.md` records deviations from **upstream behavior** along the four parity axes. The Go API
surface has no axis (spec §1), so a choice like `*bool` or "no exported `Model` constructor" is not a
divergence — it is a design decision in a space upstream does not constrain.

The one item that would need the gate is a one-call `Render`, already cut in spec §1.2.

If T-1's measurement (§3.1) shows the CLI mishandles an explicit `False`, **that** is a real
behavioral divergence, and it stops for the gate rather than being fixed inside this iteration.

## 6. Risks

1. **The `internal/renderer/generate` extraction (§1.1) is the whole risk of this plan.** It moves
   parity-critical code that 42 golden cases depend on. Mitigation: it lands first, in its own
   commits, with no behavior change and no `pkg/` file present — so the parity suite is a true
   before/after check. If any golden moves, the extraction is wrong and gets reverted, and no library
   work has been wasted.
2. **`pkg/` → `internal/` is legal Go inside one module**, so nothing but a test prevents the facade
   growing a second implementation. T-8 is that test, and it is a gate on the iteration, not a
   nicety — iteration 15's open item 7 is this port's own example of a convention that lived in a
   comment and did not hold.
3. **The doc-comment citation check (T-9) can go vacuous** if it accepts any comment containing a
   colon. It must assert the `src/rendercv/…:LINE` shape and be proved red by a planted symbol, the
   way `kindguard` was.
4. **Semver.** This surface is declared stable by `AGENTS.md` §3 while sitting on `internal/` types
   that are still moving — iteration 11 and 8 both have open items. Aliases mean an `internal/`
   change is a public change. Mitigation is scope, not mechanism: freezing is explicitly out of scope
   (spec §7) and stays a release decision behind the human gate.

## 7. Dependencies

None beyond the standard library and what `internal/` already uses. No new module requirements.

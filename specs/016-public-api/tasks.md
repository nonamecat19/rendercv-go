# Tasks 016 — The public Go API (`pkg/rendercv`)

**Spec:** [`spec.md`](spec.md) · **Plan:** [`plan.md`](plan.md)

Each task is one commit. Every commit leaves `go build ./... && go test ./...` green
(`AGENTS.md` §7). Tests land red before the code that turns them green (§4).

**Ordering is real here, and mostly sequential** — this is a facade over the pipeline spine, so
`AGENTS.md` §5's stop rule applies: **one agent owns T-2 through T-7.** Only T-1 (a measurement) and
T-9/T-10 (independent checks) are safely parallel.

---

## Phase 0 — measure before building

### T-1 · Measure the `dont_generate_*` tri-state
**Parallel-safe. Blocks T-4.** No code.

Determine against the vendored Python whether an explicit `False` differs from an absent key when
`settings.yaml` sets the corresponding `dont_generate_*` to `true`
(`rendercv_model_builder.py:24-39`, plan §3.1). Probe all five flags.

- If they differ: `*bool` is required, and **check whether the CLI mishandles it** — if it does,
  that is a behavioral divergence that is recorded in `specs/divergences.md` (plan §5) rather than being
  fixed here.
- If they do not differ: `*bool` is still built for the library, and the measurement is recorded
  in the commit body so the next reader does not redo it.

**Done when:** the measured table is in `plan.md` §3.1, replacing the prediction.

---

## Phase 1 — the extraction (no `pkg/` file exists yet)

This phase changes no behavior. The parity suite is a true before/after check, which is the whole
reason it comes first (plan §6.1).

### T-2 · Extract Typst and Markdown generation into `internal/renderer/generate`
Move the compose-and-write bodies out of `internal/cli/render.go` (`:210`, `:267`) into
`generate.Typst` / `generate.Markdown`, each mirroring its upstream generator's contract: derive the
path, honor the `dont_generate_*` gate, write, return the path.

**Done when:** `internal/cli` calls the new package, and `go test -tags conformance ./...` is
byte-identical green — 0 FAIL, `TestParity` 42/42.

### T-3 · Extract HTML, PDF and PNG generation
Same, for `render.go:289` (`RenderHTML`), `:373` and `:415` (`typstc.Compile`). PNG keeps its
`{stem}_{i+1}.png` numbering and returns the list (spec §6.1).

**Done when:** as T-2. **If any golden moves in T-2 or T-3, the extraction is wrong — revert it.**

---

## Phase 2 — the surface

### T-4 · `BuildOptions` and the tri-state
`pkg/rendercv/options.go`, mirroring `BuildRendercvModelArguments` field-for-field (spec §3.3), with
`*bool` per plan §3.1 and T-1's measurement. `modelbuilder.BuildArguments` gains the `*bool`;
`internal/cli` passes `nil`/`&true`, which is what it means today.

### T-5 · The type and error aliases
`pkg/rendercv/types.go`: `Model`, `Document`, `YamlSource`, and aliases for `UserError`,
`UserValidationError`, `InternalError`, `ValidationError` (plan §1.2). Aliases, not new types — a
test asserts `errors.As` matches an error raised inside `internal/` (spec §5.5).

### T-6 · `ReadYAML` and `Build`
The two schema entry points (spec §3.1 rows 1–2). `Build` returns `(*Document, *Model, error)` per
plan §3.4. Document that a `Model` comes from `Build` and nowhere else, and that there is no exported
constructor (plan §1.3).

### T-7 · The five generators
Thin re-exports over `internal/renderer/generate`. `("", nil)` / `(nil, nil)` for switched-off, with
the three outcomes stated in each doc comment (plan §3.2).

---

## Phase 3 — the gates

### T-8 · The no-`internal/`-in-signatures check
**Parallel-safe after T-7.** A `go/types` test asserting no exported function's parameters or results
name a type whose package path contains `/internal/`, except through an alias in `types.go`
(spec §5.1 as amended, plan §3.3).

**Must be proved red** by a planted violating signature, then reverted. `kindguard`'s shape.

### T-9 · The doc-comment citation check
**Parallel-safe after T-7.** Every exported symbol's doc comment names its upstream construct with a
`src/rendercv/...:LINE` citation (spec §5.4).

**Must assert the citation shape, not merely a non-empty comment**, and be proved red by a planted
symbol (plan §6.3). A check that accepts any comment containing a colon is vacuous.

### T-10 · The library-path artifact differential
**Parallel-safe after T-7.** Render at least one corpus case by calling `pkg/rendercv` directly and
compare `.typ`/`.md`/`.html` to that case's golden (spec §5.2). This is what makes "the library
produces what the CLI produces" mechanical rather than asserted.

### T-11 · The switched-off and error-distinguishability tests
All five `dont_generate_*` paths asserted as distinct from both success and failure (spec §5.3), and
each of the four error types reachable and distinguishable via `errors.Is`/`errors.As` rather than
string matching (spec §5.5).

### T-12 · Runnable examples
`example_test.go`: build-then-render, and the switched-off check (plan §3.2). These are the API's
documentation and they compile, so they cannot rot silently.

---

## Verification

A fresh-context `rendercv-parity-verifier` after T-12, per `AGENTS.md` §5's diamond — **not** the
agent that wrote any of it. It must specifically re-check:

1. that T-2/T-3 moved no golden;
2. that T-8 and T-9 actually go red on a planted violation, since both are the kind of check this
   port has already shipped vacuous twice (`STATE.md` pass 20);
3. that T-10 compares against a real golden rather than against the library's own output.

`specs/STATE.md` is updated by the merge owner afterwards, never by a porter as part of a feature
commit.

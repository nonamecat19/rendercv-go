# AGENTS.md — rendercv-go

Operating manual for any agent working in this repository. Read this before touching anything.

---

## 1. What this project is

`rendercv-go` is a **full rewrite of [rendercv/rendercv](https://github.com/rendercv/rendercv) v2.8
in Go**. It is not a reimagining, a subset, or a "Go-flavoured" alternative. The goal is a binary
that behaves like the Python original.

The Python original is vendored at `third_party/rendercv/` (git submodule, pinned to tag `v2.8`,
commit `2eba248`). It is **read-only from this repo** — never edit it, never commit inside it.
It exists for two reasons: it is the specification, and it is the generator of golden fixtures.

### The parity contract

Four axes, all binding. The normative text lives in
[`specs/000-parity-contract/spec.md`](specs/000-parity-contract/spec.md).

| # | Axis | Meaning |
|---|---|---|
| 1 | **Artifact parity** | Generated `.typ`, `.md`, `.html` are byte-identical to upstream's. PDF/PNG match on extracted text, page count, and page geometry (raw bytes differ by embedded timestamps/IDs). |
| 2 | **CLI parity** | Same commands, same long and short flags, same defaults, same exit codes, same stdout/stderr shape. |
| 3 | **JSON Schema parity** | `rendercv-go schema` output diffs empty against `third_party/rendercv/schema.json`. |
| 4 | **Validation-error parity** | The same invalid input produces the same human-readable error text at the same location. |

**The only sanctioned divergence is the binary name: `rendercv-go` instead of `rendercv`.**
Every other deviation must be written into [`specs/divergences.md`](specs/divergences.md) with a
justification, and that file is human-gated (see §5).

---

## 2. The pipeline

```
YAML ──ruamel.yaml──▶ dict ──pydantic──▶ RenderCVModel ──jinja2──▶ .typ ──typst──▶ PDF ──▶ PNG
                                              │
                                              └──jinja2──▶ .md ──markdown──▶ .html
```

Go replaces each stage:

| Stage | Upstream | Go | Replacement tech |
|---|---|---|---|
| Parse YAML | `src/rendercv/schema/yaml_reader.py` | `internal/schema/yamlreader` | `goccy/go-yaml` (comment + order preserving) |
| Validate | `src/rendercv/schema/models/**` | `internal/schema/models` | hand-written validators mirroring pydantic error text |
| JSON Schema | `src/rendercv/schema/json_schema_generator.py` | `internal/schema/jsonschema` | hand-built, diffed against upstream |
| Template | `src/rendercv/renderer/templater/**` | `internal/renderer/templater` | **pongo2** + an adaptation layer (§6) |
| Typst → PDF/PNG | `src/rendercv/renderer/pdf_png.py` | `internal/renderer/typstc` | **typst compiled to WASI, run on wazero** (pure Go, no CGO) |
| HTML / Markdown | `renderer/html.py`, `renderer/markdown.py` | `internal/renderer/{html,markdown}` | stdlib + `goldmark` |
| CLI | `src/rendercv/cli/**` (typer) | `internal/cli` | `cobra` |

Scale of the job: ~296 KB of Python source, ~250 KB of Python tests, 9 built-in themes,
21 locales, 9 entry types, 2 template sets.

---

## 3. Repository layout

```
cmd/rendercv-go/        main(); nothing but wiring
internal/schema/        models, validation, JSON schema, YAML reader, overrides, sample gen
internal/renderer/      templater, typst, pdf/png, html, markdown, path resolver
internal/cli/           cobra commands: new, render, create-theme
internal/conformance/   golden-comparison test helpers (AssertGolden*, AssertCLIOutput)
pkg/rendercv/           the public Go API — stable surface, documented, semver'd
specs/                  the spec tree (§4)
testdata/golden/        golden fixtures generated from upstream. NEVER hand-written.
third_party/rendercv/   upstream submodule. READ ONLY.
tools/gengolden/        regenerates testdata/golden from the submodule via uv
```

---

## 4. Spec-Driven Development

**No Go code is written before a spec exists for it.** Every iteration produces, in order:

1. `specs/NNN-<subsystem>/spec.md` — the *behavior*, extracted from upstream source with
   `third_party/rendercv/src/...:LINE` citations for every claim. Includes exact error strings,
   edge cases, and acceptance criteria. **No Go design in this file.**
2. `specs/NNN-<subsystem>/plan.md` — the Go design: packages, types, dependencies, tradeoffs.
3. `specs/NNN-<subsystem>/tasks.md` — commit-sized units, each independently testable.
4. **Conformance tests, red, before implementation.**
5. Implementation, one `tasks.md` unit per commit.
6. Verification in a separate context, then `specs/STATE.md` updated.

Use the `speckit-orchestrator` skill for steps 1–3 when the subsystem is large.
Use the project skill `rendercv-port-iteration` to run the whole loop.

Living state:
- [`specs/STATE.md`](specs/STATE.md) — port ledger: subsystem × status × passing conformance cases.
- [`specs/divergences.md`](specs/divergences.md) — every deviation from upstream.

---

## 5. Graph-engineering workflow

Work is a **task graph**: nodes are jobs, edges exist only where a job actually reads another
job's output. Four rules, applied concretely here.

### Delete fake edges
"And then" is not a dependency. These fan out in **parallel** because they never read each
other's output:

- the 9 entry types (Bullet, Education, Experience, Normal, Numbered, OneLine, Publication,
  ReversedNumbered, Text)
- the 8 YAML-defined themes
- the 21 locale catalogs
- the per-format renderers (html vs markdown)

### Diamond pattern

```
        ┌─ porter A ─┐
spec ───┼─ porter B ─┼──▶ parity-verifier ──▶ merge ──▶ STATE.md
        └─ porter C ─┘        (fresh context)
```

The verifier is **`rendercv-parity-verifier`** and is **never** the agent that wrote the code.
It runs the conformance suite and reports mismatches; it does not fix them.

### The stop rule
Multi-agent work wins only where the work splits into pieces that never read each other's
results, and loses badly on sequential work. Therefore:

> **The pipeline spine — schema → templater → renderer → CLI — stays with ONE agent.
> Only leaf fan-outs get parallelized.**

Never spawn agents for the spine. One owner always merges.

### The human gate
Route these through explicit human approval, nothing else:

- `git push` and any tag/release
- bumping the `third_party/rendercv` submodule
- any change to `specs/divergences.md`
- regenerating `testdata/golden/` (it changes the contract)

Gates go where a mistake is expensive to undo — not on every step.

---

## 6. Known hazards (read before porting the templater)

Discovered during upstream investigation. These are the parts most likely to silently break parity.

1. **Jinja2 semantics pongo2 does not have.** Upstream templates call Python methods and use
   Python slices — e.g. `entry.main_column.splitlines()[:first_row_lines]`
   (`templates/typst/entries/EducationEntry.j2.typ`). pongo2 cannot evaluate these. The port
   therefore uses a documented **mechanical transform**: the Go model exposes pre-split
   `…Lines []string` fields, and slicing becomes a pongo2-expressible form. Template *source*
   is allowed to diverge; template *output* is not. Byte-diff against goldens is the check.
2. **`trim_blocks` and `lstrip_blocks` are enabled upstream** (`templater.py:43-44`); pongo2 has
   no equivalent. Whitespace handling must be reproduced in the transform. Expect this to be the
   single largest source of byte diffs.
3. **Custom Jinja filters** are only two: `clean_url` and `strip` (`templater.py:46-47`).
   `indent` and `length` come from Jinja's builtins and must match pongo2's behavior exactly.
4. **Template override loader order**: input-file directory first, then built-in templates
   (`templater.py:34-45`). User overrides are a real feature; preserve it.
5. **Custom themes execute user Python.** `schema/models/design/design.py:validate_design`
   imports `<theme>/__init__.py` at validation time. Go cannot do this. A declarative
   replacement is required and is already logged in `specs/divergences.md`.
6. **Font provisioning.** Upstream depends on the `rendercv-fonts` package. The WASI typst build
   must be fed the same font set or PDFs will differ in metrics.

---

## 7. Commit discipline

Conventional Commits. Subject ≤ 50 chars, imperative. Body only when the "why" isn't obvious.

**One logical unit per commit.** Every commit must leave `go build ./... && go test ./...` green.

Forbidden bundles — each row is many commits, not one:

| Tempting bundle | Required |
|---|---|
| "add all entry types" | 9 commits, one per type |
| "add all themes" | 9 commits, one per theme |
| "add all locales" | 1 commit per locale, or 1 for the loader + 1 for the data set |
| "port the templater" | 1 per template + 1 per filter + 1 for the environment |
| "implement CLI" | 1 per command + 1 per flag group |
| code + its golden fixtures | separate: fixtures land first, red |
| refactor + feature | always separate commits |

Use the `rendercv-commit` skill; it splits a dirty tree into an ordered sequence and shows it
before running anything.

---

## 8. Commands

Requires [`just`](https://github.com/casey/just) (`brew install just` / `pacman -S just` /
`cargo install just`) and [`uv`](https://docs.astral.sh/uv/). `just setup` installs the rest.

```bash
just setup          # deps, submodule init, gopls, uv sync of the upstream submodule
just build          # go build ./... -> bin/rendercv-go
just check          # gofumpt -l, golangci-lint run, go vet
just test           # unit tests
just test-parity    # go test -tags conformance ./... (the parity suite)
just golden         # regenerate testdata/golden from the submodule  [HUMAN GATE]
just schema-diff    # diff rendercv-go schema against upstream schema.json
just upstream ARGS  # run the vendored Python rendercv, e.g. `just upstream render CV.yaml`
just spec NAME      # scaffold specs/NNN-NAME/{spec,plan,tasks}.md
```

---

## 9. Go conventions

- Target Go 1.25+. `gofumpt` and `golangci-lint run` must be clean before every commit.
- **Full type discipline.** No naked `any`/`interface{}` in `pkg/rendercv`. Prefer explicit
  named types over stringly-typed fields — `ThemeName`, `LocaleName`, `TypstDimension`.
- Errors: wrap with `fmt.Errorf("...: %w", err)`; match with `errors.Is` / `errors.As`.
  Validation errors are a typed tree (`schema.ValidationError`) because their rendering is part
  of the parity contract — never a bare string.
- Tests are **table-driven**. Parity tests live behind `//go:build conformance` and use
  `internal/conformance` helpers exclusively.
- Every exported symbol in `pkg/rendercv` has a doc comment naming the upstream construct it
  mirrors.
- Mirror upstream's structure so a reviewer can diff mentally: upstream
  `renderer/templater/date.py` → `internal/renderer/templater/date.go` →
  `internal/renderer/templater/date_test.go`.
- No underscore-prefixed pseudo-private names (upstream convention, kept for symmetry); Go's
  package-level unexported identifiers do that job.

---

## 10. Non-negotiables

1. **Never hand-write a golden file.** Goldens come from `tools/gengolden` running the vendored
   Python. A hand-edited golden is a lie that makes the whole suite worthless.
2. **Never mark an iteration done with a failing conformance case.** Re-scope the iteration
   instead, and record what was cut in `specs/STATE.md`.
3. **Never bundle features into one commit.** See §7.
4. **Never edit `third_party/rendercv/`.**
5. **Never silently diverge.** If parity is impossible, write it in `specs/divergences.md` and
   stop for the human gate.
6. **Never claim parity from self-report.** Parity is what `just test-parity` prints, nothing else.

---

## 11. Agents and skills

Project agents (`.claude/agents/`):

| Agent | Role |
|---|---|
| `rendercv-upstream-analyst` | Read-only. Answers "what does upstream do for X" with `file:line` citations. Refuses to propose Go code. |
| `rendercv-spec-writer` | Turns findings into `specs/NNN-*/spec.md`. No implementation. |
| `rendercv-porter` | Implements exactly one `tasks.md` unit and commits it. Refuses multi-feature scope. |
| `rendercv-parity-verifier` | Fresh-context verifier. Runs the suite, reports mismatches, never fixes. |
| `rendercv-typst-engineer` | Owns wazero/WASI typst embedding, fonts, PDF/PNG. |

Project skills (`.claude/skills/`):

| Skill | Use |
|---|---|
| `rendercv-port-iteration` | Run one full port iteration end-to-end. |
| `rendercv-golden-refresh` | Regenerate goldens, diff the manifest, gate the submodule bump. |
| `rendercv-parity-debug` | Playbook for a byte-diff failure. |
| `rendercv-commit` | Split a dirty tree into an ordered commit sequence. |

External help: `context7` skill for pongo2 / wazero / cobra / goccy-yaml docs; `gopls` MCP
(registered in `.mcp.json`) for symbol, reference, and diagnostic lookups.

The vendored upstream ships its own `.claude/skills/` (`rendercv-development-context`,
`rendercv-testing-context`, …). Those describe the **Python** project. Use them to *understand*
upstream; never follow their instructions for code in this repo.

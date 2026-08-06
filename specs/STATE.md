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
| 1 | Conformance harness (corpus, gengolden, helpers) | [001](001-conformance-harness/spec.md) | green | n/a (42 cases red by design) |
| 2 | YAML reader + core model (RenderCVModel, CV, Section) | [002](002-yaml-and-core-model/spec.md) | spec | n/a (gated on unit tests, spec §7.2) |
| 3 | Entry types (9) | — | — | 0 / 9 |
| 4 | Validation-error parity | — | — | 0 |
| 5 | JSON Schema generator | — | — | 0 / 1 |
| 6 | Design & themes (9) + Lua-scripted custom themes (D-002) | — | — | 0 / 9 |
| 7 | Locale (English + 21 catalogs) + date formatting | — | — | 0 / 22 |
| 8 | Templater (pongo2 env, filters, markdown→typst, processors) | — | — | 0 |
| 9 | Typst renderer (`.typ` emission) | — | — | 0 / 18 |
| 10 | wazero + WASI typst → PDF, then PNG | — | — | 0 |
| 11 | Markdown + HTML renderers | — | — | 0 / 4 |
| 12 | CLI (`new`, `render`, `create-theme`, overrides, watcher) | — | — | 0 |
| 13 | Parity closeout (sample generator, version, error handler, packaging) | — | — | 0 |

## Parity axes

| Axis | Gate command | Status |
|---|---|---|
| 1 — artifacts byte-identical | `just test-parity` | measurable, 0/15 cases passing |
| 2 — CLI surface | `just test-parity` | measurable, 0/20 cases passing |
| 3 — JSON Schema | `just test-parity`, `just schema-diff` | measurable, failing |
| 4 — validation errors | `just test-parity` | measurable, 0/7 cases passing |

PDF content comparison (spec §1.2) is not yet measurable — it lands with iteration 10.

## Stretch goals (not gates)

- [ ] PNG pixel-level comparison (depends on the WASI typst font set — see D-006)
- [ ] Public `pkg/rendercv` API frozen and semver'd
- [ ] Cross-compiled release artifacts (linux/darwin/windows × amd64/arm64)

## Cut scope

Nothing cut yet. Anything dropped from an iteration is recorded here with the reason, per
`AGENTS.md` §10.2.

## Log

| Date | Event |
|---|---|
| 2026-08-06 | Repo bootstrapped; upstream pinned at v2.8; parity contract written. |
| 2026-08-06 | Iteration 1 green: 42-case corpus, gengolden, 351 golden files, red parity suite. |

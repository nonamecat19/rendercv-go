# Changelog

All notable changes to this project are documented here. Versioning follows
[Semantic Versioning](https://semver.org/); the versioned surface is
`pkg/rendercv`, the public Go API (`AGENTS.md` §3, `specs/016-public-api/spec.md`).
Everything under `internal/` is not covered and may change without notice.

## [1.0.0] — 2026-08-14

First frozen release of `pkg/rendercv`. All 17 rows of the port ledger
(`specs/STATE.md`) are green: schema, templater, Typst→PDF/PNG compilation,
Markdown/HTML rendering, CLI, locales, themes (including Lua-scripted custom
themes), explicit YAML tags, and this public API facade itself. The parity
suite (`just test-parity`) passes 42/42, `just schema-diff` is empty.

### Added

- `pkg/rendercv`: `ReadYAML`, `Build`, `GenerateTypst`, `GenerateMarkdown`,
  `GenerateHTML`, `GeneratePDF`, `GeneratePNG` — mirroring the seven functions
  upstream's `run_rendercv` orchestrator calls
  (`third_party/rendercv/src/rendercv/cli/render_command/run_rendercv.py:127-198`).
- `Model`, `BuildOptions`, `Document`, `YamlSource` and the four error types
  (`UserError`, `UserValidationError`, `InternalError`, `ValidationError`),
  aliased from `internal/` so no exported signature names an internal path.
- `rendercv-go` CLI binary: `new`, `render`, `create-theme`, matching
  upstream's `rendercv` command surface (flags, defaults, exit codes).

### Stability guarantee

- Every exported symbol's signature and documented behavior is covered by
  semver from this release forward.
- A breaking change to any of the above requires a `2.0.0`.
- Artifact/CLI/JSON-Schema/validation-error parity with upstream RenderCV
  v2.8 (`specs/000-parity-contract/spec.md`) is maintained across patch and
  minor releases; a deliberate divergence is recorded in
  `specs/divergences.md` before it ships.

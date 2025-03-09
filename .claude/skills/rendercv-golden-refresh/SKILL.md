---
name: rendercv-golden-refresh
description: Regenerate testdata/golden from the vendored Python RenderCV, review the manifest diff, and gate the change. Use when adding corpus cases, when goldens are missing or stale, or when bumping the third_party/rendercv submodule. Never use it to make a failing test pass.
---

# Golden refresh

Goldens are the parity contract in file form. Regenerating them **changes the contract**, so this
is a human-gated operation (`AGENTS.md` §5).

## The one rule that matters

> A failing conformance test is never fixed by regenerating goldens.

If Go output differs from a golden, Go is wrong until proven otherwise. Regenerate only when the
*corpus* changes (new cases) or the *upstream pin* changes (submodule bump). Use
`rendercv-parity-debug` for failures.

## Preconditions

```bash
git status --porcelain          # must be clean
git -C third_party/rendercv describe --tags   # must be the pinned tag
```

## Procedure

### 1. Declare intent
State which of these this is, before running anything:

- **A — new corpus cases.** Existing goldens must not change. Any change to an existing file is a
  bug in the generator.
- **B — upstream submodule bump.** Existing goldens are expected to change. This needs explicit
  human approval *and* a `specs/STATE.md` log entry.
- **C — generator fix.** Existing goldens may change; every change must be explainable line by
  line before it is accepted.

### 2. Generate

```bash
just golden
```

`tools/gengolden` runs the vendored Python via `uv` over every corpus case and writes
`testdata/golden/<case>/…` plus `testdata/golden/manifest.json` (upstream SHA + sha256 per file).

### 3. Review the diff — this is the actual work

```bash
git status --short -- testdata/golden/
git diff --stat -- testdata/golden/
git diff -- testdata/golden/manifest.json
```

For intent **A**, `git diff --stat` must show only additions. If an existing golden changed, stop
and investigate the generator.

For **B** and **C**, walk every changed file:

```bash
git diff --word-diff=porcelain -- testdata/golden/<case>/<file>
git diff -- testdata/golden/<case>/<file> | cat -A | head -40   # whitespace visible
```

Every hunk needs a one-line explanation. "Upstream changed" is not an explanation — name what
changed upstream and cite it.

### 4. Verify the manifest
- `upstream_sha` matches `git -C third_party/rendercv rev-parse HEAD`
- every file present on disk has an entry, and vice versa
- sha256 values recompute

### 5. Present for the gate
Show the human:

- intent (A/B/C)
- file counts: added / changed / removed
- the per-file explanation for every change
- the manifest's before/after `upstream_sha`

Wait for approval. Do not commit before it.

### 6. Commit
Goldens commit **separately** from code, always:

```
test: regenerate goldens for <reason>

upstream: <sha> (<tag>)
added: N  changed: N  removed: N
<one line per changed file>
```

If this was a submodule bump, the submodule bump is its own commit, landing first.

### 7. Re-run the suite

```bash
just test-parity
```

A refresh that turns a previously-passing case red is a regression — report it, do not absorb it.

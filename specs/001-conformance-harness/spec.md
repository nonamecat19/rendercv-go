# Spec 001 — Conformance harness

**Status:** implemented · **Inherits:** [`specs/000-parity-contract/spec.md`](../000-parity-contract/spec.md)

## 1. Purpose

Make the parity contract executable. Nothing in this iteration ports RenderCV; it builds the
instrument that decides whether later iterations succeeded. Until this exists, "identical to
upstream" is an opinion.

## 2. Inputs / outputs

**In:** `testdata/corpus.json` (case declarations) and `third_party/rendercv` (the pinned Python
implementation, which supplies both the inputs and the reference behavior).

**Out:** `testdata/golden/<case>/` per case:

| File | Contents |
|---|---|
| `files/**` | every file the run created, byte-exact, PNG bytes excluded |
| `files.txt` | the created-file list, sorted — output layout is contractual (§1.3) |
| `pngs.txt` | `<path> <W>x<H>` per PNG |
| `stdout.txt`, `stderr.txt` | normalized streams |
| `exit_code.txt` | the exit code |
| `case.json` | name, axis, argv |

Plus `testdata/golden/manifest.json`: upstream SHA, upstream tag, case count, sha256 per file.

## 3. Behavior

1. Each case runs in a directory containing only its declared inputs, so "what did this run
   create" is a set difference, not a guess.
2. The child environment is fixed: `COLUMNS=80`, `NO_COLOR=1`, `TERM=dumb`, `LC_ALL=C.UTF-8`,
   `PYTHONHASHSEED=0`, `PYTHONUTF8=1`. Rich lays its panels out to terminal width, so unpinned
   width would make every CLI golden machine-dependent.
3. Exactly one normalization is applied to captured streams: wall-clock timings, together with
   the padding that follows them, collapse to `<duration> `. The padding must be absorbed —
   the column is right-padded to a fixed width, so a shorter timing means more spaces.
   Everything else, including box drawing and column alignment, is contractual.
4. Generation is atomic: a scratch tree is built first and promoted only on success, so a crash
   cannot leave the contract half-rewritten.
5. `-verify` regenerates into scratch and compares hashes against the committed manifest without
   writing. It reports three distinct problems: the upstream pin moved, a golden differs, a
   committed golden is no longer generated.
6. Generation refuses to run if `third_party/rendercv` has local modifications.
7. PNG bytes are not stored. They were ~90% of the fixture size and PNG parity is specified on
   page count and pixel dimensions (§1.2), which `pngs.txt` captures.
8. PDF bytes **are** stored — upstream renders them reproducibly — but are excluded from
   byte comparison, because the Go build uses a different Typst binary. PDF content comparison
   is iteration 10's.

## 4. Exact strings

None user-visible; this is developer tooling.

## 5. Edge cases

- A case whose expected exit code does not match aborts generation rather than recording the
  surprise as the contract.
- Duplicate case names are rejected at load.
- A missing golden is a test failure, never a skip.

## 6. Ordering and whitespace guarantees

`files.txt` is sorted. Manifest keys are JSON-marshalled and therefore sorted. Captured streams
end in exactly one newline.

## 7. Out of scope

- PDF text/geometry comparison → iteration 10.
- Pixel-level PNG comparison → stretch goal, `specs/STATE.md`.
- `--watch` behavior → iteration 12.
- Custom-theme cases → blocked on D-002, iteration 6.

## 8. Acceptance criteria

- [x] `go run ./tools/gengolden` regenerates all cases from the pinned upstream.
- [x] `go run ./tools/gengolden -verify` passes on a clean tree — i.e. generation is deterministic.
- [x] `go test ./...` green: corpus well-formed, contract coverage, goldens present, manifest
      consistent, `Normalize` pinned by table test.
- [x] `go test -tags conformance ./...` red for all 42 cases plus the schema case, each failing
      with a diff rather than an infrastructure error.
- [x] Corpus covers 9 themes, an RTL locale, and ≥5 invalid-input cases (contract §7).

## 9. Corpus additions

The initial 42 cases. Later iterations add to `testdata/corpus.json` and regenerate.

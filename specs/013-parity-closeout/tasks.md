# Tasks 013 — Parity closeout

Commit-sized units. Each leaves `go build ./... && go test ./...` green. Ordering constraints are
stated per task; everything not named is parallel.

Numbers in brackets cite [`spec.md`](spec.md) behaviors and [`plan.md`](plan.md) sections.

---

## T1 — the watcher [spec §3.7 b45–49, plan §3]

**Owns:** `internal/cli/watcher.go` (new), `internal/cli/render.go`, `internal/cli/watch_test.go`.
**Blocks:** T2.

- `WatchSet(options, resolved) []string` — the input file always, plus `--design`,
  `--locale-catalog`, `--settings` when given, plus `settings.render_command.design` and `.locale`
  resolved relative to the input file's parent when not given. CLI flags win. Absolute paths.
- `watchLoop(ctx, set, render)` — render once, then re-render on a modify event for a member of the
  set; swallow every render failure; return only on ctx cancellation.
- `Render` dispatches: `options.Watch` → `watchLoop`, else today's body (extracted as `renderOnce`).
- Delete `errWatchNotImplemented` and `TestRenderRejectsWatch`.

**Red first.** `watch_test.go` must contain, failing before the change: a `WatchSet` table over the
five sources of b47; a `watchLoop` test whose stub render returns a failure and which asserts the
loop is still running after a bounded wait (b48); and a test that the first render's bytes equal
`renderOnce`'s for the same options (§8).

## T2 — `read_yaml`'s two messages are unreachable [spec §3.6 b43–44, §4.11, plan §4]

**Owns:** `internal/cli/render.go`. **After:** T1.

- No branch anywhere in the port may reject an input on its file extension; `render ok.txt` over a
  valid CV renders at exit 0.
- Replace `errMissingFile`'s text. Neither `The file {file_path} doesn't exist!` nor
  `The input file should have one of the following extensions: …` may appear in the source.
- Call sites, exit code (1), stream and path (A) are unchanged.

**Red first.** A test rendering a valid CV named `.txt` and asserting exit 0; a source-level test
asserting neither §4.11 string occurs in the package.

## T3 — no exit code but 0, 1 and 2 [spec §3.5 b42, §6.5, plan §5]

**Owns:** `internal/cli/root.go`, `internal/cli/exitcode_test.go`.

- `execute`'s `code := 70` initialiser and its `return 70` become exit 1 — an unclassified failure
  is upstream's unhandled exception, which is 1.
- Inventory test: the set of codes the package can return is exactly {0, 1, 2}, and 70 is
  unreachable.

**Red first.** The inventory test fails while the sentinel exists.

## T4 — the packaging inventories [spec §3.8 b22, b54–56, plan §6]

**Owns:** `internal/cli/packaging_test.go` (new). Test-only; no production file changes.

- Exactly three registered commands, named `render`, `new`, `create-theme`.
- Embedded data-file counts: 21 locales, 8 themes, 13 typst templates, 12 markdown templates,
  1 html template, `sample_content.yaml`, `error_dictionary.yaml`.
- The Typst package name and version the emitter writes equals the vendored
  `renderer/rendercv_typst/typst.toml`'s `name` and `version`.
- Every file `new --create-typst-templates` writes is user-writable.

## T5 — the error-handler differential gate [spec §3.4 b31, §8, plan §7]

**Owns:** a new test-only package. Does not touch `internal/conformance`.

- Runs the vendored `third_party/rendercv/.venv/bin/rendercv` and `bin/rendercv-go` on the same
  input, in two scratch directories **outside the repository**, and compares raw stdout, raw
  stderr, exit code and file tree with **no** trailing-newline normalization.
- Covers b31's seven rows; asserts byte count, last byte, stream and exit code for each.
- `//go:build conformance`; skips when the venv or the binary is absent.
- The two fallback strings of b38 stay unreachable: a test enumerates the port's
  `RenderCVUserError` equivalents and asserts each carries a non-empty message.

## T6 — merge, verify, ledger

**Owner: the merge agent only.**

- Run `just check`, `go test ./...`, then `just test-parity` **alone**, nothing else touching the
  tree (STATE pass 22's operational rule).
- `rendercv-parity-verifier` in a fresh context.
- Update `specs/STATE.md`.

---

## Not in this iteration

P-1…P-4 (spec §10) are human-gated divergence proposals; no task depends on one. §7.3's
email-validator surface is declined here and recommended as its own iteration. §7.4's golden
trailing-newline blindness and §7.6's `caseWorkDir` fragility both need a golden regeneration and
stay gated.

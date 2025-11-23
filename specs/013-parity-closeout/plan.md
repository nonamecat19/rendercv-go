# Plan 013 — Parity closeout

Go design for [`spec.md`](spec.md). No behavior here that the spec does not cite.

---

## 1. What is already built

Four of the spec's seven subjects landed ahead of this plan, as ad-hoc fixes during iterations 12
and 14–15. They are listed so the plan does not re-open them:

| Spec subject | State | Where |
|---|---|---|
| Sample generator (§3.1) | built | `internal/cli/sample/`, `sample.go`, `scalar.go`, `blocks/` |
| Version (§3.3) | built | `internal/version`, one constant, three sites |
| Path A / path B panel split (§3.4 b28–33, §6.2) | built | `internal/cli/panel.go`, `panelnewline_test.go` |
| `--quiet` silences path B only (b36) | built | `render.go:113` |
| `Rendering...` empty body (b39) | built | `render.go:314` |

What is left is: the watcher, two reachability corrections, an exit-code invariant, and the three
mechanical inventories §8 asks for.

## 2. Packages touched

| Unit | Package | Files |
|---|---|---|
| T1 watcher | `internal/cli` | new `watcher.go`; `render.go`; `watch_test.go` |
| T2 reachability | `internal/cli` | `render.go` |
| T3 exit codes | `internal/cli` | `root.go`; `exitcode_test.go` |
| T4 packaging | `internal/cli` | new `packaging_test.go` |
| T5 error-handler gate | `internal/clidiff` (new, test-only) | new differential harness |

No new module dependency. The watcher uses `fsnotify` only if a poll loop cannot satisfy §3.7; see
§3.

## 3. The watcher

Upstream is watchdog scheduling **parent directories, non-recursively** (`watcher.py:49-58`) and
filtering `on_modified` by an absolute-path set (`:24-31`). The observable contract is narrower
than the mechanism:

- first render happens before the loop and its bytes equal a non-`--watch` render (§8);
- a failing render neither exits nor sets a code (b48);
- the loop never terminates except on interrupt (b49);
- the watched set is `collect_input_file_paths`' values (b47).

Only the last is unit-testable without a process. Design accordingly:

```go
// WatchSet returns the absolute paths render watches, in upstream's order.
func WatchSet(options RenderOptions, resolved overlayPaths) []string

// watchLoop renders once, then re-renders on a modify event for a path in set.
// It returns only on ctx cancellation; render failures are swallowed.
func watchLoop(ctx context.Context, set []string, render func() int) error
```

`WatchSet` is pure and is where b47 is pinned. `watchLoop` takes the render as a closure so the
test can drive it with a stub and assert "did not return after a failing render". The observer is
`github.com/fsnotify/fsnotify` — watchdog's non-recursive directory watch is exactly fsnotify's
`Add(dir)`, and the filter stays ours. A 1 s poll fallback is **not** used: upstream's `time.sleep(1)`
is the idle loop, not the detection mechanism, and a poller would re-render on mtime granularity
upstream does not have.

`Render` splits: `renderOnce(options, …) int` is today's body, `Render` dispatches on `options.Watch`.
`errWatchNotImplemented` is deleted.

## 4. Reachability (T2)

Behavior 43 measured that `read_yaml`'s two file-level messages cannot be reached from the CLI: the
render path passes a **string**, so neither the existence check nor the extension whitelist runs.
Consequences for the port:

- `render ok.txt` over a valid CV must render, exit 0. Nothing in the port may branch on extension.
- A missing input file is upstream's traceback (b40), already D-011's `err_missing_file`. The port
  keeps reporting a clean error at the same exit code and stream, but must not do it with
  upstream's `read_yaml` wording, which no user can ever see upstream.

So `errMissingFile`'s call sites are unchanged — both the input read and `overlayFile` keep failing,
at exit 1, on stdout, through path A. §8's criterion is textual: neither §4.11 string appears in the
source.

**Landed differently from this plan's first draft, and the difference is worth recording.** The
draft assumed `internal/cli.errMissingFile` carried one of §4.11's strings. It does not — its text is
`The file %s does not exist!`, which is nearer `schema/models/path.py:47` than either §4.11 message,
and it covers a vector that is D-011's traceback upstream. The two strings were in
`internal/schema/yamlreader.ReadFile`, together with a live existence check and extension whitelist
that the CLI never reached but that every direct caller did. That is what T2 removed
(`4b981bb`); `internal/cli/render.go` was not touched, which also kept T2 clear of T1.

The removal contradicts spec 002 §4.1, §4.2 and §5.1, which asserted all three behaviors. Those
sections pinned a divergence and their tests are gone; **spec 002 must be corrected** — T6's ledger
entry records it.

## 5. Exit codes (T3)

`execute` initialises `code := 70` and returns `70` on an unclassified `root.Execute` error. §6.5
says 0, 1, 2 and nothing else. The sentinel becomes `exitInternalError = 1` — an unclassified
failure is an unhandled exception upstream, which is exit 1 (b40). `exitcode_test.go` grows an
inventory test that walks every `return`/assignment of a code in the package and asserts the set.

## 6. Inventories (T4)

Three assertions, all cheap and all currently unheld:

1. exactly three registered cobra commands, named `render`, `new`, `create-theme` (b56);
2. the embedded data-file counts of b54, read off the `embed.FS`es the port already has;
3. the Typst package name and version the emitter writes agrees with the vendored
   `renderer/rendercv_typst/typst.toml` (b55).

Plus b22: every file `new --create-typst-templates` writes is user-writable.

## 7. The gate (T5)

§8 says "differential, no trailing-newline normalization". `internal/conformance.Normalize` appends
`\n` to both sides and is human-gated (§7.4), so the gate cannot reuse it. A separate test-only
package runs both binaries in scratch directories **outside the repository** (§7.6) and compares raw
stdout, raw stderr, exit code and file tree. It is `//go:build conformance` and skips when the
vendored venv is absent.

## 8. Sequencing

T1 and T2 both touch `render.go` and are ordered T1 → T2. T3, T4, T5 are independent of both and of
each other.

## 9. Out of this plan

The four divergence proposals P-1…P-4 (spec §10) are human-gated. No code that depends on one lands
here: T2 stops at deleting an unreachable message, and does not touch D-011's traceback surface,
`create-theme`'s stream inversion, `OS Error:`'s body or the template-syntax text.

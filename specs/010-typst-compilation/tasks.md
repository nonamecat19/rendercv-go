# Iteration 10 — tasks

One task = one commit. Every task leaves `go build ./... && go test ./...` green.

`[sequential]` everywhere below except T7 and T8: this is the renderer spine, and `AGENTS.md` §5's
stop rule keeps the spine with one owner. Nothing here fans out to porters.

---

## Blocked on the divergence entry

**T2, T3 and T6 cannot land until the distribution question in `specs/STATE.md` is answered.**
They put a 29 MB `.wasm` and 59 MB of font files into the repository, which changes what a user
installs — `spec.md` §5 count 3 says that needs a `divergences.md` entry. T1 and T4 are unblocked because they add only source.

---

## T0 — record the measurements  `[sequential]` ✅ done

`plan.md` §3b and §3c, and `spec.md` §2.6 / §2.6b. The two spec corrections are the substance:
`fontawesome:0.6.0` is downloaded rather than vendored, and typst's embedded fonts are a third
input that `rendercv_fonts` does not cover.

## T1 — the WASI shim's source  `[sequential]`

`tools/typstwasm/` — a Rust crate implementing `typst::World` over WASI preopens, mirroring
`get_typst_compiler` (`pdf_png.py:154-186`): a root, a list of font folders, a package path.

- `--root` / `--pkg` / `--font-dir` (repeatable) / `--in` / `--out` / `--format pdf|png` /
  `--ppi` / `--today`.
- Font order: folder fonts first, `typst_assets::fonts()` last, so a folder font wins a name tie —
  typst-cli's `FontSearcher` order.
- PNG pages are written `<out>_<n>.png`, one-based, matching `pdf_png.py:86-89`.
- `--ppi` defaults to **144**, which is `typst-py`'s default and what upstream gets by passing
  nothing (`typst/__init__.pyi:119`).

Source only. The build artifact is T2. Includes a `justfile` recipe to build it reproducibly.

## T2 — vendor the built `.wasm`  `[sequential]`  **GATED**

`internal/renderer/typstc/typst.wasm`, 29 MB, plus the `//go:embed`. Pinned to a recorded
`cargo build` invocation and a `typst 0.14.2` lockfile so it is reproducible rather than a blob
someone once produced.

## T3 — vendor the fonts and the two Typst packages  `[sequential]`  **GATED**

- `rendercv_fonts`' 15 folders / 62 font files, 59 MB.
- `preview/rendercv/0.3.0/{typst.toml,lib.typ}` from the submodule.
- `preview/fontawesome/0.6.0/`, 7 files, 428 KB — **not in the submodule**; it is what upstream
  downloads. Vendoring it is the divergence: the port must not fetch from the network at render
  time.

## T4 — the wazero runner  `[sequential]`

`internal/renderer/typstc`: instantiate the module, mount `/work`, `/pkg`, `/fonts`, pass argv,
map a non-zero exit to a typed error carrying the compiler's stderr. No CGO. Unit-tested against
a two-line `.typ` that needs no fonts.

## T5 — red conformance test  `[sequential]`

`internal/conformance`: `AssertGoldenPDF` — extracted text, page count, page geometry, per
`spec.md` §4 and the parity contract's axis 1. Lands **before** T6, failing for the right reason.

The 14 cases and the three-way mutation check are already specified by `plan.md` §3c; the test
codifies that sweep rather than inventing a new oracle.

## T6 — wire `generate_pdf` into the render pipeline  `[sequential]`  **GATED**

`pdf_png.py:16-59`: honour `dont_generate_pdf`, resolve `pdf_path`, copy the photo next to the
`.typ` first (`:93-111`) because the source refers to it by base name.

## T7 — PNG generation  `[parallel]` with T8

`pdf_png.py:44-91`: delete existing `<stem>_*.png` first, render at 144 ppi, write
`<stem>_<n>.png` one-based.

## T8 — PNG comparison  `[parallel]` with T7

Names and dimensions against the goldens. Pixel-level comparison stays a stretch goal
(`STATE.md`), not a gate.

## T9 — ledger  `[sequential]`

`STATE.md`: status, case count, and axis 1's PDF row, which becomes measurable for the first time.
Only after a fresh-context `rendercv-parity-verifier` pass — never from this context's self-report
(`AGENTS.md` §10.6).

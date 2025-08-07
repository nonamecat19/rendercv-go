# Iteration 10 — plan

**This file reports measurements, not a design.** `spec.md` §5 says the first task of this
iteration is to count three things before a line of Go is written, because two iterations in a row
had an estimate stated as a conclusion. Here are the counts and the commands that produced them.

---

## 1. What upstream actually compiles with

| Fact | Value | Measured by |
|---|---|---|
| `typst-py` version | **0.14.8** | `importlib.metadata.version("typst")` |
| Typst compiler the package declares | **0.14.0** (`compiler = "0.14.0"`) | `rendercv_typst/typst.toml` |
| Compiled extension size | **64.8 MB** (`_typst.abi3.so`) | `stat` on the installed wheel |
| Font folders | **15** | `rendercv_fonts.paths_to_font_folders` |
| Font files | **77** | `rglob` over those folders |
| Local Rust toolchain | cargo 1.95.0, host target only | `cargo --version`, `rustup target list --installed` |

## 2. What each count means for the decision

**The version is pinned twice and the two agree.** `typst.toml` declares compiler 0.14.0 and the
installed `typst-py` is 0.14.8 — the same minor line. A Go port must build **that** line;
`spec.md` §5 count 1 is answered in the sense that matters: there is a specific version to target,
and it is current rather than archaeological.

**64.8 MB is the number that decides distribution.** That is a native, dynamically-linked
extension; a `wasm32-wasip1` build is a different size, but the order of magnitude is the point.
Embedding tens of megabytes of WASM in `rendercv-go` changes what a user installs, which
`spec.md` §5 count 3 flags as needing a `divergences.md` entry — human-gated, and worth raising
**before** the build, not after.

**The 77 font files are the parity risk, and they are not in this repo.** They ship in the
`rendercv-fonts` Python package. A Go binary cannot import it, so the fonts must be vendored,
fetched, or found on the system — three options with three different failure modes, and the
failure is silent: a PDF renders fine and every line breaks in the wrong place
(`AGENTS.md` §6.6).

**`wasm32-wasip1` is not installed.** Adding it is one `rustup target add`, so this is not a
blocker — but it does mean nobody has built this yet, and the build has not been attempted here
because attempting it is the *implementation*, not the measurement.

## 3. The route is already decided — D-006, and this plan was wrong to reopen it

**An earlier draft of this section claimed the WASI-versus-subprocess choice was an open human
gate. It is not: `specs/divergences.md` D-006 is `approved` and settles it.**

> **D-006 — Typst compiled to WASI, executed on wazero.** Status: approved. […] typst built for
> `wasm32-wasip1`, embedded in the binary, executed via wazero. […] **Watch:** the WASI build must
> carry the same font set as `rendercv-fonts`, or PDF metrics drift and Axis 1 §1.2 fails. Tracked
> in iteration 10.

The error was asserting a gate without reading the file that records them — the same failure mode
as spec 008 §8's "only a corpus `.typ` can check this" and this port's first cut of the HTML
renderer. **A gate claimed is as much a claim as a parity result, and it is checkable in one
command.** Recorded here rather than quietly deleted, because the next person tempted to declare
something blocked should see how this went.

D-006 also **pre-answers the font question**: its `Watch` line names exactly the risk §2 measured,
and assigns it to this iteration. So the 77 files are not an open design question either — they
are a stated acceptance condition.

## 3b. The three counts are now answered by building the thing

`spec.md` §5 asked three questions before any Go was written. All three are answered by
measurement, not estimate.

| §5 count | Answer | How |
|---|---|---|
| 1 — does typst build for `wasm32-wasip1`? | **Yes.** `typst`, `typst-layout`, `typst-realize`, `typst-eval`, `typst-pdf`, `typst-render`, `typst-svg`, `typst-html` at **0.14.2** — upstream's line — all compile clean. 304 crates, no patches, no feature surgery. | `cargo build --release --target wasm32-wasip1` on a shim crate depending on `typst = "0.14"` |
| 2 — can the WASI build reach fonts and packages? | **Yes**, through wazero preopens. Three mounts: `/work` (root), `/pkg` (package path), `/fonts`. Nothing else is visible to the compiler. | `wazero.NewFSConfig().WithDirMount(...)`, `wasi_snapshot_preview1.MustInstantiate` |
| 3 — size, embedded or fetched? | **29 MB** `.wasm`, `opt-level = "z"` + LTO + `strip`. 20 MB of that is the compiler; the remaining 9 MB is `typst-assets`' embedded fonts. Plus **59 MB** of `rendercv_fonts` and **428 KB** of the `fontawesome` package, neither of which is in this repo. | `stat` on the build output |

**Runtime: 3.2 s per document on wazero, single-threaded interpreter, no compilation cache.**
Upstream's native extension is faster; the parity contract does not measure speed, but 24 corpus
cases × 3.2 s is a minute of wall clock in the conformance suite and that shapes how the suite runs.

## 3c. The differential passes 14/14

Every golden `.typ` in `testdata/golden` that has a golden `.pdf` beside it — 14 cases, covering
all nine themes and the four ATS inputs — was compiled by the WASI shim on wazero and compared
against upstream's PDF on the three things the parity contract names:

- **extracted text**, `pdftotext -layout`, byte-compared — `-layout` preserves horizontal position,
  so a metric drift shows up as shifted columns rather than passing silently;
- **page count**;
- **page geometry**, from `pdfinfo`.

**14 pass, 0 fail.** The harness is mutation-checked three ways: a cross-case comparison, a
one-character change, and a leading-space-only change on one line are each caught.

**The font risk D-006 flagged is real and was hit twice on the way here** — the `fontawesome`
package and New Computer Modern, §2.6 and §2.6b. Both failed loudly, as the plan predicted, and
both were invisible on the majority of cases: the embedded-font omission passed 12 of 14.

## 4. The first task, now that nothing blocks it

Whichever route wins, the first unit is the same and is small: compile one of the 24 `.typ`
documents this port already produces byte-identically, and compare the extracted text against
upstream's PDF for the same case. That is a differential the corpus already supports —
`tools/docprobe` renders every case with a pinned date — and it fails loudly on the font question
before any of the rest is built.

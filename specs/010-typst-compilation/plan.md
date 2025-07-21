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

## 4. The first task, now that nothing blocks it

Whichever route wins, the first unit is the same and is small: compile one of the 24 `.typ`
documents this port already produces byte-identically, and compare the extracted text against
upstream's PDF for the same case. That is a differential the corpus already supports —
`tools/docprobe` renders every case with a pinned date — and it fails loudly on the font question
before any of the rest is built.

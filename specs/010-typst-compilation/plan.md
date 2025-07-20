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

## 3. What this plan deliberately does not decide

The route — WASI on wazero versus shelling out to a `typst` binary — **stays open**, and the
measurements above sharpen rather than settle it:

- WASI keeps the port pure Go and self-contained, at the cost of embedding a large artifact and
  owning a cross-compilation of a large Rust project.
- A subprocess is a divergence in what the user must install, with identical output.

`AGENTS.md` §5 routes "any change to `specs/divergences.md`" through a human, and both routes need
one — the WASI route for what the binary contains, the subprocess route for what the user must
have. **So the choice is a human call by construction, and this plan stops here rather than
picking one and building it.**

## 4. The first task once the route is chosen

Whichever route wins, the first unit is the same and is small: compile one of the 24 `.typ`
documents this port already produces byte-identically, and compare the extracted text against
upstream's PDF for the same case. That is a differential the corpus already supports —
`tools/docprobe` renders every case with a pinned date — and it fails loudly on the font question
before any of the rest is built.

# Iteration 10 — Typst compilation: PDF and PNG

Behavior of the step that turns the `.typ` of iteration 9 into a PDF and its page images, extracted
from the vendored Python. No Go design here.

Upstream: `third_party/rendercv` @ `v2.8` (`2eba248`).
Primary sources: `src/rendercv/renderer/pdf_png.py`, and the vendored Typst package at
`src/rendercv/renderer/rendercv_typst/`.

---

## 0. What this iteration is

**The only iteration whose output is not text**, and the only one whose parity is defined by
approximation rather than by bytes. The parity contract says so explicitly
(`specs/000-parity-contract/spec.md`, axis 1): PDFs match on **extracted text, page count and page
geometry**, because the raw bytes differ by embedded timestamps and object identifiers.

It is also the only iteration that needs a **non-Go artifact**: the Typst compiler itself.

## 1. What upstream calls

1. `generate_pdf` (`pdf_png.py:16-59`) compiles the `.typ` with `typst.Compiler(...).compile(...)`
   and writes the resolved `pdf_path`. It returns `None` when `dont_generate_pdf` is set **or when
   the Typst file was not generated** — the same coupling the Markdown and HTML have.
2. `generate_pngs` compiles the same source with `format="png"`, producing one file per page, named
   from `png_path` with a page number substituted.
3. `get_typst_compiler` (`:154-186`) constructs the compiler with three things: a **root**
   directory, a list of **font paths**, and a **package path**.
4. The photo is **copied next to the `.typ`** before compiling
   (`copy_photo_next_to_typst_file`), because the Typst source refers to it by base name — which
   is why iteration 9's header emits `image("me.png")` rather than a path.

## 2. The three inputs that decide whether the PDF matches

5. **Fonts.** `rendercv_fonts.paths_to_font_folders` is a Python package of font files, plus a
   `fonts/` directory beside the input file if one exists. **Different font files produce different
   glyph metrics, so a PDF compiled without them differs in every line break** — this is
   `AGENTS.md` §6.6, and it is the most likely cause of a near-miss.
6. **The Typst packages — plural, and only one of them is vendored.** `rendercv_typst/` is in the
   submodule: `lib.typ`, `template/` and a `typst.toml` declaring `@preview/rendercv:0.3.0` —
   exactly the import iteration 9's preamble emits. `get_package_path` (`pdf_png.py:114-146`)
   copies **two** files of it into a temp `preview/rendercv/0.3.0/` and passes that as
   `package_path`.

   **`lib.typ:1` then imports `@preview/fontawesome:0.6.0`, which is not vendored anywhere.**
   Upstream resolves it by **downloading it from Typst Universe** into the compiler's own package
   cache (`~/.cache/typst/packages/preview/fontawesome/0.6.0`, 428 KB, 7 files). Measured: a
   compile with only `rendercv` in `package_path` fails with
   `file not found (searched at .../preview/fontawesome/0.6.0/typst.toml)`.

   An earlier draft of this line read "resolved through `package_path`, not downloaded". That is
   true of `rendercv` and false of its dependency, and the difference is a network fetch on first
   render — which a Go port must either vendor or reproduce.

6b. **Typst's own embedded fonts are a third input, and they are not `rendercv_fonts`.** The
   `sb2nov` theme asks for **New Computer Modern**, which `rendercv_fonts` does not ship; it comes
   from the `typst-assets` crate that every typst distribution links in. Measured: without it,
   `theme_sb2nov` renders in a fallback face — `PhD in Computer Science` extracts as
   `PhDinComputer Science` — and `theme_opal` shifts two lines by one space. The other **12 of 14**
   cases pass anyway, so this defect is visible on 2 cases out of 14.
7. **The root.** The compiler's root is the `.typ`'s own directory, which is what makes the photo's
   base-name reference resolve.

## 3. Out of scope

**3.1 The `.typ` itself** is iteration 9's and is byte-identical on all 24 differential cases.

**3.2 The CLI's `-nopdf`/`-nopng`** already exist and are honoured (iteration 12); what is missing
is only what they switch off.

---

## 4. Acceptance criteria

- [ ] A PDF whose **extracted text** equals upstream's for every corpus case.
- [ ] Equal **page count** and **page geometry**.
- [ ] One PNG per page, with upstream's names and dimensions.
- [ ] The 14 `artifacts` corpus cases green, jointly with iteration 12.

## 5. The hazard, and the measurement that must precede any design

`AGENTS.md` §2 names the intended approach: **typst compiled to WASI, run on wazero** — pure Go, no
CGO. That decision predates this spec and has not been measured. Three things must be counted
before a line of Go is written, and each has killed a plausible-looking plan elsewhere in this port:

1. **Does typst build for `wasm32-wasip1` at all, at the version that produces upstream's output?**
   `typst-py` wraps a specific compiler version; a different one may lay out a page differently,
   and that difference would appear as a text-extraction diff with no obvious cause.
2. **Can the WASI build reach the fonts and the package directory?** wazero needs an explicit
   filesystem mapping. This is mechanical, but it decides the whole shape of the embedding.
3. **What is the binary's size, and is it embedded or fetched?** Embedding a multi-megabyte WASM in
   the Go binary is a distribution decision, not a technical one, and it is the kind of thing that
   should be recorded in `divergences.md` if it changes what a user installs.

**The lesson this port keeps re-learning applies here in advance**: iteration 8 asserted a
fragment differential was impossible and hid a real bug behind it; iteration 11 asserted the HTML
needed a block-layer port and it needed one rule. Both were estimates stated as conclusions. So the
first task of this iteration is not code — it is a `plan.md` that reports the three counts above,
with the commands that produced them.

**An honest alternative must stay on the table until (1) is answered.** If the WASI route cannot
reproduce upstream's layout, shelling out to a `typst` binary is a divergence with a very different
cost profile — worse for distribution, identical for output — and choosing between them is a human
call under `AGENTS.md` §5, not something to settle by starting to build.

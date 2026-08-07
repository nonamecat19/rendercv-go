# Spec 000 — The Parity Contract

**Status:** normative · **Upstream:** `third_party/rendercv` @ `v2.8` (`2eba248`)

This spec defines what "identical to RenderCV" means for `rendercv-go`. Every other spec in this
tree inherits from it. When a later spec conflicts with this one, this one wins.

---

## 0. Scope

`rendercv-go` must be substitutable for `rendercv` v2.8 in any automated pipeline: same inputs,
same files out, same messages, same exit codes. A user swapping the binary should notice nothing
but the executable's name.

---

## 1. Axis 1 — Artifact parity

### 1.1 Byte-identical outputs

For every corpus case, these must be **byte-for-byte identical** to the artifact produced by
`third_party/rendercv`:

| Artifact | Upstream producer |
|---|---|
| `*.typ` | `renderer/typst.py` |
| `*.md` | `renderer/markdown.py` |
| `*.html` | `renderer/html.py` |
| `schema.json` | `schema/json_schema_generator.py` |

Byte-identical means: same bytes, same length, same trailing newline, same line endings (LF),
same UTF-8 encoding, no BOM. Whitespace is part of the contract — Jinja's `trim_blocks` /
`lstrip_blocks` behavior (`renderer/templater/templater.py:43-44`) is observable in the output.

### 1.2 PDF and PNG

Raw bytes of a PDF are **not** required to match: Typst embeds a creation timestamp and a
document ID. Instead, the following must match:

- page count
- extracted text content of each page, in reading order
- page dimensions (width × height, in points) for each page
- embedded font names, as a set

PNG: same page count, same pixel dimensions per page. Pixel-level comparison is a stretch goal
tracked in `specs/STATE.md`, not a gate — it depends on the WASI typst build using the exact
`rendercv-fonts` set.

### 1.3 Output file naming and layout

`render` writes into `rendercv_output/` with upstream's exact naming and directory structure
(`renderer/path_resolver.py`). Path-override flags resolve identically, including relative-path
semantics and parent-directory creation.

---

## 2. Axis 2 — CLI parity

### 2.1 Commands

`new`, `render`, `create-theme`. No commands added, none removed, none renamed.

### 2.2 Global behavior

- `--version` / `-v` prints `RenderCV v<version>` — reproduced verbatim, including the product
  name `RenderCV` (`cli/app.py:41`). The version reported is the upstream version this build
  targets (`2.8`), not a Go-specific version. A Go build identifier may be appended only on a
  separate line.
- `-h` / `--help` are both accepted (`cli/app.py:24`).
- Invoking with no command prints help and exits (`cli/app.py:42-44`).

### 2.3 `render` flags

Every long flag and every short alias, with upstream's exact spelling:

| Long | Short |
|---|---|
| `--watch` | `-w` |
| `--quiet` | `-q` |
| `--design` | `-d` |
| `--locale-catalog` | `-lc` |
| `--settings` | `-s` |
| `--pdf-path` | `-pdf` |
| `--typst-path` | `-typ` |
| `--markdown-path` | `-md` |
| `--html-path` | `-html` |
| `--png-path` | `-png` |
| `--dont-generate-pdf` | `-nopdf` |
| `--dont-generate-typst` | `-notyp` |
| `--dont-generate-markdown` | `-nomd` |
| `--dont-generate-html` | `-nohtml` |
| `--dont-generate-png` | `-nopng` |

Plus **dot-notation overrides** for any YAML key, including list indices:
`--cv.phone`, `--cv.sections.education.0.institution`, `--design.theme`
(`cli/render_command/parse_override_arguments.py`).

Note: short flags such as `-nopdf` are multi-character single-dash forms that Go's `flag` and
`cobra`'s default `pflag` do not natively support. Reproducing them is mandatory; the CLI spec
(`specs/012-cli/spec.md`) must define a pre-parse normalization step.

### 2.4 `new` and `create-theme`

`new NAME [--theme T] [--locale L] [--create-typst-templates]`;
`create-theme NAME`. Same defaults, same generated file names, same generated file contents.

### 2.5 Exit codes and streams

- `0` on success.
- `1` on user error, with the message rendered inside a bordered panel titled `Error`
  (`cli/error_handler.py:38-48`). Panel geometry is part of Axis 4, not Axis 2.
- Non-user (internal) errors keep upstream's behavior of surfacing a traceback-equivalent;
  Go emits a comparable "internal error" report and exits non-zero.
- Progress output goes where upstream sends it; `--quiet` suppresses all of it.

### 2.6 Explicitly out of contract

The PyPI version-check warning (`cli/app.py:110-133`) and its cache file are Python-packaging
concerns. `rendercv-go` does not contact PyPI. Logged in `divergences.md`.

---

## 3. Axis 3 — JSON Schema parity

The **generation path** — `go run ./tools/genschema` — must produce a document that diffs
**empty** against `third_party/rendercv/schema.json`, byte for byte: 2-space indent, keys in
upstream's emission order, non-ASCII literal, and **no trailing newline**.

*(Corrected twice, in place rather than silently, because both errors were reachable from the
original text.*

*First: this section previously read "`rendercv-go schema` (or the equivalent generation path)".
Taken as licence to add a subcommand, that breaks §2.1 — upstream's CLI has three commands and
none of them is `schema`; it generates its file from `generate_json_schema_file`, outside the CLI.
The generation path is now named, so the two axes cannot be read as contradicting.*

*Second: it said "trailing newline". Upstream's file has none — `json.dumps` does not append one
and `write_text` adds nothing, and the file's last three bytes are measured as `"\n}`. A port
appending one would diff on the last line of a 405 KB file.)*

Key order is part of the contract: editors surface schema properties in document order.

---

## 4. Axis 4 — Validation-error parity

For any invalid input, `rendercv-go` must produce:

- the same **error message text**, including punctuation, backticks, and interpolated values
  (`schema/error_dictionary.yaml`, `schema/pydantic_error_handling.py`,
  `schema/models/custom_error_types.py`);
- the same **location path** (e.g. `cv.sections.education.0.start_date`);
- the same **input echo** where upstream shows the offending value;
- the same **ordering** when multiple errors are reported;
- the same **count** of reported errors.

Reference case: `tests/schema/testdata/test_pydantic_error_handling/wrong_input.yaml` and its
expected output `tests/schema/testdata/test_pydantic_error_handling/expected_errors.yaml`. That
pair is imported directly into the conformance corpus.

Where an error originates in pydantic itself rather than RenderCV, the message must still match —
pydantic's phrasing is user-visible and therefore part of the contract.

---

## 5. The sanctioned divergence

Exactly one deviation is approved without further review:

> The executable is named **`rendercv-go`**, not `rendercv`.

This affects the binary name, `argv[0]`, and any usage strings that embed the program name.
It does not license changing the product name in `--version` output (see §2.2).

Everything else that cannot achieve parity **must** be entered in
[`specs/divergences.md`](../divergences.md), with an upstream citation and a justification, and
must pass the human gate defined in `AGENTS.md` §5.

---

## 6. How parity is measured

Parity is what `just test-parity` prints. Not a self-report, not a review comment.

- Goldens are generated by `tools/gengolden` running the vendored Python via `uv`. Hand-written
  goldens are forbidden (`AGENTS.md` §10).
- `testdata/golden/manifest.json` records the upstream commit SHA and a sha256 per file, so
  fixture drift is detectable and reviewable.
- A conformance case that has ever passed must never regress; CI fails on regression.
- `specs/STATE.md` tracks the count of passing cases per subsystem.

## 7. Acceptance criteria for the contract itself

- [ ] Corpus covers all 9 themes, all 9 entry types, at least 3 locales incl. an RTL one,
      the minimal and full model fixtures, and at least 5 invalid-input cases.
- [ ] Every axis has at least one executable test.
- [ ] `manifest.json` exists and is verified in CI.

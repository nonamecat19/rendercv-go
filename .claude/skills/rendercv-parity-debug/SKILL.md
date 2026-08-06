---
name: rendercv-parity-debug
description: Diagnose a failing parity test in the RenderCV Go port — a byte diff in .typ/.md/.html/schema.json, a mismatched validation error message, a CLI surface difference, or a PDF/PNG mismatch. Use whenever `just test-parity` or `just schema-diff` fails.
---

# Parity debugging

The golden is right. Go is wrong. Start from that assumption and only abandon it with evidence.

## 0. Classify the failure

| Symptom | Go here |
|---|---|
| `.typ` / `.md` / `.html` byte diff | §1 |
| `schema.json` diff | §2 |
| Wrong validation error text/order/count | §3 |
| CLI flag, exit code, or output-shape difference | §4 |
| PDF/PNG mismatch | §5 |

## 1. Byte diffs in rendered text

### 1.1 Localize
```bash
cmp golden.typ got.typ                       # first differing byte
diff -u golden.typ got.typ | head -40
diff -u <(cat -A golden.typ) <(cat -A got.typ) | head -40   # whitespace visible
```

Always look at `cat -A` output. Most diffs in this project are invisible otherwise.

### 1.2 Rank the causes, in this order

1. **Whitespace from `trim_blocks` / `lstrip_blocks`.** Upstream enables both
   (`third_party/rendercv/src/rendercv/renderer/templater/templater.py:43-44`); pongo2 has
   neither. `trim_blocks` removes the newline after a block tag; `lstrip_blocks` strips
   whitespace from line start to a block tag. If the diff is a stray `$` (newline) or leading
   spaces around a `{% %}`, this is it. See D-005 in `specs/divergences.md`.
2. **Line-splitting.** Upstream calls `.splitlines()` in templates; the Go port pre-splits in the
   model. Check whether the Go splitter matches Python's `str.splitlines()` — Python splits on
   `\v`, `\f`, `\x1c`-`\x1e`, `\x85`, ` `, ` ` too, and drops a trailing empty element.
   `strings.Split(s, "\n")` does none of that.
3. **Slice bounds.** Python's `x[:n]` clamps; Go's `x[:n]` panics past `len`. A silent off-by-one
   here shows up as a missing or duplicated line.
4. **Filter behavior.** Only two are custom: `clean_url`, `strip` (`templater.py:46-47`).
   Everything else — `indent`, `length` — is Jinja builtin, and pongo2's version may differ.
   Jinja's `indent(n)` does **not** indent the first line by default. Verify against Jinja, not
   against intuition.
5. **Number and date formatting.** Python `str(float)` ≠ Go `%v`. Date output comes from
   `renderer/templater/date.py` and the locale catalog.
6. **Map ordering.** Go map iteration is randomized. Anything emitted from a map must be sorted
   deterministically, matching upstream's insertion or sort order.
7. **Encoding.** UTF-8, no BOM, LF endings, and one trailing newline exactly.

### 1.3 Find the emitting code
Locate the fragment in the upstream template, then read the matching Go template and processor:

```bash
grep -rn "<fragment>" third_party/rendercv/src/rendercv/renderer/templater/templates/
grep -rn "<fragment>" internal/renderer/templater/
```

### 1.4 Confirm against live upstream
```bash
cd third_party/rendercv && uv run --frozen --all-extras rendercv render <case>.yaml -nopdf -nopng
```
Shrink the input until the diff disappears — the last removal names the trigger.

## 2. Schema diffs

```bash
just schema-diff
```
Order matters (contract §3). Check, in order: key order, `required` array order, `anyOf`/`oneOf`
member order, number formatting (`1` vs `1.0`), `$ref` spelling, and description strings copied
verbatim from docstrings.

## 3. Validation-error mismatches

Contract §4 requires identical text, location path, input echo, order, and count.

Sources upstream: `schema/error_dictionary.yaml`, `schema/pydantic_error_handling.py`,
`schema/models/custom_error_types.py`. The reference pair is
`tests/schema/testdata/test_pydantic_error_handling/{wrong_input,expected_errors}.yaml`.

Check count and order before wording — a missing error is a bigger bug than a typo, and a
wording diff is often just the last symptom of validating in a different sequence.

## 4. CLI differences

```bash
diff <(cd third_party/rendercv && uv run --frozen --all-extras rendercv render --help) \
     <(./bin/rendercv-go render --help)
```
Watch for the multi-character single-dash flags (`-nopdf`, `-lc`, `-html`) — pflag does not
support them natively and they need the pre-parse normalization from the CLI spec. Then check
exit codes (`echo $?`) and which stream each message went to.

## 5. PDF / PNG

Before touching the compiler, prove the `.typ` input is byte-identical. If it is not, this is a
§1 problem wearing a disguise.

Then compare in this order — page dimensions, page count, extracted text, embedded font names —
and stop at the first that differs. A dimension or font difference is almost always the font set
(D-006), not the compiler. Hand off to `rendercv-typst-engineer`.

## 6. Rules while fixing

- Never edit a golden (`AGENTS.md` §10.1).
- Never loosen an assertion, normalize away a diff, or add a "close enough" comparator.
- Fix the smallest thing that explains the diff, then re-run the whole parity suite — whitespace
  fixes routinely break three other cases.
- If parity is genuinely unreachable, stop and draft a `specs/divergences.md` entry for the human
  gate. Do not decide it alone.

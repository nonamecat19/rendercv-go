# Spec delta 002-N — the reader's universal-newline translation

Extends [`spec.md`](spec.md) and closes the first bullet of
[`spec-delta-printable.md`](spec-delta-printable.md) §6. Nothing here supersedes either. Every
string below was measured by running the vendored Python
(`third_party/rendercv/.venv/bin/rendercv`, RenderCV v2.8, CPython 3.12.13) and the port side by
side, per `AGENTS.md` §10.1. The recipe is in §6.

---

## 0. The class

**Axis-4 defect.** Upstream never sees a carriage return in a document it read from disk. The port
read raw bytes, so a `\r` reached the YAML scanner, and the two disagreed on the message *and* the
span of the resulting validation record — and, on one accepted document, on the name and contents
of every generated artifact.

Minimal, measured — `CV.yaml` holding the bytes `cv:\n  name: \rA\n`:

| | Location | Explanation |
|---|---|---|
| upstream | `main_yaml_file: line 3 to line 4` | `This is not a valid YAML file. while scanning a simple key.` |
| port, before | `main_yaml_file: line 1 to line 3` | `This is not a valid YAML file. while parsing a block mapping.` |

---

## 1. Where the translation lives upstream

Every input document reaches RenderCV through `pathlib.Path.read_text(encoding="utf-8")`:

| Site | What it reads |
|---|---|
| `cli/render_command/run_rendercv.py:115` | the main YAML, inside `collect_input_file_paths` |
| `cli/render_command/run_rendercv.py:140` | the main YAML, inside `run_rendercv` |
| `cli/render_command/render_command.py:212` | `--design` |
| `cli/render_command/render_command.py:213` | `--locale-catalog` |
| `cli/render_command/render_command.py:215` | `--settings` |
| `schema/yaml_reader.py:49` | `read_yaml`'s own `Path` branch, unreachable from the CLI (see `internal/schema/yamlreader.ReadFile`) |

`read_text`'s signature is `read_text(self, encoding=None, errors=None)`, and it forwards to
`self.open(mode='r', encoding=encoding, errors=errors)` — CPython 3.12.13 `pathlib.py:1022-1028`.
`newline` is therefore left at its default `None`, which is text mode with **universal-newline
translation enabled**. `IncrementalNewlineDecoder.decode` performs it (CPython 3.12.13
`_pyio.py:1925-1929`; the C accelerator in `Modules/_io/textio.c` mirrors it):

```python
if self.translate:
    if crlf:
        output = output.replace("\r\n", "\n")
    if cr:
        output = output.replace("\r", "\n")
```

So `\r\n` first, then every remaining lone `\r`, both to `\n`. The order is load-bearing: the other
order turns one CRLF into two line breaks.

### 1.1 What the rule is not

- **Not a YAML rule.** The transform runs a whole layer below ruamel, on the decoded stream, so it
  is context-free: a `\r` inside a quoted scalar or a literal block scalar is translated exactly
  like one between two mapping keys. §4's scalar rows measure both.
- **Not a line-break rule in general.** Python translates exactly three sequences. `U+0085` NEL and
  `U+2028` LINE SEPARATOR are untouched, even though YAML 1.1 treats NEL as a break — and
  `spec-delta-printable.md` §1.1 admits NEL as printable, so a document carrying one renders on
  both sides.
- **Not applied to an escape.** A `\r` *written* as the two characters `\` `r` in a double-quoted
  scalar is produced by the YAML scanner long after the read boundary. Upstream carries that
  carriage return into the rendered Markdown; §4's `escaped CR` row measures one CR byte in the
  `.md` on both sides.

## 2. Why the line numbers move

The translation changes how many lines a document has, and a validation record's Location column is
`line N` or `line N to line M` (`rendercv_model_builder.py:42-62`). Both ends move together:

- `cv:\n  name: \rA\n` is 2 lines raw and **3 lines translated**, which is why upstream's span ends
  at line 4 and the untranslated port's at line 3.
- A CRLF document has the same line count either way, but goccy's scanner reports a *different
  fault* on it: for §4's `syntax error` document upstream says `while parsing a block mapping.`
  spanning `line 2 to line 6` and the untranslated port said `[6:4] value is not allowed in this
  context.` at `line 6` — a different message, a different location shape, and a differently-sized
  panel.

A fix that only translated lone `\r` would leave the second case broken; a fix that only counted
lines would leave the first. Translating the text is what makes both follow.

## 3. Where the port must apply it

At the read boundary and nowhere else — the four sites in the port that correspond to the six
upstream sites above. Downstream reads are **not** in scope and were left alone: the Markdown file
`renderer/html.py:34` reads back is the port's own output, and the Jinja template files
`templater.py`'s loader opens are not input documents.

## 4. Measured, both sides

`render CV.yaml --settings.current_date 2025-03-05 --dont-generate-pdf --dont-generate-png`,
non-tty, `NO_COLOR=1 TERM=dumb COLUMNS=80`, in a fresh temporary directory. `before` and `after`
are the port without and with the translation; a blank cell means it equals upstream.

Documents: `BASE` is `cv:\n  name: John Doe\n  sections:\n    test_section:\n    - this is a text
entry.\n`; `SYNTAX` is `cv:\n  name: John Doe\n  sections:\n    test_section:\n    - a\n   b: c\n`.

### 4.1 Rejected documents

| Document | upstream Location | upstream Explanation | port, before |
|---|---|---|---|
| `cv:\n  name: \rA\n` | `main_yaml_file: line 3 to line 4` | `…while scanning a simple key.` | `line 1 to line 3`, `…while parsing a block mapping.` |
| `cv:\n  name: \nA\n` (the LF twin) | `main_yaml_file: line 3 to line 4` | `…while scanning a simple key.` | — |
| `SYNTAX`, LF | `main_yaml_file: line 2 to line 6` | `…while parsing a block mapping.` | — |
| `SYNTAX`, CRLF | `main_yaml_file: line 2 to line 6` | `…while parsing a block mapping.` | `line 6`, `…[6:4] value is not allowed in this context.` |
| `SYNTAX`, CR only | `main_yaml_file: line 2 to line 6` | `…while parsing a block mapping.` | `line 1 to line 6` |
| a CR inside a literal block scalar | `main_yaml_file: line 7 to line 8` | `…while scanning a simple key.` | `line 1 to line 7`, `…while parsing a block mapping.` |
| `--settings` file with a lone CR | `settings_yaml_file: line 4 to line 5` | `…while scanning a simple key.` | `line 1 to line 4`, `…while parsing a block mapping.` |
| `--locale-catalog` file with a lone CR | `locale_yaml_file: line 3 to line 4` | `…while scanning a simple key.` | `line 1 to line 3`, `…while parsing a block mapping.` |

Every Explanation is prefixed `This is not a valid YAML file. `; the column above elides it. Exit
code is 1 on every row, on all three sides.

### 4.2 Accepted documents

| Document | upstream | port, before |
|---|---|---|
| `BASE`, LF | exit 0, `John_Doe_CV.{typ,md,html}` | — |
| `BASE`, CRLF | exit 0, same three, byte-identical | — |
| `BASE`, CR only | exit 0, same three, byte-identical | — |
| `BASE`, mixed CR/LF/CRLF | exit 0, same three, byte-identical | — |
| `BASE` + a trailing `\r` | exit 0, same three | — |
| `BASE` + a trailing `\r\n` | exit 0, same three | — |
| `name: "A\rB"` (a real CR) | exit 0, `A_B_CV.*` — the break folds to a space | — |
| `name: 'A\rB'` (a real CR) | exit 0, `A_B_CV.*` | — |
| `name: "A\r\nB"` | exit 0, `A_B_CV.*` | exit 0, **`A\nB_CV.*`** — a newline in the filename, and different bytes |
| a CRLF inside a literal block scalar | exit 0, one CR-free `.md` | — |
| `- "first\\rsecond"` (an escape) | exit 0, **one CR byte in the `.md`** | — |
| `--design` overlay, CRLF | exit 0 | — |
| `--settings` overlay, CRLF | exit 0 | — |

The accepted rows are as much a part of the rule as the rejected ones. The `A\r\nB` row is the one
that proves the translation has to happen at the *read*: a port that only re-numbered lines would
still have written a file whose name contains a newline.

## 5. Acceptance criteria

1. Every row of §4 agrees with upstream on exit code, both streams byte for byte (rejected rows) or
   panel geometry plus the exact set and bytes of generated files (accepted rows).
2. The `--design`, `--locale-catalog` and `--settings` overlays are translated too.
3. No output path is translated: `escaped CR` keeps its carriage return in the `.md`.

## 6. Recipe

```py
p = pathlib.Path(d) / "CV.yaml"
p.write_bytes(document.encode("utf-8"))
subprocess.run([binary, "render", "CV.yaml", "--settings.current_date", "2025-03-05",
                "--dont-generate-pdf", "--dont-generate-png"], cwd=d,
               env={**os.environ, "NO_COLOR": "1", "TERM": "dumb", "COLUMNS": "80"})
```

## 7. Out of scope, recorded not fixed

- **Invalid UTF-8**, unchanged from `spec-delta-printable.md` §6: Python raises `UnicodeDecodeError`
  out of `read_text` before RenderCV runs (class D-011), and the port decodes to `U+FFFD`.
- **The Markdown read-back.** `renderer/html.py:34` reads the generated `.md` with `read_text` too,
  so a carriage return the port writes into the Markdown — reachable through a `\r` escape — is
  translated on upstream's way into the HTML. Both sides agree on §4's `escaped CR` row today, and
  no measured document separates them, so nothing is changed there.

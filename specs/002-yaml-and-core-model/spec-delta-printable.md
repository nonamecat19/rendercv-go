# Spec delta 002-P — the reader's printable-character rule

Extends [`spec.md`](spec.md). Nothing here supersedes it. Every string below was measured by
running the vendored Python (`third_party/rendercv/.venv/bin/rendercv`) and the port side by side,
per `AGENTS.md` §10.1. The recipe is in §5.

---

## 0. The class

**Axis-1 defect.** `goccy/go-yaml` accepts C0 and C1 control characters, and `U+FFFE`/`U+FFFF`,
anywhere in a document. ruamel refuses them **before scanning starts**, so the port rendered a
complete CV at exit 0 for documents upstream rejects at exit 1 with a validation table.

Minimal, measured — `CV.yaml` holding the bytes `cv:\n  name: \x01A\n`:

```
╭─ There are validation errors! ───────────────────────────────────────────────╮
│ ╭────────────────┬─────────────┬───────────────────────────────────────────╮ │
│ │ Location       │ Input Value │ Explanation                               │ │
│ ├────────────────┼─────────────┼───────────────────────────────────────────┤ │
│ │ main_yaml_file │ ...         │ This is not a valid YAML file.            │ │
│ │                │             │ unacceptable character #x0001: special    │ │
│ │                │             │ characters are not allowed.               │ │
│ ╰────────────────┴─────────────┴───────────────────────────────────────────╯ │
╰──────────────────────────────────────────────────────────────────────────────╯
```

---

## 1. Where the rule lives upstream

`read_yaml` (`third_party/rendercv/src/rendercv/schema/yaml_reader.py:53`) hands a **`str`** to
`yaml.load`. ruamel's `Reader.stream` setter takes the `str` branch and calls
`check_printable(val)` on the **whole document** before assigning the buffer
(`.venv/lib/python3.12/site-packages/ruamel/yaml/reader.py:105-108`). So the check precedes every
scanner, parser and composer diagnostic: a document carrying both a forbidden character and a
syntax error reports the forbidden character.

The predicate is one regular expression, `reader.py:187-189`:

```python
NON_PRINTABLE = RegExp(
    '[^\x09\x0A\x0D\x20-\x7E\x85' '\xA0-\uD7FF' '\uE000-\uFFFD' '\U00010000-\U0010FFFF' ']'
)
```

`_get_non_printable` (`reader.py:210-214`) tries an ASCII fast path first
(`_get_non_printable_ascii`, `:193-200`, whose permitted set at `:191` is the same `\x09\x0A\x0D`
plus `0x20-0x7E`) and falls back to the regex on `UnicodeEncodeError`. Both find the **first**
offending character in source order, so the two paths agree.

### 1.1 The permitted set, exactly

| Range | Verdict |
|---|---|
| `U+0000`–`U+0008` | rejected |
| `U+0009` TAB, `U+000A` LF | permitted |
| `U+000B`, `U+000C` | rejected |
| `U+000D` CR | permitted |
| `U+000E`–`U+001F` | rejected |
| `U+0020`–`U+007E` | permitted |
| `U+007F`–`U+0084` | rejected (DEL and the low C1 block) |
| `U+0085` NEL | permitted |
| `U+0086`–`U+009F` | rejected (the rest of C1) |
| `U+00A0`–`U+D7FF` | permitted |
| `U+D800`–`U+DFFF` | surrogates, unreachable from a UTF-8 file |
| `U+E000`–`U+FFFD` | permitted |
| `U+FFFE`, `U+FFFF` | rejected |
| `U+10000`–`U+10FFFF` | permitted, **the whole plane range including every non-character** |

The last row is why no measured rejection can carry a codepoint above `U+FFFF`: the regex admits
the astral range whole.

## 2. The message

`ReaderError.__str__` (`reader.py:52-56`) builds

```
unacceptable character #x{character:04x}: {reason}
  in "{name}", position {position:d}
```

with `reason` fixed at `'special characters are not allowed'` (`reader.py:221-227`). The hex is
**lowercase, zero-padded to a minimum of four digits** — a `#x{...:04x}` format, so a codepoint
above `U+FFFF` would print five or six digits, which §1.1 makes unreachable.

`read_yaml_with_validation_errors` (`rendercv_model_builder.py:86-101`) keeps
`str(e).splitlines()[0].strip()` and appends `.` when absent, so the second line — the position —
is **discarded**. The record's message is therefore

```
This is not a valid YAML file. unacceptable character #x0001: special characters are not allowed.
```

## 3. The location is empty

`get_yaml_error_location` (`rendercv_model_builder.py:42-62`) reads `context_mark` and
`problem_mark`. `ReaderError` (`reader.py:35-42`) carries **neither** — it has `name`, `character`,
`position`, `encoding`, `reason` and nothing else — so the location is `None` and the Location
column prints the bare source name, `main_yaml_file`, with no line number. This is the only
measured YAML-syntax record in the port with no coordinates at all.

## 4. Measured, both sides

Document `cv:\n  name: <CH>A\n`, non-tty, `NO_COLOR=1 TERM=dumb COLUMNS=80`, `render CV.yaml`
in a fresh temporary directory.

| `<CH>` | upstream exit | upstream verdict | port exit before this delta |
|---|---|---|---|
| `U+0000` | 1 | `unacceptable character #x0000` | 1, but `OS Error: … invalid argument` — a NUL in the output filename, not the rule |
| `U+0001` | 1 | `unacceptable character #x0001` | 0, five artifacts |
| `U+0008` | 1 | `unacceptable character #x0008` | 0, five artifacts |
| `U+000B` | 1 | `unacceptable character #x000b` | 0, five artifacts |
| `U+000C` | 1 | `unacceptable character #x000c` | 0, five artifacts |
| `U+001F` | 1 | `unacceptable character #x001f` | 0, five artifacts |
| `U+007F` | 1 | `unacceptable character #x007f` | 0, five artifacts |
| `U+0085` NEL | 0 | renders | 0, renders — agrees |
| `U+FFFE` | 1 | `unacceptable character #xfffe` | 0, five artifacts |
| `U+1F600` 😀 | 0 | renders | 0, renders — agrees |
| `U+0009` TAB | 1 | `while scanning for the next token.`, line 2 | identical panel — the tab rule, not this one |
| `U+000D` CR | 1 | `while scanning a simple key.`, line 3 to line 4 | exit 1, **different** message and span — unrelated open defect, §6 |
| `U+000A` LF | 1 | `while scanning a simple key.`, line 3 to line 4 | identical panel |

The three permitted characters are as much a part of the rule as the eight forbidden ones: a check
that rejected NEL, TAB or an astral emoji would be a new defect of the opposite sign.

## 5. Recipe

```py
p = pathlib.Path(d) / "CV.yaml"
p.write_bytes(("cv:\n  name: %sA\n" % ch).encode("utf-8"))
subprocess.run([binary, "render", "CV.yaml"], cwd=d,
               env={**os.environ, "NO_COLOR": "1", "TERM": "dumb", "COLUMNS": "80"})
```

## 6. Out of scope, recorded not fixed

- **CR.** Upstream reads the input with `pathlib.Path.read_text(encoding="utf-8")`
  (`cli/render_command/run_rendercv.py:115,140`), whose default `newline=None` applies **universal
  newline translation**: a lone `\r` becomes `\n` before ruamel sees it. The port reads bytes. For
  `cv:\n  name: \rA\n` upstream says `while scanning a simple key.` at line 3 to line 4 and the
  port says `while parsing a block mapping.` at line 1 to line 3. That is a newline-translation
  defect, not a printable-set one, and it is untouched here.
- **Invalid UTF-8.** Python raises `UnicodeDecodeError` out of `read_text` before `read_yaml` is
  reached, which is the unhandled-traceback class D-011 already covers. The port decodes such bytes
  to `U+FFFD`, which §1.1 permits, so this delta invents no message for them.

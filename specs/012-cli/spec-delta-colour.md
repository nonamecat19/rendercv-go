# Spec 012 delta — the port emits no ANSI colour, and nothing can see it

**Status:** proposal · **Extends:** [`spec.md`](spec.md) · **Blocks:** an axis-2 (CLI parity) hole

**Upstream covered:**

- `src/rendercv/cli/render_command/progress_panel.py`, `cli/error_handler.py`,
  `cli/new_command/{new_command,print_welcome}.py`, `cli/create_theme_command/create_theme_command.py`
  (where a style is named)
- `rich/console.py`, `rich/live.py`, `rich/style.py` (when a style becomes bytes)
- `typer/rich_utils.py` (`--help`, which has its **own** console and its own rules)

Citations to `src/...` are relative to `third_party/rendercv/`. Citations to `rich/...` and
`typer/...` are relative to `third_party/rendercv/.venv/lib/python3.12/site-packages/`, the resolved
dependencies the vendored submodule pins and runs — `rich` 14.3.2.

**Every sequence in this document was measured**, by running both binaries under `script -qec` on a
real PTY at `COLUMNS=100` and reading the bytes back. Nothing here is read off the source alone.
Where source is cited it is to name the *rule*; the bytes are always the measurement.

---

## 1. The finding

On a terminal, the port emits **no ANSI escape sequence of any kind**. Upstream emits many.

| Case | upstream | port |
|---|---|---|
| `render CV.yaml` | 2963 B, 12 lines carrying ESC | 1106 B, **zero** |
| `render CV.yaml` (invalid) | 3762 B, 15 lines carrying ESC | 2235 B, **zero** |
| `render --help` | 6720 B, 40 lines carrying ESC | 5068 B, **zero** |
| `new JohnDoe` | 2718 B, 14 lines carrying ESC | 2139 B, **zero** |
| `render CV.yaml --watch` (12 s) | 68,275 B, 318 × `ESC[2K`, 272 × `ESC[1A` | ~2 KB, **zero** |

The port's terminal output is byte-identical to what upstream produces with colour *disabled* —
`TTY_COMPATIBLE=0` gives upstream 1106 bytes, the port's exact size on the success path. That is the
honest summary of the state: **the port behaves as though it is never on a terminal.**

### 1.1 Why no gate sees it

Two independent reasons, either of which alone would be enough:

1. Every golden is captured by `exec.Command` with `cmd.Stdout = &stdout`
   (`tools/gengolden/main.go:214-218`) — a pipe, never a PTY. Rich turns colour off when
   `isatty()` is false (`rich/console.py:937-984`).
2. The corpus pins `{"COLUMNS": "80", "NO_COLOR": "1", "TERM": "dumb"}` (`testdata/corpus.json`),
   and either of `NO_COLOR` or `TERM=dumb` suppresses colour on its own (§3).

So the suite is blind to the port's terminal appearance and **always has been**. This is not a
regression; it is a surface that was never gated. Consequently §6 is not optional decoration: a
delta that cannot be gated will silently rot back.

### 1.2 Which half of the surface this document is about

The CLI width work that landed alongside this investigation measured everything **non-tty**, because
that is the mode the goldens are captured in. This document is about the **tty** mode, which nothing
has ever gated. The two halves do not overlap, and being explicit about the boundary is what keeps a
later reader from thinking one of them re-measured the other:

| Behavior | Mode | Gated today |
|---|---|---|
| panel and table geometry, wrapping, cropping | both | ✅ 42 golden cases, non-tty |
| `COLUMNS` honoured | non-tty | ✅ |
| every SGR sequence in §2 | **tty only** | ❌ — nothing can see it |
| OSC 8 hyperlinks (§2.5) | **tty only** | ❌ |
| cursor hide/show and the repaint loop (§5) | **tty only** | ❌ |
| width 80 with `COLUMNS` ignored (§3.4) | **tty only**, and `TERM=dumb` | ❌ |

Every ❌ row needs a PTY, which is why §6 is a unit of its own and why it has to land first.

---

## 2. The styled surface — what is styled, and with what

### 2.1 Where a style is named

| Surface | Element | Style | Citation |
|---|---|---|---|
| progress panel | border + title | `bright_black` | `progress_panel.py:61`, `:116` |
| progress panel | `✓` | `green` | `progress_panel.py:107` |
| progress panel | `<n> ms` field | `bold green` | `progress_panel.py:104` |
| progress panel | output paths | `purple` | `progress_panel.py:106` |
| user-error panel | border | `bold red` | `progress_panel.py:132`, `error_handler.py:46` |
| user-error panel | title `Error` | `bold red` | `progress_panel.py:130`, `error_handler.py:44` |
| validation panel | border + title | `bold red` | `progress_panel.py:163-165` |
| validation table | `Location` column | `cyan` | `progress_panel.py:149` |
| validation table | `Input Value` column | `magenta` | `progress_panel.py:150` |
| validation table | `Explanation` column | `orange4` | `progress_panel.py:151` |
| validation table | header row | `bold` (Rich's default header style) | measured |
| `new` welcome | `RenderCV v<version>` | `dodger_blue3` | `print_welcome.py:14` |
| `new` welcome | link titles | `bold cyan` | `print_welcome.py:22` |
| `new` welcome | the URLs | `[link=…]` → **OSC 8**, not SGR | `print_welcome.py:22` |
| `new` welcome | `Useful Links` panel | `bright_black` | `print_welcome.py:29` |
| `new` | `✓` / created path | `green` / `purple` | `new_command.py:130-131` |
| `new` | `Run: rendercv render …` | `cyan` | `new_command.py:142` |
| `new` | `Get started` panel | `bright_black` | `new_command.py:176` |
| `create-theme` | `Theme created` panel | `bright_black` | `create_theme_command.py:62` |
| `create-theme` | the `design:`/`theme:` snippet | `cyan` | `create_theme_command.py:53-54` |
| command help text | example command | `yellow` | `render_command.py:21`, `new_command.py:23` |
| command help text | `… --help` | `cyan` | `render_command.py:22` |
| override error | `--cv.phone "…"` | `cyan bold` | `render_command.py:195` |
| version warning | whole line | `bold yellow` | `app.py:130-134` — **out of scope, D-003** |

`--help` is a separate inventory; see §4.

### 2.2 What those style names become — measured

Rich resolves a style name to an SGR sequence that depends on the detected colour system. Measured
one process per system (see §2.3 for why that matters), `force_terminal=True`:

| Style | truecolor / 256 | standard (8-colour `TERM`) |
|---|---|---|
| `bright_black` | `ESC[90m` | `ESC[90m` |
| `green` | `ESC[32m` | `ESC[32m` |
| `bold green` | `ESC[1;32m` | `ESC[1;32m` |
| `purple` | `ESC[38;5;129m` | `ESC[35m` |
| `magenta` | `ESC[35m` | `ESC[35m` |
| `cyan` | `ESC[36m` | `ESC[36m` |
| `bold cyan` | `ESC[1;36m` | `ESC[1;36m` |
| `orange4` | `ESC[38;5;94m` | `ESC[33m` |
| `red` | `ESC[31m` | `ESC[31m` |
| `bold red` | `ESC[1;31m` | `ESC[1;31m` |
| `bold yellow` | `ESC[1;33m` | `ESC[1;33m` |
| `dodger_blue3` | `ESC[38;5;26m` | `ESC[94m` |
| `dim` | `ESC[2m` | `ESC[2m` |

Every run is closed with `ESC[0m`; Rich emits no other reset form.

**Three of the styles downgrade** — `purple`, `orange4`, `dodger_blue3` are 8-bit palette entries
(129, 94, 26) and collapse to the nearest standard colour when the system is `standard`. The other
ten are already standard or bold-only and do not move. So a port that hard-codes `ESC[38;5;129m`
is correct on `xterm-256color` and wrong on `xterm`.

### 2.3 A measurement hazard, recorded because it cost a false result

`Style.parse` is memoized and `Style.render` **caches its SGR on the Style object after the first
call** (`rich/style.py:350`, set at `:380`). Measuring several colour systems in one process therefore reports the
*first* system's answer for all of them — it made `purple` read `ESC[35m` under truecolor, which is
wrong. One process per colour system. Any future re-measurement must do the same.

### 2.4 How a style is applied — the run structure, not just the colour

The bytes are not "colour the row". Rich emits one styled run per segment, closing each:

```
ESC[90m│ESC[0m ESC[32m✓ESC[0m ESC[1;32m16 ms   ESC[0m Generated Typst:           ESC[38;5;129m./rendercv_output/John_Doe_CV.typESC[0m …spaces… ESC[90m│ESC[0m
```

Three details a plausible-looking implementation gets wrong:

1. **Each border character is its own run.** The left `│` and the right `│` are separately wrapped;
   the padding between them is unstyled.
2. **The trailing padding of a styled field is inside the run.** `bold green` covers `16 ms   `
   including the three pad spaces, because the padding is applied to the markup'd string before
   rendering (`progress_panel.py:104`, `{step.timing_ms + ' ms':<8}` inside the tags).
3. **A table cell is three runs, not one** — left pad, content, right pad, each opened and closed:
   `ESC[36m ESC[0mESC[36mlocale  ESC[0mESC[36m ESC[0m`. The table's own box characters are
   **unstyled**; only the enclosing panel's border carries `bold red`.

### 2.5 OSC 8 hyperlinks, and a non-deterministic id

`print_welcome.py:22` uses `[link=…]`, which is not SGR at all:

```
ESC]8;id=919290;https://rendercv.comESC\https://rendercv.comESC]8;;ESC\
```

**The `id=` is random per run.** It cannot be byte-compared, and any differential must normalize it.
This is the one styled element whose exact bytes are not reproducible even between two upstream runs.

### 2.6 `rich.print` auto-highlights, so markup is not the whole story

`from rich import print` (`print_welcome.py:3`, `error_handler.py:6`, `app.py:14`) applies Rich's
default `ReprHighlighter` to bare strings. Measured on the welcome line:

```
Welcome to ESC[38;5;26mRenderCV v2.ESC[0mESC[1;38;5;26m8ESC[0m!
```

The `8` is **bold** blue where `v2.` is plain blue — the highlighter matched the number inside the
already-styled span and added `bold`. Panel *contents* are not highlighted (measured: `1.` and `2.`
in `new`'s "Next steps" are plain), so the rule is narrow but it is real, and a port that only
implements the explicit markup will differ on exactly this one character.

---

## 3. Rich's detection rules — measured, in precedence order

Measured by constructing `rich.console.Console` the way rendercv constructs it
(`progress_panel.py:63`, and `rich.print`'s global) over a file whose `isatty()` is controlled, then
confirmed end to end against the real CLI on a PTY.

### 3.1 Is it a terminal? (`rich/console.py:937-984`)

| Condition | Result | Note |
|---|---|---|
| `TTY_COMPATIBLE=0` | **not** a terminal | wins over everything, even a real tty |
| `TTY_COMPATIBLE=1` | is a terminal | even on a pipe |
| `FORCE_COLOR` set and **non-empty** | is a terminal | `FORCE_COLOR=` (empty) is ignored |
| otherwise | `file.isatty()` | |

`CI` is **not** consulted — measured: `CI=true` on a pipe stays uncoloured.

### 3.2 Which colour system? (`rich/console.py:795-817`)

| Condition | System |
|---|---|
| not a terminal, **or** `TERM` in `{dumb, unknown}` | `None` — no SGR at all |
| `COLORTERM` in `{truecolor, 24bit}` | `truecolor` |
| the segment of `TERM` after its last `-` is `256color` or `kitty` | `256` |
| that segment is `16color`, or anything else | `standard` |

`is_dumb_terminal` is `is_terminal and TERM.lower() in ("dumb","unknown")`
(`rich/console.py:985-995`) — so `TERM=dumb` **on a pipe is not dumb**, it is merely not a terminal.
`FORCE_COLOR=1` with `TERM=dumb` on a tty still yields no colour: dumb wins over force.

### 3.3 `NO_COLOR` is not the same switch (`rich/console.py:731-734`, `:2127`)

`no_color = NO_COLOR != ""`. It does **not** disable styling; it calls `Segment.remove_color`, which
strips the colour and **keeps every other attribute**. Measured:

| Env | `bold green` renders |
|---|---|
| — | `ESC[1;32m…ESC[0m` |
| `NO_COLOR=1` | `ESC[1m…ESC[0m` — **bold survives** |
| `NO_COLOR=` (empty) | `ESC[1;32m…ESC[0m` — empty is ignored |
| `TERM=dumb` (tty) | no sequence at all |

End to end this is visible: `NO_COLOR=1 render` is 1845 B with 8 ESC lines, not 0.

### 3.4 Width, which is entangled with detection (`rich/console.py:1011-1045`)

| Condition | Width |
|---|---|
| dumb terminal (tty **and** `TERM=dumb`) | **80, and `COLUMNS` is ignored** |
| otherwise | `COLUMNS` if it is all digits, else `os.get_terminal_size`, else 80 |

**This is a second divergence in the same family.** The port implements "`COLUMNS` wins"
unconditionally (`internal/cli/panel.go:24-31`). Measured on a PTY with `TERM=dumb COLUMNS=100`:
upstream lays out to **80** columns, the port to **100**. It is invisible to the goldens because
they are captured on a pipe, where `is_dumb_terminal` is false and `COLUMNS` does win — so the
corpus's `TERM=dumb` and its `COLUMNS=80` agree by coincidence, not by rule.

### 3.5 `--help` does not obey these rules

Typer builds its own console (`typer/rich_utils.py:140-160`) with its own switches
(`:69-82`):

| Env | Effect on `--help` only |
|---|---|
| `GITHUB_ACTIONS`, `FORCE_COLOR`, `PY_COLORS` (any set) | `force_terminal=True` |
| `_TYPER_FORCE_DISABLE_TERMINAL` | `force_terminal=False` |
| `TERMINAL_WIDTH` | fixed console width |

So `PY_COLORS=1` colours `--help` and nothing else, and `FORCE_COLOR` colours both by two different
mechanisms. A port that implements one detector for the whole binary will differ here.

---

## 4. `--help` — a separate inventory

`--help` is rendered entirely by typer, from constants at `typer/rich_utils.py:29-65`:

| Element | Style | SGR (256) |
|---|---|---|
| `Usage: ` | `yellow` | `ESC[33m` |
| the usage command line | `bold` | `ESC[1m` |
| an option name (`--output-folder`) | `bold cyan` | `ESC[1;36m` |
| a short switch (`-o`) | `bold green` | `ESC[1;32m` |
| a metavar (`PATH`) | `bold yellow` | `ESC[1;33m` |
| metavar separators `[ | ] < >` | `dim` | `ESC[2m` |
| `[required]` | `dim red` | `ESC[2;31m` |
| `*` (required marker) | `red` | `ESC[31m` |
| `[default: …]`, `[env var: …]` | `dim`, `dim yellow` | `ESC[2m`, `ESC[2;33m` |
| Arguments/Options/Commands panel border | `dim` | `ESC[2m` |
| a command name in the Commands table | `bold cyan` | `ESC[1;36m` |
| **usage-error** panel border | `red` — *not* `bold red` | `ESC[31m` |
| `Try 'rendercv new -h' for help.` | `dim` + `blue` | `ESC[2m`, `ESC[2;34m` |

**Two traps.** First, the usage-error panel is plain `red` (`STYLE_ERRORS_PANEL_BORDER`,
`rich_utils.py:65`) while rendercv's own error panel is `bold red` — the port must not unify them.
Second, three `RegexHighlighter`s (`rich_utils.py:106-132`) restyle text *inside prose*: measured,
the `--help` inside the description sentence "Details: rendercv render --help" comes out as
`ESC[1;36m-ESC[0mESC[1;36m-helpESC[0m` — two runs, because the regex captures the leading `-`
separately. Help colouring is pattern-driven, not just structural.

---

## 5. The Live protocol — the largest and least deterministic part

`ProgressPanel` is a `rich.live.Live` (`progress_panel.py:39`) with `refresh_per_second=4`
(`:64`, default at `rich/live.py:64`). On a terminal that means:

1. `ESC[?25l` at start, `ESC[?25h` at stop — cursor hidden for the whole run.
2. Each `update_progress` **erases the previous panel and repaints the whole thing**:
   `\rESC[2K ESC[1A ESC[2K ESC[1A ESC[2K` then the full box again. A five-step render paints the
   panel six times.
3. A refresh **thread** repaints at 4 Hz regardless of progress, so `--watch` writes continuously:
   318 × `ESC[2K` and 272 × `ESC[1A` in 12 idle seconds, 68 KB.

**The byte stream is therefore not deterministic**, even between two upstream runs: back-to-back
captures of the same successful render gave 2963 B / 12 ESC lines and 2178 B / 10, because the
number of repaints depends on how many step boundaries fall inside a 250 ms window. Anything that
compares `--watch` or a progress render byte-for-byte will flake.

This part is a **decision, not a task**: full parity means reproducing a timer-driven repaint loop,
and the payoff is bytes no user reads (the terminal shows only the final frame). §8 recommends
splitting the decision from the colour work and taking it to the human gate.

---

## 6. What can gate this, and what cannot

**The goldens cannot.** They are pipes (§1.1), and making them PTYs would change every one of the 42
committed cases and put a non-deterministic repaint stream into `testdata/golden/` — a contract that
cannot hold. **Do not repoint `gengolden` at a PTY.**

What is needed instead is a **PTY differential**, run like the existing `just` probes rather than
committed as fixtures:

- allocate a PTY (`github.com/creack/pty`, or `script -qec` as this investigation used), run both
  binaries in the *same* directory so absolute paths in messages match, with the environment pinned
  per case;
- normalize before comparing: the OSC 8 `id=` (§2.5), timings, and — for any progress surface —
  collapse the repaint frames to the **final frame** by replaying the erase/cursor-up sequences;
- compare the **style inventory** (the multiset of SGR runs and the text each covers) rather than
  raw bytes, which is exactly what survives the non-determinism of §5;
- drive the env matrix of §3 as cases: default tty, `NO_COLOR=1`, `TERM=dumb`, `FORCE_COLOR=1`,
  `TTY_COMPATIBLE=0`, `TERM=xterm` (the downgrade of §2.2), and `PY_COLORS=1` for `--help`.

### 6.1 Validate the harness before believing a diff

A colour differential produces enormous diffs by construction, so "the port emits no escapes" and
"my capture is lying to me" look identical from outside. **Four runs have already been invalidated
across two agents on this exact surface** — two to a capture-path substitution and two to Rich
buffering `Live` when stdout is not a tty. The protocol that avoids a fifth is a **control case**:
run a configuration in which both sides *must* agree, and require byte-identity before measuring
anything else.

This investigation ran two, and they are the recommended ones:

| Control | Result |
|---|---|
| `TTY_COMPATIBLE=0`, validation-error panel, on a PTY | **byte-identical**, 2235 B both sides |
| `TTY_COMPATIBLE=0`, `new JohnDoe`, on a PTY | identical but for the binary name (§6.3) |

`TTY_COMPATIBLE=0` is the right control switch because it turns *upstream* into the thing the port
already is — not a terminal — so any residue is a real difference and not the finding under test.

### 6.2 Traps that cost a run each

1. **A creating command needs a clean directory per side, not per case.** `new` and `create-theme`
   write files, so the second side sees the first side's output and reports
   `Your YAML input file already exists` — a phantom diff. Wipe the working directory between the
   two sides. Running both in the *same* directory is right for path-in-message parity and wrong
   for this; do both by re-creating the directory, not by reusing it.
2. **Durations are not normalizable on the progress panel.** Upstream and the port take different
   times, and a four-digit `3240 ms` wraps where `0 ms` does not — so the panel differs in **line
   count**, not just in digits, and a `\d+ ms` substitution does not save you: once the token wraps,
   `ms` lands on the next line and the structure has already diverged. **The clean surfaces for a
   style sweep are the deterministic ones** — the validation panel, the `Error` panel, and `new`.
   Establish the rule there; treat the progress panel as measurable only where both sides happen to
   produce the same digit count.
3. **`Live` buffers when stdout is not a tty**, which is why this whole finding is invisible without
   a PTY and why a pipe-based harness reports a plausible-looking, wrong answer rather than an error.

### 6.3 Rebinding the binary name — two sites, not one

`portBinaryName` (`internal/cli/help.go:232`) is **not** the only place the name appears:
`newBanner` hard-codes `"  2. Run: rendercv-go render "` (`internal/cli/new.go:233`). A differential
over `new` that patches only the first chases a phantom.

And **do not rebind by substituting the captured bytes.** Measured on the `new` control: after
`s/rendercv-go/rendercv/`, the text matched but the *padding* did not, because the port had padded
the row to the 11-character name and the substitution shortened the text without fixing the pad. The
same substitution corrupts a capture whose path contains `rendercv-go`. Rebind at the source, and
capture outside the project tree.

---

## 7. The port today — blast radius

**There is no styling capability and no terminal detection at all.** `internal/`, `cmd/` and
`pkg/` contain no `isatty`, no `NO_COLOR`, no escape-sequence literal, and `go.mod` has no terminal
dependency. The port's panels are assembled as plain runes into a `strings.Builder`
(`internal/cli/panel.go:80-128`, `internal/cli/table.go:42-77`).

That is better news than it sounds. The places Rich opens and closes a run correspond one-to-one to
distinct writes the port already makes — border, pad, cell, pad — so styles attach at the write
sites without restructuring.

### 7.1 Build on the width work, not around it

The CLI width sweep landed in exactly these files first, and its mechanics are now load-bearing.
**Upstream's own model is the reason they compose:** Rich never puts style inside the measured
string. A `Text` is a plain string plus a list of `Span(start, end, style)`, and every layout
operation moves the spans to follow the plain text. Three consequences, each matching something the
port already has:

1. **Measure plain, style after.** `cellLen`, `pad`, `cutCells`, `chopCells`, `columnWidths` and
   `truncate` must keep running on plain text; a design that threads escapes through the measured
   string produces wrong widths *everywhere*, silently. This is not a port-side convention — it is
   what `Text` does.
2. **Cropping an escape-carrying string would emit a truncated escape.** The panel title is
   `[bold red]Error[/bold red]`, and `align_text` copies the `Text`, calls `truncate(width)` — which
   assigns `self.plain`, clipping the spans with it (`rich/panel.py:174-178`, `rich/text.py:859-880`)
   — and only *then* stylizes. The port's `panelTop` (`internal/cli/panel.go:143-160`) crops the
   plain band with `cutCells` in the same order. Cropping bytes that already carry SGR would cut a
   sequence in half: a real corruption of the user's terminal, not a cosmetic diff.
3. **The break offsets are the span-slicing offsets.** `Text.divide` slices `self.plain` at
   character offsets and re-attaches each span clipped to the line range (`rich/text.py:1106-1150`).
   The port's `divideLine` already returns rune offsets into the plain string
   (`internal/cli/panel.go:271-320`), and `rstripEnd` (`:322-333`) works on the same runes. **Style
   spans can be sliced with the offsets `divideLine` already returns** — that is the cheap path, and
   it exists only because that branch landed first.

### 7.2 One further constraint

**The detection decision must be made once, at startup, and passed down.** A package-level global
would make the panel and table tests environment-dependent. `Panel` and `Table` are pure
`string`-returning functions today, and that is worth keeping — which argues for a styled variant
that takes the decision as a parameter rather than for mutating the existing signatures.

Surfaces to touch: `panel.go`, `table.go`, `celltable.go`, `helptable.go`, `help.go`, `new.go`,
`render.go`, `customtheme.go`, `watcher.go`. Tests that assert full panel text
(`panel_test.go`, `table_test.go`, `help_test.go`, and the 42 golden cases) must keep passing
**unchanged**, because the golden environment (`NO_COLOR=1`, `TERM=dumb`, pipe) is exactly the one
in which no sequence is emitted. If a golden moves, the detection rule is wrong.

Out of scope by an existing divergence: the `bold yellow` version warning, D-003.

**A note for anyone running the suite from a worktree:** `go test ./...` fails
`TestTypstPackageMetadata` and `TestCorpusIsWellFormed` there unless `RENDERCV_ALLOW_MISSING_INPUT=1`
is set, because the submodule is not checked out. Two agents have now nearly read those as their own
breakage.

---

## 8. Recommended breakdown

**Not one unit, and not five — seven, and one of them is a gate rather than code.** The dependency
runs one way from the harness and the detector; the surfaces after that are leaves and fan out.

| Unit | Content | Depends on |
|---|---|---|
| **A** | PTY differential harness: the §6.1 control cases first, then the §3 env matrix, red against the port on one deterministic surface | — |
| **B** | terminal/colour-system detection and a **span** primitive: §3.1–§3.3, the §2.2 downgrade table, and slicing spans with `divideLine`'s offsets (§7.1) | — |
| C | progress/success panel — `bright_black`, `green`, `bold green`, `purple` (§2.4) | A, B |
| D | error panel + validation table — `bold red`, `cyan`, `magenta`, `orange4`, header `bold`, plus the cropped panel title (§7.1.2) | A, B |
| E | `new` + `create-theme` + welcome, including OSC 8 (§2.5) and the `rich.print` highlighter (§2.6) | A, B |
| F | `--help`, with typer's **separate** console and env rules (§3.5, §4) | A, B |
| G | §3.4's width rule: dumb tty ⇒ 80, `COLUMNS` ignored | B |

A and B are independent of each other and can run in parallel. C, D, E, F never read each other's
output and are the fan-out the graph rules want. G is small, independent of every style, and is
worth landing early because it is a live wrong answer today rather than a missing one.

**The Live repaint protocol (§5) is deliberately not in the table.** It is a decision for the human
gate: reproducing a 4 Hz repaint thread is a large piece of work whose entire output is frames no
user reads, and its byte stream is non-deterministic by construction. The three options are (i)
reproduce it, (ii) paint the final frame only and record a divergence, or (iii) reproduce the
cursor hide/show and per-step repaint but not the idle 4 Hz refresh. **I recommend (iii)** — it
matches what a user sees during a render, keeps `--watch` quiet, and is the only one of the three
whose output can be compared after frame-collapsing. It needs a `specs/divergences.md` entry either
way, which is human-gated (`AGENTS.md` §5).

**On sizing.** B is the only unit with real subtlety, and its rules are all in §3, measured — with
the span model of §7.1 deciding its shape, since spans have to survive `divideLine` and `panelTop`
rather than be re-derived. C, D, E are each a small number of write sites once B exists. F is the
largest of the leaves because of the highlighter regexes. A is the one that must not be skipped:
without it, every later unit is self-reported, and self-reported parity is not parity
(`AGENTS.md` §10.6). D is the best unit to land first among the leaves, because its surface is
deterministic (§6.2.2) and it is therefore the one whose differential can be trusted.

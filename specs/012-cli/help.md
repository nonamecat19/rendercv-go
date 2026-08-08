# Spec 012 §5 — the help renderer

The five `cli_*_help` cases are `TestParity`'s largest remaining block. This file is the
behavior; `specs/012-cli/spec.md` §5 names them as a hazard and stops there.

**Everything below is measured, not read off the goldens.** `STATE.md` records an earlier attempt
that reverse-engineered the geometry from the goldens, got two columns right and the third wrong,
and stopped. The numbers here come from `typer/rich_utils.py` and from `rich.table.Table`'s own
width calculation with `_calculate_column_widths` instrumented while the vendored CLI ran.

Citations are to `third_party/rendercv/.venv/lib/python3.12/site-packages/` unless noted.

---

## 1. Which cases this covers

| Case | Command |
|---|---|
| `cli_help` | `rendercv --help` |
| `cli_help_short` | `rendercv -h` — byte-identical stdout to `cli_help` (spec §1 behavior 3) |
| `cli_render_help` | `rendercv render --help` |
| `cli_new_help` | `rendercv new --help` |
| `cli_create_theme_help` | `rendercv create-theme --help` |

A sixth surface shares the renderer and has no case: **`rendercv` with no arguments at all prints
the root help on stdout and exits 0** (`cli/app.py:41-44` — `ctx.get_help()` then `typer.Exit()`).
The port exits 70 with no output. Closing §5 closes that too.

## 2. The page

`rich_format_help` (`typer/rich_utils.py:535-620`) prints, in order:

1. **The usage line**, as `Padding(highlighter(obj.get_usage(ctx)), 1)` (`:552-554`). A `Padding`
   of 1 is one cell on every side, so it renders as a blank line, then `` Usage: …`` with one
   leading space, then a blank line.
2. **The description**, as `Padding(Align(help_text, pad=False), (0, 1, 1, 1))` (`:559-570`) —
   no top padding, one space left and right, one blank line below. The text wraps to the console
   width minus the two padding cells.
3. **The `Arguments` panel**, if the command has positional parameters (`:588-594`).
4. **The `Options` panel** (`:606` onward).
5. **The `Commands` panel**, for a group only.

Every panel is `rich.panel.Panel(table, title=…, title_align="left")` at the console width. The
port already renders that box (`internal/cli/panel.go`), and its inner content width is 76 at the
pinned `COLUMNS=80`.

**The description is one paragraph with its newlines collapsed.** `_get_help_text` splits on
`\n\n` and, outside markdown mode, replaces every single `\n` with a space (`:270-278`). Both
docstrings in this CLI are one paragraph, so both become one wrapped run.

## 3. The panel tables

`_print_options_panel` (`:348-456`) and `_print_commands_panel` (`:459-532`) both build:

```python
Table(highlight=…, show_header=False, expand=True, box=None,
      show_lines=False, leading=0, pad_edge=False, padding=(0, 1),
      border_style=None, row_styles=None)
```

from `STYLE_OPTIONS_TABLE_*` / `STYLE_COMMANDS_TABLE_*` (`:48-63`). Three differences from the
validation-error table `internal/cli/table.go` already renders:

| | validation table | help table |
|---|---|---|
| box | `ROUNDED` | **none** — no borders, no dividers, no rules |
| `pad_edge` | default `True` | **`False`** — the outer edges lose their padding cell |
| columns | declared, with headers | **undeclared** — `add_row` creates them, so every one is default |

### 3.1 The columns

`_print_options_panel` adds **no columns**; they come into being from the first `add_row`, so all
of them are `width=None, ratio=None, no_wrap=False, overflow="ellipsis"`. Confirmed by
instrumentation. The row is (`:408-421`):

```
[required?] long, short, secondary-long, secondary-short, metavar, help
```

**The `required` column exists only when some parameter is required** (`:422-427`), which is why
an `Arguments` panel has seven columns and an `Options` panel six.

An option's `param.opts` are split on whether the string *contains* `--` (`:364-368`), so a
positional argument — whose only "opt" is its own name — lands in the **short** column, not the
long one. That is why `input_file_name` sits in column 2 of the `Arguments` panel.

`_print_commands_panel` is the exception: it **does** declare its two columns (`:487-496`), the
first `no_wrap=True` with a fixed `width=cmd_len`, the second with `ratio=10`.

### 3.2 Column padding

`padding=(0, 1)` is one cell left and right, and `pad_edge=False` drops it on the two outer
edges. So a column's natural width is its widest cell plus:

- **1** for the first column (right only) and for the last (left only);
- **2** for every column between them.

An empty column is therefore **width 0 with its 2 padding cells still spent** — the reason four
of the six option columns are 2 wide and contribute nothing but space.

### 3.3 The width rule, and the branch that surprised me

The help cell is `Columns(items)` (`:318`), and a `rich.columns.Columns` measures **`(1,
max_width)`** — minimum one cell, maximum the whole console. Its natural width is therefore *not*
the length of the help text. Instrumented:

| Panel | natural | sum | excess | final |
|---|---|---|---|---|
| root `Options` | `[21, 4, 2, 2, 2, 76]` | 107 | 31 | `[21, 4, 2, 2, 2, 45]` |
| `create-theme` `Arguments` | `[2, 2, 12, 2, 2, 6, 76]` | 102 | 26 | `[2, 2, 12, 2, 2, 6, 50]` |
| `create-theme` `Options` | `[7, 4, 2, 2, 2, 76]` | 93 | 17 | `[7, 4, 2, 2, 2, 59]` |
| `new` `Arguments` | `[2, 2, 11, 2, 2, 6, 76]` | 101 | 25 | `[2, 2, 11, 2, 2, 6, 51]` |
| `new` `Options` | `[28, 4, 2, 2, 6, 76]` | 118 | 42 | `[28, 4, 2, 2, 6, 34]` |
| `render` `Arguments` | `[2, 2, 17, 2, 2, 6, 76]` | 107 | 31 | `[2, 2, 17, 2, 2, 6, 45]` |
| `render` `Options` | `[25, 9, 2, 2, 6, 76]` | 120 | 44 | `[25, 9, 2, 2, 6, 32]` |

**The sum always exceeds the width, so `expand=True` never expands anything.** Every panel takes
`Table._collapse_widths` (`rich/table.py:556-561`), which shaves the widest wrappable column
towards the second widest — and since the help column is always the widest by a distance, the
whole excess comes off it alone. In each row above, `final[last] = 76 - excess`.

This is the piece an earlier attempt got wrong, and it is worth stating why it is easy to get
wrong: the table *is* declared `expand=True`, and reasoning forward from that flag leads to
`ratio_distribute` spreading the slack across every column. There is no slack. The flag is dead
code for these tables.

**`internal/cli/table.go` already implements this**: `collapseWidths` and `ratioReduce` are ports
of the same two functions, pinned by three validation-error goldens. What it needs is the boxless,
`pad_edge=False` variant and undeclared columns.

### 3.4 Verifying the model

Column *i*'s content begins at `sum(widths[:i]) + leftpad(i)`, where `leftpad(0) = 0` and
`leftpad(i) = 1`. Against the goldens' measured text offsets:

| Panel | predicted | golden |
|---|---|---|
| root `Options` | 0, 22, 32 | 0, 22, 32 |
| `create-theme` `Options` | 0, 8, 18 | 0, 8, 18 |
| `new` `Options` | 0, 29, 37, 43 | 0, 29, 37, 43 |
| `render` `Options` | 0, 26, 39, 45 | 0, 26, 39, 45 |
| `render` `Arguments` | 0, 5, 26, 32 | 0, 5, 26, 32 |

All five agree, so the rule is settled before any Go is written.

## 4. The help cell is a `Columns`, and it can put two items on one line

`_get_parameter_help` (`:232-318`) returns `Columns` over up to four items, in this order: the
prose help, the env var, the default, and `[required]`. Two of the four occur here.

`rich.columns.Columns` is a flow layout, not a stack, and the goldens show both outcomes:

- `render`'s `Arguments`: `The YAML input file. [required]` — **one line**, both items side by
  side, because the widest item is short enough that two fit in the 44-cell column.
- `new`'s `--theme`: the long prose wraps over six lines and `[default: classic]` lands on its
  **own** line below.

So a naive "join the items with a space" is right for the first and wrong for the second, and a
naive "one item per line" is the reverse. The port needs `Columns`' own rule.

## 5. Acceptance criteria

1. `cli_help` and `cli_help_short` are byte-identical to their goldens, and to each other.
2. `cli_render_help`, `cli_new_help` and `cli_create_theme_help` are byte-identical to theirs.
3. `rendercv-go` with no arguments prints the root help on stdout and exits 0.
4. The seven `(natural, final)` width pairs of §3.3 are reproduced by the Go layout, each pinned
   by a fixture that fails if the collapse rule is replaced by distribution.
5. Both `Columns` outcomes of §4 are pinned: two items on one line, and two items stacked.
6. The `Commands` panel's declared columns — `no_wrap` fixed first, `ratio=10` second — are
   exercised by `cli_help`, whose command column is 14 wide.

## 6. Not in scope

- **Colour.** Every golden is captured with `NO_COLOR=1` and `TERM=dumb`, so no style reaches the
  bytes. The `STYLE_*` constants are recorded above only to explain the structure.
- **`--install-completion` / `--show-completion` actually working.** They appear as rows in the
  root's `Options` panel and nothing more; making them function is not parity for these cases.
- **`create-theme` as a runnable command.** Its help panel needs the command *declared*, with its
  argument and help text; running it is D-008's, still unwritten.

## 7. The binary name re-wraps the prose — HUMAN GATE

**The renderer is finished and three of the five cases still cannot be byte-identical.** Measured
against the goldens, page by page, with the binary name substituted and shortened rows re-padded:

| Page | lines | lines differing |
|---|---:|---:|
| `cli_create_theme_help` | 14 | **0** |
| `cli_help` | 24 | 2 |
| `cli_render_help` | 62 | 2 |
| `cli_new_help` | 36 | 2 |

Every differing line carries the binary name, and `internal/cli/help_test.go` fails if one ever
does not — so the geometry of §3 is confirmed on 130 of 136 lines and the remainder is one cause.

**That cause is D-009 reaching prose.** Every help page quotes commands the reader is meant to
run — `Example: rendercv render John_Doe_CV.yaml` — so the port must print `rendercv-go`, which is
three characters longer. A help page **wraps its prose to the console before the harness sees
it**, so the line breaks somewhere else:

```
golden:  Render a YAML input file. Example: rendercv render John_Doe_CV.yaml. Details:
         rendercv render --help
port:    Render a YAML input file. Example: rendercv-go render John_Doe_CV.yaml.
         Details: rendercv-go render --help
```

`conformance.RebindBinaryName` substitutes the token and re-pads the row it shortened. **Re-padding
cannot undo a re-wrap**, and no substitution applied to finished bytes can.

### Two decisions, and this iteration makes neither

1. **The harness re-pads only bordered rows** (`binaryname.go:47-51`). A help page's `Padding`
   regions — the usage line and the description — are painted to the console width too, so they
   are just as width-sensitive, and `isPanelRow` does not reach them. Extending it is a
   one-line change and it weakens the width check on those lines exactly as D-009 already
   weakened it on panel rows. **Not made here**, because the harness is the instrument every
   other case is measured by. `cli_create_theme_help` turns green on this alone.
2. **The three re-wrapped pages need a `divergences.md` entry**, which is a human gate
   (`AGENTS.md` §5). The options are the same three D-009 weighed for `new`: record the
   divergence and leave them red; print `rendercv` in help prose and hand the reader a command
   that does not work; or teach the harness to re-wrap, which means re-implementing the wrap on
   the comparison side and is worse than either.

**Recommendation: option 1 plus a divergence entry for the rest.** The port's help is *correct* —
it names the binary the reader actually has — and the goldens cannot express that. This is the
same shape as `err_missing_file`: a case unreachable by construction rather than by effort.

### `create-theme` is a fourth blocker and a smaller one

`cli_create_theme_help` also needs the command **registered**, which it is not. Registering it for
its help alone would change `create-theme foo` from `No such command` to something that silently
does nothing, so it lands with the command (D-008), not before it.

// Command helpprobe captures the model behind typer's help pages — the usage
// line, the description, and every option and argument row — into
// internal/cli/helpdata/help.json.
//
// **The rows are data, not logic.** There are seventeen options on `render`
// alone, each with a help string, a metavar and sometimes a default, and
// transcribing them into Go would be a golden by another name (AGENTS.md
// §10.1). This asks the vendored typer for them.
//
// It calls typer's own `_get_parameter_help` rather than reassembling the help
// items, so the prose, the `[default: …]` and the `[required]` come out in
// upstream's order and wording, and a submodule bump moves them here rather
// than leaving the Go side quietly stale.
//
// Each captured string arrives as its plain text plus **the style and the spans
// rich resolved over it** — typer's markup and its three `RegexHighlighter`s,
// already applied. That keeps the port from re-deriving them: the highlighter
// patterns lean on Python's lookahead, and the one place their result is visible
// is the bytes, where a run opened one character off is a real difference
// (spec 012 delta §4).
//
// **What this tool does not capture**: the layout. Widths, padding and wrapping
// are `internal/cli/helptable.go`'s, measured separately and pinned by
// `specs/012-cli/help.md` §3.3. Nor the styles that belong to the *structure*
// rather than to a string — the panel border, the usage block's `bold`, the
// `Commands` table's first column — which are `internal/cli/helpstyle.go`'s.
//
// GENERATED, never hand-edited; regenerate with `just helpprobe`.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	upstreamDir = "third_party/rendercv"
	outPath     = "internal/cli/helpdata/help.json"
)

const helpModelScript = `
import json

import click
import typer.main
from rich.text import Text
from typer.rich_utils import (
    MARKUP_MODE_RICH,
    STYLE_METAVAR,
    _get_help_text,
    _get_parameter_help,
    _make_command_help,
    highlighter,
    metavar_highlighter,
    negative_highlighter,
)

from rendercv.cli.app import app

group = typer.main.get_command(app)


def as_text(renderable):
    """A rich renderable as one Text, spans and all.

    _get_help_text returns a Group -- the first paragraph is styled differently
    from the rest -- so a Group's parts are joined back into one paragraph.
    Text.join shifts each part's spans onto the joined string.
    """
    if hasattr(renderable, "renderables"):
        return Text("\n").join(as_text(part) for part in renderable.renderables)
    if isinstance(renderable, Text):
        return renderable
    return Text(str(renderable))


def styled(renderable):
    """A renderable as its plain text, its own style, and the spans over it.

    **The spans are the whole point.** They are where typer's markup and its
    three RegexHighlighters have already been resolved -- so the port renders
    them rather than re-deriving them from regexes that would have to be ported
    with Python's lookahead semantics intact.

    Offsets are codepoints, which is what Go counts in runes. The order is
    preserved because it decides the colour: rich lets the *last* span covering
    a character win, which is why --help comes out bold cyan (the option span)
    and not bold green (the switch span applied before it).
    """
    text = as_text(renderable)
    return {
        "text": text.plain,
        "style": str(text.style),
        "spans": [
            {"start": span.start, "end": span.end, "style": str(span.style)}
            for span in text.spans
        ],
    }


def help_items(param, ctx):
    # _get_parameter_help returns a Columns; its renderables are the prose, the
    # env var, the default and [required], in that order.
    return [styled(item) for item in _get_parameter_help(
        param=param, ctx=ctx, markup_mode=MARKUP_MODE_RICH
    ).renderables]


def metavar_of(param, ctx):
    # Built the way rich_utils.py:376-399 builds it, because its *style* is part
    # of the answer: a Text of its own carrying STYLE_METAVAR, over which
    # metavar_highlighter dims the separators.
    metavar = Text(style=STYLE_METAVAR, overflow="fold")
    text = param.make_metavar(ctx=ctx)
    # typer replaces a positional's metavar with its type name.
    if isinstance(param, click.Argument) and param.name and text == param.name.upper():
        text = param.type.name.upper()
    # "Skip booleans and choices (handled above)" -- rich_utils.py:387
    if text != "BOOLEAN":
        metavar.append(text)
    return styled(metavar_highlighter(metavar))


def row(param, ctx):
    long_strs, short_strs = [], []
    for opt in param.opts:
        (long_strs if "--" in opt else short_strs).append(opt)
    secondary_long, secondary_short = [], []
    for opt in param.secondary_opts:
        (secondary_long if "--" in opt else secondary_short).append(opt)
    return {
        "long": styled(highlighter(",".join(long_strs))),
        "short": styled(highlighter(",".join(short_strs))),
        "secondary_long": styled(negative_highlighter(",".join(secondary_long))),
        "secondary_short": styled(negative_highlighter(",".join(secondary_short))),
        "metavar": metavar_of(param, ctx),
        "help": help_items(param, ctx),
        "required": bool(param.required),
    }


def describe(command, ctx):
    arguments, options = [], []
    for param in command.get_params(ctx):
        if getattr(param, "hidden", False):
            continue
        if isinstance(param, click.Argument):
            arguments.append(row(param, ctx))
        elif isinstance(param, click.Option):
            options.append(row(param, ctx))

    # **Through typer's own _get_help_text, not command.help.** The raw
    # docstring carries rich markup -- create-theme's is
    # "Example: [yellow]rendercv create-theme customtheme[/yellow]" -- which
    # arrives here as a span rather than as text. _get_help_text also folds
    # single newlines away.
    description = styled(Text(""))
    if command.help:
        description = styled(_get_help_text(obj=command, markup_mode=MARKUP_MODE_RICH))

    return {
        # Through the highlighter, which is what puts the usage span over
        # the leading "Usage: " (rich_utils.py:552-554).
        "usage": styled(highlighter(command.get_usage(ctx))),
        "description": description,
        "arguments": arguments,
        "options": options,
    }


# The app sets help_option_names=["-h", "--help"] in its context_settings
# (cli/app.py:22-25). click applies those when it builds a context itself, so a
# hand-made one must pass them or every --help row loses its -h.
root_ctx = click.Context(group, info_name="rendercv", **group.context_settings)
model = {"root": describe(group, root_ctx)}

model["root"]["subcommands"] = [
    {
        "name": name,
        "help": styled(_make_command_help(
            help_text=command.short_help or command.help or "",
            markup_mode=MARKUP_MODE_RICH,
        )),
    }
    for name, command in sorted(group.commands.items())
]

model["commands"] = {}
for name, command in sorted(group.commands.items()):
    # help_option_names is inherited from the parent context.
    ctx = click.Context(command, info_name=name, parent=root_ctx, **command.context_settings)
    model["commands"][name] = describe(command, ctx)

print(json.dumps(model, indent=2, ensure_ascii=False))
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "helpprobe: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	command := exec.Command("uv", "run", "--frozen", "--all-extras", "python", "-c", helpModelScript)
	command.Dir = upstreamDir
	command.Stderr = os.Stderr

	out, err := command.Output()
	if err != nil {
		return fmt.Errorf("running uv (is the submodule initialized? `just setup`): %w", err)
	}

	// Reject anything that is not JSON before it reaches the tree: a warning
	// printed on stdout by a dependency would otherwise be committed as data.
	var parsed any
	if err := json.Unmarshal(out, &parsed); err != nil {
		return fmt.Errorf("upstream did not print JSON: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outPath, append(out, '\n'), 0o644); err != nil { //nolint:gosec // committed data
		return err
	}
	fmt.Printf("wrote %s (%d bytes)\n", outPath, len(out))
	return nil
}

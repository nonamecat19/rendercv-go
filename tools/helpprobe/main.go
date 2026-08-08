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
// **What this tool does not capture**: the layout. Widths, padding and wrapping
// are `internal/cli/helptable.go`'s, measured separately and pinned by
// `specs/012-cli/help.md` §3.3.
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
from typer.rich_utils import (
    MARKUP_MODE_RICH,
    _get_help_text,
    _get_parameter_help,
    _make_command_help,
)

from rendercv.cli.app import app

group = typer.main.get_command(app)


def plain(renderable):
    """The text of a rich renderable, with no styling.

    _get_help_text returns a Group -- the first line is styled differently from
    the rest -- so a Group's parts are joined back into one paragraph.
    """
    if hasattr(renderable, "renderables"):
        return "\n".join(plain(part) for part in renderable.renderables)
    if hasattr(renderable, "plain"):
        return renderable.plain
    return str(renderable)


def help_items(param, ctx):
    # _get_parameter_help returns a Columns; its renderables are the prose, the
    # env var, the default and [required], in that order.
    return [plain(item) for item in _get_parameter_help(
        param=param, ctx=ctx, markup_mode=MARKUP_MODE_RICH
    ).renderables]


def metavar_of(param, ctx):
    text = param.make_metavar(ctx=ctx)
    # typer replaces a positional's metavar with its type name.
    if isinstance(param, click.Argument) and param.name and text == param.name.upper():
        text = param.type.name.upper()
    # "Skip booleans and choices (handled above)" -- rich_utils.py:387
    return "" if text == "BOOLEAN" else text


def row(param, ctx):
    long_strs, short_strs = [], []
    for opt in param.opts:
        (long_strs if "--" in opt else short_strs).append(opt)
    secondary_long, secondary_short = [], []
    for opt in param.secondary_opts:
        (secondary_long if "--" in opt else secondary_short).append(opt)
    return {
        "long": ",".join(long_strs),
        "short": ",".join(short_strs),
        "secondary_long": ",".join(secondary_long),
        "secondary_short": ",".join(secondary_short),
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
    # "Example: [yellow]rendercv create-theme customtheme[/yellow]" -- and the
    # goldens have it stripped. _get_help_text also folds single newlines away.
    description = ""
    if command.help:
        description = plain(_get_help_text(obj=command, markup_mode=MARKUP_MODE_RICH))

    return {
        "usage": command.get_usage(ctx),
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
        "help": plain(_make_command_help(
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

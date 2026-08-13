package cli_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/cli"
)

// TestHelpTerminalIsTypersOwn drives spec 012 delta §3.5: `--help` is rendered
// through a console typer builds itself, with three variables Rich never reads
// and one that overrides them.
//
// Each row is the pair of answers — the help page's and the rest of the CLI's —
// so a port that wired one detector to both fails on the rows where they differ.
func TestHelpTerminalIsTypersOwn(t *testing.T) {
	tests := []struct {
		name       string
		env        map[string]string
		isatty     bool
		help, rest bool
	}{
		{name: "a pipe is a pipe for both"},
		{name: "a tty is a tty for both", isatty: true, help: true, rest: true},
		{
			name: "PY_COLORS forces the help page alone",
			env:  map[string]string{"PY_COLORS": "1"}, help: true,
		},
		{
			name: "GITHUB_ACTIONS forces the help page alone",
			env:  map[string]string{"GITHUB_ACTIONS": "true"}, help: true,
		},
		{
			// FORCE_COLOR reaches both, by two different mechanisms: typer's
			// force_terminal and Rich's own rule.
			name: "FORCE_COLOR forces both",
			env:  map[string]string{"FORCE_COLOR": "1"}, help: true, rest: true,
		},
		{
			// `getenv` in a boolean context, so an empty value does not force —
			// and Rich reads set-but-empty as "not a terminal".
			name: "FORCE_COLOR set but empty forces neither",
			env:  map[string]string{"FORCE_COLOR": ""}, isatty: true,
		},
		{
			name:   "_TYPER_FORCE_DISABLE_TERMINAL silences the help page alone",
			env:    map[string]string{"_TYPER_FORCE_DISABLE_TERMINAL": "1"},
			isatty: true, rest: true,
		},
		{
			// A forced value is returned before `is_terminal` reads any variable
			// at all (`rich/console.py:944-945`), so typer's force outranks the
			// switch that outranks everything else.
			name: "PY_COLORS outranks TTY_COMPATIBLE for the help page",
			env:  map[string]string{"PY_COLORS": "1", "TTY_COMPATIBLE": "0"}, help: true,
		},
		{
			// Dumbness is not part of the force: it is read off the same
			// environment by the same console code.
			name: "TERM=dumb beats a forced help terminal",
			env:  map[string]string{"PY_COLORS": "1", "TERM": "dumb"}, help: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := environment(test.env)
			help := cli.DetectHelpTerminal(env, test.isatty)
			rest := cli.DetectTerminal(env, test.isatty)

			if help.IsTerminal != test.help {
				t.Errorf("--help is a terminal = %v, want %v", help.IsTerminal, test.help)
			}
			if rest.IsTerminal != test.rest {
				t.Errorf("the rest of the CLI is a terminal = %v, want %v", rest.IsTerminal, test.rest)
			}
			// The colour system follows from the terminal question and the same
			// TERM, so a dumb row is colourless on both sides however it was
			// forced.
			if dumb, _ := env("TERM"); dumb == "dumb" && help.System != cli.ColorNone {
				t.Errorf("a dumb terminal has colour system %v", help.System)
			}
		})
	}
}

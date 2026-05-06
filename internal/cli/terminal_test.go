package cli_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/cli"
)

// environment turns a map into a cli.Environment, distinguishing "set to the
// empty string" from "not set" — a distinction both `NO_COLOR` and
// `FORCE_COLOR` turn on.
func environment(values map[string]string) cli.Environment {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

// TestDetectTerminal pins spec 012 delta §3.1-§3.3. Every row was measured
// against the vendored Rich by constructing a `Console` the way RenderCV
// constructs one and reading back `is_terminal`, `color_system` and `no_color`.
func TestDetectTerminal(t *testing.T) {
	tests := []struct {
		name   string
		env    map[string]string
		isatty bool
		want   cli.Terminal
	}{
		{
			name: "a plain tty with truecolor", isatty: true,
			env:  map[string]string{"TERM": "xterm-256color", "COLORTERM": "truecolor"},
			want: cli.Terminal{IsTerminal: true, System: cli.ColorTruecolor},
		},
		{
			name: "a tty without COLORTERM is 256 by its TERM suffix", isatty: true,
			env:  map[string]string{"TERM": "xterm-256color"},
			want: cli.Terminal{IsTerminal: true, System: cli.ColorEightBit},
		},
		{
			name: "an eight colour TERM", isatty: true,
			env:  map[string]string{"TERM": "xterm"},
			want: cli.Terminal{IsTerminal: true, System: cli.ColorStandard},
		},
		{
			name: "TERM unset is still standard", isatty: true,
			want: cli.Terminal{IsTerminal: true, System: cli.ColorStandard},
		},
		{
			name: "kitty is a 256 colour suffix", isatty: true,
			env:  map[string]string{"TERM": "xterm-kitty"},
			want: cli.Terminal{IsTerminal: true, System: cli.ColorEightBit},
		},
		{
			name: "a 24bit COLORTERM", isatty: true,
			env:  map[string]string{"TERM": "xterm", "COLORTERM": "24bit"},
			want: cli.Terminal{IsTerminal: true, System: cli.ColorTruecolor},
		},

		// A dumb terminal is a terminal with no colour at all — not the same as
		// not being a terminal, which is why both fields are asserted.
		{
			name: "TERM=dumb", isatty: true,
			env:  map[string]string{"TERM": "dumb"},
			want: cli.Terminal{IsTerminal: true, System: cli.ColorNone},
		},
		{
			name: "TERM=unknown is dumb too", isatty: true,
			env:  map[string]string{"TERM": "unknown"},
			want: cli.Terminal{IsTerminal: true, System: cli.ColorNone},
		},
		{
			name: "dumb wins over FORCE_COLOR", isatty: true,
			env:  map[string]string{"TERM": "dumb", "FORCE_COLOR": "1"},
			want: cli.Terminal{IsTerminal: true, System: cli.ColorNone},
		},
		{
			name: "TERM=dumb on a pipe is not dumb, merely not a terminal",
			env:  map[string]string{"TERM": "dumb"},
			want: cli.Terminal{IsTerminal: false, System: cli.ColorNone},
		},

		// The forcing switches, in precedence order.
		{
			name: "TTY_COMPATIBLE=0 beats a real tty", isatty: true,
			env:  map[string]string{"TERM": "xterm-256color", "TTY_COMPATIBLE": "0"},
			want: cli.Terminal{IsTerminal: false, System: cli.ColorNone},
		},
		{
			name: "TTY_COMPATIBLE=1 beats a pipe",
			env:  map[string]string{"TERM": "xterm-256color", "TTY_COMPATIBLE": "1"},
			want: cli.Terminal{IsTerminal: true, System: cli.ColorEightBit},
		},
		{
			name: "TTY_COMPATIBLE=0 beats FORCE_COLOR",
			env:  map[string]string{"TERM": "xterm-256color", "TTY_COMPATIBLE": "0", "FORCE_COLOR": "1"},
			want: cli.Terminal{IsTerminal: false, System: cli.ColorNone},
		},
		{
			name: "FORCE_COLOR on a pipe",
			env:  map[string]string{"TERM": "xterm-256color", "FORCE_COLOR": "1"},
			want: cli.Terminal{IsTerminal: true, System: cli.ColorEightBit},
		},
		{
			name: "FORCE_COLOR set but empty does not force", isatty: false,
			env:  map[string]string{"TERM": "xterm-256color", "FORCE_COLOR": ""},
			want: cli.Terminal{IsTerminal: false, System: cli.ColorNone},
		},
		{
			name: "FORCE_COLOR set but empty turns a real tty off", isatty: true,
			env:  map[string]string{"TERM": "xterm-256color", "FORCE_COLOR": ""},
			want: cli.Terminal{IsTerminal: false, System: cli.ColorNone},
		},
		{
			name: "CI is not consulted",
			env:  map[string]string{"TERM": "xterm-256color", "CI": "true"},
			want: cli.Terminal{IsTerminal: false, System: cli.ColorNone},
		},

		// NO_COLOR is a colour switch, not a terminal one: both other fields
		// stay exactly as they were.
		{
			name: "NO_COLOR", isatty: true,
			env:  map[string]string{"TERM": "xterm-256color", "NO_COLOR": "1"},
			want: cli.Terminal{IsTerminal: true, System: cli.ColorEightBit, NoColor: true},
		},
		{
			name: "NO_COLOR set but empty is ignored", isatty: true,
			env:  map[string]string{"TERM": "xterm-256color", "NO_COLOR": ""},
			want: cli.Terminal{IsTerminal: true, System: cli.ColorEightBit},
		},
		{
			name: "NO_COLOR with FORCE_COLOR is still no colour", isatty: true,
			env:  map[string]string{"TERM": "xterm-256color", "NO_COLOR": "1", "FORCE_COLOR": "1"},
			want: cli.Terminal{IsTerminal: true, System: cli.ColorEightBit, NoColor: true},
		},

		{
			name: "a pipe",
			env:  map[string]string{"TERM": "xterm-256color", "COLORTERM": "truecolor"},
			want: cli.Terminal{IsTerminal: false, System: cli.ColorNone},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := cli.DetectTerminal(environment(test.env), test.isatty)
			if got != test.want {
				t.Errorf("DetectTerminal() = %+v, want %+v", got, test.want)
			}
		})
	}
}

// TestConsoleWidthForDumbTerminals pins spec 012 delta §3.4, the width rule that
// is entangled with detection: a **dumb terminal is 80 and ignores `COLUMNS`**
// (`rich/console.py:1018-1019`), where everything else honours it.
//
// Every row was measured by running the vendored CLI and counting the runes on
// the panel's top border — on a pty through `script` for the terminal rows, and
// through a pipe for the rest. The two `TTY_COMPATIBLE` rows are the ones that
// show the rule composes through `is_terminal` rather than through `isatty`:
// forced on, a pipe is dumb; forced off, a real dumb pty is not.
func TestConsoleWidthForDumbTerminals(t *testing.T) {
	tests := []struct {
		name   string
		env    map[string]string
		isatty bool
		want   int
	}{
		{
			name: "a dumb tty ignores COLUMNS", isatty: true,
			env: map[string]string{"TERM": "dumb", "COLUMNS": "100"}, want: 80,
		},
		{
			name: "a dumb tty ignores a COLUMNS narrower than 80", isatty: true,
			env: map[string]string{"TERM": "dumb", "COLUMNS": "37"}, want: 80,
		},
		{
			// `TERM` is lowercased before the comparison, so a shell that
			// exports it shouting is still dumb.
			name: "TERM is matched case-insensitively", isatty: true,
			env: map[string]string{"TERM": "DUMB", "COLUMNS": "100"}, want: 80,
		},
		{
			name: "TERM=unknown is dumb too", isatty: true,
			env: map[string]string{"TERM": "unknown", "COLUMNS": "100"}, want: 80,
		},
		{
			name: "TERM=dumb on a pipe is not dumb, merely not a terminal",
			env:  map[string]string{"TERM": "dumb", "COLUMNS": "100"}, want: 100,
		},
		{
			name: "TTY_COMPATIBLE=1 makes a dumb pipe dumb",
			env:  map[string]string{"TERM": "dumb", "COLUMNS": "100", "TTY_COMPATIBLE": "1"},
			want: 80,
		},
		{
			name: "FORCE_COLOR makes a dumb pipe dumb",
			env:  map[string]string{"TERM": "dumb", "COLUMNS": "100", "FORCE_COLOR": "1"},
			want: 80,
		},
		{
			name: "TTY_COMPATIBLE=0 takes the dumbness off a dumb tty", isatty: true,
			env:  map[string]string{"TERM": "dumb", "COLUMNS": "100", "TTY_COMPATIBLE": "0"},
			want: 100,
		},
		{
			name: "an ordinary tty honours COLUMNS", isatty: true,
			env: map[string]string{"TERM": "xterm-256color", "COLUMNS": "100"}, want: 100,
		},
		{
			name: "a dumb tty with no COLUMNS is 80 as well", isatty: true,
			env: map[string]string{"TERM": "dumb"}, want: 80,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := cli.ConsoleWidthFor(environment(test.env), test.isatty); got != test.want {
				t.Errorf("ConsoleWidthFor() = %d, want %d", got, test.want)
			}
		})
	}
}

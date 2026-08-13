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
		// size is the window the OS reports for the first descriptor that can
		// answer it. A nil one is the pipe rows, where every ioctl fails.
		size cli.TerminalSize
		want int
	}{
		{
			name: "a dumb tty ignores COLUMNS", isatty: true, size: window(120),
			env: map[string]string{"TERM": "dumb", "COLUMNS": "100"}, want: 80,
		},
		{
			name: "a dumb tty ignores a COLUMNS narrower than 80", isatty: true, size: window(120),
			env: map[string]string{"TERM": "dumb", "COLUMNS": "37"}, want: 80,
		},
		{
			// `TERM` is lowercased before the comparison, so a shell that
			// exports it shouting is still dumb.
			name: "TERM is matched case-insensitively", isatty: true, size: window(120),
			env: map[string]string{"TERM": "DUMB", "COLUMNS": "100"}, want: 80,
		},
		{
			name: "TERM=unknown is dumb too", isatty: true, size: window(120),
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
			name: "TTY_COMPATIBLE=0 takes the dumbness off a dumb tty", isatty: true, size: window(120),
			env:  map[string]string{"TERM": "dumb", "COLUMNS": "100", "TTY_COMPATIBLE": "0"},
			want: 100,
		},
		{
			name: "an ordinary tty honours COLUMNS", isatty: true, size: window(120),
			env: map[string]string{"TERM": "xterm-256color", "COLUMNS": "100"}, want: 100,
		},
		{
			name: "a dumb tty with no COLUMNS is 80 as well", isatty: true, size: window(120),
			env: map[string]string{"TERM": "dumb"}, want: 80,
		},
		{
			// The dumb short-circuit is beaten by `COLUMNS` **and** `LINES`
			// together, which Rich reads at construction into `_width`/`_height`
			// and returns before it asks whether the terminal is dumb
			// (`rich/console.py:690-697`, `:1018-1019`). Measured on a
			// 120-column pty: 100.
			name: "COLUMNS with LINES outranks even a dumb tty", isatty: true, size: window(120),
			env:  map[string]string{"TERM": "dumb", "COLUMNS": "100", "LINES": "40"},
			want: 100,
		},
		{
			// …and a `LINES` that is not digits does not unlock it. Measured: 80.
			name: "a non-numeric LINES leaves the dumb tty at 80", isatty: true, size: window(120),
			env:  map[string]string{"TERM": "dumb", "COLUMNS": "100", "LINES": "abc"},
			want: 80,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := cli.ConsoleWidthFor(environment(test.env), test.isatty, test.size)
			if got != test.want {
				t.Errorf("ConsoleWidthFor() = %d, want %d", got, test.want)
			}
		})
	}
}

// window is a `cli.TerminalSize` that answers with the given width. `window(0)`
// is the pty whose window size was never set — a **successful** ioctl reporting
// zero columns, which is not the same answer as a nil `cli.TerminalSize`, where
// every ioctl raised.
func window(width int) cli.TerminalSize {
	return func() (int, bool) { return width, true }
}

// TestConsoleWidthForRealTerminals pins the other half of `Console.size`: with
// `COLUMNS` unset, Rich lays out to the **real window**
// (`rich/console.py:1027-1034`), where the port printed 80 into every terminal
// it was ever run in.
//
// Every row is a live measurement of the vendored `rendercv new` through a pty
// whose window size was set with `TIOCSWINSZ`, counting the runes on the
// `Useful Links` panel's top border after stripping SGR and OSC 8.
func TestConsoleWidthForRealTerminals(t *testing.T) {
	tests := []struct {
		name   string
		env    map[string]string
		isatty bool
		size   cli.TerminalSize
		want   int
	}{
		{
			name: "a 120 column window with COLUMNS unset", isatty: true, size: window(120),
			env: map[string]string{"TERM": "xterm-256color"}, want: 120,
		},
		{
			name: "a 63 column window", isatty: true, size: window(63),
			env: map[string]string{"TERM": "xterm-256color"}, want: 63,
		},
		{
			name: "a 200 column window", isatty: true, size: window(200),
			env: map[string]string{"TERM": "xterm-256color"}, want: 200,
		},
		{
			name: "COLUMNS overrides the window it was measured in", isatty: true, size: window(120),
			env: map[string]string{"TERM": "xterm-256color", "COLUMNS": "57"}, want: 57,
		},
		{
			// Rich's guard is `str.isdigit`, not `int()`: a value that is not
			// digits leaves the window size **alone** rather than falling back
			// to 80. The port used to answer 80 to all three of these.
			name: "a non-numeric COLUMNS leaves the window size alone", isatty: true, size: window(120),
			env: map[string]string{"TERM": "xterm-256color", "COLUMNS": "abc"}, want: 120,
		},
		{
			name: "a negative COLUMNS leaves the window size alone", isatty: true, size: window(120),
			env: map[string]string{"TERM": "xterm-256color", "COLUMNS": "-5"}, want: 120,
		},
		{
			name: "a space before the digits is not digits", isatty: true, size: window(120),
			env: map[string]string{"TERM": "xterm-256color", "COLUMNS": " 57"}, want: 120,
		},
		{
			name: "a leading zero is still digits", isatty: true, size: window(120),
			env: map[string]string{"TERM": "xterm-256color", "COLUMNS": "057"}, want: 57,
		},
		{
			// The fd order is `(stdin, stdout, stderr)` (`rich/console.py:98`)
			// and the loop breaks on the first ioctl that does not raise — so a
			// pty on **stdin** sets the width even though stdout is a pipe and
			// nothing is coloured. Measured: 120.
			name: "a pty on stdin sets the width for a piped stdout", isatty: false, size: window(120),
			env: map[string]string{"TERM": "xterm-256color"}, want: 120,
		},
		{
			name: "no descriptor can answer, so 80", isatty: false, size: nil,
			env: map[string]string{"TERM": "xterm-256color"}, want: 80,
		},
		{
			// A pty whose window size was never set answers 0 successfully, and
			// Rich folds that to 80 (`:1043-1044`). Measured: 80.
			name: "a window of zero columns folds to 80", isatty: true, size: window(0),
			env: map[string]string{"TERM": "xterm-256color"}, want: 80,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := cli.ConsoleWidthFor(environment(test.env), test.isatty, test.size)
			if got != test.want {
				t.Errorf("ConsoleWidthFor() = %d, want %d", got, test.want)
			}
		})
	}
}

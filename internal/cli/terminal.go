package cli

import (
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// TerminalFor is the detection `rich.console.Console` performs for the file it
// was handed (`rich/console.py:937-984`), asked of the writer the command is
// printing through rather than of the process's own stdout.
//
// **Asking the writer is what keeps package `cli` testable.** Every test in it
// passes a `bytes.Buffer`, which is not a file and therefore not a terminal —
// the same answer Rich gives for a pipe, and the answer every golden was
// captured under.
//
// The environment is still consulted for a non-file writer, because `FORCE_COLOR`
// and `TTY_COMPATIBLE` outrank `isatty` in Rich's own order and a user who sets
// them means it.
func TerminalFor(writer io.Writer) Terminal {
	file, ok := writer.(*os.File)
	return DetectTerminal(os.LookupEnv, ok && term.IsTerminal(int(file.Fd())))
}

// TerminalForHelp is the same detection for **typer's** console, which is a
// different console with different switches (spec 012 delta §3.5).
//
// `--help` is not printed through RenderCV's console at all: `rich_format_help`
// builds its own (`typer/rich_utils.py:140-158`) with `force_terminal` decided
// at import time from three environment variables Rich itself never reads
// (`:75-82`). So `PY_COLORS=1` colours the help page and nothing else, and
// `_TYPER_FORCE_DISABLE_TERMINAL` uncolours the help page and nothing else — a
// port with one detector for the whole binary is wrong on both.
func TerminalForHelp(writer io.Writer) Terminal {
	file, ok := writer.(*os.File)
	return DetectHelpTerminal(os.LookupEnv, ok && term.IsTerminal(int(file.Fd())))
}

// DetectHelpTerminal is `DetectTerminal` under typer's `FORCE_TERMINAL`.
//
// **A forced value outranks `TTY_COMPATIBLE`**, where in Rich's own order
// `TTY_COMPATIBLE` outranks everything: `is_terminal` returns `self._force_terminal`
// before it reads any variable at all (`rich/console.py:944-945`). So
// `PY_COLORS=1 TTY_COMPATIBLE=0` leaves `--help` coloured and the rest of the
// CLI plain.
//
// What it does **not** change is dumbness or `NO_COLOR`: both are read off the
// same environment by the same console code, so `TERM=dumb` still wins over a
// forced terminal and `NO_COLOR=1` still keeps the attributes.
func DetectHelpTerminal(env Environment, isatty bool) Terminal {
	forced, ok := typerForceTerminal(env)
	if !ok {
		return DetectTerminal(env, isatty)
	}
	noColorValue, _ := env("NO_COLOR")
	return Terminal{
		IsTerminal: forced,
		System:     detectColorSystem(env, forced),
		NoColor:    noColorValue != "",
	}
}

// typerForceTerminal is typer's module-level `FORCE_TERMINAL`
// (`typer/rich_utils.py:75-82`), which is a tri-state: forced on, forced off, or
// unset and left to Rich.
//
// The three that force it on are read with `getenv` in a boolean context, so an
// **empty** value does not force — and `FORCE_COLOR=` then falls through to
// Rich's own rule, which reads set-but-empty as "not a terminal". The disabling
// variable is checked afterwards and overrides all three.
func typerForceTerminal(env Environment) (forced, ok bool) {
	if disable, _ := env("_TYPER_FORCE_DISABLE_TERMINAL"); disable != "" {
		return false, true
	}
	for _, name := range []string{"GITHUB_ACTIONS", "FORCE_COLOR", "PY_COLORS"} {
		if value, _ := env(name); value != "" {
			return true, true
		}
	}
	return false, false
}

// ColorSystem is Rich's `ColorSystem` (`rich/console.py:88-97`): how much
// colour the terminal is believed to understand, which decides the SGR form a
// style is written in.
type ColorSystem uint8

// The systems Rich distinguishes on a POSIX terminal. `windows` is not here:
// it is Rich's legacy-console path (`rich/console.py:970-978`) and the port has
// no Windows console support to mirror it with.
const (
	// ColorNone emits no colour at all — not a terminal, or a dumb one.
	ColorNone ColorSystem = iota
	// ColorStandard is the sixteen ANSI colours.
	ColorStandard
	// ColorEightBit is the 256-colour palette, `ESC[38;5;N`.
	ColorEightBit
	// ColorTruecolor is 24-bit colour. Rich writes the palette colours
	// RenderCV names identically to ColorEightBit, so nothing here
	// distinguishes the two — the value exists so a future 24-bit style has a
	// system to name.
	ColorTruecolor
)

// Terminal is the decision `rich.console.Console` makes when it is constructed
// — whether it is writing to a terminal, how much colour that terminal has, and
// whether `NO_COLOR` is in force.
//
// **It is a value, deliberately, and it is passed down.** A package-level global
// would make every panel and table test depend on the environment the suite
// happens to run in.
type Terminal struct {
	// IsTerminal mirrors `Console.is_terminal` (`rich/console.py:937-984`).
	IsTerminal bool
	// System mirrors `Console._detect_color_system` (`:795-817`).
	System ColorSystem
	// NoColor mirrors `Console.no_color` (`:731-734`). It is **not** the same
	// switch as `System == ColorNone`: it strips colour and keeps every other
	// attribute, so `bold green` under `NO_COLOR=1` is still bold.
	NoColor bool
}

// Environment is a lookup of environment variables, so detection can be tested
// without touching the process's own.
type Environment func(name string) (value string, set bool)

// DetectTerminal reproduces Rich's construction-time detection — spec 012 delta
// §3, every rule of which was measured against the vendored Rich rather than
// read off its source.
//
// The three questions are answered in order, because the later ones read the
// earlier ones: a dumb terminal is only dumb if it is a terminal at all, and the
// colour system is `None` unless it is one.
func DetectTerminal(env Environment, isatty bool) Terminal {
	terminal := isTerminal(env, isatty)
	noColorValue, _ := env("NO_COLOR")
	return Terminal{
		IsTerminal: terminal,
		System:     detectColorSystem(env, terminal),
		// `NO_COLOR` is true for any **non-empty** value, so `NO_COLOR=` is
		// ignored — measured, and the empty case is the one a shell produces by
		// accident.
		NoColor: noColorValue != "",
	}
}

// isTerminal is `Console.is_terminal` (`rich/console.py:937-984`), in its
// precedence order.
//
// **`CI` is not consulted.** Measured: `CI=true` on a pipe stays uncoloured,
// which is worth stating because several tools do treat `CI` as a colour hint
// and a reader would reasonably expect it here.
func isTerminal(env Environment, isatty bool) bool {
	// `TTY_COMPATIBLE` wins over everything, a real tty included.
	if compatible, _ := env("TTY_COMPATIBLE"); compatible == "0" {
		return false
	} else if compatible == "1" {
		return true
	}
	// https://force-color.org/ — set but **empty** means "not forced", and
	// falls through to the tty check rather than forcing it off.
	if force, set := env("FORCE_COLOR"); set {
		return force != ""
	}
	return isatty
}

// isDumbTerminal is `Console.is_dumb_terminal` (`rich/console.py:985-995`):
// dumb **and** a terminal. On a pipe `TERM=dumb` is not dumb, it is merely not
// a terminal — which is why the golden corpus can pin `TERM=dumb` and still get
// upstream's ordinary width.
func isDumbTerminal(env Environment, terminal bool) bool {
	term, _ := env("TERM")
	switch strings.ToLower(term) {
	case "dumb", "unknown":
		return terminal
	}
	return false
}

// TerminalSize is `os.get_terminal_size` over the file descriptors Rich probes
// (`rich/console.py:1027-1034`), collapsed to the one number the port lays out
// to.
//
// It reports **`ok` for a successful ioctl, whatever it returned** — including
// the zero width a pseudo-terminal with an unset window size answers with,
// which Rich takes and only folds away later (`:1043-1044`). "Failed" and
// "succeeded with 0" are different answers because `COLUMNS` is read between
// them.
type TerminalSize func() (width int, ok bool)

// ConsoleWidthFor is the width half of `Console.size`
// (`rich/console.py:1011-1049`) plus the half of it that lives in
// `Console.__init__` (`:690-697`) — spec 012 delta §3.4.
//
// The order below is Rich's, and every step of it was measured on a pty
// against the vendored Python rather than read off the source.
//
// **`COLUMNS` *and* `LINES` together outrank even a dumb terminal.** Rich reads
// both at construction into `_width`/`_height` (`:690-697`), and `size` returns
// them before the dumb short-circuit if *both* are set (`:1018-1019`). Measured
// on a 120-column pty: `TERM=dumb COLUMNS=100 LINES=40` is 100, while
// `TERM=dumb COLUMNS=100 LINES=abc` is 80. The pair is what unlocks it; a lone
// `COLUMNS` does not.
//
// **A dumb terminal is otherwise 80 and `COLUMNS` is ignored** (`:1021-1022`),
// and dumbness composes through `is_terminal`, not through `isatty` alone — so
// `TTY_COMPATIBLE=1 TERM=dumb` on a *pipe* is 80 and `TTY_COMPATIBLE=0
// TERM=dumb` on a *pty* is the width its `COLUMNS` asks for (both measured).
// That is why the golden corpus can pin `TERM=dumb` with `COLUMNS=80` and see
// neither rule: through a pipe nothing is dumb, and its `COLUMNS` agrees with
// the fallback by coincidence.
//
// **Then the real window size, then `COLUMNS` on top of it.** Rich asks the OS
// first and lets `COLUMNS` overwrite the answer (`:1027-1038`), which matters
// for the values `COLUMNS` cannot be: it guards with `str.isdigit`, so
// `COLUMNS=abc` and `COLUMNS=-5` are *not* "fall back to 80", they are "leave
// the window size alone". Measured on a 120-column pty: both are 120, and
// `COLUMNS=057` is 57 while `COLUMNS=" 57"` is 120.
//
// Two known, deliberate deviations, both about a **zero** width:
//   - `COLUMNS=0` is `isdigit`, so Rich lays out to 0 and prints nothing
//     (measured: empty output). The port folds it to 80, because 0 is not a
//     width any box can be drawn at.
//   - a `COLUMNS` too large for an `int` is a bignum to Rich; the port treats it
//     as unset.
func ConsoleWidthFor(env Environment, isatty bool, size TerminalSize) int {
	columns, hasColumns := consoleDigits(env, "COLUMNS")
	if _, hasLines := consoleDigits(env, "LINES"); hasColumns && hasLines {
		return foldZeroWidth(columns)
	}
	if isDumbTerminal(env, isTerminal(env, isatty)) {
		return PanelWidth
	}
	width := 0
	if size != nil {
		if probed, ok := size(); ok {
			width = probed
		}
	}
	if hasColumns {
		width = columns
	}
	return foldZeroWidth(width)
}

// foldZeroWidth is Rich's `width = width or 80` (`rich/console.py:1044`),
// widened to any non-positive width because a box cannot be drawn at one.
func foldZeroWidth(width int) int {
	if width <= 0 {
		return PanelWidth
	}
	return width
}

// consoleDigits reads an environment variable under Python's `str.isdigit`
// guard (`rich/console.py:692`, `:1037`): every character a digit, and at least
// one of them. A sign, a space, or a decimal point makes the whole value
// invisible rather than invalid.
//
// Only ASCII digits count. Python's `isdigit` is true for `０５７` too, and
// `int` would read it as 57 — but a shell that exports a fullwidth `COLUMNS` is
// not a case worth carrying a Unicode numeral table for.
func consoleDigits(env Environment, name string) (int, bool) {
	raw, _ := env(name)
	if raw == "" {
		return 0, false
	}
	for _, character := range raw {
		if character < '0' || character > '9' {
			return 0, false
		}
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}

// detectColorSystem is `Console._detect_color_system`
// (`rich/console.py:795-817`).
func detectColorSystem(env Environment, terminal bool) ColorSystem {
	if !terminal || isDumbTerminal(env, terminal) {
		return ColorNone
	}
	colorTerm, _ := env("COLORTERM")
	switch strings.ToLower(strings.TrimSpace(colorTerm)) {
	case "truecolor", "24bit":
		return ColorTruecolor
	}
	// `TERM` is split at its **last** hyphen and only the suffix is consulted
	// (`_TERM_COLORS`, `rich/console.py:102-106`), so `xterm-256color` and
	// `screen-256color` agree and an unknown suffix is standard rather than
	// nothing.
	rawTerm, _ := env("TERM")
	term := strings.ToLower(strings.TrimSpace(rawTerm))
	if index := strings.LastIndex(term, "-"); index >= 0 {
		switch term[index+1:] {
		case "256color", "kitty":
			return ColorEightBit
		case "16color":
			return ColorStandard
		}
	}
	return ColorStandard
}

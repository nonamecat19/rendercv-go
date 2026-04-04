package cli

import "strings"

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

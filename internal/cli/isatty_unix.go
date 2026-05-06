//go:build unix

package cli

import (
	"os"

	"golang.org/x/sys/unix"
)

// stdoutIsTerminal is Python's `file.isatty()` for the stream Rich's consoles
// write to.
//
// **Stdout, because that is `Console.file`.** RenderCV builds its progress
// console with no `file` (`cli/render_command/progress_panel.py:63`) and reaches
// the rest through `rich.print`'s global one, and both default to `sys.stdout`.
//
// **A `TIOCGWINSZ` ioctl, not a character-device check.** `os.Stdout.Stat()`
// would call `/dev/null` a terminal, and `> /dev/null` under `TERM=dumb` would
// then take the 80-column dumb width where upstream honours `COLUMNS` — a wrong
// answer invented by the probe rather than found in Rich.
func stdoutIsTerminal() bool {
	_, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	return err == nil
}

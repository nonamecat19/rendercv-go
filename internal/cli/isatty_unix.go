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

// stdStreamsTerminalSize is Rich's window-size probe: `os.get_terminal_size`
// over `_STD_STREAMS` (`rich/console.py:98`, `:1027-1034`).
//
// **Stdin, stdout, stderr — in that order, and the first ioctl that *succeeds*
// wins**, not the first terminal among them. Rich's `break` is on the absence
// of an exception, so a pty on stdin decides the width even when stdout is a
// pipe: measured, upstream lays out to 120 with stdin on a 120-column pty and
// stdout redirected, where the port used to print 80. A failure is `pass`ed
// over and the next descriptor is tried; running out of descriptors leaves the
// width unset, which the caller folds to 80.
//
// The zero a pty with an unset window size reports is a **success**, and is
// reported as one — Rich folds it away after `COLUMNS` has had its turn, not
// before.
func stdStreamsTerminalSize() (int, bool) {
	for _, descriptor := range []int{
		int(os.Stdin.Fd()), int(os.Stdout.Fd()), int(os.Stderr.Fd()),
	} {
		if winsize, err := unix.IoctlGetWinsize(descriptor, unix.TIOCGWINSZ); err == nil {
			return int(winsize.Col), true
		}
	}
	return 0, false
}

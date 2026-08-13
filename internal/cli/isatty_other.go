//go:build !unix

package cli

// stdoutIsTerminal reports false where the port has no terminal probe to make.
//
// Rich's own Windows path is its legacy-console branch (`rich/console.py:970-978`),
// which the port does not mirror; answering "not a terminal" keeps every rule
// that reads it on the branch the goldens are captured in.
func stdoutIsTerminal() bool {
	return false
}

// stdStreamsTerminalSize reports no window size where the port has no probe to
// make, which is the answer Rich reaches when every `os.get_terminal_size`
// raises (`rich/console.py:1031-1032`) — the caller then folds to 80, the width
// every golden is captured at.
func stdStreamsTerminalSize() (int, bool) {
	return 0, false
}

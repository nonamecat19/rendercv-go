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

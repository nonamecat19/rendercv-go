// Command rendercv-go is a Go rewrite of RenderCV (https://github.com/rendercv/rendercv).
//
// It is a work in progress. The CLI is specified in specs/000-parity-contract/spec.md §2
// and implemented in iteration 12; until then this binary exists so the conformance
// harness has something real to drive, and every parity case fails with a diff rather
// than with "binary missing".
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr,
		"rendercv-go: not implemented yet — see specs/STATE.md for the port ledger")
	os.Exit(70) // EX_SOFTWARE: the command exists but cannot do its job.
}

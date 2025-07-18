// Command rendercv-go is a Go rewrite of RenderCV (https://github.com/rendercv/rendercv).
//
// The CLI is specified in specs/012-cli/spec.md. `render` is wired; `new`,
// `create-theme` and the Rich-rendered help panels are the rest of that
// iteration, and a command that is not wired exits 70 rather than printing a
// help screen that would be wrong in every byte.
package main

import (
	"os"

	"github.com/nonamecat19/rendercv-go/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:], os.Stdout, os.Stderr))
}

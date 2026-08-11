// Package version holds the upstream RenderCV version this port mirrors.
//
// It mirrors `src/rendercv/__init__.py:3`, and it is a package of its own for
// the reason spec 013 §3.3 behavior 26 gives: `__version__` reaches the user in
// three places — `--version`, the `new` welcome banner
// (`cli/new_command/print_welcome.py:14`) and the schema-hint comment on line 1
// of every generated starter CV (`schema/sample_generator.py:161-166`) — and a
// bump that misses one of them is a byte divergence in a golden.
package version

// RenderCV is upstream's `__version__`. It is upstream's number, not the port's:
// the port claims parity with this release, so a build of `rendercv-go` reports
// the RenderCV it reproduces.
const RenderCV = "2.8"

// Package clidiff is the differential gate of specs/013-parity-closeout §8: it
// runs the vendored Python `rendercv` and `bin/rendercv-go` over the same input
// and compares their raw output.
//
// It is test-only and every source file but this one is behind the
// `conformance` build tag. It deliberately does **not** use
// `internal/conformance`: that package's Normalize appends a trailing newline to
// both sides (`conformance.go:241-248`), which is exactly the byte spec 013 §6.2
// is about, and un-blinding it is a spec 013 §7.4 concern.
package clidiff

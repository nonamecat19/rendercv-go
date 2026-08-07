// Package assets carries the vendored Typst compiler and everything it reads.
//
// It mirrors nothing in upstream: upstream gets these from Python dependencies
// (`typst`, `rendercv-fonts`) and from a network download of
// `@preview/fontawesome:0.6.0`. A Go binary has no package manager behind it, so
// the three inputs are embedded instead — `specs/divergences.md` D-007.
package assets

import _ "embed"

// Typst is the Typst compiler built for `wasm32-wasip1`, driven through wazero.
// Built from tools/typstwasm; see README.md for the pinned toolchain.
//
//go:embed typst.wasm
var Typst []byte

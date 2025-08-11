// Package assets carries the vendored Typst compiler and everything it reads.
//
// It mirrors nothing in upstream: upstream gets these from Python dependencies
// (`typst`, `rendercv-fonts`) and from a network download of
// `@preview/fontawesome:0.6.0`. A Go binary has no package manager behind it, so
// the three inputs are embedded instead — `specs/divergences.md` D-007.
package assets

import (
	"embed"
	"io/fs"
)

// Typst is the Typst compiler built for `wasm32-wasip1`, driven through wazero.
// Built from tools/typstwasm; see README.md for the pinned toolchain.
//
//go:embed typst.wasm
var Typst []byte

// fonts is the `rendercv-fonts` package's 15 folders, laid out exactly as the
// Python package ships them so that `paths_to_font_folders` and this tree name
// the same faces (`third_party/rendercv/src/rendercv/renderer/pdf_png.py:154-186`).
//
// The `all:` prefix is required: several folders carry licence files, and one
// day a font vendor will ship a dotfile.
//
//go:embed all:fonts
var fonts embed.FS

// packages is the Typst package cache, laid out `preview/<name>/<version>/` —
// the shape `get_package_path` builds in a temp directory (`pdf_png.py:114-146`).
//
//go:embed all:packages
var packages embed.FS

// Fonts returns the vendored font tree, rooted at the folder that contains the
// per-family directories. Mount it as the compiler's font path.
func Fonts() fs.FS {
	return sub(fonts, "fonts")
}

// Packages returns the vendored Typst package cache, rooted at the directory
// that contains `preview/`.
func Packages() fs.FS {
	return sub(packages, "packages")
}

// sub strips the embed root. The paths are compile-time constants that
// `go:embed` has already proven exist, so a failure here is unreachable.
func sub(f embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic("typstc/assets: " + err.Error())
	}
	return sub
}

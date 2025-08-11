package assets_test

import (
	"io/fs"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/typstc/assets"
)

// TestFontsCoverEveryFamily pins the font tree against the families the themes
// ask for. A missing family does not fail loudly at render time — it renders in
// a fallback face, which is the failure that passed 12 of 14 PDF cases during
// iteration 10 (D-007).
func TestFontsCoverEveryFamily(t *testing.T) {
	// The folder names `rendercv-fonts` ships. Upstream's themes select by
	// family name, and the folder name is the family name for every one of them.
	want := []string{
		"EB Garamond",
		"Font Awesome 7",
		"Fontin",
		"Gentium Book Plus",
		"Lato",
		"Mukta",
		"Noto Sans",
		"Open Sans",
		"Open Sauce Sans",
		"Poppins",
		"Raleway",
		"Roboto",
		"Source Sans 3",
		"Ubuntu",
		"XCharter",
	}

	tree := assets.Fonts()
	for _, family := range want {
		entries, err := fs.ReadDir(tree, family)
		if err != nil {
			t.Errorf("font family %q: %v", family, err)
			continue
		}
		if !hasFontFile(entries) {
			t.Errorf("font family %q: no .ttf or .otf in the folder", family)
		}
	}
}

// TestPackagesAreLaidOutForTypst checks the `preview/<name>/<version>/` shape
// the compiler's package loader requires, and the two entrypoints it reads.
func TestPackagesAreLaidOutForTypst(t *testing.T) {
	tree := assets.Packages()

	// `rendercv` is the package upstream bundles: typst.toml and lib.typ only,
	// which is exactly what `get_package_path` copies (pdf_png.py:114-146).
	// `fontawesome` is the one upstream downloads instead of shipping (D-007).
	want := []string{
		"preview/rendercv/0.3.0/typst.toml",
		"preview/rendercv/0.3.0/lib.typ",
		"preview/fontawesome/0.6.0/typst.toml",
		"preview/fontawesome/0.6.0/lib.typ",
	}
	for _, path := range want {
		if _, err := fs.Stat(tree, path); err != nil {
			t.Errorf("package file %q: %v", path, err)
		}
	}
}

// TestTypstIsEmbedded guards against an empty or truncated embed, which would
// otherwise surface as an unhelpful wazero decode error.
func TestTypstIsEmbedded(t *testing.T) {
	const wasmMagic = "\x00asm"
	if len(assets.Typst) < len(wasmMagic) || string(assets.Typst[:4]) != wasmMagic {
		t.Fatalf("typst.wasm does not start with the wasm magic (%d bytes embedded)", len(assets.Typst))
	}
}

func hasFontFile(entries []fs.DirEntry) bool {
	for _, e := range entries {
		switch {
		case e.IsDir():
		case hasSuffix(e.Name(), ".ttf"), hasSuffix(e.Name(), ".otf"):
			return true
		}
	}
	return false
}

func hasSuffix(name, suffix string) bool {
	return len(name) >= len(suffix) && name[len(name)-len(suffix):] == suffix
}

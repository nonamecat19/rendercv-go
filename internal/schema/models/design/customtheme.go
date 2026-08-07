package design

import (
	"fmt"
	"os"
	"path/filepath"
)

// ValidateCustomThemeFolder reproduces the two **folder** checks upstream runs
// before it touches a custom theme (`design.py:72-86`), with upstream's wording.
//
// The name check that precedes them (`:59-70`) already shipped in iteration 6 —
// `validateThemeName` — so this begins where that leaves off rather than
// repeating it.
//
// **The messages are ported, not invented.** They are upstream's exact strings,
// so reproducing them is axis 4 parity rather than new user-visible text — which
// is why this does not need the human gate that spec 014 §2 behavior 9's
// Lua-specific messages do. A verifier found the port rendering happily where
// upstream reports each of these.
//
// `relativeTo` is the input file's directory, or the working directory when
// there is no input file (`:56`).
func ValidateCustomThemeFolder(theme, relativeTo string) error {
	folder := filepath.Join(relativeTo, theme)
	info, err := os.Stat(folder)
	if err != nil || !info.IsDir() {
		absolute, absErr := filepath.Abs(folder)
		if absErr != nil {
			absolute = folder
		}
		return errFolderMissing(absolute)
	}

	if !hasTemplate(folder) {
		absolute, absErr := filepath.Abs(folder)
		if absErr != nil {
			absolute = folder
		}
		return errNoTemplates(absolute)
	}
	return nil
}

// The two messages are upstream's verbatim, trailing full stops included, so
// `ST1005` is suppressed rather than obeyed: obeying it would be an axis-4
// divergence in text a user reads.
func errFolderMissing(folder string) error {
	//nolint:staticcheck // upstream's text; ST1005 would be an axis-4 divergence
	return fmt.Errorf("The custom theme folder `%s` does not exist. It should be in the same"+
		" directory as the input file.", folder)
}

func errNoTemplates(folder string) error {
	//nolint:staticcheck // upstream's text; ST1005 would be an axis-4 divergence
	return fmt.Errorf("The custom theme folder `%s` does not contain any *.j2.typ files."+
		" It should contain at least one *.j2.typ file.", folder)
}

// hasTemplate is upstream's `rglob("*.j2.typ")` — **recursive**, so a theme
// keeping its templates in `entries/` alone still counts.
func hasTemplate(folder string) bool {
	found := false
	_ = filepath.WalkDir(folder, func(path string, _ os.DirEntry, err error) error {
		if err != nil || found {
			return nil //nolint:nilerr // an unreadable subtree is simply not a match
		}
		if matched, _ := filepath.Match("*.j2.typ", filepath.Base(path)); matched {
			found = true
		}
		return nil
	})
	return found
}

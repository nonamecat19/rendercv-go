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
	// **`uncleanedJoin`, not `filepath.Join`.** `Join` also calls `Clean`,
	// which would silently collapse a `..` `relativeTo` carries here even
	// after `validate.go`'s `relativeTo` stopped introducing one of its
	// own — `os.Stat`/`WalkDir` handle an uncleaned `../` segment exactly
	// as well as a cleaned one, so there is no functional reason to clean
	// before using the path for real filesystem calls, only a reason not
	// to when the same string reaches a message. Found by a fresh-context
	// verifier (iteration 14's nineteenth re-verification).
	folder := uncleanedJoin(relativeTo, theme)
	// **`exists()`, not `is_dir()`.** A path that resolves to a regular
	// file still exists upstream, so it falls through to the `rglob`
	// check next and gets "does not contain any *.j2.typ files" — not
	// "does not exist". `os.Stat`'s error is the only thing that means
	// the path is actually missing; `IsDir()` was a stronger predicate
	// than upstream's. Found by a fresh-context verifier (iteration 14's
	// seventeenth re-verification).
	if _, err := os.Stat(folder); err != nil {
		return errFolderMissing(absoluteLikePython(folder))
	}

	if !hasTemplate(folder) {
		return errNoTemplates(absoluteLikePython(folder))
	}
	return nil
}

// absoluteLikePython is `Path.absolute()`, not `filepath.Abs`. **`Abs` calls
// `Clean`, which collapses `..`** — `pathlib.Path.absolute()` only prepends
// the working directory and leaves the rest of the path exactly as written,
// `..` included, so a relative input path containing one prints a different
// folder in both of this file's messages. Falls back to the unresolved path
// on a `Getwd` failure, the same way the two callers already did on an
// `Abs` failure. Found by a fresh-context verifier (iteration 14's
// non-colour-design-slice sweep).
// uncleanedJoin is `PurePath.__truediv__`: `dir`'s own parsed segments
// (`.` dropped, `..` kept literal) with `name` appended, the same parse
// `uncleanedDir` uses for the other half of this pair.
func uncleanedJoin(dir, name string) string {
	absolute, parts := pathlibParts(dir)
	parts = append(parts, name)
	return pathlibJoin(absolute, parts)
}

// ThemeScriptPath is where `<theme>/init.lua` is read from for an input file at
// `inputPath`, resolved the way upstream resolves it.
//
// **It exists so two callers cannot disagree.** `Validate` resolves the parent
// lexically (`relativeTo` → `uncleanedDir`, which is `PurePath.parent`), and
// `bridge.themeScript` used `filepath.Dir`, which calls `Clean` and collapses a
// `..` segment. On an ordinary tree both spellings name the same file. **Through
// a symlink they do not**: with `work/bb` pointing at `other/real` and `other/bb`
// a different directory, `render ./bb/../bb/CV.yaml` validated against
// `other/bb`'s script and rendered with `other/real`'s — two different themes at
// exit 0. Measured against the vendored binary, which renders the lexical one.
//
// A second implementation of "like `PurePath.parent`" is what let the two sites
// drift apart, so this is the one place either of them asks.
func ThemeScriptPath(inputPath, theme string) string {
	return themeScriptPathIn(uncleanedDir(inputPath), theme)
}

func absoluteLikePython(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	return cwd + string(filepath.Separator) + path
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
//
// **The theme folder itself may be a symlink to a directory.**
// `filepath.WalkDir` `Lstat`s its root rather than `Stat`ing it, so a
// symlinked root reads as "not a directory" and is never descended into —
// upstream's `rglob` scans straight through it, since `pathlib` resolves
// symlinks by default. `EvalSymlinks` resolves the root (and any symlink
// segment inside `relativeTo`) before the walk starts; an unresolvable
// path — already ruled out by the caller's `os.Stat` — falls back to the
// original. Found by a fresh-context verifier (iteration 14's
// non-colour-design-slice sweep).
func hasTemplate(folder string) bool {
	if resolved, err := filepath.EvalSymlinks(folder); err == nil {
		folder = resolved
	}
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

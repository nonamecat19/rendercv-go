package design

// This file exports `pathlib`'s three purely lexical path operations for the
// callers outside this package that need them — today `internal/cli`, whose
// output folder, artifact paths, overlay paths and panel lines are all derived
// from the input file's directory the way upstream derives them.
//
// **They are wrappers, not a second implementation.** `uncleanedDir`,
// `uncleanedJoin` and `pathlibParts` (validate.go) are the one parser; the
// theme-script lookup and the CLI ask it the same questions rather than each
// writing their own "like `PurePath.parent`", which is exactly how those two
// sites drifted apart and rendered two different themes at exit 0.
//
// Go's `filepath` equivalents all call `Clean`, which resolves a `..` segment
// against its neighbour. Through a symlink that names a different directory,
// so none of them can stand in for these.

// Parent is `PurePath.parent`: the path without its last segment, with `.` and
// empty segments dropped and any `..` left exactly where it was. A path with
// no parent is `.`, which is what `pathlib` prints.
//
// It is `filepath.Dir` minus the `Clean`: `Parent("./bb/../bb/CV.yaml")` is
// `bb/../bb`, where `filepath.Dir` answers `bb`.
func Parent(path string) string {
	return uncleanedDir(path)
}

// Join is `PurePath.__truediv__`: dir's own segments with name appended, and
// no cleaning. It is `filepath.Join` minus the `Clean`.
func Join(dir, name string) string {
	return uncleanedJoin(dir, name)
}

// RelativeTo is `PurePath.relative_to`: a lexical prefix strip. It reports
// false where `pathlib` raises `ValueError` — when base's segments are not a
// prefix of path's — which is the case upstream catches and answers with the
// unmodified path (`cli/render_command/progress_panel.py:96-99`).
//
// It is `filepath.Rel` minus the `Clean`, and minus `Rel`'s willingness to
// walk out of base with `..` segments of its own: `pathlib` either strips a
// prefix or fails.
func RelativeTo(path, base string) (string, bool) {
	pathAbsolute, pathParts := pathlibParts(path)
	baseAbsolute, baseParts := pathlibParts(base)
	if pathAbsolute != baseAbsolute || len(baseParts) > len(pathParts) {
		return "", false
	}
	for i, segment := range baseParts {
		if pathParts[i] != segment {
			return "", false
		}
	}
	// The remainder is always relative — `pathlib` returns a relative path
	// from `relative_to`, and an empty remainder prints as `.`.
	return pathlibJoin(false, pathParts[len(baseParts):]), true
}

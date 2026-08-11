package templater

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
)

// Format is the template directory a fragment is looked up under.
type Format string

// The three template directories. `html` holds exactly one fragment and is
// iteration 11's; it is named here because the loader's extension table needs
// it.
const (
	FormatTypst    Format = "typst"
	FormatMarkdown Format = "markdown"
	FormatHTML     Format = "html"
)

// Extension is the file suffix each format's fragments carry
// (templater.py:76-79).
func (f Format) Extension() string {
	switch f {
	case FormatTypst:
		return "typ"
	case FormatMarkdown:
		return "md"
	case FormatHTML:
		return "html"
	}
	return ""
}

// Loader resolves a fragment name to template source, reproducing upstream's
// two-loader search and its Typst-only theme lookup (spec 008 §2).
//
// **Four candidate paths for a Typst fragment and two for the others.** The
// order is what makes user overrides work, and it is not the obvious one:
//
//	<input dir>/<theme>/<name>.j2.typ     ← a theme-specific override
//	<builtin>/<theme>/<name>.j2.typ       ← never exists today, and is tried
//	<input dir>/typst/<name>.j2.typ       ← a format-wide override
//	<builtin>/typst/<name>.j2.typ         ← what ships
//
// Upstream gets this shape by asking a two-directory `FileSystemLoader` for the
// theme-qualified path first and suppressing `TemplateNotFound`, then asking for
// the format-qualified one (`templater.py:160-172`). The second row is a
// consequence of that structure rather than a feature, and it is reproduced
// because a user who ships a `classic/` directory beside the built-ins would
// otherwise see different behavior.
type Loader struct {
	// InputDir is the input file's directory, or the working directory when
	// there is no input file (`:34-41`).
	InputDir string
	// Builtin is the embedded template tree.
	Builtin fs.FS
}

// Candidates is the ordered list of paths for one fragment, exported because it
// is the thing worth asserting: a loader test that only checks the winner
// cannot tell a wrong order from a lucky one.
func (l Loader) Candidates(format Format, theme, name string) []string {
	file := name + ".j2." + format.Extension()
	if format == FormatHTML {
		// `Full.html` carries no `.j2.` infix (templater.py:152-154).
		file = name
	}

	var paths []string
	if format == FormatTypst && theme != "" {
		paths = append(paths, path.Join(theme, file))
	}
	return append(paths, path.Join(string(format), file))
}

// Load returns the first candidate that exists, searching the input directory
// before the built-ins for each.
func (l Loader) Load(format Format, theme, name string) (string, error) {
	for _, candidate := range l.Candidates(format, theme, name) {
		if l.InputDir != "" {
			// **`design.Join`, not `filepath.Join`.** `Join` calls `Clean`, which
			// collapses a `..` segment; `InputDir` is `PurePath.parent`'s answer
			// and keeps it, because upstream's loader takes
			// `input_file_path.parent` (`templater.py:38`) and cleans nothing.
			// Through a symlink the two spellings name different directories, so
			// cleaning here read a template override from a directory the rest of
			// the pipeline was not using. The idiomatic Go call is the wrong one
			// at this call site: do not simplify it back.
			source, err := os.ReadFile(design.Join(l.InputDir, filepath.FromSlash(candidate)))
			if err == nil {
				return string(source), nil
			}
			if !os.IsNotExist(err) {
				return "", fmt.Errorf("reading %s: %w", candidate, err)
			}
		}
		if l.Builtin != nil {
			source, err := fs.ReadFile(l.Builtin, candidate)
			if err == nil {
				return string(source), nil
			}
		}
	}
	return "", fmt.Errorf("%w: %s", ErrTemplateNotFound, name)
}

// ErrTemplateNotFound is what upstream raises as `jinja2.TemplateNotFound` after
// both lookups fail. The theme-qualified miss is **suppressed** and only the
// format-qualified one propagates, so this is reachable only when a built-in
// fragment is missing — a packaging bug, not a user error.
var ErrTemplateNotFound = errors.New("template not found")

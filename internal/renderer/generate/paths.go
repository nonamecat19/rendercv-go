// Package generate is upstream's five `generate_*` functions
// (`renderer/{typst,pdf_png,markdown,html}.py`) — the step that turns a
// validated model into a file on disk and returns its path.
//
// It exists as its own package because two callers need it: `internal/cli`,
// which wraps each call in a progress row, and `pkg/rendercv`, the public API
// (spec 016 §1.1). Keeping one implementation is what stops the two drifting.
package generate

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
)

// Default output paths (`settings/render_command.py:22-55`). `OUTPUT_FOLDER` is
// itself a placeholder, resolved before the name ones.
const (
	DefaultOutputFolder = "rendercv_output"
	DefaultTypstPath    = "OUTPUT_FOLDER/NAME_IN_SNAKE_CASE_CV.typ"
	DefaultPDFPath      = "OUTPUT_FOLDER/NAME_IN_SNAKE_CASE_CV.pdf"
	DefaultMarkdownPath = "OUTPUT_FOLDER/NAME_IN_SNAKE_CASE_CV.md"
	DefaultHTMLPath     = "OUTPUT_FOLDER/NAME_IN_SNAKE_CASE_CV.html"
	DefaultPNGPath      = "OUTPUT_FOLDER/NAME_IN_SNAKE_CASE_CV.png"
)

// PathInput is what `resolve_rendercv_file_path` reads besides the template.
type PathInput struct {
	// Name is `cv.name` **before** processing — the placeholders spell the
	// user's own text, not the escaped Typst one.
	// **nil is `None`**, an absent or null `cv.name`, which upstream filters
	// out of the placeholder table entirely; a non-nil empty string is
	// `cv.name: ""`, which keeps the plain `NAME` key. A plain string cannot
	// hold that difference, and the zero value has to mean the absent case.
	// See namePlaceholders.
	Name *string

	// InputDir is the input file's directory. **Every path template resolves
	// against it**, not against the working directory: all six of
	// `render_command`'s path fields are typed `PlannedPathRelativeToInput`
	// (`settings/render_command.py:30,46,54,62,71,79`), which
	// `path.py:37-41` resolves as `input_file_path.parent / path` at
	// validation time — so by the time upstream substitutes placeholders, the
	// template is already absolute.
	InputDir string

	// OutputFolder is already resolved against InputDir.
	OutputFolder string
	Catalog      locale.Catalog
	CurrentDate  process.Catalog
	Placeholders map[string]string
}

// ResolvePath is `resolve_rendercv_file_path` (path_resolver.py:40-109).
//
// Two substitutions in a fixed order, and the order is the behavior:
//
//  1. `OUTPUT_FOLDER` is replaced **as a path component**, anywhere in the path;
//  2. the name and date placeholders are substituted **in the file name only**,
//     so a directory called `NAME` is left alone.
//
// Upstream then creates the parent directories, which is why
// `render_custom_paths` can write `out/nested/custom.md` without the directory
// existing.
func ResolvePath(template string, input PathInput) (string, error) {
	path := resolveOutputFolder(absoluteToInput(template, input.InputDir), input.OutputFolder)

	dir, name := filepath.Split(path)
	name = process.SubstitutePlaceholders(name, namePlaceholders(input))

	// **`design.Join`, not `filepath.Join`** — upstream is
	// `resolved_file_path = file_path.parent / file_name`
	// (`path_resolver.py:108`), `PurePath`'s `/`, which does not clean. `Join`
	// would collapse the `..` that `outputFolderFor` just preserved, undoing
	// the lexical resolution one line before it is used. Likewise the parent
	// below: `:109` creates the directories of the uncleaned path, and
	// `MkdirAll` walks a `../` segment as happily as the kernel does.
	resolved := design.Join(dir, name)
	if parent := design.Parent(resolved); parent != "" && parent != "." {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return "", err
		}
	}
	return resolved, nil
}

// absoluteToInput is `PlannedPathRelativeToInput` (`path.py:37-41`):
// `input_file_path.parent / path`, applied to a path that is not already
// absolute.
//
// **It is `design.Join`, not `filepath.Join`** — `PurePath`'s `/` does not
// clean, and a `..` in a user's template has to survive to the message and to
// the filesystem exactly as written.
func absoluteToInput(path, inputDir string) string {
	if path == "" || filepath.IsAbs(path) || inputDir == "" {
		return path
	}
	return design.Join(inputDir, path)
}

// resolveOutputFolder resolves the `OUTPUT_FOLDER` component
// (`path_resolver.py:9-37`). It is a component rather than a substring: a file
// named `MY_OUTPUT_FOLDER.typ` is not a placeholder.
//
// **Everything before the placeholder is discarded**, not just the placeholder
// itself: upstream returns `output_folder / suffix_parts` (`:34-37`), because
// both paths are absolute and resolved from the same base, so the prefix would
// otherwise be duplicated. Replacing the component in place is equivalent only
// when nothing precedes it, which is true of all five default templates — and
// false as soon as a template is made input-relative.
func resolveOutputFolder(path, folder string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, part := range parts {
		if part != "OUTPUT_FOLDER" {
			continue
		}
		suffix := parts[i+1:]
		if len(suffix) == 0 {
			return folder
		}
		return design.Join(folder, filepath.FromSlash(strings.Join(suffix, "/")))
	}
	return filepath.FromSlash(strings.Join(parts, "/"))
}

// namePlaceholders is the seven name spellings plus the eight date ones
// (`:73-104`).
//
// **The table is filtered on `None`, not on falsiness, and the two differ for
// exactly one input.** The six derived spellings are each written
// `x.replace(...) if cv.name else None` (`:77-102`), so an *empty-string* name
// makes them `None` and they are dropped — but plain `NAME` is
// `rendercv_model.cv.name` with no guard (`:76`), and `""` is not `None`, so it
// survives and substitutes to nothing. `cv.name: ""` therefore writes
// `_IN_SNAKE_CASE_CV.typ`: the `NAME` prefix of the longer placeholder is
// replaced with the empty string and the rest of the literal stays. Measured
// against the vendored Python.
//
// An absent or null name drops all seven, and the file keeps its literal
// `NAME_IN_SNAKE_CASE_CV.typ` name — `plainName` maps both to the empty string,
// so NameIsNone is what separates them here.
func namePlaceholders(input PathInput) map[string]string {
	out := map[string]string{}
	for name, value := range input.Placeholders {
		out[name] = value
	}
	if input.Name == nil {
		return out
	}
	name := *input.Name
	out["NAME"] = name
	if name == "" {
		return out
	}

	underscored := strings.ReplaceAll(name, " ", "_")
	hyphenated := strings.ReplaceAll(name, " ", "-")

	out["NAME_IN_SNAKE_CASE"] = underscored
	out["NAME_IN_LOWER_SNAKE_CASE"] = strings.ToLower(underscored)
	out["NAME_IN_UPPER_SNAKE_CASE"] = strings.ToUpper(underscored)
	out["NAME_IN_KEBAB_CASE"] = hyphenated
	out["NAME_IN_LOWER_KEBAB_CASE"] = strings.ToLower(hyphenated)
	out["NAME_IN_UPPER_KEBAB_CASE"] = strings.ToUpper(hyphenated)
	return out
}

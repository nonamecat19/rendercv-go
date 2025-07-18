// Package cli is upstream's `cli/**` — the command surface of spec 012.
package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
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
	Name         string
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
	path := resolveOutputFolder(template, input.OutputFolder)

	dir, name := filepath.Split(path)
	name = process.SubstitutePlaceholders(name, namePlaceholders(input))

	resolved := filepath.Join(dir, name)
	if parent := filepath.Dir(resolved); parent != "" && parent != "." {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return "", err
		}
	}
	return resolved, nil
}

// resolveOutputFolder replaces the `OUTPUT_FOLDER` component
// (`path_resolver.py:15-37`). It is a component rather than a substring: a file
// named `MY_OUTPUT_FOLDER.typ` is not a placeholder.
func resolveOutputFolder(path, folder string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, part := range parts {
		if part == "OUTPUT_FOLDER" {
			parts[i] = folder
		}
	}
	return filepath.FromSlash(strings.Join(parts, "/"))
}

// namePlaceholders is the seven name spellings plus the eight date ones
// (`:73-104`).
//
// **A nameless CV drops them all** rather than substituting an empty string:
// upstream filters `None` out of the table, so `NAME_IN_SNAKE_CASE_CV.typ` stays
// literal for a document with no `cv.name`.
func namePlaceholders(input PathInput) map[string]string {
	out := map[string]string{}
	for name, value := range input.Placeholders {
		out[name] = value
	}
	if input.Name == "" {
		return out
	}

	underscored := strings.ReplaceAll(input.Name, " ", "_")
	hyphenated := strings.ReplaceAll(input.Name, " ", "-")

	out["NAME"] = input.Name
	out["NAME_IN_SNAKE_CASE"] = underscored
	out["NAME_IN_LOWER_SNAKE_CASE"] = strings.ToLower(underscored)
	out["NAME_IN_UPPER_SNAKE_CASE"] = strings.ToUpper(underscored)
	out["NAME_IN_KEBAB_CASE"] = hyphenated
	out["NAME_IN_LOWER_KEBAB_CASE"] = strings.ToLower(hyphenated)
	out["NAME_IN_UPPER_KEBAB_CASE"] = strings.ToUpper(hyphenated)
	return out
}

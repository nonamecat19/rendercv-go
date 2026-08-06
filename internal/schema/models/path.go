// Package models mirrors src/rendercv/schema/models/path.py: the two
// relative-to-input path types and their resolution and serialization rules.
package models

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// pathOtherErrorCode is CustomPydanticErrorTypes.other.value
// (schema/models/custom_error_types.py:6), the discriminator both path
// error messages of path.py:45-55 are raised with.
const pathOtherErrorCode schemaerr.Code = "rendercv_other_error"

// ResolutionBase returns the directory a relative path is resolved against:
// the input file's parent directory when the context carries one, otherwise
// the process working directory (schema/models/path.py:38-39, spec §3.36).
func ResolutionBase(ctx *valctx.ValidationContext) (string, error) {
	if p, ok := ctx.InputPath(); ok {
		return filepath.Dir(p), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolving working directory: %w", err)
	}
	return cwd, nil
}

// resolveRelativePath mirrors schema/models/path.py:resolve_relative_path
// (path.py:12-57). An empty path short-circuits: no resolution and no
// existence check (spec §3.38). An absolute path is left unchanged
// (spec §3.37). When mustExist is true, the resolved path must exist
// (§4.5) and must be a regular file (§4.6); both messages interpolate the
// path relative to the resolution base (spec §3.39).
func resolveRelativePath(path string, ctx *valctx.ValidationContext, mustExist bool) (string, error) {
	if path == "" {
		return "", nil
	}

	base, err := ResolutionBase(ctx)
	if err != nil {
		return "", err
	}

	resolved := path
	if !filepath.IsAbs(path) {
		resolved = filepath.Join(base, path)
	}

	if mustExist {
		display := relativeToBase(base, resolved)

		info, statErr := os.Stat(resolved)
		if os.IsNotExist(statErr) {
			return "", &schemaerr.ValidationError{
				Code:    pathOtherErrorCode,
				Message: fmt.Sprintf("The file `%s` does not exist.", display),
				Input:   display,
			}
		}
		if statErr != nil {
			return "", fmt.Errorf("stat %s: %w", resolved, statErr)
		}
		if !info.Mode().IsRegular() {
			return "", &schemaerr.ValidationError{
				Code:    pathOtherErrorCode,
				Message: fmt.Sprintf("The path `%s` is not a file.", display),
				Input:   display,
			}
		}
	}

	return resolved, nil
}

// relativeToBase renders resolved relative to base, falling back to
// resolved itself when it cannot be expressed that way
// (pathlib.Path.relative_to, path.py:48, :54).
func relativeToBase(base, resolved string) string {
	rel, err := filepath.Rel(base, resolved)
	if err != nil {
		return resolved
	}
	return rel
}

// ExistingPathRelativeToInput mirrors
// schema/models/path.py:ExistingPathRelativeToInput (path.py:67-72): a path
// resolved relative to the input file's directory, required to exist and to
// be a regular file (spec §3.35, §3.39).
type ExistingPathRelativeToInput struct {
	Value string
}

// ResolveExistingPath validates and resolves raw as an
// ExistingPathRelativeToInput.
func ResolveExistingPath(raw string, ctx *valctx.ValidationContext) (ExistingPathRelativeToInput, error) {
	resolved, err := resolveRelativePath(raw, ctx, true)
	if err != nil {
		return ExistingPathRelativeToInput{}, err
	}
	return ExistingPathRelativeToInput{Value: resolved}, nil
}

// PlannedPathRelativeToInput mirrors
// schema/models/path.py:PlannedPathRelativeToInput (path.py:74-80): resolved
// the same way as ExistingPathRelativeToInput, but existence is never
// checked (spec §3.40).
type PlannedPathRelativeToInput struct {
	Value string
}

// ResolvePlannedPath resolves raw as a PlannedPathRelativeToInput. It never
// fails on a nonexistent path (spec §3.40).
func ResolvePlannedPath(raw string, ctx *valctx.ValidationContext) (PlannedPathRelativeToInput, error) {
	resolved, err := resolveRelativePath(raw, ctx, false)
	if err != nil {
		return PlannedPathRelativeToInput{}, err
	}
	return PlannedPathRelativeToInput{Value: resolved}, nil
}

// Serialize mirrors schema/models/path.py:serialize_path (path.py:60-64): a
// POSIX-style path relative to the process working directory when
// expressible, and the absolute path otherwise. It never fails (spec §3.41).
func (p PlannedPathRelativeToInput) Serialize() string {
	if p.Value == "" {
		return p.Value
	}

	cwd, err := os.Getwd()
	if err != nil {
		return p.Value
	}

	rel, err := filepath.Rel(cwd, p.Value)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return p.Value
	}

	return filepath.ToSlash(rel)
}

package inputpath_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/inputpath"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

func TestResolutionBaseWithInputFile(t *testing.T) {
	ctx := &valctx.ValidationContext{InputFilePath: "/tmp/somewhere/input.yaml"}
	base, err := inputpath.ResolutionBase(ctx)
	if err != nil {
		t.Fatalf("ResolutionBase() error = %v", err)
	}
	if base != "/tmp/somewhere" {
		t.Errorf("ResolutionBase() = %q, want %q", base, "/tmp/somewhere")
	}
}

func TestResolutionBaseWithoutInputFile(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	base, err := inputpath.ResolutionBase(nil)
	if err != nil {
		t.Fatalf("ResolutionBase() error = %v", err)
	}
	if base != cwd {
		t.Errorf("ResolutionBase() = %q, want %q (spec §3.36)", base, cwd)
	}
}

// TestExistingPathVariousRelativeFormats covers spec §3.25/§5.25:
// "subdir/f", "../sibling/f", "./same/f" all resolve against the input
// file's parent (tests/schema/models/test_path.py:80-100).
func TestExistingPathVariousRelativeFormats(t *testing.T) {
	tmp := t.TempDir()
	inputFile := filepath.Join(tmp, "input.yaml")
	ctx := &valctx.ValidationContext{InputFilePath: inputFile}

	cases := []struct {
		name     string
		relative string
	}{
		{"subdir", "subdir/file.txt"},
		{"sibling", "../sibling/file.txt"},
		{"same_dir", "./same_dir/file.txt"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expected := filepath.Join(tmp, tc.relative)
			if err := os.MkdirAll(filepath.Dir(expected), 0o755); err != nil {
				t.Fatalf("MkdirAll() error = %v", err)
			}
			if err := os.WriteFile(expected, nil, 0o644); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			got, err := inputpath.ResolveExistingPath(tc.relative, ctx)
			if err != nil {
				t.Fatalf("ResolveExistingPath(%q) error = %v", tc.relative, err)
			}
			wantAbs, _ := filepath.Abs(expected)
			gotAbs, _ := filepath.Abs(got.Value)
			if gotAbs != wantAbs {
				t.Errorf("ResolveExistingPath(%q) = %q, want %q", tc.relative, gotAbs, wantAbs)
			}
		})
	}
}

// TestPlannedPathVariousRelativeFormats mirrors the same table for the
// planned type (spec §3.40).
func TestPlannedPathVariousRelativeFormats(t *testing.T) {
	tmp := t.TempDir()
	inputFile := filepath.Join(tmp, "input.yaml")
	ctx := &valctx.ValidationContext{InputFilePath: inputFile}

	cases := []string{"output/result.pdf", "../build/output.html", "./generated/doc.md"}
	for _, relative := range cases {
		t.Run(relative, func(t *testing.T) {
			got, err := inputpath.ResolvePlannedPath(relative, ctx)
			if err != nil {
				t.Fatalf("ResolvePlannedPath(%q) error = %v", relative, err)
			}
			wantAbs, _ := filepath.Abs(filepath.Join(tmp, relative))
			gotAbs, _ := filepath.Abs(got.Value)
			if gotAbs != wantAbs {
				t.Errorf("ResolvePlannedPath(%q) = %q, want %q", relative, gotAbs, wantAbs)
			}
		})
	}
}

func TestExistingAndPlannedResolveAgainstCwdWithoutInputFile(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	got, err := inputpath.ResolvePlannedPath("planned/output.pdf", nil)
	if err != nil {
		t.Fatalf("ResolvePlannedPath() error = %v", err)
	}
	want := filepath.Join(cwd, "planned/output.pdf")
	if got.Value != want {
		t.Errorf("ResolvePlannedPath() = %q, want %q (spec §3.36)", got.Value, want)
	}
}

func TestAbsolutePathLeftUnchanged(t *testing.T) {
	tmp := t.TempDir()
	existing := filepath.Join(tmp, "existing.txt")
	if err := os.WriteFile(existing, nil, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	ctx := &valctx.ValidationContext{InputFilePath: filepath.Join(tmp, "input.yaml")}

	gotExisting, err := inputpath.ResolveExistingPath(existing, ctx)
	if err != nil {
		t.Fatalf("ResolveExistingPath() error = %v", err)
	}
	if gotExisting.Value != existing {
		t.Errorf("ResolveExistingPath() = %q, want %q (spec §3.37)", gotExisting.Value, existing)
	}

	gotPlanned, err := inputpath.ResolvePlannedPath(existing, ctx)
	if err != nil {
		t.Fatalf("ResolvePlannedPath() error = %v", err)
	}
	if gotPlanned.Value != existing {
		t.Errorf("ResolvePlannedPath() = %q, want %q (spec §3.37)", gotPlanned.Value, existing)
	}
}

func TestEmptyPathShortCircuits(t *testing.T) {
	ctx := &valctx.ValidationContext{InputFilePath: "/nonexistent/root/input.yaml"}

	got, err := inputpath.ResolveExistingPath("", ctx)
	if err != nil {
		t.Fatalf("ResolveExistingPath(\"\") error = %v, want nil (spec §3.38)", err)
	}
	if got.Value != "" {
		t.Errorf("ResolveExistingPath(\"\") = %q, want empty", got.Value)
	}

	gotPlanned, err := inputpath.ResolvePlannedPath("", ctx)
	if err != nil {
		t.Fatalf("ResolvePlannedPath(\"\") error = %v, want nil", err)
	}
	if gotPlanned.Value != "" {
		t.Errorf("ResolvePlannedPath(\"\") = %q, want empty", gotPlanned.Value)
	}
}

// TestExistingPathMissingFile covers spec §4.5: the message interpolates
// the path relative to the resolution base, not the resolved absolute path.
func TestExistingPathMissingFile(t *testing.T) {
	tmp := t.TempDir()
	ctx := &valctx.ValidationContext{InputFilePath: filepath.Join(tmp, "input.yaml")}

	_, err := inputpath.ResolveExistingPath("nonexistent.txt", ctx)
	if err == nil {
		t.Fatal("ResolveExistingPath() error = nil, want an error")
	}

	var ve *schemaerr.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("error is not a *schemaerr.ValidationError: %v", err)
	}
	want := "The file `nonexistent.txt` does not exist."
	if ve.Message != want {
		t.Errorf("Message = %q, want %q", ve.Message, want)
	}
}

// TestExistingPathNotAFile covers spec §4.6.
func TestExistingPathNotAFile(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "new_dir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	ctx := &valctx.ValidationContext{InputFilePath: filepath.Join(tmp, "input.yaml")}

	_, err := inputpath.ResolveExistingPath("new_dir", ctx)
	if err == nil {
		t.Fatal("ResolveExistingPath() error = nil, want an error")
	}

	var ve *schemaerr.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("error is not a *schemaerr.ValidationError: %v", err)
	}
	want := "The path `new_dir` is not a file."
	if ve.Message != want {
		t.Errorf("Message = %q, want %q", ve.Message, want)
	}
}

// TestPlannedPathAcceptsNonexistent covers spec §3.40: the planned type
// never fails on a nonexistent path.
func TestPlannedPathAcceptsNonexistent(t *testing.T) {
	tmp := t.TempDir()
	ctx := &valctx.ValidationContext{InputFilePath: filepath.Join(tmp, "input.yaml")}

	got, err := inputpath.ResolvePlannedPath("does_not_exist.txt", ctx)
	if err != nil {
		t.Fatalf("ResolvePlannedPath() error = %v, want nil", err)
	}
	want := filepath.Join(tmp, "does_not_exist.txt")
	if got.Value != want {
		t.Errorf("ResolvePlannedPath() = %q, want %q", got.Value, want)
	}
}

// TestPlannedPathSerializeRelativeToCwd covers spec §3.41.
func TestPlannedPathSerializeRelativeToCwd(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	p := inputpath.PlannedPathRelativeToInput{Value: filepath.Join(cwd, "build", "output.pdf")}
	got := p.Serialize()
	want := "build/output.pdf"
	if got != want {
		t.Errorf("Serialize() = %q, want %q", got, want)
	}
}

// TestPlannedPathSerializeAbsoluteFallback covers spec §3.41's fallback:
// a path outside the working directory serializes as its absolute form
// rather than failing.
func TestPlannedPathSerializeAbsoluteFallback(t *testing.T) {
	tmp := t.TempDir()
	outside := filepath.Join(tmp, "build", "output.pdf")

	p := inputpath.PlannedPathRelativeToInput{Value: outside}
	got := p.Serialize()
	if got != outside {
		t.Errorf("Serialize() = %q, want %q (absolute fallback)", got, outside)
	}
}

func TestPlannedPathSerializeEmpty(t *testing.T) {
	p := inputpath.PlannedPathRelativeToInput{}
	if got := p.Serialize(); got != "" {
		t.Errorf("Serialize() = %q, want empty", got)
	}
}

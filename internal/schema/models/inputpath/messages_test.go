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

// Spec 004 §3.17 behavior 63, §4.25 and §4.26.
//
// Both messages interpolate the path **relative to the resolution base**, not
// the absolute path — which matters because the base is a temporary directory
// in a test and the user's home directory in life, and the absolute form would
// put either in the message. Pinned upstream at `expected_errors.yaml:15`,
// whose text is `The file `+"`photo_doesnt_exist.jpg`"+` does not exist.`
func TestPathMessages(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "input.yaml")
	ctx := &valctx.ValidationContext{InputFilePath: inputFile}

	subdir := filepath.Join(dir, "assets")
	if err := os.Mkdir(subdir, 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "a file that is not there",
			input: "photo_doesnt_exist.jpg",
			want:  "The file `photo_doesnt_exist.jpg` does not exist.",
		},
		{
			// A nested path keeps its written form, so the relative rendering
			// is not just "the basename".
			name:  "a nested file that is not there",
			input: "assets/photo.jpg",
			want:  "The file `assets/photo.jpg` does not exist.",
		},
		{
			name:  "a path that exists but is a directory",
			input: "assets",
			want:  "The path `assets` is not a file.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := inputpath.ResolveExistingPath(test.input, ctx)
			if err == nil {
				t.Fatal("ResolveExistingPath succeeded")
			}

			var failure *schemaerr.ValidationError
			if !errors.As(err, &failure) {
				t.Fatalf("err = %v (%T), want *schemaerr.ValidationError", err, err)
			}
			if failure.Message != test.want {
				t.Errorf("message = %q, want %q", failure.Message, test.want)
			}
			if filepath.IsAbs(failure.Input) {
				t.Errorf("input = %q, which is absolute; it must be relative to"+
					" the resolution base", failure.Input)
			}
		})
	}
}

package cli

import (
	"path/filepath"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/generate"
)

// TestOutputFolderResolvesAgainstInputDir is G-8: upstream types
// `output_folder` `PlannedPathRelativeToInput`
// (`schema/models/settings/render_command.py:30`), so `render sub/cv.yaml -o
// pyO` writes `sub/pyO/`, not `./pyO/`. Measured before the fix: the port
// wrote beside the working directory instead.
func TestOutputFolderResolvesAgainstInputDir(t *testing.T) {
	cases := []struct {
		name    string
		options RenderOptions
		want    string
	}{
		{
			name:    "default folder, relative input",
			options: RenderOptions{InputPath: "sub/cv.yaml"},
			want:    filepath.Join("sub", DefaultOutputFolder),
		},
		{
			name:    "explicit relative folder",
			options: RenderOptions{InputPath: "sub/cv.yaml", OutputFolder: "pyO"},
			want:    filepath.Join("sub", "pyO"),
		},
		{
			name:    "input in the working directory",
			options: RenderOptions{InputPath: "cv.yaml", OutputFolder: "out"},
			want:    "out",
		},
		{
			name:    "an absolute folder is taken as given",
			options: RenderOptions{InputPath: "sub/cv.yaml", OutputFolder: "/tmp/abs-out"},
			want:    "/tmp/abs-out",
		},
	}
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			if got := generate.OutputFolderFor(row.options.InputPath, row.options.OutputFolder); filepath.ToSlash(got) != filepath.ToSlash(row.want) {
				t.Errorf("= %q, want %q", got, row.want)
			}
		})
	}
}

package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/cli"
)

// TestRenderRejectsWatch is G-10: `--watch` used to parse and do a single
// one-shot render anyway, exiting 0 as though the flag had been honored.
// Upstream never returns from `--watch` until a watched file changes, so a
// silent one-shot render answers a different question than the one asked.
// Pins that `Render` now refuses instead — no artifacts, a message on
// stderr, a non-zero exit — until the watcher itself lands (iteration 13,
// spec 012 §6.2).
func TestRenderRejectsWatch(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(input, []byte("cv:\n  name: John Doe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")

	var stdout, stderr bytes.Buffer
	code := cli.Render(cli.RenderOptions{
		InputPath: input, OutputFolder: out, Watch: true,
		NoPDF: true, NoPNG: true,
	}, &stdout, &stderr)

	if code == 0 {
		t.Error("exit code = 0, want a failure")
	}
	if stderr.Len() == 0 {
		t.Error("stderr is empty, want a not-implemented message")
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Errorf("artifacts were written despite --watch being unimplemented: %v", entries)
	}
}

// TestRenderWithoutWatchStillRenders guards against a fix that rejects every
// invocation regardless of the flag.
func TestRenderWithoutWatchStillRenders(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(input, []byte("cv:\n  name: John Doe\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := cli.Render(cli.RenderOptions{
		InputPath: input, OutputFolder: filepath.Join(dir, "out"),
		NoPDF: true, NoPNG: true,
	}, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d, stderr = %q", code, stderr.String())
	}
}

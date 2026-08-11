package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
	"time"
)

// TestWatchSet is spec 013 §3.7 behavior 47: the watched set is
// `collect_input_file_paths`' values (`run_rendercv.py:99-124`) — the input
// file always, plus `--design`, `--locale-catalog` and `--settings` when the
// command line gave them, plus `settings.render_command.design` and `.locale`
// resolved against the input file's parent when it did not. The CLI flag wins.
func TestWatchSet(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	plain := write("plain.yaml", "cv:\n  name: John Doe\n")
	named := write("named.yaml", "cv:\n  name: John Doe\nsettings:\n"+
		"  render_command:\n    design: mydesign.yaml\n    locale: mylocale.yaml\n")
	broken := write("broken.yaml", "cv:\n  name: [\n")
	design := write("design.yaml", "design:\n  theme: moderncv\n")
	locale := write("locale.yaml", "locale:\n  language: french\n")
	settings := write("settings.yaml", "settings:\n  current_date: 2025-03-05\n")

	for _, row := range []struct {
		name    string
		options RenderOptions
		want    []string
	}{
		{
			name:    "input only",
			options: RenderOptions{InputPath: plain},
			want:    []string{plain},
		},
		{
			name:    "cli design",
			options: RenderOptions{InputPath: plain, DesignPath: design},
			want:    []string{plain, design},
		},
		{
			name:    "cli locale",
			options: RenderOptions{InputPath: plain, LocalePath: locale},
			want:    []string{plain, locale},
		},
		{
			name:    "cli settings",
			options: RenderOptions{InputPath: plain, SettingsPath: settings},
			want:    []string{plain, settings},
		},
		{
			name:    "all three flags",
			options: RenderOptions{InputPath: plain, DesignPath: design, LocalePath: locale, SettingsPath: settings},
			want:    []string{plain, design, locale, settings},
		},
		{
			name:    "document names its overlays",
			options: RenderOptions{InputPath: named},
			want: []string{
				named,
				filepath.Join(dir, "mydesign.yaml"),
				filepath.Join(dir, "mylocale.yaml"),
			},
		},
		{
			name:    "cli flag wins over the document",
			options: RenderOptions{InputPath: named, DesignPath: design},
			want: []string{
				named,
				design,
				filepath.Join(dir, "mylocale.yaml"),
			},
		},
		{
			// A document that does not parse still watches its own file:
			// `collect_input_file_paths` suppresses the validation error
			// (`run_rendercv.py:113`) so watch mode can start on a broken CV.
			name:    "unparseable document",
			options: RenderOptions{InputPath: broken},
			want:    []string{broken},
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			options := row.options
			raw, err := os.ReadFile(options.InputPath)
			if err != nil {
				t.Fatal(err)
			}
			// `Render` resolves the document-named overlays before it
			// collects the set, exactly as `render_command.py:200-209` does.
			_ = resolveNamedOverlays(&options, raw)

			got := WatchSet(options)
			if len(got) != len(row.want) {
				t.Fatalf("WatchSet = %v, want %v", got, row.want)
			}
			for i, want := range row.want {
				if got[i] != want {
					t.Errorf("WatchSet[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

// TestWatchSetPathsAreAbsolute pins the other half of behavior 46: upstream
// watches `str(fp.absolute())` (`watcher.py:49`), so a relative input file
// still yields an absolute member.
func TestWatchSetPathsAreAbsolute(t *testing.T) {
	set := WatchSet(RenderOptions{InputPath: filepath.Join("sub", "cv.yaml")})
	if len(set) != 1 {
		t.Fatalf("WatchSet = %v, want one member", set)
	}
	if !filepath.IsAbs(set[0]) {
		t.Errorf("WatchSet[0] = %q, want an absolute path", set[0])
	}
}

// TestWatchLoopSurvivesFailingRender is behaviors 48 and 49: every render runs
// under `contextlib.suppress(typer.Exit)` (`watcher.py:30-31`, `:62-63`), so a
// failing render neither stops the watcher nor sets an exit code, and the loop
// ends only on the interrupt — modelled here as the context's cancellation.
func TestWatchLoopSurvivesFailingRender(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(input, []byte("cv:\n  name: John Doe\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	renders := make(chan struct{}, 8)
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		done <- watchLoop(ctx, []string{input}, func() int {
			renders <- struct{}{}
			return exitValidationError
		})
	}()

	// The first render happens before the loop (`watcher.py:62-63`).
	select {
	case <-renders:
	case err := <-done:
		t.Fatalf("watchLoop returned before the first render: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("the first render never happened")
	}

	// A bounded wait: the loop must still be running.
	select {
	case err := <-done:
		t.Fatalf("watchLoop returned %v after a failing render, want it still running", err)
	case <-time.After(300 * time.Millisecond):
	}

	// A modification of a watched file re-runs the render (behavior 49).
	if err := os.WriteFile(input, []byte("cv:\n  name: Jane Doe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-renders:
	case err := <-done:
		t.Fatalf("watchLoop returned instead of re-rendering: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("a modification of a watched file did not re-render")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("watchLoop = %v, want nil on cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watchLoop did not return on cancellation")
	}
}

// TestWatchFirstRenderMatchesRenderOnce is spec 013 §8's watcher criterion:
// `--watch` performs the first render and then blocks, and that render's
// stdout is the stdout of the same invocation without `--watch`. Durations are
// scrubbed the way the conformance harness scrubs them; nothing else is.
func TestWatchFirstRenderMatchesRenderOnce(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(input, []byte("cv:\n  name: John Doe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	options := RenderOptions{
		InputPath:    input,
		OutputFolder: filepath.Join(dir, "out"),
		NoPDF:        true,
		NoPNG:        true,
	}

	var plain bytes.Buffer
	if code := renderOnce(options, &plain, &plain); code != 0 {
		t.Fatalf("renderOnce = %d, output %q", code, plain.String())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	restore := watchContext
	watchContext = func() context.Context { return ctx }
	t.Cleanup(func() { watchContext = restore })

	watched := options
	watched.Watch = true

	var live syncBuffer
	done := make(chan int, 1)
	go func() { done <- Render(watched, &live, &live) }()

	deadline := time.Now().Add(10 * time.Second)
	for live.Len() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("--watch produced no first render")
		}
		time.Sleep(10 * time.Millisecond)
	}
	first := live.String()
	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("Render --watch = %d, want 0", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Render --watch did not return on cancellation")
	}

	if got, want := scrubDurations(first), scrubDurations(plain.String()); got != want {
		t.Errorf("first --watch render = %q, want %q", got, want)
	}
}

// watchDurationPattern is `internal/conformance`'s, spelled apart from
// panel_test.go's copy of the same regexp.
var watchDurationPattern = regexp.MustCompile(`\b\d+(\.\d+)?\s?(ms|s)\b[ \t]*`)

func scrubDurations(s string) string {
	return watchDurationPattern.ReplaceAllString(s, "<duration> ")
}

// syncBuffer is a bytes.Buffer the render goroutine writes and the test reads.
type syncBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

func (b *syncBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Len()
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

// TestRenderWithoutWatchStillRenders guards the dispatch's other branch.
func TestRenderWithoutWatchStillRenders(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(input, []byte("cv:\n  name: John Doe\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Render(RenderOptions{
		InputPath: input, OutputFolder: filepath.Join(dir, "out"),
		NoPDF: true, NoPNG: true,
	}, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit = %d, stderr = %q", code, stderr.String())
	}
}

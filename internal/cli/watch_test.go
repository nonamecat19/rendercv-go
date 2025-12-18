package cli

import (
	"bytes"
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
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
	watchContext = func() (context.Context, context.CancelFunc) { return ctx, cancel }
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

// TestWatchSetIsLexical is site 7 of the lexical-path unit: upstream's watched
// set is `{str(fp.absolute()) for fp in file_paths}` (`watcher.py:49`), and
// `Path.absolute()` only prepends the working directory — it does **not**
// clean, so a `..` segment survives into the set. `WatchSet` used
// `filepath.Abs`, which calls `Clean`.
//
// See lexicalpath_test.go's header for why the idiomatic call is the wrong one.
func TestWatchSetIsLexical(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Assembled by hand: `filepath.Join` would clean the `..` away before
	// `WatchSet` ever saw it.
	set := WatchSet(RenderOptions{InputPath: "bb/../bb/CV.yaml"})
	if len(set) != 1 {
		t.Fatalf("WatchSet = %v, want one member", set)
	}
	if want := cwd + "/bb/../bb/CV.yaml"; set[0] != want {
		t.Errorf("WatchSet[0] = %q, want %q", set[0], want)
	}
}

// TestWatchSetMixesLexicalAndResolvedPaths pins the asymmetry inside one set,
// which neither of the tests either side of it can show on its own.
//
// `collect_input_file_paths` produces **two different shapes** and upstream
// keeps them: the input file and the CLI-supplied overlays are
// `str(fp.absolute())` (`watcher.py:49`) — lexical, `..` intact — while a
// document-named overlay has already been through
// `(input_file_path.parent / rc["design"]).resolve()`
// (`run_rendercv.py:120,122`) and is therefore fully resolved, symlinks and
// all.
//
// **Do not make this uniform.** A single rule would be wrong in one direction
// or the other, and the set is what the watcher matches events against.
func TestWatchSetMixesLexicalAndResolvedPaths(t *testing.T) {
	_, lexical, _, input := symlinkTree(t)

	if err := os.WriteFile(filepath.Join(lexical, "mydesign.yaml"), []byte("design: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	options := RenderOptions{InputPath: input}
	raw := []byte("cv:\n  name: John Doe\nsettings:\n  render_command:\n    design: mydesign.yaml\n")
	if err := resolveNamedOverlays(&options, raw); err != nil {
		t.Fatalf("resolveNamedOverlays: %v", err)
	}

	set := WatchSet(options)
	if len(set) != 2 {
		t.Fatalf("WatchSet = %v, want the input and the named design", set)
	}

	// The input keeps its `..`: `Path.absolute()` cleans nothing.
	if set[0] != input {
		t.Errorf("WatchSet[0] = %q, want the lexical input %q", set[0], input)
	}
	if !strings.Contains(set[0], "/../") {
		t.Errorf("WatchSet[0] = %q, want the `..` segment kept", set[0])
	}

	// The named overlay does not: `.resolve()` has already run on it.
	wantDesign, err := filepath.EvalSymlinks(filepath.Join(lexical, "mydesign.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if set[1] != wantDesign {
		t.Errorf("WatchSet[1] = %q, want the resolved %q", set[1], wantDesign)
	}
	if strings.Contains(set[1], "/../") {
		t.Errorf("WatchSet[1] = %q, want no `..` segment", set[1])
	}
}

// TestWatchLoopWatchesTheLexicalDirectory is the behavior behind site 7, and
// the reason it is not a one-line change. Upstream schedules
// `fp.absolute().parent` (`watcher.py:52`) — the lexical parent — so with
// `work/bb` a symlink to `other/real`, `--watch work/bb/../bb/CV.yaml` watches
// `other/bb`. **fsnotify cleans the path it is given**
// (`backend_inotify.go:228` → `recursivePath` → `filepath.Clean`), so handing
// it the lexical spelling would watch `other/real` — the wrong directory — and
// the events it reported would never match the set either.
//
// So the loop resolves the directory before it registers, and matches events
// on the same basis. The observable is the one that matters: an edit to the
// file the user is actually editing re-renders, and an edit to the same-named
// file in the cleaned directory does not.
func TestWatchLoopWatchesTheLexicalDirectory(t *testing.T) {
	_, lexical, cleaned, input := symlinkTree(t)

	for _, dir := range []string{lexical, cleaned} {
		if err := os.WriteFile(filepath.Join(dir, "CV.yaml"), []byte("cv:\n  name: John Doe\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	renders := make(chan struct{}, 8)
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		done <- watchLoop(ctx, WatchSet(RenderOptions{InputPath: input}), func() int {
			renders <- struct{}{}
			return 0
		})
	}()

	select {
	case <-renders:
	case err := <-done:
		t.Fatalf("watchLoop returned before the first render: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("the first render never happened")
	}

	// The cleaned directory is not the one upstream watches: an edit there is
	// not the user's file and must not re-render.
	if err := os.WriteFile(filepath.Join(cleaned, "CV.yaml"), []byte("cv:\n  name: Wrong\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-renders:
		t.Fatalf("an edit under the cleaned path %s re-rendered", cleaned)
	case <-time.After(300 * time.Millisecond):
	}

	// The lexical one is.
	if err := os.WriteFile(filepath.Join(lexical, "CV.yaml"), []byte("cv:\n  name: Jane Doe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-renders:
	case err := <-done:
		t.Fatalf("watchLoop returned instead of re-rendering: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatalf("an edit under the lexical path %s did not re-render", lexical)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("watchLoop did not return on cancellation")
	}
}

// TestWatchDoesNotHangOnAnUnreadableInput is N1: `render --watch` on an input
// it cannot read printed the panel and then **entered the loop and blocked
// forever**. Upstream reads the input and the three overlays in
// `collect_input_file_paths` and `render_command.py:211-215`, both **before**
// `run_function_if_files_change`, so every one of these exits instead of
// watching. Measured: exit 1 upstream, a timeout here.
//
// Every case is bounded — a test for a hang that itself hangs is worse than no
// test — and each also asserts the output equals the same options rendered
// without `--watch`, so the failure is reported exactly once.
func TestWatchDoesNotHangOnAnUnreadableInput(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(valid, []byte("cv:\n  name: John Doe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	unreadable := filepath.Join(dir, "unreadable.yaml")
	if err := os.WriteFile(unreadable, []byte("design:\n  theme: classic\n"), 0o000); err != nil {
		t.Fatal(err)
	}

	for _, row := range []struct {
		name    string
		options RenderOptions
		root    bool // needs a permission root ignores
	}{
		{
			name:    "a missing input file",
			options: RenderOptions{InputPath: filepath.Join(dir, "nothere.yaml")},
		},
		{
			// `pathlib.Path.read_text` on a directory is an IsADirectoryError,
			// and `os.ReadFile` is EISDIR — neither is watchable.
			name:    "a directory as the input file",
			options: RenderOptions{InputPath: dir},
		},
		{
			name:    "a missing overlay",
			options: RenderOptions{InputPath: valid, DesignPath: filepath.Join(dir, "nothere.yaml")},
		},
		{
			name:    "an unreadable overlay",
			options: RenderOptions{InputPath: valid, DesignPath: unreadable},
			root:    true,
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			if row.root && os.Geteuid() == 0 {
				t.Skip("root ignores the read permission this case removes")
			}

			options := row.options
			options.OutputFolder = filepath.Join(t.TempDir(), "out")
			options.NoPDF, options.NoPNG = true, true

			var plain bytes.Buffer
			plainCode := renderOnce(options, &plain, &plain)
			if plainCode == 0 {
				t.Fatalf("renderOnce = 0, want a failure to compare against")
			}

			watched := options
			watched.Watch = true

			var live syncBuffer
			done := make(chan int, 1)
			go func() { done <- Render(watched, &live, &live) }()

			select {
			case code := <-done:
				if code != plainCode {
					t.Errorf("Render --watch = %d, want %d", code, plainCode)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("Render --watch never returned: the pre-loop read is missing")
			}

			if got := live.String(); got != plain.String() {
				t.Errorf("--watch output = %q, want the non-watch output %q", got, plain.String())
			}
		})
	}
}

// TestWatchKeepsWatchingAfterAValidationError is the half that must keep
// holding. Upstream's watch survives a *validation* failure — the render runs
// inside the loop and `contextlib.suppress(typer.Exit)` swallows it
// (`watcher.py:30-31`, `:62-63`) — and `--watch badtheme.yaml` blocks on both
// sides. The N1 fix is about the pre-loop read alone; it must not become
// "any failure stops the watcher", which would undo behavior 48.
func TestWatchKeepsWatchingAfterAValidationError(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")
	// Readable, parseable, and rejected by the validator.
	if err := os.WriteFile(input, []byte("cv:\n  name: John Doe\ndesign:\n  theme: nosuchtheme\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	options := RenderOptions{
		InputPath:    input,
		OutputFolder: filepath.Join(dir, "out"),
		NoPDF:        true,
		NoPNG:        true,
	}

	var plain bytes.Buffer
	if code := renderOnce(options, &plain, &plain); code == 0 {
		t.Fatalf("renderOnce = 0, want the theme to be rejected")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	restore := watchContext
	watchContext = func() (context.Context, context.CancelFunc) { return ctx, cancel }
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

	// The bounded negative: the validation failure must not have ended it.
	select {
	case code := <-done:
		t.Fatalf("Render --watch = %d after a validation error, want it still watching", code)
	case <-time.After(300 * time.Millisecond):
	}

	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("Render --watch = %d on cancellation, want 0", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Render --watch did not return on cancellation")
	}
}

// TestInterruptCancelsWatchContext pins the handler itself, not the loop:
// upstream catches `KeyboardInterrupt` (`watcher.py:65-70`) and returns
// normally, so the port's watch context must end on SIGINT rather than let the
// default disposition kill the process.
//
// The test sends the signal to itself, which is only survivable because
// `watchContext` has installed a handler by then — that is the assertion.
func TestInterruptCancelsWatchContext(t *testing.T) {
	// A second registration so a SIGINT arriving after `stop` — a lost race on
	// a loaded machine — cannot kill the test binary.
	guard := make(chan os.Signal, 1)
	signal.Notify(guard, os.Interrupt)
	t.Cleanup(func() { signal.Stop(guard) })

	ctx, stop := watchContext()
	defer stop()

	if err := ctx.Err(); err != nil {
		t.Fatalf("watchContext is already done: %v", err)
	}
	interruptSelf(t)

	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("SIGINT did not cancel the watch context")
	}
}

// TestWatchExitsZeroOnInterrupt is the measured divergence: interrupting
// `render --watch` produced byte-identical output on both sides and **exit 130
// against upstream's 0**, because nothing cancelled the watch context and Go's
// default SIGINT disposition killed the process. `watcher.py:68-70` stops and
// joins the observer and falls off the end of the function, so upstream's
// `render` returns normally and typer exits 0.
//
// 130 is a code spec 013 §6.5 defines nowhere, and T3's inventory test cannot
// see it: a signal death is not a `return`.
func TestWatchExitsZeroOnInterrupt(t *testing.T) {
	guard := make(chan os.Signal, 1)
	signal.Notify(guard, os.Interrupt)
	t.Cleanup(func() { signal.Stop(guard) })

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
		Watch:        true,
	}

	var live syncBuffer
	done := make(chan int, 1)
	go func() { done <- Render(options, &live, &live) }()

	// The signal may only be sent once the watcher has installed its handler,
	// and the first render is the observable proof that it has: `watch` calls
	// `watchContext` before `watchLoop` renders.
	deadline := time.Now().Add(10 * time.Second)
	for live.Len() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("--watch produced no first render")
		}
		time.Sleep(10 * time.Millisecond)
	}
	before := live.String()

	interruptSelf(t)

	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("Render --watch interrupted = %d, want 0 (upstream's)", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Render --watch did not return on SIGINT")
	}

	// The interrupt must not add or drop a byte: upstream's 965 bytes are the
	// first render's, and stopping the observer prints nothing.
	if after := live.String(); after != before {
		t.Errorf("stdout after the interrupt = %q, want %q", after, before)
	}
}

// interruptSelf sends SIGINT to the test process. Only a test that has already
// established a handler may call it.
func interruptSelf(t *testing.T) {
	t.Helper()
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
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

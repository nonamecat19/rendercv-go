package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// watchContext is the context the watch loop runs under, and the port's
// `except KeyboardInterrupt` (`cli/render_command/watcher.py:68-70`).
//
// **Upstream returns normally from an interrupted watch**: it catches the
// exception, stops and joins the observer, and falls off the end of the
// function, so `render --watch` interrupted with SIGINT exits **0**. The port
// left the signal on Go's default disposition, which kills the process — the
// same 965 bytes on stdout and **exit 130**, a code spec 013 §6.5 defines
// nowhere. Measured on both sides.
//
// Only SIGINT is caught, because `KeyboardInterrupt` is only SIGINT: upstream
// installs no SIGTERM handler and the port must not either.
//
// It is a variable so the handler is testable. A signal handler that only
// exists in production is how this divergence got in.
var watchContext = interruptContext

// interruptContext is watchContext's production value: a context the handler
// cancels on SIGINT. The returned stop deregisters the handler, restoring the
// default disposition for any later signal.
func interruptContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}

// WatchSet is `collect_input_file_paths`' values
// (`cli/render_command/run_rendercv.py:99-124`) as absolute paths: the input
// file always, plus each of `--design`, `--locale-catalog` and `--settings`
// that has a path.
//
// It is given the options **after** `resolveNamedOverlays` has run, because
// that is where the port keeps the other half of upstream's collection — the
// `settings.render_command.design` and `.locale` a document names for itself,
// resolved against the input file's parent when no CLI flag filled the slot
// (`run_rendercv.py:113-122`). The two halves therefore arrive in one struct
// rather than the plan's second argument.
//
// Upstream's collection is a `set` (`watcher.py:49`), so duplicates collapse
// and the order is not observable; the order here is the dict's insertion
// order all the same.
func WatchSet(options RenderOptions) []string {
	set := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)

	for _, path := range []string{
		options.InputPath,
		options.DesignPath,
		options.LocalePath,
		options.SettingsPath,
	} {
		if path == "" {
			continue
		}
		// `pathlib.Path.absolute()` prepends the working directory without
		// resolving symlinks, which is what `filepath.Abs` does.
		absolute, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if _, duplicate := seen[absolute]; duplicate {
			continue
		}
		seen[absolute] = struct{}{}
		set = append(set, absolute)
	}

	return set
}

// watchLoop is `run_function_if_files_change`
// (`cli/render_command/watcher.py:34-70`): render once, then render again
// whenever a member of set is modified, and never stop for anything else.
//
// **The parent directories are what is watched, non-recursively**
// (`watcher.py:51-58`) — upstream states file-level watching to be unreliable
// across platforms, and fsnotify's `Add(dir)` is watchdog's
// `schedule(..., recursive=False)`. The filter back down to set is ours the
// way `EventHandler.on_modified` is upstream's (`:24-31`).
//
// **A failing render is swallowed**: every call is wrapped in
// `contextlib.suppress(typer.Exit)` (`:30-31`, `:62-63`), so the exit code the
// render computed is discarded and the loop carries on. Only cancellation
// returns.
func watchLoop(ctx context.Context, set []string, render func() int) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("watch: %w", err)
	}
	defer func() { _ = watcher.Close() }()

	watched := make(map[string]struct{}, len(set))
	directories := make(map[string]struct{}, len(set))
	for _, path := range set {
		watched[path] = struct{}{}
		directories[filepath.Dir(path)] = struct{}{}
	}
	for directory := range directories {
		if err := watcher.Add(directory); err != nil {
			return fmt.Errorf("watch %s: %w", directory, err)
		}
	}

	// The observer is started before the first render (`watcher.py:59-63`), so
	// an edit made while that render runs is not missed.
	render()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			// `on_modified` only, and only for a member of the set
			// (`watcher.py:24-31`): a creation or a removal in the directory
			// is not a modification of a watched file.
			if !event.Has(fsnotify.Write) {
				continue
			}
			if _, member := watched[event.Name]; !member {
				continue
			}
			render()
		case _, ok := <-watcher.Errors:
			// An observer error is not a render failure and upstream has no
			// handler for one; the loop still may not end.
			if !ok {
				return nil
			}
		}
	}
}

// watch is `render`'s `--watch` branch (`render_command.py:232-236`): the
// watched set is collected from the resolved options, and the loop renders
// with the same closure upstream hands to `run_function_if_files_change`.
//
// The set is collected from options that have already been through
// `resolveNamedOverlays`, which is why the input file is read here rather than
// left to `renderOnce`. A read failure is not reported here — `renderOnce`
// reports it, once, in the panel, exactly as it does without `--watch`.
func watch(options RenderOptions, stdout, stderr io.Writer) int {
	resolved := options
	if raw, err := os.ReadFile(options.InputPath); err == nil {
		_ = resolveNamedOverlays(&resolved, raw)
	}

	// The handler is installed before the first render, so an interrupt during
	// it is caught rather than fatal — upstream's observer is likewise started
	// before the first render (`watcher.py:59-63`).
	ctx, stop := watchContext()
	defer stop()

	if err := watchLoop(ctx, WatchSet(resolved), func() int {
		return renderOnce(options, stdout, stderr)
	}); err != nil {
		failPanel(stdout, err)
		return exitValidationError
	}
	return 0
}

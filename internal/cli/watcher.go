package cli

import (
	"context"
	"io"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/fsnotify/fsnotify"

	"github.com/nonamecat19/rendercv-go/internal/readtext"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
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
		// **`Path.absolute()`, not `filepath.Abs`.** Upstream's set is
		// `str(fp.absolute())` (`watcher.py:49`), and `absolute()` only
		// prepends the working directory — it resolves nothing and cleans
		// nothing, so a `..` segment survives into the set. `Abs` calls
		// `Clean`, which collapses it, and through a symlinked component that
		// is a different file. See lexicalpath_test.go's header.
		absolute := path
		if !filepath.IsAbs(absolute) {
			cwd, err := os.Getwd()
			if err != nil {
				continue
			}
			absolute = design.Join(cwd, path)
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
//
// **It returns nothing because upstream reports nothing.** Neither of the two
// ways watching can fail to start reaches the user there — see the two comments
// below — so there is no error for a caller to print, and no way for one to
// print it on a stream `--quiet` does not silence.
func watchLoop(ctx context.Context, set []string, render func() int) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		// Upstream cannot report this either: watchdog builds its inotify
		// buffer inside the emitter thread (`observer.start()`), so a failure
		// there kills that thread and leaves `run_function_if_files_change`
		// rendering and sleeping exactly as before. Render once, then wait for
		// the interrupt that is the only way out.
		render()
		<-ctx.Done()
		return
	}
	defer func() { _ = watcher.Close() }()

	// **The set is upstream's spelling; the observer needs the real one.**
	// Upstream schedules `fp.absolute().parent` (`watcher.py:52`) — the
	// lexical parent — and watchdog reports events under the same string, so
	// its filter compares like with like. **fsnotify cleans what it is given**
	// (`backend_inotify.go:228` → `recursivePath` → `filepath.Clean`), so
	// handing it the lexical spelling would register the *cleaned* directory:
	// through a symlink, the wrong one, and one whose events could never match
	// the set either. Resolving both sides registers the directory upstream
	// watches and keeps the comparison consistent.
	watched := make(map[string]struct{}, len(set))
	directories := make(map[string]struct{}, len(set))
	for _, path := range set {
		directory := resolveLikePython(design.Parent(path))
		watched[filepath.Join(directory, filepath.Base(path))] = struct{}{}
		directories[directory] = struct{}{}
	}
	// **A directory that cannot be watched is not an error upstream.**
	// `observer.schedule` (`watcher.py:57`) returns normally on a directory
	// inotify refuses — measured against the vendored watchdog on mode 0311,
	// where `inotify_add_watch` itself is EACCES — leaving the observer alive
	// and silently eventless. Upstream renders and loops all the same, and
	// never reports it. Treating `Add`'s error as fatal skipped the first
	// render entirely and exited 1 where upstream renders once and blocks.
	//
	// The events of the directories that *did* register still arrive, which is
	// upstream's behavior too when only one of several fails.
	for directory := range directories {
		_ = watcher.Add(directory)
	}

	// The observer is started before the first render (`watcher.py:59-63`), so
	// an edit made while that render runs is not missed.
	render()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
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
				return
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
// left to `renderOnce`.
//
// **A file the run cannot read stops it instead of being watched** (N1).
// Upstream reads the input inside `collect_input_file_paths`
// (`run_rendercv.py:113-115`) and the three overlays at
// `render_command.py:211-215`, and **both are before**
// `run_function_if_files_change` (`:232`) — so a missing file is a
// `FileNotFoundError` and a directory an `IsADirectoryError`, each exiting 1
// without ever watching. The port entered the loop anyway and blocked forever;
// measured as a timeout against upstream's exit 1 on four vectors.
//
// **The test is the ordering, not the error class.** A *validation* failure is
// raised inside the loop on both sides and keeps the watcher up (behavior 48),
// so nothing here may generalise to "a failing render stops the watch".
func watch(options RenderOptions, stdout, stderr io.Writer) int {
	resolved := options
	// `collect_input_file_paths` reads the input with `read_text` as well
	// (`run_rendercv.py:115`), so the overlay names a CRLF document gives are
	// read out of the same translated text `renderOnce` will parse.
	raw, err := readtext.File(options.InputPath)
	if err != nil {
		// `renderOnce` reports it, once, in the panel it would have printed
		// without `--watch`, and returns the same code.
		return renderOnce(options, stdout, stderr)
	}
	_ = resolveNamedOverlays(&resolved, raw)

	// The overlays upstream reads in the same pre-loop phase — the CLI's three
	// and the two a document can name for itself, which `resolveNamedOverlays`
	// has just filled in.
	for _, overlay := range []string{resolved.DesignPath, resolved.LocalePath, resolved.SettingsPath} {
		if overlay == "" {
			continue
		}
		if _, err := overlayFile(overlay); err != nil {
			return renderOnce(options, stdout, stderr)
		}
	}

	// The handler is installed before the first render, so an interrupt during
	// it is caught rather than fatal — upstream's observer is likewise started
	// before the first render (`watcher.py:59-63`).
	ctx, stop := watchContext()
	defer stop()

	watchLoop(ctx, WatchSet(resolved), func() int {
		return renderOnce(options, stdout, stderr)
	})
	return 0
}

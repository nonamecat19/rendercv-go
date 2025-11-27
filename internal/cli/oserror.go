package cli

import (
	"errors"
	"io/fs"
	"path/filepath"
	"syscall"
)

// osErrorMessage is `run_rendercv`'s `except OSError` clause
// (`cli/render_command/run_rendercv.py:195-196`), which re-raises the failure
// as `RenderCVUserError(message=f"OS Error: {e}")` and so reaches the user
// through path B's one-message panel.
//
// **The port emitted no prefix at all**: `render cv.yaml -o <read-only dir>`
// printed `open out/John_Doe_CV.typ: permission denied` where upstream printed
// `OS Error: [Errno 13] Permission denied: '/abs/out/John_Doe_CV.typ'` —
// measured at 552 bytes against 637.
//
// **The path is made absolute** because Python's `OSError.filename` is the
// absolute path the caller opened: upstream's own paths are absolute by the
// time they reach an `open`, and a relative spelling names a different file to
// anyone reading the panel from another directory.
//
// **The errno body stays Go's.** `[Errno 13] Permission denied: '<path>'` is
// unreachable from Go's `syscall.Errno`, whose `<op> <path>: <strerror>` shape
// is fixed; that residue is spec 013 §10's P-3 and is a human-gated proposal,
// not something to fake here.
//
// The second result reports whether err was an OS error at all. Only failures
// the operating system raised get the prefix — the port's own messages, and
// validation records, must not.
func osErrorMessage(err error) (string, bool) {
	// A `*fs.PathError` is what every `os` call in the render pipeline returns:
	// the artifact writes, `ResolvePath`'s `MkdirAll`, the stale-PNG sweep's
	// `Remove` and the photo copy. `errors.As` unwraps, and the message is the
	// path error's own — upstream prints `str(e)` of the `OSError`, not of
	// whatever re-raised it.
	var pathError *fs.PathError
	if errors.As(err, &pathError) {
		reported := *pathError
		if absolute, absErr := filepath.Abs(reported.Path); absErr == nil {
			reported.Path = absolute
		}
		return "OS Error: " + reported.Error(), true
	}

	// A bare errno carries no path to absolutize. `errors.As` is checked after
	// the path error, because a `*fs.PathError` wraps one of these too.
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return "OS Error: " + errno.Error(), true
	}

	return "", false
}

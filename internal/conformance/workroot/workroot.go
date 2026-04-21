// Package workroot owns the directory a conformance case runs in.
//
// It is a package of its own, and not a constant in internal/conformance,
// because the directory has two users that must agree byte for byte:
// tools/gengolden, which records upstream's behavior there, and
// internal/conformance, which replays rendercv-go there. A second copy of the
// path would be a second contract.
//
// # Why the path is part of the parity contract
//
// Upstream prints the theme folder it could not find as
// `custom_theme_folder.absolute()`
// (third_party/rendercv/src/rendercv/schema/models/design/design.py:79, cited at
// internal/cli/render.go:173), so the `err_unknown_theme` golden records the
// absolute path of whatever directory the case ran in. While that directory was
// `testdata/.work/run/<case>` *under the generating checkout*, the goldens
// carried one machine's repository path and TestParity/err_unknown_theme could
// only pass from that exact checkout — not from a git worktree, a second clone,
// or a CI runner.
//
// Root fixes that by being the same absolute string everywhere, so the recorded
// bytes are reproducible rather than merely reproducible-in-length.
package workroot

import (
	"fmt"
	"os"
	"path/filepath"
)

// Root is the canonical directory conformance cases run under.
//
// **Changing this string changes the goldens.** It is baked into
// `testdata/golden/err_unknown_theme/stdout.txt` (and, untested, into two D-011
// tracebacks), so any edit here must be followed by `just golden` and a review
// of the diff.
//
// It is deliberately *not* derived from TMPDIR, os.TempDir or the repository
// root: each of those varies between machines, which is the defect this package
// exists to close.
const Root = "/tmp/rendercv-go-conformance"

// CaseDir is the directory case name runs in. It does not create anything.
func CaseDir(name string) string {
	return filepath.Join(Root, "run", name)
}

// Prepare empties the working directory for one case, takes the case's
// cross-process lock, and returns the directory together with the release
// function.
//
// **Release when the case is finished with the directory, not when the command
// exits.** The comparison reads the artifacts the run left behind, so releasing
// early reopens exactly the window the lock closes.
//
// The lock is what makes a fixed, checkout-independent Root safe. Before it,
// two checkouts ran a case in two different directories and could not collide;
// now they name the same one, and Prepare's first act is os.RemoveAll. Without
// the lock, a second test process (or a `go run ./tools/gengolden` overlapping a
// test run) would delete the files the first one is comparing — the failure mode
// recorded in specs/STATE.md as three false FAIL reports from one fan-out.
func Prepare(name string) (dir string, release func(), err error) {
	lock, err := acquire(name)
	if err != nil {
		return "", nil, err
	}

	dir = CaseDir(name)
	if err := os.RemoveAll(dir); err != nil {
		lock()
		return "", nil, fmt.Errorf("clearing the case directory: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		lock()
		return "", nil, fmt.Errorf("creating the case directory: %w", err)
	}
	if err := checkCanonical(dir); err != nil {
		lock()
		return "", nil, err
	}
	return dir, lock, nil
}

// acquire takes the exclusive lock for one case name and returns its release.
func acquire(name string) (func(), error) {
	lockDir := filepath.Join(Root, "locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating the conformance lock directory: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(lockDir, name+".lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("opening the lock for case %s: %w", name, err)
	}
	if err := lockFile(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("locking case %s: %w", name, err)
	}
	return func() {
		_ = unlockFile(f)
		_ = f.Close()
	}, nil
}

// checkCanonical rejects a Root that reaches the filesystem through a symlink.
//
// The recorded path comes from the *process's* working directory, and the
// operating system resolves that: on macOS `/tmp` is a symlink to `/private/tmp`,
// so a case started in /tmp/... reports /private/tmp/... and every byte of the
// golden's wrapped, ellipsized path moves. That produces a mystifying diff, so it
// is named here instead.
func checkCanonical(dir string) error {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fmt.Errorf("resolving the case directory: %w", err)
	}
	if resolved != dir {
		return fmt.Errorf(
			"the conformance work root %s resolves to %s: the goldens bake the unresolved path,"+
				" so a symlinked root cannot reproduce them (regenerate with `just golden` if this"+
				" machine's root must differ)", dir, resolved)
	}
	return nil
}

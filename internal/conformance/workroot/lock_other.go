//go:build !unix

package workroot

import "os"

// lockFile is a no-op off Unix.
//
// The harness has never run anywhere but Linux and macOS — the parity job is
// ubuntu-latest and the generator shells a POSIX virtualenv — so rather than
// carry an untested LockFileEx path, this says plainly that concurrent runs are
// unserialized there. A Windows port of the harness must replace this.
func lockFile(_ *os.File) error { return nil }

func unlockFile(_ *os.File) error { return nil }

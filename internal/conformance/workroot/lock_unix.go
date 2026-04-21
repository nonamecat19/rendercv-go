//go:build unix

package workroot

import (
	"os"

	"golang.org/x/sys/unix"
)

// lockFile blocks until this process holds the exclusive advisory lock on f.
//
// flock is released by the kernel when the file descriptor closes, including on
// an abnormal exit, so a killed test run cannot leave the corpus wedged.
func lockFile(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_EX)
}

func unlockFile(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}

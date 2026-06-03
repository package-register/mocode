//go:build !windows

package store

import (
	"os"
	"syscall"
)

// LockFile acquires an exclusive advisory lock on the given file.
// Blocks until the lock is acquired or returns an error.
func LockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// LockFileShared acquires a shared (non-exclusive) advisory lock on the given
// file. Multiple readers can hold shared locks concurrently, but writers need
// exclusive locks which block until all shared locks are released.
func LockFileShared(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_SH)
}

// UnlockFile releases the lock on the given file.
func UnlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

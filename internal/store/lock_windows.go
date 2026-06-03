//go:build windows

package store

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

const (
	lockfileExclusiveLock   = 2
	lockfileFailImmediately = 1
)

func lockFileEx(handle syscall.Handle, flags, reserved, numBytesLow, numBytesHigh uint32, overlapped *syscall.Overlapped) error {
	r, _, e := procLockFileEx.Call(
		uintptr(handle),
		uintptr(flags),
		uintptr(reserved),
		uintptr(numBytesLow),
		uintptr(numBytesHigh),
		uintptr(unsafe.Pointer(overlapped)),
	)
	if r == 0 {
		return e
	}
	return nil
}

func unlockFileEx(handle syscall.Handle, reserved, numBytesLow, numBytesHigh uint32, overlapped *syscall.Overlapped) error {
	r, _, e := procUnlockFileEx.Call(
		uintptr(handle),
		uintptr(reserved),
		uintptr(numBytesLow),
		uintptr(numBytesHigh),
		uintptr(unsafe.Pointer(overlapped)),
	)
	if r == 0 {
		return e
	}
	return nil
}

// LockFile acquires an exclusive advisory lock on the given file region
// (whole file). On Windows this uses LockFileEx.
func LockFile(f *os.File) error {
	return lockFileEx(
		syscall.Handle(f.Fd()),
		lockfileExclusiveLock,
		0,
		1, 0, // lock the entire file
		&syscall.Overlapped{},
	)
}

// LockFileShared acquires a shared (non-exclusive) advisory lock on the given
// file region (whole file). Multiple readers can hold shared locks concurrently,
// but writers need exclusive locks which block until all shared locks are released.
func LockFileShared(f *os.File) error {
	return lockFileEx(
		syscall.Handle(f.Fd()),
		lockfileFailImmediately, // shared lock: no LOCKFILE_EXCLUSIVE_LOCK
		0,
		1, 0, // lock the entire file
		&syscall.Overlapped{},
	)
}

// UnlockFile releases the lock on the given file.
func UnlockFile(f *os.File) error {
	return unlockFileEx(
		syscall.Handle(f.Fd()),
		0,
		1, 0, // unlock the entire file
		&syscall.Overlapped{},
	)
}

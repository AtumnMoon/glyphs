//go:build windows

package app

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// fileLock is an exclusive lock on a file (LockFileEx). Windows releases it
// when the owning process exits, however it exits.
type fileLock struct{ f *os.File }

// tryLock takes the lock without blocking. ok=false means another process (or
// another open handle in this one) holds it.
func tryLock(path string) (l *fileLock, ok bool, err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false, err
	}
	ol := new(windows.Overlapped)
	err = windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol)
	if err != nil {
		f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &fileLock{f}, true, nil
}

func (l *fileLock) unlock() {
	ol := new(windows.Overlapped)
	_ = windows.UnlockFileEx(windows.Handle(l.f.Fd()), 0, 1, 0, ol)
	_ = l.f.Close()
}

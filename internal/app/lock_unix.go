//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package app

import (
	"errors"
	"os"
	"syscall"
)

// fileLock is an exclusive advisory lock on a file. The kernel drops it the
// moment the owning process exits, however it exits (crash, SIGKILL, power
// loss), so a held lock always means a live owner.
type fileLock struct{ f *os.File }

// tryLock takes the lock without blocking. ok=false means another process (or
// another open handle in this one) holds it.
func tryLock(path string) (l *fileLock, ok bool, err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &fileLock{f}, true, nil
}

func (l *fileLock) unlock() {
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	_ = l.f.Close()
}

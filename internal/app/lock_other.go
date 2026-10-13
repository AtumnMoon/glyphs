//go:build !windows && !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package app

// Platforms without flock/LockFileEx get no single-instance protection: every
// tryLock succeeds.
type fileLock struct{}

func tryLock(path string) (*fileLock, bool, error) { return &fileLock{}, true, nil }

func (l *fileLock) unlock() {}

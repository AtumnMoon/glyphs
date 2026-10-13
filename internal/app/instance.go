package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Single-instance handling for the TUI and the background listener.
//
// Liveness comes from a lock file, not from a pid: while a process is alive it
// holds an exclusive OS lock on <name>.lock, and the OS releases it the moment
// that process dies. So "is it running?" is just "can I take the lock?", and a
// crash cannot leave anything stale behind.
//
// A pid is still needed to ask a running instance to quit, so it is written to
// <name>.pid. That file is only ever acted on while the lock is verifiably
// held, and only if the pid belongs to this same program, so a leftover pid
// that the OS has since reused for something else is never signalled.

// sameProgram reports whether pid is running this same executable. Where /proc
// is unavailable (macOS, Windows) there is no cheap way to tell, so it assumes
// yes; the lock check already guarantees something of ours is holding the file.
// It is a variable so tests can substitute a stand-in process.
var sameProgram = func(pid int) bool {
	exe, err := os.Executable()
	if err != nil {
		return true
	}
	link, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe")
	if err != nil {
		return true
	}
	link = strings.TrimSuffix(link, " (deleted)") // binary replaced while running
	return filepath.Base(link) == filepath.Base(exe)
}

// instance is a held single-instance lock.
type instance struct {
	lock    *fileLock
	pidPath string
	self    string
}

// release drops the lock and removes our pid record (unless a newer instance
// has already replaced it).
func (i *instance) release() {
	if b, err := os.ReadFile(i.pidPath); err == nil && strings.TrimSpace(string(b)) == i.self {
		_ = os.Remove(i.pidPath)
	}
	i.lock.unlock()
}

// holderPID returns the recorded pid if it is plausible: parseable, not us, and
// the same program.
func holderPID(pidPath string) (int, bool) {
	b, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 || pid == os.Getpid() || !sameProgram(pid) {
		return 0, false
	}
	return pid, true
}

// isHeld reports whether another handle holds the lock at lockPath.
func isHeld(lockPath string) bool {
	l, ok, err := tryLock(lockPath)
	if err != nil {
		return false
	}
	if ok {
		l.unlock()
		return false
	}
	return true
}

// acquire takes the instance lock. If another instance holds it: with replace
// it is asked to quit and acquire waits (up to wait) for the lock; without
// replace acquire fails immediately, naming the holder's pid when known.
func acquire(lockPath, pidPath, what string, replace bool, wait time.Duration) (*instance, error) {
	deadline := time.Now().Add(wait)
	signalled := false
	for {
		l, ok, err := tryLock(lockPath)
		if err != nil {
			return nil, fmt.Errorf("lock %s: %w", lockPath, err)
		}
		if ok {
			self := strconv.Itoa(os.Getpid())
			if err := os.WriteFile(pidPath, []byte(self), 0o644); err != nil {
				l.unlock()
				return nil, fmt.Errorf("write %s: %w", pidPath, err)
			}
			return &instance{lock: l, pidPath: pidPath, self: self}, nil
		}

		pid, known := holderPID(pidPath)
		if !replace {
			if known {
				return nil, fmt.Errorf("%s is already running (pid %d)", what, pid)
			}
			return nil, fmt.Errorf("%s is already running", what)
		}
		// Re-check that the lock is still held before signalling, so the pid we
		// read cannot belong to an instance that has just exited.
		if known && !signalled && isHeld(lockPath) {
			if err := terminate(pid); err != nil {
				return nil, fmt.Errorf("stop %s (pid %d): %w", what, pid, err)
			}
			signalled = true
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("another %s is running and did not exit", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// stop asks the running instance to quit and waits for its lock to clear. It
// reports whether an instance was running. Nothing is signalled when no
// instance holds the lock, whatever the pid file says.
func stop(lockPath, pidPath, what string) (bool, error) {
	if !isHeld(lockPath) {
		_ = os.Remove(pidPath) // clear a leftover record
		return false, nil
	}
	pid, known := holderPID(pidPath)
	if !known {
		return false, fmt.Errorf("%s is running but its pid is unknown; try again in a moment", what)
	}
	if err := terminate(pid); err != nil {
		return false, fmt.Errorf("stop %s (pid %d): %w", what, pid, err)
	}
	for range 60 {
		if !isHeld(lockPath) {
			return true, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false, fmt.Errorf("%s (pid %d) did not exit", what, pid)
}

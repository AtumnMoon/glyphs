package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	gclip "golang.design/x/clipboard"

	"github.com/AtumnMoon/glyphs/internal/clipboard"
	"github.com/AtumnMoon/glyphs/internal/config"
)

// NewStore opens the history store from internal/clipboard. That package keeps
// clipboard_history.json and tmp_files/ inside the config directory.
func NewStore(cfg config.Config) (*clipboard.HistoryStore, error) {
	return clipboard.NewHistory(cfg.Dir, cfg.MaxHistory)
}

// Policy builds the recording policy (duplicates, expiry, exclusions) from cfg.
func Policy(cfg config.Config) (clipboard.Policy, error) {
	ex, err := clipboard.CompilePatterns(cfg.ExcludedPatterns)
	if err != nil {
		return clipboard.Policy{}, err
	}
	return clipboard.Policy{AllowDuplicates: cfg.AllowDuplicates, DeleteAfter: cfg.DeleteAfter, Exclude: ex}, nil
}

// initClipboard prepares golang.design/x/clipboard.
func initClipboard() error {
	if err := gclip.Init(); err != nil {
		return fmt.Errorf("clipboard unavailable: %w", err)
	}
	return nil
}

// IsListenerRunning reports whether a background listener is alive, i.e.
// whether something holds the listener's lock.
func IsListenerRunning(cfg config.Config) bool { return isHeld(cfg.ListenerLockFile()) }

// Kill stops the background listener if there is one.
func Kill(cfg config.Config) (bool, error) {
	return stop(cfg.ListenerLockFile(), cfg.PIDFile(), "listener")
}

// LaunchListener starts the listener as a detached background process,
// replacing any previous one.
func LaunchListener(cfg config.Config) int {
	if _, err := Kill(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: locate executable:", err)
		return 1
	}
	logf, err := os.OpenFile(cfg.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: open log:", err)
		return 1
	}
	defer logf.Close()

	cmd := exec.Command(exe, "-listen-shell")
	cmd.Stdout, cmd.Stderr = logf, logf
	detach(cmd)
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "error: start listener:", err)
		return 1
	}
	_ = cmd.Process.Release()
	fmt.Printf("glyphs listener started (log: %s)\n", cfg.LogFilePath)
	return 0
}

// RunListener runs the listener in the foreground until interrupted. It feeds
// clipboard.Watch into the store's own Listen method, dropping data while
// paused.
func RunListener(cfg config.Config) int {
	store, err := NewStore(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if err := initClipboard(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	inst, err := acquire(cfg.ListenerLockFile(), cfg.PIDFile(), "listener", false, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v; use -kill first\n", err)
		return 1
	}
	defer inst.release()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(os.Stderr, "%s glyphs listener started (pid %d)\n", time.Now().Format(time.RFC3339), os.Getpid())
	policy, err := Policy(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	store.ListenWith(gate(gclip.Watch(ctx, gclip.FmtText, gclip.FmtImage), func() bool { return IsPaused(cfg) }), policy, os.Stderr)
	return 0
}

// gate forwards clipboard data except while paused() is true. The output
// closes when the input does.
func gate(in <-chan gclip.Data, paused func() bool) <-chan gclip.Data {
	out := make(chan gclip.Data)
	go func() {
		defer close(out)
		for d := range in {
			if paused() {
				continue
			}
			out <- d
		}
	}()
	return out
}

// ---- pause ----------------------------------------------------------------

// ParseDuration accepts Go durations ("90s", "5m") or a bare number of seconds.
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if n, err := strconv.Atoi(s); err == nil {
		if n <= 0 {
			return 0, fmt.Errorf("duration must be positive")
		}
		return time.Duration(n) * time.Second, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	return d, nil
}

// Pause suspends recording for d (the listener keeps running).
func Pause(cfg config.Config, d time.Duration) error {
	until := time.Now().Add(d).Unix()
	return os.WriteFile(cfg.PauseFile(), []byte(strconv.FormatInt(until, 10)), 0o644)
}

// Resume cancels a pause.
func Resume(cfg config.Config) error {
	if err := os.Remove(cfg.PauseFile()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsPaused reports whether recording is currently suspended.
func IsPaused(cfg config.Config) bool {
	b, err := os.ReadFile(cfg.PauseFile())
	if err != nil {
		return false
	}
	until, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() >= until {
		_ = os.Remove(cfg.PauseFile())
		return false
	}
	return true
}

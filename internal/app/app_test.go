package app

import (
	"bufio"
	"fmt"
	gclip "golang.design/x/clipboard"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/AtumnMoon/glyphs/internal/config"
)

func TestParseDuration(t *testing.T) {
	ok := map[string]time.Duration{"30": 30 * time.Second, "90s": 90 * time.Second, "5m": 5 * time.Minute, "1h30m": 90 * time.Minute}
	for in, want := range ok {
		if got, err := ParseDuration(in); err != nil || got != want {
			t.Errorf("ParseDuration(%q) = %v, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "0", "-5m", "abc", "5x"} {
		if _, err := ParseDuration(bad); err == nil {
			t.Errorf("ParseDuration(%q) should fail", bad)
		}
	}
}

func TestPauseResume(t *testing.T) {
	cfg := config.Default()
	cfg.Dir = t.TempDir()
	if IsPaused(cfg) {
		t.Fatal("should not start paused")
	}
	if err := Pause(cfg, time.Minute); err != nil {
		t.Fatal(err)
	}
	if !IsPaused(cfg) {
		t.Error("should be paused")
	}
	if err := Resume(cfg); err != nil {
		t.Fatal(err)
	}
	if IsPaused(cfg) {
		t.Error("should be resumed")
	}
	// an already-expired pause clears itself
	if err := Pause(cfg, -time.Second); err != nil {
		t.Fatal(err)
	}
	if IsPaused(cfg) {
		t.Error("expired pause should not count")
	}
}

func TestKillWithoutListener(t *testing.T) {
	cfg := config.Default()
	cfg.Dir = t.TempDir()
	stopped, err := Kill(cfg)
	if err != nil || stopped {
		t.Errorf("Kill() = %v, %v; want false, nil", stopped, err)
	}
	if IsListenerRunning(cfg) {
		t.Error("no listener should be running")
	}
}

func TestGateDropsDataWhilePaused(t *testing.T) {
	in := make(chan gclip.Data)
	calls := 0
	out := gate(in, func() bool { calls++; return calls == 2 })
	go func() {
		for _, s := range []string{"a", "b", "c"} {
			in <- gclip.Data{Format: gclip.FmtText, Bytes: []byte(s)}
		}
		close(in)
	}()
	var got []string
	for d := range out {
		got = append(got, string(d.Bytes))
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Errorf("got %v, want [a c]", got)
	}
}

// ---- single-instance handling -------------------------------------------------

// The tests below need a *live* process that really holds an instance lock. The
// test binary re-executes itself in "holder" mode for that: TestMain sees
// GLYPHS_TEST_HOLD, takes the lock, prints "held" and waits to be terminated.
func TestMain(m *testing.M) {
	if kind := os.Getenv("GLYPHS_TEST_HOLD"); kind != "" {
		cfg := config.Default()
		cfg.Dir = os.Getenv("GLYPHS_TEST_DIR")
		lock, pid := cfg.TUILockFile(), cfg.TUIPIDFile()
		if kind == "listener" {
			lock, pid = cfg.ListenerLockFile(), cfg.PIDFile()
		}
		if _, err := acquire(lock, pid, kind, false, 0); err != nil {
			fmt.Println("error:", err)
			os.Exit(1)
		}
		fmt.Println("held")
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type holder struct {
	cmd  *exec.Cmd
	done chan struct{}
}

func startHolder(t *testing.T, dir, kind string) *holder {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("holder process is terminated with a unix signal")
	}
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "GLYPHS_TEST_HOLD="+kind, "GLYPHS_TEST_DIR="+dir)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	h := &holder{cmd: cmd, done: make(chan struct{})}
	go func() { _ = cmd.Wait(); close(h.done) }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	ready := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(out).ReadString('\n')
		ready <- strings.TrimSpace(line)
	}()
	select {
	case line := <-ready:
		if line != "held" {
			t.Fatalf("holder did not start: %q", line)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("holder process never reported the lock")
	}
	return h
}

func (h *holder) exited(within time.Duration) bool {
	select {
	case <-h.done:
		return true
	case <-time.After(within):
		return false
	}
}

func TestClaimTUIReplacesRunningInstance(t *testing.T) {
	cfg := config.Default()
	cfg.Dir = t.TempDir()
	old := startHolder(t, cfg.Dir, "tui")

	release, err := claimTUI(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if !old.exited(5 * time.Second) {
		t.Fatal("the previous TUI was not stopped")
	}
	if b, _ := os.ReadFile(cfg.TUIPIDFile()); string(b) != strconv.Itoa(os.Getpid()) {
		t.Errorf("pid file = %q, want our pid", b)
	}
}

func TestSecondInstanceCanBeRefusedInsteadOfReplacing(t *testing.T) {
	cfg := config.Default()
	cfg.Dir = t.TempDir()
	h := startHolder(t, cfg.Dir, "tui")
	_, err := acquire(cfg.TUILockFile(), cfg.TUIPIDFile(), "glyphs TUI", false, 0)
	if err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("want an 'already running' error, got %v", err)
	}
	if h.exited(200 * time.Millisecond) {
		t.Error("without replace the running instance must be left alone")
	}
}

func TestStalePidFileNeverSignalsAnything(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a unix `sleep` as an unrelated live process")
	}
	cfg := config.Default()
	cfg.Dir = t.TempDir()
	other := exec.Command("sleep", "60") // alive, unrelated to glyphs
	if err := other.Start(); err != nil {
		t.Skip("cannot start sleep:", err)
	}
	defer other.Process.Kill()
	// a crashed instance left its pid file behind and the OS gave that pid to
	// `sleep`; no lock is held, so nothing may be signalled
	pidText := strconv.Itoa(other.Process.Pid)
	for _, f := range []string{cfg.TUIPIDFile(), cfg.PIDFile()} {
		if err := os.WriteFile(f, []byte(pidText), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	release, err := claimTUI(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if stopped, err := Kill(cfg); err != nil || stopped {
		t.Errorf("Kill() = %v, %v; want false, nil", stopped, err)
	}
	if IsListenerRunning(cfg) {
		t.Error("a stale pid file must not count as a running listener")
	}
	if err := other.Process.Signal(syscall.Signal(0)); err != nil {
		t.Errorf("an unrelated process was terminated: %v", err)
	}
}

func TestListenerLockSurvivesTUIClaimAndKillStopsIt(t *testing.T) {
	cfg := config.Default()
	cfg.Dir = t.TempDir()
	l := startHolder(t, cfg.Dir, "listener")
	if !IsListenerRunning(cfg) {
		t.Fatal("a process holding the listener lock should count as running")
	}
	old := startHolder(t, cfg.Dir, "tui")
	release, err := claimTUI(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if !old.exited(5*time.Second) || l.exited(200*time.Millisecond) {
		t.Fatal("opening the TUI must replace the old TUI and leave the listener alone")
	}
	stopped, err := Kill(cfg)
	if err != nil || !stopped {
		t.Fatalf("Kill() = %v, %v; want true, nil", stopped, err)
	}
	if !l.exited(5*time.Second) || IsListenerRunning(cfg) {
		t.Error("Kill should stop the listener and free its lock")
	}
}

func TestReleaseKeepsNewerInstancesRecord(t *testing.T) {
	cfg := config.Default()
	cfg.Dir = t.TempDir()
	release, err := claimTUI(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// a newer instance took over the record while this one was still exiting
	if err := os.WriteFile(cfg.TUIPIDFile(), []byte("999999"), 0o644); err != nil {
		t.Fatal(err)
	}
	release()
	if b, _ := os.ReadFile(cfg.TUIPIDFile()); string(b) != "999999" {
		t.Errorf("release deleted another instance's record: %q", b)
	}
}

func TestLockIsFreedWhenOwnerIsKilled(t *testing.T) {
	cfg := config.Default()
	cfg.Dir = t.TempDir()
	h := startHolder(t, cfg.Dir, "tui")
	if !isHeld(cfg.TUILockFile()) {
		t.Fatal("lock should be held by the helper")
	}
	_ = h.cmd.Process.Kill() // no cleanup runs: the pid file stays behind
	if !h.exited(5 * time.Second) {
		t.Fatal("helper did not die")
	}
	if isHeld(cfg.TUILockFile()) {
		t.Error("the OS must release the lock when its owner dies")
	}
	release, err := claimTUI(cfg) // and a new instance starts without any kill
	if err != nil {
		t.Fatal(err)
	}
	release()
}

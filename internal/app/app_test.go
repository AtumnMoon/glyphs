package app

import (
	gclip "golang.design/x/clipboard"
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

package cmd

import (
	"testing"

	"github.com/AtumnMoon/glyphs/internal/config"
)

func testConfig(t *testing.T) config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.Dir = t.TempDir()
	return cfg
}

func TestHandlePause(t *testing.T) {
	cfg := testConfig(t)
	if rc := handlePause(cfg, "5m"); rc != 0 {
		t.Errorf("valid duration rc=%d", rc)
	}
	if rc := handlePause(cfg, "off"); rc != 0 {
		t.Errorf("off rc=%d", rc)
	}
	if rc := handlePause(cfg, "resume"); rc != 0 {
		t.Errorf("resume rc=%d", rc)
	}
	for _, bad := range []string{"abc", "-5m", "0"} {
		if rc := handlePause(cfg, bad); rc != 1 {
			t.Errorf("handlePause(%q) rc=%d, want 1", bad, rc)
		}
	}
}

func TestModifiersDoNotCountAsCommands(t *testing.T) {
	for _, m := range []string{"tab", "keep"} {
		if !modifiers[m] {
			t.Errorf("%q should be a modifier flag", m)
		}
	}
	if modifiers["listen"] {
		t.Error("listen is a command, not a modifier")
	}
}

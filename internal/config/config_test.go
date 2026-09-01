package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaults(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "glyphs")
	cfg, err := LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); err != nil {
		t.Fatalf("config.json not written: %v", err)
	}
	if cfg.MaxHistory != 100 || !cfg.QuitAfterCopy {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if got := cfg.Keys("quit"); len(got) != 2 || got[0] != "q" || got[1] != "esc" {
		t.Errorf("quit keys = %v", got)
	}
}

func TestPartialConfigKeepsDefaults(t *testing.T) {
	dir := t.TempDir()
	body := `{"maxHistory": 5, "keyBindings": {"quit": "x"}, "theme": {"accent": "#123456"}, "defaultTab": "Emoji"}`
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxHistory != 5 {
		t.Errorf("maxHistory = %d", cfg.MaxHistory)
	}
	if got := cfg.Keys("quit"); len(got) != 1 || got[0] != "x" {
		t.Errorf("quit override lost: %v", got)
	}
	if got := cfg.Keys("choose"); len(got) != 1 || got[0] != "enter" {
		t.Errorf("unspecified binding should keep default, got %v", got)
	}
	if cfg.Theme.Accent != "#123456" || cfg.Theme.Pin != DefaultTheme().Pin {
		t.Errorf("theme merge wrong: %+v", cfg.Theme)
	}
	if cfg.DefaultTab != "emoji" {
		t.Errorf("defaultTab = %q", cfg.DefaultTab)
	}
}

func TestBrokenConfigFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(dir)
	if err == nil {
		t.Fatal("expected a parse error to be reported")
	}
	if cfg.MaxHistory != 100 {
		t.Errorf("should fall back to defaults, got %+v", cfg)
	}
}

func TestResolvePath(t *testing.T) {
	t.Setenv("GLYPHS_TEST_DIR", "/opt/x")
	home, _ := os.UserHomeDir()
	cases := map[string]string{
		"hist.json":          filepath.Join("/base", "hist.json"),
		"/abs/hist.json":     filepath.Clean("/abs/hist.json"),
		"$GLYPHS_TEST_DIR/h": filepath.Clean("/opt/x/h"),
		"~/glyphs/h.json":    filepath.Join(home, "glyphs/h.json"),
		"sub/../hist.json":   filepath.Join("/base", "hist.json"),
	}
	for in, want := range cases {
		if got := ResolvePath(in, "/base"); got != want {
			t.Errorf("ResolvePath(%q) = %q, want %q", in, got, want)
		}
	}
}

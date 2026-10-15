package config

import (
	"os"
	"path/filepath"
	"strings"
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
	// Built from t.TempDir() so the expectations hold on Windows too, where
	// "/abs" is not an absolute path (no drive letter).
	base := t.TempDir()
	abs := filepath.Join(t.TempDir(), "abs", "hist.json")
	t.Setenv("GLYPHS_TEST_DIR", filepath.Dir(abs))
	home, _ := os.UserHomeDir()
	cases := map[string]string{
		"hist.json":          filepath.Join(base, "hist.json"),
		abs:                  abs,
		"$GLYPHS_TEST_DIR/h": filepath.Join(filepath.Dir(abs), "h"),
		"~/glyphs/h.json":    filepath.Join(home, "glyphs", "h.json"),
		"sub/../hist.json":   filepath.Join(base, "hist.json"),
	}
	for in, want := range cases {
		if got := ResolvePath(in, base); got != want {
			t.Errorf("ResolvePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestThemeOptionalColoursInherit(t *testing.T) {
	th := DefaultTheme()
	th.Selected = "#111111"
	th.Muted = "#222222"
	r := th.Resolve()
	if r.SelectedDesc != "#111111" || r.SelectedPin != "#111111" {
		t.Errorf("selected* should inherit selected: %+v", r)
	}
	if r.Placeholder != "#222222" || r.Description != "#222222" {
		t.Errorf("placeholder/description should inherit muted: %+v", r)
	}
	if r.HelpKey != "" {
		t.Errorf("help colours must stay empty (library default), got %q", r.HelpKey)
	}
}

func TestThemeOptionalColoursOverride(t *testing.T) {
	dir := t.TempDir()
	body := `{"theme": {"selectedDesc": "#abcdef", "placeholder": " #123456 ", "helpKey": "#fff"}}`
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := cfg.Theme.Resolve()
	if r.SelectedDesc != "#abcdef" || r.Placeholder != "#123456" || r.HelpKey != "#fff" {
		t.Errorf("overrides lost: %+v", r)
	}
	if r.SelectedPin != DefaultTheme().Selected {
		t.Errorf("untouched key should still inherit, got %q", r.SelectedPin)
	}
}

func TestUIDefaultsAndPartialOverride(t *testing.T) {
	dir := t.TempDir()
	body := `{"ui": {"pinMarker": "", "selectMarker": "*", "searchPlaceholder": "find…",
		"tabNames": {"Emoji": "Faces", "bogus": "x", "kaomoji": "  "}, "boldSelected": false}}`
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	u, d := cfg.UI, DefaultUI()
	if u.PinMarker != "" {
		t.Errorf("an explicitly empty pinMarker must hide the marker, got %q", u.PinMarker)
	}
	if u.SelectMarker != "*" || u.SearchPlaceholder != "find…" || u.BoldSelected {
		t.Errorf("overrides lost: %+v", u)
	}
	if u.SearchIdle != d.SearchIdle || u.Divider != d.Divider || !u.BoldActiveTab {
		t.Errorf("unspecified keys should keep defaults: %+v", u)
	}
	if u.TabNames["emoji"] != "Faces" || u.TabNames["kaomoji"] != "Kaomoji" || u.TabNames["clipboard"] != "Clipboard" {
		t.Errorf("tabNames merge wrong: %v", u.TabNames)
	}
	if _, ok := u.TabNames["bogus"]; ok {
		t.Error("unknown tab names should be dropped")
	}
}

func TestUIStringsAreSingleLine(t *testing.T) {
	u := UI{TabFormat: "  ", HistoryTitle: "", SearchIdle: "a\nb"}.resolve()
	if u.TabFormat != DefaultUI().TabFormat || u.HistoryTitle != "History" {
		t.Errorf("blank tabFormat/historyTitle should fall back: %+v", u)
	}
	if u.SearchIdle != "a b" {
		t.Errorf("newlines should be flattened, got %q", u.SearchIdle)
	}
}

func TestFirstRunWritesNewKeys(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "glyphs")
	if _, err := LoadFrom(dir); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, FileName))
	for _, want := range []string{`"selectedDesc"`, `"placeholder"`, `"pinMarker"`, `"searchPlaceholder"`, `"tabFormat"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("default config.json is missing %s", want)
		}
	}
}

// A user's "Emoji" must beat the default "emoji" every time, not just when
// Go's random map order happens to visit it last.
func TestTabNamesOverrideIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	body := `{"ui": {"tabNames": {"Emoji": "Faces", "clipboard": "Paste"}}}`
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	for range 300 {
		cfg, err := LoadFrom(dir)
		if err != nil {
			t.Fatal(err)
		}
		if n := cfg.UI.TabNames; n["emoji"] != "Faces" || n["clipboard"] != "Paste" || n["kaomoji"] != "Kaomoji" {
			t.Fatalf("tabNames = %v", n)
		}
	}
}

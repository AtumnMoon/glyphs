// Package config loads glyphs' user configuration (config.json) and resolves
// the on-disk locations the app owns (config dir, log, pid and pause files).
// The clipboard history file and image folder are managed by package
// clipboard inside Dir.
//
// The design follows clipse: a JSON file is created with defaults on first
// run, any key missing from the file silently falls back to its default, and
// path values may be absolute, "~"-prefixed, contain $ENV vars, or be relative
// to the config directory.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// AppDir is the directory name created inside the user's config dir.
	AppDir = "glyphs"
	// FileName is the config file name inside AppDir.
	FileName = "config.json"
	// PIDFileName holds the PID of the background listener.
	PIDFileName = "listener.pid"
	// PauseFileName holds the unix time (seconds) until which listening is paused.
	PauseFileName = "paused_until"
)

// Config is the on-disk configuration shape.
type Config struct {
	AllowDuplicates   bool              `json:"allowDuplicates"`
	MaxHistory        int               `json:"maxHistory"`
	DeleteAfter       int               `json:"deleteAfter"` // seconds; 0 disables
	ExcludedPatterns  []string          `json:"excludedPatterns"`
	LogFilePath       string            `json:"logFile"`
	MaxEntryLength    int               `json:"maxEntryLength"`
	KeyBindings       map[string]string `json:"keyBindings"`
	Theme             Theme             `json:"theme"`
	EnableMouse       bool              `json:"enableMouse"`
	EnableDescription bool              `json:"enableDescription"`
	QuitAfterCopy     bool              `json:"quitAfterCopy"`
	DefaultTab        string            `json:"defaultTab"` // kaomoji | emoji | clipboard

	// Dir is the resolved config directory (not serialised).
	Dir string `json:"-"`
}

// Default returns the built-in configuration.
func Default() Config {
	return Config{
		AllowDuplicates:   false,
		MaxHistory:        100,
		DeleteAfter:       0,
		ExcludedPatterns:  []string{},
		LogFilePath:       "glyphs.log",
		MaxEntryLength:    65,
		KeyBindings:       DefaultKeyBindings(),
		Theme:             DefaultTheme(),
		EnableMouse:       true,
		EnableDescription: true,
		QuitAfterCopy:     true,
		DefaultTab:        "kaomoji",
	}
}

// DefaultKeyBindings lists every rebindable action. Values may hold several
// keys separated by commas, e.g. "q,esc".
func DefaultKeyBindings() map[string]string {
	return map[string]string{
		"up":            "up,k",
		"down":          "down,j",
		"left":          "left,h",
		"right":         "right,l",
		"nextTab":       "tab",
		"prevTab":       "shift+tab",
		"choose":        "enter",
		"filter":        "/",
		"preview":       "space",
		"togglePin":     "p",
		"togglePinned":  "ctrl+p",
		"remove":        "backspace,delete",
		"selectSingle":  "s",
		"selectUp":      "shift+up",
		"selectDown":    "shift+down",
		"clearSelected": "S",
		"home":          "home,g",
		"end":           "end,G",
		"skinTone":      "t",
		"more":          "?",
		"quit":          "q,esc",
		"forceQuit":     "ctrl+c",
	}
}

// Dir returns the glyphs config directory, honouring $GLYPHS_CONFIG_DIR first
// and then the OS user-config dir ($XDG_CONFIG_HOME, ~/.config, ...).
func Dir() (string, error) {
	if d := os.Getenv("GLYPHS_CONFIG_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config: locate user config dir: %w", err)
	}
	return filepath.Join(base, AppDir), nil
}

// Load reads (creating it with defaults if absent) config.json from the
// standard directory and returns the resolved configuration.
func Load() (Config, error) {
	dir, err := Dir()
	if err != nil {
		return Default(), err
	}
	return LoadFrom(dir)
}

// LoadFrom is Load for an explicit directory; it also creates the history
// file and temp dir so callers can use them immediately.
func LoadFrom(dir string) (Config, error) {
	cfg := Default()
	cfg.Dir = dir

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return cfg, fmt.Errorf("config: create %s: %w", dir, err)
	}

	path := filepath.Join(dir, FileName)
	raw, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		out, mErr := json.MarshalIndent(cfg, "", "    ")
		if mErr != nil {
			return cfg, mErr
		}
		if wErr := os.WriteFile(path, append(out, '\n'), 0o644); wErr != nil {
			return cfg, fmt.Errorf("config: write defaults: %w", wErr)
		}
	case err != nil:
		return cfg, fmt.Errorf("config: read %s: %w", path, err)
	default:
		// Unmarshal over the defaults: keys that are absent keep their default.
		// keyBindings/theme are re-merged below so blank values are ignored too.
		if uErr := json.Unmarshal(raw, &cfg); uErr != nil {
			// Keep running on defaults, but tell the caller.
			cfg = Default()
			cfg.Dir = dir
			return resolve(cfg), fmt.Errorf("config: parse %s (using defaults): %w", path, uErr)
		}
		cfg.KeyBindings = mergeBindings(raw)
		cfg.Theme = mergeTheme(raw)
	}

	cfg.Dir = dir
	return resolve(cfg), nil
}

// mergeBindings overlays the user's keyBindings block on the defaults.
func mergeBindings(raw []byte) map[string]string {
	out := DefaultKeyBindings()
	var probe struct {
		KeyBindings map[string]string `json:"keyBindings"`
	}
	if json.Unmarshal(raw, &probe) == nil {
		for k, v := range probe.KeyBindings {
			if strings.TrimSpace(v) != "" {
				out[k] = v
			}
		}
	}
	return out
}

// mergeTheme overlays the user's theme block on the default theme.
func mergeTheme(raw []byte) Theme {
	out := DefaultTheme()
	var probe struct {
		Theme map[string]string `json:"theme"`
	}
	if json.Unmarshal(raw, &probe) == nil {
		out.apply(probe.Theme)
	}
	return out
}

func resolve(cfg Config) Config {
	cfg.LogFilePath = ResolvePath(cfg.LogFilePath, cfg.Dir)
	if cfg.MaxHistory < 0 {
		cfg.MaxHistory = 0
	}
	if cfg.MaxEntryLength < 10 {
		cfg.MaxEntryLength = 65
	}
	switch strings.ToLower(cfg.DefaultTab) {
	case "kaomoji", "emoji", "clipboard":
		cfg.DefaultTab = strings.ToLower(cfg.DefaultTab)
	default:
		cfg.DefaultTab = "kaomoji"
	}
	return cfg
}

// ResolvePath expands "~", $ENV vars and makes relative paths relative to base.
func ResolvePath(p, base string) string {
	if p == "" {
		return p
	}
	p = os.ExpandEnv(p)
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(base, p)
	}
	return filepath.Clean(p)
}

// PIDFile is the path of the listener's pid file.
func (c Config) PIDFile() string { return filepath.Join(c.Dir, PIDFileName) }

// PauseFile is the path of the pause marker file.
func (c Config) PauseFile() string { return filepath.Join(c.Dir, PauseFileName) }

// Keys returns the list of keys bound to action (split on commas).
func (c Config) Keys(action string) []string {
	raw, ok := c.KeyBindings[action]
	if !ok {
		raw = DefaultKeyBindings()[action]
	}
	var out []string
	for _, k := range strings.Split(raw, ",") {
		if k = strings.TrimSpace(k); k != "" {
			out = append(out, k)
		}
	}
	return out
}

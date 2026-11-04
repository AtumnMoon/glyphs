// Package config loads glyphs' user configuration (config.toml) and resolves
// the on-disk locations the app owns (config dir, log, pid and pause files).
// The clipboard history file and image folder are managed by package
// clipboard inside Dir.
//
// A commented config.toml is created on first run. Any option missing from
// the file keeps its default. Path values may be absolute, "~"-prefixed,
// contain $ENV vars, or be relative to the config directory.
//
// A file that cannot be read is reported as an *Error carrying the line and
// column; Load then returns the defaults next to it so a caller that can
// afford to (the TUI) keeps going, while one that must not (the listener,
// whose excluded_patterns would be lost) can refuse to run.
package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
)

const (
	// AppDir is the directory name created inside the user's config dir.
	AppDir = "glyphs"
	// FileName is the config file name inside AppDir.
	FileName = "config.toml"
	// PIDFileName holds the PID of the background listener.
	PIDFileName = "listener.pid"
	// LockName are the lock files that prove an instance is alive
	ListenerLockName = "listener.lock"
	TUILockName      = "tui.lock"
	// TUIPIDFileName holds the PID of the running TUI, so only one runs at a time.
	TUIPIDFileName = "tui.pid"
	// PauseFileName holds the unix time (seconds) until which listening is paused.
	PauseFileName = "paused_until"
)

// defaultTemplate is the commented config.toml written on first run. A test checks
// that it parses to Default() and that every commented default matches.
//
//go:embed default_config.toml
var defaultTemplate string

// Config is the on-disk configuration shape.
type Config struct {
	AllowDuplicates   bool                `toml:"allow_duplicates"`
	MaxHistory        int                 `toml:"max_history"`
	DeleteAfter       int                 `toml:"delete_after"` // seconds; 0 disables
	ExcludedPatterns  []string            `toml:"excluded_patterns"`
	LogFilePath       string              `toml:"log_file"`
	MaxEntryLength    int                 `toml:"max_entry_length"`
	KeyBindings       map[string][]string `toml:"keys"`
	Theme             Theme               `toml:"theme"`
	UI                UI                  `toml:"ui"`
	EnableMouse       bool                `toml:"enable_mouse"`
	EnableDescription bool                `toml:"enable_description"`
	QuitAfterCopy     bool                `toml:"quit_after_copy"`
	DefaultTab        string              `toml:"default_tab"`    // clipboard | kaomoji | emoji
	SearchOnOpen      []string            `toml:"search_on_open"` // tabs whose search box starts focused
	ImagePreview      string              `toml:"image_preview"`  // auto | sixel | basic | off
	SimpleEmoji       bool                `toml:"simple_emoji"`   // hide joined (ZWJ) emoji sequences
	EmojiColumns      int                 `toml:"emoji_columns"`  // max emoji per grid row (0 = fill the window)
	EmojiRows         int                 `toml:"emoji_rows"`     // max emoji grid rows shown at once (0 = fill the window)

	// Dir is the resolved config directory (not serialised).
	Dir string `toml:"-"`
	// Warnings are problems that do not stop the config from loading:
	// unknown options, out-of-range values that were corrected, ...
	Warnings []string `toml:"-"`
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
		UI:                DefaultUI(),
		EnableMouse:       true,
		EnableDescription: true,
		QuitAfterCopy:     true,
		DefaultTab:        "clipboard",
		SearchOnOpen:      []string{"emoji", "kaomoji"},
		ImagePreview:      "auto",
		EmojiColumns:      16,
		EmojiRows:         16,
	}
}

// DefaultKeyBindings lists every rebindable action and its keys.
func DefaultKeyBindings() map[string][]string {
	return map[string][]string{
		"up":             {"up", "k"},
		"down":           {"down", "j"},
		"left":           {"left", "h"},
		"right":          {"right", "l"},
		"next_tab":       {"tab"},
		"prev_tab":       {"shift+tab"},
		"choose":         {"enter"},
		"filter":         {"/"},
		"preview":        {"space"},
		"toggle_pin":     {"p"},
		"toggle_pinned":  {"ctrl+p"},
		"remove":         {"x", "backspace", "delete"},
		"select_single":  {"s"},
		"select_up":      {"shift+up"},
		"select_down":    {"shift+down"},
		"clear_selected": {"S"},
		"home":           {"home", "g"},
		"end":            {"end", "G"},
		"skin_tone":      {"t"},
		"cycle_level":    {"L"},
		"select_mode":    {"v"},
		"move_mode":      {"m"},
		"mode_exit":      {"esc"},
		"move_confirm":   {"enter"},
		"more":           {"?"},
		"quit":           {"q", "esc"},
		"force_quit":     {"ctrl+c"},
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

// Error is a config file that could not be used. Line and Col are 1-based and
// zero when the problem has no position (e.g. a value of the wrong type).
type Error struct {
	Path      string
	Line, Col int
	Msg       string
	Context   string // a few lines of the file around the problem, if known
}

func (e *Error) Error() string {
	loc := e.Path
	switch {
	case e.Line > 0 && e.Col > 0:
		loc = fmt.Sprintf("%s:%d:%d", e.Path, e.Line, e.Col)
	case e.Line > 0:
		loc = fmt.Sprintf("%s:%d", e.Path, e.Line)
	}
	return "config: " + loc + ": " + e.Msg
}

// Load reads config.toml from the standard directory (writing a commented
// default on first run) and returns the resolved configuration. On error the
// returned Config holds the defaults.
func Load() (Config, error) {
	dir, err := Dir()
	if err != nil {
		return resolve(Default()), err
	}
	return LoadFrom(dir)
}

// LoadFrom is Load for an explicit directory.
func LoadFrom(dir string) (Config, error) {
	defaults := Default()
	defaults.Dir = dir

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return resolve(defaults), fmt.Errorf("config: create %s: %w", dir, err)
	}

	path := filepath.Join(dir, FileName)
	raw, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if wErr := os.WriteFile(path, []byte(defaultTemplate), 0o644); wErr != nil {
			return resolve(defaults), fmt.Errorf("config: write defaults: %w", wErr)
		}
		if _, sErr := os.Stat(filepath.Join(dir, "config.json")); sErr == nil {
			defaults.Warnings = append(defaults.Warnings,
				"found an old config.json, which is no longer read: glyphs now uses "+path+" (option names are snake_case)")
		}
		return resolve(defaults), nil
	case err != nil:
		return resolve(defaults), fmt.Errorf("config: read %s: %w", path, err)
	}

	cfg, err := parse(string(raw), path, dir)
	if err != nil {
		return resolve(defaults), err
	}
	return cfg, nil
}

// parse decodes src over the defaults.
func parse(src, path, dir string) (Config, error) {
	cfg := Default()
	// These three are decoded from empty so they hold only what the user
	// wrote, then overlaid on the defaults below: that keeps a blank colour
	// or an empty key list meaning "not set", and makes the result
	// independent of map iteration order.
	cfg.KeyBindings = nil
	cfg.Theme = Theme{}
	cfg.UI.TabNames = nil

	md, err := toml.Decode(src, &cfg)
	if err != nil {
		return Config{}, newError(path, src, err)
	}

	var warn []string
	for _, k := range md.Undecoded() {
		warn = append(warn, unknownOption(k.String()))
	}

	cfg.Theme = DefaultTheme().overlay(cfg.Theme)
	keys, kw := overlayKeys(cfg.KeyBindings)
	cfg.KeyBindings = keys
	warn = append(warn, kw...)
	if cfg.UI.TabNames == nil {
		cfg.UI.TabNames = DefaultUI().TabNames
	}

	cfg.Dir = dir
	cfg = resolve(cfg)
	cfg.Warnings = append(warn, cfg.Warnings...)
	return cfg, nil
}

// typeErr matches the errors BurntSushi/toml raises when a value has the wrong
// type: it names the line and the option but gives no column.
var typeErr = regexp.MustCompile(`^toml: line (\d+) \(last key "([^"]*)"\): (.*)$`)

func newError(path, src string, err error) *Error {
	var pe toml.ParseError
	if errors.As(err, &pe) {
		ctx := pe.ErrorWithPosition()
		if i := strings.Index(ctx, "At line"); i >= 0 { // drop the header that repeats Msg
			ctx = strings.TrimRight(ctx[i:], "\n")
		}
		return &Error{Path: path, Line: pe.Position.Line, Col: pe.Position.Col, Msg: pe.Message, Context: ctx}
	}
	if m := typeErr.FindStringSubmatch(err.Error()); m != nil {
		line, _ := strconv.Atoi(m[1])
		e := &Error{Path: path, Line: line, Msg: fmt.Sprintf("option %q: %s", m[2], m[3])}
		if lines := strings.Split(src, "\n"); line >= 1 && line <= len(lines) {
			e.Context = fmt.Sprintf("At line %d:\n\n%6d | %s", line, line, lines[line-1])
		}
		return e
	}
	return &Error{Path: path, Msg: strings.TrimPrefix(err.Error(), "toml: ")}
}

// unknownOption words the warning for an option the program does not have. A
// camelCase name is almost certainly a leftover from the old JSON config.
func unknownOption(key string) string {
	leaf := key[strings.LastIndex(key, ".")+1:]
	if strings.IndexFunc(leaf, unicode.IsUpper) >= 0 {
		var b strings.Builder
		for i, r := range leaf {
			if unicode.IsUpper(r) && i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		}
		return fmt.Sprintf("unknown option %q: names are snake_case now, did you mean %q?", key, b.String())
	}
	return fmt.Sprintf("unknown option %q (ignored)", key)
}

// overlayKeys applies the user's key bindings over the defaults. An empty
// list is ignored (it would leave the action unreachable), and an action name
// the program does not have is reported.
func overlayKeys(user map[string][]string) (map[string][]string, []string) {
	out := DefaultKeyBindings()
	var warn []string
	for action, keys := range user {
		if _, known := out[action]; !known {
			warn = append(warn, unknownOption("keys."+action))
			continue
		}
		var clean []string
		for _, k := range keys {
			if k = strings.TrimSpace(k); k != "" {
				clean = append(clean, k)
			}
		}
		if len(clean) > 0 {
			out[action] = clean
		}
	}
	return out, warn
}

func resolve(cfg Config) Config {
	warn := func(format string, a ...any) { cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(format, a...)) }
	for k := range cfg.UI.TabNames {
		if !slices.Contains(TabKeys, strings.ToLower(strings.TrimSpace(k))) {
			warn("ui.tab_names: unknown tab %q (use %s)", k, strings.Join(TabKeys, ", "))
		}
	}
	cfg.UI = cfg.UI.resolve()
	cfg.LogFilePath = ResolvePath(cfg.LogFilePath, cfg.Dir)
	if cfg.MaxHistory < 0 {
		warn("max_history is negative, using 0 (unlimited)")
		cfg.MaxHistory = 0
	}
	if cfg.MaxEntryLength < 10 {
		warn("max_entry_length %d is too small, using 65", cfg.MaxEntryLength)
		cfg.MaxEntryLength = 65
	}
	if t := strings.ToLower(cfg.DefaultTab); slices.Contains(TabKeys, t) {
		cfg.DefaultTab = t
	} else {
		warn("default_tab %q is not one of %s, using clipboard", cfg.DefaultTab, strings.Join(TabKeys, ", "))
		cfg.DefaultTab = "clipboard"
	}
	if p := strings.ToLower(cfg.ImagePreview); slices.Contains([]string{"auto", "sixel", "basic", "off"}, p) {
		cfg.ImagePreview = p
	} else {
		warn("image_preview %q is not auto, sixel, basic or off, using auto", cfg.ImagePreview)
		cfg.ImagePreview = "auto"
	}
	cfg.EmojiColumns = max(cfg.EmojiColumns, 0)
	cfg.EmojiRows = max(cfg.EmojiRows, 0)
	for i, t := range cfg.SearchOnOpen {
		t = strings.ToLower(strings.TrimSpace(t))
		if !slices.Contains(TabKeys, t) {
			warn("search_on_open: unknown tab %q (use %s)", t, strings.Join(TabKeys, ", "))
		}
		cfg.SearchOnOpen[i] = t
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

// ListenerLockFile is the lock file held by the running listener.
func (c Config) ListenerLockFile() string { return filepath.Join(c.Dir, ListenerLockName) }

// TUILockFile is the lock file held by the running TUI.
func (c Config) TUILockFile() string { return filepath.Join(c.Dir, TUILockName) }

// TUIPIDFile is the path of the TUI's pid file.
func (c Config) TUIPIDFile() string { return filepath.Join(c.Dir, TUIPIDFileName) }

// PauseFile is the path of the pause marker file.
func (c Config) PauseFile() string { return filepath.Join(c.Dir, PauseFileName) }

// Keys returns the keys bound to action.
func (c Config) Keys(action string) []string {
	keys, ok := c.KeyBindings[action]
	if !ok {
		keys = DefaultKeyBindings()[action]
	}
	return slices.Clone(keys)
}

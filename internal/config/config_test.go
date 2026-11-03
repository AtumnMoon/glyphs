package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// write puts body in a fresh config dir and loads it.
func load(t *testing.T, body string) (Config, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return LoadFrom(dir)
}

func mustLoad(t *testing.T, body string) Config {
	t.Helper()
	cfg, err := load(t, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return cfg
}

func TestLoadCreatesTemplate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "glyphs")
	cfg, err := LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("%s not written: %v", FileName, err)
	}
	if string(raw) != defaultTemplate {
		t.Error("the file written on first run is not the embedded template")
	}
	if cfg.MaxHistory != 100 || !cfg.QuitAfterCopy || len(cfg.Warnings) != 0 {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if got := cfg.Keys("quit"); !reflect.DeepEqual(got, []string{"q", "esc"}) {
		t.Errorf("quit keys = %v", got)
	}
}

// normalised is Default() the way parse() leaves it: resolved, with Dir set.
func normalised(dir string) Config {
	d := Default()
	d.Dir = dir
	return resolve(d)
}

func TestTemplateParsesToDefaults(t *testing.T) {
	dir := t.TempDir()
	got, err := parse(defaultTemplate, filepath.Join(dir, FileName), dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := normalised(dir); !reflect.DeepEqual(got, want) {
		t.Errorf("template does not parse to the defaults:\n got %+v\nwant %+v", got, want)
	}
}

// Every commented-out "# option = value" line in the template documents a
// default. Uncommenting all of them must change nothing; this fails when the
// template and the code drift apart.
func TestTemplateCommentedDefaultsMatchCode(t *testing.T) {
	re := regexp.MustCompile(`(?m)^# ([a-z_]+ = )`)
	body := re.ReplaceAllString(defaultTemplate, "$1")
	if body == defaultTemplate {
		t.Fatal("found no commented defaults in the template")
	}
	dir := t.TempDir()
	got, err := parse(body, filepath.Join(dir, FileName), dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := normalised(dir); !reflect.DeepEqual(got, want) {
		t.Errorf("a commented default in the template differs from the code:\n got %+v\nwant %+v", got, want)
	}
}

// Every option the program has must be documented in the template.
func TestTemplateDocumentsEveryOption(t *testing.T) {
	var names []string
	collect := func(v any) {
		rt := reflect.TypeOf(v)
		for i := range rt.NumField() {
			if tag := strings.Split(rt.Field(i).Tag.Get("toml"), ",")[0]; tag != "" && tag != "-" {
				names = append(names, tag)
			}
		}
	}
	collect(Theme{})
	collect(UI{})
	names = slices.DeleteFunc(names, func(n string) bool { return n == "tab_names" }) // a [table], checked below
	for action := range DefaultKeyBindings() {
		names = append(names, action)
	}
	for _, n := range names {
		if !regexp.MustCompile(`(?m)^#? ?` + regexp.QuoteMeta(n) + ` = `).MatchString(defaultTemplate) {
			t.Errorf("option %q is missing from default_config.toml", n)
		}
	}
	if !strings.Contains(defaultTemplate, "[ui.tab_names]") {
		t.Error("the [ui.tab_names] table is missing from default_config.toml")
	}
	rt := reflect.TypeOf(Config{})
	for i := range rt.NumField() {
		tag := strings.Split(rt.Field(i).Tag.Get("toml"), ",")[0]
		if tag == "" || tag == "-" || tag == "keys" || tag == "theme" || tag == "ui" {
			continue
		}
		if !strings.Contains(defaultTemplate, "\n"+tag+" = ") {
			t.Errorf("top-level option %q is missing from default_config.toml", tag)
		}
	}
}

func TestPartialConfigKeepsDefaults(t *testing.T) {
	cfg := mustLoad(t, `
max_history = 5
default_tab = "Emoji"

[keys]
quit = ["x"]

[theme]
accent = "#123456"
`)
	if cfg.MaxHistory != 5 {
		t.Errorf("max_history = %d", cfg.MaxHistory)
	}
	if got := cfg.Keys("quit"); !reflect.DeepEqual(got, []string{"x"}) {
		t.Errorf("quit override lost: %v", got)
	}
	if got := cfg.Keys("choose"); !reflect.DeepEqual(got, []string{"enter"}) {
		t.Errorf("unspecified binding should keep default, got %v", got)
	}
	if cfg.Theme.Accent != "#123456" || cfg.Theme.Pin != DefaultTheme().Pin {
		t.Errorf("theme merge wrong: %+v", cfg.Theme)
	}
	if cfg.DefaultTab != "emoji" {
		t.Errorf("default_tab = %q", cfg.DefaultTab)
	}
	if !cfg.EnableMouse || !cfg.QuitAfterCopy {
		t.Errorf("options that were left out must keep their defaults: %+v", cfg)
	}
}

// The mistake that started all this: a JSON-style trailing comma. In TOML it
// is simply valid inside an array.
func TestTrailingCommaInArrayIsValid(t *testing.T) {
	cfg := mustLoad(t, "search_on_open = [\n    \"emoji\",\n]\n")
	if !reflect.DeepEqual(cfg.SearchOnOpen, []string{"emoji"}) {
		t.Errorf("search_on_open = %v", cfg.SearchOnOpen)
	}
}

func TestSyntaxErrorReportsLineAndColumn(t *testing.T) {
	cfg, err := load(t, "max_history = 5\nsearch_on_open = [\"emoji\"\nquit_after_copy = true\n")
	var ce *Error
	if !errors.As(err, &ce) {
		t.Fatalf("want *Error, got %T %v", err, err)
	}
	if ce.Line != 3 || ce.Col == 0 {
		t.Errorf("position = %d:%d, want line 3", ce.Line, ce.Col)
	}
	if !strings.Contains(ce.Error(), FileName+":3:") || ce.Context == "" {
		t.Errorf("message should locate the problem: %q (context %q)", ce.Error(), ce.Context)
	}
	if cfg.MaxHistory != 100 || cfg.Dir == "" {
		t.Errorf("an unreadable file must give the defaults, got %+v", cfg)
	}
}

func TestWrongTypeIsAnError(t *testing.T) {
	cfg, err := load(t, "max_history = \"many\"\n")
	var ce *Error
	if !errors.As(err, &ce) {
		t.Fatalf("want *Error, got %T %v", err, err)
	}
	if ce.Line != 1 || !strings.Contains(ce.Msg, `"max_history"`) || !strings.Contains(ce.Error(), FileName+":1:") || !strings.Contains(ce.Context, "max_history") {
		t.Errorf("should name the line and the option: %q (context %q)", ce.Error(), ce.Context)
	}
	if cfg.MaxHistory != 100 {
		t.Errorf("should fall back to defaults, got %+v", cfg)
	}
}

func TestUnknownOptionsAreWarnings(t *testing.T) {
	cfg, err := load(t, `
allowDuplicates = true
max_histroy = 3
quit_after_copy = false

[keys]
frobnicate = ["f"]
quit = ["x"]

[theme]
acent = "#000000"
`)
	if err != nil {
		t.Fatalf("unknown options must not be fatal: %v", err)
	}
	if cfg.QuitAfterCopy || cfg.Keys("quit")[0] != "x" {
		t.Error("known options next to unknown ones must still apply")
	}
	all := strings.Join(cfg.Warnings, "\n")
	for _, want := range []string{`"allowDuplicates"`, `"allow_duplicates"`, `"max_histroy"`, `keys.frobnicate`, `"theme.acent"`} {
		if !strings.Contains(all, want) {
			t.Errorf("warnings are missing %s:\n%s", want, all)
		}
	}
}

func TestOutOfRangeValuesAreCorrectedWithWarnings(t *testing.T) {
	cfg := mustLoad(t, "default_tab = \"nope\"\nimage_preview = \"huge\"\nmax_entry_length = 2\nsearch_on_open = [\"emoji\", \"bogus\"]\n")
	if cfg.DefaultTab != "clipboard" || cfg.ImagePreview != "auto" || cfg.MaxEntryLength != 65 {
		t.Errorf("values were not corrected: %+v", cfg)
	}
	if len(cfg.Warnings) != 4 {
		t.Errorf("want 4 warnings, got %q", cfg.Warnings)
	}
}

func TestKeyBindingsAreArrays(t *testing.T) {
	cfg := mustLoad(t, "[keys]\nquit = [\"q\", \"esc\", \",\"]\nup = []\nremove = [\" \", \"d\"]\n")
	if got := cfg.Keys("quit"); !reflect.DeepEqual(got, []string{"q", "esc", ","}) {
		t.Errorf("a literal comma must be bindable, got %v", got)
	}
	if got := cfg.Keys("up"); !reflect.DeepEqual(got, []string{"up", "k"}) {
		t.Errorf("an empty list must keep the default, got %v", got)
	}
	if got := cfg.Keys("remove"); !reflect.DeepEqual(got, []string{"d"}) {
		t.Errorf("blank keys are dropped, got %v", got)
	}
	got := cfg.Keys("quit")
	got[0] = "mutated"
	if cfg.Keys("quit")[0] != "q" {
		t.Error("Keys must return a copy")
	}
}

func TestOldJSONConfigIsNoticedOnFirstRun(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], "config.json") {
		t.Errorf("want one warning about the old file, got %q", cfg.Warnings)
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); err != nil {
		t.Errorf("a new %s should be written: %v", FileName, err)
	}
	if cfg2, _ := LoadFrom(dir); len(cfg2.Warnings) != 0 {
		t.Errorf("the notice is for the first run only, got %q", cfg2.Warnings)
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
	cfg := mustLoad(t, "[theme]\nselected_desc = \"#abcdef\"\nplaceholder = \" #123456 \"\nhelp_key = \"#fff\"\naccent = \"\"\n")
	r := cfg.Theme.Resolve()
	if r.SelectedDesc != "#abcdef" || r.Placeholder != "#123456" || r.HelpKey != "#fff" {
		t.Errorf("overrides lost: %+v", r)
	}
	if r.SelectedPin != DefaultTheme().Selected {
		t.Errorf("untouched key should still inherit, got %q", r.SelectedPin)
	}
	if r.Accent != DefaultTheme().Accent {
		t.Errorf("a blank required colour must keep its default, got %q", r.Accent)
	}
}

func TestUIDefaultsAndPartialOverride(t *testing.T) {
	cfg := mustLoad(t, `
[ui]
pin_marker = ""
select_marker = "*"
search_placeholder = "find…"
bold_selected = false

[ui.tab_names]
Emoji = "Faces"
bogus = "x"
kaomoji = "  "
`)
	u, d := cfg.UI, DefaultUI()
	if u.PinMarker != "" {
		t.Errorf("an explicitly empty pin_marker must hide the marker, got %q", u.PinMarker)
	}
	if u.SelectMarker != "*" || u.SearchPlaceholder != "find…" || u.BoldSelected {
		t.Errorf("overrides lost: %+v", u)
	}
	if u.SearchIdle != d.SearchIdle || u.Divider != d.Divider || !u.BoldActiveTab {
		t.Errorf("unspecified keys should keep defaults: %+v", u)
	}
	if u.TabNames["emoji"] != "Faces" || u.TabNames["kaomoji"] != "Kaomoji" || u.TabNames["clipboard"] != "Clipboard" {
		t.Errorf("tab_names merge wrong: %v", u.TabNames)
	}
	if _, ok := u.TabNames["bogus"]; ok {
		t.Error("unknown tab names should be dropped")
	}
	if !strings.Contains(strings.Join(cfg.Warnings, "\n"), `"bogus"`) {
		t.Errorf("the dropped tab name should be reported: %q", cfg.Warnings)
	}
}

func TestUIStringsAreSingleLine(t *testing.T) {
	u := UI{TabFormat: "  ", HistoryTitle: "", SearchIdle: "a\nb"}.resolve()
	if u.TabFormat != DefaultUI().TabFormat || u.HistoryTitle != "History" {
		t.Errorf("blank tab_format/history_title should fall back: %+v", u)
	}
	if u.SearchIdle != "a b" {
		t.Errorf("newlines should be flattened, got %q", u.SearchIdle)
	}
}

// A user's "Emoji" must beat the default "emoji" every time, not just when
// Go's random map order happens to visit it last.
func TestTabNamesOverrideIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	body := "[ui.tab_names]\nEmoji = \"Faces\"\nclipboard = \"Paste\"\n"
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	for range 300 {
		cfg, err := LoadFrom(dir)
		if err != nil {
			t.Fatal(err)
		}
		if n := cfg.UI.TabNames; n["emoji"] != "Faces" || n["clipboard"] != "Paste" || n["kaomoji"] != "Kaomoji" {
			t.Fatalf("tab_names = %v", n)
		}
	}
}

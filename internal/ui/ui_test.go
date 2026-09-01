package ui

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/AtumnMoon/glyphs/internal/clipboard"
	"github.com/AtumnMoon/glyphs/internal/config"
	"github.com/AtumnMoon/glyphs/internal/data"
)

// ---- fakes & helpers -------------------------------------------------------

type fakeSys struct {
	mu     sync.Mutex
	text   []string
	images [][]byte
}

func (f *fakeSys) texts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.text...)
}

func (f *fakeSys) imgs() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]byte(nil), f.images...)
}

func (f *fakeSys) WriteText(s string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.text = append(f.text, s)
	return nil
}

func (f *fakeSys) WriteImage(b []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.images = append(f.images, b)
	return nil
}

type harness struct {
	t     *testing.T
	m     tea.Model
	sys   *fakeSys
	hist  *clipboard.HistoryStore
	quit  bool
	lastC tea.Cmd
}

func newHarness(t *testing.T, startTab string, mut func(*config.Config)) *harness {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Dir = dir
	if mut != nil {
		mut(&cfg)
	}
	hist, err := clipboard.NewHistory(dir, 100)
	if err != nil {
		t.Fatal(err)
	}
	kao, _ := data.NewKaomojiStore("")
	emo, _ := data.NewEmojiStore("")
	sys := &fakeSys{}
	h := &harness{t: t, sys: sys, hist: hist}
	histFile := filepath.Join(dir, "clipboard_history.json")
	h.m = New(Deps{
		Config: cfg, Kaomoji: kao, Emoji: emo, History: hist,
		WriteText: sys.WriteText, WriteImage: sys.WriteImage,
		ModTime: func() time.Time {
			fi, err := os.Stat(histFile)
			if err != nil {
				return time.Time{}
			}
			return fi.ModTime()
		},
	}, startTab)
	h.send(tea.WindowSizeMsg{Width: 110, Height: 32})
	return h
}

func keyMsg(name string) tea.KeyPressMsg {
	switch name {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "ctrl+p":
		return tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}
	case "shift+down":
		return tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift}
	}
	r := []rune(name)
	if len(r) != 1 {
		panic("unknown key " + name)
	}
	return tea.KeyPressMsg{Code: r[0], Text: name}
}

func (h *harness) send(msg tea.Msg) tea.Cmd {
	m, cmd := h.m.Update(msg)
	h.m = m
	if h.m.(*model).quitting {
		h.quit = true
	}
	return cmd
}

// press sends keys; any returned command that yields a copiedMsg is executed
// and fed back, mirroring what the Bubble Tea runtime does.
func (h *harness) press(keys ...string) {
	h.t.Helper()
	for _, k := range keys {
		cmd := h.send(keyMsg(k))
		h.settle(cmd)
	}
}

func (h *harness) typeText(s string) {
	for _, r := range s {
		h.press(string(r))
	}
}

func (h *harness) settle(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	// Copy commands return immediately; blink/tick commands sleep. Give each
	// command a short window and feed back only copiedMsg results.
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		if cm, ok := msg.(copiedMsg); ok {
			h.send(cm)
		}
	case <-time.After(40 * time.Millisecond):
	}
}

func (h *harness) view() string { return ansi.Strip(h.m.View().Content) }

func (h *harness) model() *model { return h.m.(*model) }

// ---- tests -----------------------------------------------------------------

func TestInitialKaomojiView(t *testing.T) {
	h := newHarness(t, "", nil)
	v := h.view()
	for _, want := range []string{"Kaomoji", "Emoji", "Clipboard", "Categories", "Joy", "(*^ω^)"[:1]} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q:\n%s", want, v)
		}
	}
}

func TestPickKaomojiCopiesAndQuits(t *testing.T) {
	h := newHarness(t, "kaomoji", nil)
	h.press("right") // focus the grid
	if !h.model().kao.focusItems {
		t.Fatal("right should focus the grid")
	}
	want := h.model().kao.shown[0].text
	h.press("enter")
	if len(h.sys.texts()) != 1 || h.sys.texts()[0] != want {
		t.Errorf("clipboard got %q, want %q", h.sys.texts(), want)
	}
	if !h.quit {
		t.Error("should quit after copy by default")
	}
}

func TestStayOpenAfterCopyWhenConfigured(t *testing.T) {
	h := newHarness(t, "kaomoji", func(c *config.Config) { c.QuitAfterCopy = false })
	h.press("right", "enter")
	if h.quit {
		t.Fatal("quitAfterCopy=false must keep the UI open")
	}
	if !strings.Contains(h.view(), "Copied") {
		t.Errorf("expected a Copied status:\n%s", h.view())
	}
}

func TestCategoryNavigationChangesGrid(t *testing.T) {
	h := newHarness(t, "kaomoji", nil)
	all := len(h.model().kao.shown)
	h.press("down") // "All" -> first category
	got := len(h.model().kao.shown)
	if got == 0 || got >= all {
		t.Errorf("category should narrow the grid: all=%d now=%d", all, got)
	}
	cat := h.model().kao.names[1]
	if !strings.Contains(h.view(), cat) {
		t.Errorf("category %q not shown", cat)
	}
}

func TestGridLeftReturnsToCategories(t *testing.T) {
	h := newHarness(t, "kaomoji", nil)
	h.press("right", "left")
	if h.model().kao.focusItems {
		t.Error("left at column 0 should return focus to categories")
	}
}

func TestFilterKaomoji(t *testing.T) {
	h := newHarness(t, "kaomoji", nil)
	h.press("/")
	if !h.model().filtering {
		t.Fatal("/ should start filtering")
	}
	h.typeText("cat")
	b := h.model().kao
	if len(b.shown) == 0 {
		t.Fatal("no results for 'cat'")
	}
	for _, it := range b.shown {
		if !strings.Contains(it.hay, "cat") {
			t.Errorf("result %q does not match", it.text)
		}
	}
	h.press("enter") // accept the filter
	if h.model().filtering || h.model().query != "cat" {
		t.Errorf("enter should keep the filter applied: filtering=%v query=%q", h.model().filtering, h.model().query)
	}
	h.press("esc") // first esc clears
	if h.quit || h.model().query != "" {
		t.Errorf("esc should clear the filter, not quit (quit=%v query=%q)", h.quit, h.model().query)
	}
	h.press("esc")
	if !h.quit {
		t.Error("second esc should quit")
	}
}

func TestEmojiSkinTone(t *testing.T) {
	h := newHarness(t, "emoji", nil)
	h.press("/")
	h.typeText("waving hand")
	h.press("enter")
	b := h.model().emo
	if len(b.shown) == 0 {
		t.Fatal("no emoji for 'waving hand'")
	}
	base := b.shown[0].glyph(0)
	h.press("t", "t", "t") // medium
	if got := b.shown[0].glyph(h.model().skin); got == base || !strings.Contains(h.view(), "Skin tone: medium") {
		t.Errorf("skin tone not applied: base=%q now=%q\n%s", base, got, h.view())
	}
	h.press("right", "enter")
	if len(h.sys.texts()) != 1 || h.sys.texts()[0] == base {
		t.Errorf("should copy the toned variant, got %q (base %q)", h.sys.texts(), base)
	}
}

func TestEmojiHidesComponentGroup(t *testing.T) {
	h := newHarness(t, "emoji", nil)
	for _, n := range h.model().emo.names {
		if n == "Component" {
			t.Error("Component group should be hidden")
		}
	}
}

func TestTabSwitching(t *testing.T) {
	h := newHarness(t, "kaomoji", nil)
	h.press("tab")
	if h.model().tab != tabEmoji {
		t.Fatalf("tab = %d", h.model().tab)
	}
	h.press("tab", "tab")
	if h.model().tab != tabKaomoji {
		t.Errorf("tab should wrap, got %d", h.model().tab)
	}
	h.press("shift+tab")
	if h.model().tab != tabClipboard {
		t.Errorf("shift+tab should go back, got %d", h.model().tab)
	}
}

func seed(t *testing.T, h *harness, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if _, err := h.hist.AddText(s); err != nil {
			t.Fatal(err)
		}
	}
	h.model().reloadClip(true)
}

func TestClipboardListAndCopy(t *testing.T) {
	h := newHarness(t, "clipboard", nil)
	seed(t, h, "first copy", "second copy")
	h.send(tea.WindowSizeMsg{Width: 110, Height: 32})
	v := h.view()
	if !strings.Contains(v, "second copy") || !strings.Contains(v, "first copy") {
		t.Fatalf("entries not listed:\n%s", v)
	}
	h.press("down", "enter")
	if len(h.sys.texts()) != 1 || h.sys.texts()[0] != "first copy" {
		t.Errorf("copied %q", h.sys.texts())
	}
}

func TestClipboardPinDeleteConfirm(t *testing.T) {
	h := newHarness(t, "clipboard", nil)
	seed(t, h, "keep me", "drop me")

	h.press("down", "p") // pin "keep me" (second row)
	all, _ := h.hist.All()
	if !all[1].Pinned || all[1].Value != "keep me" {
		t.Fatalf("pin failed: %+v", all)
	}
	if !strings.Contains(h.view(), "★") {
		t.Error("pin marker not shown")
	}

	h.press("backspace") // deleting a pinned entry must ask first
	if h.model().confirm == nil || !strings.Contains(h.view(), "pinned") {
		t.Fatal("expected a confirmation for a pinned entry")
	}
	h.press("n")
	if all, _ := h.hist.All(); len(all) != 2 {
		t.Error("cancelling must not delete")
	}
	h.press("backspace", "y")
	if got, _ := h.hist.All(); len(got) != 1 || got[0].Value != "drop me" {
		t.Errorf("after confirmed delete: %+v", got)
	}

	h.press("backspace") // unpinned: deleted immediately
	if got, _ := h.hist.All(); len(got) != 0 {
		t.Errorf("unpinned delete failed: %+v", got)
	}
}

func TestClipboardMultiSelectJoins(t *testing.T) {
	h := newHarness(t, "clipboard", nil)
	seed(t, h, "a", "b", "c") // list order: c, b, a
	h.press("shift+down", "shift+down", "enter")
	if len(h.sys.texts()) != 1 || h.sys.texts()[0] != "c\nb" {
		t.Errorf("multi copy = %q", h.sys.texts())
	}
}

func TestClipboardFilterAndPinnedOnly(t *testing.T) {
	h := newHarness(t, "clipboard", nil)
	seed(t, h, "alpha one", "beta two", "alphabet soup")
	h.press("/")
	h.typeText("alp")
	c := h.model().clip
	if len(c.view) != 2 {
		t.Fatalf("fuzzy 'alp' should match 2, got %d", len(c.view))
	}
	h.press("esc", "p") // clear filter, pin current
	h.press("ctrl+p")
	if !h.model().clip.pinnedOnly || len(h.model().clip.view) != 1 {
		t.Errorf("pinned-only view wrong: %v / %d", h.model().clip.pinnedOnly, len(h.model().clip.view))
	}
}

func TestClipboardRealtimeReload(t *testing.T) {
	h := newHarness(t, "clipboard", nil)
	if len(h.model().clip.entries) != 0 {
		t.Fatal("expected empty history")
	}
	h.hist.AddText("added by the listener")
	h.send(tickMsg{})
	if len(h.model().clip.entries) != 1 || !strings.Contains(h.view(), "added by the listener") {
		t.Errorf("tick should pick up external changes:\n%s", h.view())
	}
}

func TestClipboardImagePreviewAndCopy(t *testing.T) {
	h := newHarness(t, "clipboard", nil)
	png := onePixelPNG(t)
	if _, err := h.hist.AddImage(png); err != nil {
		t.Fatal(err)
	}
	h.model().reloadClip(true)
	h.send(tea.WindowSizeMsg{Width: 110, Height: 32})
	v := h.view()
	if !strings.Contains(v, "image") || !strings.Contains(v, "1×1") {
		t.Errorf("image row/preview missing:\n%s", v)
	}
	h.press("enter")
	if len(h.sys.imgs()) != 1 {
		t.Errorf("image not copied: %d", len(h.sys.imgs()))
	}
}

func TestPreviewToggleAndNarrowLayout(t *testing.T) {
	h := newHarness(t, "clipboard", nil)
	seed(t, h, "hello world")
	if !strings.Contains(h.view(), "Preview") {
		t.Error("wide layout should show the preview pane")
	}
	h.press("space")
	if strings.Contains(h.view(), "Preview") {
		t.Error("space should hide the preview")
	}
	h.press("space")
	h.send(tea.WindowSizeMsg{Width: 60, Height: 20})
	v := h.view()
	if !strings.Contains(v, "Preview") || strings.Contains(v, "History —") {
		t.Errorf("narrow layout should show preview instead of the list:\n%s", v)
	}
}

func TestViewNeverOverflows(t *testing.T) {
	sizes := [][2]int{{40, 12}, {60, 20}, {80, 24}, {110, 32}, {200, 50}}
	for _, tabName := range []string{"kaomoji", "emoji", "clipboard"} {
		for _, sz := range sizes {
			h := newHarness(t, tabName, nil)
			seed(t, h, "short", strings.Repeat("a long clipboard line ", 20), "multi\nline\nentry")
			h.send(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
			for _, extra := range []string{"", "?"} {
				if extra != "" {
					h.press(extra)
				}
				lines := strings.Split(h.view(), "\n")
				if len(lines) > sz[1] {
					t.Errorf("%s %dx%d help=%q: %d lines exceed height", tabName, sz[0], sz[1], extra, len(lines))
				}
				for i, l := range lines {
					if w := ansi.StringWidth(l); w > sz[0] {
						t.Errorf("%s %dx%d line %d is %d wide: %q", tabName, sz[0], sz[1], i, w, l)
						break
					}
				}
			}
		}
	}
}

func TestRebindableKeys(t *testing.T) {
	h := newHarness(t, "kaomoji", func(c *config.Config) {
		c.KeyBindings["quit"] = "x"
		c.KeyBindings["nextTab"] = "n"
	})
	h.press("n")
	if h.model().tab != tabEmoji {
		t.Error("custom nextTab binding ignored")
	}
	h.press("q")
	if h.quit {
		t.Error("default quit key should be unbound after override")
	}
	h.press("x")
	if !h.quit {
		t.Error("custom quit binding ignored")
	}
}

func TestForceQuitWorksWhileFiltering(t *testing.T) {
	h := newHarness(t, "kaomoji", nil)
	h.press("/")
	h.send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !h.quit {
		t.Error("ctrl+c must quit even in filter mode")
	}
}

func onePixelPNG(t *testing.T) []byte {
	t.Helper()
	// a valid 1x1 red PNG
	b := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xdd, 0x8d, 0xb0, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
	return b
}

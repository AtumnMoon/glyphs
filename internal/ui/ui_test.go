package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
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
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
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
	// Commands the tests trigger (copy, quit) return immediately. The window
	// is generous so slow CI machines don't make this flaky; a timer command
	// would simply be abandoned after it.
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		if cm, ok := msg.(copiedMsg); ok {
			h.send(cm)
		}
	case <-time.After(3 * time.Second):
	}
}

func (h *harness) view() string { return ansi.Strip(h.m.View().Content) }

func (h *harness) model() *model { return h.m.(*model) }

// ---- tests -----------------------------------------------------------------

func TestInitialKaomojiView(t *testing.T) {
	h := newHarness(t, "kaomoji", nil)
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

func TestSearchKaomoji(t *testing.T) {
	h := newHarness(t, "kaomoji", nil)
	if h.model().typing {
		t.Fatal("kaomoji search should not grab the keyboard by default")
	}
	h.press("/")
	if !h.model().typing {
		t.Fatal("/ should start typing in the search bar")
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
	h.press("esc") // stop typing, keep the query
	if h.model().typing || h.model().query != "cat" {
		t.Errorf("esc should leave typing mode but keep the query: typing=%v query=%q", h.model().typing, h.model().query)
	}
	h.press("esc") // normal mode: clears the query
	if h.quit || h.model().query != "" {
		t.Errorf("esc should clear the query, not quit (quit=%v query=%q)", h.quit, h.model().query)
	}
	h.press("esc")
	if !h.quit {
		t.Error("another esc should quit")
	}
}

func TestSearchBarIsAlwaysVisible(t *testing.T) {
	for _, tab := range []string{"clipboard", "kaomoji", "emoji"} {
		h := newHarness(t, tab, nil)
		if !strings.Contains(h.view(), "search") {
			t.Errorf("%s: no search bar in view:\n%s", tab, h.view())
		}
	}
}
func TestEmojiStartsInSearchMode(t *testing.T) {
	h := newHarness(t, "emoji", nil)
	if !h.model().typing {
		t.Fatal("emoji search should be focused on open")
	}
	h.typeText("happy")
	if h.model().query != "happy" {
		t.Errorf("query = %q", h.model().query)
	}
	if !strings.Contains(h.view(), "type to search") && !strings.Contains(h.view(), "happy") {
		t.Error("typed text not shown")
	}
	// q is a command only outside typing mode
	h.typeText("q")
	if h.quit {
		t.Error("q must be typed into the search while typing")
	}
}

func TestEmojiSearchNarrowsGroupsAndCopies(t *testing.T) {
	h := newHarness(t, "emoji", nil)
	b := h.model().emo
	allGroups := len(b.vis)
	h.typeText("grinning")
	if len(b.shown) == 0 {
		t.Fatal("no results")
	}
	if len(b.vis) >= allGroups {
		t.Errorf("group pane should only list groups with matches: %d of %d", len(b.vis), allGroups)
	}
	if !strings.Contains(h.view(), "“grinning”") {
		t.Errorf("results header missing:\n%s", h.view())
	}
	want := b.shown[0].text
	h.press("enter") // Enter copies straight from the search box
	if got := h.sys.texts(); len(got) != 1 || got[0] != want {
		t.Errorf("copied %q, want %q", got, want)
	}
}

func TestEmojiSkinTone(t *testing.T) {
	h := newHarness(t, "emoji", nil)
	h.typeText("waving hand")
	h.press("esc") // leave typing mode so "t" is a command
	b := h.model().emo
	if len(b.shown) == 0 {
		t.Fatal("no emoji for 'waving hand'")
	}
	base := b.shown[0].glyph(0)
	h.press("t", "t", "t") // medium
	if got := b.shown[0].glyph(h.model().skin); got == base || !strings.Contains(h.view(), "Skin tone: medium") {
		t.Errorf("skin tone not applied: base=%q now=%q\n%s", base, got, h.view())
	}
	h.press("enter")
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

func TestTabOrderAndDefault(t *testing.T) {
	if tabNames[0] != "Clipboard" || tabNames[1] != "Kaomoji" || tabNames[2] != "Emoji" {
		t.Fatalf("tab order = %v", tabNames)
	}
	h := newHarness(t, "", nil) // config default
	if h.model().tab != tabClipboard {
		t.Errorf("default tab = %d, want clipboard", h.model().tab)
	}
	v := h.view()
	i, j, k := strings.Index(v, "1 Clipboard"), strings.Index(v, "2 Kaomoji"), strings.Index(v, "3 Emoji")
	if i < 0 || j < i || k < j {
		t.Errorf("tab bar order wrong:\n%s", v)
	}
}

func TestTabSwitching(t *testing.T) {
	h := newHarness(t, "clipboard", nil)
	h.press("tab")
	if h.model().tab != tabKaomoji {
		t.Fatalf("tab = %d", h.model().tab)
	}
	h.press("tab")
	if h.model().tab != tabEmoji || !h.model().typing {
		t.Fatalf("emoji should open with the search focused: tab=%d typing=%v", h.model().tab, h.model().typing)
	}
	h.press("tab")
	if h.model().tab != tabClipboard {
		t.Errorf("tab should wrap, got %d", h.model().tab)
	}
	h.press("shift+tab")
	if h.model().tab != tabEmoji {
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
	h.press("esc", "esc", "p") // leave typing, clear the query, pin current
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

func TestClipboardImageRowAndCopy(t *testing.T) {
	h := newHarness(t, "clipboard", nil)
	if _, err := h.hist.AddImage(onePixelPNG(t)); err != nil {
		t.Fatal(err)
	}
	h.model().reloadClip(true)
	h.send(tea.WindowSizeMsg{Width: 110, Height: 32})
	v := h.view()
	if !strings.Contains(v, "image") || !strings.Contains(v, "1×1") {
		t.Errorf("image row missing:\n%s", v)
	}
	h.press("enter")
	if len(h.sys.imgs()) != 1 {
		t.Errorf("image not copied: %d", len(h.sys.imgs()))
	}
}

// runCmd executes a command (and any batch it expands to) and returns the
// messages it produced. Only use with commands that return promptly.
func runCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, runCmd(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func imageHarness(t *testing.T, mut func(*config.Config)) *harness {
	h := newHarness(t, "clipboard", mut)
	if _, err := h.hist.AddImage(onePixelPNG(t)); err != nil {
		t.Fatal(err)
	}
	h.model().reloadClip(true)
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	return h
}

func TestImagePreviewBasicWithoutSixel(t *testing.T) {
	h := imageHarness(t, nil) // terminal never reported sixel support
	h.press("space")
	v := h.view()
	if !strings.Contains(v, "Preview") || !strings.Contains(v, "▀") {
		t.Errorf("expected half-block fallback:\n%s", v)
	}
	if cmds := runCmd(h.send(tea.WindowSizeMsg{Width: 100, Height: 30})); len(cmds) != 0 {
		t.Errorf("no sixel expected without terminal support, got %v", cmds)
	}
}

func TestImagePreviewSixelWhenTerminalSupportsIt(t *testing.T) {
	h := imageHarness(t, nil)
	h.send(uv.PrimaryDeviceAttributesEvent{62, 4, 22}) // attribute 4 = sixel
	h.send(uv.CellSizeEvent{Width: 10, Height: 20})
	if !h.model().sixelOK || h.model().cellW != 10 {
		t.Fatalf("capabilities not recorded: %+v", h.model())
	}
	var msgs []tea.Msg
	for _, c := range []tea.Cmd{h.send(keyMsg("space"))} {
		msgs = runCmd(c)
	}
	if v := h.view(); strings.Contains(v, "▀") || !strings.Contains(v, "Preview") {
		t.Errorf("sixel mode should leave the area blank for the bitmap:\n%s", v)
	}
	var painted string
	for _, msg := range msgs {
		if im, ok := msg.(imageMsg); ok {
			for _, out := range runCmd(h.send(im)) {
				if raw, ok := out.(tea.RawMsg); ok {
					painted, _ = raw.Msg.(string)
				}
			}
		}
	}
	if painted == "" {
		t.Fatalf("no sixel was written; messages: %v", msgs)
	}
	for _, want := range []string{"\x1b7", "\x1bP", "\x1b8"} { // save cursor, DCS (sixel start), restore
		if !strings.Contains(painted, want) {
			t.Errorf("sixel output missing %q", want)
		}
	}
	// it is painted once, not on every keypress
	if again := runCmd(h.send(tea.WindowSizeMsg{Width: 100, Height: 30})); len(again) == 0 {
		t.Error("a resize repaints the screen, so the image must be painted again")
	}
	cmd := h.send(keyMsg("esc"))
	if h.model().imgSig != "" {
		t.Error("closing the preview should forget the painted image")
	}
	cleared := false
	for _, msg := range runCmd(cmd) {
		if reflect.TypeOf(msg) == reflect.TypeOf(tea.ClearScreen()) {
			cleared = true
		}
	}
	if !cleared {
		t.Error("closing an image preview must force a full repaint, otherwise the sixel bitmap stays on screen")
	}
}

func TestImagePreviewCanBeDisabled(t *testing.T) {
	h := imageHarness(t, func(c *config.Config) { c.ImagePreview = "off" })
	h.send(uv.PrimaryDeviceAttributesEvent{62, 4})
	h.send(uv.CellSizeEvent{Width: 10, Height: 20})
	h.press("space")
	if v := h.view(); !strings.Contains(v, "turned off") || strings.Contains(v, "▀") {
		t.Errorf("imagePreview=off not honoured:\n%s", v)
	}
}

func TestPreviewIsClosedByDefaultAndOpensWithSpace(t *testing.T) {
	h := newHarness(t, "clipboard", nil)
	long := strings.Repeat("line of text\n", 200)
	seed(t, h, long)
	if strings.Contains(h.view(), "Preview") {
		t.Fatal("preview must not be open by default")
	}
	h.press("space")
	v := h.view()
	if !strings.Contains(v, "Preview") || !strings.Contains(v, "0%") || strings.Contains(v, "History —") {
		t.Fatalf("space should open the full-pane preview at 0%%:\n%s", v)
	}
	if !strings.Contains(v, "page down") {
		t.Errorf("preview help should mention paging:\n%s", v)
	}
	h.press("down", "down", "down")
	if h.model().clip.prevOff != 3 {
		t.Errorf("down should scroll by a line, off=%d", h.model().clip.prevOff)
	}
	h.press("right")
	if h.model().clip.prevOff <= 3 {
		t.Error("right should page down")
	}
	h.press("end")
	if !strings.Contains(h.view(), "100%") {
		t.Errorf("end should reach 100%%:\n%s", h.view())
	}
	h.press("left", "left", "left", "left", "left", "left", "left", "left", "left", "left", "left", "left", "left", "left")
	h.press("home")
	if h.model().clip.prevOff != 0 {
		t.Error("home should return to the top")
	}
	h.press("space")
	if h.model().preview || !strings.Contains(h.view(), "History —") {
		t.Error("space should close the preview")
	}
}

func TestPreviewEnterCopiesAndEscCloses(t *testing.T) {
	h := newHarness(t, "clipboard", func(c *config.Config) { c.QuitAfterCopy = false })
	seed(t, h, "copy me from the preview")
	h.press("space", "enter")
	if got := h.sys.texts(); len(got) != 1 || got[0] != "copy me from the preview" {
		t.Errorf("copied %q", got)
	}
	h.press("esc")
	if h.model().preview || h.quit {
		t.Errorf("esc should close the preview without quitting (preview=%v quit=%v)", h.model().preview, h.quit)
	}
}
func TestViewNeverOverflows(t *testing.T) {
	sizes := [][2]int{{40, 12}, {60, 20}, {80, 24}, {110, 32}, {200, 50}}
	for _, tabName := range []string{"kaomoji", "emoji", "clipboard"} {
		for _, sz := range sizes {
			h := newHarness(t, tabName, nil)
			seed(t, h, "short", strings.Repeat("a long clipboard line ", 20), "multi\nline\nentry")
			h.send(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
			for _, step := range []string{"", "?", "?", "space", "space"} {
				if step != "" {
					h.press(step)
				}
				lines := strings.Split(h.view(), "\n")
				if len(lines) > sz[1] {
					t.Errorf("%s %dx%d step=%q: %d lines exceed height", tabName, sz[0], sz[1], step, len(lines))
				}
				for i, l := range lines {
					if w := ansi.StringWidth(l); w > sz[0] {
						t.Errorf("%s %dx%d step=%q line %d is %d wide: %q", tabName, sz[0], sz[1], step, i, w, l)
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
	h.press("esc") // emoji opens in search mode; hand the keyboard back
	h.press("q")
	if h.quit {
		t.Error("default quit key should be unbound after override")
	}
	h.press("x")
	if !h.quit {
		t.Error("custom quit binding ignored")
	}
}

func TestForceQuitWorksWhileTyping(t *testing.T) {
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

func TestXDeletesInClipboardTab(t *testing.T) {
	h := newHarness(t, "clipboard", nil)
	seed(t, h, "keep", "remove me")
	h.press("x") // newest entry is highlighted
	got, _ := h.hist.All()
	if len(got) != 1 || got[0].Value != "keep" {
		t.Errorf("x should delete the highlighted entry: %+v", got)
	}
	// while typing in the search box, x is just a letter
	h.press("/")
	h.press("x")
	if got, _ := h.hist.All(); len(got) != 1 {
		t.Error("x must not delete while the search box has focus")
	}
	if h.model().query != "x" {
		t.Errorf("query = %q, want x", h.model().query)
	}
}

func TestSimpleEmojiHidesJoinedSequencesAndOtherIsLast(t *testing.T) {
	full := newHarness(t, "emoji", nil).model().emo
	simple := newHarness(t, "emoji", func(c *config.Config) { c.SimpleEmoji = true }).model().emo
	joined := func(b *browser) int {
		n := 0
		for _, it := range b.groups[0] {
			if strings.ContainsRune(it.text, '\u200d') {
				n++
			}
		}
		return n
	}
	if joined(full) == 0 {
		t.Fatal("expected the full set to contain ZWJ sequences")
	}
	if joined(simple) != 0 || len(simple.groups[0]) >= len(full.groups[0]) {
		t.Errorf("simpleEmoji should drop ZWJ sequences: full=%d simple=%d joined=%d", len(full.groups[0]), len(simple.groups[0]), joined(simple))
	}
	if last := full.names[len(full.names)-1]; last != "Other" {
		t.Errorf("catch-all group should be last, got %q (names %v)", last, full.names)
	}
}

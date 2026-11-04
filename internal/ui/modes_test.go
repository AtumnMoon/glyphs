package ui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AtumnMoon/glyphs/internal/clipboard"
	"github.com/AtumnMoon/glyphs/internal/config"
)

func listed(h *harness) []string {
	c := h.model().clip
	var out []string
	for _, ei := range c.view {
		out = append(out, c.entries[ei].Value)
	}
	return out
}

func stored(t *testing.T, h *harness) []string {
	t.Helper()
	all, err := h.hist.All()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range all {
		out = append(out, e.Value)
	}
	return out
}

func wantOrder(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

// moveHarness lists d c b a (newest first), the cursor on d.
func moveHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, "clipboard", nil)
	seed(t, h, "a", "b", "c", "d")
	return h
}

// ---- moveBlock (pure) ------------------------------------------------------------

func entriesOf(vals ...string) []clipboard.ClipboardEntry {
	out := make([]clipboard.ClipboardEntry, len(vals))
	for i, v := range vals {
		out[i] = clipboard.ClipboardEntry{Value: v, Recorded: "t", FilePath: "null", Pinned: strings.HasSuffix(v, "*")}
	}
	return out
}

func valuesOf(es []clipboard.ClipboardEntry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Value
	}
	return out
}

func TestMoveBlock(t *testing.T) {
	all := func(clipboard.ClipboardEntry) bool { return true }
	pinned := func(e clipboard.ClipboardEntry) bool { return e.Pinned }
	tgt := func(es []clipboard.ClipboardEntry, names ...string) map[string]bool {
		m := map[string]bool{}
		for _, e := range es {
			if slices.Contains(names, e.Value) {
				m[e.Key()] = true
			}
		}
		return m
	}
	for _, tc := range []struct {
		name    string
		in      []string
		targets []string
		dir     int
		visible func(clipboard.ClipboardEntry) bool
		want    []string
		moved   bool
	}{
		{"up", []string{"a", "b", "c", "d"}, []string{"c"}, -1, all, []string{"a", "c", "b", "d"}, true},
		{"down", []string{"a", "b", "c", "d"}, []string{"b"}, 1, all, []string{"a", "c", "b", "d"}, true},
		{"top edge", []string{"a", "b", "c"}, []string{"a"}, -1, all, []string{"a", "b", "c"}, false},
		{"bottom edge", []string{"a", "b", "c"}, []string{"c"}, 1, all, []string{"a", "b", "c"}, false},
		{"block up", []string{"a", "b", "c", "d"}, []string{"b", "c"}, -1, all, []string{"b", "c", "a", "d"}, true},
		{"block down", []string{"a", "b", "c", "d"}, []string{"b", "c"}, 1, all, []string{"a", "d", "b", "c"}, true},
		{"block stops at the edge as one", []string{"a", "b", "c", "d"}, []string{"a", "b"}, -1, all, []string{"a", "b", "c", "d"}, false},
		{"split targets move independently", []string{"a", "b", "c", "d", "e"}, []string{"b", "d"}, -1, all, []string{"b", "a", "d", "c", "e"}, true},
		{"one target held at the edge, the other moves", []string{"a", "b", "c"}, []string{"a", "c"}, -1, all, []string{"a", "c", "b"}, true},
		{"hops hidden entries", []string{"a*", "x", "b*", "y"}, []string{"b*"}, -1, pinned, []string{"b*", "a*", "x", "y"}, true},
		{"hops hidden entries down", []string{"a*", "x", "b*", "y"}, []string{"a*"}, 1, pinned, []string{"x", "b*", "a*", "y"}, true},
		{"nothing visible beyond: stays", []string{"a*", "x", "y"}, []string{"a*"}, 1, pinned, []string{"a*", "x", "y"}, false},
		{"no targets", []string{"a", "b"}, nil, -1, all, []string{"a", "b"}, false},
	} {
		es := entriesOf(tc.in...)
		moved := moveBlock(es, tgt(es, tc.targets...), tc.dir, tc.visible)
		if moved != tc.moved || !slices.Equal(valuesOf(es), tc.want) {
			t.Errorf("%s: got %v (moved=%v), want %v (moved=%v)", tc.name, valuesOf(es), moved, tc.want, tc.moved)
		}
	}
}

// ---- move mode ---------------------------------------------------------------------

func TestMoveModeDraftsThenSaves(t *testing.T) {
	h := moveHarness(t)
	wantOrder(t, "start", listed(h), "d", "c", "b", "a")

	h.press("m")
	if h.model().mode != modeMove {
		t.Fatal("m should enter move mode")
	}
	if v := h.view(); !strings.Contains(v, "MOVE") || !strings.Contains(v, "carry") {
		t.Errorf("the mode and its keys should be shown:\n%s", v)
	}
	h.press("down")
	wantOrder(t, "draft", listed(h), "c", "d", "b", "a")
	wantOrder(t, "file while drafting", stored(t, h), "d", "c", "b", "a")
	if cur, _ := h.model().clip.current(); cur.Value != "d" {
		t.Errorf("the cursor must follow the carried entry, is on %q", cur.Value)
	}

	h.press("down")
	h.press("enter")
	if h.model().mode != modeNormal {
		t.Error("enter should leave move mode")
	}
	wantOrder(t, "file", stored(t, h), "c", "b", "d", "a")
	wantOrder(t, "view", listed(h), "c", "b", "d", "a")
	if h.model().status != "Order saved" {
		t.Errorf("status = %q", h.model().status)
	}
}

func TestMoveModeEscDiscards(t *testing.T) {
	h := moveHarness(t)
	h.press("m", "down", "down", "esc")
	if h.model().mode != modeNormal || h.quit {
		t.Fatalf("esc cancels the move, it does not quit (mode=%d quit=%v)", h.model().mode, h.quit)
	}
	wantOrder(t, "view", listed(h), "d", "c", "b", "a")
	wantOrder(t, "file", stored(t, h), "d", "c", "b", "a")
}

func TestMoveModeWithoutChangeWritesNothing(t *testing.T) {
	h := moveHarness(t)
	h.press("m", "up") // already on top
	if h.model().moved {
		t.Error("nothing moved")
	}
	h.press("enter")
	if h.model().status != "Order unchanged" || h.model().mode != modeNormal {
		t.Errorf("status=%q mode=%d", h.model().status, h.model().mode)
	}
}

func TestMoveModeCarriesTheWholeSelection(t *testing.T) {
	h := moveHarness(t)
	h.press("v", "down", "esc") // select d and c
	h.press("m", "down")
	wantOrder(t, "draft", listed(h), "b", "d", "c", "a")
	h.press("enter")
	wantOrder(t, "file", stored(t, h), "b", "d", "c", "a")
	if n := len(h.model().clip.selected); n != 2 {
		t.Errorf("the selection should survive, %d selected", n)
	}
}

func TestMoveModeHomeEnd(t *testing.T) {
	h := moveHarness(t)
	h.press("m", "end")
	wantOrder(t, "end", listed(h), "c", "b", "a", "d")
	h.press("home")
	wantOrder(t, "home", listed(h), "d", "c", "b", "a")
}

func TestMoveModeRefusesWhileFiltering(t *testing.T) {
	h := moveHarness(t)
	h.press("/")
	h.typeText("c")
	h.press("esc") // stop typing, keep the query
	h.press("m")
	if h.model().mode != modeNormal {
		t.Error("a filtered list must not be reorderable")
	}
	if !strings.Contains(h.model().status, "filter") {
		t.Errorf("explain why: status = %q", h.model().status)
	}
}

// The listener keeps recording while the user arranges the list; the draft on
// screen must not be replaced, and the new entry must survive the save.
func TestMoveModeSurvivesListenerActivity(t *testing.T) {
	h := moveHarness(t)
	h.press("m", "down")
	if _, err := h.hist.AddText("fresh"); err != nil {
		t.Fatal(err)
	}
	h.send(tickMsg(time.Now()))
	wantOrder(t, "draft after a refresh tick", listed(h), "c", "d", "b", "a")

	h.press("enter")
	wantOrder(t, "file", stored(t, h), "fresh", "c", "d", "b", "a")
	wantOrder(t, "view", listed(h), "fresh", "c", "d", "b", "a")
}

func TestMoveModeInPinnedOnlyViewHopsHiddenEntries(t *testing.T) {
	h := moveHarness(t) // d c b a
	h.press("down")
	h.press("p") // pin c
	h.press("down", "down")
	h.press("p") // pin a
	h.press("ctrl+p")
	wantOrder(t, "pinned view", listed(h), "c", "a")
	h.press("home", "m", "down")
	wantOrder(t, "draft", listed(h), "a", "c")
	h.press("enter")
	wantOrder(t, "file", stored(t, h), "d", "b", "a", "c")
}

func TestMoveModeIgnoresOtherCommands(t *testing.T) {
	h := moveHarness(t)
	h.press("m", "x", "p", "s", "q")
	if h.quit {
		t.Error("q must not quit from move mode")
	}
	wantOrder(t, "nothing deleted", stored(t, h), "d", "c", "b", "a")
	for _, e := range h.model().clip.entries {
		if e.Pinned {
			t.Error("p must not pin in move mode")
		}
	}
}

func TestLeavingTheTabDropsADraft(t *testing.T) {
	h := moveHarness(t)
	h.press("m", "down")
	h.model().switchTab(tabEmoji) // e.g. a mouse click on another tab
	h.model().switchTab(tabClipboard)
	if h.model().mode != modeNormal {
		t.Error("the mode must end with the tab")
	}
	wantOrder(t, "view", listed(h), "d", "c", "b", "a")
	wantOrder(t, "file", stored(t, h), "d", "c", "b", "a")
}

func TestModeKeysDoNothingOffTheClipboardTab(t *testing.T) {
	h := newHarness(t, "kaomoji", func(c *config.Config) { c.SearchOnOpen = nil })
	h.press("m", "v")
	if h.model().mode != modeNormal {
		t.Errorf("mode = %d", h.model().mode)
	}
}

func TestModeKeysAreTypedWhileSearching(t *testing.T) {
	h := moveHarness(t)
	h.press("/")
	h.typeText("mv")
	if h.model().query != "mv" || h.model().mode != modeNormal {
		t.Errorf("query=%q mode=%d", h.model().query, h.model().mode)
	}
}

func TestModeKeysAreRebindable(t *testing.T) {
	h := newHarness(t, "clipboard", func(c *config.Config) {
		c.KeyBindings["move_mode"] = []string{"M"}
		c.KeyBindings["mode_exit"] = []string{"ctrl+g"}
		c.KeyBindings["select_mode"] = []string{"V"}
	})
	seed(t, h, "a", "b")
	h.press("m")
	if h.model().mode != modeNormal {
		t.Error("the default key must stop working once rebound")
	}
	h.press("M")
	if h.model().mode != modeMove {
		t.Error("the rebound key should work")
	}
}

// ---- select mode -----------------------------------------------------------------------

func selectedValues(h *harness) []string {
	c := h.model().clip
	var out []string
	for _, e := range c.entries {
		if c.selected[entryKey(e)] {
			out = append(out, e.Value)
		}
	}
	return out
}

func TestSelectModeExtendsAndShrinks(t *testing.T) {
	h := moveHarness(t)
	h.press("v")
	if h.model().mode != modeSelect || !strings.Contains(h.view(), "SELECT") {
		t.Fatalf("v should start select mode and say so:\n%s", h.view())
	}
	wantOrder(t, "just the cursor row", selectedValues(h), "d")
	h.press("down", "down")
	wantOrder(t, "extended", selectedValues(h), "d", "c", "b")
	h.press("up")
	wantOrder(t, "moving back shrinks it", selectedValues(h), "d", "c")
	h.press("up", "up") // above the anchor: nothing below, the anchor row stays
	wantOrder(t, "stopped at the top", selectedValues(h), "d")

	h.press("esc")
	if h.model().mode != modeNormal || h.quit {
		t.Errorf("esc leaves the mode, it does not quit (mode=%d quit=%v)", h.model().mode, h.quit)
	}
	wantOrder(t, "selection kept", selectedValues(h), "d")
}

func TestSelectModeKeepsWhatWasSelectedBefore(t *testing.T) {
	h := moveHarness(t)
	h.press("down", "down", "down", "s") // pick "a" by itself
	h.press("home", "v", "down")
	got := selectedValues(h)
	slices.Sort(got)
	wantOrder(t, "selection", got, "a", "c", "d")
}

func TestSelectModeCanBeToggledOffWithItsKey(t *testing.T) {
	h := moveHarness(t)
	h.press("v", "down", "v")
	if h.model().mode != modeNormal || len(selectedValues(h)) != 2 {
		t.Errorf("mode=%d selected=%v", h.model().mode, selectedValues(h))
	}
}

func TestSelectModeActsOnTheSelection(t *testing.T) {
	h := moveHarness(t)
	h.press("v", "down", "x") // delete d and c
	wantOrder(t, "file", stored(t, h), "b", "a")
	if h.model().mode != modeNormal {
		t.Error("with nothing left selected the mode must end")
	}
}

func TestSelectModeEndsWhenSelectionIsCleared(t *testing.T) {
	h := moveHarness(t)
	h.press("v", "down", "S")
	if h.model().mode != modeNormal || len(selectedValues(h)) != 0 {
		t.Errorf("mode=%d selected=%v", h.model().mode, selectedValues(h))
	}
}

func TestSelectModeEndsWhenTheFilterChanges(t *testing.T) {
	h := moveHarness(t)
	h.press("v", "/")
	h.typeText("c")
	if h.model().mode != modeNormal {
		t.Error("the rows the anchor refers to changed, so the mode must end")
	}
}

func TestSelectModeSurvivesNewEntries(t *testing.T) {
	h := moveHarness(t)
	h.press("down", "v", "down") // anchored on c, c and b selected
	if _, err := h.hist.AddText("fresh"); err != nil {
		t.Fatal(err)
	}
	h.send(tickMsg(time.Now()))
	h.press("down")
	got := selectedValues(h)
	slices.Sort(got)
	wantOrder(t, "the anchor must not slide when rows are added above it", got, "a", "b", "c")
}

func TestSelectThenMove(t *testing.T) {
	h := moveHarness(t)
	h.press("v", "down", "m", "end")
	wantOrder(t, "draft", listed(h), "b", "a", "d", "c")
	h.press("enter")
	wantOrder(t, "file", stored(t, h), "b", "a", "d", "c")
}

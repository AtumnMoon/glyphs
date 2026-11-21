package ui

import (
	"strings"
	"testing"

	"github.com/AtumnMoon/glyphs/internal/config"
	"github.com/AtumnMoon/glyphs/internal/data"
)

func symbolHarness(t *testing.T, tab string) *harness {
	t.Helper()
	return newHarnessWith(t, tab, func(c *config.Config) { c.SearchOnOpen = nil }, func(d *Deps) {
		d.Symbols = data.NewSymbolStore
		d.Nerd = data.NewNerdStore
	})
}

// A lazy browser shows no combined group until a query exists, builds items
// only for the selected group, and takes group sizes from counts.
func TestLazyBrowserGroups(t *testing.T) {
	built := 0
	load := func(gi int) []item {
		built++
		return []item{{text: "a", width: 1}, {text: "b", width: 1}}
	}
	search := func(q string) [][]item {
		return [][]item{{{text: "b", width: 1}}, nil, {{text: "b", width: 1}}}
	}
	b := newLazyBrowser([]string{"All results", "One", "Two"}, []int{0, 2, 2}, load, search)
	if len(b.vis) != 2 || b.vis[0] != 1 {
		t.Fatalf("vis = %v, want the two real groups only", b.vis)
	}
	if len(b.shown) != 2 || built != 1 {
		t.Errorf("shown=%d built=%d, want the first group loaded once", len(b.shown), built)
	}
	if b.count(1) != 2 {
		t.Errorf("count = %d, want 2 from counts", b.count(1))
	}
	b.refresh("b")
	if len(b.vis) != 2 || b.vis[0] != 0 || b.vis[1] != 2 {
		t.Errorf("vis with a query = %v, want [0 2]", b.vis)
	}
	if built != 1 {
		t.Error("searching must not build groups")
	}
	b.refresh("")
	if len(b.vis) != 2 || b.vis[0] != 1 {
		t.Errorf("clearing the query should drop the combined group, vis = %v", b.vis)
	}
}

func TestUnicodeTabBrowsesAndSearches(t *testing.T) {
	h := symbolHarness(t, "unicode")
	if v := h.view(); !strings.Contains(v, "Basic Latin") || strings.Contains(v, "All results") {
		t.Fatalf("blocks should list without a combined group:\n%s", v)
	}
	for _, hidden := range []string{"Private Use Area", "Low Surrogates"} {
		if strings.Contains(h.view(), hidden) {
			t.Errorf("%q must not be offered", hidden)
		}
	}
	h.press("/")
	h.typeText("grinning face")
	b := h.model().uni
	if len(b.shown) == 0 || b.shown[0].text != "\U0001F600" {
		t.Fatalf("shown[0] = %+v", b.shown)
	}
	v := h.view()
	if !strings.Contains(v, "All results") || !strings.Contains(v, "U+1F600") {
		t.Errorf("results or status line missing:\n%s", v)
	}
	h.press("enter")
	if got := h.sys.texts(); len(got) != 1 || got[0] != "\U0001F600" {
		t.Errorf("copied %q", got)
	}
}

// Invisible code points are drawn as a stand-in but copy as themselves.
func TestUnicodeInvisibleCopiesTheRealCharacter(t *testing.T) {
	h := symbolHarness(t, "unicode")
	b := h.model().uni
	if b.shown[0].text != "\x00" || b.shown[0].show != "\u2400" {
		t.Fatalf("U+0000 item = %+v", b.shown[0])
	}
	h.press("right", "enter")
	if got := h.sys.texts(); len(got) != 1 || got[0] != "\x00" {
		t.Errorf("copied %q, want a NUL", got)
	}
}

func TestNerdTabSearchesByNameAndAlias(t *testing.T) {
	h := symbolHarness(t, "nerd")
	if v := h.view(); !strings.Contains(v, "Material Design") {
		t.Fatalf("icon sets missing:\n%s", v)
	}
	h.press("/")
	h.typeText("arrow left")
	b := h.model().nerd
	if len(b.shown) == 0 {
		t.Fatal("no results for 'arrow left'")
	}
	if v := h.view(); !strings.Contains(v, "U+") {
		t.Errorf("code point missing from the status line:\n%s", v)
	}
}

func TestSymbolTabsSurviveMissingData(t *testing.T) {
	h := newHarness(t, "unicode", nil) // no Symbols provider
	if v := h.view(); strings.Contains(v, "panic") {
		t.Fatal(v)
	}
	h.press("tab")
	if h.model().tab != tabNerd {
		t.Fatalf("tab = %d", h.model().tab)
	}
	h.view()
}

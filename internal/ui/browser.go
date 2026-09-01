package ui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/AtumnMoon/glyphs/internal/data"
)

// item is one pickable glyph in the Kaomoji/Emoji panels.
type item struct {
	text   string      // what gets copied / drawn
	label  string      // short description shown under the grid
	detail string      // extra info (tags / category)
	hay    string      // lowercase search haystack
	emoji  *data.Emoji // set for emoji, used for skin tones
}

// browser is the two-pane "groups | grid" view shared by both picker tabs.
type browser struct {
	names      []string // left pane labels; index 0 is always "All"
	groups     [][]item // groups[0] is the union of everything
	gcur       cursor   // selected group
	icur       cursor   // selected item (idx into shown)
	focusItems bool

	shown     []item // what the grid currently displays
	cellW     int    // display width of a grid cell
	cols      int    // grid columns (set by layout)
	gridRows  int    // visible grid rows (set by layout)
	groupRows int    // visible group rows (set by layout)
}

func newBrowser(names []string, groups [][]item) *browser {
	b := &browser{names: names, groups: groups}
	b.refresh("")
	return b
}

// refresh recomputes b.shown for the active group/query and resets the item
// cursor. A non-empty query searches every item regardless of group.
func (b *browser) refresh(query string) {
	src := b.groups[b.gcur.idx]
	if q := strings.ToLower(strings.TrimSpace(query)); q != "" {
		terms := strings.Fields(q)
		var out []item
		for _, it := range b.groups[0] {
			ok := true
			for _, t := range terms {
				if !strings.Contains(it.hay, t) {
					ok = false
					break
				}
			}
			if ok {
				out = append(out, it)
			}
		}
		src = out
	}
	b.shown = src
	b.icur = cursor{}
	w := 0
	for _, it := range b.shown {
		w = max(w, ansi.StringWidth(it.text))
	}
	b.cellW = w + 2
}

// current returns the highlighted item.
func (b *browser) current() (item, bool) {
	if len(b.shown) == 0 {
		return item{}, false
	}
	return b.shown[clamp(b.icur.idx, 0, len(b.shown)-1)], true
}

func (b *browser) cols0() int { return max(b.cols, 1) }

// move handles directional keys. While a search is active the group pane is
// bypassed and every key drives the grid. It returns true if the selected
// group changed (the caller must then refresh with the active query).
func (b *browser) move(dir string, searching bool) (groupChanged bool) {
	n := len(b.shown)
	c := b.cols0()
	grid := b.focusItems || searching
	switch dir {
	case "up":
		if grid {
			if b.icur.idx-c >= 0 {
				b.icur.idx -= c
			}
		} else if b.gcur.idx > 0 {
			b.gcur.idx--
			return true
		}
	case "down":
		if grid {
			switch {
			case b.icur.idx+c < n:
				b.icur.idx += c
			case n > 0 && b.icur.idx/c < (n-1)/c:
				b.icur.idx = n - 1 // short last row
			}
		} else if b.gcur.idx < len(b.groups)-1 {
			b.gcur.idx++
			return true
		}
	case "left":
		switch {
		case !grid:
		case b.icur.idx%c == 0 && !searching:
			b.focusItems = false // leave the grid for the group pane
		case b.icur.idx > 0:
			b.icur.idx--
		}
	case "right":
		switch {
		case !grid:
			b.focusItems = n > 0
		case b.icur.idx < n-1:
			b.icur.idx++
		}
	case "home":
		if grid {
			b.icur.idx = 0
		} else if b.gcur.idx != 0 {
			b.gcur.idx = 0
			return true
		}
	case "end":
		if grid {
			b.icur.idx = max(n-1, 0)
		} else if b.gcur.idx != len(b.groups)-1 {
			b.gcur.idx = len(b.groups) - 1
			return true
		}
	}
	return false
}

// ---- dataset construction -------------------------------------------------

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func buildKaomoji(store *data.KaomojiStore) *browser {
	var all []item
	for _, k := range store.All() {
		all = append(all, item{
			text:   k.Value,
			label:  title(k.Category),
			detail: "category: " + k.Category,
			hay:    strings.ToLower(k.Category + " " + k.Value),
		})
	}
	names := []string{"All"}
	groups := [][]item{all}
	for _, c := range store.Categories() {
		var g []item
		for _, k := range store.ByCategory(c) {
			g = append(g, item{
				text: k.Value, label: title(k.Category), detail: "category: " + k.Category,
				hay: strings.ToLower(k.Category + " " + k.Value),
			})
		}
		names = append(names, title(c))
		groups = append(groups, g)
	}
	return newBrowser(names, groups)
}

// componentGroup is the Unicode "Component" group (skin-tone swatches, hair
// styles); those aren't useful as standalone picks, so the picker hides them.
const componentGroup = 2

func buildEmoji(store *data.EmojiStore) *browser {
	type bucket struct {
		name  string
		items []item
	}
	buckets := map[int]*bucket{}
	var all []item
	for _, e := range store.All() {
		if e.Group != nil && *e.Group == componentGroup {
			continue
		}
		e := e
		gk, gname := -1, "Other"
		if e.Group != nil {
			gk, gname = *e.Group, e.GroupName()
		}
		it := item{
			text:   e.Unicode,
			label:  e.Label,
			detail: strings.Join(e.Tags, ", "),
			hay:    strings.ToLower(e.Label + " " + strings.Join(e.Tags, " ")),
			emoji:  &e,
		}
		all = append(all, it)
		if buckets[gk] == nil {
			buckets[gk] = &bucket{name: gname}
		}
		buckets[gk].items = append(buckets[gk].items, it)
	}
	keys := make([]int, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	names := []string{"All"}
	groups := [][]item{all}
	for _, k := range keys {
		names = append(names, buckets[k].name)
		groups = append(groups, buckets[k].items)
	}
	b := newBrowser(names, groups)
	b.cellW = 4
	return b
}

var skinNames = []string{"none", "light", "medium-light", "medium", "medium-dark", "dark"}

// glyph returns the text to draw/copy for it, applying the skin tone (0 = none)
// when the emoji has the standard five variants.
func (it item) glyph(skin int) string {
	if skin > 0 && it.emoji != nil && len(it.emoji.Skins) == 5 {
		return it.emoji.Skins[skin-1].Unicode
	}
	return it.text
}

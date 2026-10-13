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
	width  int         // display width of text in cells (cached)
	label  string      // short description shown under the grid
	detail string      // extra info (tags / category)
	hay    string      // lowercase search haystack
	emoji  *data.Emoji // set for emoji, used for skin tones
}

// browser is the two-pane "groups | grid" view shared by both picker tabs.
//
// groups[0] is "All"; the others are categories. While a search is active the
// group pane only lists groups that still contain matches (with match counts),
// so typing "happy" immediately shows where the matches live and picking a
// group narrows the results to it.
type browser struct {
	names  []string // group labels; index 0 is always "All"
	groups [][]item // full, unfiltered contents
	maxW   int      // widest item, in cells
	maxCol int      // upper bound on grid columns (0 = unlimited)

	filtered [][]item // groups after applying the query
	vis      []int    // indexes into groups of non-empty groups (0 always present)
	gcur     cursor   // selected row in vis
	icur     cursor   // selected item (idx into shown)
	shown    []item   // filtered[vis[gcur.idx]]

	focusItems bool
	cellW      int // display width of a grid cell
	cols       int // grid columns (set by layout)
	gridRows   int // visible grid rows (set by layout)
	groupRows  int // visible group rows (set by layout)
}

func newBrowser(names []string, groups [][]item) *browser {
	b := &browser{names: names, groups: groups}
	for _, it := range groups[0] {
		b.maxW = max(b.maxW, it.width)
	}
	b.cellW = b.maxW + 2
	b.refresh("")
	return b
}

func matches(it item, terms []string) bool {
	for _, t := range terms {
		if !strings.Contains(it.hay, t) {
			return false
		}
	}
	return true
}

// currentGroup returns the index into groups of the selected group.
func (b *browser) currentGroup() int {
	if len(b.vis) == 0 {
		return 0
	}
	return b.vis[clamp(b.gcur.idx, 0, len(b.vis)-1)]
}

// refresh re-applies the query. The selected group is kept when it still has
// matches. With a non-empty query, focus moves to the results so the arrow
// keys and Enter act on them straight away.
func (b *browser) refresh(query string) {
	prev := b.currentGroup()
	terms := strings.Fields(strings.ToLower(query))

	b.filtered = make([][]item, len(b.groups))
	b.vis = b.vis[:0]
	for gi, g := range b.groups {
		if len(terms) == 0 {
			b.filtered[gi] = g
		} else {
			for _, it := range g {
				if matches(it, terms) {
					b.filtered[gi] = append(b.filtered[gi], it)
				}
			}
		}
		if gi == 0 || len(b.filtered[gi]) > 0 {
			b.vis = append(b.vis, gi)
		}
	}

	b.gcur.idx = 0
	for vi, gi := range b.vis {
		if gi == prev {
			b.gcur.idx = vi
		}
	}
	b.gcur.off = 0
	b.selectGroup()
	if len(terms) > 0 {
		b.focusItems = len(b.shown) > 0
	}
}

// selectGroup recomputes shown for the selected group and resets the item cursor.
func (b *browser) selectGroup() {
	b.shown = b.filtered[b.currentGroup()]
	b.icur = cursor{}
}

// current returns the highlighted item.
func (b *browser) current() (item, bool) {
	if len(b.shown) == 0 {
		return item{}, false
	}
	return b.shown[clamp(b.icur.idx, 0, len(b.shown)-1)], true
}

func (b *browser) cols0() int { return max(b.cols, 1) }

// move handles directional keys and reports whether the selected group changed.
func (b *browser) move(dir string) (groupChanged bool) {
	n := len(b.shown)
	c := b.cols0()
	grid := b.focusItems
	switch dir {
	case "up":
		if grid {
			if b.icur.idx-c >= 0 {
				b.icur.idx -= c
			}
		} else if b.gcur.idx > 0 {
			b.gcur.idx--
			b.selectGroup()
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
		} else if b.gcur.idx < len(b.vis)-1 {
			b.gcur.idx++
			b.selectGroup()
			return true
		}
	case "left":
		switch {
		case !grid:
		case b.icur.idx%c == 0:
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
			b.selectGroup()
			return true
		}
	case "end":
		if grid {
			b.icur.idx = max(n-1, 0)
		} else if b.gcur.idx != len(b.vis)-1 {
			b.gcur.idx = len(b.vis) - 1
			b.selectGroup()
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
			width:  ansi.StringWidth(k.Value),
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
				text: k.Value, width: ansi.StringWidth(k.Value), label: title(k.Category), detail: "category: " + k.Category,
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

// buildEmoji builds the emoji picker. With simple set, multi-person/profession
// sequences joined by U+200D are left out: some terminals (xterm, older VTE)
// draw them as several glyphs and lose track of the cursor.
func buildEmoji(store *data.EmojiStore, simple bool) *browser {
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
		if simple && strings.ContainsRune(e.Unicode, '\u200d') {
			continue
		}
		e := e
		gk, gname := -1, "Other"
		if e.Group != nil {
			gk, gname = *e.Group, e.GroupName()
		}
		it := item{
			text:   e.Unicode,
			width:  ansi.StringWidth(e.Unicode),
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
	// real groups in Unicode order; the catch-all "Other" (-1) goes last
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if (a < 0) != (b < 0) {
			return b < 0
		}
		return a < b
	})
	names := []string{"All"}
	groups := [][]item{all}
	for _, k := range keys {
		names = append(names, buckets[k].name)
		groups = append(groups, buckets[k].items)
	}
	b := newBrowser(names, groups)
	// Emoji are two cells wide. Cap the column count: a 45-column grid of
	// colour glyphs is slow for terminals to repaint and hard to scan.
	b.cellW = 4
	b.maxCol = 20
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

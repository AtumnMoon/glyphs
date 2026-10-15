package ui

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/AtumnMoon/glyphs/internal/clipboard"
)

// entryKey identifies an entry across reloads of the history file.
func entryKey(e clipboard.ClipboardEntry) string {
	return e.Recorded + "\x00" + e.FilePath + "\x00" + e.Value
}

// isImage reports whether the entry points at an image file ("null" marks text).
func isImage(e clipboard.ClipboardEntry) bool { return e.FilePath != "null" && e.FilePath != "" }

// clipState is the clipboard-history tab.
type clipState struct {
	entries    []clipboard.ClipboardEntry // snapshot of the file, newest first
	view       []int                      // indexes into entries currently listed
	cur        cursor
	selected   map[string]bool // multi-select, by entry key
	pinnedOnly bool
	prevOff    int // first visible line of the text preview
	modTime    time.Time
	rowsVis    int // list rows (items) visible; set by layout
	imgCache   map[string]string
	wrapKey    string   // cache key of wrapped
	wrapped    []string // text preview lines for wrapKey
	landKey    string   // entry to highlight after the next reload (set by deletes)
}

func newClipState() *clipState {
	return &clipState{selected: map[string]bool{}, imgCache: map[string]string{}}
}

// apply rebuilds the visible list for the query and pinned-only toggle,
// keeping the cursor on the same entry when it is still listed.
func (c *clipState) apply(query string) {
	c.rebuild(query, c.currentKey())
}

// setEntries swaps in a fresh snapshot. The highlighted entry's key is
// captured first: c.view holds indexes into the *old* slice.
func (c *clipState) setEntries(entries []clipboard.ClipboardEntry, query string) {
	keep := c.currentKey()
	if c.landKey != "" {
		keep, c.landKey = c.landKey, ""
	}
	c.entries = entries
	c.rebuild(query, keep)
}

// survivorAfter picks the entry the cursor should land on once the entries in
// gone are deleted, like clipse: the next surviving item below the cursor, or
// the closest one above when the cursor was at the end of the list.
func (c *clipState) survivorAfter(gone map[string]bool) string {
	pick := func(i int) (string, bool) {
		if i < 0 || i >= len(c.view) || c.view[i] >= len(c.entries) {
			return "", false
		}
		k := entryKey(c.entries[c.view[i]])
		return k, !gone[k]
	}
	for i := c.cur.idx; i < len(c.view); i++ {
		if k, ok := pick(i); ok {
			return k
		}
	}
	for i := c.cur.idx - 1; i >= 0; i-- {
		if k, ok := pick(i); ok {
			return k
		}
	}
	return ""
}

func (c *clipState) currentKey() string {
	if cur, ok := c.current(); ok {
		return entryKey(cur)
	}
	return ""
}

func (c *clipState) rebuild(query, keepKey string) {

	type scored struct{ i, s int }
	var hits []scored
	q := strings.TrimSpace(query)
	for i, e := range c.entries {
		if c.pinnedOnly && !e.Pinned {
			continue
		}
		if q == "" {
			hits = append(hits, scored{i: i})
			continue
		}
		if s, ok := fuzzyScore(q, e.Value); ok {
			hits = append(hits, scored{i, s})
		}
	}
	if q != "" {
		sort.SliceStable(hits, func(a, b int) bool { return hits[a].s > hits[b].s })
	}
	c.view = c.view[:0]
	for _, h := range hits {
		c.view = append(c.view, h.i)
	}

	c.cur.idx = 0
	if keepKey != "" {
		for vi, ei := range c.view {
			if entryKey(c.entries[ei]) == keepKey {
				c.cur.idx = vi
				break
			}
		}
	}
	c.cur.fix(len(c.view), c.cur.idx, c.rowsVis)

	// drop selections whose entries vanished
	live := map[string]bool{}
	for _, e := range c.entries {
		live[entryKey(e)] = true
	}
	for k := range c.selected {
		if !live[k] {
			delete(c.selected, k)
		}
	}
}

func (c *clipState) current() (clipboard.ClipboardEntry, bool) {
	if c.cur.idx < 0 || c.cur.idx >= len(c.view) || c.view[c.cur.idx] >= len(c.entries) {
		return clipboard.ClipboardEntry{}, false
	}
	return c.entries[c.view[c.cur.idx]], true
}

// targets returns the selected entries (in list order) or, with no selection,
// just the highlighted one.
func (c *clipState) targets() []clipboard.ClipboardEntry {
	var out []clipboard.ClipboardEntry
	if len(c.selected) > 0 {
		for _, e := range c.entries {
			if c.selected[entryKey(e)] {
				out = append(out, e)
			}
		}
		return out
	}
	if e, ok := c.current(); ok {
		out = append(out, e)
	}
	return out
}

func (c *clipState) move(delta int) {
	if len(c.view) == 0 {
		return
	}
	c.cur.idx = clamp(c.cur.idx+delta, 0, len(c.view)-1)
}

// ---- row/preview text -----------------------------------------------------

func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// imageInfo returns "WxH · size" for an image entry.
func imageInfo(path string) string {
	var parts []string
	if f, err := os.Open(path); err == nil {
		if cfg, _, err := image.DecodeConfig(f); err == nil {
			parts = append(parts, fmt.Sprintf("%d×%d", cfg.Width, cfg.Height))
		}
		f.Close()
	}
	if fi, err := os.Stat(path); err == nil {
		parts = append(parts, humanSize(fi.Size()))
	} else {
		parts = append(parts, "file missing")
	}
	return strings.Join(parts, " · ")
}

// title is the one-line label of an entry.
func entryTitle(e clipboard.ClipboardEntry) string {
	if isImage(e) {
		return "🖼  image"
	}
	return oneLine(e.Value)
}

// description is the dim second line of an entry.
func entryDesc(e clipboard.ClipboardEntry) string {
	if isImage(e) {
		return e.Recorded + " · " + imageInfo(e.FilePath)
	}
	lines := strings.Count(e.Value, "\n") + 1
	return fmt.Sprintf("%s · %d chars · %d line(s)", e.Recorded, len([]rune(e.Value)), lines)
}

type clipEntry = clipboard.ClipboardEntry

// textLines returns the entry's text hard-wrapped to width w, cached because
// the preview asks for it on every keypress.
func (c *clipState) textLines(e clipEntry, w int) []string {
	key := fmt.Sprintf("%s|%d", entryKey(e), w)
	if key != c.wrapKey {
		text := strings.ReplaceAll(strings.ReplaceAll(e.Value, "\r\n", "\n"), "\t", "    ")
		c.wrapped = strings.Split(ansi.Hardwrap(text, max(w, 1), true), "\n")
		c.wrapKey = key
	}
	return c.wrapped
}

// basicImage renders the half-block fallback, cached per size.
func (c *clipState) basicImage(path string, w, h int) string {
	key := fmt.Sprintf("%s|%d|%d", path, w, h)
	if s, ok := c.imgCache[key]; ok {
		return s
	}
	if len(c.imgCache) > 8 {
		c.imgCache = map[string]string{}
	}
	s := renderBasic(path, w, h)
	c.imgCache[key] = s
	return s
}

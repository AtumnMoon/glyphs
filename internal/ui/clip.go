package ui

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"sort"
	"strings"
	"time"

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
	entries     []clipboard.ClipboardEntry // snapshot of the file, newest first
	view        []int                      // indexes into entries currently listed
	cur         cursor
	selected    map[string]bool // multi-select, by entry key
	pinnedOnly  bool
	showPreview bool
	modTime     time.Time
	rowsVis     int // list rows (items) visible; set by layout
	imgCache    map[string]string
}

func newClipState() *clipState {
	return &clipState{selected: map[string]bool{}, showPreview: true, imgCache: map[string]string{}}
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
	c.entries = entries
	c.rebuild(query, keep)
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

// renderImage draws a PNG/JPEG/GIF using half-block characters (two pixels
// per cell, 24-bit colour), scaled to fit w×h cells. Results are cached.
func (c *clipState) renderImage(path string, w, h int) string {
	cacheKey := fmt.Sprintf("%s|%d|%d", path, w, h)
	if s, ok := c.imgCache[cacheKey]; ok {
		return s
	}
	out := drawHalfBlocks(path, w, h)
	if len(c.imgCache) > 16 {
		c.imgCache = map[string]string{}
	}
	c.imgCache[cacheKey] = out
	return out
}

func drawHalfBlocks(path string, w, h int) string {
	f, err := os.Open(path)
	if err != nil {
		return "(image file missing)"
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		// try the generic decoder (jpeg/gif registered above)
		if _, serr := f.Seek(0, 0); serr != nil {
			return "(cannot read image)"
		}
		img, _, err = image.Decode(f)
		if err != nil {
			return "(cannot decode image)"
		}
	}
	b := img.Bounds()
	iw, ih := b.Dx(), b.Dy()
	if iw == 0 || ih == 0 || w < 1 || h < 1 {
		return ""
	}
	// each cell is 1 px wide, 2 px tall
	pxW, pxH := w, h*2
	scale := min(float64(pxW)/float64(iw), float64(pxH)/float64(ih))
	if scale > 1 {
		scale = 1
	}
	tw := max(int(float64(iw)*scale), 1)
	th := max(int(float64(ih)*scale), 1)
	if th%2 == 1 {
		th++
	}

	// box-average sampling
	sample := func(x, y int) (r, g, bl uint32) {
		x0, x1 := x*iw/tw, max((x+1)*iw/tw, x*iw/tw+1)
		y0, y1 := y*ih/th, max((y+1)*ih/th, y*ih/th+1)
		var rs, gs, bs, n uint32
		for yy := y0; yy < y1 && yy < ih; yy++ {
			for xx := x0; xx < x1 && xx < iw; xx++ {
				cr, cg, cb, ca := img.At(b.Min.X+xx, b.Min.Y+yy).RGBA()
				if ca == 0 {
					continue
				}
				rs += cr >> 8
				gs += cg >> 8
				bs += cb >> 8
				n++
			}
		}
		if n == 0 {
			return 0, 0, 0
		}
		return rs / n, gs / n, bs / n
	}

	var sb strings.Builder
	for y := 0; y < th; y += 2 {
		for x := 0; x < tw; x++ {
			r1, g1, b1 := sample(x, y)
			r2, g2, b2 := sample(x, y+1)
			fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", r1, g1, b1, r2, g2, b2)
		}
		sb.WriteString("\x1b[0m")
		if y+2 < th {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

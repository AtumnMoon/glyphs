package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	tabBarH  = 1
	searchH  = 1
	bottomH  = 2 // detail line + status line
	dividerW = 3 // " │ "
	minBodyH = 3
)

// geom is the layout derived from the window size and current state.
type geom struct {
	bodyH         int
	leftW, rightW int // picker tabs: group pane / grid pane
	rowsPerItem   int // clipboard list
	helpView      string
}

// layout recomputes geometry and pushes the pane sizes into the tab states so
// navigation (which runs before the next render) uses current numbers.
func (m *model) layout() {
	helpKey := fmt.Sprintf("%d|%v|%v|%v|%d", m.tab, m.help.ShowAll, m.preview, m.typing, m.width)
	if helpKey != m.helpKey {
		m.helpKey = helpKey
		keys := tabKeys{k: m.keys, tab: m.tab, preview: m.preview, typing: m.typing}
		m.helpView = m.clipHelp(m.help.View(keys))
		m.helpH = lipgloss.Height(m.helpView)
		// On a short terminal the expanded help would crowd out the content:
		// fall back to the one-line help.
		if m.help.ShowAll && m.height-tabBarH-searchH-bottomH-m.helpH < minBodyH {
			m.help.ShowAll = false
			m.helpView = m.clipHelp(m.help.View(keys))
			m.helpH = lipgloss.Height(m.helpView)
			m.help.ShowAll = true
		}
	}
	g := geom{helpView: m.helpView}
	g.bodyH = max(m.height-tabBarH-searchH-bottomH-m.helpH, minBodyH)

	w := m.width
	switch m.tab {
	case tabKaomoji, tabEmoji:
		g.leftW = clamp(w*22/100, 14, 26)
		g.rightW = max(w-g.leftW-dividerW, 4)
		b := m.browser()
		cell := min(max(b.cellW, 2), g.rightW)
		b.cols = max(g.rightW/cell, 1)
		if b.maxCol > 0 {
			b.cols = min(b.cols, b.maxCol)
		}
		b.gridRows = max(g.bodyH-1, 1)
		if b.maxRows > 0 {
			b.gridRows = min(b.gridRows, b.maxRows)
		}
		b.groupRows = max(g.bodyH-1, 1)
	case tabClipboard:
		g.rowsPerItem = 1
		if m.cfg.EnableDescription {
			g.rowsPerItem = 2
		}
		m.clip.rowsVis = max((g.bodyH-1)/g.rowsPerItem, 1)
	}
	m.g = g
}

func (m *model) fixCursors() {
	if b := m.browser(); b != nil {
		b.gcur.fix(len(b.vis), b.gcur.idx, b.groupRows)
		b.icur.fix(len(b.shown), b.icur.idx/b.cols0(), b.gridRows)
		return
	}
	c := m.clip
	c.cur.fix(len(c.view), c.cur.idx, c.rowsVis)
	if m.preview {
		if _, ok := c.current(); !ok {
			m.preview = false // entry vanished (deleted/expired)
			return
		}
		lines := c.textLines(m.previewEntry(), m.width)
		c.prevOff = clamp(c.prevOff, 0, max(len(lines)-m.previewRows(), 0))
	}
}

// previewRows is the number of content rows in the preview pane (the pane has
// a header line and a footer line around them).
func (m *model) previewRows() int { return max(m.g.bodyH-2, 1) }

func (m *model) View() tea.View {
	v := tea.NewView("")
	v.WindowTitle = "Glyphs"
	v.AltScreen = true
	if m.cfg.EnableMouse {
		v.MouseMode = tea.MouseModeCellMotion
	}
	if m.quitting {
		return v
	}
	if m.width == 0 || m.height == 0 {
		v.Content = "Loading…"
		return v
	}

	var body []string
	switch {
	case m.tab == tabClipboard && m.preview:
		body = m.viewPreview()
	case m.tab == tabClipboard:
		body = padLines(m.viewClipList(m.width), m.g.bodyH)
	default:
		body = m.viewBrowser(m.browser())
	}

	out := make([]string, 0, m.height)
	out = append(out, m.viewTabs(), m.viewSearch())
	out = append(out, body...)
	out = append(out, m.viewDetail(), m.viewStatus(), m.g.helpView)
	v.Content = strings.Join(out, "\n")
	return v
}

// clipHelp truncates every help line to the window width; the expanded help
// does not wrap or truncate on its own.
func (m *model) clipHelp(s string) string {
	if m.width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, m.width, "…")
	}
	return strings.Join(lines, "\n")
}

// ---- tab bar and search bar --------------------------------------------------

func (m *model) viewTabs() string {
	n := len(tabNames)
	var parts []string
	used := 0
	for i, name := range tabNames {
		w := m.width / n
		if i == n-1 {
			w = m.width - used
		}
		used += w
		label := fmt.Sprintf("%d %s", i+1, name)
		pad := max(w-ansi.StringWidth(label), 0)
		cell := strings.Repeat(" ", pad/2) + label + strings.Repeat(" ", pad-pad/2)
		if tabID(i) == m.tab {
			parts = append(parts, m.st.tabActive.Render(cell))
		} else {
			parts = append(parts, m.st.tabInactive.Render(cell))
		}
	}
	return strings.Join(parts, "")
}

// viewSearch is the always-visible search bar.
func (m *model) viewSearch() string {
	prompt := m.st.prompt.Render("/ ")
	switch {
	case m.typing && m.query == "":
		return prompt + m.st.prompt.Render("▏") + m.st.muted.Render("type to search…")
	case m.typing:
		return prompt + m.st.text.Render(m.query) + m.st.prompt.Render("▏")
	case m.query != "":
		return prompt + m.st.text.Render(m.query) + m.st.muted.Render("   (esc clears)")
	}
	return m.st.muted.Render("press '/' for filter")
}

// ---- picker tabs ---------------------------------------------------------------

func (m *model) viewBrowser(b *browser) []string {
	g := m.g
	groupFocus := !b.focusItems

	// left: group list
	left := make([]string, 0, g.bodyH)
	titleStyle := m.st.paneTitle
	if !groupFocus {
		titleStyle = m.st.paneTitleDim
	}
	heading := "Categories"
	if m.tab == tabEmoji {
		heading = "Groups"
	}
	left = append(left, titleStyle.Render(fit(heading, g.leftW)))
	for vi := b.gcur.off; vi < len(b.vis) && len(left) < g.bodyH; vi++ {
		gi := b.vis[vi]
		label := " " + b.names[gi]
		count := fmt.Sprintf("%d ", len(b.filtered[gi]))
		w := g.leftW - ansi.StringWidth(count)
		line := fit(label, w) + count
		switch {
		case vi == b.gcur.idx && groupFocus:
			left = append(left, m.st.rowSel.Render(line))
		case vi == b.gcur.idx:
			left = append(left, m.st.rowSelDim.Render(line))
		default:
			left = append(left, m.st.row.Render(fit(label, w))+m.st.muted.Render(count))
		}
	}

	// right: grid
	right := make([]string, 0, g.bodyH)
	head := fmt.Sprintf("%s — %d", b.names[b.currentGroup()], len(b.shown))
	if q := strings.TrimSpace(m.query); q != "" {
		head = fmt.Sprintf("“%s” in %s — %d", q, b.names[b.currentGroup()], len(b.shown))
	}
	rt := m.st.paneTitle
	if groupFocus {
		rt = m.st.paneTitleDim
	}
	right = append(right, rt.Render(fit(head, g.rightW)))

	if len(b.shown) == 0 {
		right = append(right, m.st.muted.Render(fit("  Nothing matches.", g.rightW)))
	}
	cell := min(max(b.cellW, 2), g.rightW)
	cols := b.cols0()
	for row := b.icur.off; len(right) < g.bodyH && row-b.icur.off < b.gridRows && row*cols < len(b.shown); row++ {
		right = append(right, m.gridRow(b, row, cols, cell, g.rightW, groupFocus))
	}
	return m.joinPanes(left, right, g.leftW, g.rightW, g.bodyH)
}

// gridRow renders one grid row. Cells are built from cached widths and styled
// in at most three runs (before / selected / after) instead of one style call
// per cell, which is what made large emoji grids slow to redraw.
func (m *model) gridRow(b *browser, row, cols, cell, rightW int, groupFocus bool) string {
	var before, after strings.Builder
	var sel string
	selected := false
	for c := range cols {
		idx := row*cols + c
		if idx >= len(b.shown) {
			break
		}
		it := b.shown[idx]
		text := it.text
		if m.tab == tabEmoji {
			text = it.glyph(m.skin)
		}
		var s string
		if it.width+1 > cell {
			s = fit(" "+text, cell)
		} else {
			s = " " + text + strings.Repeat(" ", cell-1-it.width)
		}
		switch {
		case idx == b.icur.idx:
			selected = true
			if groupFocus {
				sel = m.st.rowSelDim.Render(s)
			} else {
				sel = m.st.rowSel.Render(s)
			}
		case selected:
			after.WriteString(s)
		default:
			before.WriteString(s)
		}
	}
	line := m.st.row.Render(before.String()) + sel + m.st.row.Render(after.String())
	return line
}

// joinPanes zips two panes line by line around a divider, padding each to its
// width and the block to height h.
func (m *model) joinPanes(left, right []string, lw, rw, h int) []string {
	div := m.st.divider.Render(" │ ")
	out := make([]string, 0, h)
	for i := range h {
		l, r := strings.Repeat(" ", lw), ""
		if i < len(left) {
			l = padTo(left[i], lw)
		}
		if i < len(right) {
			r = padTo(right[i], rw)
		}
		out = append(out, l+div+r)
	}
	return out
}

// padTo right-pads an already-styled string with spaces up to w cells.
func padTo(s string, w int) string {
	if pad := w - ansi.StringWidth(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

func padLines(lines []string, h int) []string {
	for len(lines) < h {
		lines = append(lines, "")
	}
	return lines[:h]
}

// ---- clipboard tab ---------------------------------------------------------------

func (m *model) listenerBadge() string {
	switch {
	case m.deps.Paused != nil && m.deps.Paused():
		return m.st.pin.Render("⏸ paused")
	case m.deps.ListenerRunning != nil && m.deps.ListenerRunning():
		return m.st.status.Render("● listening")
	}
	return m.st.muted.Render("○ listener off")
}

func (m *model) viewClipList(w int) []string {
	g, c := m.g, m.clip
	head := fmt.Sprintf("History — %d", len(c.view))
	if c.pinnedOnly {
		head += " · pinned only"
	}
	if n := len(c.selected); n > 0 {
		head += fmt.Sprintf(" · %d selected", n)
	}
	badge := m.listenerBadge()
	hw := w - ansi.StringWidth(badge) - 1
	lines := []string{m.st.paneTitle.Render(fit(head, hw)) + " " + badge}

	if len(c.view) == 0 {
		msg := "History is empty. Run `glyphs -listen` to start recording."
		if c.pinnedOnly {
			msg = "No pinned entries."
		} else if strings.TrimSpace(m.query) != "" {
			msg = "Nothing matches."
		}
		return append(lines, m.st.muted.Render(fit("  "+msg, w)))
	}

	for vi := c.cur.off; vi < len(c.view) && len(lines) < g.bodyH; vi++ {
		e := c.entries[c.view[vi]]
		isCur := vi == c.cur.idx
		rowSt := m.st.row
		if isCur {
			rowSt = m.st.rowSel
		}

		sel, pin := " ", " "
		if c.selected[entryKey(e)] {
			sel = "✓"
		}
		if e.Pinned {
			pin = "📌"
		}
		mark := func(glyph string, st lipgloss.Style) string {
			if isCur {
				return rowSt.Render(glyph)
			}
			return st.Render(glyph)
		}
		prefix := mark(sel, m.st.multi) + rowSt.Render(" ") + mark(pin, m.st.pin) + rowSt.Render(" ")
		textW := w - 4
		titleW := min(m.cfg.MaxEntryLength, textW)
		lines = append(lines, prefix+rowSt.Render(fit(entryTitle(e), titleW)+strings.Repeat(" ", max(textW-titleW, 0))))

		if g.rowsPerItem == 2 && len(lines) < g.bodyH {
			d := m.st.muted
			if isCur {
				d = m.st.rowSelDim
			}
			lines = append(lines, d.Render(fit("    "+entryDesc(e), w)))
		}
	}
	return lines
}

// previewEntry returns the entry shown in the preview.
func (m *model) previewEntry() clipEntry {
	e, _ := m.clip.current()
	return e
}

// viewPreview is the clipse-style full-pane preview: a header, the content, and
// a footer with the scroll position.
func (m *model) viewPreview() []string {
	w, rows := m.width, m.previewRows()
	e := m.previewEntry()
	c := m.clip

	header := "Preview"
	lines := make([]string, 0, m.g.bodyH)
	var footerLeft, footerRight string

	if isImage(e) {
		footerLeft = imageInfo(e.FilePath)
		var content []string
		if m.sixelActive() {
			content = make([]string, rows) // blank: the image is painted over it
		} else {
			content = strings.Split(c.basicImage(e.FilePath, w, rows), "\n")
			if len(content) > rows {
				content = content[:rows]
			}
			header += "  (basic image mode)"
			if m.cfg.ImagePreview != "off" {
				footerRight = ""
			}
		}
		if m.cfg.ImagePreview == "off" {
			content = []string{m.st.muted.Render("  Image preview is turned off (imagePreview in config.json).")}
		}
		lines = append(lines, m.st.paneTitle.Render(fit(header, w)))
		lines = append(lines, content...)
	} else {
		all := c.textLines(e, w)
		off := clamp(c.prevOff, 0, max(len(all)-rows, 0))
		end := min(off+rows, len(all))
		lines = append(lines, m.st.paneTitle.Render(fit(header, w)))
		for _, l := range all[off:end] {
			lines = append(lines, m.st.text.Render(l))
		}
		pct := 100
		if len(all) > rows {
			pct = off * 100 / (len(all) - rows)
		}
		footerLeft = fmt.Sprintf("%d line(s)", len(all))
		footerRight = fmt.Sprintf("%d%%", pct)
	}

	lines = padLines(lines, rows+1)
	pct := m.st.paneTitle.Render(footerRight)
	left := m.st.muted.Render(fit(footerLeft, max(w-ansi.StringWidth(footerRight)-1, 0)))
	lines = append(lines, left+" "+pct)
	return padLines(lines, m.g.bodyH)
}

// ---- bottom lines ----------------------------------------------------------------

func (m *model) viewDetail() string {
	if m.confirm != nil {
		return m.st.err.Render(fit(m.confirmQ, m.width))
	}
	if b := m.browser(); b != nil {
		it, ok := b.current()
		if !ok {
			return ""
		}
		var s string
		if m.tab == tabEmoji {
			s = fmt.Sprintf("%s  %s", it.label, it.detail)
			if it.emoji != nil {
				s = fmt.Sprintf("%s  ·  U+%s  ·  %s", it.label, strings.ReplaceAll(it.emoji.Hexcode, "-", " U+"), it.detail)
			}
		} else {
			s = fmt.Sprintf("%s  ·  %d chars", it.label, len([]rune(it.text)))
		}
		return m.st.muted.Render(fit(s, m.width))
	}
	return ""
}

func (m *model) viewStatus() string {
	switch {
	case m.status == "":
		return ""
	case m.statusErr:
		return m.st.err.Render(fit(m.status, m.width))
	}
	return m.st.status.Render(fit(m.status, m.width))
}

// ---- sixel images ------------------------------------------------------------------

// sixelActive reports whether the preview should be drawn as a sixel bitmap.
func (m *model) sixelActive() bool {
	switch m.cfg.ImagePreview {
	case "sixel":
		return true
	case "auto":
		return m.sixelOK && m.cellW > 0 && m.cellH > 0
	}
	return false
}

// cellPx returns the cell size in pixels, with a typical fallback when the
// terminal never answered (only reachable with imagePreview = "sixel").
func (m *model) cellPx() (int, int) {
	if m.cellW > 0 && m.cellH > 0 {
		return m.cellW, m.cellH
	}
	return 8, 16
}

// syncImage paints the sixel bitmap when the preview shows an image that is not
// on screen yet. The terminal is only told after a short delay so Bubble Tea's
// own frame has been flushed first (otherwise the frame could overwrite it).
func (m *model) syncImage() tea.Cmd {
	e, ok := m.clip.current()
	if m.tab != tabClipboard || !m.preview || !ok || !isImage(e) || !m.sixelActive() || m.width == 0 {
		if m.imgSig != "" || m.imgReq != "" {
			// A bitmap may be on screen. Bubble Tea only rewrites cells whose
			// text changed, and the cells under the image were blank before and
			// after, so it would never erase the picture: force a full repaint.
			m.imgSig, m.imgReq = "", ""
			return tea.ClearScreen
		}
		return nil
	}
	cw, ch := m.cellPx()
	col, row := 1, tabBarH+searchH+1+1 // 1-based cell below the preview header
	cols, rows := m.width, m.previewRows()
	sig := fmt.Sprintf("%s|%d,%d|%d,%d|%d,%d", e.FilePath, col, row, cols, rows, cw, ch)
	if sig == m.imgSig || sig == m.imgReq {
		return nil
	}
	m.imgReq = sig
	path := e.FilePath
	return func() tea.Msg {
		time.Sleep(40 * time.Millisecond)
		data, err := sixelPaint(path, col, row, cols, rows, cw, ch)
		if err != nil {
			return imageMsg{sig: sig}
		}
		return imageMsg{sig: sig, data: data}
	}
}

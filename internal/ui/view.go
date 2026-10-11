package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/AtumnMoon/glyphs/internal/clipboard"
)

const (
	tabBarH   = 1
	bottomH   = 2 // detail/filter line + status line
	dividerW  = 3 // " │ "
	minBodyH  = 3
	narrowCut = 90 // below this width the clipboard preview replaces the list
)

// geom is the layout derived from the window size and current state.
type geom struct {
	bodyH         int
	leftW, rightW int // picker tabs: group pane / grid pane
	listW, prevW  int // clipboard tab: list / preview (0 = hidden)
	rowsPerItem   int
	helpView      string
}

// layout recomputes geometry and pushes the pane sizes into the tab states so
// navigation (which runs before the next render) uses current numbers.
func (m *model) layout() {
	helpView := m.clipHelp(m.help.View(tabKeys{m.keys, m.tab}))
	g := geom{helpView: helpView}
	g.bodyH = max(m.height-tabBarH-bottomH-lipgloss.Height(helpView), minBodyH)

	w := m.width
	switch m.tab {
	case tabKaomoji, tabEmoji:
		g.leftW = clamp(w*22/100, 14, 24)
		g.rightW = max(w-g.leftW-dividerW, 4)
		b := m.browser()
		cell := min(max(b.cellW, 2), g.rightW)
		b.cols = max(g.rightW/cell, 1)
		b.gridRows = max(g.bodyH-1, 1)
		b.groupRows = max(g.bodyH-1, 1)
	case tabClipboard:
		g.rowsPerItem = 1
		if m.cfg.EnableDescription {
			g.rowsPerItem = 2
		}
		switch {
		case !m.clip.showPreview:
			g.listW = w
		case w >= narrowCut:
			g.listW = w * 50 / 100
			g.prevW = max(w-g.listW-dividerW, 4)
		default:
			g.prevW = w // narrow: preview replaces the list
		}
		m.clip.rowsVis = max((g.bodyH-1)/g.rowsPerItem, 1)
	}
	m.g = g
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

func (m *model) fixCursors() {
	if b := m.browser(); b != nil {
		b.gcur.fix(len(b.groups), b.gcur.idx, b.groupRows)
		b.icur.fix(len(b.shown), b.icur.idx/b.cols0(), b.gridRows)
		return
	}
	c := m.clip
	c.cur.fix(len(c.view), c.cur.idx, c.rowsVis)
}

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
	m.layout()

	var body []string
	switch m.tab {
	case tabKaomoji, tabEmoji:
		body = m.viewBrowser(m.browser())
	case tabClipboard:
		body = m.viewClipboard()
	}

	var out []string
	out = append(out, m.viewTabs())
	out = append(out, body...)
	out = append(out, m.viewDetail(), m.viewStatus(), m.g.helpView)
	v.Content = strings.Join(out, "\n")
	return v
}

// ---- tab bar ---------------------------------------------------------------

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

// ---- picker tabs -----------------------------------------------------------

func (m *model) viewBrowser(b *browser) []string {
	g := m.g
	searching := strings.TrimSpace(m.query) != ""
	groupFocus := !b.focusItems && !searching

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
	for i := b.gcur.off; i < len(b.groups) && len(left) < g.bodyH; i++ {
		label := fmt.Sprintf(" %s", b.names[i])
		count := fmt.Sprintf("%d ", len(b.groups[i]))
		w := g.leftW - ansi.StringWidth(count)
		line := fit(label, w) + count
		switch {
		case i == b.gcur.idx && groupFocus:
			left = append(left, m.st.rowSel.Render(line))
		case i == b.gcur.idx:
			left = append(left, m.st.rowSelDim.Render(line))
		default:
			left = append(left, m.st.row.Render(fit(label, w))+m.st.muted.Render(count))
		}
	}

	// right: grid
	right := make([]string, 0, g.bodyH)
	var head string
	if searching {
		head = fmt.Sprintf("Results for “%s” — %d", strings.TrimSpace(m.query), len(b.shown))
	} else {
		head = fmt.Sprintf("%s — %d", b.names[b.gcur.idx], len(b.shown))
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
	for row := b.icur.off; len(right) < g.bodyH; row++ {
		if row*cols >= len(b.shown) {
			break
		}
		var sb strings.Builder
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
			s := fit(" "+text, cell)
			switch {
			case idx == b.icur.idx && !groupFocus:
				s = m.st.rowSel.Render(s)
			case idx == b.icur.idx:
				s = m.st.rowSelDim.Render(s)
			default:
				s = m.st.row.Render(s)
			}
			sb.WriteString(s)
		}
		right = append(right, sb.String())
	}
	return m.joinPanes(left, right, g.leftW, g.rightW, g.bodyH)
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

// ---- clipboard tab ---------------------------------------------------------

func (m *model) listenerBadge() string {
	switch {
	case m.deps.Paused != nil && m.deps.Paused():
		return m.st.pin.Render("⏸ paused")
	case m.deps.ListenerRunning != nil && m.deps.ListenerRunning():
		return m.st.status.Render("● listening")
	}
	return m.st.muted.Render("○ listener off")
}

func (m *model) viewClipboard() []string {
	g, c := m.g, m.clip

	var list []string
	if g.listW > 0 {
		list = m.viewClipList(g.listW)
	}
	var prev []string
	if g.prevW > 0 {
		prev = m.viewClipPreview(g.prevW)
	}
	switch {
	case g.listW > 0 && g.prevW > 0:
		return m.joinPanes(list, prev, g.listW, g.prevW, g.bodyH)
	case g.prevW > 0:
		return padLines(prev, g.bodyH)
	}
	_ = c
	return padLines(list, g.bodyH)
}

func padLines(lines []string, h int) []string {
	for len(lines) < h {
		lines = append(lines, "")
	}
	return lines[:h]
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
			pin = "★"
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

func (m *model) viewClipPreview(w int) []string {
	g, c := m.g, m.clip
	lines := []string{m.st.paneTitle.Render(fit("Preview", w))}
	e, ok := c.current()
	if !ok {
		return append(lines, m.st.muted.Render(fit("  Nothing selected.", w)))
	}
	avail := g.bodyH - 1

	if isImage(e) {
		info := imageInfo(e.FilePath)
		lines = append(lines, m.st.muted.Render(fit(info, w)))
		img := c.renderImage(e.FilePath, w, max(avail-1, 1))
		return append(lines, strings.Split(img, "\n")...)
	}

	wrapped := ansi.Hardwrap(strings.ReplaceAll(e.Value, "\t", "    "), w, true)
	all := strings.Split(wrapped, "\n")
	for i, l := range all {
		if i >= avail {
			lines[len(lines)-1] = padTo(lines[len(lines)-1], w)
			lines[len(lines)-1] = m.st.muted.Render(fit(fmt.Sprintf("… %d more line(s)", len(all)-i+1), w))
			break
		}
		lines = append(lines, m.st.text.Render(l))
	}
	return lines
}

// ---- bottom lines ----------------------------------------------------------

func (m *model) viewDetail() string {
	switch {
	case m.confirm != nil:
		return m.st.err.Render(fit(m.confirmQ, m.width))
	case m.filtering:
		return m.st.prompt.Render("/ ") + m.st.text.Render(m.query) + m.st.prompt.Render("▏")
	case strings.TrimSpace(m.query) != "":
		return m.st.prompt.Render("/ ") + m.st.text.Render(m.query) + m.st.muted.Render("   (esc to clear)")
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

	if e, ok := m.clip.current(); ok {
		return m.st.muted.Render(fit(shortEntryDetail(e), m.width))
	}
	return ""
}

func shortEntryDetail(e clipboard.ClipboardEntry) string {
	if isImage(e) {
		return e.FilePath
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

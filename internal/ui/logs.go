package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/AtumnMoon/glyphs/internal/logx"
)

// logLimit is how many entries the tab loads from the end of the log file.
const logLimit = 1000

// levelAll means "no level filter".
const levelAll = -1

// logState is the Logs tab: the config problems found at startup followed by
// the log file, newest first, filtered by level and by words.
type logState struct {
	problems []logx.Entry // config errors and warnings, shown above the file
	file     []logx.Entry
	entries  []logx.Entry // problems, then file
	view     []int        // indexes into entries currently listed
	cur      cursor
	level    int // levelAll, or the one logx.Level to show
	prevOff  int // first visible line of the preview
	modTime  time.Time
	rowsVis  int
}

func newLogState(problems []logx.Entry) *logState {
	l := &logState{problems: problems, level: levelAll}
	l.entries = append([]logx.Entry(nil), problems...)
	return l
}

func logKey(e logx.Entry) string {
	return fmt.Sprintf("%d|%d|%s", e.Time.UnixNano(), e.Level, e.Msg)
}

// keepKey is the key of the entry the cursor should stay on across a rebuild,
// or "" when it should not stay anywhere: a cursor on the first row keeps
// following the newest entry, like tailing a log.
func (l *logState) keepKey() string {
	if l.cur.idx == 0 {
		return ""
	}
	if e, ok := l.current(); ok {
		return logKey(e)
	}
	return ""
}

// setFile swaps in a fresh read of the log file. The cursor's entry is
// captured first: l.view holds indexes into the *old* slice.
func (l *logState) setFile(file []logx.Entry, query string) {
	keep := l.keepKey()
	l.file = file
	l.entries = append(append([]logx.Entry(nil), l.problems...), file...)
	l.rebuild(query, keep)
}

func (l *logState) apply(query string) { l.rebuild(query, l.keepKey()) }

func (l *logState) rebuild(query, keep string) {
	words := strings.Fields(strings.ToLower(query))
	l.view = l.view[:0]
	for i, e := range l.entries {
		if l.level != levelAll && int(e.Level) != l.level {
			continue
		}
		if matchesWords(e, words) {
			l.view = append(l.view, i)
		}
	}
	l.cur.idx = 0
	if keep != "" {
		for vi, ei := range l.view {
			if logKey(l.entries[ei]) == keep {
				l.cur.idx = vi
				break
			}
		}
	}
	l.cur.fix(len(l.view), l.cur.idx, l.rowsVis)
}

// matchesWords reports whether every word occurs in the entry's text or level
// (so "error" finds ERROR entries as well as messages that mention it).
func matchesWords(e logx.Entry, words []string) bool {
	hay := strings.ToLower(e.Level.String() + " " + e.Msg)
	for _, w := range words {
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}

func (l *logState) current() (logx.Entry, bool) {
	if l.cur.idx < 0 || l.cur.idx >= len(l.view) || l.view[l.cur.idx] >= len(l.entries) {
		return logx.Entry{}, false
	}
	return l.entries[l.view[l.cur.idx]], true
}

func (l *logState) move(delta int) {
	if len(l.view) > 0 {
		l.cur.idx = clamp(l.cur.idx+delta, 0, len(l.view)-1)
	}
}

// cycleLevel steps through: all, error, warn, info.
func (l *logState) cycleLevel() {
	switch l.level {
	case levelAll:
		l.level = int(logx.Error)
	case int(logx.Error):
		l.level = int(logx.Warn)
	case int(logx.Warn):
		l.level = int(logx.Info)
	default:
		l.level = levelAll
	}
}

func (l *logState) levelName() string {
	if l.level == levelAll {
		return ""
	}
	return strings.ToLower(logx.Level(l.level).String())
}

// ---- model glue ------------------------------------------------------------

// reloadLogs re-reads the log file when it changed (or with force).
func (m *model) reloadLogs(force bool) {
	l := m.logs
	if m.deps.Logs == nil {
		return
	}
	var mt time.Time
	if m.deps.LogModTime != nil {
		mt = m.deps.LogModTime()
	}
	if !force && mt.Equal(l.modTime) {
		return
	}
	entries, err := m.deps.Logs(logLimit)
	if err != nil {
		m.setStatus("logs: "+err.Error(), true)
		return
	}
	l.modTime = mt
	q := ""
	if m.tab == tabLogs {
		q = m.query
	}
	l.setFile(entries, q)
}

func (m *model) onLogKey(msg tea.KeyPressMsg) {
	l := m.logs
	switch {
	case key.Matches(msg, m.keys.CycleLevel):
		l.cycleLevel()
		l.apply(m.query)
	case key.Matches(msg, m.keys.Preview):
		if _, ok := l.current(); ok {
			m.preview = true
			l.prevOff = 0
		}
	}
}

// onLogPreviewKey: up/down scroll a line, left/right a page, Enter copies the
// message, Space/Esc close.
func (m *model) onLogPreviewKey(msg tea.KeyPressMsg) tea.Cmd {
	k, l := m.keys, m.logs
	page := max(m.g.bodyH-3, 1)
	switch {
	case key.Matches(msg, k.Preview), key.Matches(msg, k.Quit):
		m.preview = false
	case key.Matches(msg, k.Choose):
		return m.choose()
	case key.Matches(msg, k.Up):
		l.prevOff--
	case key.Matches(msg, k.Down):
		l.prevOff++
	case key.Matches(msg, k.Left):
		l.prevOff -= page
	case key.Matches(msg, k.Right):
		l.prevOff += page
	case key.Matches(msg, k.Home):
		l.prevOff = 0
	case key.Matches(msg, k.End):
		l.prevOff = 1 << 30 // clamped by fixCursors
	}
	return nil
}

// logLines is the wrapped text of the preview.
func (m *model) logLines(e logx.Entry, w int) []string {
	head := e.Level.String()
	if !e.Time.IsZero() {
		head = e.Time.Local().Format("2006-01-02 15:04:05") + "  " + head
	}
	text := head + "\n\n" + e.Msg
	return strings.Split(ansi.Wrap(text, max(w, 1), ""), "\n")
}

func (m *model) levelStyle(l logx.Level) lipgloss.Style {
	switch l {
	case logx.Error:
		return m.st.err
	case logx.Warn:
		return m.st.warn
	}
	return m.st.muted
}

// configBadge is shown next to the listener badge while the config file has
// problems; the details are in the Logs tab.
func (m *model) configBadge() string {
	if len(m.logs.problems) == 0 {
		return ""
	}
	st := m.st.warn
	for _, p := range m.logs.problems {
		if p.Level == logx.Error {
			st = m.st.err
		}
	}
	return st.Render(m.cfg.UI.BadgeConfig)
}

// headerBadges joins the config and listener badges.
func (m *model) headerBadges() string {
	b := m.listenerBadge()
	if c := m.configBadge(); c != "" {
		b = c + " " + b
	}
	return b
}

func (m *model) viewLogList(w int) []string {
	g, l, ui := m.g, m.logs, m.cfg.UI
	head := fmt.Sprintf("%s — %d", ui.LogsTitle, len(l.view))
	if n := l.levelName(); n != "" {
		head += strings.ReplaceAll(ui.LogsLevel, "{level}", n)
	}
	var lines []string
	if badge := m.headerBadges(); ansi.StringWidth(badge) > 0 {
		hw := w - ansi.StringWidth(badge) - 1
		lines = []string{m.st.paneTitle.Render(fit(head, hw)) + " " + badge}
	} else {
		lines = []string{m.st.paneTitle.Render(fit(head, w))}
	}
	if len(l.view) == 0 {
		msg := ui.EmptyLogs
		if len(l.entries) > 0 {
			msg = ui.NoMatches
		}
		return append(lines, m.st.muted.Render(fit("  "+msg, w)))
	}

	const tagW = 5 // ERROR / WARN / INFO
	prefixW := tagW + 1
	for vi := l.cur.off; vi < len(l.view) && len(lines) < g.bodyH; vi++ {
		e := l.entries[l.view[vi]]
		isCur := vi == l.cur.idx
		rowSt := m.st.row
		if isCur {
			rowSt = m.st.rowSel
		}
		tag := fmt.Sprintf("%-*s", tagW, e.Level)
		tagSt := m.levelStyle(e.Level)
		if isCur {
			tagSt = rowSt
		}
		textW := max(w-prefixW, 0)
		lines = append(lines, tagSt.Render(tag)+rowSt.Render(" "+fit(e.Msg, textW)+strings.Repeat(" ", max(textW-ansi.StringWidth(fit(e.Msg, textW)), 0))))

		if g.rowsPerItem == 2 && len(lines) < g.bodyH {
			d := m.st.desc
			if isCur {
				d = m.st.rowSelDesc
			}
			when := "no timestamp"
			if !e.Time.IsZero() {
				when = e.Time.Local().Format("2006-01-02 15:04:05")
			}
			lines = append(lines, d.Render(fit(strings.Repeat(" ", prefixW)+when, w)))
		}
	}
	return lines
}

func (m *model) viewLogPreview() []string {
	w, rows, l := m.width, m.previewRows(), m.logs
	e, _ := l.current()
	all := m.logLines(e, w)
	off := clamp(l.prevOff, 0, max(len(all)-rows, 0))
	lines := []string{m.st.paneTitle.Render(fit(m.cfg.UI.PreviewTitle, w))}
	for i := off; i < len(all) && i < off+rows; i++ {
		lines = append(lines, m.st.text.Render(fit(all[i], w)))
	}
	for len(lines) < rows+1 {
		lines = append(lines, "")
	}
	foot := fmt.Sprintf("lines %d-%d of %d", off+1, min(off+rows, len(all)), len(all))
	return append(lines, m.st.muted.Render(fit(foot, w)))
}

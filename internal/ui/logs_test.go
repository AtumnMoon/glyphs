package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/AtumnMoon/glyphs/internal/config"
	"github.com/AtumnMoon/glyphs/internal/logx"
)

var logT0 = time.Date(2026, 11, 5, 11, 0, 0, 0, time.UTC)

func logEntry(min int, level logx.Level, msg string) logx.Entry {
	return logx.Entry{Time: logT0.Add(time.Duration(min) * time.Minute), Level: level, Msg: msg}
}

// logSource is a fake log file whose content a test can change.
type logSource struct {
	entries []logx.Entry // newest first
	mod     time.Time
}

func (s *logSource) set(e ...logx.Entry) { s.entries = e; s.mod = s.mod.Add(time.Second) }

func logHarness(t *testing.T, src *logSource, problems ...logx.Entry) *harness {
	t.Helper()
	return newHarnessWith(t, "logs", nil, func(d *Deps) {
		d.Logs = func(n int) ([]logx.Entry, error) { return src.entries, nil }
		d.LogModTime = func() time.Time { return src.mod }
		d.Problems = problems
	})
}

func sampleLogs() *logSource {
	s := &logSource{mod: logT0}
	s.entries = []logx.Entry{ // newest first
		logEntry(5, logx.Error, "record: disk full"),
		logEntry(4, logx.Info, "listener started (pid 7)"),
		logEntry(3, logx.Warn, "config: unknown option \"frob\" (ignored)"),
		logEntry(2, logx.Info, "listener stopped"),
		logEntry(1, logx.Error, "record: permission denied"),
	}
	return s
}

func shown(h *harness) []string {
	l := h.model().logs
	var out []string
	for _, i := range l.view {
		out = append(out, l.entries[i].Msg)
	}
	return out
}

func TestLogsTabListsNewestFirstWithLevels(t *testing.T) {
	h := logHarness(t, sampleLogs())
	v := h.view()
	for _, want := range []string{"4 Logs", "Logs — 5", "ERROR", "INFO", "WARN", "record: disk full", "2026-11-05"} {
		if !strings.Contains(v, want) {
			t.Errorf("view is missing %q:\n%s", want, v)
		}
	}
	if i, j := strings.Index(v, "disk full"), strings.Index(v, "permission denied"); i < 0 || j < i {
		t.Error("newest entry must be listed first")
	}
}

func TestLogsEmptyState(t *testing.T) {
	h := logHarness(t, &logSource{})
	if v := h.view(); !strings.Contains(v, "No log entries yet") {
		t.Errorf("want the empty message:\n%s", v)
	}
}

func TestLogsLevelFilterCycles(t *testing.T) {
	h := logHarness(t, sampleLogs())
	want := []struct {
		label string
		n     int
	}{{"error only", 2}, {"warn only", 1}, {"info only", 2}, {"", 5}}
	for _, w := range want {
		h.press("L")
		if got := len(shown(h)); got != w.n {
			t.Fatalf("after L (%q): %d entries, want %d", w.label, got, w.n)
		}
		v := h.view()
		if w.label != "" && !strings.Contains(v, w.label) {
			t.Errorf("header should say %q:\n%s", w.label, v)
		}
		if w.label == "" && strings.Contains(v, " only") {
			t.Errorf("no level label expected when showing everything:\n%s", v)
		}
	}
	h.press("L")
	for _, m := range shown(h) {
		if !strings.HasPrefix(m, "record:") {
			t.Errorf("error filter let %q through", m)
		}
	}
}

func TestLogsWordSearch(t *testing.T) {
	h := logHarness(t, sampleLogs())
	h.press("/")
	h.typeText("RECORD full")
	if got := shown(h); len(got) != 1 || got[0] != "record: disk full" {
		t.Errorf("all words must match, case-insensitively: %v", got)
	}
	h.press("esc", "esc") // stop typing, clear
	h.press("/")
	h.typeText("warn")
	if got := shown(h); len(got) != 1 || !strings.Contains(got[0], "frob") {
		t.Errorf("the level name is searchable too: %v", got)
	}
	h.press("esc", "esc", "/")
	h.typeText("nothing like this")
	if v := h.view(); !strings.Contains(v, "Nothing matches.") {
		t.Errorf("want the no-match message:\n%s", v)
	}
}

func TestLogsLevelAndWordsCombine(t *testing.T) {
	h := logHarness(t, sampleLogs())
	h.press("L") // errors only
	h.press("/")
	h.typeText("denied")
	if got := shown(h); len(got) != 1 || !strings.Contains(got[0], "permission") {
		t.Errorf("got %v", got)
	}
	h.press("esc", "esc") // clear the words; the level filter stays
	if got := len(shown(h)); got != 2 {
		t.Errorf("clearing the query must keep the level filter, got %d", got)
	}
}

func TestLogsEnterCopiesTheMessage(t *testing.T) {
	h := logHarness(t, sampleLogs())
	h.press("down")
	h.press("enter")
	if got := h.sys.texts(); len(got) != 1 || got[0] != "listener started (pid 7)" {
		t.Errorf("copied %v", got)
	}
}

func TestLogsPreviewShowsTheWholeMessage(t *testing.T) {
	long := "config: /home/rin/.config/glyphs/config.toml:3:27: expected a comma or array terminator but got 'q' " + strings.Repeat("padding ", 12) + "THE-END"
	src := &logSource{entries: []logx.Entry{logEntry(1, logx.Error, long)}}
	h := logHarness(t, src)
	h.send(tea.WindowSizeMsg{Width: 60, Height: 20})
	if strings.Contains(h.view(), "THE-END") {
		t.Fatal("the list row should truncate a long message")
	}
	h.press("space")
	if !h.model().preview {
		t.Fatal("space should open the preview")
	}
	v := h.view()
	if !strings.Contains(v, "THE-END") || !strings.Contains(v, "2026-11-05") {
		t.Errorf("the preview should show the time and the whole message, wrapped:\n%s", v)
	}
	h.press("esc")
	if h.model().preview || h.quit {
		t.Errorf("esc closes the preview without quitting (preview=%v quit=%v)", h.model().preview, h.quit)
	}
}

func TestConfigProblemsAreShownFirstAndFlagged(t *testing.T) {
	problems := []logx.Entry{
		{Time: logT0, Level: logx.Error, Msg: "config: config.toml:3:27: expected a comma"},
		{Time: logT0, Level: logx.Warn, Msg: "config: unknown option \"allowDuplicates\""},
	}
	h := logHarness(t, sampleLogs(), problems...)
	if got := shown(h); len(got) != 7 || !strings.Contains(got[0], "config.toml:3:27") {
		t.Errorf("problems come before the file entries: %v", got)
	}
	h.press("L")
	if got := shown(h); len(got) != 3 {
		t.Errorf("problems obey the level filter: %v", got)
	}
	h.press("tab") // wraps round to the clipboard tab
	if h.model().tab != tabClipboard {
		t.Fatalf("tab = %d", h.model().tab)
	}
	if v := h.view(); !strings.Contains(v, "⚠ config") {
		t.Errorf("the clipboard header should carry the config badge:\n%s", v)
	}
}

func TestNoConfigBadgeWithoutProblems(t *testing.T) {
	h := logHarness(t, sampleLogs())
	if strings.Contains(h.view(), "⚠") {
		t.Error("no badge expected")
	}
	h.press("tab")
	if strings.Contains(h.view(), "⚠") {
		t.Error("no badge expected on the clipboard tab")
	}
}

func TestLogsFollowNewestUnlessScrolledAway(t *testing.T) {
	src := sampleLogs()
	h := logHarness(t, src)

	src.set(append([]logx.Entry{logEntry(6, logx.Info, "fresh one")}, src.entries...)...)
	h.send(tickMsg(time.Now()))
	if got := shown(h); got[0] != "fresh one" || h.model().logs.cur.idx != 0 {
		t.Fatalf("a cursor on the first row follows the newest entry: %v idx=%d", got[:2], h.model().logs.cur.idx)
	}

	h.press("down", "down") // now on "listener started"
	cur, _ := h.model().logs.current()
	src.set(append([]logx.Entry{logEntry(7, logx.Info, "another")}, src.entries...)...)
	h.send(tickMsg(time.Now()))
	if now, _ := h.model().logs.current(); now.Msg != cur.Msg {
		t.Errorf("a cursor further down must stay on its entry: was %q, now %q", cur.Msg, now.Msg)
	}
}

func TestLogsIgnoreAnUnchangedFile(t *testing.T) {
	src := sampleLogs()
	calls := 0
	h := newHarnessWith(t, "logs", nil, func(d *Deps) {
		d.Logs = func(int) ([]logx.Entry, error) { calls++; return src.entries, nil }
		d.LogModTime = func() time.Time { return src.mod }
	})
	before := calls
	for range 5 {
		h.send(tickMsg(time.Now()))
	}
	if calls != before {
		t.Errorf("the log was re-read %d times although it did not change", calls-before)
	}
}

func TestLogsViewNeverOverflows(t *testing.T) {
	long := strings.Repeat("a very long log message ", 30)
	src := &logSource{entries: []logx.Entry{logEntry(3, logx.Error, long), logEntry(2, logx.Warn, "short"), logEntry(1, logx.Info, "multi ⏎ line")}}
	for _, sz := range [][2]int{{40, 12}, {60, 20}, {110, 32}} {
		h := logHarness(t, src, logx.Entry{Time: logT0, Level: logx.Error, Msg: long})
		h.send(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		for _, step := range []string{"", "?", "?", "space", "down", "space", "L"} {
			if step != "" {
				h.press(step)
			}
			lines := strings.Split(h.view(), "\n")
			if len(lines) > sz[1] {
				t.Errorf("%dx%d step=%q: %d lines exceed the height", sz[0], sz[1], step, len(lines))
			}
			for i, l := range lines {
				if w := ansi.StringWidth(l); w > sz[0] {
					t.Errorf("%dx%d step=%q line %d is %d wide: %q", sz[0], sz[1], step, i, w, l)
					break
				}
			}
		}
	}
}

func TestLogsTabCanBeRenamedAndOpenedByDefault(t *testing.T) {
	h := newHarness(t, "", func(c *config.Config) {
		c.DefaultTab = "logs"
		c.UI.TabNames["logs"] = "Journal"
		c.UI.LogsTitle = "Events"
	})
	if h.model().tab != tabLogs {
		t.Fatalf("default_tab = logs should open the Logs tab, got %d", h.model().tab)
	}
	if v := h.view(); !strings.Contains(v, "4 Journal") || !strings.Contains(v, "Events — 0") {
		t.Errorf("custom names missing:\n%s", v)
	}
}

// View is a pure function of the model: whatever a filter removed must be gone
// from the frame, so nothing stale can come from the app (only from a terminal).
func TestLogsFilteredFrameHasNoStaleRows(t *testing.T) {
	h := logHarness(t, sampleLogs())
	if !strings.Contains(h.view(), "listener started") {
		t.Fatal("precondition: the info entries are listed")
	}
	h.press("L") // errors only
	v := h.view()
	for _, gone := range []string{"listener started", "listener stopped", "unknown option"} {
		if strings.Contains(v, gone) {
			t.Errorf("%q survived the filter:\n%s", gone, v)
		}
	}
	if !strings.Contains(v, "disk full") || !strings.Contains(v, "permission denied") {
		t.Errorf("the error entries must be listed:\n%s", v)
	}
	if n := strings.Count(v, "\n") + 1; n != h.model().height {
		t.Errorf("the frame must always fill the window: %d lines, window %d", n, h.model().height)
	}
}

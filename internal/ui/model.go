// Package ui implements the Bubble Tea TUI: a Kaomoji picker, an Emoji picker
// and a clipboard-history manager sharing one tab bar, filter box and help bar.
package ui

import (
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/AtumnMoon/glyphs/internal/clipboard"
	"github.com/AtumnMoon/glyphs/internal/config"
	"github.com/AtumnMoon/glyphs/internal/data"
)

type tabID int

const (
	tabClipboard tabID = iota
	tabKaomoji
	tabEmoji
)

var tabNames = []string{"Clipboard", "Kaomoji", "Emoji"}

func (t tabID) key() string { return strings.ToLower(tabNames[t]) }

// Deps is everything the UI needs from the outside world.
type Deps struct {
	Config  config.Config
	Kaomoji *data.KaomojiStore
	Emoji   *data.EmojiStore
	History *clipboard.HistoryStore

	// Clipboard access, backed by golang.design/x/clipboard in the app and by
	// fakes in tests.
	WriteText  func(string) error
	WriteImage func([]byte) error
	ModTime    func() time.Time // history file mtime, polled for live updates

	// Optional status probes for the clipboard tab header.
	ListenerRunning func() bool
	Paused          func() bool
}

type (
	tickMsg   time.Time
	copiedMsg struct {
		label string
		err   error
	}
	// imageMsg carries an encoded sixel image ready to be written to the
	// terminal. sig identifies what it was encoded for; stale ones are dropped.
	imageMsg struct {
		sig  string
		data string
	}
)

type model struct {
	deps Deps
	cfg  config.Config
	keys keyMap
	help help.Model
	st   styles

	width, height int
	g             geom
	helpKey       string // layout cache key for helpView
	helpView      string
	helpH         int

	tab     tabID
	typing  bool // search box has focus: printable keys edit the query
	query   string
	preview bool // clipboard: full-pane preview is open

	kao, emo *browser
	clip     *clipState
	skin     int // 0 = default, 1..5 = skin tone

	// Terminal capabilities, learned from replies to queries sent at startup.
	sixelOK        bool
	cellW, cellH   int    // cell size in pixels (0 = unknown)
	imgSig, imgReq string // signature of the image on screen / being encoded

	status    string
	statusErr bool
	confirm   []string // entry keys awaiting delete confirmation
	confirmQ  string
	quitting  bool
}

// New creates the root model. startTab may be "", "kaomoji", "emoji" or
// "clipboard"; empty falls back to the configured default tab.
func New(d Deps, startTab string) tea.Model {
	cfg := d.Config
	if startTab == "" {
		startTab = cfg.DefaultTab
	}

	m := &model{
		deps: d,
		cfg:  cfg,
		keys: newKeyMap(cfg),
		help: help.New(),
		st:   newStyles(cfg.Theme),
		kao:  buildKaomoji(d.Kaomoji),
		emo:  buildEmoji(d.Emoji, cfg.SimpleEmoji),
		clip: newClipState(),
	}
	switch strings.ToLower(startTab) {
	case "emoji":
		m.tab = tabEmoji
	case "kaomoji":
		m.tab = tabKaomoji
	default:
		m.tab = tabClipboard
	}
	// A terminal must rasterize every distinct emoji the first time it is drawn,
	// which is slow for hundreds of colour glyphs at once. Showing a bounded
	// window of the grid keeps the first paint of a tab or category instant.
	m.emo.maxCol, m.emo.maxRows = cfg.EmojiColumns, cfg.EmojiRows
	m.typing = m.searchOnOpen(m.tab)
	m.reloadClip(true)
	return m
}

// Init starts the refresh tick and asks the terminal two things: does it do
// sixel graphics (primary device attributes), and how big is a cell in pixels.
// Terminals that don't answer simply keep the character-cell fallback.
func (m *model) Init() tea.Cmd {
	return tea.Batch(
		tick(),
		tea.Raw(ansi.RequestPrimaryDeviceAttributes),
		tea.Raw(ansi.WindowOp(ansi.RequestCellSizeWinOp)),
	)
}

func (m *model) searchOnOpen(t tabID) bool {
	return slices.Contains(m.cfg.SearchOnOpen, t.key())
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// browser returns the active picker (nil on the clipboard tab).
func (m *model) browser() *browser {
	switch m.tab {
	case tabKaomoji:
		return m.kao
	case tabEmoji:
		return m.emo
	}
	return nil
}

func (m *model) setStatus(s string, isErr bool) { m.status, m.statusErr = s, isErr }

// reloadClip refreshes the clipboard snapshot when the history file changed
// (or unconditionally with force). This is what makes the UI update live while
// the listener records new copies.
func (m *model) reloadClip(force bool) {
	if m.deps.History == nil {
		return
	}
	mt := m.deps.ModTime()
	if !force && mt.Equal(m.clip.modTime) {
		return
	}
	entries, err := m.deps.History.All()
	if err != nil {
		m.setStatus("history: "+err.Error(), true)
		return
	}
	m.clip.modTime = m.deps.ModTime()
	q := ""
	if m.tab == tabClipboard {
		q = m.query
	}
	m.clip.setEntries(entries, q)
}

// applyQuery re-runs the active tab's filter.
func (m *model) applyQuery() {
	if b := m.browser(); b != nil {
		b.refresh(m.query)
		return
	}
	m.clip.apply(m.query)
}

func (m *model) switchTab(t tabID) {
	m.tab = t
	m.query = ""
	m.typing = m.searchOnOpen(t)
	m.preview = false
	m.confirm = nil
	m.kao.refresh("")
	m.emo.refresh("")
	if t == tabClipboard {
		m.reloadClip(true)
	}
	m.clip.apply("")
}

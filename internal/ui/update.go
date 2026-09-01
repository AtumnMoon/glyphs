package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.SetWidth(m.width)

	case tickMsg:
		m.reloadClip(false)
		cmd = tick()

	case copiedMsg:
		if msg.err != nil {
			m.setStatus("copy failed: "+msg.err.Error(), true)
		} else if m.cfg.QuitAfterCopy {
			m.quitting = true
			return m, tea.Quit
		} else {
			m.setStatus("Copied "+msg.label, false)
		}

	case tea.MouseWheelMsg:
		if m.confirm == nil {
			switch msg.Mouse().Button {
			case tea.MouseWheelUp:
				for i := 0; i < 3; i++ {
					m.navigate("up")
				}
			case tea.MouseWheelDown:
				for i := 0; i < 3; i++ {
					m.navigate("down")
				}
			}
		}

	case tea.MouseClickMsg:
		if mo := msg.Mouse(); mo.Button == tea.MouseLeft && mo.Y == 0 && m.width > 0 && m.confirm == nil {
			if t := tabID(mo.X * len(tabNames) / m.width); t != m.tab {
				m.switchTab(t)
			}
		}

	case tea.KeyPressMsg:
		cmd = m.onKey(msg)
		if m.quitting {
			return m, cmd
		}

	}

	m.layout()
	m.fixCursors()
	return m, cmd
}

func (m *model) onKey(msg tea.KeyPressMsg) tea.Cmd {
	m.setStatus("", false)

	if key.Matches(msg, m.keys.ForceQuit) {
		m.quitting = true
		return tea.Quit
	}
	if m.confirm != nil {
		return m.onConfirm(msg)
	}
	if m.filtering {
		return m.onFilterKey(msg)
	}
	return m.onNormalKey(msg)
}

func (m *model) onFilterKey(msg tea.KeyPressMsg) tea.Cmd {
	s := msg.String()
	switch {
	case s == "esc":
		m.filtering, m.query = false, ""
		m.applyQuery()
		return nil
	case s == "enter":
		m.filtering = false
		return nil
	case s == "up" || s == "down":
		m.navigate(s)
		return nil
	case key.Matches(msg, m.keys.NextTab):
		m.switchTab((m.tab + 1) % tabID(len(tabNames)))
		return nil
	case key.Matches(msg, m.keys.PrevTab):
		m.switchTab((m.tab + tabID(len(tabNames)) - 1) % tabID(len(tabNames)))
		return nil
	}
	// Minimal line editor: append printable text, backspace deletes a rune.
	switch {
	case s == "backspace":
		if r := []rune(m.query); len(r) > 0 {
			m.query = string(r[:len(r)-1])
			m.applyQuery()
		}
	case msg.Text != "":
		m.query += msg.Text
		m.applyQuery()
	}
	return nil
}

func (m *model) onConfirm(msg tea.KeyPressMsg) tea.Cmd {
	keys := m.confirm
	m.confirm = nil
	if s := msg.String(); s == "y" || s == "Y" || s == "enter" {
		m.deleteKeys(keys)
	} else {
		m.setStatus("Cancelled", false)
	}
	return nil
}

func (m *model) onNormalKey(msg tea.KeyPressMsg) tea.Cmd {
	k := m.keys
	switch {
	case key.Matches(msg, k.Quit):
		if m.query != "" {
			m.query = ""
			m.applyQuery()
			return nil
		}
		m.quitting = true
		return tea.Quit
	case key.Matches(msg, k.More):
		m.help.ShowAll = !m.help.ShowAll
	case key.Matches(msg, k.NextTab):
		m.switchTab((m.tab + 1) % tabID(len(tabNames)))
	case key.Matches(msg, k.PrevTab):
		m.switchTab((m.tab + tabID(len(tabNames)) - 1) % tabID(len(tabNames)))
	case key.Matches(msg, k.Filter):
		m.filtering = true
		return nil
	case key.Matches(msg, k.Up):
		m.navigate("up")
	case key.Matches(msg, k.Down):
		m.navigate("down")
	case key.Matches(msg, k.Left):
		m.navigate("left")
	case key.Matches(msg, k.Right):
		m.navigate("right")
	case key.Matches(msg, k.Home):
		m.navigate("home")
	case key.Matches(msg, k.End):
		m.navigate("end")
	case key.Matches(msg, k.Choose):
		return m.choose()
	case key.Matches(msg, k.Skin) && m.tab == tabEmoji:
		m.skin = (m.skin + 1) % len(skinNames)
		m.setStatus("Skin tone: "+skinNames[m.skin]+" (applies to emoji that have variants)", false)
	case m.tab == tabClipboard:
		m.onClipKey(msg)
	}
	return nil
}

// navigate moves the cursor of whichever pane is active.
func (m *model) navigate(dir string) {
	if b := m.browser(); b != nil {
		if b.move(dir, strings.TrimSpace(m.query) != "") {
			b.refresh(m.query)
		}
		return
	}
	c := m.clip
	page := max(c.rowsVis-1, 1)
	switch dir {
	case "up":
		c.move(-1)
	case "down":
		c.move(1)
	case "left":
		c.move(-page)
	case "right":
		c.move(page)
	case "home":
		c.cur.idx = 0
	case "end":
		c.cur.idx = max(len(c.view)-1, 0)
	}
}

// ---- clipboard-tab actions ------------------------------------------------

func (m *model) onClipKey(msg tea.KeyPressMsg) {
	k, c := m.keys, m.clip
	switch {
	case key.Matches(msg, k.Preview):
		c.showPreview = !c.showPreview
	case key.Matches(msg, k.TogglePinned):
		c.pinnedOnly = !c.pinnedOnly
		c.apply(m.query)
		if c.pinnedOnly {
			m.setStatus("Showing pinned entries only", false)
		}
	case key.Matches(msg, k.TogglePin):
		m.togglePin()
	case key.Matches(msg, k.Remove):
		m.requestDelete()
	case key.Matches(msg, k.SelectSingle):
		if e, ok := c.current(); ok {
			if c.selected[entryKey(e)] {
				delete(c.selected, entryKey(e))
			} else {
				c.selected[entryKey(e)] = true
			}
		}
	case key.Matches(msg, k.SelectDown):
		if e, ok := c.current(); ok {
			c.selected[entryKey(e)] = true
		}
		c.move(1)
	case key.Matches(msg, k.SelectUp):
		if e, ok := c.current(); ok {
			c.selected[entryKey(e)] = true
		}
		c.move(-1)
	case key.Matches(msg, k.ClearS):
		c.selected = map[string]bool{}
	}
}

func (m *model) togglePin() {
	targets := m.clip.targets()
	if len(targets) == 0 {
		return
	}
	pin := false // pin unless every target is already pinned
	for _, e := range targets {
		if !e.Pinned {
			pin = true
		}
	}
	for _, e := range targets {
		if err := m.pinEntry(entryKey(e), pin); err != nil {
			m.setStatus("pin: "+err.Error(), true)
			return
		}
	}
	m.reloadClip(true)
	if pin {
		m.setStatus(fmt.Sprintf("Pinned %d item(s)", len(targets)), false)
	} else {
		m.setStatus(fmt.Sprintf("Unpinned %d item(s)", len(targets)), false)
	}
}

func (m *model) requestDelete() {
	targets := m.clip.targets()
	if len(targets) == 0 {
		return
	}
	keys, pinned := make([]string, 0, len(targets)), 0
	for _, e := range targets {
		keys = append(keys, entryKey(e))
		if e.Pinned {
			pinned++
		}
	}
	if pinned > 0 {
		m.confirm = keys
		m.confirmQ = fmt.Sprintf("Delete %d item(s), %d of them pinned?  [y] yes  [any key] cancel", len(keys), pinned)
		return
	}
	m.deleteKeys(keys)
}

// fileIndex finds an entry's current position in the history file. The
// clipboard package edits by index, and the listener may insert entries at the
// front while this UI holds an older snapshot, so the index is re-resolved by
// identity right before every edit.
func (m *model) fileIndex(key string) (int, error) {
	all, err := m.deps.History.All()
	if err != nil {
		return 0, err
	}
	for i, e := range all {
		if entryKey(e) == key {
			return i, nil
		}
	}
	return 0, fmt.Errorf("entry no longer exists")
}

func (m *model) pinEntry(key string, pin bool) error {
	i, err := m.fileIndex(key)
	if err != nil {
		return err
	}
	return m.deps.History.Pin(i, pin)
}

func (m *model) deleteKeys(keys []string) {
	for _, k := range keys {
		i, err := m.fileIndex(k)
		if err != nil {
			continue // already gone
		}
		if err := m.deps.History.Delete(i); err != nil {
			m.setStatus("delete: "+err.Error(), true)
			return
		}
		delete(m.clip.selected, k)
	}
	m.reloadClip(true)
	m.setStatus(fmt.Sprintf("Deleted %d item(s)", len(keys)), false)
}

// ---- choosing (copying) ---------------------------------------------------

func (m *model) choose() tea.Cmd {
	if b := m.browser(); b != nil {
		it, ok := b.current()
		if !ok {
			return nil
		}
		text := it.text
		if m.tab == tabEmoji {
			text = it.glyph(m.skin)
		}
		return func() tea.Msg { return copiedMsg{label: text, err: m.deps.WriteText(text)} }
	}

	targets := m.clip.targets()
	if len(targets) == 0 {
		return nil
	}
	if len(targets) == 1 && isImage(targets[0]) {
		path := targets[0].FilePath
		return func() tea.Msg { return copiedMsg{label: "image", err: m.copyImageFile(path)} }
	}
	var parts []string
	for _, e := range targets {
		if !isImage(e) {
			parts = append(parts, e.Value)
		}
	}
	if len(parts) == 0 {
		m.setStatus("Select a single image to copy it", true)
		return nil
	}
	text := strings.Join(parts, "\n")
	label := fmt.Sprintf("%d item(s)", len(parts))
	return func() tea.Msg { return copiedMsg{label: label, err: m.deps.WriteText(text)} }
}

func (m *model) copyImageFile(path string) error {
	b, err := readFile(path)
	if err != nil {
		return err
	}
	return m.deps.WriteImage(b)
}

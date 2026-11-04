package ui

import (
	"maps"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/AtumnMoon/glyphs/internal/clipboard"
)

// mode is the clipboard tab's input mode, in the style of Helix: the same
// movement keys do different things depending on it.
//
//	normal  navigate, and run commands on the highlighted entry or selection
//	select  movement extends the selection from where it started (v)
//	move    movement carries the selected entries up and down the list (m);
//	        Enter saves the new order, Esc throws it away
type mode int

const (
	modeNormal mode = iota
	modeSelect
	modeMove
)

// ---- select mode -------------------------------------------------------------

func (m *model) enterSelect() {
	c := m.clip
	e, ok := c.current()
	if !ok {
		return
	}
	m.mode = modeSelect
	m.selBase = maps.Clone(c.selected)
	m.selAnchor = entryKey(e)
	m.extendSelection()
	m.setStatus("SELECT: move to extend the selection, v or esc to finish", false)
}

func (m *model) exitSelect() {
	m.mode = modeNormal
	m.selBase, m.selAnchor = nil, ""
}

// extendSelection selects everything between the anchor and the cursor, on top
// of whatever was selected before the mode started. The anchor is remembered by
// key so a refresh that shifts the list cannot move it.
func (m *model) extendSelection() {
	c := m.clip
	anchor := -1
	for vi, ei := range c.view {
		if entryKey(c.entries[ei]) == m.selAnchor {
			anchor = vi
			break
		}
	}
	if anchor < 0 { // the anchor was deleted or filtered out
		m.exitSelect()
		return
	}
	lo, hi := min(anchor, c.cur.idx), max(anchor, c.cur.idx)
	sel := maps.Clone(m.selBase)
	if sel == nil {
		sel = map[string]bool{}
	}
	for vi := lo; vi <= hi && vi < len(c.view); vi++ {
		sel[entryKey(c.entries[c.view[vi]])] = true
	}
	c.selected = sel
}

// syncMode leaves select mode once there is nothing left selected (after a
// delete, or clearing the selection).
func (m *model) syncMode() {
	if m.mode == modeSelect && len(m.clip.selected) == 0 {
		m.exitSelect()
	}
}

// ---- move mode ----------------------------------------------------------------

func (m *model) enterMove() {
	c := m.clip
	switch {
	case strings.TrimSpace(m.query) != "":
		m.setStatus("Clear the filter first (esc): a filtered list can't be reordered", true)
		return
	case len(c.view) == 0:
		return
	}
	targets := c.targets()
	if len(targets) == 0 {
		return
	}
	m.mode = modeMove
	m.moved = false
	m.moveOrig = slices.Clone(c.entries)
	m.moveTargets = map[string]bool{}
	for _, e := range targets {
		m.moveTargets[entryKey(e)] = true
	}
	m.preview = false
	m.setStatus("MOVE: arrows carry the entry, enter saves, esc cancels", false)
}

func (m *model) onMoveKey(msg tea.KeyPressMsg) tea.Cmd {
	k := m.keys
	page := max(m.clip.rowsVis-1, 1)
	switch {
	case key.Matches(msg, k.MoveConfirm):
		m.commitMove()
	case key.Matches(msg, k.ModeExit):
		m.cancelMove()
	case key.Matches(msg, k.Up):
		m.moveEntries(-1, 1)
	case key.Matches(msg, k.Down):
		m.moveEntries(1, 1)
	case key.Matches(msg, k.Left):
		m.moveEntries(-1, page)
	case key.Matches(msg, k.Right):
		m.moveEntries(1, page)
	case key.Matches(msg, k.Home):
		m.moveEntries(-1, len(m.clip.entries))
	case key.Matches(msg, k.End):
		m.moveEntries(1, len(m.clip.entries))
	}
	return nil
}

// moveEntries carries the moving entries n steps in dir (-1 up, +1 down). The
// change is only in memory until commitMove.
func (m *model) moveEntries(dir, n int) {
	c := m.clip
	entries := slices.Clone(c.entries)
	visible := func(e clipboard.ClipboardEntry) bool { return !c.pinnedOnly || e.Pinned }
	changed := false
	for range n {
		if !moveBlock(entries, m.moveTargets, dir, visible) {
			break
		}
		changed = true
	}
	if !changed {
		return
	}
	m.moved = true
	c.setEntries(entries, m.query) // the cursor follows its entry
}

func (m *model) commitMove() {
	if !m.moved {
		m.leaveMove()
		m.setStatus("Order unchanged", false)
		return
	}
	keys := make([]string, len(m.clip.entries))
	for i, e := range m.clip.entries {
		keys[i] = entryKey(e)
	}
	if err := m.deps.History.Reorder(keys); err != nil {
		m.setStatus("could not save the order: "+err.Error(), true) // stay in move mode
		return
	}
	m.leaveMove()
	m.reloadClip(true)
	m.setStatus("Order saved", false)
}

func (m *model) cancelMove() {
	if m.moved {
		m.clip.setEntries(m.moveOrig, m.query)
	}
	m.leaveMove()
	m.setStatus("Cancelled", false)
}

func (m *model) leaveMove() {
	m.mode = modeNormal
	m.moved, m.moveOrig, m.moveTargets = false, nil, nil
}

// leaveModes drops out of any mode without saving, for when the tab or the
// query changes under it.
func (m *model) leaveModes() {
	switch m.mode {
	case modeMove:
		if m.moved {
			m.clip.setEntries(m.moveOrig, m.query)
		}
		m.leaveMove()
	case modeSelect:
		m.exitSelect()
	}
}

// moveBlock moves every entry whose key is in targets one step toward the front
// (dir < 0) or the back (dir > 0), hopping over the nearest visible neighbour
// that is not itself moving. Entries that are not visible (a pinned-only view
// hides the rest) are stepped over, so the order the user sees changes. Targets
// travel as a block: one that is held up by the edge, or by another target that
// is, stays put. It reports whether anything moved.
func moveBlock(entries []clipboard.ClipboardEntry, targets map[string]bool, dir int, visible func(clipboard.ClipboardEntry) bool) bool {
	isTarget := func(i int) bool { return targets[entries[i].Key()] }
	moved := false
	n := len(entries)
	if dir < 0 {
		for i := 0; i < n; i++ {
			if !isTarget(i) {
				continue
			}
			j := i - 1
			for j >= 0 && !isTarget(j) && !visible(entries[j]) {
				j--
			}
			if j < 0 || isTarget(j) {
				continue
			}
			e := entries[i]
			copy(entries[j+1:i+1], entries[j:i])
			entries[j] = e
			moved = true
		}
		return moved
	}
	for i := n - 1; i >= 0; i-- {
		if !isTarget(i) {
			continue
		}
		j := i + 1
		for j < n && !isTarget(j) && !visible(entries[j]) {
			j++
		}
		if j >= n || isTarget(j) {
			continue
		}
		e := entries[i]
		copy(entries[i:j], entries[i+1:j+1])
		entries[j] = e
		moved = true
	}
	return moved
}

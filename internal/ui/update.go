package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// If we set a width on the help menu it can gracefully truncate
		// its view as needed.
		m.help.SetWidth(m.width)

		// Adjust the height and width for layout calculation
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Filter):
			// For setting filter input later
			m.isFilter = !m.isFilter

		// Cycle through the tabs
		case key.Matches(msg, m.keys.Tab):
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
		case key.Matches(msg, m.keys.BackTab):
			m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)

		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

package ui

import (
	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type model struct {
	keys       keyMap
	help       help.Model
	inputStyle lipgloss.Style
	quitting   bool
	width      int
	height     int
	isFilter   bool
	tabs       []string
	activeTab  int
}

func NewModel() model {
	return model{
		keys:       keys,
		help:       help.New(),
		inputStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#FF75B7")),
		tabs:       []string{"Kaomoji", "Emoji", "Clipboard"},
		activeTab:  0,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

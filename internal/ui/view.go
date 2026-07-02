package ui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m model) View() tea.View {
	v := tea.NewView("")
	v.WindowTitle = "Glyphs"
	v.AltScreen = true

	if m.quitting {
		v.Content = "Bye!"
		return v
	}

	// Split the main window into content window and help window
	windowHeight := m.height * 90 / 100
	helpViewHeight := m.height - windowHeight

	// Define the main content panel
	windowStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(windowHeight).
		AlignHorizontal(lipgloss.Center).
		AlignVertical(lipgloss.Center)

	// Define the tab's active and inactive style
	activeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFF")).
		Background(lipgloss.Color("#7D56F4")).
		Bold(true)

	inactiveStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#AAA")).
		Background(lipgloss.Color("#333"))

	// Render the tab bar row
	var renderedTabs []string
	for i, t := range m.tabs {
		if i == m.activeTab {
			renderedTabs = append(renderedTabs, activeStyle.Render(t))
		} else {
			renderedTabs = append(renderedTabs, inactiveStyle.Render(t))
		}
	}
	tabRow := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)

	// Render the content box based on the active selection
	var content string
	switch m.activeTab {
	case 0:
		content = "Kaomoji picker running..."
	case 1:
		content = "Emoji picker running..."
	case 2:
		content = "Clipboard running..."
	}

	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 2).
		Width(lipgloss.Width(tabRow))

	contentBox := contentStyle.Render(content)

	// Combine components vertically
	windowContent := lipgloss.JoinVertical(lipgloss.Left, tabRow, contentBox)
	window := windowStyle.Render(windowContent)

	// Define the help panel
	helpStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(helpViewHeight).
		AlignHorizontal(lipgloss.Center).
		AlignVertical(lipgloss.Bottom).
		PaddingBottom(1)

	helpContent := m.help.View(m.keys)
	help := helpStyle.Render(helpContent)

	full := lipgloss.JoinVertical(lipgloss.Left, window, help)

	v.Content = full
	return v
}

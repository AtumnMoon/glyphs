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
	helpViewWidth := m.width

	// Calculate tab and tab content window size
	tabHeight := 1 // N characters only
	tabWidth := m.width / 3

	tabContentHeight := windowHeight - tabHeight
	tabContentWidth := m.width

	// Define the tab's active and inactive style
	tabStyle := lipgloss.NewStyle().
		Height(tabHeight).
		Width(tabWidth).
		Padding(0, 1).
		AlignHorizontal(lipgloss.Center).
		AlignVertical(lipgloss.Center)

	activeStyle := tabStyle.
		Foreground(lipgloss.Color("#FFF")).
		Background(lipgloss.Color("#7D56F4")).
		Bold(true)

	inactiveStyle := tabStyle.
		Foreground(lipgloss.Color("#AAA")).
		Background(lipgloss.Color("#333"))

	// Render the tab bar row
	var renderedTabs []string
	for index, tab := range m.tabs {
		if index == m.activeTab {
			renderedTabs = append(renderedTabs, activeStyle.Render(tab))
		} else {
			renderedTabs = append(renderedTabs, inactiveStyle.Render(tab))
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
		Height(tabContentHeight).
		Width(tabContentWidth).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 2).
		Width(lipgloss.Width(tabRow))

	contentBox := contentStyle.Render(content)

	// Combine components vertically
	window := lipgloss.JoinVertical(lipgloss.Left, tabRow, contentBox)

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

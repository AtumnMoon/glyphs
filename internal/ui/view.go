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

	helpContent := m.help.View(m.keys)
	helpViewHeight := lipgloss.Height(helpContent)

	windowHeight := m.height - helpViewHeight

	// Calculate tab and tab content window size
	tabHeight := 1 // N characters only
	tabWidth := m.width / 3

	tabContentHeight := max(windowHeight-tabHeight, 1)

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
		content = kaomojiUI()
	case 1:
		content = emojiUI()
	case 2:
		content = clipboardUI()
	}

	contentStyle := lipgloss.NewStyle().
		Height(tabContentHeight).
		Width(lipgloss.Width(tabRow)).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#7D56F4"))

	contentBox := contentStyle.Render(content)

	// Combine components vertically
	window := lipgloss.JoinVertical(lipgloss.Center, tabRow, contentBox)

	// Define the help panel
	helpStyle := lipgloss.NewStyle().
		Width(m.width).
		AlignHorizontal(lipgloss.Center)

	help := helpStyle.Render(helpContent)

	full := lipgloss.JoinVertical(lipgloss.Center, window, help)

	v.Content = full
	return v
}

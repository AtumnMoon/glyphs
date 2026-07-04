package app

import (
	"fmt"
	"glyphs/internal/ui"

	tea "charm.land/bubbletea/v2"
)

func LaunchTUI() int {
	// Create the BubbleTea application
	if _, err := tea.NewProgram(ui.NewModel()).Run(); err != nil {
		fmt.Printf("Could not start program :(\n%v\n", err)
		return 1
	}

	// Successfully exit
	return 0
}

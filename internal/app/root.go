package app

import (
	"fmt"
	"glyphs/internal/ui"
	"os"

	tea "charm.land/bubbletea/v2"
)

func Main() int {
	if os.Getenv("DEBUG") != "" {
		f, err := tea.LogToFile("debug.log", "help")
		if err != nil {
			fmt.Println("Couldn't open a file for logging:", err)
			os.Exit(1)
		}
		defer f.Close()
	}

	// Create the BubbleTea application
	if _, err := tea.NewProgram(ui.NewModel()).Run(); err != nil {
		fmt.Printf("Could not start program :(\n%v\n", err)
		return 1
	}

	// Successfully exit
	return 0
}

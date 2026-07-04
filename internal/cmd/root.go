package cmd

import (
	"flag"
	"fmt"
	"glyphs/internal/app"
	"os"
)

var (
	version = "v0.0.1dev"
	help    = flag.Bool("help", false, "Show help message")
	listen  = flag.Bool("listen", false, "Start background process for monitoring clipbaord activity on Wayland/X11/MacOS/Windows.")
)

func Main() int {
	flag.Parse()

	switch {
	case flag.NFlag() == 0:
		if len(os.Args) > 2 {
			fmt.Println("Too many args provided. See usage:")
			flag.PrintDefaults()
			return 1
		}
		app.LaunchTUI()

	case flag.NFlag() > 1:
		fmt.Printf("Too many flags provided. Use %s --help for more info.", os.Args[0])
		return 1

	case *help:
		flag.PrintDefaults()

	case *listen:
		app.LaunchListener()
	}

	// Successfully exit
	return 0
}

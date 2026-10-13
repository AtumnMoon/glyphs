package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/AtumnMoon/glyphs/internal/app"
	"github.com/AtumnMoon/glyphs/internal/clipboard"
	"github.com/AtumnMoon/glyphs/internal/config"
)

var version = "v0.1.0"

var (
	help          = flag.Bool("help", false, "Show this help message.")
	showVersion   = flag.Bool("v", false, "Show app version.")
	listen        = flag.Bool("listen", false, "Start a background process that records clipboard activity (replaces any running one).")
	listenShell   = flag.Bool("listen-shell", false, "Run the clipboard listener in the current terminal (useful for debugging).")
	kill          = flag.Bool("kill", false, "Stop the background listener.")
	pause         = flag.String("pause", "", "Pause recording for a duration, e.g. 30s, 5m, 1h. Use \"off\" to resume.")
	add           = flag.Bool("a", false, "Add the following argument (or piped stdin) to the history without copying it.")
	copyInput     = flag.Bool("c", false, "Copy the following argument (or piped stdin) to the system clipboard.")
	paste         = flag.Bool("p", false, "Print the current clipboard text.")
	clearUnpinned = flag.Bool("clear", false, "Remove all history except pinned entries.")
	clean         = flag.Bool("clean", false, "Sanitise text entries and remove orphaned image entries/files.")
	clearAll      = flag.Bool("clear-all", false, "Remove the entire history, pinned entries included.")
	clearImages   = flag.Bool("clear-images", false, "Remove all images from the history, pinned included.")
	clearText     = flag.Bool("clear-text", false, "Remove all text entries from the history, pinned included.")
	outputAll     = flag.String("output-all", "", "Print every text entry, newest first. Value: raw (one line per entry, newlines escaped) or unescaped.")
	hold          = flag.String("hold", "", "Internal: own the clipboard with stdin data (text|image) until replaced.")
	keep          = flag.Bool("keep", false, "Keep the TUI open after copying an item.")
	tab           = flag.String("tab", "", "Open the TUI on a tab: kaomoji, emoji or clipboard (default: config defaultTab).")
)

// modifiers don't select a command by themselves.
var modifiers = map[string]bool{"tab": true, "keep": true}

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), "glyphs %s - kaomoji, emoji and clipboard history in your terminal\n\nUsage:\n  glyphs [-tab kaomoji|emoji|clipboard]   open the TUI\n  glyphs <command> [args]\n\nCommands and options:\n", version)
	flag.PrintDefaults()
}

// Main runs the CLI and returns the process exit code.
func Main() int {
	flag.Usage = usage
	flag.Parse()

	var set []string
	flag.Visit(func(f *flag.Flag) {
		if !modifiers[f.Name] {
			set = append(set, f.Name)
		}
	})

	switch {
	case len(set) > 1:
		fmt.Fprintf(os.Stderr, "Too many flags provided (%s). Use %s -help for usage.\n", strings.Join(set, ", "), os.Args[0])
		return 1
	case *help:
		usage()
		return 0
	case *showVersion:
		fmt.Println(os.Args[0], version)
		return 0
	}

	cfg, err := config.Load()
	if err != nil {
		// Parse problems fall back to defaults; keep going but say so.
		fmt.Fprintln(os.Stderr, "warning:", err)
	}

	switch {
	case len(set) == 0:
		if !*add && !*copyInput && flag.NArg() > 0 {
			fmt.Fprintln(os.Stderr, "Unexpected arguments. See usage:")
			usage()
			return 1
		}

		if *keep {
			cfg.QuitAfterCopy = false
		}
		return app.LaunchTUI(cfg, *tab)

	case *add:
		return app.Add(cfg, flag.Args())
	case *copyInput:
		return app.Copy(cfg, flag.Args())
	case *paste:
		return app.Paste(cfg)
	case *listen:
		return app.LaunchListener(cfg)
	case *listenShell:
		_, _ = app.Kill(cfg)
		return app.RunListener(cfg)
	case *kill:
		stopped, err := app.Kill(cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		if stopped {
			fmt.Println("listener stopped")
		} else {
			fmt.Println("no listener is running")
		}
		return 0
	case *pause != "":
		return handlePause(cfg, *pause)
	case *clearUnpinned:
		return app.Clear(cfg, clipboard.ClearUnpinned)
	case *clearAll:
		return app.Clear(cfg, clipboard.ClearAll)
	case *clearImages:
		return app.Clear(cfg, clipboard.ClearImages)
	case *clearText:
		return app.Clear(cfg, clipboard.ClearText)
	case *hold != "":
		return app.RunHolder(*hold)
	case *clean:
		return app.Clean(cfg)
	case *outputAll != "":
		if *outputAll != "raw" && *outputAll != "unescaped" {
			fmt.Fprintln(os.Stderr, "-output-all takes raw or unescaped")
			return 1
		}
		return app.OutputAll(cfg, *outputAll)
	}

	fmt.Fprintf(os.Stderr, "Command not recognised. See %s -help.\n", os.Args[0])
	return 1
}

func handlePause(cfg config.Config, arg string) int {
	if a := strings.ToLower(arg); a == "off" || a == "resume" {
		if err := app.Resume(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Println("recording resumed")
		return 0
	}
	d, err := app.ParseDuration(arg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid duration %q: %v\nExample: %s -pause 5m   (or \"-pause off\" to resume)\n", arg, err, os.Args[0])
		return 1
	}
	if !app.IsListenerRunning(cfg) {
		fmt.Fprintln(os.Stderr, "note: no listener is running, so there is nothing to pause yet (start one with -listen)")
	}
	if err := app.Pause(cfg, d); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Printf("recording paused for %s\n", d)
	return 0
}

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AtumnMoon/glyphs/internal/config"
	"github.com/AtumnMoon/glyphs/internal/data"
	"github.com/AtumnMoon/glyphs/internal/ui"
)

// LaunchTUI opens the interactive UI. startTab may be "" (use the configured
// default), "kaomoji", "emoji" or "clipboard".
func LaunchTUI(cfg config.Config, startTab string) int {
	switch strings.ToLower(startTab) {
	case "", "kaomoji", "emoji", "clipboard":
	default:
		fmt.Fprintf(os.Stderr, "Unknown tab %q (use kaomoji, emoji or clipboard)\n", startTab)
		return 1
	}
	kao, err := data.NewKaomojiStore("")
	if err != nil {
		return fail(err)
	}
	emo, err := data.NewEmojiStore("")
	if err != nil {
		return fail(err)
	}
	hist, err := NewStore(cfg)
	if err != nil {
		return fail(err)
	}
	if _, err := hist.Expire(cfg.DeleteAfter); err != nil {
		return fail(err)
	}

	historyFile := filepath.Join(cfg.Dir, "clipboard_history.json")
	model := ui.New(ui.Deps{
		Config:     cfg,
		Kaomoji:    kao,
		Emoji:      emo,
		History:    hist,
		WriteText:  PersistText,
		WriteImage: PersistImage,
		ModTime: func() time.Time {
			if fi, err := os.Stat(historyFile); err == nil {
				return fi.ModTime()
			}
			return time.Time{}
		},
		ListenerRunning: func() bool { return IsListenerRunning(cfg) },
		Paused:          func() bool { return IsPaused(cfg) },
	}, startTab)

	if _, err := tea.NewProgram(model).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Could not start program :(\n%v\n", err)
		return 1
	}
	return 0
}

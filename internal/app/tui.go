package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AtumnMoon/glyphs/internal/config"
	"github.com/AtumnMoon/glyphs/internal/data"
	"github.com/AtumnMoon/glyphs/internal/logx"
	"github.com/AtumnMoon/glyphs/internal/ui"
)

// LaunchTUI opens the interactive UI. startTab may be "" (use the configured
// default) or a tab key (clipboard, kaomoji, emoji, logs). cfgErr is the error
// config.Load returned, if any: the UI runs on the defaults and reports it.
func LaunchTUI(cfg config.Config, startTab string, cfgErr error) int {
	if startTab != "" && !slices.Contains(config.TabKeys, strings.ToLower(startTab)) {
		fmt.Fprintf(os.Stderr, "Unknown tab %q (use %s)\n", startTab, strings.Join(config.TabKeys, ", "))
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

	// Only one TUI at a time: a new one replaces any that is already open.
	release, err := claimTUI(cfg)
	if err != nil {
		return fail(err)
	}
	defer release()

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
		Logs:            func(n int) ([]logx.Entry, error) { return logx.Read(cfg.LogFilePath, n) },
		LogModTime: func() time.Time {
			if fi, err := os.Stat(cfg.LogFilePath); err == nil {
				return fi.ModTime()
			}
			return time.Time{}
		},
		Problems: configProblems(cfg, cfgErr, time.Now()),
	}, startTab)

	if _, err := tea.NewProgram(model).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Could not start program :(\n%v\n", err)
		return 1
	}
	return 0
}

// configProblems turns what config.Load reported into Logs-tab entries: the
// error first (the config in use is the defaults, which is why it matters most),
// then the warnings.
func configProblems(cfg config.Config, cfgErr error, now time.Time) []logx.Entry {
	var out []logx.Entry
	if cfgErr != nil {
		out = append(out, logx.Entry{Time: now, Level: logx.Error,
			Msg: cfgErr.Error() + " (running on the defaults; the listener will not start until this is fixed)"})
	}
	for _, w := range cfg.Warnings {
		out = append(out, logx.Entry{Time: now, Level: logx.Warn, Msg: "config: " + w})
	}
	return out
}

// claimTUI makes this process the only TUI: any other running TUI is asked to
// quit first. The returned function gives up the claim on exit.
func claimTUI(cfg config.Config) (release func(), err error) {
	inst, err := acquire(cfg.TUILockFile(), cfg.TUIPIDFile(), "glyphs TUI", true, 3*time.Second)
	if err != nil {
		return nil, err
	}
	return inst.release, nil
}

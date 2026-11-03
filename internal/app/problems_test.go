package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AtumnMoon/glyphs/internal/config"
	"github.com/AtumnMoon/glyphs/internal/logx"
)

func TestConfigProblems(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

	if got := configProblems(config.Config{}, nil, now); len(got) != 0 {
		t.Errorf("a clean config has no problems, got %+v", got)
	}

	cfg := config.Config{Warnings: []string{`unknown option "x" (ignored)`, "default_tab is wrong"}}
	cfgErr := &config.Error{Path: "/c/config.toml", Line: 3, Col: 27, Msg: "expected a comma"}
	got := configProblems(cfg, cfgErr, now)
	if len(got) != 3 {
		t.Fatalf("want the error and two warnings, got %+v", got)
	}
	if got[0].Level != logx.Error || !strings.Contains(got[0].Msg, "config.toml:3:27") || !strings.Contains(got[0].Msg, "defaults") {
		t.Errorf("the error comes first and says what is running instead: %+v", got[0])
	}
	for _, w := range got[1:] {
		if w.Level != logx.Warn || !strings.HasPrefix(w.Msg, "config: ") || !w.Time.Equal(now) {
			t.Errorf("warning entry = %+v", w)
		}
	}

	// A plain error (not *config.Error) still shows up.
	if got := configProblems(config.Config{}, errors.New("config: read x: permission denied"), now); len(got) != 1 || got[0].Level != logx.Error {
		t.Errorf("got %+v", got)
	}
}

func TestLogErrorWritesToTheLogFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	LogError(cfg, "listener refused to start: %v", "bad config")
	got, err := logx.Read(cfg.LogFilePath, 5)
	if err != nil || len(got) != 1 || got[0].Level != logx.Error || !strings.Contains(got[0].Msg, "bad config") {
		t.Errorf("got %+v, %v", got, err)
	}
}

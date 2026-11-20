package app

import (
	"fmt"
	"io"
	"os"
	"strings"

	gclip "golang.design/x/clipboard"

	"github.com/AtumnMoon/glyphs/internal/clipboard"
	"github.com/AtumnMoon/glyphs/internal/config"
)

func inputFrom(args []string) (string, error) {
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}
	if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice == 0 {
		b, err := io.ReadAll(os.Stdin)
		return string(b), err
	}
	return "", fmt.Errorf("no input: pass an argument or pipe data on stdin")
}

// Add records input in the history without touching the system clipboard. It
// reports whether anything was recorded (false: empty or excluded).
func Add(cfg config.Config, args []string) (bool, error) {
	text, err := inputFrom(args)
	if err != nil {
		return false, err
	}
	store, err := NewStore(cfg)
	if err != nil {
		return false, err
	}
	policy, err := Policy(cfg)
	if err != nil {
		return false, configErr(err)
	}
	_, ok, err := store.RecordText(text, policy)
	return ok, err
}

// Copy puts input on the system clipboard (a running listener records it).
func Copy(cfg config.Config, args []string) error {
	text, err := inputFrom(args)
	if err != nil {
		return err
	}
	return PersistText(text)
}

// Paste returns the current clipboard text.
func Paste(cfg config.Config) (string, error) {
	if err := initClipboard(); err != nil {
		return "", err
	}
	return string(gclip.Read(gclip.FmtText)), nil
}

// Clear removes history entries by scope and returns how many it removed.
func Clear(cfg config.Config, scope clipboard.ClearScope) (int, error) {
	store, err := NewStore(cfg)
	if err != nil {
		return 0, err
	}
	return store.ClearScoped(scope)
}

// Clean sanitises text entries and removes orphaned images. It returns the
// number of entries and of orphaned files removed.
func Clean(cfg config.Config) (entries, files int, err error) {
	store, err := NewStore(cfg)
	if err != nil {
		return 0, 0, err
	}
	return store.Clean()
}

// OutputAll writes every text entry to w, newest first. mode "unescaped"
// writes entries verbatim separated by newlines; anything else ("raw")
// escapes newlines inside entries so each entry is exactly one line.
func OutputAll(w io.Writer, cfg config.Config, mode string) error {
	store, err := NewStore(cfg)
	if err != nil {
		return err
	}
	entries, err := store.All()
	if err != nil {
		return err
	}
	esc := strings.NewReplacer("\r", "\\r", "\n", "\\n")
	for _, e := range entries {
		if e.IsImage() {
			continue
		}
		v := e.Value
		if mode != "unescaped" {
			v = esc.Replace(v)
		}
		if _, err := fmt.Fprintln(w, v); err != nil {
			return err
		}
	}
	return nil
}

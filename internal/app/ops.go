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

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "error:", err)
	return 1
}

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

// Add records input in the history without touching the system clipboard.
func Add(cfg config.Config, args []string) int {
	text, err := inputFrom(args)
	if err != nil {
		return fail(err)
	}
	store, err := NewStore(cfg)
	if err != nil {
		return fail(err)
	}
	policy, err := Policy(cfg)
	if err != nil {
		return fail(err)
	}
	if _, ok, err := store.RecordText(text, policy); err != nil {
		return fail(err)
	} else if !ok {
		fmt.Fprintln(os.Stderr, "nothing added (empty or excluded)")
	}
	return 0
}

// Copy puts input on the system clipboard (a running listener records it).
func Copy(cfg config.Config, args []string) int {
	text, err := inputFrom(args)
	if err != nil {
		return fail(err)
	}
	if err := PersistText(text); err != nil {
		return fail(err)
	}
	return 0
}

// Paste prints the current clipboard text.
func Paste(cfg config.Config) int {
	if err := initClipboard(); err != nil {
		return fail(err)
	}
	fmt.Print(string(gclip.Read(gclip.FmtText)))
	return 0
}

// Clear removes history entries by scope.
func Clear(cfg config.Config, scope clipboard.ClearScope) int {
	store, err := NewStore(cfg)
	if err != nil {
		return fail(err)
	}
	n, err := store.ClearScoped(scope)
	if err != nil {
		return fail(err)
	}
	fmt.Printf("removed %d entries\n", n)
	return 0
}

// Clean sanitises text entries and removes orphaned images.
func Clean(cfg config.Config) int {
	store, err := NewStore(cfg)
	if err != nil {
		return fail(err)
	}
	e, f, err := store.Clean()
	if err != nil {
		return fail(err)
	}
	fmt.Printf("removed %d entries, %d orphaned files\n", e, f)
	return 0
}

// OutputAll prints every text entry, newest first. mode "unescaped" prints
// entries verbatim separated by newlines; anything else ("raw") escapes
// newlines inside entries so each entry is exactly one line.
func OutputAll(cfg config.Config, mode string) int {
	store, err := NewStore(cfg)
	if err != nil {
		return fail(err)
	}
	entries, err := store.All()
	if err != nil {
		return fail(err)
	}
	esc := strings.NewReplacer("\r", "\\r", "\n", "\\n")
	for _, e := range entries {
		if e.IsImage() {
			continue
		}
		if mode == "unescaped" {
			fmt.Println(e.Value)
		} else {
			fmt.Println(esc.Replace(e.Value))
		}
	}
	return 0
}

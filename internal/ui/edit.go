package ui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/AtumnMoon/glyphs/internal/clipboard"
)

// editSession is a clipboard entry that is open in the user's editor.
type editSession struct {
	key  string // Key() of the entry being edited, when it was opened
	orig string // its text then
	dir  string // private temp dir, removed when the editor returns
	path string // the file the editor was given
}

// editDoneMsg arrives when the editor has exited.
type editDoneMsg struct{ err error }

// startEdit opens the highlighted entry in $VISUAL/$EDITOR. The TUI is
// suspended for as long as the editor runs (tea.ExecProcess).
//
// The text goes through a file in a fresh 0700 directory (the file is 0600):
// clipboard text is often secret. The whole directory is removed afterwards,
// which also takes care of the swap and backup files editors leave beside it.
func (m *model) startEdit() tea.Cmd {
	targets := m.clip.targets()
	switch {
	case len(targets) == 0:
		return nil
	case len(targets) > 1:
		m.setStatus("Edit works on one entry: clear the selection first (S)", true)
		return nil
	case targets[0].IsImage():
		m.setStatus("Images can't be edited", true)
		return nil
	}
	e := targets[0]

	dir, err := os.MkdirTemp("", "glyphs-edit-")
	if err != nil {
		m.setStatus("edit: "+err.Error(), true)
		return nil
	}
	path := filepath.Join(dir, "entry.txt")
	if err := os.WriteFile(path, []byte(e.Value), 0o600); err != nil {
		os.RemoveAll(dir)
		m.setStatus("edit: "+err.Error(), true)
		return nil
	}
	editor := m.deps.Editor
	if editor == nil {
		editor = defaultEditor
	}
	cmd, err := editor(path)
	if err != nil {
		os.RemoveAll(dir)
		m.setStatus(err.Error(), true)
		return nil
	}
	m.edit = &editSession{key: entryKey(e), orig: e.Value, dir: dir, path: path}
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return editDoneMsg{err} })
}

// finishEdit reads the edited text back and stores it.
func (m *model) finishEdit(runErr error) {
	s := m.edit
	if s == nil {
		return
	}
	m.edit = nil
	defer os.RemoveAll(s.dir)

	if runErr != nil {
		m.setStatus("editor failed, entry unchanged: "+runErr.Error(), true)
		return
	}
	b, err := os.ReadFile(s.path)
	if err != nil {
		m.setStatus("edit: "+err.Error(), true)
		return
	}
	text := editedText(string(b), s.orig)
	switch {
	case text == s.orig:
		m.setStatus("Unchanged", false)
		return
	case strings.TrimSpace(text) == "":
		m.setStatus("Empty text: edit discarded", false)
		return
	}
	updated, err := m.deps.History.EditText(s.key, text, !m.cfg.AllowDuplicates)
	switch {
	case errors.Is(err, clipboard.ErrNoEntry):
		m.setStatus("The entry was removed while you were editing it", true)
		return
	case err != nil:
		m.setStatus("edit: "+err.Error(), true)
		return
	}
	m.clip.landKey = updated.Key() // keep the cursor on it
	m.reloadClip(true)
	m.syncMode() // a selection that held the old key is gone
	m.setStatus("Entry updated", false)
}

// editedText undoes the newline editors add: most write a final "\n" even if
// the text had none, and an entry is not supposed to gain one by being edited.
func editedText(got, orig string) string {
	if strings.HasSuffix(orig, "\n") {
		return got
	}
	if t, ok := strings.CutSuffix(got, "\r\n"); ok {
		return t
	}
	return strings.TrimSuffix(got, "\n")
}

// defaultEditor builds the command for $VISUAL, then $EDITOR, then vi. The
// variable is split on spaces ("code --wait" works); quoting is not supported,
// so point it at a small script for anything fancier.
func defaultEditor(path string) (*exec.Cmd, error) {
	var fields []string
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if v := strings.Fields(os.Getenv(env)); len(v) > 0 {
			fields = v
			break
		}
	}
	if len(fields) == 0 {
		fields = []string{"vi"}
	}
	if _, err := exec.LookPath(fields[0]); err != nil {
		return nil, fmt.Errorf("editor %q not found: set $VISUAL or $EDITOR", fields[0])
	}
	return exec.Command(fields[0], append(fields[1:], path)...), nil
}

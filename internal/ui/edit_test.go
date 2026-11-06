package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AtumnMoon/glyphs/internal/clipboard"
	"github.com/AtumnMoon/glyphs/internal/config"
)

// editor is a stand-in for $EDITOR: it records the file it was given and lets
// the test play the user by writing to it.
type editor struct {
	path string
	err  error
}

func (e *editor) command(path string) (*exec.Cmd, error) {
	e.path = path
	if e.err != nil {
		return nil, e.err
	}
	return exec.Command("true"), nil
}

func editHarness(t *testing.T, mut func(*config.Config)) (*harness, *editor) {
	t.Helper()
	ed := &editor{}
	h := newHarnessWith(t, "clipboard", mut, func(d *Deps) { d.Editor = ed.command })
	seed(t, h, "a", "b", "c") // c b a, cursor on c
	return h, ed
}

// finishWith plays the editor's side: write text to the file, then report that
// the editor exited.
func finishWith(t *testing.T, h *harness, ed *editor, text string) {
	t.Helper()
	if h.model().edit == nil {
		t.Fatal("no edit session is open")
	}
	if err := os.WriteFile(ed.path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	h.send(editDoneMsg{})
}

func TestEditReplacesTheTextInPlace(t *testing.T) {
	h, ed := editHarness(t, nil)
	h.press("down") // b
	cmd := h.send(keyMsg("e"))
	if cmd == nil || h.model().edit == nil {
		t.Fatal("e should suspend the UI and open the editor")
	}
	if b, _ := os.ReadFile(ed.path); string(b) != "b" {
		t.Errorf("the editor must be handed the entry's text, got %q", b)
	}
	dir := filepath.Dir(ed.path)

	finishWith(t, h, ed, "bee\n") // editors append a newline
	wantOrder(t, "file", stored(t, h), "c", "bee", "a")
	if cur, _ := h.model().clip.current(); cur.Value != "bee" {
		t.Errorf("the cursor should stay on the edited entry, is on %q", cur.Value)
	}
	if h.model().status != "Entry updated" {
		t.Errorf("status = %q", h.model().status)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("the temp directory must be removed afterwards")
	}
}

func TestEditKeepsMultilineTextAndInnerNewlines(t *testing.T) {
	h, ed := editHarness(t, nil)
	h.send(keyMsg("e"))
	finishWith(t, h, ed, "line one\nline two\n")
	wantOrder(t, "file", stored(t, h), "line one\nline two", "b", "a")
}

func TestEditedTextNewlineRules(t *testing.T) {
	for _, tc := range []struct{ got, orig, want string }{
		{"x\n", "x", "x"},
		{"x\r\n", "x", "x"},
		{"x", "x", "x"},
		{"x\n\n", "x", "x\n"}, // only the editor's own newline goes
		{"x\n", "x\n", "x\n"}, // the original had one: leave it
	} {
		if got := editedText(tc.got, tc.orig); got != tc.want {
			t.Errorf("editedText(%q, %q) = %q, want %q", tc.got, tc.orig, got, tc.want)
		}
	}
}

func TestEditTempFileIsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permissions")
	}
	h, ed := editHarness(t, nil)
	h.send(keyMsg("e"))
	fi, err := os.Stat(ed.path)
	if err != nil {
		t.Fatal(err)
	}
	di, _ := os.Stat(filepath.Dir(ed.path))
	if fi.Mode().Perm() != 0o600 || di.Mode().Perm() != 0o700 {
		t.Errorf("clipboard text can be secret: file %v, dir %v", fi.Mode().Perm(), di.Mode().Perm())
	}
	finishWith(t, h, ed, "c")
}

func TestEditUnchangedAndEmptyChangeNothing(t *testing.T) {
	h, ed := editHarness(t, nil)
	h.send(keyMsg("e"))
	finishWith(t, h, ed, "c\n")
	if h.model().status != "Unchanged" {
		t.Errorf("status = %q", h.model().status)
	}
	h.send(keyMsg("e"))
	finishWith(t, h, ed, " \n\n")
	if !strings.Contains(h.model().status, "discarded") {
		t.Errorf("status = %q", h.model().status)
	}
	wantOrder(t, "file", stored(t, h), "c", "b", "a")
}

func TestEditorFailureLeavesTheEntryAndCleansUp(t *testing.T) {
	h, ed := editHarness(t, nil)
	h.send(keyMsg("e"))
	dir := filepath.Dir(ed.path)
	os.WriteFile(ed.path, []byte("half-edited"), 0o600)
	h.send(editDoneMsg{err: os.ErrInvalid})
	wantOrder(t, "file", stored(t, h), "c", "b", "a")
	if !h.model().statusErr || !strings.Contains(h.model().status, "unchanged") {
		t.Errorf("status = %q (err=%v)", h.model().status, h.model().statusErr)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("the temp directory must be removed even when the editor failed")
	}
}

func TestEditNeedsExactlyOneTextEntry(t *testing.T) {
	h, _ := editHarness(t, nil)
	h.press("v", "down", "esc") // two selected
	if cmd := h.send(keyMsg("e")); cmd != nil || h.model().edit != nil {
		t.Error("editing a multi-selection must not start")
	}
	if !strings.Contains(h.model().status, "one entry") {
		t.Errorf("status = %q", h.model().status)
	}

	h.press("S")
	h.hist.RecordImage([]byte("png-bytes"), clipboardPolicy())
	h.model().reloadClip(true)
	h.press("home")
	if cmd := h.send(keyMsg("e")); cmd != nil || h.model().edit != nil {
		t.Error("an image can't be edited")
	}
	if !strings.Contains(h.model().status, "Images") {
		t.Errorf("status = %q", h.model().status)
	}
}

func TestEditWorksOnASingleSelectedEntryInSelectMode(t *testing.T) {
	h, ed := editHarness(t, nil)
	h.press("down", "v") // b selected, still in select mode
	if cmd := h.send(keyMsg("e")); cmd == nil {
		t.Fatal("one selected entry should be editable")
	}
	finishWith(t, h, ed, "bee")
	wantOrder(t, "file", stored(t, h), "c", "bee", "a")
	if h.model().mode != modeNormal {
		t.Errorf("the selection held the old key, so the mode should end (mode=%d)", h.model().mode)
	}
}

func TestEditIsIgnoredInMoveMode(t *testing.T) {
	h, _ := editHarness(t, nil)
	h.press("m")
	if cmd := h.send(keyMsg("e")); cmd != nil || h.model().edit != nil {
		t.Error("e must do nothing in move mode")
	}
}

func TestEditWhenTheEntryVanishedMeanwhile(t *testing.T) {
	h, ed := editHarness(t, nil)
	h.send(keyMsg("e"))
	if err := h.hist.Delete(0); err != nil { // the entry being edited
		t.Fatal(err)
	}
	finishWith(t, h, ed, "new text")
	if !strings.Contains(h.model().status, "removed") {
		t.Errorf("status = %q", h.model().status)
	}
	wantOrder(t, "file", stored(t, h), "b", "a")
}

func TestEditMergesWithAnExistingDuplicate(t *testing.T) {
	h, ed := editHarness(t, nil)
	h.send(keyMsg("e"))
	finishWith(t, h, ed, "a") // c becomes "a", which already exists
	wantOrder(t, "file", stored(t, h), "a", "b")

	h2, ed2 := editHarness(t, func(c *config.Config) { c.AllowDuplicates = true })
	h2.send(keyMsg("e"))
	finishWith(t, h2, ed2, "a")
	wantOrder(t, "allow_duplicates keeps both", stored(t, h2), "a", "b", "a")
}

func TestEditorStartFailureIsReported(t *testing.T) {
	h, ed := editHarness(t, nil)
	ed.err = os.ErrNotExist
	if cmd := h.send(keyMsg("e")); cmd != nil || h.model().edit != nil {
		t.Error("no editor, no session")
	}
	if !h.model().statusErr {
		t.Error("the failure should be shown")
	}
}

// One test runs a real process through the same command shape the app uses.
func TestEditThroughARealEditorProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs sh")
	}
	var ed *exec.Cmd
	h := newHarnessWith(t, "clipboard", nil, func(d *Deps) {
		d.Editor = func(path string) (*exec.Cmd, error) {
			ed = exec.Command("sh", "-c", `printf '%s\n' "$(cat "$1")-edited" > "$1"`, "sh", path)
			return ed, nil
		}
	})
	seed(t, h, "hello")
	h.send(keyMsg("e"))
	if err := ed.Run(); err != nil {
		t.Fatal(err)
	}
	h.send(editDoneMsg{})
	wantOrder(t, "file", stored(t, h), "hello-edited")
}

func TestDefaultEditorResolution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs echo")
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "echo --wait")
	cmd, err := defaultEditor("/tmp/f")
	if err != nil {
		t.Fatal(err)
	}
	if got := cmd.Args[1:]; len(got) != 2 || got[0] != "--wait" || got[1] != "/tmp/f" {
		t.Errorf("arguments in $EDITOR must be kept and the path appended: %v", cmd.Args)
	}

	t.Setenv("VISUAL", "echo visual")
	if cmd, _ := defaultEditor("/tmp/f"); cmd == nil || cmd.Args[1] != "visual" {
		t.Errorf("$VISUAL wins over $EDITOR: %v", cmd)
	}

	t.Setenv("VISUAL", "no-such-editor-xyz --flag")
	if _, err := defaultEditor("/tmp/f"); err == nil || !strings.Contains(err.Error(), "$VISUAL or $EDITOR") {
		t.Errorf("a missing editor needs a helpful error: %v", err)
	}
}

func clipboardPolicy() clipboard.Policy { return clipboard.Policy{} }

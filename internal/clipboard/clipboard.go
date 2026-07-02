package clipboard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.design/x/clipboard"
)

// ClipboardEntry mirrors a single clipboard_history.json record.
type ClipboardEntry struct {
	Value    string `json:"value"`
	Recorded string `json:"recorded"` // "2006-01-02 15:04:05" style
	FilePath string `json:"filePath"` // "null" string when there's no file
	Pinned   bool   `json:"pinned"`
}

// history is the top-level document shape: {"clipboardHistory": [...]}.
type history struct {
	ClipboardHistory []ClipboardEntry `json:"clipboardHistory"`
}

// HistoryStore manages a clipboard_history.json file plus an image dir, guarded by
// a mutex since the listener and any UI reading it may run concurrently
// within the same process.
type HistoryStore struct {
	mu          sync.Mutex
	historyPath string
	imageDir    string
	maxItems    int
}

// NewHistory creates a Store rooted at configDir (e.g. "~/.config/..."),
// creating the directory structure if needed. maxItems <= 0 means unlimited.
func NewHistory(configDir string, maxItems int) (*HistoryStore, error) {
	imageDir := filepath.Join(configDir, "tmp_files")
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		return nil, fmt.Errorf("clipboardhistory: create dirs: %w", err)
	}
	return &HistoryStore{
		historyPath: filepath.Join(configDir, "clipboard_history.json"),
		imageDir:    imageDir,
		maxItems:    maxItems,
	}, nil
}

// recordedNow formats a timestamp: "2006-01-02 15:04:05".
func recordedNow() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// loadHistory reads the history file, returning an empty history if it doesn't exist yet.
func (s *HistoryStore) loadHistory() (history, error) {
	var h history
	b, err := os.ReadFile(s.historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return h, nil
		}
		return h, err
	}
	if len(b) == 0 {
		return h, nil
	}
	if err := json.Unmarshal(b, &h); err != nil {
		return h, err
	}
	return h, nil
}

// saveHistory writes the history file atomically (write to temp file, then rename)
// to avoid leaving a half-written JSON file behind if the process dies mid-write.
func (s *HistoryStore) saveHistory(h history) error {
	b, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.historyPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.historyPath)
}

// AddText appends a text entry to the history.
func (s *HistoryStore) AddText(value string) (ClipboardEntry, error) {
	entry := ClipboardEntry{
		Value:    value,
		Recorded: recordedNow(),
		FilePath: "null",
		Pinned:   false,
	}
	return entry, s.append(entry)
}

// AddImage writes img bytes to the tmp_files dir and records an entry
// pointing at that file. value is whatever short label/preview text you
// want stored alongside it (clipse uses the file path itself, but you can
// put a description here if you prefer).
func (s *HistoryStore) AddImage(img []byte) (ClipboardEntry, error) {
	fname := fmt.Sprintf("%d.png", time.Now().UnixNano())
	imgPath := filepath.Join(s.imageDir, fname)
	if err := os.WriteFile(imgPath, img, 0o644); err != nil {
		return ClipboardEntry{}, fmt.Errorf("clipboardhistory: write image: %w", err)
	}
	entry := ClipboardEntry{
		Value:    imgPath,
		Recorded: recordedNow(),
		FilePath: imgPath,
		Pinned:   false,
	}
	return entry, s.append(entry)
}

// append adds an entry, trims unpinned overflow past maxItems, and saves.
func (s *HistoryStore) append(entry ClipboardEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	h, err := s.loadHistory()
	if err != nil {
		return err
	}

	// newest first, matching the screenshot's ordering
	h.ClipboardHistory = append([]ClipboardEntry{entry}, h.ClipboardHistory...)

	if s.maxItems > 0 {
		h.ClipboardHistory = trimUnpinned(h.ClipboardHistory, s.maxItems)
	}

	return s.saveHistory(h)
}

// trimUnpinned keeps all pinned entries plus the most recent unpinned ones,
// until the total is at most max.
func trimUnpinned(entries []ClipboardEntry, max int) []ClipboardEntry {
	if len(entries) <= max {
		return entries
	}
	var pinned, unpinned []ClipboardEntry
	for _, e := range entries {
		if e.Pinned {
			pinned = append(pinned, e)
		} else {
			unpinned = append(unpinned, e)
		}
	}
	room := max - len(pinned)
	if room < 0 {
		room = 0
	}
	if room > len(unpinned) {
		room = len(unpinned)
	}
	kept := append(pinned, unpinned[:room]...)
	return kept
}

// All returns the full history, newest first.
func (s *HistoryStore) All() ([]ClipboardEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, err := s.loadHistory()
	return h.ClipboardHistory, err
}

// Pin sets pinned=true/false on the entry at index i (as returned by All).
func (s *HistoryStore) Pin(i int, pinned bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, err := s.loadHistory()
	if err != nil {
		return err
	}
	if i < 0 || i >= len(h.ClipboardHistory) {
		return fmt.Errorf("clipboardhistory: index %d out of range", i)
	}
	h.ClipboardHistory[i].Pinned = pinned
	return s.saveHistory(h)
}

// Delete removes the entry at index i, and its backing image file if it has one.
func (s *HistoryStore) Delete(i int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, err := s.loadHistory()
	if err != nil {
		return err
	}
	if i < 0 || i >= len(h.ClipboardHistory) {
		return fmt.Errorf("clipboardhistory: index %d out of range", i)
	}
	entry := h.ClipboardHistory[i]
	if entry.FilePath != "null" && entry.FilePath != "" {
		_ = os.Remove(entry.FilePath) // best-effort; don't fail the delete over it
	}
	h.ClipboardHistory = append(h.ClipboardHistory[:i], h.ClipboardHistory[i+1:]...)
	return s.saveHistory(h)
}

// Clear wipes all unpinned entries (and their image files), keeping pinned ones.
func (s *HistoryStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, err := s.loadHistory()
	if err != nil {
		return err
	}
	var kept []ClipboardEntry
	for _, e := range h.ClipboardHistory {
		if e.Pinned {
			kept = append(kept, e)
			continue
		}
		if e.FilePath != "null" && e.FilePath != "" {
			_ = os.Remove(e.FilePath)
		}
	}
	h.ClipboardHistory = kept
	return s.saveHistory(h)
}

// Clean sanitizes text entries and removes orphaned image files, mirroring
//   - strips null bytes and trims leading/trailing whitespace from text values
//   - drops entries left pointing at an image file that no longer exists
//   - deletes any file in tmp_files/ that isn't referenced by a remaining entry
func (s *HistoryStore) Clean() (removedEntries, removedFiles int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	h, err := s.loadHistory()
	if err != nil {
		return 0, 0, err
	}

	referenced := make(map[string]bool)
	kept := h.ClipboardHistory[:0]
	for _, e := range h.ClipboardHistory {
		if e.FilePath != "null" && e.FilePath != "" {
			if _, statErr := os.Stat(e.FilePath); statErr != nil {
				// image entry with a missing backing file: drop it (orphaned entry)
				removedEntries++
				continue
			}
			referenced[e.FilePath] = true
		} else {
			e.Value = sanitizeText(e.Value)
		}
		kept = append(kept, e)
	}
	h.ClipboardHistory = kept

	// remove any files in imageDir not referenced by a surviving entry
	files, readErr := os.ReadDir(s.imageDir)
	if readErr == nil {
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			full := filepath.Join(s.imageDir, f.Name())
			if !referenced[full] {
				if rmErr := os.Remove(full); rmErr == nil {
					removedFiles++
				}
			}
		}
	}

	if err := s.saveHistory(h); err != nil {
		return removedEntries, removedFiles, err
	}
	return removedEntries, removedFiles, nil
}

// sanitizeText strips null bytes and trims surrounding whitespace, which is
// the kind of junk that sometimes sneaks into clipboard text from browsers
// or terminal apps.
func sanitizeText(s string) string {
	b := make([]rune, 0, len(s))
	for _, r := range s {
		if r == 0 {
			continue
		}
		b = append(b, r)
	}
	return strings.TrimSpace(string(b))
}

// Listen blocks, watching the system clipboard and recording every text/image
// change into the store until ctx is done (pass context.Background() to run
// forever). It's meant to be run the same way clipse runs `clipse -listen`.
func (s *HistoryStore) Listen(watch <-chan clipboard.Data) {
	for data := range watch {
		switch data.Format {
		case clipboard.FmtText:
			if _, err := s.AddText(string(data.Bytes)); err != nil {
				fmt.Fprintln(os.Stderr, "clipboardhistory: add text:", err)
			}
		case clipboard.FmtImage:
			if _, err := s.AddImage(data.Bytes); err != nil {
				fmt.Fprintln(os.Stderr, "clipboardhistory: add image:", err)
			}
		}
	}
}

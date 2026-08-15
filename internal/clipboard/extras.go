package clipboard

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

	gclip "golang.design/x/clipboard"
)

// Policy controls how RecordText/RecordImage/ListenWith treat new data.
type Policy struct {
	AllowDuplicates bool             // false: a repeat moves the old entry to the top
	DeleteAfter     int              // seconds; unpinned entries older than this are purged (0 = never)
	Exclude         []*regexp.Regexp // matching text is never recorded
}

// CompilePatterns compiles exclusion regexps, reporting the first bad one.
func CompilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("bad excludedPatterns entry %q: %w", p, err)
		}
		out = append(out, re)
	}
	return out, nil
}

// IsImage reports whether the entry points at an image file.
func (e ClipboardEntry) IsImage() bool { return e.FilePath != "null" && e.FilePath != "" }

// Key identifies an entry across reloads of the history file.
func (e ClipboardEntry) Key() string { return e.Recorded + "\x00" + e.FilePath + "\x00" + e.Value }

func (p Policy) excluded(text string) bool {
	for _, re := range p.Exclude {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

// RecordText records text per policy. Blank or excluded text is skipped
// (ok=false). A repeated value moves to the top and keeps its pinned state.
func (s *HistoryStore) RecordText(value string, p Policy) (entry ClipboardEntry, ok bool, err error) {
	value = sanitizeText(value)
	if value == "" || p.excluded(value) {
		return ClipboardEntry{}, false, nil
	}
	entry = ClipboardEntry{Value: value, Recorded: recordedNow(), FilePath: "null"}
	err = s.edit(func(h *history) {
		if !p.AllowDuplicates {
			for i, e := range h.ClipboardHistory {
				if !e.IsImage() && e.Value == value {
					entry.Pinned = e.Pinned
					h.ClipboardHistory = append(h.ClipboardHistory[:i], h.ClipboardHistory[i+1:]...)
					break
				}
			}
		}
		s.prepend(h, entry)
	})
	return entry, err == nil, err
}

// RecordImage stores img under a content-hash file name (so one picture is
// never stored twice) and records it per policy.
func (s *HistoryStore) RecordImage(img []byte, p Policy) (entry ClipboardEntry, ok bool, err error) {
	if len(img) == 0 {
		return ClipboardEntry{}, false, nil
	}
	sum := sha256.Sum256(img)
	path := filepath.Join(s.imageDir, hex.EncodeToString(sum[:8])+".png")
	if _, statErr := os.Stat(path); statErr != nil {
		if err := os.WriteFile(path, img, 0o644); err != nil {
			return ClipboardEntry{}, false, fmt.Errorf("clipboardhistory: write image: %w", err)
		}
	}
	entry = ClipboardEntry{Value: path, Recorded: recordedNow(), FilePath: path}
	err = s.edit(func(h *history) {
		if !p.AllowDuplicates {
			for i, e := range h.ClipboardHistory {
				if e.IsImage() && e.FilePath == path {
					entry.Pinned = e.Pinned
					h.ClipboardHistory = append(h.ClipboardHistory[:i], h.ClipboardHistory[i+1:]...)
					break
				}
			}
		}
		s.prepend(h, entry)
	})
	return entry, err == nil, err
}

// edit loads, mutates and saves the history under the lock.
func (s *HistoryStore) edit(fn func(h *history)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, err := s.loadHistory()
	if err != nil {
		return err
	}
	fn(&h)
	return s.saveHistory(h)
}

// prepend inserts newest-first and enforces maxItems, deleting image files of
// entries that fall off the end (unless another entry still uses the file).
func (s *HistoryStore) prepend(h *history, entry ClipboardEntry) {
	before := append([]ClipboardEntry{entry}, h.ClipboardHistory...)
	after := before
	if s.maxItems > 0 {
		after = trimUnpinned(before, s.maxItems)
	}
	h.ClipboardHistory = after
	removeOrphanFiles(before, after)
}

func removeOrphanFiles(before, after []ClipboardEntry) {
	keep := map[string]bool{}
	for _, e := range after {
		if e.IsImage() {
			keep[e.FilePath] = true
		}
	}
	for _, e := range before {
		if e.IsImage() && !keep[e.FilePath] {
			_ = os.Remove(e.FilePath)
		}
	}
}

// Expire purges unpinned entries older than seconds (no-op when <= 0).
func (s *HistoryStore) Expire(seconds int) (removed int, err error) {
	if seconds <= 0 {
		return 0, nil
	}
	cutoff := time.Now().Add(-time.Duration(seconds) * time.Second)
	err = s.edit(func(h *history) {
		var kept []ClipboardEntry
		for _, e := range h.ClipboardHistory {
			t, perr := time.ParseInLocation("2006-01-02 15:04:05", e.Recorded, time.Local)
			if !e.Pinned && perr == nil && t.Before(cutoff) {
				removed++
				continue
			}
			kept = append(kept, e)
		}
		if removed > 0 {
			removeOrphanFiles(h.ClipboardHistory, kept)
			h.ClipboardHistory = kept
		}
	})
	return removed, err
}

// ClearScope selects what ClearScoped removes.
type ClearScope int

const (
	ClearUnpinned ClearScope = iota // everything except pinned entries
	ClearAll                        // everything, pinned included
	ClearImages                     // every image, pinned included
	ClearText                       // every text entry, pinned included
)

// ClearScoped removes entries by scope and returns how many were removed.
func (s *HistoryStore) ClearScoped(scope ClearScope) (n int, err error) {
	err = s.edit(func(h *history) {
		var kept []ClipboardEntry
		for _, e := range h.ClipboardHistory {
			drop := false
			switch scope {
			case ClearUnpinned:
				drop = !e.Pinned
			case ClearAll:
				drop = true
			case ClearImages:
				drop = e.IsImage()
			case ClearText:
				drop = !e.IsImage()
			}
			if drop {
				n++
			} else {
				kept = append(kept, e)
			}
		}
		removeOrphanFiles(h.ClipboardHistory, kept)
		h.ClipboardHistory = kept
	})
	return n, err
}

// ListenWith is Listen with a Policy applied: it records every Data from
// watch until the channel closes, purging expired entries as it goes.
// Problems are written to logw (may be nil) and never stop the loop.
func (s *HistoryStore) ListenWith(watch <-chan gclip.Data, p Policy, logw io.Writer) {
	logf := func(format string, a ...any) {
		if logw != nil {
			fmt.Fprintf(logw, format+"\n", a...)
		}
	}
	for d := range watch {
		var err error
		switch d.Format {
		case gclip.FmtText:
			_, _, err = s.RecordText(string(d.Bytes), p)
		case gclip.FmtImage:
			_, _, err = s.RecordImage(d.Bytes, p)
		}
		if err != nil {
			logf("record: %v", err)
		}
		if _, err := s.Expire(p.DeleteAfter); err != nil {
			logf("expire: %v", err)
		}
	}
}

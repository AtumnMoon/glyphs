package clipboard

import "errors"

var (
	// ErrNoEntry means the entry to edit is not in the history (any more).
	ErrNoEntry = errors.New("clipboardhistory: no such entry")
	// ErrNotText means the entry is an image.
	ErrNotText = errors.New("clipboardhistory: only text entries can be edited")
	// ErrEmpty means the new text is blank once sanitised.
	ErrEmpty = errors.New("clipboardhistory: the text is empty")
)

// EditText replaces the text of the entry with the given Key, in place: its
// position, pin and recorded time are kept. The text is sanitised like anything
// the listener records.
//
// With dedupe set (the allow_duplicates = false behaviour), any other text entry
// that now holds the same text is removed, and the edited entry is pinned if
// either of them was. The entry is found by key under the lock rather than by
// index, because the listener may have prepended since the caller looked.
func (s *HistoryStore) EditText(key, text string, dedupe bool) (ClipboardEntry, error) {
	text = sanitizeText(text)
	if text == "" {
		return ClipboardEntry{}, ErrEmpty
	}
	var out ClipboardEntry
	var failure error
	err := s.edit(func(h *history) {
		idx := -1
		for i, e := range h.ClipboardHistory {
			if e.Key() == key {
				idx = i
				break
			}
		}
		switch {
		case idx < 0:
			failure = ErrNoEntry
			return
		case h.ClipboardHistory[idx].IsImage():
			failure = ErrNotText
			return
		}
		h.ClipboardHistory[idx].Value = text
		if dedupe {
			kept := h.ClipboardHistory[:0:0]
			for i, e := range h.ClipboardHistory {
				if i != idx && !e.IsImage() && e.Value == text {
					h.ClipboardHistory[idx].Pinned = h.ClipboardHistory[idx].Pinned || e.Pinned
					continue
				}
				kept = append(kept, e)
			}
			h.ClipboardHistory = kept
		}
		for _, e := range h.ClipboardHistory {
			if e.Recorded == h.ClipboardHistory[idx].Recorded && e.Value == text && !e.IsImage() {
				out = e
				break
			}
		}
	})
	if err != nil {
		return ClipboardEntry{}, err
	}
	return out, failure
}

package clipboard

// Reorder rearranges the history to the order given by keys (ClipboardEntry.Key
// values, in the wanted order, which is what the UI's Move mode produces).
//
// It resolves everything by key under the store's lock, against the file as it
// is now, because the history changes under the caller: the listener keeps
// prepending. So:
//
//   - entries the caller did not mention (copied meanwhile) stay in front, in
//     their current order, as new entries should;
//   - keys that no longer exist (deleted or expired meanwhile) are skipped;
//   - nothing is ever dropped or duplicated.
func (s *HistoryStore) Reorder(keys []string) error {
	return s.edit(func(h *history) {
		h.ClipboardHistory = reordered(h.ClipboardHistory, keys)
	})
}

func reordered(cur []ClipboardEntry, keys []string) []ClipboardEntry {
	// Two entries can share a key (same text recorded in the same second with
	// allow_duplicates), so each key owns a queue rather than one slot.
	byKey := map[string][]ClipboardEntry{}
	for _, e := range cur {
		byKey[e.Key()] = append(byKey[e.Key()], e)
	}
	listed := map[string]int{}
	for _, k := range keys {
		listed[k]++
	}

	out := make([]ClipboardEntry, 0, len(cur))
	// Entries the caller never saw, newest first as in the file. When a key was
	// listed fewer times than it occurs, the surplus counts as unseen.
	seen := map[string]int{}
	for _, e := range cur {
		k := e.Key()
		if seen[k] >= listed[k] {
			out = append(out, e)
		}
		seen[k]++
	}
	// The caller's order. Taking from the front of each queue keeps duplicates
	// in their relative order.
	used := map[string]int{}
	for _, k := range keys {
		q := byKey[k]
		if used[k] >= len(q) || used[k] >= listed[k] {
			continue
		}
		out = append(out, q[used[k]])
		used[k]++
	}
	return out
}

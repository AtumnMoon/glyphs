package clipboard

import "testing"

func keysOf(t *testing.T, s *HistoryStore) []string {
	t.Helper()
	all, err := s.All()
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(all))
	for i, e := range all {
		out[i] = e.Key()
	}
	return out
}

// seeded returns a store holding d c b a (newest first) and its keys.
func seeded(t *testing.T) (*HistoryStore, map[string]string) {
	t.Helper()
	s := newStore(t, 0)
	for _, v := range []string{"a", "b", "c", "d"} {
		s.RecordText(v, Policy{AllowDuplicates: true})
	}
	all, _ := s.All()
	k := map[string]string{}
	for _, e := range all {
		k[e.Value] = e.Key()
	}
	return s, k
}

func TestReorder(t *testing.T) {
	s, k := seeded(t)
	if err := s.Reorder([]string{k["b"], k["d"], k["a"], k["c"]}); err != nil {
		t.Fatal(err)
	}
	if got := vals(t, s); !same(got, []string{"b", "d", "a", "c"}) {
		t.Errorf("got %v", got)
	}
}

func TestReorderKeepsPinsAndData(t *testing.T) {
	s, k := seeded(t)
	s.Pin(0, true) // "d"
	before, _ := s.All()
	if err := s.Reorder([]string{k["a"], k["b"], k["c"], k["d"]}); err != nil {
		t.Fatal(err)
	}
	after, _ := s.All()
	if len(after) != len(before) {
		t.Fatalf("entries lost or duplicated: %d -> %d", len(before), len(after))
	}
	if after[3].Value != "d" || !after[3].Pinned {
		t.Errorf("pin must travel with its entry: %+v", after)
	}
}

// The listener keeps recording while the UI is in Move mode. Whatever it added
// must survive, at the front, and the user's order must apply to the rest.
func TestReorderKeepsEntriesAddedMeanwhile(t *testing.T) {
	s, k := seeded(t)
	snapshot := []string{k["d"], k["c"], k["b"], k["a"]}
	s.RecordText("new1", Policy{AllowDuplicates: true})
	s.RecordText("new2", Policy{AllowDuplicates: true})

	// the user reversed what they saw
	if err := s.Reorder([]string{snapshot[3], snapshot[2], snapshot[1], snapshot[0]}); err != nil {
		t.Fatal(err)
	}
	if got := vals(t, s); !same(got, []string{"new2", "new1", "a", "b", "c", "d"}) {
		t.Errorf("got %v", got)
	}
}

func TestReorderSkipsEntriesDeletedMeanwhile(t *testing.T) {
	s, k := seeded(t)
	s.Delete(1) // "c"
	if err := s.Reorder([]string{k["a"], k["c"], k["b"], k["d"]}); err != nil {
		t.Fatal(err)
	}
	if got := vals(t, s); !same(got, []string{"a", "b", "d"}) {
		t.Errorf("a vanished key must be ignored, got %v", got)
	}
}

func TestReorderPartialListLeavesTheRestInFront(t *testing.T) {
	s, k := seeded(t)
	if err := s.Reorder([]string{k["a"], k["b"]}); err != nil {
		t.Fatal(err)
	}
	if got := vals(t, s); !same(got, []string{"d", "c", "a", "b"}) {
		t.Errorf("got %v", got)
	}
}

func TestReorderWithDuplicateKeys(t *testing.T) {
	s := newStore(t, 0)
	p := Policy{AllowDuplicates: true}
	s.RecordText("x", p)
	s.RecordText("x", p) // same text, same second: the same key twice
	s.RecordText("y", p)
	all, _ := s.All()
	if all[1].Key() != all[2].Key() && all[1].Value == all[2].Value {
		t.Skip("the two x entries were recorded in different seconds")
	}
	before := keysOf(t, s)
	if err := s.Reorder(before); err != nil {
		t.Fatal(err)
	}
	if after := keysOf(t, s); !same(before, after) {
		t.Errorf("an unchanged order must be a no-op: %v -> %v", before, after)
	}
	if got := vals(t, s); len(got) != 3 {
		t.Errorf("no entry may be lost or duplicated: %v", got)
	}
}

func TestReorderIgnoresDuplicateKeysInTheRequest(t *testing.T) {
	s, k := seeded(t)
	if err := s.Reorder([]string{k["a"], k["a"], k["a"]}); err != nil {
		t.Fatal(err)
	}
	if got := vals(t, s); len(got) != 4 || got[3] != "a" {
		t.Errorf("a key listed three times must still move one entry: %v", got)
	}
}

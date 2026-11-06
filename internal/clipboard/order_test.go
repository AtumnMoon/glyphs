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

// ---- EditText ----------------------------------------------------------------

func TestEditTextKeepsPositionPinAndTime(t *testing.T) {
	s, k := seeded(t) // d c b a
	s.Pin(2, true)    // "b"
	before, _ := s.All()
	got, err := s.EditText(k["b"], "  bee\x00 ", true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != "bee" || !got.Pinned || got.Recorded != before[2].Recorded {
		t.Errorf("returned %+v", got)
	}
	if v := vals(t, s); !same(v, []string{"d", "c", "bee", "a"}) {
		t.Errorf("the entry must stay where it was and be sanitised: %v", v)
	}
	if got.Key() == k["b"] {
		t.Error("the key must change with the text")
	}
}

func TestEditTextDedupes(t *testing.T) {
	s, k := seeded(t) // d c b a
	s.Pin(0, true)    // "d" is pinned
	got, err := s.EditText(k["b"], "d", true)
	if err != nil {
		t.Fatal(err)
	}
	if v := vals(t, s); !same(v, []string{"c", "d", "a"}) {
		t.Errorf("the other entry holding the same text goes: %v", v)
	}
	if !got.Pinned {
		t.Error("the survivor inherits the pin of the entry that was merged into it")
	}
}

func TestEditTextWithDuplicatesAllowed(t *testing.T) {
	s, k := seeded(t)
	if _, err := s.EditText(k["b"], "d", false); err != nil {
		t.Fatal(err)
	}
	if v := vals(t, s); !same(v, []string{"d", "c", "d", "a"}) {
		t.Errorf("both copies stay: %v", v)
	}
}

func TestEditTextErrors(t *testing.T) {
	s, k := seeded(t)
	if _, err := s.EditText(k["a"], " \x00 ", true); err != ErrEmpty {
		t.Errorf("blank: %v", err)
	}
	if _, err := s.EditText("gone", "x", true); err != ErrNoEntry {
		t.Errorf("vanished: %v", err)
	}
	img, _, _ := s.RecordImage([]byte("png"), Policy{})
	if _, err := s.EditText(img.Key(), "x", true); err != ErrNotText {
		t.Errorf("image: %v", err)
	}
	if v := vals(t, s); len(v) != 5 {
		t.Errorf("a failed edit must change nothing: %v", v)
	}
}

// The listener may prepend between the user opening the editor and saving it.
func TestEditTextFindsTheEntryAfterNewOnesArrive(t *testing.T) {
	s, k := seeded(t)
	s.RecordText("fresh", Policy{AllowDuplicates: true})
	if _, err := s.EditText(k["c"], "see", true); err != nil {
		t.Fatal(err)
	}
	if v := vals(t, s); !same(v, []string{"fresh", "d", "see", "b", "a"}) {
		t.Errorf("got %v", v)
	}
}

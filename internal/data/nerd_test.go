package data

import "testing"

func TestEmbeddedNerd(t *testing.T) {
	s, err := NewNerdStore()
	if err != nil {
		t.Fatal(err)
	}
	if s.Version == "" || len(s.BySet("md")) == 0 {
		t.Fatalf("version %q, md=%d", s.Version, len(s.BySet("md")))
	}
	if got := s.Search("arrow left", 10); len(got) == 0 {
		t.Error("no hits for 'arrow left'")
	}
}

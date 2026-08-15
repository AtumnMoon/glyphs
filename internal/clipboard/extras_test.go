package clipboard

import (
	"os"
	"regexp"
	"testing"
	"time"

	gclip "golang.design/x/clipboard"
)

func newStore(t *testing.T, max int) *HistoryStore {
	t.Helper()
	s, err := NewHistory(t.TempDir(), max)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func vals(t *testing.T, s *HistoryStore) []string {
	t.Helper()
	all, err := s.All()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range all {
		out = append(out, e.Value)
	}
	return out
}

func same(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRecordTextDedupeKeepsPin(t *testing.T) {
	s := newStore(t, 0)
	p := Policy{}
	s.RecordText("one", p)
	s.RecordText("two", p)
	all, _ := s.All()
	s.Pin(1, true) // "one"
	_ = all
	s.RecordText("one", p)
	got, _ := s.All()
	if len(got) != 2 || got[0].Value != "one" || !got[0].Pinned {
		t.Errorf("dedupe/pin: %+v", got)
	}
	if _, ok, _ := s.RecordText("  \x00 ", p); ok {
		t.Error("blank text should be skipped")
	}
}

func TestAllowDuplicates(t *testing.T) {
	s := newStore(t, 0)
	p := Policy{AllowDuplicates: true}
	s.RecordText("x", p)
	s.RecordText("x", p)
	if got := vals(t, s); len(got) != 2 {
		t.Errorf("got %v", got)
	}
}

func TestExclude(t *testing.T) {
	ex, err := CompilePatterns([]string{`^sk-[a-z0-9]+$`})
	if err != nil {
		t.Fatal(err)
	}
	s := newStore(t, 0)
	p := Policy{Exclude: ex}
	if _, ok, _ := s.RecordText("sk-abc123", p); ok {
		t.Error("secret recorded")
	}
	if _, ok, _ := s.RecordText("fine", p); !ok {
		t.Error("normal text skipped")
	}
	if _, err := CompilePatterns([]string{"("}); err == nil {
		t.Error("bad pattern should error")
	}
	_ = regexp.MustCompile
}

func TestExpireKeepsPinned(t *testing.T) {
	s := newStore(t, 0)
	old := time.Now().Add(-2 * time.Hour).Format("2006-01-02 15:04:05")
	s.edit(func(h *history) {
		h.ClipboardHistory = []ClipboardEntry{
			{Value: "fresh", Recorded: recordedNow(), FilePath: "null"},
			{Value: "old", Recorded: old, FilePath: "null"},
			{Value: "old pinned", Recorded: old, FilePath: "null", Pinned: true},
		}
	})
	n, err := s.Expire(60)
	if err != nil || n != 1 {
		t.Fatalf("expired %d, %v", n, err)
	}
	if got := vals(t, s); !same(got, []string{"fresh", "old pinned"}) {
		t.Errorf("got %v", got)
	}
	if n, _ := s.Expire(0); n != 0 {
		t.Error("Expire(0) must be a no-op")
	}
}

func TestImagesDedupeAndMaxItemsCleanup(t *testing.T) {
	s := newStore(t, 1)
	a, _, _ := s.RecordImage([]byte("img-a"), Policy{})
	s.RecordImage([]byte("img-a"), Policy{})
	if all, _ := s.All(); len(all) != 1 {
		t.Fatalf("same image twice -> %d entries", len(all))
	}
	s.RecordText("pushes image out", Policy{})
	if _, err := os.Stat(a.FilePath); !os.IsNotExist(err) {
		t.Error("image file should be deleted when trimmed from history")
	}
}

func TestClearScoped(t *testing.T) {
	setup := func() *HistoryStore {
		s := newStore(t, 0)
		s.RecordText("pinned", Policy{})
		s.Pin(0, true)
		s.RecordText("plain", Policy{})
		s.RecordImage([]byte("img"), Policy{})
		return s
	}
	check := func(scope ClearScope, want int) {
		s := setup()
		if _, err := s.ClearScoped(scope); err != nil {
			t.Fatal(err)
		}
		if all, _ := s.All(); len(all) != want {
			t.Errorf("scope %d left %d, want %d", scope, len(all), want)
		}
	}
	check(ClearUnpinned, 1)
	check(ClearAll, 0)
	check(ClearImages, 2)
	check(ClearText, 1)
}

func TestListenWithAppliesPolicy(t *testing.T) {
	s := newStore(t, 0)
	ch := make(chan gclip.Data, 4)
	ch <- gclip.Data{Format: gclip.FmtText, Bytes: []byte("a")}
	ch <- gclip.Data{Format: gclip.FmtText, Bytes: []byte("b")}
	ch <- gclip.Data{Format: gclip.FmtText, Bytes: []byte("a")}
	ch <- gclip.Data{Format: gclip.FmtImage, Bytes: []byte("png")}
	close(ch)
	s.ListenWith(ch, Policy{}, nil)
	all, _ := s.All()
	if len(all) != 3 || !all[0].IsImage() || all[1].Value != "a" || all[2].Value != "b" {
		t.Errorf("unexpected history: %+v", all)
	}
}

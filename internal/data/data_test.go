package data

import "testing"

func TestEmbeddedDatasetsLoad(t *testing.T) {
	k, err := NewKaomojiStore("")
	if err != nil {
		t.Fatal(err)
	}
	if k.Count() < 700 || len(k.Categories()) < 30 {
		t.Errorf("kaomoji: %d items, %d categories", k.Count(), len(k.Categories()))
	}
	if len(k.ByCategory("JOY")) == 0 {
		t.Error("ByCategory should be case-insensitive")
	}
	if len(k.ByCategory("embarrassment")) == 0 || len(k.ByCategory("embarassment")) != 0 {
		t.Error("category typo should be fixed in the dataset")
	}
	e, err := NewEmojiStore("")
	if err != nil {
		t.Fatal(err)
	}
	if e.Count() < 1800 {
		t.Errorf("emoji: %d items", e.Count())
	}
	if got := e.Search("grinning"); len(got) == 0 {
		t.Error("emoji search found nothing")
	}
}

package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func benchHarness(b *testing.B, tab string, w, h int) *harness {
	b.Helper()
	t := &testing.T{}
	hh := newHarness(t, tab, nil)
	hh.send(tea.WindowSizeMsg{Width: w, Height: h})
	return hh
}

// One keypress = Update + the render Bubble Tea performs afterwards.
func BenchmarkEmojiCategorySwitch(b *testing.B) {
	h := benchHarness(b, "emoji", 200, 50)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "down"
		if i%2 == 1 {
			key = "up"
		}
		h.send(keyMsg(key))
		_ = h.m.View()
	}
}

func BenchmarkEmojiGridMove(b *testing.B) {
	h := benchHarness(b, "emoji", 200, 50)
	h.send(keyMsg("right"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "down"
		if i%2 == 1 {
			key = "up"
		}
		h.send(keyMsg(key))
		_ = h.m.View()
	}
}

func BenchmarkKaomojiCategorySwitch(b *testing.B) {
	h := benchHarness(b, "kaomoji", 200, 50)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.send(keyMsg("down"))
		_ = h.m.View()
	}
}

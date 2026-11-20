package data

import "testing"

func TestEmbeddedUnicode(t *testing.T) {
	s, err := NewSymbolStore()
	if err != nil {
		t.Fatal(err)
	}
	for cp, want := range map[rune]string{
		0x41:    "LATIN CAPITAL LETTER A",
		0xAC00:  "HANGUL SYLLABLE GA",
		0x4E00:  "CJK UNIFIED IDEOGRAPH-4E00",
		0x1F600: "GRINNING FACE",
	} {
		if got, ok := s.Lookup(cp); !ok || got.Name != want {
			t.Errorf("%U: %+v ok=%v, want %q", cp, got, ok, want)
		}
	}
	for _, cp := range []rune{0xE000, 0xD800, 0x378, 0xF0001, 0x10FFFF} {
		if _, ok := s.Lookup(cp); ok {
			t.Errorf("%U should be omitted", cp)
		}
	}
	for _, b := range s.Blocks() {
		first, last := b.At(0), b.At(b.Len()-1)
		if first.R < b.Start || last.R > b.End || first.R > last.R {
			t.Errorf("block %q: %U..%U outside %U..%U", b.Name, first.R, last.R, b.Start, b.End)
		}
	}
	if got := s.Search("grinning face", 5); len(got) == 0 || got[0].R != 0x1F600 {
		t.Errorf("search: %+v", got)
	}
	if got := s.Search("U+263A", 5); len(got) != 1 || got[0].R != 0x263A {
		t.Errorf("code point search: %+v", got)
	}
}

func TestDisplay(t *testing.T) {
	s, _ := NewSymbolStore()
	for cp, want := range map[rune]string{0x00: "\u2400", 0x7F: "\u2421", 0x20: "\u2423", 0x301: "\u25CC\u0301", 0x41: "A"} {
		sym, ok := s.Lookup(cp)
		if !ok || sym.Display() != want {
			t.Errorf("%U: %q, want %q", cp, sym.Display(), want)
		}
	}
}

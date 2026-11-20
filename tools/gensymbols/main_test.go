package main

import (
	"strings"
	"testing"
)

func TestParseUnicodeData(t *testing.T) {
	in := "0000;<control>;Cc;0;BN;;;;;N;NULL;;;;\n" +
		"0041;LATIN CAPITAL LETTER A;Lu;0;L;;;;;N;;;;0061;\n" +
		"0080;<control>;Cc;0;BN;;;;;N;;;;;\n" +
		"4E00;<CJK Ideograph, First>;Lo;0;L;;;;;N;;;;;\n" +
		"9FFF;<CJK Ideograph, Last>;Lo;0;L;;;;;N;;;;;\n" +
		"D800;<Non Private Use High Surrogate, First>;Cs;0;L;;;;;N;;;;;\n" +
		"DB7F;<Non Private Use High Surrogate, Last>;Cs;0;L;;;;;N;;;;;\n"
	got, err := parseUnicodeData(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := []entry{
		{0, 0, "Cc", "NULL"},
		{0x41, 0x41, "Lu", "LATIN CAPITAL LETTER A"},
		{0x80, 0x80, "Cc", "CONTROL-0080"},
		{0x4E00, 0x9FFF, "Lo", "CJK UNIFIED IDEOGRAPH"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestExpandHangul(t *testing.T) {
	got := expand(entry{0xAC00, 0xAC01, "Lo", "HANGUL SYLLABLE"})
	if len(got) != 2 || got[0].name != "HANGUL SYLLABLE GA" || got[1].name != "HANGUL SYLLABLE GAG" {
		t.Fatalf("%+v", got)
	}
}

func TestHangulName(t *testing.T) {
	for cp, want := range map[rune]string{0xAC00: "HANGUL SYLLABLE GA", 0xD55C: "HANGUL SYLLABLE HAN", 0xD7A3: "HANGUL SYLLABLE HIH"} {
		if got := hangulName(cp); got != want {
			t.Errorf("%U: %q, want %q", cp, got, want)
		}
	}
}

func TestCollapseKeepsCrossSetDuplicates(t *testing.T) {
	got := collapse([]glyph{{0xE7AD, "dev-amazonwebservices", nil}, {0xE7AD, "dev-aws", nil}, {0xE7AD, "fa-aws", nil}})
	if len(got) != 2 || len(got[0].aliases) != 1 || got[0].aliases[0] != "dev-aws" {
		t.Fatalf("%+v", got)
	}
}

func TestParseBlocks(t *testing.T) {
	v, bs, err := parseBlocks(strings.NewReader("# Blocks-18.0.0.txt\n\n0000..007F; Basic Latin\n"))
	if err != nil || v != "18.0.0" || len(bs) != 1 || bs[0] != (block{0, 0x7F, "Basic Latin"}) {
		t.Fatalf("%q %+v %v", v, bs, err)
	}
}

func TestWriteUnicodeRejectsOrphans(t *testing.T) {
	var sb strings.Builder
	err := writeUnicode(&sb, "x", []block{{0, 0x7F, "Basic Latin"}}, []entry{{0x100, 0x100, "Lu", "X"}})
	if err == nil {
		t.Fatal("want error for a code point outside every block")
	}
}

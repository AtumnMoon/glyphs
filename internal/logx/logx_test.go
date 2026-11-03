package logx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixedClock(l *Logger) {
	t := time.Date(2026, 11, 5, 11, 24, 9, 0, time.FixedZone("PST", 8*3600))
	l.now = func() time.Time { t = t.Add(time.Second); return t }
}

func TestWriteAndReadBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "glyphs.log")
	var tee bytes.Buffer
	l, err := Open(path, &tee)
	if err != nil {
		t.Fatal(err)
	}
	fixedClock(l)
	l.Infof("listener started (pid %d)", 1038)
	l.Warnf("config: unknown option %q", "x")
	l.Errorf("record: multi\nline")
	l.Close()

	got, err := Read(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d entries: %+v", len(got), got)
	}
	// newest first
	if got[0].Level != Error || got[0].Msg != "record: multi ⏎ line" {
		t.Errorf("newest = %+v", got[0])
	}
	if got[1].Level != Warn || got[2].Level != Info || got[2].Msg != "listener started (pid 1038)" {
		t.Errorf("entries = %+v", got)
	}
	if !got[2].Time.Before(got[0].Time) || got[2].Time.IsZero() {
		t.Errorf("times should survive the round trip and increase: %v %v", got[2].Time, got[0].Time)
	}
	if strings.Count(tee.String(), "\n") != 3 || !strings.Contains(tee.String(), "INFO listener started") {
		t.Errorf("tee should receive every line, got %q", tee.String())
	}
}

func TestReadMissingFileIsEmpty(t *testing.T) {
	got, err := Read(filepath.Join(t.TempDir(), "nope.log"), 5)
	if err != nil || len(got) != 0 {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestReadLimitsAndTails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "glyphs.log")
	l, _ := Open(path, nil)
	for i := range 50 {
		l.Infof("line %d", i)
	}
	l.Close()
	got, _ := Read(path, 3)
	if len(got) != 3 || got[0].Msg != "line 49" || got[2].Msg != "line 47" {
		t.Errorf("got %+v", got)
	}
}

func TestReadStartsOnALineBoundaryWhenTheFileIsBig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.log")
	var b strings.Builder
	for i := range 20000 {
		b.WriteString("2026-11-05T11:24:09+08:00 INFO filler line number " + strings.Repeat("x", 20) + " " + string(rune('a'+i%26)) + "\n")
	}
	b.WriteString("2026-11-05T11:25:00+08:00 ERROR the end\n")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Msg != "the end" {
		t.Errorf("newest = %+v", got[0])
	}
	for _, e := range got {
		if e.Time.IsZero() {
			t.Fatalf("a torn first line leaked into the result: %+v", e)
		}
	}
}

func TestForeignLinesAreKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.log")
	body := "2026-11-05T11:24:09+08:00 glyphs listener started (pid 5)\npanic: boom\ngoroutine 1 [running]:\n"
	os.WriteFile(path, []byte(body), 0o644)
	got, _ := Read(path, 10)
	if len(got) != 3 {
		t.Fatalf("got %+v", got)
	}
	if got[2].Level != Info || !got[2].Time.IsZero() {
		t.Errorf("an unparsable line is an Info entry without a time: %+v", got[2])
	}
	if got[1].Level != Error {
		t.Errorf("a panic line is an error: %+v", got[1])
	}
}

func TestRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "glyphs.log")
	l, _ := Open(path, nil)
	l.max = 300
	for i := range 20 {
		l.Infof("entry number %02d with some padding text", i)
	}
	l.Close()
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("the full log should have moved to .1: %v", err)
	}
	fi, _ := os.Stat(path)
	if fi.Size() > 300 {
		t.Errorf("the live log is %d bytes, over the limit", fi.Size())
	}
	got, _ := Read(path, 100)
	if got[0].Msg != "entry number 19 with some padding text" {
		t.Errorf("newest entry lost across rotation: %+v", got[0])
	}
}

func TestOpenRotatesAnAlreadyFullLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "glyphs.log")
	os.WriteFile(path, []byte(strings.Repeat("x\n", MaxBytes/2+1)), 0o644)
	l, err := Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	if fi, _ := os.Stat(path); fi.Size() != 0 {
		t.Errorf("the log should start empty after rotation, is %d bytes", fi.Size())
	}
}

func TestWriterSplitsLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "glyphs.log")
	l, _ := Open(path, nil)
	w := l.Writer(Error)
	w.Write([]byte("first problem\nsecond "))
	w.Write([]byte("problem\n\n"))
	l.Close()
	got, _ := Read(path, 10)
	if len(got) != 2 || got[0].Msg != "second problem" || got[1].Msg != "first problem" || got[0].Level != Error {
		t.Errorf("got %+v", got)
	}
}

func TestAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "glyphs.log")
	if err := Append(path, Error, "listener refused to start: %v", "bad config"); err != nil {
		t.Fatal(err)
	}
	got, _ := Read(path, 1)
	if len(got) != 1 || got[0].Level != Error || !strings.Contains(got[0].Msg, "bad config") {
		t.Errorf("got %+v", got)
	}
}

func TestLevels(t *testing.T) {
	for _, s := range []string{"info", "WARN", "warning", "Error"} {
		if _, ok := ParseLevel(s); !ok {
			t.Errorf("%q should parse", s)
		}
	}
	if _, ok := ParseLevel("loud"); ok {
		t.Error("loud is not a level")
	}
	if !(Info < Warn && Warn < Error) {
		t.Error("levels must be ordered by severity")
	}
}

func TestFlattenDropsBlankLines(t *testing.T) {
	if got := flatten("first\n\n  \tsecond  \r\nthird\n"); got != "first ⏎ second ⏎ third" {
		t.Errorf("got %q", got)
	}
}

// Package logx is glyphs' small log file: one line per event, written by the
// listener (and the commands that record data) and read by the TUI's Logs tab.
//
// A line looks like
//
//	2026-11-05T11:24:09+08:00 INFO listener started (pid 1038)
//
// The listener has to write the file itself. Under systemd its stderr goes to
// the journal, not to a file, so redirecting stderr would leave the Logs tab
// empty exactly when the listener runs as a service.
package logx

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// Level is the severity of an entry. The order is the filter order: showing
// Warn also shows Error.
type Level int

const (
	Info Level = iota
	Warn
	Error
)

func (l Level) String() string {
	switch l {
	case Warn:
		return "WARN"
	case Error:
		return "ERROR"
	}
	return "INFO"
}

// ParseLevel is the inverse of String; ok is false for anything else.
func ParseLevel(s string) (Level, bool) {
	switch strings.ToUpper(s) {
	case "INFO":
		return Info, true
	case "WARN", "WARNING":
		return Warn, true
	case "ERROR", "ERR":
		return Error, true
	}
	return Info, false
}

// Entry is one log line.
type Entry struct {
	Time  time.Time // zero for lines that were not written by Logger
	Level Level
	Msg   string
}

const timeLayout = time.RFC3339

// MaxBytes is the size at which the log is rotated to <path>.1.
const MaxBytes = 1 << 20

// Logger appends to a log file and optionally copies every line to tee.
type Logger struct {
	mu   sync.Mutex
	path string
	f    *os.File
	size int64
	tee  io.Writer
	max  int64
	now  func() time.Time
}

// Open opens (creating) the log at path. tee, if not nil, also receives every
// line, which is how a foreground listener shows its log on stderr/journald.
func Open(path string, tee io.Writer) (*Logger, error) {
	l := &Logger{path: path, tee: tee, max: MaxBytes, now: time.Now}
	if err := l.open(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *Logger) open() error {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return fmt.Errorf("open log: %w", err)
	}
	l.f, l.size = f, fi.Size()
	if l.size >= l.max {
		return l.rotate()
	}
	return nil
}

// rotate moves the full log aside (keeping one generation) and starts anew.
// Callers hold l.mu or are still constructing l.
func (l *Logger) rotate() error {
	l.f.Close()
	_ = os.Remove(l.path + ".1")
	if err := os.Rename(l.path, l.path+".1"); err != nil {
		return fmt.Errorf("rotate log: %w", err)
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("rotate log: %w", err)
	}
	l.f, l.size = f, 0
	return nil
}

// Close closes the file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.f.Close()
}

// CaptureCrashes also sends a fatal Go runtime error (a panic that kills the
// process, which bypasses every logging call) to the log file.
func (l *Logger) CaptureCrashes() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return debug.SetCrashOutput(l.f, debug.CrashOptions{})
}

// flatten keeps an entry on one line: its lines are trimmed, blank ones are
// dropped and the rest joined with " ⏎ ".
func flatten(s string) string {
	var parts []string
	for _, line := range strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == '\r' }) {
		if line = strings.TrimSpace(line); line != "" {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, " ⏎ ")
}

func (l *Logger) write(level Level, msg string) {
	line := fmt.Sprintf("%s %s %s\n", l.now().Format(timeLayout), level, flatten(msg))
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.size+int64(len(line)) > l.max {
		_ = l.rotate() // if it fails keep appending to the old file
	}
	if n, err := l.f.WriteString(line); err == nil {
		l.size += int64(n)
	}
	if l.tee != nil {
		_, _ = io.WriteString(l.tee, line)
	}
}

func (l *Logger) Infof(format string, a ...any)  { l.write(Info, fmt.Sprintf(format, a...)) }
func (l *Logger) Warnf(format string, a ...any)  { l.write(Warn, fmt.Sprintf(format, a...)) }
func (l *Logger) Errorf(format string, a ...any) { l.write(Error, fmt.Sprintf(format, a...)) }

// Writer returns an io.Writer that logs each line written to it at level. It
// adapts code that wants a place to print problems (clipboard.ListenWith).
func (l *Logger) Writer(level Level) io.Writer { return &lineWriter{l: l, level: level} }

type lineWriter struct {
	l     *Logger
	level Level
	buf   []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := strings.IndexByte(string(w.buf), '\n')
		if i < 0 {
			break
		}
		if line := strings.TrimSpace(string(w.buf[:i])); line != "" {
			w.l.write(w.level, line)
		}
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

// Append writes one line to the log at path without keeping it open: for a
// command that fails once and exits.
func Append(path string, level Level, format string, a ...any) error {
	l, err := Open(path, nil)
	if err != nil {
		return err
	}
	defer l.Close()
	l.write(level, fmt.Sprintf(format, a...))
	return nil
}

// ParseLine reads one line. A line that Logger did not write (output of an
// older glyphs, a crash dump) is still returned, as an Info entry without a
// time, or as Error when it is a Go panic.
func ParseLine(line string) (Entry, bool) {
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" {
		return Entry{}, false
	}
	if ts, rest, ok := strings.Cut(line, " "); ok {
		if t, err := time.Parse(timeLayout, ts); err == nil {
			lv, msg, _ := strings.Cut(rest, " ")
			if level, ok := ParseLevel(lv); ok {
				return Entry{Time: t, Level: level, Msg: msg}, true
			}
		}
	}
	level := Info
	if strings.HasPrefix(line, "panic:") || strings.HasPrefix(line, "fatal error:") {
		level = Error
	}
	return Entry{Level: level, Msg: line}, true
}

// tailBytes bounds how much of the file Read looks at.
const tailBytes = 256 << 10

// Read returns up to max entries from the end of the log, newest first. A
// missing file is not an error: it is an empty log.
func Read(path string, max int) ([]Entry, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	start := max64(fi.Size()-tailBytes, 0)
	buf := make([]byte, fi.Size()-start)
	if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
		return nil, err
	}
	text := string(buf)
	if start > 0 { // began mid-line
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			text = text[i+1:]
		}
	}
	lines := strings.Split(text, "\n")
	var out []Entry
	for i := len(lines) - 1; i >= 0 && len(out) < max; i-- {
		if e, ok := ParseLine(lines[i]); ok {
			out = append(out, e)
		}
	}
	return out, nil
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

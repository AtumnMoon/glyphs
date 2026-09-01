package ui

import (
	"os"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// fit truncates s to w display cells (adding "…") and pads it to exactly w.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	if pad := w - ansi.StringWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// oneLine collapses all whitespace runs (newlines, tabs) into single spaces.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func clamp(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	return max(lo, min(v, hi))
}

// cursor tracks a selected index and the first visible row of a scrolling view.
type cursor struct{ idx, off int }

// fix keeps idx within [0,n) and scrolls so the row `row` is inside a window of
// vis rows.
func (c *cursor) fix(n, row, vis int) {
	if n <= 0 {
		c.idx, c.off = 0, 0
		return
	}
	c.idx = clamp(c.idx, 0, n-1)
	if vis < 1 {
		vis = 1
	}
	if row < c.off {
		c.off = row
	}
	if row >= c.off+vis {
		c.off = row - vis + 1
	}
	if c.off < 0 {
		c.off = 0
	}
}

// fuzzyScore reports whether pattern is a case-insensitive subsequence of s and
// scores the match: consecutive runs and word-start hits score higher.
func fuzzyScore(pattern, s string) (int, bool) {
	p := []rune(strings.ToLower(pattern))
	if len(p) == 0 {
		return 0, true
	}
	r := []rune(strings.ToLower(s))
	score, pi, run := 0, 0, 0
	for i, ch := range r {
		if pi < len(p) && ch == p[pi] {
			pi++
			run++
			score += 1 + run*2
			if i == 0 || unicode.IsSpace(r[i-1]) || unicode.IsPunct(r[i-1]) {
				score += 3
			}
		} else {
			run = 0
		}
	}
	if pi < len(p) {
		return 0, false
	}
	// prefer shorter haystacks for equal matches
	return score*1000 - len(r), true
}

func readFile(path string) ([]byte, error) { return os.ReadFile(path) }

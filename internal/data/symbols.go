package data

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// Symbol is one assigned Unicode code point.
type Symbol struct {
	R    rune
	Name string
	Cat  string // general category: Lu, Mn, Cc, ...
}

// Text is what gets copied.
func (s Symbol) Text() string { return string(s.R) }

// Hex is the status-bar form, "U+0041".
func (s Symbol) Hex() string { return fmt.Sprintf("U+%04X", s.R) }

// Invisible reports code points that draw nothing (or that disturb a terminal),
// so the UI should show Display() and may want to confirm before copying.
func (s Symbol) Invisible() bool {
	switch s.Cat {
	case "Cc", "Cf", "Zs", "Zl", "Zp":
		return true
	}
	return false
}

// Display is a safe, visible stand-in for grid cells.
func (s Symbol) Display() string {
	switch {
	case s.Cat == "Cc" && s.R < 0x20:
		return string('\u2400' + s.R)
	case s.Cat == "Cc" && s.R == 0x7F:
		return "\u2421"
	case s.Cat == "Zs":
		return "\u2423"
	case s.Invisible():
		return "\u25AF"
	case strings.HasPrefix(s.Cat, "M"):
		return "\u25CC" + string(s.R)
	}
	return string(s.R)
}

type run struct {
	start, end rune
	cat        string
	name       string // ranged: base name, full name is base + "-" + hex
	offset     int    // index of the run's first code point within its block
	ranged     bool
}

func (r run) size() int { return int(r.end-r.start) + 1 }

func (r run) symbol(cp rune) Symbol {
	name := r.name
	if r.ranged {
		name = fmt.Sprintf("%s-%04X", r.name, cp)
	}
	return Symbol{R: cp, Name: name, Cat: r.cat}
}

// Block is a Unicode block holding only its assigned code points.
type Block struct {
	Name       string
	Start, End rune
	runs       []run
	count      int
}

// Len is the number of assigned code points in the block.
func (b *Block) Len() int { return b.count }

// Plane is 0 for the BMP, 1 for the SMP, and so on.
func (b *Block) Plane() int { return int(b.Start >> 16) }

// At returns the i-th assigned code point, 0 <= i < Len().
func (b *Block) At(i int) Symbol {
	if i < 0 || i >= b.count {
		panic(fmt.Sprintf("data: Block.At(%d) out of range [0,%d)", i, b.count))
	}
	j := sort.Search(len(b.runs), func(k int) bool { return b.runs[k].offset+b.runs[k].size() > i })
	r := b.runs[j]
	return r.symbol(r.start + rune(i-r.offset))
}

func (b *Block) add(r run) {
	r.offset = b.count
	b.count += r.size()
	b.runs = append(b.runs, r)
}

// SymbolStore is the Unicode tab's data: blocks in code point order.
type SymbolStore struct {
	Version string
	blocks  []*Block
}

// NewSymbolStore parses the embedded unicode.txt.gz.
func NewSymbolStore() (*SymbolStore, error) { return parseSymbols(bytes.NewReader(unicodeGz)) }

func parseCP(s string) (rune, error) {
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil || v > 0x10FFFF {
		return 0, fmt.Errorf("bad code point %q", s)
	}
	return rune(v), nil
}

func parseSymbols(src io.Reader) (*SymbolStore, error) {
	zr, err := gzip.NewReader(src)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	s := &SymbolStore{}
	var cur *Block
	sc := bufio.NewScanner(zr)
	for ln := 1; sc.Scan(); ln++ {
		line := sc.Text()
		if line == "" {
			continue
		}
		if v, ok := strings.CutPrefix(line, "# unicode "); ok {
			s.Version = v
			continue
		}
		f := strings.Split(line, "\t")
		bad := func(msg string) error { return fmt.Errorf("unicode.txt line %d: %s", ln, msg) }
		switch f[0] {
		case "B":
			if len(f) != 4 {
				return nil, bad("B needs 4 fields")
			}
			a, err1 := parseCP(f[1])
			b, err2 := parseCP(f[2])
			if err1 != nil || err2 != nil {
				return nil, bad("bad block range")
			}
			cur = &Block{Name: f[3], Start: a, End: b}
			s.blocks = append(s.blocks, cur)
		case "C":
			if len(f) != 4 || cur == nil {
				return nil, bad("malformed C record")
			}
			cp, err := parseCP(f[1])
			if err != nil {
				return nil, bad(err.Error())
			}
			cur.add(run{start: cp, end: cp, cat: f[2], name: f[3]})
		case "R":
			if len(f) != 5 || cur == nil {
				return nil, bad("malformed R record")
			}
			a, err1 := parseCP(f[1])
			b, err2 := parseCP(f[2])
			if err1 != nil || err2 != nil {
				return nil, bad("bad range")
			}
			cur.add(run{start: a, end: b, cat: f[3], name: f[4], ranged: true})
		default:
			return nil, bad("unknown record " + f[0])
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(s.blocks) == 0 {
		return nil, fmt.Errorf("unicode.txt: no blocks")
	}
	return s, nil
}

// Blocks returns every block that has at least one assigned code point.
func (s *SymbolStore) Blocks() []*Block { return s.blocks }

// Lookup resolves a single code point; false for unassigned, surrogate and
// private-use code points (which the store omits by design).
func (s *SymbolStore) Lookup(cp rune) (Symbol, bool) {
	i := sort.Search(len(s.blocks), func(k int) bool { return s.blocks[k].End >= cp })
	if i == len(s.blocks) || s.blocks[i].Start > cp {
		return Symbol{}, false
	}
	runs := s.blocks[i].runs
	j := sort.Search(len(runs), func(k int) bool { return runs[k].end >= cp })
	if j == len(runs) || runs[j].start > cp {
		return Symbol{}, false
	}
	return runs[j].symbol(cp), true
}

// codePointQuery recognises "U+1F600" and "0x1F600". Bare hex is not accepted
// because it collides with words ("face", "bad", "dead").
func codePointQuery(q string) (rune, bool) {
	l := strings.ToLower(q)
	for _, p := range []string{"u+", "0x"} {
		if h, ok := strings.CutPrefix(l, p); ok {
			cp, err := parseCP(h)
			return cp, err == nil
		}
	}
	return 0, false
}

// Search matches names case-insensitively; every whitespace-separated word
// must occur in the name. "U+XXXX" / "0xXXXX" looks up that code point.
// Ranged runs (CJK, Tangut, ...) match on their base name only and are
// enumerated lazily. limit <= 0 means unlimited.
func (s *SymbolStore) Search(query string, limit int) []Symbol {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}
	if cp, ok := codePointQuery(q); ok {
		if sym, ok := s.Lookup(cp); ok {
			return []Symbol{sym}
		}
		return nil
	}
	var out []Symbol
	words := strings.Fields(strings.ToUpper(q))
	full := func() bool { return limit > 0 && len(out) >= limit }
	for _, b := range s.blocks {
		for _, r := range b.runs {
			ok := true
			for _, w := range words {
				if !strings.Contains(r.name, w) {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			for cp := r.start; cp <= r.end && !full(); cp++ {
				out = append(out, r.symbol(cp))
				if !r.ranged {
					break
				}
			}
			if full() {
				return out
			}
		}
	}
	return out
}

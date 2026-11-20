package data

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"strings"
)

// NerdGlyph is one Nerd Fonts icon. Set is the name prefix ("md", "fa", ...).
type NerdGlyph struct {
	R       rune
	Name    string
	Set     string
	Aliases []string
	hay     string
}

func (g NerdGlyph) Text() string { return string(g.R) }
func (g NerdGlyph) Hex() string  { return fmt.Sprintf("U+%04X", g.R) }

type NerdStore struct {
	Version string
	glyphs  []NerdGlyph
	sets    []string
	span    map[string][2]int
}

func NewNerdStore() (*NerdStore, error) {
	zr, err := gzip.NewReader(bytes.NewReader(nerdGz))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	s := &NerdStore{span: map[string][2]int{}}
	sc := bufio.NewScanner(zr)
	for ln := 1; sc.Scan(); ln++ {
		line := sc.Text()
		if line == "" {
			continue
		}
		if v, ok := strings.CutPrefix(line, "# nerd "); ok {
			s.Version = v
			continue
		}
		f := strings.Split(line, "\t")
		if f[0] != "G" || len(f) < 3 {
			return nil, fmt.Errorf("nerd.txt line %d: malformed", ln)
		}
		cp, err := parseCP(f[1])
		if err != nil {
			return nil, fmt.Errorf("nerd.txt line %d: %w", ln, err)
		}
		set, _, _ := strings.Cut(f[2], "-")
		g := NerdGlyph{R: cp, Name: f[2], Set: set, Aliases: f[3:]}
		g.hay = nerdHay(append([]string{g.Name}, g.Aliases...))
		if sp, seen := s.span[set]; !seen {
			s.sets = append(s.sets, set)
			s.span[set] = [2]int{len(s.glyphs), len(s.glyphs) + 1}
		} else if sp[1] != len(s.glyphs) {
			return nil, fmt.Errorf("nerd.txt line %d: set %q is not contiguous", ln, set)
		} else {
			s.span[set] = [2]int{sp[0], sp[1] + 1}
		}
		s.glyphs = append(s.glyphs, g)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(s.glyphs) == 0 {
		return nil, fmt.Errorf("nerd.txt: no glyphs")
	}
	return s, nil
}

func nerdHay(names []string) string {
	return strings.NewReplacer("_", " ", "-", " ").Replace(strings.ToLower(strings.Join(names, " ")))
}

func (s *NerdStore) Count() int { return len(s.glyphs) }

// Sets lists set prefixes in name order.
func (s *NerdStore) Sets() []string { return s.sets }

// BySet returns a set's glyphs, name-sorted. The slice aliases the store.
func (s *NerdStore) BySet(set string) []NerdGlyph {
	sp, ok := s.span[set]
	if !ok {
		return nil
	}
	return s.glyphs[sp[0]:sp[1]]
}

// Search matches words (all must occur) against names and aliases.
func (s *NerdStore) Search(query string, limit int) []NerdGlyph {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}
	cp, byCP := codePointQuery(q)
	words := strings.Fields(nerdHay([]string{q}))
	var out []NerdGlyph
	for _, g := range s.glyphs {
		ok := byCP && g.R == cp
		if !byCP {
			ok = true
			for _, w := range words {
				if !strings.Contains(g.hay, w) {
					ok = false
					break
				}
			}
		}
		if ok {
			out = append(out, g)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out
}

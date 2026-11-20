// Command gensymbols converts the downloaded Unicode and Nerd Fonts sources
// into compact gzip'd text files that internal/data embeds.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type block struct {
	start, end rune
	name       string
}

type entry struct { // one code point, or a First/Last range
	start, end rune
	cat        string
	name       string // ranges: base name; the runtime name is base + "-" + hex
}

type glyph struct {
	code    rune
	name    string
	aliases []string // other names for the same code within the same set
}

func main() {
	src := flag.String("src", "internal/data/sources", "directory with the downloaded files")
	out := flag.String("out", "internal/data", "directory for unicode.txt.gz and nerd.txt.gz")
	flag.Parse()
	if err := run(*src, *out); err != nil {
		fmt.Fprintln(os.Stderr, "gensymbols:", err)
		os.Exit(1)
	}
}

func run(src, out string) error {
	var (
		ver     string
		blocks  []block
		entries []entry
		glyphs  []glyph
		nerdVer string
	)
	steps := []struct {
		file  string
		parse func(io.Reader) error
	}{
		{"Blocks.txt", func(r io.Reader) (err error) { ver, blocks, err = parseBlocks(r); return }},
		{"UnicodeData.txt", func(r io.Reader) (err error) { entries, err = parseUnicodeData(r); return }},
		{"glyphnames.json", func(r io.Reader) (err error) { nerdVer, glyphs, err = parseGlyphnames(r); return }},
	}
	for _, s := range steps {
		f, err := os.Open(filepath.Join(src, s.file))
		if err != nil {
			return err
		}
		err = s.parse(f)
		f.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", s.file, err)
		}
	}
	if err := writeGz(filepath.Join(out, "unicode.txt.gz"), func(w io.Writer) error {
		return writeUnicode(w, ver, blocks, entries)
	}); err != nil {
		return fmt.Errorf("unicode: %w", err)
	}
	if err := writeGz(filepath.Join(out, "nerd.txt.gz"), func(w io.Writer) error {
		return writeNerd(w, nerdVer, glyphs)
	}); err != nil {
		return fmt.Errorf("nerd: %w", err)
	}
	fmt.Printf("unicode %s: %d entries; nerd fonts %s: %d glyphs\n", ver, len(entries), nerdVer, len(glyphs))
	return nil
}

func parseHex(s string) (rune, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 16, 32)
	if err != nil || v > 0x10FFFF {
		return 0, fmt.Errorf("bad code point %q", s)
	}
	return rune(v), nil
}

// ---- Blocks.txt: "0000..007F; Basic Latin" ----

var blocksVersion = regexp.MustCompile(`Blocks-(\d+\.\d+\.\d+)\.txt`)

func parseBlocks(r io.Reader) (version string, bs []block, err error) {
	sc := bufio.NewScanner(r)
	for ln := 1; sc.Scan(); ln++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if m := blocksVersion.FindStringSubmatch(line); m != nil && version == "" {
				version = m[1]
			}
			continue
		}
		rng, name, ok := strings.Cut(line, ";")
		lo, hi, ok2 := strings.Cut(rng, "..")
		if !ok || !ok2 {
			return "", nil, fmt.Errorf("line %d: malformed %q", ln, line)
		}
		s, err := parseHex(lo)
		if err != nil {
			return "", nil, fmt.Errorf("line %d: %w", ln, err)
		}
		e, err := parseHex(hi)
		if err != nil {
			return "", nil, fmt.Errorf("line %d: %w", ln, err)
		}
		bs = append(bs, block{s, e, strings.TrimSpace(name)})
	}
	if version == "" {
		version = "unknown"
	}
	sort.Slice(bs, func(i, j int) bool { return bs[i].start < bs[j].start })
	return version, bs, sc.Err()
}

// ---- UnicodeData.txt ----
// code;name;category;...;field 10 = Unicode 1.0 name (the only name controls have).
// Large ranges appear as "<X, First>" / "<X, Last>" line pairs.

func rangeName(name, suffix string) string {
	return strings.ToUpper(strings.TrimSuffix(strings.TrimPrefix(name, "<"), suffix))
}

func parseUnicodeData(r io.Reader) ([]entry, error) {
	var out []entry
	var first *entry
	add := func(e entry) {
		if e.cat != "Cs" && e.cat != "Co" { // surrogates and private use are not Unicode-tab content
			out = append(out, e)
		}
	}
	sc := bufio.NewScanner(r)
	for ln := 1; sc.Scan(); ln++ {
		line := sc.Text()
		if line == "" {
			continue
		}
		f := strings.Split(line, ";")
		if len(f) < 11 {
			return nil, fmt.Errorf("line %d: %d fields, want >= 11", ln, len(f))
		}
		cp, err := parseHex(f[0])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", ln, err)
		}
		name, cat := f[1], f[2]
		switch {
		case strings.HasSuffix(name, ", First>"):
			first = &entry{start: cp, cat: cat, name: rangeName(name, ", First>")}
			continue
		case strings.HasSuffix(name, ", Last>"):
			if first == nil || first.name != rangeName(name, ", Last>") {
				return nil, fmt.Errorf("line %d: unmatched range end %q", ln, name)
			}
			first.end = cp
			for _, e := range expand(*first) {
				add(e)
			}
			first = nil
			continue
		case name == "<control>":
			if name = f[10]; name == "" {
				name = fmt.Sprintf("CONTROL-%04X", cp)
			}
		}
		add(entry{start: cp, end: cp, cat: cat, name: name})
	}
	if first != nil {
		return nil, fmt.Errorf("range %q never closed", first.name)
	}
	return out, sc.Err()
}

// expand turns a First/Last range into the entries to store. Hangul syllables
// are expanded one per line, because their names are composed from jamo and
// the runtime should not need that algorithm. CJK and Tangut ranges are
// renamed to their real algorithmic base names.
func expand(e entry) []entry {
	switch {
	case e.name == "HANGUL SYLLABLE":
		out := make([]entry, 0, e.end-e.start+1)
		for cp := e.start; cp <= e.end; cp++ {
			out = append(out, entry{cp, cp, e.cat, hangulName(cp)})
		}
		return out
	case strings.HasPrefix(e.name, "CJK IDEOGRAPH"):
		e.name = "CJK UNIFIED IDEOGRAPH"
	case strings.HasPrefix(e.name, "TANGUT IDEOGRAPH"):
		e.name = "TANGUT IDEOGRAPH"
	}
	return []entry{e}
}

// The empty entries (double space in jamoL, leading space in jamoT) are the
// intentional "no consonant" and "no final" jamo: lengths 19, 21, 28.
var (
	jamoL = strings.Split("G GG N D DD R M B BB S SS  J JJ C K T P H", " ")
	jamoV = strings.Split("A AE YA YAE EO E YEO YE O WA WAE OE YO U WEO WE WI YU EU YI I", " ")
	jamoT = strings.Split(" G GG GS N NJ NH D L LG LM LB LS LT LP LH M B BS S SS NG J C K T P H", " ")
)

func hangulName(cp rune) string {
	s := cp - 0xAC00
	return "HANGUL SYLLABLE " + jamoL[s/588] + jamoV[s%588/28] + jamoT[s%28]
}

// ---- output: unicode.txt ----
//
//	# unicode 18.0.0
//	B <start> <end> <block name>          (tab-separated; hex without 0x)
//	C <cp> <cat> <name>                   single code point, belongs to the last B
//	R <start> <end> <cat> <base name>     range; runtime name = base + "-" + hex
//
// Blocks with no assigned code points are dropped.

func writeUnicode(w io.Writer, version string, blocks []block, entries []entry) error {
	bw := bufio.NewWriter(w)
	fmt.Fprintf(bw, "# unicode %s\n", version)
	i := 0
	for _, b := range blocks {
		if i < len(entries) && entries[i].start < b.start {
			return fmt.Errorf("U+%04X is in no block (Blocks.txt and UnicodeData.txt from different versions?)", entries[i].start)
		}
		j := i
		for j < len(entries) && entries[j].start <= b.end {
			if entries[j].end > b.end {
				return fmt.Errorf("range U+%04X..U+%04X crosses block %q", entries[j].start, entries[j].end, b.name)
			}
			j++
		}
		if j == i {
			continue
		}
		fmt.Fprintf(bw, "B\t%04X\t%04X\t%s\n", b.start, b.end, b.name)
		for _, e := range entries[i:j] {
			if e.start == e.end {
				fmt.Fprintf(bw, "C\t%04X\t%s\t%s\n", e.start, e.cat, e.name)
			} else {
				fmt.Fprintf(bw, "R\t%04X\t%04X\t%s\t%s\n", e.start, e.end, e.cat, e.name)
			}
		}
		i = j
	}
	if i != len(entries) {
		return fmt.Errorf("U+%04X is in no block", entries[i].start)
	}
	return bw.Flush()
}

// ---- glyphnames.json ----
// {"METADATA": {"version": "3.x", ...}, "fa-glass": {"char": "...", "code": "f000"}, ...}

func parseGlyphnames(r io.Reader) (version string, gs []glyph, err error) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return "", nil, err
	}
	for name, v := range raw {
		if name == "METADATA" {
			var m struct {
				Version string `json:"version"`
			}
			_ = json.Unmarshal(v, &m)
			version = m.Version
			continue
		}
		var g struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(v, &g); err != nil || g.Code == "" {
			return "", nil, fmt.Errorf("entry %q: no usable \"code\"", name)
		}
		cp, err := parseHex(g.Code)
		if err != nil {
			return "", nil, fmt.Errorf("entry %q: %w", name, err)
		}
		gs = append(gs, glyph{code: cp, name: name})
	}
	if version == "" {
		version = "unknown"
	}
	sort.Slice(gs, func(i, j int) bool { // by set prefix, then name: the UI's grouping order
		si, sj := setOf(gs[i].name), setOf(gs[j].name)
		if si != sj {
			return si < sj
		}
		return gs[i].name < gs[j].name
	})
	return version, collapse(gs), nil
}

func setOf(name string) string {
	set, _, _ := strings.Cut(name, "-")
	return set
}

// collapse merges names that share a code within one set; the alphabetically
// first name stays primary and the rest become search aliases. The same code
// in different sets is kept, since each set is its own group in the UI.
func collapse(gs []glyph) []glyph {
	type key struct {
		set  string
		code rune
	}
	idx := map[key]int{}
	var out []glyph
	for _, g := range gs {
		k := key{setOf(g.name), g.code}
		if i, ok := idx[k]; ok {
			out[i].aliases = append(out[i].aliases, g.name)
			continue
		}
		idx[k] = len(out)
		out = append(out, g)
	}
	return out
}

// nerd.txt:
//
//	# nerd <version>
//	G <hex code> <name> [alias...]        (tab-separated; set = name prefix before '-')
func writeNerd(w io.Writer, version string, gs []glyph) error {
	bw := bufio.NewWriter(w)
	fmt.Fprintf(bw, "# nerd %s\n", version)
	for _, g := range gs {
		fmt.Fprintf(bw, "G\t%X\t%s", g.code, g.name)
		for _, a := range g.aliases {
			fmt.Fprintf(bw, "\t%s", a)
		}
		bw.WriteByte('\n')
	}
	return bw.Flush()
}

func writeGz(path string, fill func(io.Writer) error) error {
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err := fill(zw); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644) // zero gzip mtime: output is deterministic
}

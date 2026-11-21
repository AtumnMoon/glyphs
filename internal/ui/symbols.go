package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/AtumnMoon/glyphs/internal/data"
)

// unicodeSearchLimit caps one Unicode search; the grid is only a window anyway,
// and broad words ("letter") would otherwise match tens of thousands.
const unicodeSearchLimit = 3000

var categoryNames = map[string]string{
	"Lu": "Uppercase letter", "Ll": "Lowercase letter", "Lt": "Titlecase letter",
	"Lm": "Modifier letter", "Lo": "Other letter",
	"Mn": "Nonspacing mark", "Mc": "Spacing mark", "Me": "Enclosing mark",
	"Nd": "Decimal number", "Nl": "Letter number", "No": "Other number",
	"Pc": "Connector punctuation", "Pd": "Dash punctuation", "Ps": "Open punctuation",
	"Pe": "Close punctuation", "Pi": "Initial quote", "Pf": "Final quote", "Po": "Other punctuation",
	"Sm": "Math symbol", "Sc": "Currency symbol", "Sk": "Modifier symbol", "So": "Other symbol",
	"Zs": "Space", "Zl": "Line separator", "Zp": "Paragraph separator",
	"Cc": "Control", "Cf": "Format", "Cn": "Unassigned",
}

// symbolItem turns a code point into a grid item. Invisible code points are
// drawn as a stand-in (show) but still copy as themselves (text).
func symbolItem(s data.Symbol) item {
	show := s.Display()
	return item{text: s.Text(), show: show, width: ansi.StringWidth(show), label: s.Name, cat: s.Cat}
}

// buildUnicode builds the Unicode picker: one group per block, items made
// only for the block on screen, and name search across everything.
func buildUnicode(m *model) (*browser, error) {
	store, err := m.deps.Symbols()
	if err != nil {
		return nil, err
	}
	blocks := store.Blocks()
	names := make([]string, 1, len(blocks)+1)
	counts := make([]int, 1, len(blocks)+1)
	names[0] = "All results"
	for _, bl := range blocks {
		names = append(names, bl.Name)
		counts = append(counts, bl.Len())
	}
	load := func(gi int) []item {
		bl := blocks[gi-1]
		out := make([]item, bl.Len())
		for i := range out {
			out[i] = symbolItem(bl.At(i))
		}
		return out
	}
	search := func(q string) [][]item {
		out := make([][]item, len(names))
		for _, s := range store.Search(q, unicodeSearchLimit) {
			it := symbolItem(s)
			out[0] = append(out[0], it)
			if bi := sort.Search(len(blocks), func(k int) bool { return blocks[k].End >= s.R }); bi < len(blocks) {
				out[bi+1] = append(out[bi+1], it)
			}
		}
		return out
	}
	return newLazyBrowser(names, counts, load, search), nil
}

// nerdSetNames labels the Nerd Fonts icon sets; unknown prefixes show as is.
var nerdSetNames = map[string]string{
	"md": "Material Design", "fa": "Font Awesome", "fae": "Font Awesome Ext.",
	"cod": "Codicons", "dev": "Devicons", "oct": "Octicons", "seti": "Seti-UI",
	"weather": "Weather Icons", "linux": "Font Logos", "ple": "Powerline Extra",
	"pl": "Powerline", "pom": "Pomicons", "iec": "IEC Power", "custom": "Custom",
	"extra": "Extra", "indent": "Indent", "indentation": "Indentation",
}

func nerdHay(g data.NerdGlyph) string {
	s := strings.ToLower(g.Name + " " + strings.Join(g.Aliases, " "))
	s = strings.NewReplacer("_", " ", "-", " ").Replace(s)
	return fmt.Sprintf("%s u+%04x", s, g.R)
}

// buildNerd builds the Nerd Fonts picker: a group per icon set, searchable by
// name, alias or code point. About 10k items, so it is built eagerly.
func buildNerd(m *model) (*browser, error) {
	store, err := m.deps.Nerd()
	if err != nil {
		return nil, err
	}
	names := []string{"All"}
	groups := [][]item{nil}
	for _, set := range store.Sets() {
		label := set
		if n, ok := nerdSetNames[set]; ok {
			label = n
		}
		var g []item
		for _, gl := range store.BySet(set) {
			t := gl.Text()
			g = append(g, item{
				text: t, width: ansi.StringWidth(t),
				label: gl.Name, detail: strings.Join(gl.Aliases, ", "), hay: nerdHay(gl),
			})
		}
		names = append(names, label)
		groups = append(groups, g)
		groups[0] = append(groups[0], g...)
	}
	b := newBrowser(names, groups)
	b.cellW = 4
	return b, nil
}

// symbolDetail is the status line under the Unicode and Nerd grids.
func symbolDetail(tab tabID, it item) string {
	r := []rune(it.text)[0]
	if tab == tabUnicode {
		cat := categoryNames[it.cat]
		if cat == "" {
			cat = it.cat
		}
		return fmt.Sprintf("%s  ·  U+%04X  ·  %s  ·  % X", it.label, r, cat, []byte(it.text))
	}
	s := fmt.Sprintf("%s  ·  U+%04X", it.label, r)
	if it.detail != "" {
		s += "  ·  also: " + it.detail
	}
	return s
}

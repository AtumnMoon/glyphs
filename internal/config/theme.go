package config

import (
	"reflect"
	"strings"
)

// Theme holds every colour the TUI uses as a hex string (or ANSI number).
// Users override any subset under "theme" in config.json.
//
// The first block always has a value. The second block is optional: an empty
// value means "inherit", and Resolve fills in the inherited colour (named in
// each comment), so a theme that only sets the basics still looks coherent.
type Theme struct {
	Accent        string `json:"accent"`       // active tab bg, borders
	AccentFg      string `json:"accentFg"`     // text on accent
	Muted         string `json:"muted"`        // secondary text
	Text          string `json:"text"`         // primary text
	Selected      string `json:"selected"`     // highlighted row
	SelectedBg    string `json:"selectedBg"`   // highlighted row background
	Pin           string `json:"pin"`          // pin marker
	Multi         string `json:"multi"`        // multi-select marker
	Status        string `json:"status"`       // status messages
	Error         string `json:"error"`        // error messages
	FilterPrompt  string `json:"filterPrompt"` // "/" prompt
	InactiveTabFg string `json:"inactiveTabFg"`
	InactiveTabBg string `json:"inactiveTabBg"`

	// ---- optional (empty = inherit) ----

	// Highlighted row.
	SelectedDesc  string `json:"selectedDesc"`  // 2nd line under the highlighted row (inherits selected)
	SelectedPin   string `json:"selectedPin"`   // pin marker on the highlighted row (inherits selected)
	SelectedMulti string `json:"selectedMulti"` // multi-select marker on the highlighted row (inherits selected)
	SelectedDim   string `json:"selectedDim"`   // highlighted row while its pane is not focused (inherits text)
	SelectedDimBg string `json:"selectedDimBg"` // background of that row (inherits selectedBg)

	// Search bar.
	Placeholder  string `json:"placeholder"`  // placeholder / idle hint text (inherits muted)
	FilterText   string `json:"filterText"`   // the query being typed (inherits text)
	FilterCursor string `json:"filterCursor"` // the caret (inherits filterPrompt)
	FilterHint   string `json:"filterHint"`   // "(esc clears)" (inherits muted)

	// Lists and panes.
	Description  string `json:"description"`  // 2nd line of unselected rows (inherits muted)
	Detail       string `json:"detail"`       // detail line under the grid (inherits muted)
	Divider      string `json:"divider"`      // line between the panes (inherits accent)
	PaneTitle    string `json:"paneTitle"`    // pane heading, focused (inherits accent)
	PaneTitleDim string `json:"paneTitleDim"` // pane heading, unfocused (inherits muted)

	// Listener badge in the clipboard header.
	BadgeListening string `json:"badgeListening"` // (inherits status)
	BadgePaused    string `json:"badgePaused"`    // (inherits pin)
	BadgeOff       string `json:"badgeOff"`       // (inherits muted)

	// Help bar. Empty keeps the built-in grey.
	HelpKey       string `json:"helpKey"`
	HelpDesc      string `json:"helpDesc"`
	HelpSeparator string `json:"helpSeparator"`
}

func DefaultTheme() Theme {
	return Theme{
		AccentFg:      "#090C09", // Onyx
		Accent:        "#7A9B76", // Sage Green
		Muted:         "#BAC9BA", // Ash Gray
		Text:          "#BAC9BA", // Ash Gray
		Selected:      "#090C09", // Onyx
		SelectedBg:    "#7A7B76", // Sage Green
		Pin:           "#6B8E9B", // Air Force Blue
		Multi:         "#7A9B76", // Sage Green
		Status:        "#7A9B76", // Sage Green
		Error:         "#B36A5E", // Teracotta Clay
		FilterPrompt:  "#7A9B76", // Sage Green
		InactiveTabFg: "#BAC9BA", // Ash Gray
		InactiveTabBg: "#2D3B2B", // Charcoal Brown
	}
}

// jsonFields maps every "theme" JSON key to the matching field of t.
func (t *Theme) jsonFields() map[string]*string {
	out := map[string]*string{}
	v := reflect.ValueOf(t).Elem()
	for i := 0; i < v.NumField(); i++ {
		name, _, _ := strings.Cut(v.Type().Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			out[name] = v.Field(i).Addr().Interface().(*string)
		}
	}
	return out
}

func (t *Theme) apply(m map[string]string) {
	set := t.jsonFields()
	for k, v := range m {
		if p, ok := set[k]; ok && strings.TrimSpace(v) != "" {
			*p = strings.TrimSpace(v)
		}
	}
}

// Resolve returns a copy of the theme with every optional colour that was
// left empty replaced by the colour it inherits from.
func (t Theme) Resolve() Theme {
	inherit := func(dst *string, from string) {
		if *dst == "" {
			*dst = from
		}
	}
	inherit(&t.SelectedDesc, t.Selected)
	inherit(&t.SelectedPin, t.Selected)
	inherit(&t.SelectedMulti, t.Selected)
	inherit(&t.SelectedDim, t.Text)
	inherit(&t.SelectedDimBg, t.SelectedBg)
	inherit(&t.Placeholder, t.Muted)
	inherit(&t.FilterText, t.Text)
	inherit(&t.FilterCursor, t.FilterPrompt)
	inherit(&t.FilterHint, t.Muted)
	inherit(&t.Description, t.Muted)
	inherit(&t.Detail, t.Muted)
	inherit(&t.Divider, t.Accent)
	inherit(&t.PaneTitle, t.Accent)
	inherit(&t.PaneTitleDim, t.Muted)
	inherit(&t.BadgeListening, t.Status)
	inherit(&t.BadgePaused, t.Pin)
	inherit(&t.BadgeOff, t.Muted)
	return t
}

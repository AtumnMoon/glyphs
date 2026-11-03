package config

import (
	"reflect"
	"strings"
)

// Theme holds every colour the TUI uses as a hex string (or ANSI number).
// Users override any subset in the [theme] table of config.toml.
//
// The first block always has a value. The second block is optional: an empty
// value means "inherit", and Resolve fills in the inherited colour (named in
// each comment), so a theme that only sets the basics still looks coherent.
type Theme struct {
	Accent        string `toml:"accent"`        // active tab bg, borders
	AccentFg      string `toml:"accent_fg"`     // text on accent
	Muted         string `toml:"muted"`         // secondary text
	Text          string `toml:"text"`          // primary text
	Selected      string `toml:"selected"`      // highlighted row
	SelectedBg    string `toml:"selected_bg"`   // highlighted row background
	Pin           string `toml:"pin"`           // pin marker
	Multi         string `toml:"multi"`         // multi-select marker
	Status        string `toml:"status"`        // status messages
	Error         string `toml:"error"`         // error messages
	Warning       string `toml:"warning"`       // warnings (Logs tab)
	FilterPrompt  string `toml:"filter_prompt"` // "/" prompt
	InactiveTabFg string `toml:"inactive_tab_fg"`
	InactiveTabBg string `toml:"inactive_tab_bg"`

	// ---- optional (empty = inherit) ----

	// Highlighted row.
	SelectedDesc  string `toml:"selected_desc"`   // 2nd line under the highlighted row (inherits selected)
	SelectedPin   string `toml:"selected_pin"`    // pin marker on the highlighted row (inherits selected)
	SelectedMulti string `toml:"selected_multi"`  // multi-select marker on the highlighted row (inherits selected)
	SelectedDim   string `toml:"selected_dim"`    // highlighted row while its pane is not focused (inherits text)
	SelectedDimBg string `toml:"selected_dim_bg"` // background of that row (inherits selectedBg)

	// Search bar.
	Placeholder  string `toml:"placeholder"`   // placeholder / idle hint text (inherits muted)
	FilterText   string `toml:"filter_text"`   // the query being typed (inherits text)
	FilterCursor string `toml:"filter_cursor"` // the caret (inherits filterPrompt)
	FilterHint   string `toml:"filter_hint"`   // "(esc clears)" (inherits muted)

	// Lists and panes.
	Description  string `toml:"description"`    // 2nd line of unselected rows (inherits muted)
	Detail       string `toml:"detail"`         // detail line under the grid (inherits muted)
	Divider      string `toml:"divider"`        // line between the panes (inherits accent)
	PaneTitle    string `toml:"pane_title"`     // pane heading, focused (inherits accent)
	PaneTitleDim string `toml:"pane_title_dim"` // pane heading, unfocused (inherits muted)

	// Listener badge in the clipboard header.
	BadgeListening string `toml:"badge_listening"` // (inherits status)
	BadgePaused    string `toml:"badge_paused"`    // (inherits pin)
	BadgeOff       string `toml:"badge_off"`       // (inherits muted)

	// Help bar. Empty keeps the built-in grey.
	HelpKey       string `toml:"help_key"`
	HelpDesc      string `toml:"help_desc"`
	HelpSeparator string `toml:"help_separator"`
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
		Warning:       "#C8A35E", // Ochre
		FilterPrompt:  "#7A9B76", // Sage Green
		InactiveTabFg: "#BAC9BA", // Ash Gray
		InactiveTabBg: "#2D3B2B", // Charcoal Brown
	}
}

// overlay returns t with every non-blank string field of user applied on top.
// A blank value never overrides: it means "not set".
func (t Theme) overlay(user Theme) Theme {
	dst, src := reflect.ValueOf(&t).Elem(), reflect.ValueOf(user)
	for i := range dst.NumField() {
		if v := strings.TrimSpace(src.Field(i).String()); v != "" {
			dst.Field(i).SetString(v)
		}
	}
	return t
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

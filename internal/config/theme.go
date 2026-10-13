package config

import "strings"

// Theme holds every colour the TUI uses as a hex string (or ANSI number).
// Users override any subset under "theme" in config.json.
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

func (t *Theme) apply(m map[string]string) {
	set := map[string]*string{
		"accent": &t.Accent, "accentFg": &t.AccentFg, "muted": &t.Muted,
		"text": &t.Text, "selected": &t.Selected, "selectedBg": &t.SelectedBg,
		"pin": &t.Pin, "multi": &t.Multi, "status": &t.Status, "error": &t.Error,
		"filterPrompt": &t.FilterPrompt, "inactiveTabFg": &t.InactiveTabFg,
		"inactiveTabBg": &t.InactiveTabBg,
	}
	for k, v := range m {
		if p, ok := set[k]; ok && strings.TrimSpace(v) != "" {
			*p = v
		}
	}
}

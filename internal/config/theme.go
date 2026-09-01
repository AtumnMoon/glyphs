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

// DefaultTheme is a purple/pink palette matching the original glyphs look.
func DefaultTheme() Theme {
	return Theme{
		Accent:        "#7D56F4",
		AccentFg:      "#FFFFFF",
		Muted:         "#808080",
		Text:          "#E6E6E6",
		Selected:      "#FF75B7",
		SelectedBg:    "#2A2140",
		Pin:           "#FFD700",
		Multi:         "#2ECC71",
		Status:        "#2ECC71",
		Error:         "#FF5555",
		FilterPrompt:  "#2ECC71",
		InactiveTabFg: "#AAAAAA",
		InactiveTabBg: "#333333",
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

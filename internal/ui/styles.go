package ui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/lipgloss/v2"

	"github.com/AtumnMoon/glyphs/internal/config"
)

type styles struct {
	tabActive, tabInactive            lipgloss.Style
	text, muted, divider              lipgloss.Style
	row, rowSel, rowSelDim            lipgloss.Style
	rowSelDesc                        lipgloss.Style // 2nd line under the highlighted row
	pin, multi, status, err, prompt   lipgloss.Style
	selPin, selMulti                  lipgloss.Style // markers on the highlighted row
	paneTitle, paneTitleDim, previewB lipgloss.Style
	desc, detail                      lipgloss.Style // 2nd line of other rows, grid detail line
	placeholder, filterText           lipgloss.Style
	filterCursor, filterHint          lipgloss.Style
	badgeOn, badgePaused, badgeOff    lipgloss.Style
}

func newStyles(theme config.Theme, ui config.UI) styles {
	t := theme.Resolve()
	c := lipgloss.Color
	base := lipgloss.NewStyle()
	return styles{
		tabActive:    base.Foreground(c(t.AccentFg)).Background(c(t.Accent)).Bold(ui.BoldActiveTab),
		tabInactive:  base.Foreground(c(t.InactiveTabFg)).Background(c(t.InactiveTabBg)),
		text:         base.Foreground(c(t.Text)),
		muted:        base.Foreground(c(t.Muted)),
		divider:      base.Foreground(c(t.Divider)),
		row:          base.Foreground(c(t.Text)),
		rowSel:       base.Foreground(c(t.Selected)).Background(c(t.SelectedBg)).Bold(ui.BoldSelected),
		rowSelDim:    base.Foreground(c(t.SelectedDim)).Background(c(t.SelectedDimBg)),
		rowSelDesc:   base.Foreground(c(t.SelectedDesc)).Background(c(t.SelectedBg)),
		pin:          base.Foreground(c(t.Pin)),
		multi:        base.Foreground(c(t.Multi)).Bold(true),
		selPin:       base.Foreground(c(t.SelectedPin)).Background(c(t.SelectedBg)).Bold(ui.BoldSelected),
		selMulti:     base.Foreground(c(t.SelectedMulti)).Background(c(t.SelectedBg)).Bold(ui.BoldSelected),
		status:       base.Foreground(c(t.Status)),
		err:          base.Foreground(c(t.Error)),
		prompt:       base.Foreground(c(t.FilterPrompt)).Bold(true),
		paneTitle:    base.Foreground(c(t.PaneTitle)).Bold(true),
		paneTitleDim: base.Foreground(c(t.PaneTitleDim)).Bold(true),
		previewB:     base.Foreground(c(t.Accent)),
		desc:         base.Foreground(c(t.Description)),
		detail:       base.Foreground(c(t.Detail)),
		placeholder:  base.Foreground(c(t.Placeholder)),
		filterText:   base.Foreground(c(t.FilterText)),
		filterCursor: base.Foreground(c(t.FilterCursor)).Bold(true),
		filterHint:   base.Foreground(c(t.FilterHint)),
		badgeOn:      base.Foreground(c(t.BadgeListening)),
		badgePaused:  base.Foreground(c(t.BadgePaused)),
		badgeOff:     base.Foreground(c(t.BadgeOff)),
	}
}

// newHelp returns the help bar model, recoloured by the theme's help* keys.
// Keys left empty keep the library's default greys.
func newHelp(theme config.Theme) help.Model {
	h := help.New()
	recolor := func(s *lipgloss.Style, color string) {
		if color != "" {
			*s = s.Foreground(lipgloss.Color(color))
		}
	}
	recolor(&h.Styles.ShortKey, theme.HelpKey)
	recolor(&h.Styles.FullKey, theme.HelpKey)
	recolor(&h.Styles.ShortDesc, theme.HelpDesc)
	recolor(&h.Styles.FullDesc, theme.HelpDesc)
	recolor(&h.Styles.ShortSeparator, theme.HelpSeparator)
	recolor(&h.Styles.FullSeparator, theme.HelpSeparator)
	recolor(&h.Styles.Ellipsis, theme.HelpSeparator)
	return h
}

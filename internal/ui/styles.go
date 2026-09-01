package ui

import (
	"charm.land/lipgloss/v2"

	"github.com/AtumnMoon/glyphs/internal/config"
)

type styles struct {
	tabActive, tabInactive            lipgloss.Style
	text, muted, divider              lipgloss.Style
	row, rowSel, rowSelDim            lipgloss.Style
	pin, multi, status, err, prompt   lipgloss.Style
	paneTitle, paneTitleDim, previewB lipgloss.Style
}

func newStyles(t config.Theme) styles {
	c := lipgloss.Color
	base := lipgloss.NewStyle()
	return styles{
		tabActive:    base.Foreground(c(t.AccentFg)).Background(c(t.Accent)).Bold(true),
		tabInactive:  base.Foreground(c(t.InactiveTabFg)).Background(c(t.InactiveTabBg)),
		text:         base.Foreground(c(t.Text)),
		muted:        base.Foreground(c(t.Muted)),
		divider:      base.Foreground(c(t.Accent)),
		row:          base.Foreground(c(t.Text)),
		rowSel:       base.Foreground(c(t.Selected)).Background(c(t.SelectedBg)).Bold(true),
		rowSelDim:    base.Foreground(c(t.Text)).Background(c(t.SelectedBg)),
		pin:          base.Foreground(c(t.Pin)),
		multi:        base.Foreground(c(t.Multi)).Bold(true),
		status:       base.Foreground(c(t.Status)),
		err:          base.Foreground(c(t.Error)),
		prompt:       base.Foreground(c(t.FilterPrompt)).Bold(true),
		paneTitle:    base.Foreground(c(t.Accent)).Bold(true),
		paneTitleDim: base.Foreground(c(t.Muted)).Bold(true),
		previewB:     base.Foreground(c(t.Accent)),
	}
}

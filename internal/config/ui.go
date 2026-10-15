package config

import (
	"sort"
	"strings"
)

// UI holds every piece of text, marker and small layout switch the TUI shows,
// under "ui" in config.json. Any key left out keeps its default and a key set to
// "" is honoured where it makes sense (e.g. an empty pinMarker hides the pin
// marker, an empty searchPlaceholder shows no placeholder).
//
// Some strings take {placeholders}, replaced at draw time:
//
//	tabFormat        {n} tab number, {name} tab name
//	searchIdle       {key} first key bound to "filter"
//	selectedLabel    {count} number of selected entries
type UI struct {
	WindowTitle string `json:"windowTitle"` // terminal window title

	// Markers in the clipboard list.
	PinMarker    string `json:"pinMarker"`    // shown on pinned entries
	SelectMarker string `json:"selectMarker"` // shown on multi-selected entries

	// Search bar.
	SearchPrompt      string `json:"searchPrompt"`      // before the query
	SearchCursor      string `json:"searchCursor"`      // caret while typing
	SearchPlaceholder string `json:"searchPlaceholder"` // focused, nothing typed yet
	SearchIdle        string `json:"searchIdle"`        // not focused, no query
	SearchClearHint   string `json:"searchClearHint"`   // after a kept query

	// Tab bar. tabNames is keyed by clipboard | kaomoji | emoji.
	TabNames  map[string]string `json:"tabNames"`
	TabFormat string            `json:"tabFormat"`

	// Pane headings and the divider between panes.
	KaomojiHeading string `json:"kaomojiHeading"`
	EmojiHeading   string `json:"emojiHeading"`
	HistoryTitle   string `json:"historyTitle"`
	PinnedOnly     string `json:"pinnedOnly"`    // appended to the title in pinned-only mode
	SelectedLabel  string `json:"selectedLabel"` // appended to the title while multi-selecting
	PreviewTitle   string `json:"previewTitle"`
	Divider        string `json:"divider"`

	// Clipboard header badge.
	BadgeListening string `json:"badgeListening"`
	BadgePaused    string `json:"badgePaused"`
	BadgeOff       string `json:"badgeOff"`

	// Empty states.
	EmptyHistory string `json:"emptyHistory"`
	EmptyPinned  string `json:"emptyPinned"`
	NoMatches    string `json:"noMatches"`

	// Text weight.
	BoldSelected  bool `json:"boldSelected"`  // bold highlighted row
	BoldActiveTab bool `json:"boldActiveTab"` // bold active tab
}

// TabKeys are the stable names of the tabs, in display order.
var TabKeys = []string{"clipboard", "kaomoji", "emoji"}

// DefaultUI returns the built-in strings and markers.
func DefaultUI() UI {
	return UI{
		WindowTitle:       "Glyphs",
		PinMarker:         "🖈",
		SelectMarker:      "✓",
		SearchPrompt:      "❯ ",
		SearchCursor:      "▏",
		SearchPlaceholder: "type to search…",
		SearchIdle:        "press '{key}' for filter",
		SearchClearHint:   "   (esc clears)",
		TabNames: map[string]string{
			"clipboard": "Clipboard",
			"kaomoji":   "Kaomoji",
			"emoji":     "Emoji",
		},
		TabFormat:      "{n} {name}",
		KaomojiHeading: "Categories",
		EmojiHeading:   "Groups",
		HistoryTitle:   "History",
		PinnedOnly:     " · pinned only",
		SelectedLabel:  " · {count} selected",
		PreviewTitle:   "Preview",
		Divider:        " │ ",
		BadgeListening: "● listening",
		BadgePaused:    "⏸ paused",
		BadgeOff:       "○ listener off",
		EmptyHistory:   "History is empty. Run `glyphs -listen` to start recording.",
		EmptyPinned:    "No pinned entries.",
		NoMatches:      "Nothing matches.",
		BoldSelected:   true,
		BoldActiveTab:  true,
	}
}

// oneLineText drops line breaks: every UI string is drawn on a single row.
func oneLineText(s string) string {
	return strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s)
}

// resolve cleans the user-supplied strings. Strings that must never be blank
// fall back to their default; the rest may be emptied on purpose.
func (u UI) resolve() UI {
	d := DefaultUI()
	for _, p := range []*string{
		&u.WindowTitle, &u.PinMarker, &u.SelectMarker, &u.SearchPrompt, &u.SearchCursor,
		&u.SearchPlaceholder, &u.SearchIdle, &u.SearchClearHint, &u.TabFormat,
		&u.KaomojiHeading, &u.EmojiHeading, &u.HistoryTitle, &u.PinnedOnly,
		&u.SelectedLabel, &u.PreviewTitle, &u.Divider, &u.BadgeListening,
		&u.BadgePaused, &u.BadgeOff, &u.EmptyHistory, &u.EmptyPinned, &u.NoMatches,
	} {
		*p = oneLineText(*p)
	}
	// A title or format that is blank would make a tab or pane unreadable.
	if strings.TrimSpace(u.TabFormat) == "" {
		u.TabFormat = d.TabFormat
	}
	if strings.TrimSpace(u.HistoryTitle) == "" {
		u.HistoryTitle = d.HistoryTitle
	}
	// Tab names: start from the defaults, accept non-blank overrides for known
	// tabs only (case-insensitive keys).
	// Keys are visited in sorted order so the result is deterministic even if
	// the user wrote the same tab twice ("Emoji" and "emoji").
	names := DefaultUI().TabNames
	keys := make([]string, 0, len(u.TabNames))
	for k := range u.TabNames {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, rawKey := range keys {
		k, v := strings.ToLower(strings.TrimSpace(rawKey)), u.TabNames[rawKey]
		if _, known := names[k]; known && strings.TrimSpace(v) != "" {
			names[k] = oneLineText(strings.TrimSpace(v))
		}
	}
	u.TabNames = names
	return u
}

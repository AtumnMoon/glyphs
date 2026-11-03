package config

import (
	"sort"
	"strings"
)

// UI holds every piece of text, marker and small layout switch the TUI shows,
// in the [ui] table of config.toml. Any key left out keeps its default and a key set to
// "" is honoured where it makes sense (e.g. an empty pin_marker hides the pin
// marker, an empty search_placeholder shows no placeholder).
//
// Some strings take {placeholders}, replaced at draw time:
//
//	tab_format       {n} tab number, {name} tab name
//	search_idle      {key} first key bound to "filter"
//	selected_label   {count} number of selected entries
//	logs_level       {level} the level being shown (info, warn, error)
type UI struct {
	WindowTitle string `toml:"window_title"` // terminal window title

	// Markers in the clipboard list.
	PinMarker    string `toml:"pin_marker"`    // shown on pinned entries
	SelectMarker string `toml:"select_marker"` // shown on multi-selected entries

	// Search bar.
	SearchPrompt      string `toml:"search_prompt"`      // before the query
	SearchCursor      string `toml:"search_cursor"`      // caret while typing
	SearchPlaceholder string `toml:"search_placeholder"` // focused, nothing typed yet
	SearchIdle        string `toml:"search_idle"`        // not focused, no query
	SearchClearHint   string `toml:"search_clear_hint"`  // after a kept query

	// Tab bar. tab_names is keyed by clipboard | kaomoji | emoji | logs.
	TabNames  map[string]string `toml:"tab_names"`
	TabFormat string            `toml:"tab_format"`

	// Pane headings and the divider between panes.
	KaomojiHeading string `toml:"kaomoji_heading"`
	EmojiHeading   string `toml:"emoji_heading"`
	HistoryTitle   string `toml:"history_title"`
	PinnedOnly     string `toml:"pinned_only"`    // appended to the title in pinned-only mode
	SelectedLabel  string `toml:"selected_label"` // appended to the title while multi-selecting
	PreviewTitle   string `toml:"preview_title"`
	Divider        string `toml:"divider"`

	// Clipboard header badge.
	BadgeListening string `toml:"badge_listening"`
	BadgePaused    string `toml:"badge_paused"`
	BadgeOff       string `toml:"badge_off"`
	BadgeConfig    string `toml:"badge_config"` // shown while the config file has problems

	// Logs tab.
	LogsTitle string `toml:"logs_title"`
	LogsLevel string `toml:"logs_level"` // appended to the title while a level filter is on; {level}
	EmptyLogs string `toml:"empty_logs"`

	// Empty states.
	EmptyHistory string `toml:"empty_history"`
	EmptyPinned  string `toml:"empty_pinned"`
	NoMatches    string `toml:"no_matches"`

	// Text weight.
	BoldSelected  bool `toml:"bold_selected"`   // bold highlighted row
	BoldActiveTab bool `toml:"bold_active_tab"` // bold active tab
}

// TabKeys are the stable names of the tabs, in display order.
var TabKeys = []string{"clipboard", "kaomoji", "emoji", "logs"}

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
			"logs":      "Logs",
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
		BadgeConfig:    "⚠ config",
		LogsTitle:      "Logs",
		LogsLevel:      " · {level} only",
		EmptyLogs:      "No log entries yet. Start the listener with `glyphs listen`.",
		EmptyHistory:   "History is empty. Run `glyphs listen` to start recording.",
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
		&u.BadgePaused, &u.BadgeOff, &u.BadgeConfig, &u.LogsTitle, &u.LogsLevel, &u.EmptyLogs, &u.EmptyHistory, &u.EmptyPinned, &u.NoMatches,
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
	if strings.TrimSpace(u.LogsTitle) == "" {
		u.LogsTitle = d.LogsTitle
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

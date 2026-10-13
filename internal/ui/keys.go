package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"

	"github.com/AtumnMoon/glyphs/internal/config"
)

type keyMap struct {
	Up, Down, Left, Right                      key.Binding
	NextTab, PrevTab                           key.Binding
	Choose, Filter, Preview                    key.Binding
	TogglePin, TogglePinned, Remove            key.Binding
	SelectSingle, SelectUp, SelectDown, ClearS key.Binding
	Home, End, Skin, More, Quit, ForceQuit     key.Binding
}

// bind builds a binding for the action from config, labelling it in help with
// the first configured key.
func bind(cfg config.Config, action, helpKey, desc string) key.Binding {
	keys := cfg.Keys(action)
	if helpKey == "" && len(keys) > 0 {
		helpKey = pretty(keys[0])
	}
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(helpKey, desc))
}

// pretty renders a key name for the help bar.
func pretty(k string) string {
	r := strings.NewReplacer("up", "↑", "down", "↓", "left", "←", "right", "→", "shift+", "⇧", "ctrl+", "^")
	switch k {
	case "space":
		return "space"
	case "backspace":
		return "⌫"
	case "enter":
		return "⏎"
	case "tab":
		return "⇥"
	case "shift+tab":
		return "⇤"
	}
	return r.Replace(k)
}

func newKeyMap(cfg config.Config) keyMap {
	return keyMap{
		Up:           bind(cfg, "up", "", "up"),
		Down:         bind(cfg, "down", "", "down"),
		Left:         bind(cfg, "left", "", "left"),
		Right:        bind(cfg, "right", "", "right"),
		NextTab:      bind(cfg, "nextTab", "", "next tab"),
		PrevTab:      bind(cfg, "prevTab", "", "prev tab"),
		Choose:       bind(cfg, "choose", "", "copy"),
		Filter:       bind(cfg, "filter", "", "filter"),
		Preview:      bind(cfg, "preview", "", "preview"),
		TogglePin:    bind(cfg, "togglePin", "", "pin/unpin"),
		TogglePinned: bind(cfg, "togglePinned", "", "pinned only"),
		Remove:       bind(cfg, "remove", "", "delete"),
		SelectSingle: bind(cfg, "selectSingle", "", "select"),
		SelectUp:     bind(cfg, "selectUp", "", "select ↑"),
		SelectDown:   bind(cfg, "selectDown", "", "select ↓"),
		ClearS:       bind(cfg, "clearSelected", "", "clear selection"),
		Home:         bind(cfg, "home", "", "first"),
		End:          bind(cfg, "end", "", "last"),
		Skin:         bind(cfg, "skinTone", "", "skin tone"),
		More:         bind(cfg, "more", "", "more"),
		Quit:         bind(cfg, "quit", "", "quit"),
		ForceQuit:    bind(cfg, "forceQuit", "", "force quit"),
	}
}

// tabKeys adapts keyMap to help.KeyMap for the active tab and mode.
type tabKeys struct {
	k       keyMap
	tab     tabID
	preview bool // clipboard preview is open
	typing  bool // search box has focus
}

func (t tabKeys) ShortHelp() []key.Binding {
	k := t.k
	nav := key.NewBinding(key.WithKeys("up", "down", "left", "right"), key.WithHelp("↑↓←→", "navigate"))
	switch {
	case t.preview:
		// Same hints as clipse's preview: arrows scroll, Enter copies.
		return []key.Binding{
			key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
			key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
			key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "page down")),
			key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "page up")),
			k.Choose,
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("space/esc", "close")),
		}
	case t.typing:
		stop := key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "stop typing"))
		return []key.Binding{nav, k.Choose, stop, k.NextTab}
	}
	switch t.tab {
	case tabClipboard:
		return []key.Binding{nav, k.Choose, k.Filter, k.Preview, k.TogglePin, k.Remove, k.NextTab, k.More, k.Quit}
	case tabEmoji:
		return []key.Binding{nav, k.Choose, k.Filter, k.Skin, k.NextTab, k.More, k.Quit}
	}
	return []key.Binding{nav, k.Choose, k.Filter, k.NextTab, k.More, k.Quit}
}

func (t tabKeys) FullHelp() [][]key.Binding {
	k := t.k
	if t.preview || t.typing {
		return [][]key.Binding{t.ShortHelp()}
	}
	cols := [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right, k.Home, k.End},
		{k.Choose, k.Filter, k.NextTab, k.PrevTab},
	}
	switch t.tab {
	case tabClipboard:
		cols = append(cols,
			[]key.Binding{k.Preview, k.TogglePin, k.TogglePinned, k.Remove},
			[]key.Binding{k.SelectSingle, k.SelectUp, k.SelectDown, k.ClearS})
	case tabEmoji:
		cols = append(cols, []key.Binding{k.Skin})
	}
	return append(cols, []key.Binding{k.More, k.Quit, k.ForceQuit})
}

package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"

	"github.com/AtumnMoon/glyphs/internal/config"
)

type keyMap struct {
	Up, Down, Left, Right                       key.Binding
	NextTab, PrevTab                            key.Binding
	Choose, Filter, Preview                     key.Binding
	TogglePin, TogglePinned, Remove             key.Binding
	SelectSingle, SelectUp, SelectDown, ClearS  key.Binding
	Home, End, Skin, More, Quit, ForceQuit      key.Binding
	CycleLevel                                  key.Binding
	SelectMode, MoveMode, ModeExit, MoveConfirm key.Binding
	Edit                                        key.Binding
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
		NextTab:      bind(cfg, "next_tab", "", "next tab"),
		PrevTab:      bind(cfg, "prev_tab", "", "prev tab"),
		Choose:       bind(cfg, "choose", "", "copy"),
		Filter:       bind(cfg, "filter", "", "filter"),
		Preview:      bind(cfg, "preview", "", "preview"),
		TogglePin:    bind(cfg, "toggle_pin", "", "pin/unpin"),
		TogglePinned: bind(cfg, "toggle_pinned", "", "pinned only"),
		Remove:       bind(cfg, "remove", "", "delete"),
		SelectSingle: bind(cfg, "select_single", "", "select"),
		SelectUp:     bind(cfg, "select_up", "", "select ↑"),
		SelectDown:   bind(cfg, "select_down", "", "select ↓"),
		ClearS:       bind(cfg, "clear_selected", "", "clear selection"),
		Home:         bind(cfg, "home", "", "first"),
		End:          bind(cfg, "end", "", "last"),
		Skin:         bind(cfg, "skin_tone", "", "skin tone"),
		More:         bind(cfg, "more", "", "more"),
		Quit:         bind(cfg, "quit", "", "quit"),
		ForceQuit:    bind(cfg, "force_quit", "", "force quit"),
		CycleLevel:   bind(cfg, "cycle_level", "", "level"),
		SelectMode:   bind(cfg, "select_mode", "", "select mode"),
		MoveMode:     bind(cfg, "move_mode", "", "move mode"),
		ModeExit:     bind(cfg, "mode_exit", "", "leave mode"),
		MoveConfirm:  bind(cfg, "move_confirm", "", "save order"),
		Edit:         bind(cfg, "edit", "", "edit"),
	}
}

// tabKeys adapts keyMap to help.KeyMap for the active tab and mode.
type tabKeys struct {
	k       keyMap
	tab     tabID
	preview bool // clipboard preview is open
	mode    mode
	typing  bool // search box has focus
}

func (t tabKeys) ShortHelp() []key.Binding {
	k := t.k
	nav := key.NewBinding(key.WithKeys("up", "down", "left", "right"), key.WithHelp("↑↓←→", "navigate"))
	switch {
	case t.mode == modeMove:
		return []key.Binding{
			key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "carry")),
			key.NewBinding(key.WithKeys("home", "end"), key.WithHelp("home/end", "to top/bottom")),
			k.MoveConfirm, k.ModeExit,
		}
	case t.mode == modeSelect:
		done := key.NewBinding(key.WithKeys("v", "esc"), key.WithHelp(k.SelectMode.Help().Key+"/"+k.ModeExit.Help().Key, "done"))
		return []key.Binding{
			key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "extend")),
			k.Choose, k.TogglePin, k.Remove, k.MoveMode, done,
		}
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
		return []key.Binding{nav, k.Choose, k.Filter, k.Preview, k.TogglePin, k.Remove, k.SelectMode, k.MoveMode, k.Edit, k.NextTab, k.More, k.Quit}
	case tabEmoji:
		return []key.Binding{nav, k.Choose, k.Filter, k.Skin, k.NextTab, k.More, k.Quit}
	case tabLogs:
		return []key.Binding{nav, k.Choose, k.Filter, k.CycleLevel, k.Preview, k.NextTab, k.More, k.Quit}
	}
	return []key.Binding{nav, k.Choose, k.Filter, k.NextTab, k.More, k.Quit}
}

func (t tabKeys) FullHelp() [][]key.Binding {
	k := t.k
	if t.preview || t.typing || t.mode != modeNormal {
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
			[]key.Binding{k.SelectMode, k.SelectSingle, k.SelectUp, k.SelectDown, k.ClearS},
			[]key.Binding{k.MoveMode, k.Edit})
	case tabEmoji:
		cols = append(cols, []key.Binding{k.Skin})
	case tabLogs:
		cols = append(cols, []key.Binding{k.CycleLevel, k.Preview})
	}
	return append(cols, []key.Binding{k.More, k.Quit, k.ForceQuit})
}

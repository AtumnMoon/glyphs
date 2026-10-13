# glyphs

Kaomoji picker, emoji picker and clipboard-history manager in one terminal UI.
The clipboard side is inspired by [clipse](https://github.com/savedra1/clipse).

This is an extemely customized one for me but if you wanna use, feel free.

```
glyphs                      # open the TUI
glyphs -tab emoji           # open on a given tab (clipboard | kaomoji | emoji)
glyphs -listen              # record clipboard changes in the background
```

## Tabs

| Tab | What it does |
| --- | --- |
| **Clipboard** | History list (opens first). Pin, multi-select, delete, fuzzy search, live updates. `Space` opens a full-pane preview. |
| **Kaomoji** | Category list + grid. |
| **Emoji** | Group list + grid, `t` cycles skin tone. Search box is focused when the tab opens. |

### Clipboard preview
Closed by default. `Space` opens it over the whole pane, like clipse: `↑↓`
scroll a line, `←→` a page (with a scroll percentage), `Enter` copies, `Space`
or `Esc` closes. Images are drawn as a real bitmap in terminals that support
sixel (foot, WezTerm, mlterm, ...). glyphs asks the terminal at startup, so no
setup is needed. Elsewhere it falls back to a half-block approximation.

Set `imagePreview` to `sixel` (default), `basic` or `off` to force a mode.

### Keys
All rebindable (press `?` in the app).

- Arrows or `hjkl` for movement
- `Enter` copies,
- `/` searches,
- `Tab` / `Shift+Tab` switch tab.
- Clipboard: `p` pin
- `Ctrl+P` pinned-only,
- `x` / `Backspace` delete (asks first for pinned)
- `s` / `Shift+↑↓` multi-select
- `Space` preview.

## CLI

```
glyphs -listen            start background listener (replaces a running one)
glyphs -listen-shell      run the listener in the foreground (debugging)
glyphs -kill              stop the listener
glyphs -pause 5m          pause recording (30s, 5m, 1h ...); -pause off resumes
glyphs -a <text>          add to history without copying (or pipe stdin)
glyphs -c <text>          copy to the system clipboard (or pipe stdin)
glyphs -p                 print current clipboard text
glyphs -output-all raw    print all text entries, one per line (newlines escaped)
glyphs -output-all unescaped   same, entries verbatim
glyphs -clear             remove everything except pinned entries
glyphs -clear-all | -clear-images | -clear-text
glyphs -keep              open the TUI and stay open after copying
glyphs -clean             sanitise text, drop orphaned images/entries
glyphs -v | -help
```

## Configuration

`~/.config/glyphs/config.json` (or `$XDG_CONFIG_HOME`, or `$GLYPHS_CONFIG_DIR`)
is created on first run. Missing keys fall back to defaults.

| Option | Default | Meaning |
| --- | --- | --- |
| `allowDuplicates` | `false` | `false`: re-copying moves the old entry to the top (pin kept) |
| `deleteAfter` | `0` | Auto-delete unpinned entries older than N seconds; `0` = never |
| `excludedPatterns` | `[]` | Regexes; matching text is never recorded, e.g. `"^sk-[A-Za-z0-9]+$"` |
| `maxHistory` | `100` | Max entries (pinned never count against it); `0` = unlimited |
| `quitAfterCopy` | `true` | Exit after picking an item |
| `defaultTab` | `clipboard` | Tab opened by default |
| `searchOnOpen` | `["emoji"]` | Tabs whose search box starts focused (`clipboard`, `kaomoji`, `emoji`) |
| `imagePreview` | `auto` | `auto`, `sixel`, `basic` or `off` |
| `simpleEmoji` | `false` | Hide joined (ZWJ) emoji sequences; turn on if your terminal garbles them |
| `maxEntryLength` | `65` | Max characters of an entry shown in the list |
| `enableDescription` | `true` | Second line (time, size) per entry |
| `enableMouse` | `true` | Wheel scrolling, click tabs |
| `logFile` | `glyphs.log` | Listener log; absolute, `~`, `$VAR`, or relative to the config dir |
| `keyBindings` | | Action → comma-separated keys, e.g. `"quit": "q,esc"` |
| `theme` | purple/pink | Hex colours: `accent`, `selected`, `pin`, ... |

The clipboard history (`clipboard_history.json`, `tmp_files/`) lives in the
config directory and is handled by `internal/clipboard`, on top of
`golang.design/x/clipboard`.

## How clipboard persistence works

History is stored in `clipboard_history.json` (plus `tmp_files/` for images)
inside the config directory, via `internal/clipboard` and
`golang.design/x/clipboard`; `extras.go` there adds dedupe, expiry, exclusion
and scoped clearing without changing the base file.

On Linux/X11 the clipboard only lives as long as the program that copied it.
So every copy from glyphs (`Enter` in the TUI, `-c`) is handed to a tiny
detached holder process (`glyphs -hold`, internal) that owns the clipboard
until something else is copied. macOS and Windows write directly.

## Credits

The clipboard-manager side of glyphs (history file format, listener, CLI and
config design) is inspired by [clipse](https://github.com/savedra1/clipse) by
M Savedra (MIT licensed). glyphs is an independent implementation built on
Bubble Tea v2 and `golang.design/x/clipboard`, and adds the kaomoji and emoji
pickers.

## Install

```
go install github.com/AtumnMoon/glyphs@latest
```

## Development

```
go build ./...
go test ./...
```

## License

MIT, see [LICENSE](LICENSE).

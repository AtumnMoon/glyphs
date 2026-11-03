# glyphs

Kaomoji picker, emoji picker and clipboard-history manager in one terminal UI.
The clipboard side is inspired by [clipse](https://github.com/savedra1/clipse).

This is an extemely customized one for me but if you wanna use, feel free.

```
glyphs                      # open the TUI
glyphs --tab emoji          # open on a given tab (clipboard | kaomoji | emoji)
glyphs listen               # record clipboard changes in the background
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

Set `imagePreview` to `sixel`, `basic` or `off` to force a mode. Default is `auto`.

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

glyphs uses subcommands (built with Cobra). `glyphs --help` and
`glyphs <command> --help` describe everything, shell completions come from
`glyphs completion bash|zsh|fish`, and release archives ship man pages.

```sh
glyphs listen                  # start the background listener (replaces a running one)
glyphs listen --foreground     # run the listener in this process (debugging, systemd)
glyphs kill                    # stop the listener
glyphs pause 5m                # pause recording (30s, 5m, 1h ...); `glyphs pause off` resumes
glyphs add <text>              # add to history without copying (or pipe stdin)
glyphs copy <text>             # copy to the system clipboard (or pipe stdin)
glyphs paste                   # print current clipboard text
glyphs output                  # print all text entries, one per line (newlines escaped)
glyphs output --unescaped      # same, entries verbatim
glyphs clear                   # remove everything except pinned entries
glyphs clear --all|--images|--text
glyphs clean                   # sanitise text, drop orphaned images/entries
glyphs --keep                  # open the TUI and stay open after copying
glyphs --version
```

### Running the listener under systemd

Use the unit in [`contrib/glyphs-listener.service`](contrib/glyphs-listener.service):

```sh
mkdir -p ~/.config/systemd/user
cp contrib/glyphs-listener.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now glyphs-listener.service
```

It runs `glyphs listen --foreground`. Do not point a unit at plain
`glyphs listen`: that command starts a detached child and exits, so systemd
sees the service finish and kills the child with the rest of the service's
control group, which leaves the TUI showing "listener off". Any other
supervisor that tracks the process it starts (runit, s6, ...) needs
`--foreground` for the same reason.

## Configuration

`~/.config/glyphs/config.toml` (or `$XDG_CONFIG_HOME`, or `$GLYPHS_CONFIG_DIR`)
is created on first run, fully commented. Every option is optional: leave it
out and the default applies. Option names are `snake_case`. (There is no
migration from the old `config.json`: it is ignored, with a notice on the
first run.)

**An invalid config is never silently ignored.** The listener (and
`glyphs add`) refuse to run and exit with status 78, because the defaults they
would fall back to have no `excluded_patterns`, so they would record what you
meant to keep out. The TUI still starts, on the defaults, with a `⚠ config`
badge in the clipboard header and the details, with file, line and column, in
the **Logs** tab. Unknown options are only warnings (a camelCase leftover gets
a "did you mean" hint).

| Option | Default | Meaning |
| --- | --- | --- |
| `allow_duplicates` | `false` | `false`: re-copying moves the old entry to the top. A pinned entry keeps its place |
| `delete_after` | `0` | Auto-delete unpinned entries older than N seconds; `0` = never |
| `excluded_patterns` | `[]` | Regexes; matching text is never recorded, e.g. `["^sk-[A-Za-z0-9]+$"]` |
| `max_history` | `100` | Max entries (pinned never count against it); `0` = unlimited |
| `quit_after_copy` | `true` | Exit after picking an item |
| `default_tab` | `"clipboard"` | Tab opened by default (`clipboard`, `kaomoji`, `emoji`, `logs`) |
| `search_on_open` | `["emoji", "kaomoji"]` | Tabs whose search box starts focused |
| `image_preview` | `"auto"` | `auto`, `sixel`, `basic` or `off` |
| `simple_emoji` | `false` | Hide joined (ZWJ) emoji sequences; turn on if your terminal garbles them |
| `emoji_columns` / `emoji_rows` | `16` / `16` | Emoji grid size. Drawing fewer glyphs per frame keeps first paint instant; `0` fills the window |
| `max_entry_length` | `65` | Max characters of an entry shown in the list |
| `enable_description` | `true` | Second line (time, size) per entry |
| `enable_mouse` | `true` | Wheel scrolling, click tabs |
| `log_file` | `"glyphs.log"` | Listener log; absolute, `~`, `$VAR`, or relative to the config dir |
| `[keys]` | | Action → list of keys, e.g. `quit = ["q", "esc"]`; see below |
| `[theme]` | green | Colours, see [Theme](#theme) |
| `[ui]` | | Text, markers and small layout switches, see [UI text and markers](#ui-text-and-markers) |

### Keys

Keys are lists, so any key can be bound, including the comma (`quit = ["q", ","]`).
An empty list is ignored. The actions and their defaults are listed (commented
out) in the generated `config.toml`.

### Theme

Every colour is a quoted hex string (or ANSI number) under `[theme]`. Set only
what you want to change. Optional keys left empty inherit the colour named
below, so changing `selected` or `muted` alone keeps everything coherent.

| Key | Meaning | Inherits |
| --- | --- | --- |
| `accent`, `accent_fg` | Active tab background / text, pane titles, divider | |
| `inactive_tab_fg`, `inactive_tab_bg` | Inactive tabs | |
| `text`, `muted` | Primary / secondary text | |
| `selected`, `selected_bg` | Highlighted row text / background | |
| `selected_desc` | The second line (time, size) under the highlighted row | `selected` |
| `selected_pin`, `selected_multi` | Pin / select marker on the highlighted row | `selected` |
| `selected_dim`, `selected_dim_bg` | Highlighted row while its pane is not focused | `text`, `selected_bg` |
| `pin`, `multi` | Pin / multi-select marker on other rows | |
| `status`, `error` | Status line, errors and delete prompt | |
| `filter_prompt` | The search prompt | |
| `filter_text`, `filter_cursor`, `filter_hint` | Typed query, caret, "(esc clears)" | `text`, `filter_prompt`, `muted` |
| `placeholder` | Search placeholder and idle hint | `muted` |
| `description` | Second line of other rows | `muted` |
| `detail` | Detail line under the emoji/kaomoji grid | `muted` |
| `divider`, `pane_title`, `pane_title_dim` | Pane divider and headings | `accent`, `accent`, `muted` |
| `badge_listening`, `badge_paused`, `badge_off` | Listener badge | `status`, `pin`, `muted` |
| `help_key`, `help_desc`, `help_separator` | Help bar (empty = built-in grey) | |

Light highlight colour? Set `selected` (title) and `selected_desc` (second line)
to something dark, e.g. `selected_desc = "#2D3B2B"`.

### UI text and markers

Everything under `[ui]` is optional. A key set to `""` is honoured where it makes
sense (`pin_marker = ""` hides the marker, `search_placeholder = ""` shows none).
Rows stay aligned whatever marker width you pick.

| Key | Default |
| --- | --- |
| `pin_marker` / `select_marker` | `🖈` / `✓` |
| `search_prompt` / `search_cursor` | `"❯ "` / `▏` |
| `search_placeholder` | `type to search…` (search box focused, nothing typed) |
| `search_idle` | `press '{key}' for filter` (`{key}` = your `filter` binding) |
| `search_clear_hint` | `   (esc clears)` |
| `[ui.tab_names]` | `clipboard = "Clipboard"`, `kaomoji = "Kaomoji"`, `emoji = "Emoji"` |
| `tab_format` | `{n} {name}` (e.g. `"{name}"` to drop the numbers) |
| `kaomoji_heading` / `emoji_heading` | `Categories` / `Groups` |
| `history_title` / `preview_title` | `History` / `Preview` |
| `pinned_only` / `selected_label` | ` · pinned only` / ` · {count} selected` |
| `divider` | `" │ "` |
| `badge_listening` / `badge_paused` / `badge_off` | `● listening` / `⏸ paused` / `○ listener off` |
| `empty_history` / `empty_pinned` / `no_matches` | empty-state messages |
| `window_title` | `Glyphs` |
| `bold_selected` / `bold_active_tab` | `true` / `true` |

Example:

```toml
search_on_open = ["emoji", "kaomoji"]

[theme]
selected = "#090C09"
selected_bg = "#7A9B76"
selected_desc = "#2D3B2B"
placeholder = "#7A9B76"
help_key = "#7A9B76"

[ui]
pin_marker = "🖈"
search_idle = "/ to search"
tab_format = "{name}"
```

The clipboard history (`clipboard_history.json`, `tmp_files/`) lives in the
config directory and is handled by `internal/clipboard`, on top of
`golang.design/x/clipboard`.

## How clipboard persistence works

History is stored in `clipboard_history.json` (plus `tmp_files/` for images)
inside the config directory, via `internal/clipboard` and
`golang.design/x/clipboard`; `extras.go` there adds dedupe, expiry, exclusion
and scoped clearing without changing the base file.

On Linux/X11 the clipboard only lives as long as the program that copied it.
So every copy from glyphs (`Enter` in the TUI, `glyphs copy`) is handed to a tiny
detached holder process (`glyphs hold`, internal) that owns the clipboard
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

## Systemd Service

When starting the listener as a systemd service, use `/usr/bin/glyphs -listen-shell`.

## Development

```
go build ./...
go test ./...
```

## License

MIT, see [LICENSE](LICENSE).

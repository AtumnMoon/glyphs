# Configuration

glyphs is configured with a single TOML file. You don't have to write it
yourself: it is created on first run, fully commented, so you can just
uncomment what you want to change.

### Table of Contents

- [Where the file lives](#where-the-file-lives)
- [If something is wrong](#if-something-is-wrong)
- [Options](#options)
- [Keys](#keys)
- [Theme](#theme)
- [UI text and markers](#ui-text-and-markers)
- [Example](#example)

## Where the file lives

`config.toml` sits in the glyphs config directory, picked in this order:

1. `$GLYPHS_CONFIG_DIR`
2. `$XDG_CONFIG_HOME/glyphs`
3. your OS config dir, e.g. `~/.config/glyphs`

Every option is optional: leave it out (or commented out) and the default
applies. Option names are `snake_case`.

The same directory also holds the clipboard history (`clipboard_history.json`,
`tmp_files/`), the listener log (`glyphs.log`) and a few small runtime files
(pid, lock and pause files). You shouldn't need to touch those.

> There is no migration from the old `config.json`. It is ignored, and you get a
> notice about it on the first run.

## If something is wrong

**An invalid config is never silently ignored.**

- **The listener** (and `glyphs add`) refuse to run and exit with status `78`.
  The defaults they would fall back to have no `excluded_patterns`, so they
  would happily record the things you meant to keep out.
- **The TUI** still starts, on the defaults, with a `⚠ config` badge in the
  clipboard header. The details (file, line and column) are in the **Logs** tab.

What counts as what:

| Problem | Result |
| --- | --- |
| Syntax error, or a value of the wrong type | **Error.** Everything falls back to the defaults, as above |
| Unknown option | Warning, the option is ignored. A camelCase name (an old `config.json` leftover) gets a "did you mean" hint |
| Unknown action under `[keys]` | Warning, ignored |
| Unknown tab in `[ui.tab_names]` or `search_on_open` | Warning |
| `max_history` below 0 | Warning, uses `0` (unlimited) |
| `max_entry_length` below 10 | Warning, uses `65` |
| `default_tab` not a tab name | Warning, uses `clipboard` |
| `image_preview` not a valid mode | Warning, uses `auto` |
| `emoji_columns` / `emoji_rows` below 0 | Quietly treated as `0` |

## Options

Paths (`log_file`) may be absolute, start with `~`, contain `$VARS`, or be
relative to the config directory.

### History

| Option | Default | Meaning |
| --- | --- | --- |
| `allow_duplicates` | `false` | `false`: re-copying something moves the old entry to the top. A pinned entry keeps its place and only refreshes its time |
| `max_history` | `100` | Max entries kept. Pinned entries never count against it. `0` = unlimited |
| `delete_after` | `0` | Auto-delete unpinned entries older than N seconds. `0` = never. Checked by the listener and whenever the TUI opens |
| `excluded_patterns` | `[]` | Regexes. Text that matches is never recorded, e.g. `["^sk-[A-Za-z0-9]+$"]` |
| `log_file` | `"glyphs.log"` | Where the listener writes its log (shown in the Logs tab). Rotates at 1 MiB |

### Interface

| Option | Default | Meaning |
| --- | --- | --- |
| `default_tab` | `"clipboard"` | Tab opened by default: `clipboard`, `kaomoji`, `emoji`, `unicode`, `nerd` or `logs`. `glyphs --tab <name>` overrides it for one run |
| `search_on_open` | `["emoji", "kaomoji"]` | Tabs whose search box starts focused. Takes the same tab names as above |
| `quit_after_copy` | `true` | Exit after picking an item. `glyphs --keep` turns it off for one run |
| `max_entry_length` | `65` | Max characters of an entry shown in the list (minimum `10`) |
| `enable_description` | `true` | Show the second line (time, size) under every entry |
| `enable_mouse` | `true` | Wheel scrolling and clicking the tabs |
| `image_preview` | `"auto"` | `auto`, `sixel`, `basic` or `off`. `auto` asks your terminal; see [Clipboard preview](../README.md#clipboard-preview) |
| `simple_emoji` | `false` | Hide joined (ZWJ) emoji sequences. Try it if your terminal garbles them |
| `emoji_columns` | `16` | Emoji per grid row. `0` fills the window |
| `emoji_rows` | `16` | Emoji grid rows shown at once. `0` fills the window. A smaller grid draws fewer glyphs per frame, which keeps the first paint instant |

### Tables

These have their own sections below.

| Table | What it is |
| --- | --- |
| `[keys]` | Action → list of keys. See [Keys](#keys) |
| `[theme]` | Colours. See [Theme](#theme) |
| `[ui]` | Text, markers and small layout switches. See [UI text and markers](#ui-text-and-markers) |
| `[ui.tab_names]` | Rename the tabs. Part of `[ui]` |

## Keys

Every action takes a **list** of keys, so any key can be bound, including the
comma (`quit = ["q", ","]`). An empty list is ignored, and so is an action name
glyphs doesn't have (with a warning). Press `?` in the app to see what is bound
right now. Setting an action replaces its keys, it doesn't add to them.

| Action | Default | What it does |
| --- | --- | --- |
| `up` / `down` | `["up", "k"]` / `["down", "j"]` | Move the cursor |
| `left` / `right` | `["left", "h"]` / `["right", "l"]` | Move left / right (grids, panes), or page in the preview |
| `home` / `end` | `["home", "g"]` / `["end", "G"]` | Jump to the first / last item |
| `next_tab` / `prev_tab` | `["tab"]` / `["shift+tab"]` | Switch tab |
| `choose` | `["enter"]` | Copy the highlighted item |
| `filter` | `["/"]` | Start searching |
| `preview` | `["space"]` | Open or close the full-pane preview (Clipboard) |
| `toggle_pin` | `["p"]` | Pin / unpin the entry |
| `toggle_pinned` | `["ctrl+p"]` | Show pinned entries only |
| `remove` | `["x", "backspace", "delete"]` | Delete the entry (asks first for pinned ones) |
| `select_single` | `["s"]` | Toggle selection of the entry |
| `select_up` / `select_down` | `["shift+up"]` / `["shift+down"]` | Extend the selection |
| `clear_selected` | `["S"]` | Clear the selection |
| `select_mode` | `["v"]` | Select mode: movement extends the selection |
| `move_mode` | `["m"]` | Move mode: movement carries the entry, which reorders the list |
| `mode_exit` | `["esc"]` | Leave select mode, or cancel move mode |
| `move_confirm` | `["enter"]` | Move mode: save the new order |
| `edit` | `["e"]` | Edit the entry in `$VISUAL` / `$EDITOR` |
| `skin_tone` | `["t"]` | Cycle emoji skin tone |
| `cycle_level` | `["L"]` | Logs tab: cycle all → error → warn → info |
| `more` | `["?"]` | Show the full help |
| `quit` | `["q", "esc"]` | Quit |
| `force_quit` | `["ctrl+c"]` | Quit no matter what (even while typing) |

Key names follow Bubble Tea: `enter`, `esc`, `space`, `tab`, `backspace`,
`delete`, `up`, `home`, `ctrl+p`, `shift+tab`, and plain characters such as `q`
or `G`.

## Theme

Every colour is a **quoted** hex string (or ANSI number) under `[theme]`. Set
only what you want to change.

The first block always has a value. Everything in the second block is optional:
leave it empty and it inherits the colour named in the last column, so changing
`selected` or `muted` alone keeps everything coherent.

### Core colours

| Key | Default | Meaning |
| --- | --- | --- |
| `accent` | `#7A9B76` | Active tab background, pane titles, divider |
| `accent_fg` | `#090C09` | Text on `accent` |
| `inactive_tab_fg` / `inactive_tab_bg` | `#BAC9BA` / `#2D3B2B` | Inactive tabs |
| `text` | `#BAC9BA` | Primary text |
| `muted` | `#BAC9BA` | Secondary text |
| `selected` | `#090C09` | Highlighted row text |
| `selected_bg` | `#7A7B76` | Highlighted row background |
| `pin` | `#6B8E9B` | Pin marker on other rows |
| `multi` | `#7A9B76` | Multi-select marker on other rows |
| `status` | `#7A9B76` | Status line |
| `error` | `#B36A5E` | Errors and the delete prompt |
| `warning` | `#C8A35E` | Warnings (Logs tab) |
| `filter_prompt` | `#7A9B76` | The search prompt |

### Optional colours

| Key | Meaning | Inherits |
| --- | --- | --- |
| `selected_desc` | Second line (time, size) under the highlighted row | `selected` |
| `selected_pin` | Pin marker on the highlighted row | `selected` |
| `selected_multi` | Select marker on the highlighted row | `selected` |
| `selected_dim` | Highlighted row while its pane is not focused | `text` |
| `selected_dim_bg` | Background of that row | `selected_bg` |
| `placeholder` | Search placeholder and idle hint | `muted` |
| `filter_text` | The query you're typing | `text` |
| `filter_cursor` | The caret | `filter_prompt` |
| `filter_hint` | The "(esc clears)" hint | `muted` |
| `description` | Second line of other rows | `muted` |
| `detail` | Detail line under the emoji / kaomoji grid | `muted` |
| `divider` | Line between panes | `accent` |
| `pane_title` | Focused pane heading | `accent` |
| `pane_title_dim` | Unfocused pane heading | `muted` |
| `badge_listening` | Listener badge: listening | `status` |
| `badge_paused` | Listener badge: paused | `pin` |
| `badge_off` | Listener badge: off | `muted` |
| `help_key` / `help_desc` / `help_separator` | Help bar. Empty keeps the built-in grey | |

Light highlight colour? Set `selected` (title) and `selected_desc` (second line)
to something dark, e.g. `selected_desc = "#2D3B2B"`.

## UI text and markers

Everything under `[ui]` is optional. A key set to `""` is honoured where it
makes sense: `pin_marker = ""` hides the marker, `search_placeholder = ""` shows
none. Rows stay aligned whatever marker width you pick.

A few rules:

- Every string is drawn on one row, so line breaks are turned into spaces.
- `tab_format`, `history_title` and `logs_title` can't be blank (a blank tab or
  pane would be unreadable), so an empty value falls back to the default.
- Some strings take `{placeholders}`, filled in when drawn:

| String | Placeholders |
| --- | --- |
| `tab_format` | `{n}` tab number, `{name}` tab name |
| `search_idle` | `{key}` first key bound to `filter` |
| `selected_label` | `{count}` number of selected entries |
| `logs_level` | `{level}` level being shown (info, warn, error) |

### Window, markers and search

| Key | Default |
| --- | --- |
| `window_title` | `Glyphs` |
| `pin_marker` | `🖈` |
| `select_marker` | `✓` |
| `search_prompt` | `"❯ "` |
| `search_cursor` | `▏` |
| `search_placeholder` | `type to search…` (search box focused, nothing typed) |
| `search_idle` | `press '{key}' for filter` |
| `search_clear_hint` | `   (esc clears)` |

### Tabs

| Key | Default |
| --- | --- |
| `tab_format` | `{n} {name}` (use `"{name}"` to drop the numbers) |
| `[ui.tab_names]` | `clipboard = "Clipboard"`, `kaomoji = "Kaomoji"`, `emoji = "Emoji"`, `unicode = "Unicode"`, `nerd = "Nerd"`, `logs = "Logs"` |

`[ui.tab_names]` only accepts those six tab names (case doesn't matter) and
ignores blank values.

### Panes and modes

| Key | Default |
| --- | --- |
| `kaomoji_heading` | `Categories` |
| `emoji_heading` | `Groups` |
| `unicode_heading` | `Blocks` |
| `nerd_heading` | `Sets` |
| `history_title` | `History` |
| `preview_title` | `Preview` |
| `pinned_only` | ` · pinned only` |
| `mode_select` | ` · SELECT` |
| `mode_move` | ` · MOVE` |
| `selected_label` | ` · {count} selected` |
| `divider` | `" │ "` |

### Badges

| Key | Default |
| --- | --- |
| `badge_listening` | `● listening` |
| `badge_paused` | `⏸ paused` |
| `badge_off` | `○ listener off` |
| `badge_config` | `⚠ config` (shown while the config file has problems) |

### Logs tab and empty states

| Key | Default |
| --- | --- |
| `logs_title` | `Logs` |
| `logs_level` | ` · {level} only` |
| `empty_logs` | ``No log entries yet. Start the listener with `glyphs listen`.`` |
| `empty_history` | ``History is empty. Run `glyphs listen` to start recording.`` |
| `empty_pinned` | `No pinned entries.` |
| `no_matches` | `Nothing matches.` |

### Text weight

| Key | Default | Meaning |
| --- | --- | --- |
| `bold_selected` | `true` | Bold highlighted row |
| `bold_active_tab` | `true` | Bold active tab |

## Example

```toml
max_history = 200
excluded_patterns = ["^sk-[A-Za-z0-9]+$"]
default_tab = "emoji"
search_on_open = ["emoji", "kaomoji", "unicode", "nerd"]

[keys]
quit = ["q", ","]

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

[ui.tab_names]
nerd = "Nerd Font"
```

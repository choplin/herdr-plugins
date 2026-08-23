# Herdr Quick Select

Herdr Quick Select brings the hint-based selection model familiar from
[Vim EasyMotion](https://github.com/easymotion/vim-easymotion) to terminal text. It marks
selectable text in the visible viewport with short labels. Type a label to choose the text and
immediately perform the requested operation.

Quick Select ships with two ready-to-use presets: `copy` finds common terminal tokens and copies
the chosen value; `open-url` finds URLs and opens the chosen one in the system browser. Each
preset is a command composed of one or more selectors, which identify selectable text, and one
action, which handles the selected value. Selectors and actions are customizable and reusable, so
custom commands can target other text or pass values to external executables.

## Presets

### Copy

![Herdr Quick Select labeling terminal values, copying a selected path, and confirming the copy](assets/quickselect-copy.gif)

### Open URL

![Herdr Quick Select labeling a URL and confirming that it opened in the browser](assets/quickselect-open-url.gif)

## Requirements

- Herdr 0.8.2 or newer
- Linux or macOS
- Go 1.22 or newer during installation

Clipboard actions also require one platform tool:

- macOS: `pbcopy`, included with macOS
- Linux Wayland: `wl-copy`
- Linux X11: `xclip` or `xsel`

URL opening uses `open` on macOS and `xdg-open` on Linux.

## Quick start

Install the plugin:

```sh
herdr plugin install choplin/herdr-plugins/herdr-quickselect
```

Add a URL-opening keybinding to `~/.config/herdr/config.toml`:

```toml
[[keys.command]]
key = "ctrl+shift+p"
type = "plugin_action"
command = "choplin.quickselect.open-url"
description = "Open visible URL"
```

Reload Herdr:

```sh
herdr server reload-config
```

Display a URL in the focused pane, press `Ctrl+Shift+P`, and type the cyan hint shown at the start
of the URL. Quick Select opens the selected URL with the system browser, returns to the original
tab, and closes its temporary picker tab. Press `Escape` or `Ctrl-C` to cancel instead.

To copy any built-in match type, bind the copy action to another key:

```toml
[[keys.command]]
key = "ctrl+shift+x"
type = "plugin_action"
command = "choplin.quickselect.copy"
description = "Copy visible item"
```

## How it works

Quick Select reads only the focused pane's visible viewport. It opens a temporary tab with the
same split layout, renders the captured viewport in the target pane, and places one- or two-letter
hints over matching text. The source pane remains unchanged.

Every visible occurrence of the same value shares one hint. After selection or cancellation,
Quick Select focuses the source tab and removes the temporary tab. While a picker is active,
additional Quick Select invocations in the same workspace are ignored.

When Herdr notifications are enabled, successful `clipboard` and `open` actions also show
`Copied to clipboard` and `Opened URL in browser` notifications. Notification delivery is
best-effort and does not change whether the underlying action succeeds.

The viewport is a snapshot taken when the picker opens; later terminal output is not added. Quick
Select assigns hints to at most 676 unique values in one viewport.

## Configuration model

Quick Select reads `quickselect.toml` beside Herdr's `config.toml`. With the default Herdr
configuration path, create it at `${XDG_CONFIG_HOME:-$HOME/.config}/herdr/quickselect.toml`:

```sh
${EDITOR:-vi} "${XDG_CONFIG_HOME:-$HOME/.config}/herdr/quickselect.toml"
```

When `HERDR_CONFIG_PATH` points to another Herdr configuration, Quick Select reads
`quickselect.toml` from that file's directory. The file is optional; built-in configuration is
used when it does not exist.

Configuration has three reusable object types:

| Object | Purpose |
| --- | --- |
| `commands` | The unit invoked by a Herdr plugin action; chooses selectors and one action. |
| `selectors` | Finds values in visible terminal text. |
| `actions` | Processes the selected value. |

Entries with the same ID replace built-ins. New IDs are added. Quick Select rejects unknown keys,
duplicate IDs, invalid regular expressions, and references to missing selectors or actions.

### Change a built-in command

The manifest exposes two plugin actions:

| Plugin action | Command ID | Default selectors | Default action |
| --- | --- | --- | --- |
| `choplin.quickselect.copy` | `copy` | All built-in selectors | `clipboard` |
| `choplin.quickselect.open-url` | `open-url` | `url` | `browser` |

Override a command with the same ID to change an existing keybinding without editing the plugin
manifest. This example makes `choplin.quickselect.copy` copy URLs only:

```toml
[[commands]]
id = "copy"
label = "Copy URL"
selectors = ["url"]
action = "clipboard"
```

### Add a custom command

On macOS, this example recognizes Jira issue keys and opens the selected issue in the browser:

```toml
[[selectors]]
id = "jira"
label = "Jira issue"
regex = "\\b[A-Z][A-Z0-9]+-[0-9]+\\b"
priority = 5

[[actions]]
id = "jira-browser"
label = "Open Jira"
type = "exec"
argv = ["open", "https://jira.example.com/browse/${value}"]

[[commands]]
id = "jira"
label = "Open Jira issue"
selectors = ["jira"]
action = "jira-browser"
```

Herdr plugin actions are declared statically in `herdr-plugin.toml`. To expose the new `jira`
command, add a manifest action in a local checkout:

```toml
[[actions]]
id = "jira"
title = "Open Jira issue"
contexts = ["pane", "workspace"]
command = ["./herdr-quickselect", "launch", "jira"]
```

Build and link that checkout, then bind `choplin.quickselect.jira` in Herdr:

```sh
go build -trimpath -o herdr-quickselect ./cmd/herdr-quickselect
herdr plugin link .
```

```toml
[[keys.command]]
key = "prefix+j"
type = "plugin_action"
command = "choplin.quickselect.jira"
description = "Open Jira issue"
```

## Configuration reference

### Commands

Each `[[commands]]` entry supports:

- `id`: unique command ID passed to the plugin's `launch` entrypoint by a manifest action.
- `label`: temporary picker tab label.
- `selectors`: one or more selector IDs.
- `action`: action ID applied to the selected value.

### Selectors

Each `[[selectors]]` entry supports:

- `id`: unique selector ID.
- `label`: match category label.
- `matcher`: optional matcher type. Use `"url"` for structured URL extraction; omit it or use
  `"regex"` for a regular-expression selector.
- `regex`: Go regular expression evaluated on each visible logical line.
- `capture`: optional named capture whose value is selected instead of the full match.
- `priority`: overlap priority; lower values win.

The built-in `url` selector uses the URL matcher. The other built-in selectors are `path`, `uuid`,
`git-sha`, `ipv4`, and `number`.

Use a named capture to select only part of a match:

```toml
[[selectors]]
id = "trace"
label = "Trace ID"
regex = "trace_id=(?P<match>[A-Za-z0-9_-]+)"
capture = "match"
priority = 5
```

### Actions

Each `[[actions]]` entry has an `id`, a `label`, and one of these action types:

| Type | Behavior | Additional fields |
| --- | --- | --- |
| `clipboard` | Copies the selected value. | None |
| `open` | Opens the selected value with the system handler. | None |
| `exec` | Executes an argv array without a shell. | `argv`, optional `stdin` |

Exec actions expand placeholders inside each argv item:

| Placeholder | Value |
| --- | --- |
| `${value}` | Selected text |
| `${pane_id}` | Source Herdr pane ID |
| `${cwd}` | Focused pane's working directory, when available |

Prefix a placeholder with an extra `$` to pass it literally: `$${value}` becomes `${value}`.
Set `stdin = true` to also send the selected value to the executable's standard input.
An exec action must use an unescaped `${value}` placeholder unless `stdin = true`.

```toml
[[actions]]
id = "inspect"
label = "Inspect resource"
type = "exec"
argv = ["resource-inspector", "--source-pane", "${pane_id}", "${value}"]
```

## Manual invocation and logs

Invoke either built-in command without a keybinding:

```sh
herdr plugin action invoke copy --plugin choplin.quickselect
herdr plugin action invoke open-url --plugin choplin.quickselect
```

Inspect failures and configuration diagnostics in the plugin action logs:

```sh
herdr plugin log list --plugin choplin.quickselect
```

## Local development

`herdr plugin link` does not run manifest build commands. Build the binary before linking:

```sh
nix develop
cd herdr-quickselect
go build -trimpath -o herdr-quickselect ./cmd/herdr-quickselect
herdr plugin link .
```

Run formatting, tests, and static analysis from `herdr-quickselect`:

```sh
gofumpt -w .
go test ./...
golangci-lint run
```

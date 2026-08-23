# Herdr Agent Update Time

Add the time of each Agent's last state change to Herdr's Agents sidebar.

![Agents showing their latest state-change times in Herdr's Agents sidebar](docs/agent-update-time-annotated.png)

If an Agent changes from `working` to `blocked` at 14:32, the plugin reports
`$updated=14:32`. The timestamp remains unchanged until the Agent's semantic state changes again.

## Install

Installation requires Herdr 0.8.2 or newer and Go 1.22 or newer:

```sh
herdr plugin install choplin/herdr-plugins/herdr-agent-update-time
```

The manifest builds a native binary during installation, so the plugin has no runtime language
dependency.

## Usage

Add `$updated` to the Agent sidebar layout in `~/.config/herdr/config.toml`:

```toml
[ui.sidebar.agents]
rows = [
  ["state_icon", "workspace", "tab"],
  ["agent", "$updated"],
]
```

If `[ui.sidebar.agents]` already exists, add `$updated` to its existing `rows` instead of adding a
second table.

Start an Agent, or wait for a running Agent to change state. The Agents sidebar now shows the time
next to the Agent name, such as `codex · 14:32`. The time changes when the Agent moves between
semantic states such as `working`, `blocked`, `done`, or `idle`.

## How it works

The plugin watches Agent detection and semantic state changes, tracks each Agent across pane
changes, and reports the latest observation time as the `$updated` metadata token. It also clears
metadata for Agents and panes that no longer exist.

## Manual reconciliation and logs

The plugin reconciles Agent update times when Herdr starts and whenever a relevant Agent or pane
event occurs.

To reconcile all current Agents manually:

```sh
herdr plugin action invoke reconcile --plugin choplin.agent-update-time
```

Inspect command logs with:

```sh
herdr plugin log list --plugin choplin.agent-update-time
```

## Local development

`herdr plugin link` does not run manifest build commands, so build before linking:

```sh
nix develop
cd herdr-agent-update-time
go build -trimpath -o herdr-agent-update-time ./cmd/herdr-agent-update-time
herdr plugin link .
```

Run the test suite with:

```sh
go test ./...
```

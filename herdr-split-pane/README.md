# Herdr Split Pane

Run a caller-provided command directly in a Herdr split pane. The split closes when the command
exits.

![Lazygit opening in a right split pane](docs/split-pane-demo.gif)

## Install

Installation requires Herdr 0.8.2 or newer:

```sh
herdr plugin install choplin/herdr-plugins/herdr-split-pane
```

## Example

Add the following to `~/.config/herdr/config.toml` to open Lazygit on the right with `Ctrl+G`:

```toml
[[keys.command]]
key = "ctrl+g"
type = "shell"
description = "Open lazygit in a split pane"
command = '''
exec "$HERDR_BIN_PATH" plugin pane open \
  --plugin choplin.split-pane \
  --entrypoint command \
  --placement split \
  --target-pane "$HERDR_ACTIVE_PANE_ID" \
  --direction right \
  --cwd "$HERDR_ACTIVE_PANE_CWD" \
  --env "HERDR_SPLIT_COMMAND=lazygit" \
  --focus
'''
```

## Configuration

Split Pane accepts one input: `HERDR_SPLIT_COMMAND`. It is required and contains the command to run
in the new pane. The example sets it to `lazygit`:

```sh
--env "HERDR_SPLIT_COMMAND=lazygit"
```

The value may include arguments, quoting, pipelines, and redirections. For example:

```sh
--env 'HERDR_SPLIT_COMMAND=git log --oneline --decorate'
```

The plugin evaluates the value with non-login `/bin/sh`. For a simple command, `exec` replaces the
shell with the command process. When that process exits, Herdr closes the plugin pane.

For Herdr's `[[keys.command]]` fields and environment variables, see
[Custom command keybindings](https://herdr.dev/docs/configuration/#custom-command-keybindings).
For `plugin pane open` and its options, see the
[Herdr CLI reference](https://herdr.dev/docs/cli-reference/#plugins).

## Local development

The plugin has no build step. Link it from the repository root:

```sh
herdr plugin link ./herdr-split-pane --enabled
```

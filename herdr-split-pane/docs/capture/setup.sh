#!/bin/sh

set -eu

session_name=split-pane-showcase
demo_root=/tmp/herdr-split-pane-showcase
config_root=/tmp/herdr-split-pane-showcase-config
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
plugin_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)

HERDR_CONFIG_PATH="$config_root/config.toml" \
  herdr session stop "$session_name" >/dev/null 2>&1 || true
HERDR_CONFIG_PATH="$config_root/config.toml" \
  herdr session delete "$session_name" >/dev/null 2>&1 || true

rm -rf /tmp/herdr-split-pane-showcase /tmp/herdr-split-pane-showcase-config
mkdir -p "$demo_root/acme-cli" "$config_root/bin"

cat >"$demo_root/acme-cli/README.md" <<'EOF'
# Acme CLI

A small command-line project used to demonstrate Herdr Split Pane.
EOF

cat >"$demo_root/acme-cli/main.go" <<'EOF'
package main

import "fmt"

func main() {
	fmt.Println("Acme CLI")
}
EOF

cat >"$demo_root/acme-cli/config.toml" <<'EOF'
[server]
address = "127.0.0.1:8080"
EOF

git -C "$demo_root/acme-cli" init -q
git -C "$demo_root/acme-cli" config user.name "Demo User"
git -C "$demo_root/acme-cli" config user.email "demo@example.com"
git -C "$demo_root/acme-cli" add README.md main.go config.toml
git -C "$demo_root/acme-cli" commit -qm "Initial project"
git -C "$demo_root/acme-cli" branch -M main

cat >"$demo_root/acme-cli/main.go" <<'EOF'
package main

import "fmt"

func main() {
	fmt.Println("Acme CLI")
	fmt.Println("Ready")
}
EOF

cat >"$config_root/bin/open-in-split" <<'EOF'
#!/bin/sh

set -eu

command=$1
direction=$2
herdr_bin=${HERDR_BIN_PATH:-herdr}
target_pane=${HERDR_ACTIVE_PANE_ID:-${HERDR_PANE_ID:?HERDR_PANE_ID is required}}
target_cwd=${HERDR_ACTIVE_PANE_CWD:-$PWD}

"$herdr_bin" plugin pane open \
  --plugin choplin.split-pane \
  --entrypoint command \
  --placement split \
  --target-pane "$target_pane" \
  --direction "$direction" \
  --cwd "$target_cwd" \
  --env "HERDR_SPLIT_COMMAND=$command" \
  --focus >/dev/null

printf '\033[2J\033[H'
printf '\033[1;38;2;166;227;161m❯\033[0m # Ctrl+G opens lazygit in a split pane.\n'
printf '\033[1;38;2;166;227;161m❯\033[0m # The pane closes when lazygit exits.\n'
EOF
chmod +x "$config_root/bin/open-in-split"

real_lazygit=$(command -v lazygit)
cat >"$config_root/bin/lazygit" <<EOF
#!/bin/sh

exec "$real_lazygit" \
  --use-config-file=/tmp/herdr-split-pane-showcase-config/lazygit/config.yml \
  "\$@"
EOF
chmod +x "$config_root/bin/lazygit"

cat >"$config_root/bin/launch-herdr" <<'EOF'
#!/bin/sh

cd /tmp/herdr-split-pane-showcase/acme-cli

exec env -i \
  HOME="$HOME" \
  PATH="/tmp/herdr-split-pane-showcase-config/bin:$PATH" \
  TERM=xterm-256color \
  COLORTERM=truecolor \
  LANG=en_US.UTF-8 \
  LC_ALL=en_US.UTF-8 \
  PS1='\[\e[1;38;2;166;227;161m\]❯\[\e[0m\] ' \
  PS2='· ' \
  HERDR_CONFIG_PATH=/tmp/herdr-split-pane-showcase-config/config.toml \
  herdr --session split-pane-showcase
EOF
chmod +x "$config_root/bin/launch-herdr"

mkdir -p "$config_root/lazygit"
cat >"$config_root/lazygit/config.yml" <<'EOF'
gui:
  showIcons: true
  nerdFontsVersion: "3"
  showFileIcons: true
  showRandomTip: false
  showCommandLog: false
  theme:
    activeBorderColor:
      - "#a6e3a1"
      - bold
    inactiveBorderColor:
      - "#6c7086"
    searchingActiveBorderColor:
      - "#89dceb"
      - bold
    optionsTextColor:
      - "#89b4fa"
    selectedLineBgColor:
      - "#313244"
    inactiveViewSelectedLineBgColor:
      - "#1e1e2e"
    cherryPickedCommitFgColor:
      - "#cdd6f4"
    cherryPickedCommitBgColor:
      - "#45475a"
    markedBaseCommitFgColor:
      - "#11111b"
    markedBaseCommitBgColor:
      - "#f9e2af"
    unstagedChangesColor:
      - "#f38ba8"
    defaultFgColor:
      - "#cdd6f4"
EOF

cat >"$config_root/config.toml" <<'EOF'
onboarding = false

[theme]
name = "catppuccin"

[terminal]
default_shell = "/bin/sh"
shell_mode = "non_login"

[keys]
prefix = "ctrl+b"

[[keys.command]]
key = "ctrl+g"
type = "shell"
description = "Open lazygit in a split pane"
command = "exec open-in-split lazygit right"

[ui]
sidebar_start_collapsed = false
hide_tab_bar_when_single_tab = true
pane_borders = true
pane_gaps = true
EOF

HERDR_CONFIG_PATH="$config_root/config.toml" herdr config check
HERDR_CONFIG_PATH="$config_root/config.toml" \
  herdr plugin link "$plugin_root" --enabled >/dev/null

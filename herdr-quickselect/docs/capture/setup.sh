#!/bin/sh

set -eu

session_name=quickselect-showcase
demo_root=/tmp/herdr-quickselect-showcase
config_root=/tmp/herdr-quickselect-showcase-config
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
plugin_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)

HERDR_CONFIG_PATH="$config_root/config.toml" \
  herdr session stop "$session_name" >/dev/null 2>&1 || true
HERDR_CONFIG_PATH="$config_root/config.toml" \
  herdr session delete "$session_name" >/dev/null 2>&1 || true

rm -rf -- "$demo_root" "$config_root"
mkdir -p "$demo_root" "$config_root/bin"

cat >"$demo_root/release.txt" <<'EOF'
Artifact  /opt/herdr/plugins/quickselect-v0.3.0.tar.gz
Docs      https://herdr.dev/plugins/quick-select
Commit    4a8c2f97b61d
Runner    192.0.2.42
Build     2026082301
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
key = "ctrl+shift+x"
type = "plugin_action"
command = "choplin.quickselect.copy"
description = "Copy visible item"

# VHS does not deliver Ctrl+Shift+X reliably. This capture-only binding invokes
# the same plugin action without entering input in the terminal pane.
[[keys.command]]
key = "ctrl+x"
type = "plugin_action"
command = "choplin.quickselect.copy"
description = "Copy visible item"

[[keys.command]]
key = "ctrl+shift+p"
type = "plugin_action"
command = "choplin.quickselect.open-url"
description = "Open visible URL"

# VHS does not deliver Ctrl+Shift+P reliably. This capture-only binding invokes
# the same plugin action without entering input in the terminal pane.
[[keys.command]]
key = "ctrl+p"
type = "plugin_action"
command = "choplin.quickselect.open-url"
description = "Open visible URL"

[ui]
sidebar_start_collapsed = true
hide_tab_bar_when_single_tab = true
pane_borders = true
pane_gaps = true

[ui.toast]
delivery = "herdr"
delay_seconds = 0
EOF

cat >"$config_root/bin/open" <<'EOF'
#!/bin/sh

set -eu

test "$#" -eq 1
printf '%s\n' "$1" >/tmp/herdr-quickselect-showcase-config/opened-url
EOF

cat >"$config_root/bin/launch-herdr" <<'EOF'
#!/bin/sh

cd /tmp/herdr-quickselect-showcase

exec env -i \
  HOME="$HOME" \
  PATH="/tmp/herdr-quickselect-showcase-config/bin:$PATH" \
  TERM=xterm-256color \
  COLORTERM=truecolor \
  LANG=en_US.UTF-8 \
  LC_ALL=en_US.UTF-8 \
  PS1='\[\e[1;38;2;166;227;161m\]❯\[\e[0m\] ' \
  PS2='· ' \
  HERDR_CONFIG_PATH=/tmp/herdr-quickselect-showcase-config/config.toml \
  herdr --session quickselect-showcase
EOF
chmod +x "$config_root/bin/launch-herdr" "$config_root/bin/open"

HERDR_CONFIG_PATH="$config_root/config.toml" herdr config check

(
  cd "$plugin_root"
  TMPDIR="$config_root" GOCACHE="$config_root/go-build-cache" \
    GOMODCACHE=/tmp/herdr-quickselect-showcase-go-mod-cache \
    go build -trimpath -o herdr-quickselect ./cmd/herdr-quickselect
)
HERDR_CONFIG_PATH="$config_root/config.toml" \
  herdr plugin link "$plugin_root" --enabled >/dev/null

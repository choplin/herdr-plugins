# Herdr Plugins

This repository collects focused Herdr plugins, each built to solve one specific problem in
day-to-day Herdr workflows. Each plugin is published from its own repository and included here as a
Git submodule for unified local development.

## Clone

Clone the collection and all plugins:

```sh
git clone --recurse-submodules https://github.com/choplin/herdr-plugins.git
```

Initialize the plugins in an existing checkout:

```sh
git submodule update --init --recursive
```

## Plugins

### [Agent Update Time](https://github.com/choplin/herdr-agent-update-time)

See when each Agent last changed state directly in Herdr's Agents sidebar.

![Herdr Agents sidebar showing the last state-change time for each Agent](https://raw.githubusercontent.com/choplin/herdr-agent-update-time/main/docs/agent-update-time-annotated.png)

### [Next Agent](https://github.com/choplin/herdr-next-agent)

Move forward or backward through Agents in configured semantic states across all workspaces and
tabs.

![Moving forward and backward through blocked Agents](https://raw.githubusercontent.com/choplin/herdr-next-agent/main/docs/next-agent-demo.gif)

### [Quick Select](https://github.com/choplin/herdr-quickselect)

Select structured text with inline keyboard hints, then run configurable copy, open, or argv actions.

#### Copy

![Selecting and copying a visible terminal value with Herdr Quick Select](https://raw.githubusercontent.com/choplin/herdr-quickselect/main/assets/quickselect-copy.gif)

#### Open URL

![Selecting and opening a visible URL with Herdr Quick Select](https://raw.githubusercontent.com/choplin/herdr-quickselect/main/assets/quickselect-open-url.gif)

### [Repository Identity](https://github.com/choplin/herdr-repository-identity)

See the shared Git repository name for each workspace, including worktrees of the same repository.

![Herdr Spaces sidebar showing one shared repository name across different worktrees](https://raw.githubusercontent.com/choplin/herdr-repository-identity/main/docs/repository-identity-annotated.png)

### [Split Pane](https://github.com/choplin/herdr-split-pane)

Open a caller-provided command directly in a split pane without launching an intermediate login shell.

![Lazygit opening in a right split pane in Herdr](https://raw.githubusercontent.com/choplin/herdr-split-pane/main/docs/split-pane-demo.gif)

## Development

Work inside a plugin directory as in a regular repository. Commit and push the plugin change there,
then record its new commit in this collection:

```sh
cd herdr-quickselect
git switch main
# edit, test, commit, and push

cd ..
git add herdr-quickselect
# commit and push the updated submodule reference
```

Update every plugin to the latest commit on its configured `main` branch:

```sh
git submodule update --remote
```

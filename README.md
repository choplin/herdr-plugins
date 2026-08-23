# Herdr Plugins

This repository collects focused Herdr plugins, each built to solve one specific problem in
day-to-day Herdr workflows.

## Plugins

### [Agent Update Time](herdr-agent-update-time/)

See when each Agent last changed state directly in Herdr's Agents sidebar.

![Herdr Agents sidebar showing the last state-change time for each Agent](herdr-agent-update-time/docs/agent-update-time-annotated.png)

### [Next Agent](herdr-next-agent/)

Move forward or backward through Agents in configured semantic states across all workspaces and
tabs.

![Moving forward and backward through blocked Agents](herdr-next-agent/docs/next-agent-demo.gif)

### [Quick Select](herdr-quickselect/)

Select structured text with inline keyboard hints, then run configurable copy, open, or argv actions.

#### Copy

![Selecting and copying a visible terminal value with Herdr Quick Select](herdr-quickselect/assets/quickselect-copy.gif)

#### Open URL

![Selecting and opening a visible URL with Herdr Quick Select](herdr-quickselect/assets/quickselect-open-url.gif)

### [Repository Identity](herdr-repository-identity/)

See the shared Git repository name for each workspace, including worktrees of the same repository.

![Herdr Spaces sidebar showing one shared repository name across different worktrees](herdr-repository-identity/docs/repository-identity-annotated.png)

### [Split Pane](herdr-split-pane/)

Open a caller-provided command directly in a split pane without launching an intermediate login shell.

![Lazygit opening in a right split pane in Herdr](herdr-split-pane/docs/split-pane-demo.gif)

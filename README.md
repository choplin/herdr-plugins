# Herdr Plugins

This repository collects focused Herdr plugins, each built to solve one specific problem in
day-to-day Herdr workflows.

## Plugins

| Plugin | What it shows | Install |
| --- | --- | --- |
| [Agent Update Time](herdr-agent-update-time/) | The local time when each Agent's semantic state last changed | `herdr plugin install choplin/herdr-plugins/herdr-agent-update-time` |
| [Repository Identity](herdr-repository-identity/) | The shared Git repository identity for each workspace | `herdr plugin install choplin/herdr-plugins/herdr-repository-identity` |

Both plugins support Linux and macOS and require Herdr 0.8.2 or newer. Installation builds a
native binary, so Go 1.22 or newer must also be available on the machine performing the install.
Repository Identity additionally requires Git at runtime.

## Development

Enter the Nix development shell from the repository root:

```sh
nix develop
```

Run the test suite for each plugin from its directory:

```sh
cd herdr-agent-update-time
go test ./...

cd ../herdr-repository-identity
go test ./...
```

See each plugin's README for local linking and usage details.

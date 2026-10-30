# Environment

Environment variables that cairn reads. Flags win over variables where both apply.

## Variables

| Variable | Read by | Default when unset | Meaning |
|---|---|---|---|
| `CAIRN_HOME` | Every command that uses the registry or a machine-mode database | Platform data directory (see `storage.md`) | Data directory with `registry.json` and machine-mode databases. Used exactly as given; use an absolute path. An empty value counts as unset. |
| `CAIRN_PROJECT` | Every command that opens a project | No project named | Registered project name to use when `--project` is not given. Skips the walk from the current directory. An empty value counts as unset. |
| `HOME` | Default data directory on macOS and other systems | If empty and `CAIRN_HOME` is unset, cairn fails | Base of the default data directory. |
| `XDG_DATA_HOME` | Default data directory on systems other than macOS | `$HOME/.local/share` | Base of the default data directory: `$XDG_DATA_HOME/cairn`. Ignored on macOS. An empty value counts as unset. |
| `SUDO_UID` | Trust check on project-mode stores, only when the process runs as root | Unset | Numeric user ID that invoked `sudo`. A project store owned by this user is trusted by root. Other stores are refused. |

Precedence for the project name: `--project` flag, then `CAIRN_PROJECT`, then the walk from the current directory.

Precedence for the data directory: `CAIRN_HOME`, then the platform default.

## Examples

Use a project by name, without a flag:

```sh
export CAIRN_PROJECT=tidepool
cairn brief TP-03
```

Use a project for one command only:

```sh
CAIRN_PROJECT=tidepool cairn list --task TP-03
```

Keep the registry and machine-mode databases in a chosen folder:

```sh
CAIRN_HOME=/srv/cairn-data cairn init --name tidepool --mode machine --repos server,client
CAIRN_HOME=/srv/cairn-data cairn projects
```

Pass a project as a flag, which overrides `CAIRN_PROJECT`. Here the flag selects `tidepool` even though the variable names another registered project, `NAME`:

```sh
CAIRN_PROJECT=NAME cairn context --project tidepool
```

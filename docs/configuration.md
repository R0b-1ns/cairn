# Configuration

A project has five configuration keys. They are set once by `cairn init`, stored in the `project` table, and never change afterward.

## Keys

| Key | `init` flag | Default | Allowed values | Limits | Meaning |
|---|---|---|---|---|---|
| `name` | `--name` | none; required | Must match `^[a-z0-9][a-z0-9_-]{0,39}$`: lowercase letters, digits, `-`, `_`; first character a letter or digit | 1-40 characters; unique among registered projects | Name used with `--project` and `CAIRN_PROJECT`. Folder name for machine mode. |
| `mode` | `--mode` | `project` | `project`, `machine` | One of the two values | `project`: database in `<root>/.cairn/`. `machine`: database in the data directory. See `storage.md`. |
| `context` | `--context` | empty | Any text. Newline and tab allowed | At most 2000 characters; no other control characters | Free text describing the project. Printed by `cairn context`. |
| `repos` | `--repos` (comma-separated; spaces trimmed; empty items dropped) | none; required | At least 1 and at most 50 names, each matching `^[a-z0-9][a-z0-9._-]{0,39}$` | 1-40 characters per name | The only repo names a note may use in `repo` or `impacts`. |
| `task_pattern` | `--task-pattern` | `^(?P<base>[A-Z]{2,5}-\d{1,4})(?:\.(?P<ext>\d{1,3}))?$` | A Go (RE2) regular expression with a named group `base`. A named group `ext` is optional. | At most 200 bytes | Recognizes task IDs. See below. |

The flag `--path` sets the project root folder. It is not a configuration key.

`--mode` is also validated by `init`: any value other than `project` or `machine` is refused with exit 3.

## Task pattern

A task ID is any string that the pattern matches. Matching uses Go's regular expression engine (RE2), which runs in linear time.

- The pattern must compile and must contain the named group `(?P<base>...)`. The group `(?P<ext>...)` is optional.
- The `base` group becomes the note's `task_base`. The `ext` group, when it matched, becomes `task_ext`. The whole ID is stored as `task`.
- A pattern is not anchored by cairn. Write `^` and `$` yourself, or a longer ID that contains a valid task will match too.
- Keep the `base` group from matching an empty string. cairn does not check this.

### Examples with the default pattern

| ID | Matches | `base` | `ext` |
|---|---|---|---|
| `TP-03` | yes | `TP-03` | none |
| `TP-3` | yes | `TP-3` | none |
| `TP-0003` | yes | `TP-0003` | none |
| `TP-03.1` | yes | `TP-03` | `1` |
| `TP-03.001` | yes | `TP-03` | `001` |
| `ABCDE-1234.999` | yes | `ABCDE-1234` | `999` |
| `tp-03` | no | | |
| `TP03` | no | | |
| `T-03` | no | | |
| `ABCDEF-03` | no | | |
| `TP-12345` | no | | |
| `TP-03.1000` | no | | |
| `TP-03.` | no | | |
| `TP-03.1.2` | no | | |
| ` TP-03` | no | | |

### Example of a custom pattern

```sh
cairn init --name tidepool --repos server,client \
  --task-pattern '^(?P<base>[A-Z]+-\d+)(?:/(?P<ext>[a-z]+))?$'
```

| ID | Matches | `base` | `ext` |
|---|---|---|---|
| `TP-3` | yes | `TP-3` | none |
| `TP-3/api` | yes | `TP-3` | `api` |
| `TP-3/API` | no | | |
| `tp-3` | no | | |

## Example: init

Project mode (default):

```sh
cairn init --name tidepool --repos server,client --context "Tide data service."
```

Machine mode:

```sh
cairn init --name tidepool --mode machine --repos server,client
```

Output on success:

```
initialized project tidepool (project mode) at /Users/example/Project/tidepool
database: /Users/example/Project/tidepool/.cairn/cairn.db
```

## Immutability

- The `project` row is protected by immutability triggers. No command changes it.
- To change a value, create a new project with the new configuration and import the notes. `import` validates each note against the receiving project's `repos` and `task_pattern`, so a note that does not fit is refused.
- `cairn context` shows the values. `cairn context --json` prints the project object with the keys `name`, `mode`, `context`, `repos`, `task_pattern`.

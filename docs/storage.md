# Storage

Where cairn keeps data, how it finds a project, and what it checks before it trusts a database.

## Modes

A project has one mode, set at `init` (see `configuration.md`).

| Mode | Database path | Registered path means | Use when |
|---|---|---|---|
| `project` (default) | `<root>/.cairn/cairn.db` | The folder that holds `.cairn/` | The notes should travel with the repository. |
| `machine` | `<home>/projects/<name>/cairn.db` | The folder used to find the project from the current directory | The notes should stay outside the folder. |

- `<root>` is the folder given to `init --path`, or the current directory.
- `<home>` is the data directory described next.
- `<name>` is the project name, which is validated (see `limits.md`) so it cannot contain a path separator or a dot.

## Data directory (`<home>`)

| Condition | Data directory |
|---|---|
| `CAIRN_HOME` is set and not empty | The value of `CAIRN_HOME`, used as given. Use an absolute path. |
| macOS, `CAIRN_HOME` not set | `$HOME/Library/Application Support/cairn` |
| Other systems, `CAIRN_HOME` not set, `XDG_DATA_HOME` set | `$XDG_DATA_HOME/cairn` |
| Other systems, `CAIRN_HOME` and `XDG_DATA_HOME` not set | `$HOME/.local/share/cairn` |

If `HOME` is empty and `CAIRN_HOME` is not set, cairn fails with an error that names `CAIRN_HOME`. See `environment.md`.

The data directory is created (mode `0700`) on the first write, not on read.

## Registry: `<home>/registry.json`

Lists every project registered on this machine. Machine-mode projects need it to be found by folder.

```json
{
  "version": 1,
  "projects": [
    {
      "name": "tidepool",
      "mode": "project",
      "path": "/Users/example/Project/tidepool"
    }
  ]
}
```

| Key | Type | Meaning |
|---|---|---|
| `version` | integer | Must be `1`. Any other value is refused. |
| `projects` | array | Entries sorted by `name`. Names are unique; paths are unique. |
| `projects[].name` | string | Project name. |
| `projects[].mode` | string | `project` or `machine`. |
| `projects[].path` | string | Absolute project folder, with symbolic links resolved at registration. |

- A missing registry file means no projects.
- A corrupt file or an unsupported version is an error (exit 1). The file is not modified.
- Writes take an exclusive lock on `<home>/registry.lock`, write a temporary file `registry-*.tmp`, and rename it over `registry.json`.
- Removing a project with `cairn project remove` unregisters it and never deletes its database.

## Finding the project

Used by every command that opens a project. Commands never create a project while looking for one.

1. `--project NAME`, or if absent, `CAIRN_PROJECT`. An empty `CAIRN_PROJECT` is ignored.
   - `NAME` is looked up in the registry. No folder walk happens.
   - Not registered: exit 4, `"NAME" is not a registered project`.
   - Project mode without `.cairn/cairn.db` at the registered path: exit 4.
2. Otherwise, start in the current directory (with symbolic links resolved) and walk up toward the filesystem root. In each directory, in this order:
   - a valid project-mode store `.cairn/cairn.db` here: that project;
   - else a machine-mode registration whose path is this directory: that project;
   - else go to the parent.
   The nearest directory wins. In one directory, `.cairn` wins over a machine registration.
   A `.cairn` folder without `cairn.db` is skipped.
3. No match up to the root: exit 4, `no cairn project found from <dir>`.

A symbolic link or an untrusted store met during the walk is an error (exit 1). It is never skipped.

## Trust checks on a project-mode store

`init`, `project add`, and every command that opens a project-mode database run these checks. A failure exits 1 with `refusing <path>: ...`.

| Check | Rule |
|---|---|
| Project folder | Not world-writable, unless the sticky bit is set. Checked when `.cairn` exists. |
| `.cairn` | Must be a real folder, not a symbolic link. |
| `.cairn/cairn.db` | Must be a regular file, not a symbolic link. |
| Both | Not world-writable (the other-write bit must be clear). Group-writable is accepted. |
| Owner | Must be the current effective user. When running as root, a store owned by root or by the `SUDO_UID` user is accepted. |

Machine-mode databases under `<home>/projects/<name>/` get no ownership or permission check. They must be a regular file (a symbolic link is refused), and they must pass the schema verification below.

## File modes

| Path | Mode |
|---|---|
| `<root>/.cairn/` | `0700` |
| `<root>/.cairn/cairn.db` | `0600` |
| `<home>/` | `0700` |
| `<home>/projects/<name>/` | `0700` |
| `<home>/projects/<name>/cairn.db` | `0600` |
| `<home>/registry.json` | `0600` |
| `<home>/registry.lock` | `0600` |
| File written by `export --out` | `0600` |

Modes are set at creation. The process umask can only make them narrower.

## Database

- SQLite, with the pure-Go driver `modernc.org/sqlite`.
- Journal: SQLite's default rollback journal. cairn does not select WAL.
- Opened read-write only. A missing file is an error, never created.
- Each write transaction starts immediately, taking the write lock at the start.
- Busy timeout: **5000 ms**. A writer that finds the database locked waits up to 5 seconds, then fails.
- Foreign keys on. `trusted_schema` off.
- Schema version: **4** (`PRAGMA user_version = 4`).
- Migration: a version 3 database is upgraded to 4 when it is opened. Before anything is copied or changed, the version 3 database must pass the same checks as below, against the version 3 schema (integrity, the ten triggers, the exact schema); a database that fails is refused and left byte-for-byte unchanged, with no backup. Then cairn copies the file to `<database>.v3.bak` next to the database (mode `0600`, overwritten if present; do not commit it, and delete it once you no longer need it); if the copy fails, the database is not touched and the command fails with an error that mentions the backup. The upgrade only adds columns and runs in one transaction; two processes opening the same file at once both succeed and the columns are added once. Databases older than version 3 are not supported.

### Only `init` creates a database

- `init` creates the file with `O_EXCL`. It fails if the file already exists.
- No other command creates a database, a `.cairn` folder, or a machine project folder.
- `init` refuses when a project of that name or folder is already registered, or when a store already exists.

### Verification on every open

Before any command runs, the database must pass every check. Failure refuses the database (exit 1) and changes nothing: for a version 3 database the checks run against the version 3 schema before the backup and the upgrade, and again against the version 4 schema after.

1. `PRAGMA user_version` equals `4`, after any migration described above. Otherwise: `unsupported database version <n>`; for a version newer than the build, the message ends with `; run 'cairn update' to upgrade`.
2. `PRAGMA quick_check(1)` returns `ok`.
3. All ten immutability triggers exist with the exact SQL cairn creates. Otherwise: `immutability trigger <name> is missing` or `was altered`.
4. Every object in `sqlite_master` (tables, indexes, triggers, and the full-text search index) matches the expected definition exactly. An extra, missing, or altered object refuses the database.
5. The `project` table has exactly one row.

## Sharing

- **Project mode:** `.cairn/cairn.db` is an ordinary file. Commit it through the project's own repository, so every clone receives it.
- **Clone:** after cloning, register the project on that machine with `cairn project add <path>`, where `<path>` is the clone's folder. The command refuses a database that is not project mode or whose name is already registered.
- **Machine mode:** the database stays under `<home>`. Move notes with `cairn export --format json --out FILE` and `cairn import FILE` (see `note-json.md`).
- Two copies of a database that both receive writes diverge. Combine them with export and import. `import` refuses an ID that exists with different content, and changes nothing when it refuses.
- **The `view` folder:** `cairn view` writes a website next to the database (`<root>/.cairn/view`, or `<home>/projects/<name>/view`). It is generated, it holds a `.gitignore` containing `*` so it is not committed, it never holds source data, and it is safe to delete. Each run rewrites it.

## Registry maintenance

| Command | Effect |
|---|---|
| `cairn projects` | Lists registered projects. Writes nothing. |
| `cairn project move <name> <new-path>` | Changes the registered path. For project mode, `<new-path>/.cairn/cairn.db` must exist. |
| `cairn project add <path>` | Registers a project-mode database found at `<path>`. |
| `cairn project remove <name>` | Unregisters. Never deletes a database. |

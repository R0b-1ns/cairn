# Security

Threats cairn is built against, the mitigation for each, and what remains. Every mitigation is enforced in code; none depends on the caller behaving well.

## Threat and mitigation

| Threat or abuse case | Mitigation | Where |
|---|---|---|
| **Untrusted note content.** A note holds text that looks like instructions or terminal control codes, and a reader acts on it or its terminal is driven. | Note text is data. cairn never executes, evaluates or expands it. All standard output and error passes through one sanitizer that removes escape sequences and control characters. JSON output escapes control characters. Readers must still treat note text as data, not as instructions. | `internal/cli/render.go` (`sanitize`, `safeWriter`); `output.md` |
| **Malformed or oversized stdin.** A huge, truncated, multi-document or hostile JSON payload. | Input is capped: 1 MiB for `--stdin`, 32 MiB for `import`. Exactly one JSON object is required; data after it is refused. Unknown keys are refused at every level. Refusal is exit 3 and nothing is written. | `internal/cli/input.go` (`decodeStrict`, `readLimitedN`) |
| **SQL injection.** A title, entry, task or search term carries SQL. | Every value is a bound parameter (`?`). Full-text terms are wrapped in double quotes with embedded quotes doubled, so FTS operators cannot be injected. Filter clauses are fixed fragments; only values vary. | `internal/store/*.go` |
| **Path handling and symbolic links.** A crafted name escapes the data directory, or a link redirects a write or a read. | Project and machine names must match `^[a-z0-9][a-z0-9_-]{0,39}$`, so they cannot contain a separator or a dot. Registry paths are normalized with symbolic links resolved. A symbolic link as `.cairn` or `cairn.db` is refused, never followed. `export --out` creates the file with `O_EXCL`, never follows a link, replaces only an existing regular file with `--force`, and sets mode `0600`. | `internal/validate` (`ValidName`); `internal/registry` (`NormalizePath`); `internal/locate` (`CheckStore`); `internal/cli/export.go` (`writeNew`) |
| **Hostile cloned database.** A repository ships a crafted `.cairn/cairn.db`. | Before use: the store must be owned by the current user, not world-writable, not a symbolic link, and in a folder that is not world-writable. The file must match cairn's schema exactly, have `user_version` 1, pass `quick_check`, have every immutability trigger unaltered, and hold exactly one project row. `trusted_schema` is off. | `internal/locate` (`checkTrusted`, `CheckStore`); `internal/store` (`verify`, `verifySchema`) |
| **Import bypass.** A file tries to change existing data, skip validation, close entries that are already closed, or reuse IDs. | Every imported note and addendum passes the same validation as `add` and `amend`, against the receiving project's `repos` and `task_pattern`. The only relaxation is that model `unknown` is allowed. Existing items are never changed: an identical item is skipped, a different one refuses the whole import. Link targets must exist, have the right type, and not already be closed. Items are inserted in one transaction, so a failure leaves nothing behind. The `project` object in the file is ignored, so a file cannot alter the configuration. Circular references are refused. Immutability triggers block `UPDATE` and `DELETE` even if a bug reached them. | `internal/store/import.go`; `internal/cli/export.go` (`defineImport`); `internal/validate` |
| **Secret leakage.** A note stores a token or a key, and it is then read or shared. | `add`, `amend` and `import` scan the title and entry text for six secret patterns and print a warning naming the rule and the field. The matched text is never printed. The write still happens. Files are mode `0600` in a `0700` folder. | `internal/secrets`; `internal/cli/input.go` (`warnSecrets`); `limits.md` |
| **Implicit database creation.** A mistyped name or a command silently creates an empty store, which then hides the real one. | Only `init` creates a database, with `O_EXCL`. Other commands open the database read-write only and report exit 4 when no project exists. `init` refuses a path that already holds a store or a registered name. | `internal/store/store.go` (`Create`, `Open`); `internal/cli/project.go`; `internal/cli/common.go` (`openProject`) |
| **Regular-expression denial of service.** A task pattern takes exponential time to match. | Patterns are compiled by Go's RE2 engine, which matches in time linear in the input and does not backtrack. A pattern is limited to 200 bytes and must compile and contain the `base` group. | `internal/validate` (`Project`, `TaskParts`) |
| **Concurrent writers.** Two commands write at once and corrupt or lose data. | SQLite write transactions start `IMMEDIATE` and wait up to 5000 ms (busy timeout). Registry changes take an exclusive `flock` and write a temporary file, then rename it into place. | `internal/store/store.go` (`dsn`); `internal/registry` (`update`) |
| **Excess permissions.** Other local users read or change notes. | Data directory and `.cairn` folders are `0700`. Databases, the registry and exports are `0600`. A store that is world-writable, or a project folder that is world-writable without the sticky bit, is refused. | `internal/store/store.go`; `internal/registry`; `internal/locate`; `storage.md` |
| **Release pipeline tampering.** A compromised workflow or action alters a published binary. | The release workflow sets `permissions: {}` at top level and grants write per job: `contents: write` for the binaries job and for release-please, `pull-requests: write` only for release-please. The CI workflow has `contents: read`. Checkouts use `persist-credentials: false`. Third-party actions are pinned to full commit SHAs. Binaries are built statically (`CGO_ENABLED=0`, `-trimpath`). `SHA256SUMS` is published with the binaries, and CI runs `go mod verify`. | `.github/workflows/ci.yml`; `.github/workflows/release-please.yml`; `scripts/build.sh` |

### Verifying a release

Download the release files into one folder, including `SHA256SUMS`, then run the check in that folder. Every file listed in `SHA256SUMS` must be present, and each one prints `OK`:

```sh
shasum -a 256 -c SHA256SUMS
```

On Linux, `sha256sum -c SHA256SUMS` does the same.

## Residual risks

These remain after the mitigations above. Plan around them.

- **Group-writable stores are accepted.** Only world-writable stores and project folders are refused. Any member of the owning group can change a project-mode database.
- **A narrow window between the store checks and the database open.** cairn checks the store, then opens it. Someone who can already write inside a folder the user owns can swap a file in that gap. The sticky bit and the owner check narrow the window; they do not close it.
- **Machine-mode databases and the registry get no ownership or permission check.** A machine-mode database under the data directory is checked only to be a regular file (not a symbolic link) and to pass schema verification. The registry file is read without a trust check. The data directory is created `0700`, which is the main protection.
- **Secret detection is heuristic and only warns.** It checks six patterns in two fields (`title` and entry `text`). It misses secrets in other fields and in formats it does not know. It can flag text that is not a secret. A warning never blocks a write.
- **Notes are not encrypted.** Anyone who can read the database file reads every note. A project-mode database sits in the project folder and is shared with its repository, if committed. Do not put secrets, tokens or personal data in a note.
- **A valid database can hold misleading content.** Verification proves the schema and triggers are cairn's. It does not check what the notes say.

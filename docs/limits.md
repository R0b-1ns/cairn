# Limits

Every limit cairn enforces, with its exact value. A value over a limit is refused with the exit code shown (see `exit-codes.md`).

## Input size

| Limit | Value | Applies to | Over the limit |
|---|---|---|---|
| Standard input | 1 MiB (1048576 bytes) | `add --stdin`, `amend <id> --stdin`, `check --done-file` | `add`, `amend`: exit 3 (`input too large`). `check --done-file`: exit 1. |
| Import file | 32 MiB (33554432 bytes) | `import <file>` (and `import -`) | Exit 3. |

Reads stop at the limit plus one byte, so an oversized input is never held in memory whole.

## Text fields

| Field | Maximum | Unit | Required | Line rule |
|---|---|---|---|---|
| `title` | 200 | characters | yes, not blank | one line |
| entry `text` | 4000 | characters | yes, not blank | newline and tab allowed |
| project `context` | 2000 | characters | no | newline and tab allowed |
| `agent` | 80 | characters | yes, not blank | one line |
| `model` | 80 | characters | yes, not blank | one line |
| `task` | 80 | characters | no | one line |
| `task_pattern` | 200 | **bytes** | (set by `init`) | a regular expression |
| project `name` | 40 | characters | yes | must match `^[a-z0-9][a-z0-9_-]{0,39}$` |
| repo (in `repos`) | 40 | characters | yes, at least one repo | must match `^[a-z0-9][a-z0-9._-]{0,39}$` |

"Characters" means Unicode code points, not bytes. `task_pattern` is the one field measured in bytes.

Entry text equal to `none` (ignoring case and surrounding spaces) is refused. Blank entry text is refused as `is required`.

## Counts

| Limit | Value | Applies to |
|---|---|---|
| Entries | 100 | Per note, and per addendum. |
| `impacts` | 50 items | Per note. |
| `commits` | 50 items | Per note. |
| `resolves` | 50 items | Per note, and per addendum. |
| `supersedes` | 50 items | Per note, and per addendum. |
| `repos` | 50 names | Per project. At least one name is required. |
| Repeated `--decision`, `--change` and other entry flags | No cairn limit | Subject to the 100-entry limit. |

## Output counts

| Option | Default | Minimum | Maximum |
|---|---|---|---|
| `--max` (on `list`, `search`, `brief`, `check`) | 50 | 1 | None |
| `recent [N]` | 10 | 1 | None |
| `check --older-than` | 14 days | 0 | None |

`--max` caps the printed text lines of each list. JSON output is also capped for `list`, `search`, and `check`, with no "more" marker. See `output.md`.

## Formats

| Value | Rule |
|---|---|
| Note, addendum and entry IDs | `n-`, `a-`, `e-` followed by exactly 6 characters from `0123456789abcdefghjkmnpqrstvwxyz`. |
| Commit hash | Lowercase hexadecimal, 7 to 64 characters: `^[0-9a-f]{7,64}$`. Uppercase is refused. |
| Repo in `repo` and `impacts` | Must be exactly one of the project's `repos`. |
| Task | Must match the project's `task_pattern`. |
| `repo` values of an export | Same as above, checked against the receiving project on import. |

## Control characters

Refused in every text field, single-line or multi-line:

- U+0000 to U+001F (includes NUL, tab and newline, except as noted below),
- U+007F to U+009F,
- U+2028 (line separator) and U+2029 (paragraph separator),
- any byte sequence that is not valid UTF-8.

Exception: **newline (U+000A) and tab (U+0009) are allowed only in project `context` and in entry `text`.** In `title`, `agent`, `model`, `task`, names and repos they are refused.

CRLF: see `note-json.md`.

## Secret warnings

cairn scans for six patterns. A match prints a warning and does **not** stop the write. The matched text is never printed.

| Rule | Matches |
|---|---|
| `aws-access-key` | `AKIA` or `ASIA` followed by 16 characters from `0-9A-Z`. |
| `private-key` | A line `-----BEGIN ... PRIVATE KEY-----`. |
| `github-token` | `gh` plus one of `p o u s r`, `_`, then 36 or more letters or digits; or `github_pat_` then 22 or more letters, digits or underscores. |
| `slack-token` | `xox` plus one of `a b p r s`, `-`, then 10 or more letters, digits or hyphens. |
| `jwt` | `eyJ`, then three segments of 8 or more URL-safe characters, separated by dots. |
| `credential-assignment` | (case-insensitive) `password`, `passwd`, `secret`, `token`, `api_key`, `api-key` or `apikey`, optional quote, optional spaces, `:` or `=`, optional spaces or quote, then 8 or more characters that are not whitespace or quotes. |

Scanned fields: `title` and each entry `text`, on `add`, `amend` and `import`. Other fields (`agent`, `task`, `context`, and so on) are not scanned.

Warning format on standard error:

```
cairn: warning: possible secret (aws-access-key) in entries[0].text; notes are stored unencrypted and may be shared
```

The scan is a heuristic. A missed secret is not an error, and a match is not proof of a secret. See `security.md`.

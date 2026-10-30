# cairn search

Full-text search over note titles and entries. All terms must match.

## Usage

```
cairn search <term> [<term>...] [--max N] [--project NAME] [--json]
cairn search -- <term> [<term>...]
```

## Arguments

| Argument | Required | Meaning |
|---|---|---|
| `<term>` | yes, at least one | A word or phrase to find. Each argument is split on whitespace, so `"tidal index"` is two terms. Terms are matched as literal text; FTS operators and punctuation have no special meaning. |

## Flags

| Flag | Default | Allowed values | Meaning |
|---|---|---|---|
| `--max` | `50` | integer, 1 or more | Maximum hits printed. The rest is counted in `... N more (raise --max)`. |
| `--project` | (empty) | a registered project name | Project to search. Default: found from the current directory, or env `CAIRN_PROJECT`. |
| `--json` | `false` | flag, no value | Print a JSON array instead of lines. |

## Output

One line per hit: ID, kind, snippet, and the note (and addendum) that holds it.

```
e-7h2m9q  decision Index buckets are hourly, to match the sensor rate.  (n-4k7q2m)
n-4k7q2m  title    Add tidal index  (n-4k7q2m)
e-8k3m5p  note     Tidal index rebuild takes about 40 s on the full history.  (n-4k7q2m via a-9x3b2c)
... 4 more (raise --max)
```

The kind column is `title` for a note title and the entry type for an entry. The snippet is an excerpt of the matched text, clipped to 160 characters; `…` marks an excerpt cut inside the text. With no match the output is `no matches`.

`--json` prints an array of objects with the keys `kind` (`note` or `entry`), `id`, `note_id`, `addendum_id`, `type` (empty for a title), and `snippet`. `--max` applies. An empty result is `[]`.

## Exit codes

- `0`: searched (including `no matches`).
- `1`: runtime error.
- `2`: usage error: no terms, or `--max` below 1.
- `4`: no project found.

## Examples

```
cairn search tidal index
```

Prints the hits that contain both `tidal` and `index`. Exit code 0.

```
cairn search -- -retry
```

Searches for the term `-retry`. The `--` ends flag parsing; without it, `-retry` is read as a flag and the command exits 2.

```
cairn search "sensor feed" --max 5 --json
```

Prints up to 5 hits as JSON; `"sensor"` and `"feed"` must both match. Exit code 0.

## Notes

- Put `--` before any term that starts with `-`. Flags may appear before or after the terms; `--` stops flag parsing.
- Each term matches whole words only: there is no prefix or wildcard matching. A search for `tidal` does not match `tidally`.
- Search reads note titles and entry text, including addendum entries. Use `cairn show <id>` on a hit to read the full text.

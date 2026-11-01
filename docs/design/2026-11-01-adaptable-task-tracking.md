# Adaptable task tracking

## Intent

Today a note's `task` is an optional string checked against a regular expression chosen at `init` (default `TP-03` shape). That fits a team that numbers its tasks by hand. It fits badly three other people:

- A solo developer who asks an AI for feature X, then Y, then many more, and has no tracker. They want cairn to number the work for them, so they can say "resume 7" or "7 is blocked".
- A team that already uses Jira, Linear or GitHub issues. Their IDs already exist; cairn should accept them, link back to the ticket, and never ask them to invent a second numbering.
- A reader of the notes (human, or the `view` site) who wants to know who wrote a note and on which branch.

This design makes task tracking adapt to all three without breaking any existing project.

A second design, a shared PostgreSQL backend for teams, is written later and builds on this one. It is out of scope here. Everything below must keep working when the storage changes: the next number is claimed inside the same transaction that inserts the note.

## Goals

1. `init` accepts a task mode: `auto`, `jira`, `github`, `linear` or `custom`. A mode other than `custom` replaces the hand-written regex.
2. Mode `auto`: cairn assigns the numbers (`T-1`, `T-2`, ...).
3. A note can carry a `ref`, the URL of its ticket. The project can hold a URL template so the user does not retype it.
4. Every note and every addendum records who wrote it (the human) and the git branch it was written on. Both appear on the note card and in `view`, and the branch is sortable and filterable on the `view` site.
5. Existing projects keep working exactly as before, including the ones with a custom `task_pattern`.

## Non-goals

- No call to Jira, Linear or GitHub. cairn stores and displays an URL; it never contacts a tracker and holds no tracker credentials.
- No changing the mode or pattern of an existing project (the configuration is write-once, as today).
- No per-branch numbering. The branch is data, never part of an ID.
- No shared database in this design (see above).

## Anti-goals

- A project created with the old `--task-pattern` behaving even slightly differently afterward: accepting or refusing different IDs, extension rules, `brief`/`check` selection.
- A migration that can lose or alter an existing note.

## Task modes

A mode is a named, fixed regular expression. At `init`, cairn stores the resulting regex in the existing `task_pattern` column. The validator keeps reading only `task_pattern`, as today, so the mode is not on the validation path. The mode is stored separately and used for three things: `auto` numbering, the `ref` URL template, and display.

| Mode | Regex stored | Example IDs |
|---|---|---|
| `auto` | `^(?P<base>PREFIX-\d{1,6})(?:\.(?P<ext>\d{1,3}))?$` | `T-7`, `T-7.1` |
| `jira` | `^(?P<base>[A-Z][A-Z0-9]{1,9}-\d{1,6})(?:\.(?P<ext>\d{1,3}))?$` | `PROJ-123`, `PROJ-123.1` |
| `linear` | `^(?P<base>[A-Z]{1,10}-\d{1,6})(?:\.(?P<ext>\d{1,3}))?$` | `ENG-42` |
| `github` | `^(?P<base>#\d{1,7})(?:\.(?P<ext>\d{1,3}))?$` | `#128`, `#128.1` |
| `custom` | the user's `--task-pattern`, unchanged | anything |

- The flags are `--tasks MODE`, `--task-prefix`, `--task-url`, `--task-pattern`, and `--no-input`.
- `--task-pattern` given alone means `custom`. Given together with another mode, `init` refuses (exit code 2).
- `auto` takes `--task-prefix` (default `T`, matching `[A-Z]{1,5}`), used in the stored regex.
- Extensions (`T-7.1`) work as today in every mode: `T-7` selects `T-7` and its extensions.
- The limits of `limits.md` still apply: `task` is at most 80 characters, `task_pattern` at most 200 bytes.

### Interactive `init`

Everything stays possible with flags. When a person runs `init` in a terminal, cairn asks the task questions that the flags did not already answer.

- **When it prompts:** standard input and standard error are both terminals and `--no-input` was not given. Otherwise it never prompts. It asks only the questions whose flag was not given, so `cairn init --name foo` in a terminal asks everything else, and a command with every flag asks nothing.
- **Every question shows its default in brackets, and Enter accepts it.** (The user asked for "space"; Enter on an empty line is the usual way and is what this design uses. A line of only spaces also counts as empty.) A question with no default is asked again until it has an answer, and an invalid answer is explained and asked again.
- **Order and defaults:**
  1. Project name. Default: the lowercased name of the current folder if it is valid, otherwise none.
  2. Storage mode, `project` or `machine`, each with its meaning. Default `project`.
  3. Project root folder. Default: the current directory.
  4. Repos, comma-separated. No default, except the project name when the folder looks like a single repo (a `.git` entry is present) — a suggestion, shown as the default.
  5. Default repo, only when there are several repos. Default: none.
  6. Context, one line, optional. Default: empty.
  7. The task questions below.
- **Existing rules apply unchanged** to every answer (name pattern, uniqueness, repo pattern, limits).
- **Prompts go to standard error**, answers are read from standard input, and the standard output stays the same as today. No new dependency: terminal detection uses the standard library (`os.Stdin.Stat()` and `ModeCharDevice`).
- **Question 1, the numbering.** Each choice is shown with what it is for, and the first one is the default (Enter accepts it):
  1. `auto`: "cairn numbers your work for you (T-1, T-2, ...). Pick this if you have no ticket tracker, or you work alone."
  2. `jira`: "Use your Jira keys (PROJ-123). cairn does not talk to Jira; it only checks the format and can link to the ticket."
  3. `linear`: "Use your Linear IDs (ENG-42)."
  4. `github`: "Use GitHub or GitLab issue numbers (#128)."
  5. `custom`: "Use your own format, given as a regular expression."
- **Question 2, only for `auto`: the tag.** "Which tag do you want? Enter keeps `T` (T-1, T-2, ...), or type your own letters, for example `FEAT` (FEAT-1)." The answer is validated as `[A-Z]{1,5}`; an invalid answer is explained and asked again.
- **Question 3, only for `custom`:** the regular expression, validated with the existing rules and asked again on error.
- **Question 4, only for `jira`, `linear`, `github`:** an optional ticket URL template, with an example shown. Empty means none.
- A closing line shows the equivalent flags (`cairn init --tasks auto --task-prefix FEAT ...`), so the user can repeat it in a script.
- End of input (Ctrl-D) or Ctrl-C during a prompt cancels `init` without writing anything (exit 1).
- The flags `--name`, `--repos` and the rest keep working exactly as today.

### Default without a terminal

When `init` runs without a terminal (a script, CI, an agent) and no task flag is given, the project gets the **legacy default**: mode `custom` with the old default pattern, exactly as today. cairn prints one line on standard error saying that numbering was not chosen and naming `--tasks`. This keeps every existing script working unchanged and makes `TP-03` in current docs still true for non-interactive use. Docs and README examples that run `init` say which mode they use.

### Instructions for AI agents

An agent cannot answer prompts, so the docs the agent reads must tell it what to do. This goes in `docs/guide.md` (printed by `cairn guide`) and in the README section for agents:

- Before running `cairn init`, ask the user which numbering they want: `auto` (cairn numbers the work), or the tracker they use (`jira`, `linear`, `github`), or `custom`. Explain each in one sentence, as in the interactive prompt.
- If the user wants `auto`, propose the default `T` (`T-1`, `T-2`, ...) and let them choose their own letters instead.
- Run `init` with the answers as flags (`--tasks`, `--task-prefix`, `--task-url`). Never rely on the default without asking.
- When the user chose `auto`, create each new piece of work with `add --new-task` and continue it with `--task <ID>` from the number that `add` printed.

A test checks that `guide.md` contains these instructions, in the same way `docs_test.go` checks other docs.

## Auto numbering

- `cairn add --new-task ...` assigns the next number and prints it (and returns it in `--json`). Without `--new-task`, `--task T-7` continues task 7, as today.
- The next number is `max(number part of every task_base) + 1`, over all notes, for the project's prefix; the first one is 1. No counter is stored, because rows are immutable.
- The number is claimed by the note insert itself, inside one write transaction (`BEGIN IMMEDIATE` on SQLite), so two writers cannot get the same number.
- The next number looks only at notes whose `task_base` matches the project's prefix and parses the number from the base.
- On `add --stdin`, the JSON takes an optional boolean `new_task`, mutually exclusive with `task` (both given: exit 3). `amend` is unaffected: an addendum has no task of its own and inherits its note's.
- `--task T-99` for a number that does not exist yet is accepted in `auto` mode (the user knows what they are doing, for example after an import). `--new-task` and `--task` together give exit code 2.
- Numbers are not zero-padded and are never reused. A gap is normal.
- `--new-task` outside `auto` mode gives exit code 2 with a message naming the project's mode.

## The `ref` link

- `add --ref URL` stores an optional `ref` on the note, validated as described under Author and branch.
- `init --task-url 'https://x.atlassian.net/browse/{task}'` stores a template. It must contain `{task}`, start with `http://` or `https://`, be one line and at most 500 characters; otherwise exit 3. When a note has a task but no `ref`, display commands compute the link from the template. `{task}` is replaced by the base task without a leading `#`, so `#128` gives `128` and `.../issues/{task}` yields a working link. An extension is ignored: `PROJ-12.1` links to `PROJ-12`. A stored `ref` always wins over the template.
- `brief`, `show`, `export` and `view` print the link when there is one. The template is stored only in the project row, so it cannot be changed later (same rule as the other keys).

### Linking tickets later (for example after opening the project to the public)

A project started with `auto` (`T-1`, `T-2`, ...) can later gain GitHub issues, or a Jira, without changing its mode. The mode is write-once, but a `ref` is not tied to it:

- `amend` gains `--ref URL`, and the addendum JSON gains `ref`. An addendum that carries only a `ref` is valid (today an empty addendum is refused with exit 3; a `ref` counts as content). So `cairn amend n-4k7q2m --agent human --model none --ref https://github.com/acme/tidepool/issues/128` links issue 128 to the task of that note without touching anything else.
- A task can have several refs: its notes and addenda may each carry one. `brief`, `show`, `export` and `view` show all refs of the task, newest first, deduplicated. A task's refs are those of every note and addendum with the same `task_base`.
- Reverse lookup: `list --ref URL` finds the notes of the task that carries that URL, so someone who starts from issue 128 can find T-7. The match is exact on the stored URL.
- When the project later also wants a URL template for its numbering, that is not possible (write-once). Individual refs cover that need.
- Nothing else about the project changes: the old IDs stay valid and notes already written stay untouched.

## Author and branch

Both are set by cairn at write time, like `at`, on notes and on addenda.

- **author**: stored as two columns, `author_name` and `author_email`. Read from `git config user.name` / `user.email` in the current directory. Override with `--author "Name <email>"` or the env `CAIRN_AUTHOR` (same format). A value without `<...>` is a name only. If git has a name but no email, the email is empty. If nothing is available, both are empty and the write still succeeds. This is the human; the existing `agent` field remains the AI.
- **Validation** (on `add`, `add --stdin` and `import` alike; invalid values are refused with exit 3, never sanitized): `author_name` at most 80 characters, `author_email` at most 120, `branch` at most 200, `ref` at most 500; all one line, with no control characters. `ref` must start with `http://` or `https://`. Add these to `limits.md`.
- **Filters.** `--author VALUE` on `list` and `brief` matches a note when VALUE equals (exact, case-sensitive) the author name or the author email of the note or of any of its addenda. `--branch VALUE` matches when the note or any of its addenda was written on that branch. `view` uses the same rule for its Notes table and filters; the entry views use the author and branch of the note or addendum that holds the entry.
- **branch**: `git rev-parse --abbrev-ref HEAD` in the current directory. Override with `--branch`. Empty outside a git repository or in detached HEAD. At most 200 characters, one line.
- Both are stored on the row and cannot be edited afterward. A correction is an addendum with its own author and branch.
- Display: both appear on the note card in `show`, `brief` and `view`, and on each addendum.
- `list` and `brief` gain `--branch` and `--author` filters (exact match).
- `view` adds a Branch column and an Author column to the Notes table, a Branch filter and an Author filter to every view, and both are sortable. Their values come from the rows being listed, like the other filters. Entry views take the branch and author of the note or addendum that holds the entry.
- `export` (JSON and Markdown) and `import` round-trip all of `author`, `branch` and `ref`. In `note-json.md` they are optional input fields on `add --stdin`.

## Compatibility with existing projects

Hard requirement: nothing that worked before works differently.

- **Pattern.** An existing project has no mode. It is read as `custom` with its stored `task_pattern`. The code path that validates a task is unchanged and still reads only `task_pattern`. A test must run the same set of IDs through the old and new code and get identical results for the default pattern and for at least one custom pattern with and without `ext`.
- **Schema.** The database is verified exactly on every open. The schema version is **3** in the code (`SchemaVersion` in `internal/store/schema.go`); `docs/storage.md` still says 1 and is wrong, and must be corrected as part of this work. No migration code exists today: earlier version bumps simply made older databases unsupported. This change adds the first migration, **3 to 4**, and the small framework it needs (an ordered list of steps keyed by version). Databases older than 3 stay unsupported, as now.
  - The new columns are `author_name`, `author_email`, `branch`, `ref` on notes and addenda (all `TEXT NOT NULL DEFAULT ''`) and `task_mode`, `task_prefix`, `task_url` on the project row (`TEXT NOT NULL DEFAULT ''`). The migration is only `ALTER TABLE ... ADD COLUMN`. It updates no row, so the immutability triggers are not involved. It runs in one transaction, sets `user_version = 4`, and is checked by the same exact-schema verification, which now expects the version 4 shape.
  - **Every command that opens a version 3 database migrates it first** (reads included). There is no tolerant read mode: that would make every query handle two shapes. Before the first migration of a file, cairn copies it to `cairn.db.v3.bak` next to it, and refuses to migrate (exit 1) if the copy fails or the directory is read-only. `cairn view`'s rule "never changes the database" becomes "never changes it except this one-time migration".
  - Existing rows get the empty string for the new columns. An empty `task_mode` means `custom`. Old notes show no author, branch or ref.
  - A binary older than this change refuses a version 4 database with its existing message (`unsupported database version 4 (this build supports 3)`). That is acceptable, but the release notes must say it, and the message should point to `cairn update`.
- **Export and import.** Files exported before this change import without the new fields. Files exported after it carry them.
- **IDs and output.** Existing IDs, JSON keys and exit codes stay the same. New JSON keys are additions only.

## Constraints

- Go, no new dependency. SQLite stays the only storage in this design.
- Follow the repository's conventions: one `docs/commands/<name>.md` per command, `docs/configuration.md` and `docs/limits.md` updated, `docs_test.go` kept passing, English docs.
- `view` stays a static, offline, read-only site.

## Left to the builder

- Exact flag help text and error wording, following the existing messages.
- How `view` presents the author and branch sorting in the table header.
- How the migration is structured in `internal/store`, as long as the requirements above hold.
- Whether `--task-prefix` accepts digits (default: letters only).

## Decisions made by the assistant, not by the user

These should be checked while reading:

1. In a terminal, `init` asks and proposes `auto` with tag `T` first. Without a terminal and without task flags, the project keeps the legacy default pattern (to avoid breaking scripts), with a hint on standard error.
10. Prompts go to standard error, `--no-input` disables them, and `init` asks every question whose flag is missing (name, mode, path, repos, default repo, context, tasks), each with its default; Enter accepts it.
11. A `ref` can also be added later by `amend --ref`, and a task can carry several refs.
2. `--task T-99` is accepted in `auto` mode even when no such number exists.
3. `ref` is stored per note (there is no task table); the display uses the template when the note has none.
4. The author is read from git config; the branch is read automatically; both can be overridden.
5. The `jira` and `linear` regexes are broader than the old default (longer keys, more digits).
6. Every command migrates a version 3 database to 4 on open, after making a `.v3.bak` copy.
8. Author is stored as name plus email, matched exactly; filters also match addenda.
9. Invalid author, branch or ref values are refused (exit 3), not sanitized.
7. The `view` site also gets an Author filter and column, not only Branch.

package store

import (
	"regexp"
	"strings"
)

// SchemaVersion is stored in PRAGMA user_version.
const SchemaVersion = 3

// contentTables are the tables protected by immutability triggers.
var contentTables = []string{"project", "notes", "entries", "addenda", "links"}

const tableSQL = `
CREATE TABLE project (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	name TEXT NOT NULL,
	mode TEXT NOT NULL,
	context TEXT NOT NULL,
	repos TEXT NOT NULL,
	default_repo TEXT NOT NULL DEFAULT '',
	task_pattern TEXT NOT NULL
);
CREATE TABLE notes (
	id TEXT PRIMARY KEY,
	agent TEXT NOT NULL,
	repo TEXT NOT NULL,
	at TEXT NOT NULL,
	task TEXT NOT NULL DEFAULT '',
	task_base TEXT NOT NULL DEFAULT '',
	task_ext TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL,
	title TEXT NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('done','in_progress','blocked')),
	none TEXT NOT NULL,
	impacts TEXT NOT NULL,
	commits TEXT NOT NULL
);
CREATE TABLE addenda (
	id TEXT PRIMARY KEY,
	note_id TEXT NOT NULL REFERENCES notes(id),
	agent TEXT NOT NULL,
	model TEXT NOT NULL,
	at TEXT NOT NULL,
	status TEXT CHECK (status IS NULL OR status IN ('done','in_progress','blocked'))
);
CREATE TABLE entries (
	id TEXT PRIMARY KEY,
	note_id TEXT REFERENCES notes(id),
	addendum_id TEXT REFERENCES addenda(id),
	pos INTEGER NOT NULL,
	type TEXT NOT NULL CHECK (type IN ('change','decision','open','note','helper')),
	text TEXT NOT NULL,
	decided_by TEXT NOT NULL DEFAULT '',
	CHECK ((note_id IS NULL) <> (addendum_id IS NULL))
);
CREATE TABLE links (
	seq INTEGER PRIMARY KEY,
	kind TEXT NOT NULL CHECK (kind IN ('resolves','supersedes')),
	target_entry_id TEXT NOT NULL REFERENCES entries(id),
	source_note_id TEXT REFERENCES notes(id),
	source_addendum_id TEXT REFERENCES addenda(id),
	UNIQUE (target_entry_id, kind),
	CHECK ((source_note_id IS NULL) <> (source_addendum_id IS NULL))
);
CREATE INDEX notes_task ON notes(task_base, task);
CREATE INDEX entries_note ON entries(note_id);
CREATE INDEX entries_addendum ON entries(addendum_id);
CREATE INDEX addenda_note ON addenda(note_id);
CREATE VIRTUAL TABLE search USING fts5(text, kind UNINDEXED, id UNINDEXED, note_id UNINDEXED);
`

// replaceGuards is, per content table, the condition that an INSERT would replace
// an existing row. INSERT OR REPLACE deletes the old row without firing DELETE
// triggers, so it needs a trigger of its own.
var replaceGuards = map[string]string{
	"project": "id = NEW.id",
	"notes":   "id = NEW.id",
	"entries": "id = NEW.id",
	"addenda": "id = NEW.id",
	"links":   "seq = NEW.seq OR (target_entry_id = NEW.target_entry_id AND kind = NEW.kind)",
}

// triggerSQL returns the name -> CREATE TRIGGER statement for every immutability trigger.
func triggerSQL() map[string]string {
	out := map[string]string{}
	for _, t := range contentTables {
		for _, op := range []string{"update", "delete"} {
			name := t + "_no_" + op
			out[name] = "CREATE TRIGGER " + name + " BEFORE " + strings.ToUpper(op) + " ON " + t +
				" BEGIN SELECT RAISE(ABORT, 'immutable: " + t + " rows cannot be modified or deleted'); END"
		}
		name := t + "_no_replace"
		out[name] = "CREATE TRIGGER " + name + " BEFORE INSERT ON " + t +
			" WHEN EXISTS (SELECT 1 FROM " + t + " WHERE " + replaceGuards[t] + ")" +
			" BEGIN SELECT RAISE(ABORT, 'immutable: " + t + " rows cannot be replaced'); END"
	}
	return out
}

var spaceRun = regexp.MustCompile(`\s+`)

func normalizeSQL(s string) string {
	return strings.TrimSpace(spaceRun.ReplaceAllString(s, " "))
}

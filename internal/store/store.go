// Package store owns the SQLite schema, the immutability triggers and all queries.
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go driver, includes FTS5

	"github.com/R0b-1ns/cairn/internal/ids"
	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/validate"
)

// Sentinel errors.
var (
	ErrConflict = errors.New("id already exists")
	ErrNotFound = errors.New("not found")
)

// LinkError reports an invalid resolves/supersedes target.
type LinkError struct{ Field, Target, Reason string }

func (e *LinkError) Error() string { return fmt.Sprintf("%s: %s: %s", e.Field, e.Target, e.Reason) }

// Store is an open cairn database.
type Store struct {
	db   *sql.DB
	path string
}

func dsn(path, mode string) string {
	q := url.Values{}
	q.Add("mode", mode)
	q.Add("_txlock", "immediate")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "trusted_schema(0)")
	q.Add("_pragma", "busy_timeout(5000)")
	u := url.URL{Scheme: "file", Path: path, RawQuery: q.Encode()}
	return u.String()
}

// Create makes a new database with the given project. It fails if the file exists.
func Create(path string, p model.Project) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create database: %w", err)
	}
	f.Close()
	db, err := sql.Open("sqlite", dsn(path, "rw"))
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, path: path}
	if err := s.init(p); err != nil {
		db.Close()
		os.Remove(path)
		return nil, err
	}
	return s, nil
}

func (s *Store) init(p model.Project) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(tableSQL); err != nil {
		return err
	}
	for _, stmt := range triggerSQL() {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	repos, _ := json.Marshal(p.Repos)
	if _, err := tx.Exec(`INSERT INTO project(id,name,mode,context,repos,task_pattern) VALUES (1,?,?,?,?,?)`,
		p.Name, p.Mode, p.Context, string(repos), p.TaskPattern); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	_, err = s.db.Exec(fmt.Sprintf("PRAGMA user_version = %d", SchemaVersion))
	return err
}

// Open opens an existing database and verifies it is a trustworthy cairn database:
// right schema version, intact, and every immutability trigger present and unmodified.
func Open(path string) (*Store, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	db, err := sql.Open("sqlite", dsn(path, "rw"))
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, path: path}
	if err := s.verify(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) verify() error {
	var v int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return fmt.Errorf("not a cairn database: %w", err)
	}
	if v != SchemaVersion {
		return fmt.Errorf("unsupported database version %d (this build supports %d)", v, SchemaVersion)
	}
	var check string
	if err := s.db.QueryRow("PRAGMA quick_check(1)").Scan(&check); err != nil || check != "ok" {
		return fmt.Errorf("database integrity check failed: %v %s", err, check)
	}
	rows, err := s.db.Query("SELECT name, COALESCE(sql,'') FROM sqlite_master WHERE type='trigger'")
	if err != nil {
		return fmt.Errorf("not a cairn database: %w", err)
	}
	defer rows.Close()
	have := map[string]string{}
	for rows.Next() {
		var n, q string
		if err := rows.Scan(&n, &q); err != nil {
			return err
		}
		have[n] = q
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for name, want := range triggerSQL() {
		got, ok := have[name]
		if !ok {
			return fmt.Errorf("immutability trigger %s is missing: database refused", name)
		}
		if normalizeSQL(got) != normalizeSQL(want) {
			return fmt.Errorf("immutability trigger %s was altered: database refused", name)
		}
	}
	if err := s.verifySchema(); err != nil {
		return err
	}
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM project").Scan(&n); err != nil || n != 1 {
		return fmt.Errorf("project row missing: database refused")
	}
	return nil
}

// schemaObjects maps "type:name" to normalized SQL for everything in sqlite_master.
func schemaObjects(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query("SELECT type, name, COALESCE(sql,'') FROM sqlite_master")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var t, n, q string
		if err := rows.Scan(&t, &n, &q); err != nil {
			return nil, err
		}
		out[t+":"+n] = normalizeSQL(q)
	}
	return out, rows.Err()
}

// expectedSchema builds the reference schema in memory from the same SQL used by Create.
func expectedSchema() (map[string]string, error) {
	db, err := sql.Open("sqlite", "file::memory:")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(tableSQL); err != nil {
		return nil, err
	}
	for _, stmt := range triggerSQL() {
		if _, err := db.Exec(stmt); err != nil {
			return nil, err
		}
	}
	return schemaObjects(db)
}

// verifySchema refuses a database whose schema differs in any way from ours:
// extra or missing tables, indexes, views or triggers, or altered definitions.
func (s *Store) verifySchema() error {
	want, err := expectedSchema()
	if err != nil {
		return err
	}
	have, err := schemaObjects(s.db)
	if err != nil {
		return fmt.Errorf("not a cairn database: %w", err)
	}
	for k, v := range have {
		if w, ok := want[k]; !ok {
			return fmt.Errorf("unexpected schema object %s: database refused", k)
		} else if w != v {
			return fmt.Errorf("schema object %s differs from the expected definition: database refused", k)
		}
	}
	for k := range want {
		if _, ok := have[k]; !ok {
			return fmt.Errorf("schema object %s is missing: database refused", k)
		}
	}
	return nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Path returns the database file path.
func (s *Store) Path() string { return s.path }

// Project returns the stored project configuration.
func (s *Store) Project() (model.Project, error) {
	var p model.Project
	var repos string
	err := s.db.QueryRow(`SELECT name,mode,context,repos,task_pattern FROM project WHERE id=1`).
		Scan(&p.Name, &p.Mode, &p.Context, &repos, &p.TaskPattern)
	if err != nil {
		return p, err
	}
	err = json.Unmarshal([]byte(repos), &p.Repos)
	return p, err
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

func idTaken(tx execer, id string) (bool, error) {
	var one int
	err := tx.QueryRow(`SELECT 1 FROM (SELECT id FROM notes UNION ALL SELECT id FROM addenda UNION ALL SELECT id FROM entries) WHERE id=?`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// assignID returns given when set (failing if taken) or a fresh free ID.
func assignID(tx execer, kind byte, given string) (string, error) {
	if given != "" {
		taken, err := idTaken(tx, given)
		if err != nil {
			return "", err
		}
		if taken {
			return "", fmt.Errorf("%w: %s", ErrConflict, given)
		}
		return given, nil
	}
	for {
		id := ids.New(kind)
		taken, err := idTaken(tx, id)
		if err != nil {
			return "", err
		}
		if !taken {
			return id, nil
		}
	}
}

func checkTarget(tx execer, field, target string, want model.EntryType, kind string) error {
	var typ string
	err := tx.QueryRow(`SELECT type FROM entries WHERE id=?`, target).Scan(&typ)
	if errors.Is(err, sql.ErrNoRows) {
		return &LinkError{field, target, "no such entry"}
	}
	if err != nil {
		return err
	}
	if model.EntryType(typ) != want {
		return &LinkError{field, target, fmt.Sprintf("is a %s entry, expected %s", typ, want)}
	}
	var one int
	err = tx.QueryRow(`SELECT 1 FROM links WHERE target_entry_id=? AND kind=?`, target, kind).Scan(&one)
	if err == nil {
		return &LinkError{field, target, "already " + map[string]string{"resolves": "resolved", "supersedes": "superseded"}[kind]}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

func insertLinks(tx execer, srcNote, srcAdd string, resolves, supersedes []string) error {
	nn := sql.NullString{String: srcNote, Valid: srcNote != ""}
	na := sql.NullString{String: srcAdd, Valid: srcAdd != ""}
	for _, t := range resolves {
		if err := checkTarget(tx, "resolves", t, model.TypeOpen, "resolves"); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO links(kind,target_entry_id,source_note_id,source_addendum_id) VALUES ('resolves',?,?,?)`, t, nn, na); err != nil {
			return err
		}
	}
	for _, t := range supersedes {
		if err := checkTarget(tx, "supersedes", t, model.TypeDecision, "supersedes"); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO links(kind,target_entry_id,source_note_id,source_addendum_id) VALUES ('supersedes',?,?,?)`, t, nn, na); err != nil {
			return err
		}
	}
	return nil
}

func insertEntries(tx execer, noteID, addID string, entries []model.Entry) ([]model.Entry, error) {
	out := make([]model.Entry, len(entries))
	nn := sql.NullString{String: noteID, Valid: noteID != ""}
	na := sql.NullString{String: addID, Valid: addID != ""}
	for i, e := range entries {
		id, err := assignID(tx, 'e', e.ID)
		if err != nil {
			return nil, err
		}
		e.ID = id
		if _, err := tx.Exec(`INSERT INTO entries(id,note_id,addendum_id,pos,type,text,decided_by) VALUES (?,?,?,?,?,?,?)`,
			e.ID, nn, na, i, string(e.Type), e.Text, e.By); err != nil {
			return nil, err
		}
		owner := noteID
		if owner == "" {
			owner = ownerNote(tx, addID)
		}
		if _, err := tx.Exec(`INSERT INTO search(text,kind,id,note_id) VALUES (?,?,?,?)`, e.Text, "entry", e.ID, owner); err != nil {
			return nil, err
		}
		out[i] = e
	}
	return out, nil
}

func ownerNote(tx execer, addID string) string {
	var n string
	tx.QueryRow(`SELECT note_id FROM addenda WHERE id=?`, addID).Scan(&n)
	return n
}

func jsonList(v any) string {
	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" {
		return "[]"
	}
	return string(b)
}

// InsertNote stores a validated note, assigning missing IDs. Given IDs are kept
// (used by import); a taken ID yields ErrConflict. The whole insert is atomic.
func (s *Store) InsertNote(n model.Note) (model.Note, error) {
	p, err := s.Project()
	if err != nil {
		return n, err
	}
	base, ext, _ := validate.TaskParts(p, n.Task)
	if n.At, err = normAt(n.At); err != nil {
		return n, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return n, err
	}
	defer tx.Rollback()
	if n.ID, err = assignID(tx, 'n', n.ID); err != nil {
		return n, err
	}
	if _, err = tx.Exec(`INSERT INTO notes(id,agent,repo,at,task,task_base,task_ext,model,title,status,none,impacts,commits) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		n.ID, n.Agent, n.Repo, n.At, n.Task, base, ext, n.Model, n.Title, string(n.Status),
		jsonList(n.None), jsonList(n.Impacts), jsonList(n.Commits)); err != nil {
		return n, err
	}
	if _, err = tx.Exec(`INSERT INTO search(text,kind,id,note_id) VALUES (?,?,?,?)`, n.Title, "note", n.ID, n.ID); err != nil {
		return n, err
	}
	if n.Entries, err = insertEntries(tx, n.ID, "", n.Entries); err != nil {
		return n, err
	}
	if err = insertLinks(tx, n.ID, "", n.Resolves, n.Supersedes); err != nil {
		return n, err
	}
	return n, tx.Commit()
}

// InsertAddendum stores a validated addendum attached to an existing note.
func (s *Store) InsertAddendum(a model.Addendum) (model.Addendum, error) {
	var err error
	if a.At, err = normAt(a.At); err != nil {
		return a, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return a, err
	}
	defer tx.Rollback()
	var one int
	if err = tx.QueryRow(`SELECT 1 FROM notes WHERE id=?`, a.NoteID).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return a, fmt.Errorf("%w: note %s", ErrNotFound, a.NoteID)
		}
		return a, err
	}
	if a.ID, err = assignID(tx, 'a', a.ID); err != nil {
		return a, err
	}
	st := sql.NullString{String: string(a.Status), Valid: a.Status != ""}
	if _, err = tx.Exec(`INSERT INTO addenda(id,note_id,agent,model,at,status) VALUES (?,?,?,?,?,?)`,
		a.ID, a.NoteID, a.Agent, a.Model, a.At, st); err != nil {
		return a, err
	}
	if a.Entries, err = insertEntries(tx, "", a.ID, a.Entries); err != nil {
		return a, err
	}
	if err = insertLinks(tx, "", a.ID, a.Resolves, a.Supersedes); err != nil {
		return a, err
	}
	return a, tx.Commit()
}

func loadEntries(q interface {
	Query(string, ...any) (*sql.Rows, error)
}, col, id string) ([]model.Entry, error) {
	rows, err := q.Query(`SELECT id,type,text,decided_by FROM entries WHERE `+col+`=? ORDER BY pos`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Entry
	for rows.Next() {
		var e model.Entry
		var t string
		if err := rows.Scan(&e.ID, &t, &e.Text, &e.By); err != nil {
			return nil, err
		}
		e.Type = model.EntryType(t)
		out = append(out, e)
	}
	return out, rows.Err()
}

func loadLinks(db *sql.DB, col, id string) (resolves, supersedes []string, err error) {
	rows, err := db.Query(`SELECT kind,target_entry_id FROM links WHERE `+col+`=? ORDER BY seq`, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, t string
		if err := rows.Scan(&k, &t); err != nil {
			return nil, nil, err
		}
		if k == "resolves" {
			resolves = append(resolves, t)
		} else {
			supersedes = append(supersedes, t)
		}
	}
	return resolves, supersedes, rows.Err()
}

// GetNote returns a note with its entries and links.
func (s *Store) GetNote(id string) (model.Note, error) {
	var n model.Note
	var st, none, impacts, commits string
	err := s.db.QueryRow(`SELECT id,agent,repo,at,task,model,title,status,none,impacts,commits FROM notes WHERE id=?`, id).
		Scan(&n.ID, &n.Agent, &n.Repo, &n.At, &n.Task, &n.Model, &n.Title, &st, &none, &impacts, &commits)
	if errors.Is(err, sql.ErrNoRows) {
		return n, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if err != nil {
		return n, err
	}
	n.Status = model.Status(st)
	for dst, src := range map[any]string{&n.None: none, &n.Impacts: impacts, &n.Commits: commits} {
		if err := json.Unmarshal([]byte(src), dst); err != nil {
			return n, err
		}
	}
	if len(n.None) == 0 {
		n.None = nil
	}
	if len(n.Impacts) == 0 {
		n.Impacts = nil
	}
	if len(n.Commits) == 0 {
		n.Commits = nil
	}
	if n.Entries, err = loadEntries(s.db, "note_id", id); err != nil {
		return n, err
	}
	n.Resolves, n.Supersedes, err = loadLinks(s.db, "source_note_id", id)
	return n, err
}

// GetAddendum returns an addendum with its entries and links.
func (s *Store) GetAddendum(id string) (model.Addendum, error) {
	var a model.Addendum
	var st sql.NullString
	err := s.db.QueryRow(`SELECT id,note_id,agent,model,at,status FROM addenda WHERE id=?`, id).
		Scan(&a.ID, &a.NoteID, &a.Agent, &a.Model, &a.At, &st)
	if errors.Is(err, sql.ErrNoRows) {
		return a, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if err != nil {
		return a, err
	}
	a.Status = model.Status(st.String)
	if a.Entries, err = loadEntries(s.db, "addendum_id", id); err != nil {
		return a, err
	}
	a.Resolves, a.Supersedes, err = loadLinks(s.db, "source_addendum_id", id)
	return a, err
}

// CountNotes returns the number of notes.
func (s *Store) CountNotes() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM notes`).Scan(&n)
	return n, err
}

package store

import (
	"strings"

	"github.com/R0b-1ns/cairn/internal/model"
)

// StateFilter selects entries by whether open points and decisions are still live.
type StateFilter int

// States. Only open entries and decisions have a state; other types are always active.
const (
	StateActive StateFilter = iota // hide resolved open points and superseded decisions
	StateClosed                    // only resolved open points and superseded decisions
	StateAll
)

// Filter selects entries. Zero values mean "no constraint". Since is inclusive and
// Until exclusive; both are RFC3339 UTC strings compared as text.
type Filter struct {
	Types    []model.EntryType
	TaskBase string // matches the task and its extensions
	Task     string // matches exactly one task ID
	Repo     string // repo of the note
	Impacts  string // a repo listed in the note's impacts
	Since    string
	Until    string
	State    StateFilter
	Limit    int // 0 = no limit
}

// ListEntries returns matching entries newest first and the total number that match
// (before Limit). All filter values are bound parameters.
func (s *Store) ListEntries(f Filter) ([]EntryInfo, int, error) {
	var where []string
	var args []any
	if len(f.Types) > 0 {
		ph := make([]string, len(f.Types))
		for i, t := range f.Types {
			ph[i] = "?"
			args = append(args, string(t))
		}
		where = append(where, "e.type IN ("+strings.Join(ph, ",")+")")
	}
	add := func(cond string, v string) {
		if v != "" {
			where = append(where, cond)
			args = append(args, v)
		}
	}
	add("n.task_base = ?", f.TaskBase)
	add("n.task = ?", f.Task)
	add("n.repo = ?", f.Repo)
	add("EXISTS (SELECT 1 FROM json_each(n.impacts) WHERE value = ?)", f.Impacts)
	add("COALESCE(a.at, n.at) >= ?", f.Since)
	add("COALESCE(a.at, n.at) < ?", f.Until)
	switch f.State {
	case StateActive:
		where = append(where, "(e.type NOT IN ('open','decision') OR l.target_entry_id IS NULL)")
	case StateClosed:
		where = append(where, "(e.type IN ('open','decision') AND l.target_entry_id IS NOT NULL)")
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + strings.Join(where, " AND ")
	}
	var total int
	count := `SELECT COUNT(*) FROM entries e
		LEFT JOIN addenda a ON a.id = e.addendum_id
		JOIN notes n ON n.id = COALESCE(e.note_id, a.note_id)
		LEFT JOIN links l ON l.target_entry_id = e.id` + cond
	if err := s.db.QueryRow(count, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := entrySelect + cond + " ORDER BY COALESCE(a.at, n.at) DESC, e.rowid DESC"
	if f.Limit > 0 {
		q += " LIMIT ?"
		args = append(args, f.Limit)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []EntryInfo
	for rows.Next() {
		e, err := scanEntryInfo(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

// Subject is a unit of work: a task (all notes sharing its base) or a single
// note that has no task.
type Subject struct {
	Key    string // task base, or the note ID for a task-less note
	IsTask bool
	LastAt string // latest note or addendum time
	Notes  int
	NoteID string // the most recently active note of the subject
}

const activityCTE = `WITH act AS (
	SELECT id AS note_id, task_base, at FROM notes
	UNION ALL
	SELECT a.note_id, n.task_base, a.at FROM addenda a JOIN notes n ON n.id = a.note_id)`

// Subjects returns up to limit subjects ordered by latest activity, newest first.
func (s *Store) Subjects(limit int) ([]Subject, error) {
	rows, err := s.db.Query(activityCTE+`
		SELECT CASE WHEN task_base <> '' THEN task_base ELSE note_id END AS k,
		       task_base <> '', MAX(at), COUNT(DISTINCT note_id)
		FROM act GROUP BY k ORDER BY MAX(at) DESC, k LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	var out []Subject
	for rows.Next() {
		var sub Subject
		if err := rows.Scan(&sub.Key, &sub.IsTask, &sub.LastAt, &sub.Notes); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, sub)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		err := s.db.QueryRow(activityCTE+`
			SELECT note_id FROM act WHERE (task_base = ? OR note_id = ?) ORDER BY at DESC LIMIT 1`,
			out[i].Key, out[i].Key).Scan(&out[i].NoteID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// NoteFilter selects note headers; zero values mean "no constraint".
type NoteFilter struct{ TaskBase, Task, ID string }

// NoteHeader is a note without its entries; Status is the current status
// (latest addendum carrying one, else the note's own).
type NoteHeader struct {
	ID, Task, Repo, Agent, Model, At, Title string
	Status                                  model.Status
}

// NoteHeaders returns matching notes oldest first.
func (s *Store) NoteHeaders(f NoteFilter) ([]NoteHeader, error) {
	q := `SELECT n.id, n.task, n.repo, n.agent, n.model, n.at, n.title,
		COALESCE((SELECT a.status FROM addenda a WHERE a.note_id = n.id AND a.status IS NOT NULL
		          ORDER BY a.at DESC, a.rowid DESC LIMIT 1), n.status)
		FROM notes n WHERE (? = '' OR n.task_base = ?) AND (? = '' OR n.task = ?) AND (? = '' OR n.id = ?)
		ORDER BY n.at, n.rowid`
	rows, err := s.db.Query(q, f.TaskBase, f.TaskBase, f.Task, f.Task, f.ID, f.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NoteHeader
	for rows.Next() {
		var h NoteHeader
		var st string
		if err := rows.Scan(&h.ID, &h.Task, &h.Repo, &h.Agent, &h.Model, &h.At, &h.Title, &st); err != nil {
			return nil, err
		}
		h.Status = model.Status(st)
		out = append(out, h)
	}
	return out, rows.Err()
}

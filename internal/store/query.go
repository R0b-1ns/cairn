package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/R0b-1ns/cairn/internal/model"
)

// normAt converts any RFC3339 timestamp to UTC so that text comparison orders by time.
func normAt(at string) (string, error) {
	t, err := time.Parse(time.RFC3339, at)
	if err != nil {
		return "", fmt.Errorf("bad timestamp %q: %w", at, err)
	}
	return t.UTC().Format(time.RFC3339), nil
}

// EntryInfo is an entry with where it lives and its computed state.
// Closed means resolved (open entries) or superseded (decisions); the entry
// text itself is never changed.
type EntryInfo struct {
	model.Entry
	NoteID     string `json:"note_id"`     // the note the entry belongs to (the parent note for addendum entries)
	AddendumID string `json:"addendum_id"` // set when the entry was written in an addendum
	Task       string `json:"task"`        // of the parent note
	Repo       string `json:"repo"`
	At         string `json:"at"` // time of the note or addendum that holds the entry
	Closed     bool   `json:"closed"`
	ClosedBy   string `json:"closed_by"` // ID of the note or addendum that closes it
	ClosedAt   string `json:"closed_at"`
}

const entrySelect = `
SELECT e.id, e.type, e.text, e.decided_by, COALESCE(e.addendum_id,''), n.id, n.task, n.repo, COALESCE(a.at, n.at),
       COALESCE(l.source_addendum_id, l.source_note_id, ''), COALESCE(la.at, ln.at, '')
FROM entries e
LEFT JOIN addenda a ON a.id = e.addendum_id
JOIN notes n ON n.id = COALESCE(e.note_id, a.note_id)
LEFT JOIN links l ON l.target_entry_id = e.id
LEFT JOIN addenda la ON la.id = l.source_addendum_id
LEFT JOIN notes ln ON ln.id = l.source_note_id`

type scanner interface{ Scan(...any) error }

func scanEntryInfo(r scanner) (EntryInfo, error) {
	var e EntryInfo
	var typ string
	err := r.Scan(&e.ID, &typ, &e.Text, &e.By, &e.AddendumID, &e.NoteID, &e.Task, &e.Repo, &e.At, &e.ClosedBy, &e.ClosedAt)
	e.Type = model.EntryType(typ)
	e.Closed = e.ClosedBy != ""
	return e, err
}

// Entry returns one entry with its location and computed state.
func (s *Store) Entry(id string) (EntryInfo, error) {
	e, err := scanEntryInfo(s.db.QueryRow(entrySelect+` WHERE e.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return e, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return e, err
}

// CurrentStatus is the status of the most recent addendum carrying one
// (ordered by time, then insertion), otherwise the note's own status.
func (s *Store) CurrentStatus(noteID string) (model.Status, error) {
	var st string
	err := s.db.QueryRow(`SELECT status FROM addenda WHERE note_id = ? AND status IS NOT NULL
		ORDER BY at DESC, rowid DESC LIMIT 1`, noteID).Scan(&st)
	if err == nil {
		return model.Status(st), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	err = s.db.QueryRow(`SELECT status FROM notes WHERE id = ?`, noteID).Scan(&st)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("%w: %s", ErrNotFound, noteID)
	}
	return model.Status(st), err
}

// Addenda returns the addenda of a note in chronological order.
func (s *Store) Addenda(noteID string) ([]model.Addendum, error) {
	rows, err := s.db.Query(`SELECT id FROM addenda WHERE note_id = ? ORDER BY at, rowid`, noteID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]model.Addendum, 0, len(ids))
	for _, id := range ids {
		a, err := s.GetAddendum(id)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

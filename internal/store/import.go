package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/R0b-1ns/cairn/internal/model"
)

// ImportItem is one note or one addendum to import; exactly one field is set.
// Items must already be validated and carry their IDs.
type ImportItem struct {
	Note     *model.Note
	Addendum *model.Addendum
}

// ImportResult counts what an import did.
type ImportResult struct{ Inserted, Notes, Addenda, Skipped int }

// canon normalizes an item for comparison with what is stored: UTC timestamps
// and nil-versus-empty lists must not count as differences.
func canon(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func normNote(n model.Note) model.Note {
	n.At, _ = normAt(n.At)
	if len(n.None) == 0 {
		n.None = nil
	}
	if len(n.Impacts) == 0 {
		n.Impacts = nil
	}
	if len(n.Commits) == 0 {
		n.Commits = nil
	}
	if len(n.Resolves) == 0 {
		n.Resolves = nil
	}
	if len(n.Supersedes) == 0 {
		n.Supersedes = nil
	}
	return n
}

func normAdd(a model.Addendum) model.Addendum {
	a.At, _ = normAt(a.At)
	if len(a.Entries) == 0 {
		a.Entries = nil
	}
	if len(a.Resolves) == 0 {
		a.Resolves = nil
	}
	if len(a.Supersedes) == 0 {
		a.Supersedes = nil
	}
	return a
}

// Import inserts items in dependency order inside one transaction. An item whose
// ID already exists with identical content is skipped; with different content the
// whole import fails with ErrConflict. Existing data is never modified.
func (s *Store) Import(items []ImportItem) (ImportResult, error) {
	var res ImportResult
	p, err := s.Project()
	if err != nil {
		return res, err
	}
	var todo []ImportItem
	for _, it := range items {
		switch {
		case it.Note != nil:
			old, err := s.GetNote(it.Note.ID)
			if errors.Is(err, ErrNotFound) {
				todo = append(todo, it)
				continue
			}
			if err != nil {
				return res, err
			}
			if !reflect.DeepEqual(normNote(old), normNote(*it.Note)) {
				return res, fmt.Errorf("%w: note %s already exists with different content", ErrConflict, old.ID)
			}
			res.Skipped++
		case it.Addendum != nil:
			old, err := s.GetAddendum(it.Addendum.ID)
			if errors.Is(err, ErrNotFound) {
				todo = append(todo, it)
				continue
			}
			if err != nil {
				return res, err
			}
			if !reflect.DeepEqual(normAdd(old), normAdd(*it.Addendum)) {
				return res, fmt.Errorf("%w: addendum %s already exists with different content", ErrConflict, old.ID)
			}
			res.Skipped++
		default:
			return res, errors.New("empty import item")
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return res, err
	}
	defer tx.Rollback()

	// Which pending item creates each entry or note, so links and parents order the inserts.
	creator := map[string]int{}
	for i, it := range todo {
		if it.Note != nil {
			creator[it.Note.ID] = i
			for _, e := range it.Note.Entries {
				creator[e.ID] = i
			}
		} else {
			for _, e := range it.Addendum.Entries {
				creator[e.ID] = i
			}
		}
	}
	deps := func(i int) []int {
		var ids []string
		if n := todo[i].Note; n != nil {
			ids = append(append(ids, n.Resolves...), n.Supersedes...)
		} else {
			a := todo[i].Addendum
			ids = append(append([]string{a.NoteID}, a.Resolves...), a.Supersedes...)
		}
		var out []int
		for _, id := range ids {
			if j, ok := creator[id]; ok && j != i {
				out = append(out, j)
			}
		}
		return out
	}
	done := make([]bool, len(todo))
	for remaining := len(todo); remaining > 0; {
		progressed := false
		for i := range todo {
			if done[i] {
				continue
			}
			ready := true
			for _, j := range deps(i) {
				ready = ready && done[j]
			}
			if !ready {
				continue
			}
			if todo[i].Note != nil {
				_, err = insertNoteTx(tx, p, *todo[i].Note)
				res.Notes++
			} else {
				_, err = insertAddendumTx(tx, *todo[i].Addendum)
				res.Addenda++
			}
			if err != nil {
				return ImportResult{}, err
			}
			done[i], progressed = true, true
			remaining--
			res.Inserted++
		}
		if !progressed {
			return ImportResult{}, errors.New("import has circular references between notes")
		}
	}
	return res, tx.Commit()
}

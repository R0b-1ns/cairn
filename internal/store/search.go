package store

import "strings"

// Hit is one full-text match: a note title or an entry text.
type Hit struct {
	Kind       string `json:"kind"` // "note" or "entry"
	ID         string `json:"id"`
	NoteID     string `json:"note_id"`
	AddendumID string `json:"addendum_id"` // set when the entry was written in an addendum
	Type       string `json:"type"`        // entry type, empty for note titles
	Text       string `json:"text"`        // the real text of the entry or note title, never the index copy
}

// ftsQuery quotes every term so FTS5 operators and punctuation in user input
// are plain text. All terms must match (implicit AND).
func ftsQuery(terms []string) string {
	q := make([]string, 0, len(terms))
	for _, t := range terms {
		q = append(q, `"`+strings.ReplaceAll(t, `"`, `""`)+`"`)
	}
	return strings.Join(q, " ")
}

// Search runs a full-text query over note titles and entry texts, best match
// first, and returns the hits (up to limit) and the total number of matches.
// The index only decides what matches: the text shown always comes from the
// immutable tables, and an index row with no real entry or note behind it is ignored.
func (s *Store) Search(terms []string, limit int) ([]Hit, int, error) {
	q := ftsQuery(terms)
	const from = ` FROM search
		LEFT JOIN entries e ON search.kind = 'entry' AND e.id = search.id
		LEFT JOIN notes n ON search.kind = 'note' AND n.id = search.id
		WHERE search MATCH ? AND (e.id IS NOT NULL OR n.id IS NOT NULL)`
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*)`+from, q).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(`SELECT search.kind, search.id, COALESCE(e.note_id, (SELECT note_id FROM addenda WHERE id = e.addendum_id), n.id),
			COALESCE(e.type, ''), COALESCE(e.addendum_id, ''), COALESCE(e.text, n.title)`+from+
		` ORDER BY rank, search.rowid LIMIT ?`, q, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Hit
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.Kind, &h.ID, &h.NoteID, &h.Type, &h.AddendumID, &h.Text); err != nil {
			return nil, 0, err
		}
		out = append(out, h)
	}
	return out, total, rows.Err()
}

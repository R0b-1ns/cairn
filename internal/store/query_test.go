package store

import (
	"testing"

	"github.com/R0b-1ns/cairn/internal/model"
)

func addAddendum(t *testing.T, s *Store, a model.Addendum) model.Addendum {
	t.Helper()
	if a.Agent == "" {
		a.Agent = "builder"
	}
	if a.Model == "" {
		a.Model = "claude-test"
	}
	out, err := s.InsertAddendum(a)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCurrentStatusLatestAddendum(t *testing.T) {
	s, _ := newStore(t)
	n, _ := s.InsertNote(sampleNote()) // in_progress
	if st, err := s.CurrentStatus(n.ID); err != nil || st != model.StatusInProgress {
		t.Fatalf("own status: %v %v", st, err)
	}
	addAddendum(t, s, model.Addendum{NoteID: n.ID, At: "2026-10-30T12:00:00Z", Status: model.StatusDone,
		Entries: []model.Entry{{Type: model.TypeNote, Text: "a"}}})
	// Inserted later but older: must not win.
	addAddendum(t, s, model.Addendum{NoteID: n.ID, At: "2026-10-30T11:00:00Z", Status: model.StatusBlocked})
	// Newest, but carries no status: ignored.
	addAddendum(t, s, model.Addendum{NoteID: n.ID, At: "2026-10-30T13:00:00Z",
		Entries: []model.Entry{{Type: model.TypeNote, Text: "b"}}})
	if st, _ := s.CurrentStatus(n.ID); st != model.StatusDone {
		t.Fatalf("want done, got %s", st)
	}
	if _, err := s.CurrentStatus("n-zzzzzz"); err == nil {
		t.Fatal("unknown note must fail")
	}
}

func TestCurrentStatusTieBreaksByInsertion(t *testing.T) {
	s, _ := newStore(t)
	n, _ := s.InsertNote(sampleNote())
	addAddendum(t, s, model.Addendum{NoteID: n.ID, At: "2026-10-30T12:00:00Z", Status: model.StatusBlocked})
	addAddendum(t, s, model.Addendum{NoteID: n.ID, At: "2026-10-30T12:00:00Z", Status: model.StatusDone})
	if st, _ := s.CurrentStatus(n.ID); st != model.StatusDone {
		t.Fatalf("want last inserted on tie, got %s", st)
	}
}

func TestEntryStateComputedNotStored(t *testing.T) {
	s, path := newStore(t)
	n, _ := s.InsertNote(sampleNote())
	open, decision := entryIDs(n)

	e, err := s.Entry(open)
	if err != nil || e.Closed || e.NoteID != n.ID || e.Task != "TP-03" || e.Repo != "server" {
		t.Fatalf("fresh open entry: %+v %v", e, err)
	}
	a := addAddendum(t, s, model.Addendum{NoteID: n.ID, At: "2026-10-30T11:00:00Z",
		Resolves: []string{open}, Supersedes: []string{decision},
		Entries: []model.Entry{{Type: model.TypeDecision, Text: "Use a hash index.", By: "agent"}}})

	e, _ = s.Entry(open)
	if !e.Closed || e.ClosedBy != a.ID || e.ClosedAt != "2026-10-30T11:00:00Z" {
		t.Fatalf("resolved open entry: %+v", e)
	}
	d, _ := s.Entry(decision)
	if !d.Closed || d.ClosedBy != a.ID || d.Text != "Use a b-tree." {
		t.Fatalf("superseded decision must keep its text: %+v", d)
	}
	// The addendum's own entry is attributed to the addendum and its parent note.
	newDec, _ := s.Entry(a.Entries[0].ID)
	if newDec.AddendumID != a.ID || newDec.NoteID != n.ID || newDec.Closed {
		t.Fatalf("addendum entry: %+v", newDec)
	}
	// Nothing was rewritten: the stored entry row is untouched.
	var text string
	rawDB(t, path).QueryRow("SELECT text FROM entries WHERE id=?", decision).Scan(&text)
	if text != "Use a b-tree." {
		t.Fatalf("stored text changed: %q", text)
	}
	if _, err := s.Entry("e-zzzzzz"); err == nil {
		t.Fatal("unknown entry must fail")
	}
}

func TestEntryClosedByNoteLevelLink(t *testing.T) {
	s, _ := newStore(t)
	first, _ := s.InsertNote(sampleNote())
	open, _ := entryIDs(first)
	second := sampleNote()
	second.Resolves = []string{open}
	second, _ = s.InsertNote(second)
	e, _ := s.Entry(open)
	if !e.Closed || e.ClosedBy != second.ID {
		t.Fatalf("note-level resolve: %+v", e)
	}
}

func TestAddendaOrdered(t *testing.T) {
	s, _ := newStore(t)
	n, _ := s.InsertNote(sampleNote())
	b := addAddendum(t, s, model.Addendum{NoteID: n.ID, At: "2026-10-30T12:00:00Z", Status: model.StatusDone})
	a := addAddendum(t, s, model.Addendum{NoteID: n.ID, At: "2026-10-30T11:00:00Z", Status: model.StatusBlocked})
	got, err := s.Addenda(n.ID)
	if err != nil || len(got) != 2 || got[0].ID != a.ID || got[1].ID != b.ID {
		t.Fatalf("want chronological order: %+v %v", got, err)
	}
}

func TestTimestampsNormalizedToUTC(t *testing.T) {
	s, _ := newStore(t)
	n := sampleNote()
	n.At = "2026-10-30T12:00:00+02:00"
	in, err := s.InsertNote(n)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetNote(in.ID)
	if got.At != "2026-10-30T10:00:00Z" {
		t.Fatalf("note at = %q", got.At)
	}
	a := addAddendum(t, s, model.Addendum{NoteID: in.ID, At: "2026-10-30T13:30:00-01:00", Status: model.StatusDone})
	ga, _ := s.GetAddendum(a.ID)
	if ga.At != "2026-10-30T14:30:00Z" {
		t.Fatalf("addendum at = %q", ga.At)
	}
}

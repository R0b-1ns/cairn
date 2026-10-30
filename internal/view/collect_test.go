package view

import (
	"path/filepath"
	"testing"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
)

func testProject() model.Project {
	return model.Project{
		Name: "tidepool", Mode: model.ModeProject, Context: "Tide data service.",
		Repos: []string{"server"}, TaskPattern: model.DefaultTaskPattern,
	}
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Create(filepath.Join(t.TempDir(), ".cairn", "cairn.db"), testProject())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func note(at, title string, entries ...model.Entry) model.Note {
	return model.Note{
		Agent: "builder", Repo: "server", At: at, Task: "TP-01", Model: "m", Title: title,
		Status: model.StatusInProgress, Entries: entries,
		None: []model.EntryType{model.TypeChange, model.TypeDecision, model.TypeOpen, model.TypeNote, model.TypeHelper},
	}
}

func insert(t *testing.T, s *store.Store, n model.Note) model.Note {
	t.Helper()
	// the entry types present are not "none"
	var none []model.EntryType
	for _, typ := range model.EntryTypes {
		present := false
		for _, e := range n.Entries {
			if e.Type == typ {
				present = true
			}
		}
		if !present {
			none = append(none, typ)
		}
	}
	n.None = none
	out, err := s.InsertNote(n)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func collect(t *testing.T, s *store.Store) *Site {
	t.Helper()
	site, err := Collect(s, testProject(), "2026-10-31T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return site
}

func TestCollectCurrentAndWrittenStatus(t *testing.T) {
	s := newStore(t)
	n := insert(t, s, note("2026-10-30T10:00:00Z", "A", model.Entry{Type: model.TypeChange, Text: "x"}))
	if _, err := s.InsertAddendum(model.Addendum{NoteID: n.ID, Agent: "reviewer", Model: "m", At: "2026-10-30T11:00:00Z", Status: model.StatusDone}); err != nil {
		t.Fatal(err)
	}
	site := collect(t, s)
	if len(site.Notes) != 1 {
		t.Fatalf("notes %d", len(site.Notes))
	}
	nv := site.Notes[0]
	if nv.Header.Status != model.StatusDone || nv.Written != model.StatusInProgress {
		t.Fatalf("status %q written %q", nv.Header.Status, nv.Written)
	}
	if len(nv.Addenda) != 1 || nv.Note.ID != n.ID {
		t.Fatalf("addenda %d note %q", len(nv.Addenda), nv.Note.ID)
	}
	if site.Generated != "2026-10-31T00:00:00Z" || site.Project.Name != "tidepool" {
		t.Fatalf("site %+v", site)
	}
}

func TestCollectOpenPointsCountsOnlyUnresolved(t *testing.T) {
	s := newStore(t)
	n := insert(t, s, note("2026-10-30T10:00:00Z", "A",
		model.Entry{Type: model.TypeOpen, Text: "q1"}, model.Entry{Type: model.TypeOpen, Text: "q2"}))
	if _, err := s.InsertAddendum(model.Addendum{NoteID: n.ID, Agent: "r", Model: "m", At: "2026-10-30T11:00:00Z",
		Resolves: []string{n.Entries[0].ID}}); err != nil {
		t.Fatal(err)
	}
	site := collect(t, s)
	if got := site.Notes[0].OpenPoints; got != 1 {
		t.Fatalf("open points %d", got)
	}
}

func TestCollectStates(t *testing.T) {
	s := newStore(t)
	n1 := insert(t, s, note("2026-10-30T10:00:00Z", "A", model.Entry{Type: model.TypeDecision, Text: "b-tree", By: "user"}))
	n2 := note("2026-10-30T12:00:00Z", "B", model.Entry{Type: model.TypeDecision, Text: "hash", By: "user"})
	n2.Supersedes = []string{n1.Entries[0].ID}
	n2 = insert(t, s, n2)
	site := collect(t, s)
	var first NoteView
	for _, nv := range site.Notes {
		if nv.Header.ID == n1.ID {
			first = nv
		}
	}
	st := first.States[n1.Entries[0].ID]
	if !st.Closed || st.ClosedBy != n2.ID {
		t.Fatalf("state %+v", st)
	}
}

func TestCollectNotesNewestFirst(t *testing.T) {
	s := newStore(t)
	insert(t, s, note("2026-10-30T10:00:00Z", "old", model.Entry{Type: model.TypeChange, Text: "x"}))
	insert(t, s, note("2026-10-30T12:00:00Z", "new", model.Entry{Type: model.TypeChange, Text: "y"}))
	site := collect(t, s)
	if site.Notes[0].Header.Title != "new" || site.Notes[1].Header.Title != "old" {
		t.Fatalf("order %q %q", site.Notes[0].Header.Title, site.Notes[1].Header.Title)
	}
	if site.Entries[0].Text != "y" {
		t.Fatalf("entries order %q", site.Entries[0].Text)
	}
}

func TestCollectEntryAgent(t *testing.T) {
	s := newStore(t)
	n := insert(t, s, note("2026-10-30T10:00:00Z", "A", model.Entry{Type: model.TypeChange, Text: "x"}))
	if _, err := s.InsertAddendum(model.Addendum{NoteID: n.ID, Agent: "reviewer", Model: "m", At: "2026-10-30T11:00:00Z",
		Entries: []model.Entry{{Type: model.TypeNote, Text: "late"}}}); err != nil {
		t.Fatal(err)
	}
	site := collect(t, s)
	got := map[string]string{}
	for _, e := range site.Entries {
		got[e.Text] = e.Agent
	}
	if got["x"] != "builder" || got["late"] != "reviewer" {
		t.Fatalf("agents %v", got)
	}
}

func TestCollectEmptyProject(t *testing.T) {
	site := collect(t, newStore(t))
	if site.Notes == nil || len(site.Notes) != 0 || site.Entries == nil || len(site.Entries) != 0 {
		t.Fatalf("notes %v entries %v", site.Notes, site.Entries)
	}
}

func TestEntriesOfFiltersByType(t *testing.T) {
	s := newStore(t)
	insert(t, s, note("2026-10-30T10:00:00Z", "A",
		model.Entry{Type: model.TypeChange, Text: "c"}, model.Entry{Type: model.TypeNote, Text: "n"}))
	site := collect(t, s)
	got := site.EntriesOf(model.TypeNote)
	if len(got) != 1 || got[0].Text != "n" {
		t.Fatalf("got %+v", got)
	}
}

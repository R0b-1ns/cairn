package store

import (
	"testing"

	"github.com/R0b-1ns/cairn/internal/model"
)

type seeded struct {
	note1, note2, a1         string
	change, d1, o1, d2, n2ch string
}

func seed(t *testing.T, s *Store) seeded {
	t.Helper()
	n1 := sampleNote() // TP-03, server, impacts client, change/decision/open
	n1.At = "2026-10-30T10:00:00Z"
	n1, _ = s.InsertNote(n1)
	var sd seeded
	sd.note1 = n1.ID
	sd.change = n1.Entries[0].ID
	sd.o1, sd.d1 = entryIDs(n1)
	a1 := addAddendum(t, s, model.Addendum{NoteID: n1.ID, At: "2026-10-30T11:00:00Z",
		Resolves: []string{sd.o1}, Supersedes: []string{sd.d1},
		Entries: []model.Entry{{Type: model.TypeDecision, Text: "Use a hash index.", By: "agent"}}})
	sd.a1, sd.d2 = a1.ID, a1.Entries[0].ID
	n2 := sampleNote()
	n2.Task, n2.Repo, n2.Impacts, n2.At = "TP-03.1", "client", nil, "2026-11-02T09:00:00Z"
	n2.Entries = []model.Entry{{Type: model.TypeChange, Text: "Client uses the index."}}
	n2.None = []model.EntryType{model.TypeDecision, model.TypeOpen, model.TypeNote, model.TypeHelper}
	n2, err := s.InsertNote(n2)
	if err != nil {
		t.Fatal(err)
	}
	sd.note2, sd.n2ch = n2.ID, n2.Entries[0].ID
	return sd
}

func idsOf(items []EntryInfo) map[string]bool {
	m := map[string]bool{}
	for _, e := range items {
		m[e.ID] = true
	}
	return m
}

func TestListEntriesStates(t *testing.T) {
	s, _ := newStore(t)
	sd := seed(t, s)
	items, total, err := s.ListEntries(Filter{State: StateActive})
	got := idsOf(items)
	if err != nil || total != 3 || !got[sd.change] || !got[sd.d2] || !got[sd.n2ch] || got[sd.o1] || got[sd.d1] {
		t.Fatalf("active: %v total=%d err=%v", got, total, err)
	}
	items, total, _ = s.ListEntries(Filter{State: StateClosed})
	got = idsOf(items)
	if total != 2 || !got[sd.o1] || !got[sd.d1] {
		t.Fatalf("closed: %v", got)
	}
	for _, e := range items {
		if !e.Closed || e.ClosedBy != sd.a1 {
			t.Fatalf("closed marker: %+v", e)
		}
	}
	_, total, _ = s.ListEntries(Filter{State: StateAll})
	if total != 5 {
		t.Fatalf("all: total=%d", total)
	}
}

func TestListEntriesFilters(t *testing.T) {
	s, _ := newStore(t)
	sd := seed(t, s)
	count := func(f Filter) (map[string]bool, int) {
		f.State = StateAll
		items, total, err := s.ListEntries(f)
		if err != nil {
			t.Fatal(err)
		}
		return idsOf(items), total
	}
	if g, n := count(Filter{TaskBase: "TP-03"}); n != 5 || !g[sd.n2ch] {
		t.Errorf("base includes extensions: %v", g)
	}
	if g, n := count(Filter{Task: "TP-03.1"}); n != 1 || !g[sd.n2ch] {
		t.Errorf("exact task: %v", g)
	}
	if g, n := count(Filter{Repo: "client"}); n != 1 || !g[sd.n2ch] {
		t.Errorf("repo: %v", g)
	}
	if _, n := count(Filter{Impacts: "client"}); n != 4 { // note1 entries (3) + its addendum entry
		t.Errorf("impacts: %d", n)
	}
	if g, n := count(Filter{Types: []model.EntryType{model.TypeOpen, model.TypeDecision}}); n != 3 || g[sd.change] {
		t.Errorf("types: %v", g)
	}
	if g, n := count(Filter{Since: "2026-11-01T00:00:00Z"}); n != 1 || !g[sd.n2ch] {
		t.Errorf("since: %v", g)
	}
	if _, n := count(Filter{Until: "2026-10-30T11:00:00Z"}); n != 3 { // until is exclusive: the addendum entry is out
		t.Errorf("until: %d", n)
	}
	if _, n := count(Filter{Repo: "server", Types: []model.EntryType{model.TypeChange}}); n != 1 {
		t.Errorf("combined: %d", n)
	}
}

func TestListEntriesOrderAndLimit(t *testing.T) {
	s, _ := newStore(t)
	sd := seed(t, s)
	items, total, _ := s.ListEntries(Filter{State: StateAll, Limit: 2})
	if total != 5 || len(items) != 2 || items[0].ID != sd.n2ch || items[1].ID != sd.d2 {
		t.Fatalf("newest first with limit: %+v total=%d", items, total)
	}
}

func TestListEntriesFilterValuesAreData(t *testing.T) {
	s, _ := newStore(t)
	seed(t, s)
	for _, f := range []Filter{{Repo: "x' OR '1'='1"}, {Impacts: `"); DROP TABLE notes;--`}, {TaskBase: "%"}} {
		items, total, err := s.ListEntries(f)
		if err != nil || total != 0 || len(items) != 0 {
			t.Fatalf("filter %+v matched: %d %v", f, total, err)
		}
	}
}

func TestSubjects(t *testing.T) {
	s, _ := newStore(t)
	sd := seed(t, s)
	loose := sampleNote()
	loose.Task, loose.At = "", "2026-10-31T08:00:00Z"
	loose, _ = s.InsertNote(loose)
	subs, err := s.Subjects(10)
	if err != nil || len(subs) != 2 {
		t.Fatalf("%+v %v", subs, err)
	}
	// TP-03 (two notes, latest activity 2026-11-02) before the task-less note (2026-10-31).
	if subs[0].Key != "TP-03" || !subs[0].IsTask || subs[0].Notes != 2 || subs[0].LastAt != "2026-11-02T09:00:00Z" || subs[0].NoteID != sd.note2 {
		t.Fatalf("first: %+v", subs[0])
	}
	if subs[1].Key != loose.ID || subs[1].IsTask || subs[1].Notes != 1 {
		t.Fatalf("second: %+v", subs[1])
	}
	one, _ := s.Subjects(1)
	if len(one) != 1 || one[0].Key != "TP-03" {
		t.Fatalf("limit: %+v", one)
	}
	// An addendum counts as activity on its note's subject.
	addAddendum(t, s, model.Addendum{NoteID: loose.ID, At: "2026-12-01T00:00:00Z", Status: model.StatusDone})
	subs, _ = s.Subjects(10)
	if subs[0].Key != loose.ID || subs[0].LastAt != "2026-12-01T00:00:00Z" {
		t.Fatalf("addendum activity: %+v", subs)
	}
}

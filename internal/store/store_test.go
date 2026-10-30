package store

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/R0b-1ns/cairn/internal/model"
)

func testProject() model.Project {
	return model.Project{
		Name: "tidepool", Mode: model.ModeProject, Context: "Tide data service.",
		Repos: []string{"server", "client"}, TaskPattern: model.DefaultTaskPattern,
	}
}

func sampleNote() model.Note {
	return model.Note{
		Agent: "builder", Repo: "server", At: "2026-10-30T10:00:00Z", Task: "TP-03",
		Model: "claude-test", Title: "Add tidal index", Status: model.StatusInProgress,
		Entries: []model.Entry{
			{Type: model.TypeChange, Text: "Added the index."},
			{Type: model.TypeDecision, Text: "Use a b-tree.", By: "user"},
			{Type: model.TypeOpen, Text: "Should it be rebuilt nightly?"},
		},
		None:    []model.EntryType{model.TypeNote, model.TypeHelper},
		Impacts: []string{"client"},
		Commits: []string{"abc1234"},
	}
}

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".cairn", "cairn.db")
	s, err := Create(path, testProject())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func rawDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func entryIDs(n model.Note) (open, decision string) {
	for _, e := range n.Entries {
		switch e.Type {
		case model.TypeOpen:
			open = e.ID
		case model.TypeDecision:
			decision = e.ID
		}
	}
	return
}

func TestInsertAssignsIDsAndRoundTrips(t *testing.T) {
	s, _ := newStore(t)
	n, err := s.InsertNote(sampleNote())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(n.ID, "n-") {
		t.Fatalf("note id %q", n.ID)
	}
	for _, e := range n.Entries {
		if !strings.HasPrefix(e.ID, "e-") {
			t.Fatalf("entry id %q", e.ID)
		}
	}
	got, err := s.GetNote(n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != n.Title || len(got.Entries) != 3 || got.Entries[1].By != "user" ||
		got.Entries[0].Type != model.TypeChange || got.Impacts[0] != "client" ||
		got.Commits[0] != "abc1234" || len(got.None) != 2 || got.Task != "TP-03" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestInsertPreservesGivenIDsAndRejectsDuplicates(t *testing.T) {
	s, _ := newStore(t)
	n := sampleNote()
	n.ID = "n-abcdef"
	n.Entries[0].ID = "e-aaaaaa"
	if _, err := s.InsertNote(n); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetNote("n-abcdef")
	if got.Entries[0].ID != "e-aaaaaa" {
		t.Fatalf("id not preserved: %+v", got.Entries[0])
	}
	if _, err := s.InsertNote(n); !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestImmutabilityTriggers(t *testing.T) {
	s, path := newStore(t)
	n, _ := s.InsertNote(sampleNote())
	open, _ := entryIDs(n)
	if _, err := s.InsertAddendum(model.Addendum{
		NoteID: n.ID, Agent: "builder", Model: "claude-test", At: "2026-10-30T11:00:00Z",
		Resolves: []string{open},
	}); err != nil {
		t.Fatal(err)
	}
	db := rawDB(t, path)
	updates := map[string]string{
		"project": "UPDATE project SET name='x'",
		"notes":   "UPDATE notes SET title='x'",
		"entries": "UPDATE entries SET text='x'",
		"addenda": "UPDATE addenda SET agent='x'",
		"links":   "UPDATE links SET kind='x'",
	}
	for table, stmt := range updates {
		if _, err := db.Exec(stmt); err == nil || !strings.Contains(err.Error(), "immutable") {
			t.Errorf("%s UPDATE not rejected: %v", table, err)
		}
		if _, err := db.Exec("DELETE FROM " + table); err == nil || !strings.Contains(err.Error(), "immutable") {
			t.Errorf("%s DELETE not rejected: %v", table, err)
		}
	}
}

func TestInjectionStringsAreData(t *testing.T) {
	s, _ := newStore(t)
	n := sampleNote()
	n.Title = `x'); DROP TABLE notes;--`
	n.Entries[0].Text = `"; DELETE FROM entries; --`
	in, err := s.InsertNote(n)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetNote(in.ID)
	if err != nil || got.Title != n.Title || got.Entries[0].Text != n.Entries[0].Text {
		t.Fatalf("not stored verbatim: %v %+v", err, got)
	}
}

func TestOpenRejectsMissingTriggers(t *testing.T) {
	s, path := newStore(t)
	s.Close()
	db := rawDB(t, path)
	if _, err := db.Exec("DROP TRIGGER notes_no_update"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "trigger") {
		t.Fatalf("want trigger error, got %v", err)
	}
}

func TestOpenRejectsForeignSchemaObjects(t *testing.T) {
	cases := map[string]string{
		"extra trigger": "CREATE TRIGGER sneaky AFTER INSERT ON notes BEGIN SELECT 1; END",
		"extra view":    "CREATE VIEW v AS SELECT * FROM notes",
		"extra index":   "CREATE INDEX sneaky_idx ON notes(title)",
		"extra table":   "CREATE TABLE sneaky(x)",
		"altered table": "ALTER TABLE notes ADD COLUMN extra TEXT",
	}
	for name, stmt := range cases {
		t.Run(name, func(t *testing.T) {
			s, path := newStore(t)
			s.Close()
			db := rawDB(t, path)
			if _, err := db.Exec(stmt); err != nil {
				t.Fatal(err)
			}
			db.Close()
			if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "schema") {
				t.Fatalf("want schema error, got %v", err)
			}
		})
	}
}

func TestOpenRejectsBadVersionAndNonCairnDB(t *testing.T) {
	s, path := newStore(t)
	s.Close()
	db := rawDB(t, path)
	if _, err := db.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("want version error, got %v", err)
	}
	other := filepath.Join(t.TempDir(), "other.db")
	d2 := rawDB(t, other)
	d2.Exec("CREATE TABLE t(x)")
	d2.Close()
	if _, err := Open(other); err == nil {
		t.Fatal("foreign database must be refused")
	}
}

func TestOpenMissingFileDoesNotCreate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope", "cairn.db")
	if _, err := Open(path); err == nil {
		t.Fatal("want error")
	}
	if _, err := os.Stat(filepath.Dir(path)); err == nil {
		t.Fatal("Open created a directory")
	}
}

func TestCreateRefusesExisting(t *testing.T) {
	_, path := newStore(t)
	if _, err := Create(path, testProject()); err == nil {
		t.Fatal("second Create must fail")
	}
}

func TestFileModes(t *testing.T) {
	_, path := newStore(t)
	fi, _ := os.Stat(path)
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("db mode %v", fi.Mode().Perm())
	}
	di, _ := os.Stat(filepath.Dir(path))
	if di.Mode().Perm() != 0o700 {
		t.Errorf("dir mode %v", di.Mode().Perm())
	}
}

func TestConcurrentAdds(t *testing.T) {
	_, path := newStore(t)
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	ids := make(chan string, 40)
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := Open(path)
			if err != nil {
				errs <- err
				return
			}
			defer s.Close()
			for i := 0; i < 20; i++ {
				n, err := s.InsertNote(sampleNote())
				if err != nil {
					errs <- err
					return
				}
				ids <- n.ID
			}
		}()
	}
	wg.Wait()
	close(errs)
	close(ids)
	for err := range errs {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate id %s", id)
		}
		seen[id] = true
	}
	if len(seen) != 40 {
		t.Fatalf("want 40 notes, got %d", len(seen))
	}
}

func TestLinkTargetTypeChecked(t *testing.T) {
	s, _ := newStore(t)
	n, _ := s.InsertNote(sampleNote())
	open, decision := entryIDs(n)
	add := func(resolves, supersedes []string) error {
		_, err := s.InsertAddendum(model.Addendum{NoteID: n.ID, Agent: "a", Model: "m",
			At: "2026-10-30T11:00:00Z", Resolves: resolves, Supersedes: supersedes})
		return err
	}
	if err := add([]string{decision}, nil); err == nil {
		t.Error("resolving a decision must fail")
	}
	if err := add(nil, []string{open}); err == nil {
		t.Error("superseding an open point must fail")
	}
	if err := add([]string{"e-zzzzzz"}, nil); err == nil {
		t.Error("unknown target must fail")
	}
	if err := add([]string{open}, nil); err != nil {
		t.Fatalf("valid resolve failed: %v", err)
	}
	if err := add([]string{open}, nil); err == nil {
		t.Error("already resolved target must fail")
	}
	if err := add(nil, []string{decision}); err != nil {
		t.Fatalf("valid supersede failed: %v", err)
	}
	if err := add(nil, []string{decision}); err == nil {
		t.Error("already superseded target must fail")
	}
}

func TestNoteLevelLinksAcrossNotes(t *testing.T) {
	s, _ := newStore(t)
	first, _ := s.InsertNote(sampleNote())
	open, _ := entryIDs(first)
	second := sampleNote()
	second.Task = "TP-04"
	second.Resolves = []string{open}
	if _, err := s.InsertNote(second); err != nil {
		t.Fatalf("cross-note resolve: %v", err)
	}
}

func TestInsertIsAtomic(t *testing.T) {
	s, path := newStore(t)
	n, _ := s.InsertNote(sampleNote())
	_, err := s.InsertAddendum(model.Addendum{NoteID: n.ID, Agent: "a", Model: "m",
		At: "2026-10-30T11:00:00Z", Entries: []model.Entry{{Type: model.TypeNote, Text: "x"}},
		Resolves: []string{"e-zzzzzz"}})
	if err == nil {
		t.Fatal("want error")
	}
	var c int
	rawDB(t, path).QueryRow("SELECT (SELECT COUNT(*) FROM addenda)+(SELECT COUNT(*) FROM entries WHERE addendum_id IS NOT NULL)").Scan(&c)
	if c != 0 {
		t.Fatalf("partial insert left %d rows", c)
	}
}

func TestAddendumUnknownNote(t *testing.T) {
	s, _ := newStore(t)
	_, err := s.InsertAddendum(model.Addendum{NoteID: "n-zzzzzz", Agent: "a", Model: "m",
		At: "2026-10-30T11:00:00Z", Status: model.StatusDone})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// INSERT OR REPLACE deletes the old row without firing DELETE triggers, so it
// must be refused by its own trigger on every content table.
func TestReplaceCannotRewriteHistory(t *testing.T) {
	s, path := newStore(t)
	n, _ := s.InsertNote(sampleNote())
	open, _ := entryIDs(n)
	addAddendum(t, s, model.Addendum{NoteID: n.ID, At: "2026-10-30T11:00:00Z", Resolves: []string{open}})
	db := rawDB(t, path)
	stmts := map[string]string{
		"project":             "INSERT OR REPLACE INTO project SELECT id,'hijacked',mode,context,repos,default_repo,task_pattern FROM project",
		"notes":               "INSERT OR REPLACE INTO notes SELECT id,agent,repo,at,task,task_base,task_ext,model,'REWRITTEN',status,none,impacts,commits FROM notes",
		"entries":             "INSERT OR REPLACE INTO entries SELECT id,note_id,addendum_id,pos,type,'REWRITTEN',decided_by FROM entries",
		"addenda":             "INSERT OR REPLACE INTO addenda SELECT id,note_id,'REWRITTEN',model,at,status FROM addenda",
		"links":               "INSERT OR REPLACE INTO links SELECT seq,kind,target_entry_id,source_note_id,source_addendum_id FROM links",
		"links by unique key": "REPLACE INTO links(kind,target_entry_id,source_note_id) SELECT kind,target_entry_id,source_addendum_id IS NULL OR 1 FROM links",
	}
	for name, stmt := range stmts {
		if _, err := db.Exec(stmt); err == nil || !strings.Contains(err.Error(), "immutable") {
			t.Errorf("%s: REPLACE not rejected: %v", name, err)
		}
	}
	var title, agent, project string
	db.QueryRow("SELECT title FROM notes").Scan(&title)
	db.QueryRow("SELECT agent FROM addenda").Scan(&agent)
	db.QueryRow("SELECT name FROM project").Scan(&project)
	if title != "Add tidal index" || agent != "builder" || project != "tidepool" {
		t.Fatalf("history was rewritten: %q %q %q", title, agent, project)
	}
	var links int
	db.QueryRow("SELECT COUNT(*) FROM links").Scan(&links)
	if links != 1 {
		t.Fatalf("links = %d", links)
	}
}

func TestProjectDefaultRepoRoundTrip(t *testing.T) {
	p := testProject()
	p.DefaultRepo = "client"
	path := filepath.Join(t.TempDir(), ".cairn", "cairn.db")
	s, err := Create(path, p)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, _ := s.Project()
	if got.DefaultRepo != "client" {
		t.Fatalf("default repo = %q", got.DefaultRepo)
	}
}

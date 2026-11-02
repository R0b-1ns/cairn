package store

import (
	"bytes"
	"errors"
	"github.com/R0b-1ns/cairn/internal/model"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// makeV3 builds a version 3 database by hand, with one project and one note.
func makeV3(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cairn.db")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	db := rawDB(t, path)
	if _, err := db.Exec(tableSQLv3); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range triggerSQL() {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO project(id,name,mode,context,repos,default_repo,task_pattern) VALUES (1,'old','project','c','["server"]','','^x$')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO notes(id,agent,repo,at,task,task_base,task_ext,model,title,status,none,impacts,commits) VALUES ('n-old','a','server','2026-10-30T10:00:00Z','','','','m','Old note','done','[]','[]','[]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version = 3"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	return path
}

func userVersion(t *testing.T, path string) int {
	t.Helper()
	db := rawDB(t, path)
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestMigrateV3ToV5(t *testing.T) {
	path := makeV3(t)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if v := userVersion(t, path); v != 5 {
		t.Fatalf("user_version = %d, want 5", v)
	}
	n, err := s.GetNote("n-old")
	if err != nil {
		t.Fatal(err)
	}
	if n.Branch != "" || n.AuthorName != "" {
		t.Fatalf("old note should read back with empty new fields: %+v", n)
	}
	bak := path + ".v3.bak"
	fi, err := os.Stat(bak)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("backup mode = %v, want 0600", fi.Mode().Perm())
	}
	if v := userVersion(t, bak); v != 3 {
		t.Fatalf("backup user_version = %d, want 3", v)
	}
	if err := verifySchema(s.db, SchemaVersion); err != nil {
		t.Fatalf("migrated schema must equal a fresh one: %v", err)
	}
}

func TestMigrateConcurrent(t *testing.T) {
	path := makeV3(t)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := Open(path)
			if err == nil {
				s.Close()
			}
			errs[i] = err
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	db := rawDB(t, path)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('notes') WHERE name='branch'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("branch columns = %d, want 1", n)
	}
	if v := userVersion(t, path+".v3.bak"); v != 3 {
		t.Fatalf("backup user_version = %d, want 3", v)
	}
}

func TestMigrateRefusesWhenBackupFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("read-only directories do not stop root")
	}
	path := makeV3(t)
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })
	_, err := Open(path)
	if err == nil || !strings.Contains(err.Error(), "backup") {
		t.Fatalf("want backup error, got %v", err)
	}
	os.Chmod(dir, 0o700)
	if v := userVersion(t, path); v != 3 {
		t.Fatalf("user_version = %d, want 3", v)
	}
}

func TestOldVersionStillRefused(t *testing.T) {
	path := makeV3(t)
	db := rawDB(t, path)
	if _, err := db.Exec("PRAGMA user_version = 2"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	_, err := Open(path)
	if err == nil || !strings.Contains(err.Error(), "unsupported database version 2 (this build supports 5)") {
		t.Fatalf("got %v", err)
	}
}

func TestHostileV3IsRefusedUnchanged(t *testing.T) {
	cases := map[string]string{
		"altered trigger": "DROP TRIGGER notes_no_update; CREATE TRIGGER notes_no_update BEFORE UPDATE ON notes BEGIN SELECT 1; END",
		"extra table":     "CREATE TABLE evil(x)",
	}
	for name, stmt := range cases {
		t.Run(name, func(t *testing.T) {
			path := makeV3(t)
			db := rawDB(t, path)
			if _, err := db.Exec(stmt); err != nil {
				t.Fatal(err)
			}
			db.Close()
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Open(path); err == nil {
				t.Fatal("hostile version 3 database must be refused")
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("refused database was modified")
			}
			if _, err := os.Stat(path + ".v3.bak"); err == nil {
				t.Fatal("no backup may be made for a refused database")
			}
		})
	}
}

func TestExistingBackupIsOverwritten(t *testing.T) {
	path := makeV3(t)
	if err := os.WriteFile(path+".v3.bak", []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	if v := userVersion(t, path+".v3.bak"); v != 3 {
		t.Fatalf("backup user_version = %d, want 3", v)
	}
}

func TestCurrentDatabaseMakesNoBackup(t *testing.T) {
	s, path := newStore(t)
	s.Close()
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s2.Close()
	if _, err := os.Stat(path + ".v3.bak"); err == nil {
		t.Fatal("a current database must not be backed up")
	}
}

func TestMigrateKeepsEntriesAddendaAndSearch(t *testing.T) {
	path := makeV3(t)
	db := rawDB(t, path)
	for _, q := range []string{
		`INSERT INTO entries(id,note_id,addendum_id,pos,type,text,decided_by) VALUES ('e-aaaaaa','n-old',NULL,0,'change','Wrote the zebra index.','')`,
		`INSERT INTO addenda(id,note_id,agent,model,at,status) VALUES ('a-bbbbbb','n-old','a','m','2026-10-30T11:00:00Z','done')`,
		`INSERT INTO entries(id,note_id,addendum_id,pos,type,text,decided_by) VALUES ('e-cccccc',NULL,'a-bbbbbb',0,'note','Follow-up giraffe remark.','')`,
		`INSERT INTO search(text,kind,id,note_id) VALUES ('Wrote the zebra index.','entry','e-aaaaaa','n-old')`,
		`INSERT INTO search(text,kind,id,note_id) VALUES ('Follow-up giraffe remark.','entry','e-cccccc','n-old')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	n, err := s.GetNote("n-old")
	if err != nil {
		t.Fatal(err)
	}
	if n.Title != "Old note" || len(n.Entries) != 1 || n.Entries[0].Text != "Wrote the zebra index." {
		t.Fatalf("note read back wrong: %+v", n)
	}
	a, err := s.GetAddendum("a-bbbbbb")
	if err != nil {
		t.Fatal(err)
	}
	if a.NoteID != "n-old" || len(a.Entries) != 1 || a.Entries[0].Text != "Follow-up giraffe remark." {
		t.Fatalf("addendum read back wrong: %+v", a)
	}
	for _, term := range []string{"zebra", "giraffe"} {
		hits, _, err := s.Search([]string{term}, 10)
		if err != nil || len(hits) == 0 {
			t.Fatalf("search %q: hits=%v err=%v", term, hits, err)
		}
	}
}

func TestNewerVersionSaysUpdate(t *testing.T) {
	s, path := newStore(t)
	s.Close()
	db := rawDB(t, path)
	if _, err := db.Exec("PRAGMA user_version = 6"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	_, err := Open(path)
	want := "unsupported database version 6 (this build supports 5); run 'cairn update' to upgrade"
	if err == nil || err.Error() != want {
		t.Fatalf("got %v, want %q", err, want)
	}
}

func TestOpenCurrentNeverWrites(t *testing.T) {
	path := makeV3(t)
	db := rawDB(t, path)
	if _, err := db.Exec(migrations[0].sql); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version = 4"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	before, _ := os.ReadFile(path)
	fiBefore, _ := os.Stat(path)
	_, err := OpenCurrent(path)
	var nm *NeedsMigrationError
	if !errors.As(err, &nm) || nm.Version != 4 || nm.Path != path || !strings.Contains(err.Error(), path) {
		t.Fatalf("err = %v", err)
	}
	after, _ := os.ReadFile(path)
	fiAfter, _ := os.Stat(path)
	if !bytes.Equal(before, after) || !fiBefore.ModTime().Equal(fiAfter.ModTime()) {
		t.Error("the file changed")
	}
	matches, _ := filepath.Glob(path + "*")
	if len(matches) != 1 {
		t.Errorf("extra files: %v", matches)
	}
	// A current database behaves like Open.
	cur := filepath.Join(t.TempDir(), "cairn.db")
	c, err := Create(cur, model.Project{Name: "p", Mode: "project", Repos: []string{"a"}, TaskPattern: "^x$"})
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	s, err := OpenCurrent(cur)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
}

package store

import (
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

func TestFreshDatabaseIsVersion4(t *testing.T) {
	s, path := newStore(t)
	s.Close()
	if v := userVersion(t, path); v != 4 {
		t.Fatalf("user_version = %d, want 4", v)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s2.Close()
}

func TestMigrateV3ToV4(t *testing.T) {
	path := makeV3(t)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if v := userVersion(t, path); v != 4 {
		t.Fatalf("user_version = %d, want 4", v)
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
	if err := s.verifySchema(); err != nil {
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
	if err == nil || !strings.Contains(err.Error(), "unsupported database version 2 (this build supports 4)") {
		t.Fatalf("got %v", err)
	}
}

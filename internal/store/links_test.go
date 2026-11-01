package store

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/R0b-1ns/cairn/internal/model"
)

// makeV4 builds a version 4 database by hand, with one project and one note.
func makeV4(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cairn.db")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	db := rawDB(t, path)
	if err := buildSchema(db, 4); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO project(id,name,mode,context,repos,default_repo,task_pattern) VALUES (1,'old','project','c','["server"]','','^x$')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO notes(id,agent,repo,at,task,task_base,task_ext,model,title,status,none,impacts,commits) VALUES ('n-old','a','server','2026-10-30T10:00:00Z','','','','m','Old note','done','[]','[]','[]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version = 4"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	return path
}

func countLinkEvents(t *testing.T, path string) int {
	t.Helper()
	var n int
	if err := rawDB(t, path).QueryRow(`SELECT COUNT(*) FROM link_events`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMigrateV4ToV5(t *testing.T) {
	path := makeV4(t)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if v := userVersion(t, path); v != 5 {
		t.Fatalf("user_version = %d, want 5", v)
	}
	n, err := s.GetNote("n-old")
	if err != nil || n.Title != "Old note" {
		t.Fatalf("note read back wrong: %+v %v", n, err)
	}
	bak := path + ".v4.bak"
	fi, err := os.Stat(bak)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("backup mode = %v, want 0600", fi.Mode().Perm())
	}
	if v := userVersion(t, bak); v != 4 {
		t.Fatalf("backup user_version = %d, want 4", v)
	}
	if err := verifySchema(s.db, SchemaVersion); err != nil {
		t.Fatalf("migrated schema must equal a fresh one: %v", err)
	}
	if n := countLinkEvents(t, path); n != 0 {
		t.Fatalf("link_events rows = %d, want 0", n)
	}
}

func TestMigrateV3ToV5InOneOpen(t *testing.T) {
	path := makeV3(t)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if v := userVersion(t, path); v != 5 {
		t.Fatalf("user_version = %d, want 5", v)
	}
	if _, err := os.Stat(path + ".v3.bak"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".v4.bak"); err == nil {
		t.Fatal("a chained migration must make only the .v3.bak backup")
	}
	n, err := s.GetNote("n-old")
	if err != nil || n.Title != "Old note" {
		t.Fatalf("note read back wrong: %+v %v", n, err)
	}
	if err := verifySchema(s.db, SchemaVersion); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateV4ToV5Concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("starts subprocesses")
	}
	path := makeV4(t)
	goFile := filepath.Join(filepath.Dir(path), "go")
	cmds := make([]*exec.Cmd, helperProcs)
	outs := make([]*bytes.Buffer, helperProcs)
	for i := range cmds {
		cmd := exec.Command(os.Args[0], "-test.run=^TestMigrateHelperProcess$", "-test.count=1")
		cmd.Env = append(os.Environ(), helperDBEnv+"="+path, helperGoEnv+"="+goFile, helperLagEnv+"="+strconv.Itoa(i*3))
		outs[i] = &bytes.Buffer{}
		cmd.Stdout, cmd.Stderr = outs[i], outs[i]
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		cmds[i] = cmd
	}
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(goFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for i, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Errorf("process %d: %v: %s", i, err, outs[i].String())
		}
	}
	if t.Failed() {
		t.FailNow()
	}
	if v := userVersion(t, path); v != 5 {
		t.Fatalf("user_version = %d, want 5", v)
	}
	var n int
	if err := rawDB(t, path).QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='link_events'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("link_events tables = %d (%v), want 1", n, err)
	}
	if v := userVersion(t, path+".v4.bak"); v != 4 {
		t.Fatalf("backup user_version = %d, want 4", v)
	}
}

func TestFreshDatabaseIsVersion5(t *testing.T) {
	s, path := newStore(t)
	s.Close()
	if v := userVersion(t, path); v != 5 {
		t.Fatalf("user_version = %d, want 5", v)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s2.Close()
	for _, name := range []string{"link_events_no_update", "link_events_no_delete", "link_events_no_replace"} {
		var n int
		if err := rawDB(t, path).QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name=?`, name).Scan(&n); err != nil || n != 1 {
			t.Fatalf("trigger %s: count %d err %v", name, n, err)
		}
	}
}

func TestLinkEventsAreImmutable(t *testing.T) {
	s, _ := newStore(t)
	if _, err := s.EnsureProjectID(); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`UPDATE link_events SET name='x'`,
		`DELETE FROM link_events`,
		`INSERT OR REPLACE INTO link_events(seq,kind,name) VALUES (1,'id','p-aaaaaaaaaa')`,
	} {
		_, err := s.db.Exec(q)
		if err == nil || !strings.Contains(err.Error(), "immutable") {
			t.Fatalf("%s: want immutable error, got %v", q, err)
		}
	}
}

func TestEnsureProjectIDIsStable(t *testing.T) {
	s, path := newStore(t)
	a, err := s.EnsureProjectID()
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.EnsureProjectID()
	if err != nil {
		t.Fatal(err)
	}
	if a != b || !strings.HasPrefix(a, "p-") || len(a) != 12 {
		t.Fatalf("ids %q %q", a, b)
	}
	if n := countLinkEvents(t, path); n != 1 {
		t.Fatalf("events = %d, want 1", n)
	}
	l, err := s.Links()
	if err != nil || l.ID != a {
		t.Fatalf("Links().ID = %q, %v", l.ID, err)
	}
}

func TestAddRepoMergesIntoProject(t *testing.T) {
	s, _ := newStore(t)
	if err := s.AddRepo("web"); err != nil {
		t.Fatal(err)
	}
	p, err := s.Project()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"server", "client", "web"}; !reflect.DeepEqual(p.Repos, want) {
		t.Fatalf("repos = %v, want %v", p.Repos, want)
	}
	if err := s.AddRepo("web"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddRepo("server"); err != nil {
		t.Fatal(err)
	}
	p, _ = s.Project()
	if len(p.Repos) != 3 {
		t.Fatalf("no-op adds changed repos: %v", p.Repos)
	}
	if err := s.AddRepo("Bad Name"); err == nil {
		t.Fatal("invalid repo name must error")
	}
	for i := 0; len(p.Repos) < 50; i++ {
		if err := s.AddRepo("r" + strconv.Itoa(i)); err != nil {
			t.Fatal(err)
		}
		p, _ = s.Project()
	}
	if err := s.AddRepo("one-too-many"); err == nil {
		t.Fatal("adding past 50 repos must error")
	}
	l, _ := s.Links()
	if len(l.ExtraRepos) != 48 {
		t.Fatalf("extra repos = %d, want 48", len(l.ExtraRepos))
	}
}

func TestChildrenReplay(t *testing.T) {
	s, _ := newStore(t)
	a := model.Child{Name: "a", Repo: "a-repo", Kind: model.ChildAttached}
	b := model.Child{Name: "b", Repo: "b-repo", Kind: model.ChildLocal}
	for _, c := range []model.Child{a, b} {
		if err := s.LinkChild(c); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DetachChild("a"); err != nil {
		t.Fatal(err)
	}
	l, err := s.Links()
	if err != nil || !reflect.DeepEqual(l.Children, []model.Child{b}) {
		t.Fatalf("children = %+v, %v", l.Children, err)
	}
	if err := s.LinkChild(b); err == nil {
		t.Fatal("linking an active child again must error")
	}
	if err := s.LinkChild(a); err != nil {
		t.Fatal(err)
	}
	l, _ = s.Links()
	if !reflect.DeepEqual(l.Children, []model.Child{b, a}) {
		t.Fatalf("children = %+v, want [b a]", l.Children)
	}
	if err := s.DetachChild("zzz"); err == nil {
		t.Fatal("detaching an unknown child must error")
	}
}

func TestParentReplay(t *testing.T) {
	s, _ := newStore(t)
	p := model.ParentRef{Name: "tidepool", ID: "p-0123456789"}
	if err := s.SetParent(p); err != nil {
		t.Fatal(err)
	}
	l, err := s.Links()
	if err != nil || l.Parent == nil || *l.Parent != p {
		t.Fatalf("parent = %+v, %v", l.Parent, err)
	}
	if err := s.ClearParent(); err != nil {
		t.Fatal(err)
	}
	l, _ = s.Links()
	if l.Parent != nil {
		t.Fatalf("parent = %+v, want nil", l.Parent)
	}
}

func TestReplayRefusesInvalidEvents(t *testing.T) {
	for _, c := range []struct{ kind, name, repo, pid string }{
		{"id", "p-bad\x1b[2J", "", ""},
		{"repo_added", "", "Bad Repo", ""},
		{"child_linked", "web\x1b[2J", "web", ""},
		{"child_linked", "web", "bad repo", ""},
		{"child_detached", "Bad Name", "", ""},
		{"parent_set", "shop", "", "p-zzz"},
		{"parent_set", "Bad\nName", "", ""},
	} {
		s, _ := newStore(t)
		if _, err := s.db.Exec(`INSERT INTO link_events(kind,name,repo,parent_id) VALUES (?,?,?,?)`, c.kind, c.name, c.repo, c.pid); err != nil {
			t.Fatal(err)
		}
		_, err := s.Links()
		if err == nil || !strings.Contains(err.Error(), "link event 1") {
			t.Errorf("%+v: want refusal naming seq 1, got %v", c, err)
		}
	}
}

func TestLinkWritersRejectInvalidValues(t *testing.T) {
	s, path := newStore(t)
	if err := s.LinkChild(model.Child{Name: "Bad\x1b", Repo: "web", Kind: model.ChildAttached}); err == nil {
		t.Error("LinkChild accepted a bad name")
	}
	if err := s.LinkChild(model.Child{Name: "web", Repo: "Bad Repo", Kind: model.ChildAttached}); err == nil {
		t.Error("LinkChild accepted a bad repo")
	}
	if err := s.SetParent(model.ParentRef{Name: "Bad Name"}); err == nil {
		t.Error("SetParent accepted a bad name")
	}
	if err := s.SetParent(model.ParentRef{Name: "shop", ID: "nope"}); err == nil {
		t.Error("SetParent accepted a bad id")
	}
	if err := s.AddRepo("Bad Repo"); err == nil {
		t.Error("AddRepo accepted a bad repo")
	}
	if n := countLinkEvents(t, path); n != 0 {
		t.Errorf("rejected writes left %d events", n)
	}
}

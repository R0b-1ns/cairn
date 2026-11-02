package cli

import (
	"bytes"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/R0b-1ns/cairn/internal/locate"
	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
	_ "modernc.org/sqlite"
)

// parentLinks opens the parent's database (the env root) and replays its links.
func parentLinks(t *testing.T, e *testEnv) store.LinkState {
	t.Helper()
	st, err := store.Open(filepath.Join(e.root, ".cairn", "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ls, err := st.Links()
	if err != nil {
		t.Fatal(err)
	}
	return ls
}

// countEvents counts the rows of link_events through a raw handle.
func countEvents(t *testing.T, dbPath string) int {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM link_events`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestInitAttachedChild(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	child := realTemp(t)
	r := e.runIn(child, "", "init", "--project", "tidepool", "--repo", "web")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.out, r.err)
	}
	if _, err := os.Lstat(filepath.Join(child, ".cairn", "cairn.db")); err == nil {
		t.Error("attached child has a database")
	}
	p, found, err := locate.ReadPointer(child)
	if err != nil || !found {
		t.Fatalf("pointer: %v %v", found, err)
	}
	ls := parentLinks(t, e)
	if p.Parent != "tidepool" || p.Repo != "web" || p.ParentID != ls.ID || ls.ID == "" {
		t.Errorf("pointer %+v, parent id %q", p, ls.ID)
	}
	if len(ls.Children) != 1 || ls.Children[0] != (model.Child{Name: "web", Repo: "web", Kind: model.ChildAttached}) {
		t.Errorf("children %+v", ls.Children)
	}
	st, _ := store.Open(filepath.Join(e.root, ".cairn", "cairn.db"))
	proj, _ := st.Project()
	st.Close()
	if !strings.Contains(strings.Join(proj.Repos, ","), "web") {
		t.Errorf("repos %v", proj.Repos)
	}
	reg, _ := openRegistry(&Ctx{Getenv: e.getenv})
	ent, err := reg.Get("tidepool")
	if err != nil || !sameFolder(ent.Children["web"], child) {
		t.Errorf("registry children %+v %v", ent.Children, err)
	}
	if !strings.Contains(r.out, "cairn.db") || !strings.Contains(r.out, "commit") {
		t.Errorf("stdout must ask for a commit: %q", r.out)
	}
	// A command run inside the child works and stores the child's repo.
	if r := e.runIn(child, "", "add", "--agent", "a", "--model", "m", "--title", "t", "--status", "done", "--change", "x", "--none", "decision,open,note,helper"); r.code != 0 {
		t.Fatalf("add in child: %d %s", r.code, r.err)
	}
	if out := e.runIn(child, "", "list", "--type", "change", "--json").out; !strings.Contains(out, `"web"`) {
		t.Errorf("note not stored with the child's repo: %s", out)
	}
}

func TestInitLocalChild(t *testing.T) {
	e := newEnv(t)
	e.initProject("--tasks", "auto", "--task-prefix", "FEAT", "--task-url", "https://x.example/{task}")
	child := realTemp(t)
	r := e.runIn(child, "", "init", "--project", "tidepool", "--repo", "web", "--local")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.out, r.err)
	}
	db := filepath.Join(child, ".cairn", "cairn.db")
	cs, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	cp, _ := cs.Project()
	if strings.Join(cp.Repos, ",") != "web" || cp.DefaultRepo != "web" || cp.TaskPattern == "" {
		t.Errorf("child project %+v", cp)
	}
	pls := parentLinks(t, e)
	cls, _ := cs.Links()
	if cls.Parent == nil || *cls.Parent != (model.ParentRef{Name: "tidepool", ID: pls.ID}) {
		t.Errorf("child parent %+v", cls.Parent)
	}
	if len(pls.Children) != 1 || pls.Children[0] != (model.Child{Name: "web", Repo: "web", Kind: model.ChildLocal}) {
		t.Errorf("parent children %+v", pls.Children)
	}
	reg, _ := openRegistry(&Ctx{Getenv: e.getenv})
	pe, _ := reg.Get("tidepool")
	ce, err := reg.Get("web")
	if err != nil || !sameFolder(ce.Path, child) || ce.ID != cls.ID || ce.ID == "" {
		t.Fatalf("child entry %+v %v", ce, err)
	}
	if cp.TaskMode != "auto" || cp.TaskPrefix != "FEAT" || cp.TaskURL != "https://x.example/{task}" {
		t.Errorf("task settings %+v", cp)
	}
	if !sameFolder(pe.Children["web"], child) {
		t.Errorf("parent registry children %+v", pe.Children)
	}

	child2 := realTemp(t)
	if r := e.runIn(child2, "", "init", "--project", "tidepool", "--repo", "api", "--local", "--name", "front"); r.code != 0 {
		t.Fatalf("exit %d %s", r.code, r.err)
	}
	if _, err := reg.Get("front"); err != nil {
		t.Errorf("front not registered: %v", err)
	}
	found := false
	for _, ch := range parentLinks(t, e).Children {
		found = found || ch == (model.Child{Name: "front", Repo: "api", Kind: model.ChildLocal})
	}
	if !found {
		t.Error("front not linked")
	}
}

func TestInitChildRefusals(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	pdb := filepath.Join(e.root, ".cairn", "cairn.db")
	reg, _ := openRegistry(&Ctx{Getenv: e.getenv})

	// an active child "web" and a folder holding a database
	if r := e.runIn(realTemp(t), "", "init", "--project", "tidepool", "--repo", "web"); r.code != 0 {
		t.Fatal(r.err)
	}
	holder := realTemp(t)
	if r := e.runIn(holder, "", "init", "--name", "other", "--repos", "a"); r.code != 0 {
		t.Fatal(r.err)
	}
	pointed := realTemp(t)
	if r := e.runIn(pointed, "", "init", "--project", "tidepool", "--repo", "docs"); r.code != 0 {
		t.Fatal(r.err)
	}
	// a child that itself has a parent: nesting
	localChild := realTemp(t)
	if r := e.runIn(localChild, "", "init", "--project", "tidepool", "--repo", "lib", "--local"); r.code != 0 {
		t.Fatal(r.err)
	}
	base := countEvents(t, pdb)
	before, _ := reg.List()

	cases := []struct {
		name string
		dir  string
		args []string
		code int
		msg  string
	}{
		{"unknown parent", realTemp(t), []string{"--project", "nope", "--repo", "x"}, 1, "cairn project add"},
		{"repos with project", realTemp(t), []string{"--project", "tidepool", "--repo", "x", "--repos", "a"}, 2, ""},
		{"name without local", realTemp(t), []string{"--project", "tidepool", "--repo", "x", "--name", "n"}, 2, ""},
		{"local without project", realTemp(t), []string{"--repo", "x", "--local"}, 2, ""},
		{"duplicate child", realTemp(t), []string{"--project", "tidepool", "--repo", "web"}, 1, ""},
		{"folder with database", holder, []string{"--project", "tidepool", "--repo", "x"}, 1, ""},
		{"folder with pointer", pointed, []string{"--project", "tidepool", "--repo", "x"}, 1, ""},
		{"parent is a child", realTemp(t), []string{"--project", "lib", "--repo", "x"}, 1, "nesting"},
		{"folder is a parent", e.root, []string{"--project", "other", "--repo", "x"}, 1, "nesting"},
		{"context without local", realTemp(t), []string{"--project", "tidepool", "--repo", "x", "--context", "c"}, 2, "--context"},
		{"local dotted repo without name", realTemp(t), []string{"--project", "tidepool", "--repo", "web.app", "--local"}, 3, "--name"},
		{"bad repo", realTemp(t), []string{"--project", "tidepool", "--repo", "Bad Repo"}, 3, ""},
		{"bad local name", realTemp(t), []string{"--project", "tidepool", "--repo", "x", "--local", "--name", "Bad"}, 3, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := e.runIn(tc.dir, "", append([]string{"init"}, tc.args...)...)
			if r.code != tc.code {
				t.Fatalf("exit %d, want %d\n%s", r.code, tc.code, r.err)
			}
			if tc.dir != holder && tc.dir != pointed && tc.dir != e.root && tc.dir != localChild {
				if _, err := os.Lstat(filepath.Join(tc.dir, ".cairn")); err == nil {
					t.Errorf("%s: refused directory has a .cairn folder", tc.name)
				}
			}
			if tc.msg != "" && !strings.Contains(r.err, tc.msg) {
				t.Errorf("stderr %q lacks %q", r.err, tc.msg)
			}
		})
	}
	if n := countEvents(t, pdb); n != base {
		t.Errorf("parent events changed: %d -> %d", base, n)
	}
	after, _ := reg.List()
	if len(after) != len(before) {
		t.Errorf("registry changed: %d -> %d", len(before), len(after))
	}
}

func TestInitChildRepoAlreadyAllowed(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	pdb := filepath.Join(e.root, ".cairn", "cairn.db")
	before := countEvents(t, pdb)
	child := realTemp(t)
	if r := e.runIn(child, "", "init", "--project", "tidepool", "--repo", "server"); r.code != 0 {
		t.Fatalf("exit %d %s", r.code, r.err)
	}
	ls := parentLinks(t, e)
	if len(ls.ExtraRepos) != 0 {
		t.Errorf("repo_added written: %v", ls.ExtraRepos)
	}
	if n := countEvents(t, pdb); n != before+1 {
		t.Errorf("expected exactly one new link, got %d", n-before)
	}
}

type blockReader struct{ read bool }

func (b *blockReader) Read([]byte) (int, error) {
	b.read = true
	return 0, errors.New("stdin must not be read")
}

func TestInitChildNeverPrompts(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	e.t.Chdir(realTemp(t))
	br := &blockReader{}
	var out, errb bytes.Buffer
	c := &Ctx{Stdin: br, Stdout: &out, Stderr: &errb, Getenv: e.getenv, Interactive: true}
	if code := runCtx(c, []string{"init", "--project", "tidepool", "--repo", "web"}); code != 0 {
		t.Fatalf("exit %d %s", code, errb.String())
	}
	if br.read {
		t.Error("stdin was read")
	}
	pdb := filepath.Join(e.root, ".cairn", "cairn.db")
	base := countEvents(t, pdb)
	for _, extra := range [][]string{{"--mode", "project"}, {"--default-repo", "web"}, {"--tasks", "auto"}, {"--task-pattern", "^(?P<base>X)$"}, {"--task-prefix", "AB"}, {"--task-url", "https://x/{task}"}} {
		dir := realTemp(t)
		r := e.runIn(dir, "", append([]string{"init", "--project", "tidepool", "--repo", "api"}, extra...)...)
		if r.code != 2 {
			t.Errorf("%v: exit %d", extra, r.code)
		}
		if _, err := os.Lstat(filepath.Join(dir, ".cairn")); err == nil {
			t.Errorf("%v: wrote .cairn", extra)
		}
	}
	if n := countEvents(t, pdb); n != base {
		t.Errorf("events changed")
	}
}

func TestInitChildMachineParent(t *testing.T) {
	e := newEnv(t)
	e.initProject("--mode", "machine")
	child := realTemp(t)
	r := e.runIn(child, "", "init", "--project", "tidepool", "--repo", "web")
	if r.code != 0 {
		t.Fatalf("exit %d %s", r.code, r.err)
	}
	if strings.Contains(r.out, "commit") {
		t.Errorf("machine parent must not ask for a commit: %q", r.out)
	}
	if _, found, err := locate.ReadPointer(child); err != nil || !found {
		t.Errorf("pointer %v %v", found, err)
	}
}

func TestInitDottedRepoChild(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	child := realTemp(t)
	if r := e.runIn(child, "", "init", "--project", "tidepool", "--repo", "web.app"); r.code != 0 {
		t.Fatalf("exit %d %s", r.code, r.err)
	}
	if r := e.runIn(child, "", "add", "--agent", "a", "--model", "m", "--title", "t", "--status", "done", "--change", "x", "--none", "decision,open,note,helper"); r.code != 0 {
		t.Fatalf("add in child: %d %s", r.code, r.err)
	}
	if out := e.runIn(child, "", "list", "--type", "change", "--json").out; !strings.Contains(out, `"web.app"`) {
		t.Errorf("repo not stored: %s", out)
	}
	if ls := parentLinks(t, e); len(ls.Children) != 1 || ls.Children[0].Name != "web.app" {
		t.Errorf("children %+v", ls.Children)
	}
}

func TestInitChildRollback(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permissions do not bind root")
	}
	e := newEnv(t)
	e.initProject()
	pdb := filepath.Join(e.root, ".cairn", "cairn.db")
	child := realTemp(t)
	dot := filepath.Join(child, ".cairn")
	if err := os.Mkdir(dot, 0o500); err != nil { // the pointer cannot be written
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dot, 0o700) })
	before := countEvents(t, pdb)
	r := e.runIn(child, "", "init", "--project", "tidepool", "--repo", "web")
	if r.code != 1 {
		t.Fatalf("exit %d, want 1\n%s", r.code, r.err)
	}
	if _, err := os.Lstat(filepath.Join(dot, "links.json")); err == nil {
		t.Error("pointer left behind")
	}
	reg, _ := openRegistry(&Ctx{Getenv: e.getenv})
	if ent, _ := reg.Get("tidepool"); len(ent.Children) != 0 {
		t.Errorf("registry children left: %+v", ent.Children)
	}
	ls := parentLinks(t, e)
	if len(ls.Children) != 0 {
		t.Errorf("child still active: %+v", ls.Children)
	}
	// repo_added (web) + child_linked + child_detached, plus the id on first use
	if n := countEvents(t, pdb); n < before+3 {
		t.Errorf("expected a link and a detach event, events %d -> %d", before, n)
	}
}

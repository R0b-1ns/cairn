package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/R0b-1ns/cairn/internal/locate"
	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
)

// childFixture is a parent project in e.root with one child "web" in a second folder.
type childFixture struct {
	e     *testEnv
	child string
}

func newChildFixture(t *testing.T, local bool) childFixture {
	t.Helper()
	e := newEnv(t)
	e.initProject()
	child := realTemp(t)
	args := []string{"init", "--project", "tidepool", "--repo", "web"}
	if local {
		args = append(args, "--local")
	}
	if r := e.runIn(child, "", args...); r.code != 0 {
		t.Fatalf("init child: %d %s %s", r.code, r.out, r.err)
	}
	return childFixture{e, child}
}

func (f childFixture) addNote(t *testing.T, dir, title string) {
	t.Helper()
	r := f.e.runIn(dir, "", "add", "--agent", "a", "--model", "m", "--title", title, "--status", "done",
		"--change", "x", "--none", "decision,open,note,helper", "--repo", "web")
	if r.code != 0 {
		t.Fatalf("add: %d %s", r.code, r.err)
	}
}

func countNotesAt(t *testing.T, root string) int {
	t.Helper()
	st, err := store.Open(filepath.Join(root, ".cairn", "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	n, err := st.CountNotes()
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// secondMachine registers the parent on a fresh machine (own CAIRN_HOME).
func secondMachine(t *testing.T, f childFixture) *testEnv {
	t.Helper()
	m := newEnv(t)
	if r := m.runIn(m.root, "", "project", "add", f.e.root); r.code != 0 {
		t.Fatalf("project add: %d %s", r.code, r.err)
	}
	return m
}

func registryChild(t *testing.T, e *testEnv, parent, child string) (string, bool) {
	t.Helper()
	reg, _ := openRegistry(&Ctx{Getenv: e.getenv})
	ent, err := reg.Get(parent)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := ent.Children[child]
	return p, ok
}

func TestProjectLinkRecordsPath(t *testing.T) {
	for _, local := range []bool{true, false} {
		f := newChildFixture(t, local)
		m := secondMachine(t, f)
		// Before linking, an attached child is refused on the new machine.
		r := m.runIn(m.root, "", "project", "link", "--project", "tidepool", "web", f.child)
		if r.code != 0 {
			t.Fatalf("local=%v link: %d %s", local, r.code, r.err)
		}
		got, ok := registryChild(t, m, "tidepool", "web")
		if !ok || !sameFolder(got, f.child) {
			t.Errorf("local=%v registry path %q", local, got)
		}
		if !local {
			if r := m.runIn(f.child, "", "context"); r.code != 0 {
				t.Errorf("attached child still unresolved after link: %d %s", r.code, r.err)
			}
		}
	}
}

func TestProjectLinkFromParentFolder(t *testing.T) {
	f := newChildFixture(t, false)
	m := secondMachine(t, f)
	r := m.runIn(f.e.root, "", "project", "link", "web", f.child)
	if r.code != 0 {
		t.Fatalf("link from the parent folder: %d %s", r.code, r.err)
	}
}

func TestProjectLinkRefusesWrongFolder(t *testing.T) {
	f := newChildFixture(t, true)
	m := secondMachine(t, f)
	plain := realTemp(t)
	other := realTemp(t) // another project
	if r := m.runIn(other, "", "init", "--name", "other", "--repos", "web"); r.code != 0 {
		t.Fatal(r.err)
	}
	// A child of a different parent (same child name).
	m.mustRun("init", "--name", "tidepool2", "--repos", "server")
	otherParentChild := realTemp(t)
	if r := m.runIn(otherParentChild, "", "init", "--project", "tidepool2", "--repo", "web", "--local"); r.code != 0 {
		t.Fatalf("%s", r.err)
	}
	for _, folder := range []string{plain, other, otherParentChild, filepath.Join(plain, "missing")} {
		r := m.runIn(m.root, "", "project", "link", "--project", "tidepool", "web", folder)
		if r.code != 1 {
			t.Errorf("%s: exit %d, want 1 (%s)", folder, r.code, r.err)
		}
	}
	if _, ok := registryChild(t, m, "tidepool", "web"); ok {
		t.Error("a path was recorded for a refused link")
	}
	if r := m.runIn(m.root, "", "project", "link", "--project", "tidepool", "nosuch", f.child); r.code != 1 {
		t.Errorf("unknown child: exit %d", r.code)
	}
	if r := m.runIn(m.root, "", "project", "link", "--project", "tidepool", "web"); r.code != 2 {
		t.Errorf("missing argument: exit %d", r.code)
	}
	// An attached pointer naming another parent id is refused too.
	fa := newChildFixture(t, false)
	ma := secondMachine(t, fa)
	if err := locate.WritePointer(fa.child, locate.Pointer{Parent: "tidepool", ParentID: "p-aaaaaaaaaa", Repo: "web"}); err != nil {
		t.Fatal(err)
	}
	if r := ma.runIn(ma.root, "", "project", "link", "--project", "tidepool", "web", fa.child); r.code != 1 {
		t.Errorf("wrong parent id: exit %d", r.code)
	}
}

func TestUnlinkFromParent(t *testing.T) {
	for _, local := range []bool{true, false} {
		f := newChildFixture(t, local)
		f.addNote(t, f.child, "n1")
		before := 0
		if local {
			before = countNotesAt(t, f.child)
		}
		r := f.e.runIn(f.e.root, "", "project", "unlink", "web")
		if r.code != 0 {
			t.Fatalf("local=%v: %d %s", local, r.code, r.err)
		}
		if ls := parentLinks(t, f.e); len(ls.Children) != 0 {
			t.Errorf("children %+v", ls.Children)
		}
		if _, ok := registryChild(t, f.e, "tidepool", "web"); ok {
			t.Error("registry path still there")
		}
		st, _ := store.Open(filepath.Join(f.e.root, ".cairn", "cairn.db"))
		p, _ := st.Project()
		st.Close()
		if !strings.Contains(strings.Join(p.Repos, ","), "web") {
			t.Errorf("repo web gone from %v", p.Repos)
		}
		if local {
			if n := countNotesAt(t, f.child); n != before {
				t.Errorf("notes %d -> %d", before, n)
			}
			cs, _ := store.Open(filepath.Join(f.child, ".cairn", "cairn.db"))
			ls, _ := cs.Links()
			cs.Close()
			if ls.Parent != nil {
				t.Errorf("child still has parent %+v", ls.Parent)
			}
		} else if _, found, _ := locate.ReadPointer(f.child); found {
			t.Error("pointer still there")
		}
	}
	f := newChildFixture(t, true)
	if r := f.e.runIn(f.e.root, "", "project", "unlink", "nosuch"); r.code != 1 {
		t.Errorf("unknown child exit %d", r.code)
	}
}

func TestUnlinkFromChild(t *testing.T) {
	for _, local := range []bool{true, false} {
		f := newChildFixture(t, local)
		r := f.e.runIn(f.child, "", "project", "unlink")
		if r.code != 0 {
			t.Fatalf("local=%v: %d %s", local, r.code, r.err)
		}
		if ls := parentLinks(t, f.e); len(ls.Children) != 0 {
			t.Errorf("local=%v children %+v", local, ls.Children)
		}
		if _, ok := registryChild(t, f.e, "tidepool", "web"); ok {
			t.Error("registry path still there")
		}
		if local {
			cs, _ := store.Open(filepath.Join(f.child, ".cairn", "cairn.db"))
			ls, _ := cs.Links()
			cs.Close()
			if ls.Parent != nil {
				t.Errorf("child still has parent")
			}
		} else if _, found, _ := locate.ReadPointer(f.child); found {
			t.Error("pointer still there")
		}
		// A second run has nothing to detach.
		if r := f.e.runIn(f.child, "", "project", "unlink"); r.code == 0 {
			t.Errorf("second unlink exit %d", r.code)
		}
	}
}

func TestUnlinkFromChildOfUnlinkedFolder(t *testing.T) {
	// An attached folder on a machine that never ran link cannot be resolved by
	// locate, but unlink still works from it.
	f := newChildFixture(t, false)
	m := secondMachine(t, f)
	if r := m.runIn(f.child, "", "context"); r.code == 0 {
		t.Fatal("unlinked attached folder resolved")
	}
	r := m.runIn(f.child, "", "project", "unlink")
	if r.code != 0 {
		t.Fatalf("%d %s", r.code, r.err)
	}
	if _, found, _ := locate.ReadPointer(f.child); found {
		t.Error("pointer still there")
	}
	if ls := parentLinks(t, f.e); len(ls.Children) != 0 {
		t.Errorf("children %+v", ls.Children)
	}
}

func TestUnlinkFromChildParentUnreachable(t *testing.T) {
	for _, local := range []bool{true, false} {
		f := newChildFixture(t, local)
		m := newEnv(t) // a machine that does not know the parent
		sub := filepath.Join(f.child, "src")
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		r := m.runIn(sub, "", "project", "unlink")
		if r.code != 0 {
			t.Fatalf("local=%v: %d %s", local, r.code, r.err)
		}
		if !strings.Contains(r.err, "cairn: warning") || !strings.Contains(r.err, "cairn project unlink --project tidepool web") {
			t.Errorf("warning %q", r.err)
		}
		if local {
			cs, _ := store.Open(filepath.Join(f.child, ".cairn", "cairn.db"))
			ls, _ := cs.Links()
			cs.Close()
			if ls.Parent != nil {
				t.Error("parent kept")
			}
		} else if _, found, _ := locate.ReadPointer(f.child); found {
			t.Error("pointer kept")
		}
		// The parent still lists the child until it is told.
		if ls := parentLinks(t, f.e); len(ls.Children) != 1 {
			t.Errorf("parent children %+v", ls.Children)
		}
	}
}

func TestUnlinkWithoutChildPath(t *testing.T) {
	f := newChildFixture(t, false)
	m := secondMachine(t, f) // the child's path is not known here
	r := m.runIn(m.root, "", "project", "unlink", "--project", "tidepool", "web")
	if r.code != 0 {
		t.Fatalf("%d %s", r.code, r.err)
	}
	if !strings.Contains(r.err, "cairn: warning") || !strings.Contains(r.err, "cairn project unlink") {
		t.Errorf("warning %q", r.err)
	}
	if ls := parentLinks(t, f.e); len(ls.Children) != 0 {
		t.Errorf("children %+v", ls.Children)
	}
	// The child's later commands name the fix and do not crash.
	r = m.runIn(f.child, "", "context")
	if r.code == 0 || !strings.Contains(r.err, "cairn project") {
		t.Errorf("later command: %d %q", r.code, r.err)
	}
}

func TestRelinkAfterUnlink(t *testing.T) {
	f := newChildFixture(t, false)
	f.addNote(t, f.child, "before")
	parentNotes := countNotesAt(t, f.e.root)
	if r := f.e.runIn(f.e.root, "", "project", "unlink", "web"); r.code != 0 {
		t.Fatalf("%s", r.err)
	}
	if r := f.e.runIn(f.child, "", "init", "--project", "tidepool", "--repo", "web"); r.code != 0 {
		t.Fatalf("relink: %d %s", r.code, r.err)
	}
	if n := countNotesAt(t, f.e.root); n != parentNotes {
		t.Errorf("parent notes %d -> %d", parentNotes, n)
	}
	if ls := parentLinks(t, f.e); len(ls.Children) != 1 || ls.Children[0] != (model.Child{Name: "web", Repo: "web", Kind: model.ChildAttached}) {
		t.Errorf("children %+v", ls.Children)
	}
	if r := f.e.runIn(f.child, "", "context"); r.code != 0 {
		t.Errorf("child unusable after relink: %d %s", r.code, r.err)
	}
}

func TestRelinkLocalChildAfterUnlink(t *testing.T) {
	f := newChildFixture(t, true)
	f.addNote(t, f.child, "before")
	childNotes := countNotesAt(t, f.child)
	if r := f.e.runIn(f.child, "", "project", "unlink"); r.code != 0 {
		t.Fatalf("%s", r.err)
	}
	if n := countNotesAt(t, f.child); n != childNotes {
		t.Errorf("child notes %d -> %d", childNotes, n)
	}
	// The unlinked folder is a standalone project again; nothing was lost.
	if r := f.e.runIn(f.child, "", "list", "--type", "change"); r.code != 0 || !strings.Contains(r.out, "change") {
		t.Errorf("standalone child: %d %s %s", r.code, r.out, r.err)
	}
}

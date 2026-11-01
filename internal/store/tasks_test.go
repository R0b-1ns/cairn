package store

import (
	"errors"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/tasks"
)

func autoProject() model.Project {
	p := testProject()
	p.TaskMode, p.TaskPrefix = tasks.Auto, "T"
	p.TaskPattern, _ = tasks.Pattern(tasks.Auto, "T")
	return p
}

func newAutoStore(t *testing.T) *Store {
	t.Helper()
	s, err := Create(filepath.Join(t.TempDir(), ".cairn", "cairn.db"), autoProject())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func taskNote(task string, newTask bool) model.Note {
	return model.Note{
		Agent: "builder", Repo: "server", At: "2026-10-30T10:00:00Z", Task: task, NewTask: newTask,
		Model: "m", Title: "t", Status: model.StatusInProgress,
		Entries: []model.Entry{{Type: model.TypeChange, Text: "x"}},
	}
}

func TestNewTaskNumbers(t *testing.T) {
	s := newAutoStore(t)
	for i := 1; i <= 3; i++ {
		n, err := s.InsertNote(taskNote("", true))
		if err != nil {
			t.Fatal(err)
		}
		if want := tasks.Format("T", i); n.Task != want || n.NewTask {
			t.Fatalf("got %q newtask=%v, want %q", n.Task, n.NewTask, want)
		}
		got, _ := s.GetNote(n.ID)
		if got.Task != n.Task {
			t.Fatalf("stored %q", got.Task)
		}
	}
	if _, err := s.InsertNote(taskNote("T-9.1", false)); err != nil {
		t.Fatal(err)
	}
	n, err := s.InsertNote(taskNote("", true))
	if err != nil || n.Task != "T-10" {
		t.Fatalf("got %q, %v", n.Task, err)
	}
	// A foreign-looking task is ignored by the count.
	if _, err := s.InsertNote(taskNote("TP-03", false)); err != nil {
		t.Fatal(err)
	}
	if n, _ = s.InsertNote(taskNote("", true)); n.Task != "T-11" {
		t.Fatalf("got %q", n.Task)
	}
}

func TestNewTaskRefusedOutsideAuto(t *testing.T) {
	s, _ := newStore(t)
	_, err := s.InsertNote(taskNote("", true))
	if !errors.Is(err, ErrNotAuto) {
		t.Fatalf("got %v", err)
	}
	if c, _ := s.CountNotes(); c != 0 {
		t.Fatal("note inserted")
	}
}

func TestNewTaskEmptyPrefixRefused(t *testing.T) {
	p := autoProject()
	p.TaskPrefix = ""
	s, err := Create(filepath.Join(t.TempDir(), "c.db"), p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.InsertNote(taskNote("", true)); err == nil || errors.Is(err, ErrNotAuto) {
		t.Fatalf("got %v", err)
	}
}

func TestNewTaskConcurrent(t *testing.T) {
	s := newAutoStore(t)
	var wg sync.WaitGroup
	got := make([]string, 8)
	errs := make([]error, 8)
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := s.InsertNote(taskNote("", true))
			got[i], errs[i] = n.Task, err
		}()
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	sort.Slice(got, func(i, j int) bool { return len(got[i]) < len(got[j]) || len(got[i]) == len(got[j]) && got[i] < got[j] })
	for i, g := range got {
		if want := tasks.Format("T", i+1); g != want {
			t.Fatalf("got %v", got)
		}
	}
}

func TestProjectRoundTripTaskFields(t *testing.T) {
	s := newAutoStore(t)
	p, err := s.Project()
	if err != nil || p.TaskMode != "auto" || p.TaskPrefix != "T" {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestRoundTripNewFields(t *testing.T) {
	s, _ := newStore(t)
	n := sampleNote()
	n.AuthorName, n.AuthorEmail, n.Branch, n.Ref = "Ann", "ann@x.io", "feat/a", "https://t.example/1"
	n, err := s.InsertNote(n)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetNote(n.ID)
	if got.AuthorName != "Ann" || got.AuthorEmail != "ann@x.io" || got.Branch != "feat/a" || got.Ref != "https://t.example/1" {
		t.Fatalf("%+v", got)
	}
	a, err := s.InsertAddendum(model.Addendum{NoteID: n.ID, Agent: "b", Model: "m", At: "2026-10-31T10:00:00Z",
		AuthorName: "Bob", AuthorEmail: "bob@x.io", Branch: "fix/x", Ref: "https://t.example/2",
		Entries: []model.Entry{{Type: model.TypeNote, Text: "n"}}})
	if err != nil {
		t.Fatal(err)
	}
	ga, _ := s.GetAddendum(a.ID)
	if ga.AuthorName != "Bob" || ga.AuthorEmail != "bob@x.io" || ga.Branch != "fix/x" || ga.Ref != "https://t.example/2" {
		t.Fatalf("%+v", ga)
	}
	list, _ := s.Addenda(n.ID)
	if len(list) != 1 || list[0].Ref != "https://t.example/2" {
		t.Fatalf("%+v", list)
	}
}

func addAdd(t *testing.T, s *Store, noteID, at, branch, ref, name, email string) {
	t.Helper()
	_, err := s.InsertAddendum(model.Addendum{NoteID: noteID, Agent: "b", Model: "m", At: at,
		Branch: branch, Ref: ref, AuthorName: name, AuthorEmail: email,
		Entries: []model.Entry{{Type: model.TypeNote, Text: "n"}}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTaskRefs(t *testing.T) {
	s, _ := newStore(t)
	a := taskNote("TP-07", false)
	a.Ref, a.At = "https://t.example/old", "2026-10-01T10:00:00Z"
	a, _ = s.InsertNote(a)
	b := taskNote("TP-07", false)
	b.Ref, b.At = "https://t.example/old", "2026-10-02T10:00:00Z"
	if _, err := s.InsertNote(b); err != nil {
		t.Fatal(err)
	}
	addAdd(t, s, a.ID, "2026-10-03T10:00:00Z", "", "https://t.example/new", "", "")
	other := taskNote("TP-08", false)
	other.Ref = "https://t.example/other"
	s.InsertNote(other)
	refs, err := s.TaskRefs("TP-07")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 || refs[0] != "https://t.example/new" || refs[1] != "https://t.example/old" {
		t.Fatalf("%v", refs)
	}
}

func TestFilterBranchAuthorRef(t *testing.T) {
	s, _ := newStore(t)
	n := taskNote("TP-01", false)
	n.Branch, n.AuthorName, n.AuthorEmail = "main", "Ann", "ann@x.io"
	n, _ = s.InsertNote(n)
	sib := taskNote("TP-01", false)
	sib.At = "2026-10-31T10:00:00Z"
	sib, _ = s.InsertNote(sib)
	lone := taskNote("", false)
	lone, _ = s.InsertNote(lone)
	addAdd(t, s, n.ID, "2026-11-01T10:00:00Z", "fix/x", "https://t.example/9", "Bob", "bob@x.io")
	addAdd(t, s, lone.ID, "2026-11-01T11:00:00Z", "", "https://t.example/lone", "", "")

	ids := func(f NoteFilter) []string {
		hs, err := s.NoteHeaders(f)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, h := range hs {
			out = append(out, h.ID)
		}
		return out
	}
	if g := ids(NoteFilter{Branch: "fix/x"}); len(g) != 1 || g[0] != n.ID {
		t.Fatalf("branch %v", g)
	}
	if g := ids(NoteFilter{Branch: "main"}); len(g) != 1 || g[0] != n.ID {
		t.Fatalf("branch main %v", g)
	}
	for _, a := range []string{"Ann", "ann@x.io", "Bob", "bob@x.io"} {
		if g := ids(NoteFilter{Author: a}); len(g) != 1 || g[0] != n.ID {
			t.Fatalf("author %s %v", a, g)
		}
	}
	if g := ids(NoteFilter{Author: "nobody"}); len(g) != 0 {
		t.Fatalf("%v", g)
	}
	if g := ids(NoteFilter{Ref: "https://t.example/9"}); len(g) != 2 {
		t.Fatalf("ref task wide %v", g)
	}
	if g := ids(NoteFilter{Ref: "https://t.example/lone"}); len(g) != 1 || g[0] != lone.ID {
		t.Fatalf("ref lone %v", g)
	}
	hs, _ := s.NoteHeaders(NoteFilter{ID: n.ID})
	if hs[0].Branch != "main" || hs[0].AuthorName != "Ann" || hs[0].AuthorEmail != "ann@x.io" {
		t.Fatalf("%+v", hs[0])
	}
	_ = sib

	es, total, err := s.ListEntries(Filter{Branch: "fix/x", Limit: 0, State: StateAll})
	if err != nil || total != 2 {
		t.Fatalf("entries branch %d %v", total, err)
	}
	if _, total, _ = s.ListEntries(Filter{Ref: "https://t.example/9", State: StateAll}); total != 3 {
		t.Fatalf("entries ref %d", total)
	}
	if _, total, _ = s.ListEntries(Filter{Author: "Bob", State: StateAll}); total != 2 {
		t.Fatalf("entries author %d", total)
	}
	_ = es
}

func TestImportIdempotentWithNewFields(t *testing.T) {
	src, _ := newStore(t)
	n := sampleNote()
	n.AuthorName, n.AuthorEmail, n.Branch, n.Ref = "Ann", "ann@x.io", "feat/a", "https://t.example/1"
	n, err := src.InsertNote(n)
	if err != nil {
		t.Fatal(err)
	}
	addAdd(t, src, n.ID, "2026-11-01T10:00:00Z", "fix/x", "https://t.example/2", "Bob", "bob@x.io")
	items := exportItems(t, src, n.ID)
	dst, _ := newStore(t)
	if r, err := dst.Import(items); err != nil || r.Inserted != 2 {
		t.Fatalf("%+v %v", r, err)
	}
	got, _ := dst.GetNote(n.ID)
	if got.Branch != "feat/a" || got.AuthorEmail != "ann@x.io" {
		t.Fatalf("%+v", got)
	}
	r, err := dst.Import(items)
	if err != nil || r.Skipped != 2 || r.Inserted != 0 {
		t.Fatalf("%+v %v", r, err)
	}
	n2 := *items[0].Note
	n2.Branch = "other"
	if _, err := dst.Import([]ImportItem{{Note: &n2}}); !errors.Is(err, ErrConflict) {
		t.Fatalf("got %v", err)
	}
}

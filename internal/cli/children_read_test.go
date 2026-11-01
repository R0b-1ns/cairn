package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
)

// putNote inserts a note with one change entry straight into the database at root.
func putNote(t *testing.T, root string, n model.Note) model.Note {
	t.Helper()
	st, err := store.Open(filepath.Join(root, ".cairn", "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if n.Agent == "" {
		n.Agent, n.Model, n.Status = "builder", "claude-test", model.StatusDone
	}
	if n.None == nil {
		n.None = []model.EntryType{model.TypeDecision, model.TypeOpen, model.TypeNote, model.TypeHelper}
	}
	out, err := st.InsertNote(n)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func chg(text string) []model.Entry { return []model.Entry{{Type: model.TypeChange, Text: text}} }

type readFx struct {
	childFixture
	p1, p2, c1, c2 model.Note
}

// newReadFx is a parent (repo server) with a local child "web"; every note says "quartz".
func newReadFx(t *testing.T) readFx {
	f := readFx{childFixture: newChildFixture(t, true)}
	f.p1 = putNote(t, f.e.root, model.Note{Repo: "server", Task: "TP-01", At: "2026-10-01T10:00:00Z", Title: "Parent one quartz", Entries: chg("parent quartz one")})
	f.c1 = putNote(t, f.child, model.Note{Repo: "web", Task: "TP-07", At: "2026-10-02T10:00:00Z", Title: "Child one quartz", Entries: chg("child quartz one"),
		AuthorName: "Ada", AuthorEmail: "ada@example.org", Branch: "feat/x", Ref: "https://t.example.org/TP-07"})
	f.p2 = putNote(t, f.e.root, model.Note{Repo: "server", Task: "TP-02", At: "2026-10-03T10:00:00Z", Title: "Parent two quartz", Entries: chg("parent quartz two")})
	f.c2 = putNote(t, f.child, model.Note{Repo: "web", Task: "TP-08", At: "2026-10-04T10:00:00Z", Title: "Child two quartz", Entries: chg("child quartz two")})
	return f
}

func TestSearchIncludesChildren(t *testing.T) {
	f := newReadFx(t)
	r := f.e.mustRun("search", "quartz")
	lines := strings.Split(strings.TrimRight(r.out, "\n"), "\n")
	if len(lines) != 8 {
		t.Fatalf("want 8 lines:\n%s", r.out)
	}
	for i, l := range lines {
		child := strings.HasSuffix(l, "  [web]")
		if child != (i >= 4) {
			t.Errorf("line %d child label wrong: %q", i, l)
		}
	}
	r = f.e.mustRun("search", "quartz", "--json")
	var hits []map[string]any
	if err := json.Unmarshal([]byte(r.out), &hits); err != nil || len(hits) != 8 {
		t.Fatalf("json: %v %s", err, r.out)
	}
	for i, h := range hits {
		c, has := h["child"]
		if has != (i >= 4) || (has && c != "web") {
			t.Errorf("hit %d child %v", i, h)
		}
	}
}

func TestSearchMaxAppliesToMergedList(t *testing.T) {
	f := newReadFx(t)
	r := f.e.mustRun("search", "quartz", "--max", "5")
	lines := strings.Split(strings.TrimRight(r.out, "\n"), "\n")
	if len(lines) != 6 || lines[5] != "... 3 more (raise --max)" || !strings.HasSuffix(lines[4], "[web]") || strings.HasSuffix(lines[3], "[web]") {
		t.Fatalf("got:\n%s", r.out)
	}
	r = f.e.mustRun("search", "quartz", "--max", "2")
	if !strings.Contains(r.out, "... 6 more") || strings.Contains(r.out, "[web]") {
		t.Fatalf("got:\n%s", r.out)
	}
}

func TestListIncludesChildren(t *testing.T) {
	f := newReadFx(t)
	r := f.e.mustRun("list")
	lines := strings.Split(strings.TrimRight(r.out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("got:\n%s", r.out)
	}
	wantNotes := []string{f.c2.ID, f.p2.ID, f.c1.ID, f.p1.ID}
	for i, l := range lines {
		if !strings.Contains(l, wantNotes[i]) {
			t.Errorf("line %d want note %s: %q", i, wantNotes[i], l)
		}
		if strings.HasSuffix(l, "[web]") != (wantNotes[i] == f.c1.ID || wantNotes[i] == f.c2.ID) {
			t.Errorf("line %d label: %q", i, l)
		}
	}
	r = f.e.mustRun("list", "--max", "3")
	if !strings.Contains(r.out, "... 1 more") || strings.Count(r.out, "\n") != 4 {
		t.Fatalf("got:\n%s", r.out)
	}
	// filters apply per source
	if r = f.e.mustRun("list", "--repo", "web"); strings.Count(r.out, "[web]") != 2 || strings.Count(r.out, "\n") != 2 {
		t.Fatalf("--repo web:\n%s", r.out)
	}
	if r = f.e.mustRun("list", "--repo", "server"); strings.Contains(r.out, "[web]") || strings.Count(r.out, "\n") != 2 {
		t.Fatalf("--repo server:\n%s", r.out)
	}
	if r = f.e.mustRun("list", "--since", "2026-10-03"); strings.Count(r.out, "\n") != 2 {
		t.Fatalf("--since:\n%s", r.out)
	}
	if r = f.e.mustRun("list", "--task", "TP-07"); strings.Count(r.out, "\n") != 1 || !strings.Contains(r.out, f.c1.ID) {
		t.Fatalf("--task:\n%s", r.out)
	}
	if r = f.e.mustRun("list", "--type", "decision"); strings.TrimSpace(r.out) != "no entries" {
		t.Fatalf("--type:\n%s", r.out)
	}
	r = f.e.mustRun("list", "--json")
	var items []map[string]any
	if err := json.Unmarshal([]byte(r.out), &items); err != nil || len(items) != 4 {
		t.Fatalf("json: %v %s", err, r.out)
	}
	if items[0]["child"] != "web" || items[1]["child"] != nil {
		t.Errorf("child keys: %v", items)
	}
}

func TestListTaskPatternIsPerSource(t *testing.T) {
	f := newReadFx(t)
	// Make the child's pattern differ: it accepts only ZZ-n.
	raw, err := sqlOpenRaw(filepath.Join(f.child, ".cairn", "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := raw.Query(`SELECT name FROM sqlite_master WHERE type = 'trigger' AND tbl_name = 'project'`)
	if err != nil {
		t.Fatal(err)
	}
	var trg []string
	for rows.Next() {
		var n string
		rows.Scan(&n)
		trg = append(trg, n)
	}
	rows.Close()
	for _, n := range trg {
		if _, err := raw.Exec(`DROP TRIGGER ` + n); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := raw.Exec(`UPDATE project SET task_pattern = '^ZZ-[0-9]+$'`); err != nil {
		t.Fatalf("alter: %v", err)
	}
	raw.Close()
	r := f.e.mustRun("list", "--task", "TP-01")
	if strings.Count(r.out, "\n") != 1 || !strings.Contains(r.out, f.p1.ID) {
		t.Fatalf("got:\n%s", r.out)
	}
}

func TestShowFindsChildNote(t *testing.T) {
	f := newReadFx(t)
	r := f.e.mustRun("show", f.c1.ID)
	if !strings.Contains(r.out, "child: web\n") || !strings.Contains(r.out, "Child one quartz") {
		t.Fatalf("got:\n%s", r.out)
	}
	r = f.e.mustRun("show", f.c1.ID, "--json")
	var m map[string]any
	if err := json.Unmarshal([]byte(r.out), &m); err != nil || m["child"] != "web" {
		t.Fatalf("json: %v %s", err, r.out)
	}
	r = f.e.mustRun("show", f.p1.ID)
	if strings.Contains(r.out, "child:") {
		t.Fatalf("parent note shows child:\n%s", r.out)
	}
	r = f.e.mustRun("show", f.p1.ID, "--json")
	if strings.Contains(r.out, `"child"`) {
		t.Fatalf("parent json has child:\n%s", r.out)
	}
	eid := f.c1.Entries[0].ID
	if eid == "" {
		t.Fatal("no entry id")
	}
	r = f.e.mustRun("show", eid)
	if !strings.Contains(r.out, "child: web") {
		t.Fatalf("entry:\n%s", r.out)
	}
	r = f.e.mustRun("show", eid, "--json")
	if !strings.Contains(r.out, `"child": "web"`) {
		t.Fatalf("entry json:\n%s", r.out)
	}
	if r = f.e.run("show", "n-zzzzzz"); r.code != ExitRuntime || !strings.Contains(r.err, "n-zzzzzz not found") {
		t.Fatalf("not found: %+v", r)
	}
}

func TestShowFindsChildAddendum(t *testing.T) {
	f := newReadFx(t)
	st, err := store.Open(filepath.Join(f.child, ".cairn", "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := st.InsertAddendum(model.Addendum{NoteID: f.c1.ID, Agent: "b", Model: "m", At: "2026-10-05T10:00:00Z", Entries: []model.Entry{{Type: model.TypeNote, Text: "late"}}})
	st.Close()
	if err != nil {
		t.Fatal(err)
	}
	r := f.e.mustRun("show", a.ID)
	if !strings.Contains(r.out, "child: web") {
		t.Fatalf("got:\n%s", r.out)
	}
	r = f.e.mustRun("show", a.ID, "--json")
	if !strings.Contains(r.out, `"child": "web"`) || !strings.Contains(r.out, `"note_id"`) {
		t.Fatalf("got:\n%s", r.out)
	}
}

func TestShowAmbiguousID(t *testing.T) {
	f := newReadFx(t)
	putNote(t, f.e.root, model.Note{ID: "n-abcdef", Repo: "server", At: "2026-10-06T10:00:00Z", Title: "Same id", Entries: chg("p")})
	putNote(t, f.child, model.Note{ID: "n-abcdef", Repo: "web", At: "2026-10-06T10:00:00Z", Title: "Same id", Entries: chg("c")})
	r := f.e.run("show", "n-abcdef")
	if r.code != ExitRuntime || r.out != "" {
		t.Fatalf("got %+v", r)
	}
	if !strings.Contains(r.err, "tidepool") || !strings.Contains(r.err, "web") || !strings.Contains(r.err, "n-abcdef") {
		t.Fatalf("message: %q", r.err)
	}
}

func TestChildDoesNotSeeParent(t *testing.T) {
	f := newReadFx(t)
	for _, args := range [][]string{{"search", "quartz"}, {"list"}} {
		r := f.e.runIn(f.child, "", args...)
		if r.code != 0 || r.err != "" || strings.Contains(r.out, "[") || strings.Contains(r.out, "Parent") ||
			strings.Contains(r.out, f.p1.ID) || strings.Contains(r.out, f.p2.ID) {
			t.Fatalf("%v: %+v", args, r)
		}
		if !strings.Contains(r.out, f.c1.ID) || !strings.Contains(r.out, f.c2.ID) {
			t.Fatalf("%v lacks child notes: %s", args, r.out)
		}
	}
	if r := f.e.runIn(f.child, "", "show", f.p1.ID); r.code != ExitRuntime {
		t.Fatalf("child must not show parent note: %+v", r)
	}
}

func TestAttachedChildSeesParent(t *testing.T) {
	f := newChildFixture(t, false)
	p := putNote(t, f.e.root, model.Note{Repo: "server", Task: "TP-01", At: "2026-10-01T10:00:00Z", Title: "Parent quartz", Entries: chg("pq")})
	r := f.e.runIn(f.child, "", "search", "quartz")
	if r.code != 0 || !strings.Contains(r.out, p.ID) || strings.Contains(r.out, "[") {
		t.Fatalf("%+v", r)
	}
}

func TestChildResultsKeepAuthorAndBranch(t *testing.T) {
	f := newReadFx(t)
	r := f.e.mustRun("list", "--author", "ada@example.org", "--branch", "feat/x", "--ref", "https://t.example.org/TP-07")
	if strings.Count(r.out, "\n") != 1 || !strings.Contains(r.out, f.c1.ID) || !strings.HasSuffix(strings.TrimSpace(r.out), "[web]") {
		t.Fatalf("list:\n%s", r.out)
	}
	r = f.e.mustRun("show", f.c1.ID)
	for _, w := range []string{"by Ada <ada@example.org> on feat/x", "ref: https://t.example.org/TP-07", "child: web"} {
		if !strings.Contains(r.out, w) {
			t.Errorf("show lacks %q:\n%s", w, r.out)
		}
	}
	r = f.e.mustRun("show", f.c1.ID, "--json")
	for _, w := range []string{`"author_name": "Ada"`, `"branch": "feat/x"`, `"ref": "https://t.example.org/TP-07"`, `"child": "web"`} {
		if !strings.Contains(r.out, w) {
			t.Errorf("json lacks %q:\n%s", w, r.out)
		}
	}
}

func TestMissingChildWarnsOnStderrOnly(t *testing.T) {
	f := newReadFx(t)
	reg, _ := openRegistry(&Ctx{Getenv: f.e.getenv})
	if err := reg.RemoveChild("tidepool", "web"); err != nil {
		t.Fatal(err)
	}
	r := f.e.mustRun("search", "quartz", "--json")
	var hits []map[string]any
	if err := json.Unmarshal([]byte(r.out), &hits); err != nil || len(hits) != 4 {
		t.Fatalf("json: %v %s", err, r.out)
	}
	if !strings.HasPrefix(r.err, "cairn: warning") || !strings.Contains(r.err, "cairn project link --project tidepool web") {
		t.Fatalf("stderr %q", r.err)
	}
}

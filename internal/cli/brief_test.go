package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
)

func setClock(t *testing.T, ts string) {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatal(err)
	}
	old := now
	now = func() time.Time { return tm }
	t.Cleanup(func() { now = old })
}

// addTaskNote adds a note for a task through the CLI and returns its note ID and entry IDs.
func addTaskNote(t *testing.T, e *testEnv, task string, args ...string) (string, []string) {
	t.Helper()
	full := []string{"add", "--agent", "builder", "--repo", "server", "--task", task, "--model", "claude-test",
		"--title", "Work on " + task, "--status", "done"}
	r := e.mustRun(append(full, args...)...)
	return noteIDs(r.out)
}

// briefFixture: TP-03 has a decision later superseded, an open point later resolved,
// one decision in force and one open point; TP-03.1 is an extension; TP-04 is unrelated.
type briefFixture struct {
	note1, note2, other             string
	oldDecision, openDone, openLive string
	liveDecision                    string
}

func newBriefFixture(t *testing.T) (*testEnv, briefFixture) {
	t.Helper()
	e := newEnv(t)
	e.initProject()
	var f briefFixture
	setClock(t, "2026-10-30T10:00:00Z")
	var ids []string
	f.note1, ids = addTaskNote(t, e, "TP-03",
		"--change", "Added the tidal index.", "--decision", "user:Use a b-tree.",
		"--open", "Rebuild nightly?", "--open", "Which retention?", "--none", "note,helper")
	f.oldDecision, f.openDone, f.openLive = ids[1], ids[2], ids[3]
	setClock(t, "2026-10-30T11:00:00Z")
	_, ids = addTaskNote(t, e, "TP-03", "--decision", "agent:Use a hash index.", "--supersedes", f.oldDecision,
		"--resolves", f.openDone, "--none", "change,open,note,helper")
	f.liveDecision = ids[0]
	setClock(t, "2026-11-02T09:00:00Z")
	f.note2, _ = addTaskNote(t, e, "TP-03.1", "--change", "Client reads the index.", "--status", "in_progress",
		"--none", "decision,open,note,helper")
	f.other, _ = addTaskNote(t, e, "TP-04", "--change", "Unrelated work.", "--none", "decision,open,note,helper")
	return e, f
}

func TestBriefMergesNotes(t *testing.T) {
	e, f := newBriefFixture(t)
	r := e.mustRun("brief", "TP-03")
	for _, want := range []string{f.note1, f.note2, f.liveDecision, f.openLive, "Added the tidal index.", "Use a hash index.", "Which retention?",
		"decisions in force", "open points"} {
		if !strings.Contains(r.out, want) {
			t.Errorf("brief missing %q:\n%s", want, r.out)
		}
	}
	if strings.Contains(r.out, f.other) || strings.Contains(r.out, "Unrelated work.") {
		t.Errorf("brief leaked another task:\n%s", r.out)
	}
}

func TestBriefHidesClosed(t *testing.T) {
	e, f := newBriefFixture(t)
	r := e.mustRun("brief", "TP-03")
	for _, hidden := range []string{f.oldDecision, f.openDone, "Use a b-tree.", "Rebuild nightly?"} {
		if strings.Contains(r.out, hidden) {
			t.Errorf("brief shows closed item %q:\n%s", hidden, r.out)
		}
	}
}

func TestBriefIncludesExtensions(t *testing.T) {
	e, f := newBriefFixture(t)
	if r := e.mustRun("brief", "TP-03"); !strings.Contains(r.out, "Client reads the index.") || !strings.Contains(r.out, f.note2) {
		t.Fatalf("extension missing:\n%s", r.out)
	}
}

func TestBriefExactExtension(t *testing.T) {
	e, f := newBriefFixture(t)
	r := e.mustRun("brief", "TP-03.1")
	if !strings.Contains(r.out, f.note2) || strings.Contains(r.out, f.note1) || strings.Contains(r.out, "Added the tidal index.") {
		t.Fatalf("exact extension brief wrong:\n%s", r.out)
	}
}

func TestBriefUsesCurrentStatus(t *testing.T) {
	e, f := newBriefFixture(t)
	if r := e.mustRun("brief", "TP-03"); !strings.Contains(r.out, "in_progress") {
		t.Fatalf("TP-03.1 starts in_progress:\n%s", r.out)
	}
	s, err := store.Open(e.root + "/.cairn/cairn.db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertAddendum(model.Addendum{NoteID: f.note2, Agent: "builder", Model: "claude-test",
		At: "2026-11-03T00:00:00Z", Status: model.StatusBlocked}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	r := e.mustRun("brief", "TP-03")
	if !strings.Contains(r.out, "blocked") || strings.Contains(r.out, "in_progress") {
		t.Fatalf("brief must show the addendum status:\n%s", r.out)
	}
}

func TestBriefUnknownTask(t *testing.T) {
	e, _ := newBriefFixture(t)
	r := e.run("brief", "TP-99")
	if r.code != ExitOK || !strings.Contains(r.out, "no notes for task TP-99") {
		t.Fatalf("%+v", r)
	}
}

func TestBriefBadTaskIsUsageError(t *testing.T) {
	e, _ := newBriefFixture(t)
	for _, args := range [][]string{{"brief", "nope"}, {"brief"}, {"brief", "TP-03", "TP-04"}} {
		if r := e.run(args...); r.code != ExitUsage {
			t.Errorf("%v: want exit 2, got %+v", args, r)
		}
	}
}

func TestBriefMax(t *testing.T) {
	e, _ := newBriefFixture(t)
	r := e.mustRun("brief", "TP-03", "--max", "1")
	if !strings.Contains(r.out, "more (raise --max)") {
		t.Fatalf("no cap marker:\n%s", r.out)
	}
	if r := e.run("brief", "TP-03", "--max", "0"); r.code != ExitUsage {
		t.Fatalf("--max 0: %+v", r)
	}
}

func TestBriefJSON(t *testing.T) {
	e, f := newBriefFixture(t)
	r := e.mustRun("brief", "TP-03", "--json")
	var out struct {
		Task      string
		Notes     []struct{ ID, Status string }
		Changes   []struct{ ID, Text string }
		Decisions []struct{ ID string }
		Open      []struct{ ID string }
	}
	if err := json.Unmarshal([]byte(r.out), &out); err != nil {
		t.Fatalf("%v\n%s", err, r.out)
	}
	if out.Task != "TP-03" || len(out.Notes) != 3 || len(out.Decisions) != 1 || out.Decisions[0].ID != f.liveDecision ||
		len(out.Open) != 1 || out.Open[0].ID != f.openLive || len(out.Changes) != 2 {
		t.Fatalf("json: %+v", out)
	}
}

func TestBriefNoProject(t *testing.T) {
	e := newEnv(t)
	if r := e.run("brief", "TP-03"); r.code != ExitNoProject {
		t.Fatalf("%+v", r)
	}
}

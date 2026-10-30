package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
)

func TestCheckMissingNotes(t *testing.T) {
	e, _ := newBriefFixture(t)
	r := e.run("check", "--done", "TP-04,TP-99", "--older-than", "999")
	if r.code != ExitFindings || !strings.Contains(r.out, "TP-99") || strings.Contains(r.out, "TP-04 ") {
		t.Fatalf("%+v", r)
	}
	if r := e.run("check", "--done", "TP-04", "--older-than", "999"); r.code != ExitOK {
		t.Fatalf("TP-04 has a done note: %+v", r)
	}
}

func TestCheckExtensionCountsForBase(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	setClock(t, "2026-10-30T10:00:00Z")
	addTaskNote(t, e, "TP-07.1", "--change", "x", "--none", "decision,open,note,helper")
	if r := e.run("check", "--done", "TP-07", "--older-than", "999"); r.code != ExitOK {
		t.Fatalf("a base task is covered by its extension notes: %+v", r)
	}
	if r := e.run("check", "--done", "TP-07.2", "--older-than", "999"); r.code != ExitFindings {
		t.Fatalf("a different extension is not covered: %+v", r)
	}
}

func TestCheckDoneFileFormat(t *testing.T) {
	e, _ := newBriefFixture(t)
	file := filepath.Join(realTemp(t), "done.txt")
	os.WriteFile(file, []byte("# finished tasks\nTP-03\n\n  TP-98  # trailing comment\nTP-99\n"), 0o600)
	r := e.run("check", "--done-file", file, "--older-than", "999")
	if r.code != ExitFindings || !strings.Contains(r.out, "TP-98") || !strings.Contains(r.out, "TP-99") || strings.Contains(r.out, "trailing") {
		t.Fatalf("%+v", r)
	}
	r = e.runIn(e.root, "TP-97\n", "check", "--done-file", "-", "--older-than", "999")
	if r.code != ExitFindings || !strings.Contains(r.out, "TP-97") {
		t.Fatalf("stdin: %+v", r)
	}
	if r := e.run("check", "--done-file", filepath.Join(file, "nope")); r.code != ExitRuntime {
		t.Fatalf("unreadable file: %+v", r)
	}
}

func TestCheckUsesCurrentStatus(t *testing.T) {
	e, f := newBriefFixture(t)
	r := e.run("check", "--done", "TP-03.1", "--older-than", "999")
	if r.code != ExitFindings || !strings.Contains(r.out, "in_progress") {
		t.Fatalf("latest note is in_progress: %+v", r)
	}
	s, _ := store.Open(e.root + "/.cairn/cairn.db")
	if _, err := s.InsertAddendum(model.Addendum{NoteID: f.note2, Agent: "b", Model: "m", At: "2026-11-03T00:00:00Z", Status: model.StatusDone}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if r := e.run("check", "--done", "TP-03.1", "--older-than", "999"); r.code != ExitOK {
		t.Fatalf("addendum made it done: %+v", r)
	}
}

func TestCheckOldOpenPoints(t *testing.T) {
	e, f := newBriefFixture(t) // clock ends at 2026-11-02; open points date from 2026-10-30
	r := e.run("check", "--older-than", "2")
	if r.code != ExitFindings || !strings.Contains(r.out, f.openLive) || strings.Contains(r.out, f.openDone) {
		t.Fatalf("live open point older than 2 days must be listed, resolved one not:\n%+v", r)
	}
	if r := e.run("check", "--older-than", "14"); r.code != ExitOK {
		t.Fatalf("not old enough: %+v", r)
	}
}

func TestCheckUsageErrors(t *testing.T) {
	e, _ := newBriefFixture(t)
	for _, args := range [][]string{{"check", "--done", "nope"}, {"check", "--older-than", "-1"}, {"check", "--older-than", "x"}, {"check", "extra"}} {
		if r := e.run(args...); r.code != ExitUsage {
			t.Errorf("%v: want exit 2, got %+v", args, r)
		}
	}
	if r := newEnv(t).run("check"); r.code != ExitNoProject {
		t.Fatalf("no project: %+v", r)
	}
}

func TestCheckJSON(t *testing.T) {
	e, _ := newBriefFixture(t)
	r := e.run("check", "--done", "TP-99", "--older-than", "2", "--json")
	if r.code != ExitFindings || !strings.Contains(r.out, `"missing_notes"`) || !strings.Contains(r.out, "TP-99") || !strings.Contains(r.out, `"old_open"`) {
		t.Fatalf("%+v", r)
	}
}

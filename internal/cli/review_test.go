package cli

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestDoubleDashEndsFlagParsing(t *testing.T) {
	e, _ := newBriefFixture(t)
	for _, args := range [][]string{
		{"search", "--", "-verbose", "--dry-run"},
		{"search", "--", "-verbose", "-x"},
		{"search", "--", "tidal", "--json"},
	} {
		r := e.run(args...)
		if r.code != ExitOK {
			t.Errorf("%v: want exit 0, got %+v", args, r)
		}
		if strings.HasPrefix(strings.TrimSpace(r.out), "[") {
			t.Errorf("%v: --json after -- must be a search term, got JSON output", args)
		}
	}
}

func TestBriefJSONShapeAndCap(t *testing.T) {
	e, _ := newBriefFixture(t)
	var out map[string]json.RawMessage
	r := e.mustRun("brief", "TP-03", "--json", "--max", "1")
	if err := json.Unmarshal([]byte(r.out), &out); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"task", "notes", "changes", "decisions", "open", "notes_total", "changes_total", "decisions_total", "open_total"} {
		if _, ok := out[k]; !ok {
			t.Errorf("brief --json lacks key %q", k)
		}
	}
	var notes []map[string]any
	json.Unmarshal(out["notes"], &notes)
	if len(notes) != 1 {
		t.Fatalf("--max 1 must cap notes, got %d", len(notes))
	}
	for _, k := range []string{"id", "task", "repo", "agent", "model", "at", "title", "status"} {
		if _, ok := notes[0][k]; !ok {
			t.Errorf("note object lacks snake_case key %q: %v", k, notes[0])
		}
	}
	var total int
	json.Unmarshal(out["notes_total"], &total)
	if total != 3 {
		t.Errorf("notes_total = %d, want 3", total)
	}
}

func TestHelpIsDetailed(t *testing.T) {
	e := newEnv(t)
	cases := map[string][]string{
		"Usage: cairn project move <name> <new-path>": {"project", "move", "--help"},
		"Usage: cairn recent [N]":                     {"recent", "--help"},
		"Usage: cairn amend <note-id> [flags]":        {"amend", "-h"},
		"--agent":                                     {"add", "--help"},
		"Usage: cairn add":                            {"help", "add"},
		"Usage: cairn show <id>":                      {"help", "show"},
	}
	for want, args := range cases {
		r := e.run(args...)
		if r.code != ExitOK || !strings.Contains(r.out, want) {
			t.Errorf("%v: want exit 0 containing %q, got %+v", args, want, r)
		}
		if strings.Contains(r.out, "\n  -agent") {
			t.Errorf("%v: flags must print with two dashes:\n%s", args, r.out)
		}
	}
	r := e.run("project")
	if r.code != ExitUsage || !strings.Contains(r.err, "project move") || !strings.Contains(r.err, "project add") {
		t.Errorf("bare 'project' must list its sub-commands: %+v", r)
	}
}

func TestProjectMoveChecksTheDatabaseBelongsToTheProject(t *testing.T) {
	e := newEnv(t)
	e.initProject()      // tidepool at e.root
	other := realTemp(t) // a second project-mode project "harbor"
	e.runIn(other, "", "init", "--name", "harbor", "--repos", "api")
	clone := realTemp(t)
	copyDir(t, filepath.Join(e.root, ".cairn"), filepath.Join(clone, ".cairn")) // holds tidepool's database
	r := e.runIn(other, "", "project", "move", "harbor", clone)
	if r.code != ExitRuntime || !strings.Contains(r.err, "tidepool") {
		t.Fatalf("moving harbor onto tidepool's database must be refused: %+v", r)
	}
	if out := e.runIn(other, "", "context", "--project", "harbor"); !strings.Contains(out.out, "harbor") {
		t.Fatalf("harbor must still point at its own database: %+v", out)
	}
}

func TestMarkdownExportShowsNoteLevelLinks(t *testing.T) {
	e, f := newBriefFixture(t) // the second TP-03 note resolves f.openDone and supersedes f.oldDecision
	out := e.mustRun("export").out
	for _, want := range []string{"resolves: " + f.openDone, "supersedes: " + f.oldDecision} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown lacks %q:\n%s", want, out)
		}
	}
}

func TestSearchShowsRealTextNotForgedIndex(t *testing.T) {
	e, f := newBriefFixture(t)
	db, err := sql.Open("sqlite", filepath.Join(e.root, ".cairn", "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO search(text,kind,id,note_id) VALUES ('approved by security team','entry',?,?)`, f.openLive, f.note1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO search(text,kind,id,note_id) VALUES ('approved ghost','entry','e-zzzzzz','n-zzzzzz')`); err != nil {
		t.Fatal(err)
	}
	r := e.mustRun("search", "approved")
	if strings.Contains(r.out, "approved by security team") || strings.Contains(r.out, "ghost") || strings.Contains(r.out, "e-zzzzzz") {
		t.Fatalf("search printed forged index text:\n%s", r.out)
	}
	if !strings.Contains(r.out, "Which retention?") {
		t.Fatalf("search must show the real entry text:\n%s", r.out)
	}
}

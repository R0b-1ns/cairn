package cli

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

func TestInitProjectModeCreatesDotCairn(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	if !exists(filepath.Join(e.root, ".cairn", "cairn.db")) {
		t.Fatal("no .cairn/cairn.db")
	}
	r := e.mustRun("context")
	for _, want := range []string{"tidepool", "Tide data service.", "server", "client"} {
		if !strings.Contains(r.out, want) {
			t.Errorf("context output missing %q:\n%s", want, r.out)
		}
	}
}

func TestInitMachineMode(t *testing.T) {
	e := newEnv(t)
	e.initProject("--mode", "machine")
	if exists(filepath.Join(e.root, ".cairn")) {
		t.Fatal("machine mode must not create .cairn in the project")
	}
	if !exists(filepath.Join(e.home, "projects", "tidepool", "cairn.db")) {
		t.Fatal("machine database missing")
	}
	sub := filepath.Join(e.root, "deep", "er")
	os.MkdirAll(sub, 0o755)
	if r := e.runIn(sub, "", "context"); r.code != 0 || !strings.Contains(r.out, "tidepool") {
		t.Fatalf("context from subfolder: %+v", r)
	}
}

func TestInitRefusesOverwrite(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	db := filepath.Join(e.root, ".cairn", "cairn.db")
	before, _ := os.ReadFile(db)
	r := e.run("init", "--name", "tidepool", "--repos", "server")
	if r.code != ExitRuntime || !strings.Contains(r.err, "already") {
		t.Fatalf("want exit 1 'already', got %+v", r)
	}
	after, _ := os.ReadFile(db)
	if string(before) != string(after) {
		t.Fatal("database changed")
	}
}

func TestInitRefusesDuplicateNameOrPath(t *testing.T) {
	e := newEnv(t)
	e.initProject("--mode", "machine")
	other := realTemp(t)
	if r := e.runIn(other, "", "init", "--name", "tidepool", "--mode", "machine", "--repos", "server"); r.code == 0 {
		t.Fatal("duplicate name accepted")
	}
	if exists(filepath.Join(other, ".cairn")) {
		t.Fatal("leftover files after refused init")
	}
	if r := e.runIn(e.root, "", "init", "--name", "other", "--mode", "machine", "--repos", "server"); r.code == 0 {
		t.Fatal("duplicate path accepted")
	}
	if exists(filepath.Join(e.home, "projects", "other")) {
		t.Fatal("leftover machine db after refused init")
	}
}

func TestInitInvalidReportsFields(t *testing.T) {
	e := newEnv(t)
	r := e.run("init", "--name", "Bad Name", "--repos", "")
	if r.code != ExitValidation || !strings.Contains(r.err, "name:") || !strings.Contains(r.err, "repos:") {
		t.Fatalf("want exit 3 listing name and repos, got %+v", r)
	}
	if exists(filepath.Join(e.root, ".cairn")) || exists(e.home) {
		t.Fatal("invalid init created files")
	}
}

func TestNoProjectCreatesNothing(t *testing.T) {
	e := newEnv(t)
	for _, cmd := range [][]string{{"context"}} {
		r := e.run(cmd...)
		if r.code != ExitNoProject || !strings.Contains(r.err, "cairn init") {
			t.Fatalf("%v: want exit 4 mentioning 'cairn init', got %+v", cmd, r)
		}
	}
	entries, _ := os.ReadDir(e.root)
	if len(entries) != 0 {
		t.Fatalf("files created: %v", entries)
	}
	if exists(e.home) {
		t.Fatal("data home created")
	}
}

func TestInitRegistersBoth(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	other := realTemp(t)
	e.runIn(other, "", "init", "--name", "harbor", "--mode", "machine", "--repos", "api")
	r := e.mustRun("projects")
	for _, want := range []string{"tidepool", "project", "harbor", "machine", e.root, other} {
		if !strings.Contains(r.out, want) {
			t.Errorf("projects missing %q:\n%s", want, r.out)
		}
	}
}

func TestProjectsJSONAndMissingPath(t *testing.T) {
	e := newEnv(t)
	gone := realTemp(t)
	e.runIn(gone, "", "init", "--name", "harbor", "--mode", "machine", "--repos", "api")
	e.initProject()
	os.RemoveAll(gone)
	r := e.mustRun("projects", "--json")
	var rows []struct {
		Name    string `json:"name"`
		Mode    string `json:"mode"`
		Path    string `json:"path"`
		Notes   int    `json:"notes"`
		Missing bool   `json:"missing"`
	}
	if err := json.Unmarshal([]byte(r.out), &rows); err != nil {
		t.Fatalf("bad json: %v\n%s", err, r.out)
	}
	got := map[string]bool{}
	for _, row := range rows {
		got[row.Name] = row.Missing
	}
	if len(rows) != 2 || got["harbor"] != true || got["tidepool"] != false {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	if txt := e.mustRun("projects"); !strings.Contains(txt.out, "missing") {
		t.Fatalf("text output must flag missing path:\n%s", txt.out)
	}
}

func TestProjectMoveFixesPath(t *testing.T) {
	e := newEnv(t)
	e.initProject("--mode", "machine")
	moved := filepath.Join(realTemp(t), "moved")
	if err := os.Rename(e.root, moved); err != nil {
		t.Fatal(err)
	}
	if r := e.runIn(moved, "", "context"); r.code != ExitNoProject {
		t.Fatalf("stale registry path must not match: %+v", r)
	}
	if r := e.runIn(realTemp(t), "", "project", "move", "tidepool", moved); r.code != 0 {
		t.Fatalf("move: %+v", r)
	}
	if r := e.runIn(moved, "", "context"); r.code != 0 {
		t.Fatalf("context after move: %+v", r)
	}
	if r := e.runIn(moved, "", "projects"); strings.Contains(r.out, "missing") || !strings.Contains(r.out, moved) {
		t.Fatalf("projects after move:\n%s", r.out)
	}
	if r := e.runIn(moved, "", "project", "move", "tidepool", filepath.Join(moved, "nope")); r.code == 0 {
		t.Fatal("move to a non-existing folder must fail")
	}
	if r := e.runIn(moved, "", "project", "move", "ghost", moved); r.code == 0 {
		t.Fatal("move of unknown project must fail")
	}
}

func TestProjectAddFromClone(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	clone := realTemp(t)
	copyDir(t, filepath.Join(e.root, ".cairn"), filepath.Join(clone, ".cairn"))
	other := newEnv(t) // a second machine with its own registry
	r := other.runIn(realTemp(t), "", "project", "add", clone)
	if r.code != 0 {
		t.Fatalf("add: %+v", r)
	}
	if out := other.runIn(clone, "", "projects"); !strings.Contains(out.out, "tidepool") || !strings.Contains(out.out, clone) {
		t.Fatalf("projects:\n%s", out.out)
	}
	if out := other.runIn(clone, "", "context"); out.code != 0 {
		t.Fatalf("context in clone: %+v", out)
	}
}

func TestProjectAddRejectsTamperedDB(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	clone := realTemp(t)
	copyDir(t, filepath.Join(e.root, ".cairn"), filepath.Join(clone, ".cairn"))
	db, err := sql.Open("sqlite", filepath.Join(clone, ".cairn", "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DROP TRIGGER entries_no_delete"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	other := newEnv(t)
	r := other.runIn(realTemp(t), "", "project", "add", clone)
	if r.code != ExitRuntime || !strings.Contains(r.err, "trigger") {
		t.Fatalf("want exit 1 mentioning trigger, got %+v", r)
	}
	if exists(other.home) {
		t.Fatal("tampered database was registered")
	}
}

func TestProjectAddWithoutDB(t *testing.T) {
	e := newEnv(t)
	if r := e.runIn(realTemp(t), "", "project", "add", realTemp(t)); r.code == 0 {
		t.Fatal("add of a folder without .cairn must fail")
	}
}

func TestProjectRemoveKeepsDB(t *testing.T) {
	e := newEnv(t)
	e.initProject("--mode", "machine")
	db := filepath.Join(e.home, "projects", "tidepool", "cairn.db")
	r := e.run("project", "remove", "tidepool")
	if r.code != 0 || !exists(db) {
		t.Fatalf("remove: %+v, db exists=%v", r, exists(db))
	}
	if out := e.run("projects"); strings.Contains(out.out, "tidepool") {
		t.Fatalf("still listed:\n%s", out.out)
	}
	if r := e.run("project", "remove", "tidepool"); r.code == 0 {
		t.Fatal("second remove must fail")
	}
}

func TestProjectFlagAndEnvOverride(t *testing.T) {
	e := newEnv(t)
	e.initProject("--mode", "machine")
	elsewhere := realTemp(t)
	if r := e.runIn(elsewhere, "", "context"); r.code != ExitNoProject {
		t.Fatalf("want exit 4, got %+v", r)
	}
	if r := e.runIn(elsewhere, "", "context", "--project", "tidepool"); r.code != 0 || !strings.Contains(r.out, "tidepool") {
		t.Fatalf("--project: %+v", r)
	}
	e.env["CAIRN_PROJECT"] = "tidepool"
	if r := e.runIn(elsewhere, "", "context"); r.code != 0 {
		t.Fatalf("CAIRN_PROJECT: %+v", r)
	}
	if r := e.runIn(elsewhere, "", "context", "--project", "ghost"); r.code != ExitNoProject {
		t.Fatalf("unknown project: want exit 4, got %+v", r)
	}
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	if err := os.MkdirAll(to, 0o700); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	for _, en := range entries {
		b, err := os.ReadFile(filepath.Join(from, en.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(to, en.Name()), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

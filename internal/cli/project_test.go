package cli

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/tasks"
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

func contextJSON(t *testing.T, e *testEnv) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(e.mustRun("context", "--json").out), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestInitTasksAuto(t *testing.T) {
	e := newEnv(t)
	e.initProject("--tasks", "auto", "--task-prefix", "FEAT")
	m := contextJSON(t, e)
	want, _ := tasks.Pattern("auto", "FEAT")
	if m["task_mode"] != "auto" || m["task_prefix"] != "FEAT" || m["task_pattern"] != want {
		t.Fatalf("got %v", m)
	}
}

func TestInitTasksAutoDefaultPrefix(t *testing.T) {
	e := newEnv(t)
	e.initProject("--tasks", "auto")
	if m := contextJSON(t, e); m["task_prefix"] != "T" {
		t.Fatalf("got %v", m)
	}
}

func TestInitTasksJiraWithURL(t *testing.T) {
	e := newEnv(t)
	e.initProject("--tasks", "jira", "--task-url", "https://x.atlassian.net/browse/{task}")
	m := contextJSON(t, e)
	want, _ := tasks.Pattern("jira", "")
	if m["task_mode"] != "jira" || m["task_pattern"] != want || m["task_url"] != "https://x.atlassian.net/browse/{task}" {
		t.Fatalf("got %v", m)
	}
}

func TestInitTasksFlagErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		code int
	}{
		{"conflict", []string{"--tasks", "jira", "--task-pattern", "x"}, ExitUsage},
		{"conflict auto", []string{"--tasks", "auto", "--task-pattern", "x"}, ExitUsage},
		{"empty pattern", []string{"--task-pattern", ""}, ExitValidation},
		{"prefix alone", []string{"--task-prefix", "AB"}, ExitValidation},
		{"invalid prefix", []string{"--tasks", "auto", "--task-prefix", "ab1"}, ExitValidation},
		{"prefix without auto", []string{"--tasks", "jira", "--task-prefix", "AB"}, ExitValidation},
		{"bad url", []string{"--tasks", "jira", "--task-url", "https://x/y"}, ExitValidation},
		{"unknown mode", []string{"--tasks", "bogus"}, ExitValidation},
		{"custom needs pattern", []string{"--tasks", "custom"}, ExitValidation},
	}
	for _, tc := range cases {
		e := newEnv(t)
		r := e.run(append([]string{"init", "--name", "p", "--repos", "a"}, tc.args...)...)
		if r.code != tc.code {
			t.Errorf("%s: exit %d, want %d (%s)", tc.name, r.code, tc.code, r.err)
		}
		if exists(filepath.Join(e.root, ".cairn")) {
			t.Errorf("%s: database created", tc.name)
		}
	}
	e := newEnv(t)
	if r := e.run("init", "--name", "p", "--repos", "a", "--tasks", "bogus"); !strings.Contains(r.err, "auto") || !strings.Contains(r.err, "github") {
		t.Errorf("unknown mode must list valid modes: %s", r.err)
	}
}

func TestInitLegacyDefaultWithHint(t *testing.T) {
	e := newEnv(t)
	r := e.mustRun("init", "--name", "p", "--repos", "a")
	hint := "cairn: note: task numbering was not chosen; using the default TP-03 format (see --tasks)\n"
	if strings.Count(r.err, "task numbering was not chosen") != 1 || !strings.Contains(r.err, hint) {
		t.Fatalf("stderr: %q", r.err)
	}
	m := contextJSON(t, e)
	if m["task_mode"] != "custom" || m["task_pattern"] != model.DefaultTaskPattern {
		t.Fatalf("got %v", m)
	}
	e.mustRun(addWithTask("a", "TP-03")...)
}

func TestInitNoHintWhenTaskFlagGiven(t *testing.T) {
	e := newEnv(t)
	r := e.mustRun("init", "--name", "p", "--repos", "a", "--tasks", "github", "--no-input")
	if strings.Contains(r.err, "task numbering") {
		t.Fatalf("unexpected hint: %q", r.err)
	}
}

func TestExistingCustomPatternUnchanged(t *testing.T) {
	e := newEnv(t)
	e.initProject("--task-pattern", `^(?P<base>[A-Z]+-\d+)(?:/(?P<ext>[a-z]+))?$`)
	if m := contextJSON(t, e); m["task_mode"] != "custom" {
		t.Fatalf("got %v", m)
	}
	if r := e.run(addWithTask("server", "TP-3/api")...); r.code != 0 {
		t.Fatalf("TP-3/api: %+v", r)
	}
	if r := e.run(addWithTask("server", "tp-3")...); r.code != ExitValidation {
		t.Fatalf("tp-3: %+v", r)
	}
}

// addWithTask is a minimal valid flag-mode add for the given repo and task.
func addWithTask(repo, task string) []string {
	return withFlags(append(noneBut("change"), "--change", "x", "--repo", repo, "--task", task)...)
}

func TestInitTasksCustomWithPattern(t *testing.T) {
	e := newEnv(t)
	pat := `^(?P<base>[A-Z]+-\d+)$`
	e.initProject("--tasks", "custom", "--task-pattern", pat)
	m := contextJSON(t, e)
	if m["task_mode"] != "custom" || m["task_pattern"] != pat {
		t.Fatalf("got %v", m)
	}
}

func TestInitTasksLinearAndGitHub(t *testing.T) {
	for mode, id := range map[string]string{"linear": "ENG-42", "github": "#128"} {
		e := newEnv(t)
		e.initProject("--tasks", mode)
		want, _ := tasks.Pattern(mode, "")
		if m := contextJSON(t, e); m["task_mode"] != mode || m["task_pattern"] != want {
			t.Fatalf("%s: got %v", mode, m)
		}
		if r := e.run(addWithTask("server", id)...); r.code != 0 {
			t.Errorf("%s %s: %+v", mode, id, r)
		}
	}
}

func TestInitPrefixAloneMessage(t *testing.T) {
	e := newEnv(t)
	r := e.run("init", "--name", "p", "--repos", "a", "--task-prefix", "AB")
	if r.code != ExitValidation || !strings.Contains(r.err, "--tasks auto") {
		t.Fatalf("%+v", r)
	}
}

func TestInitTaskErrorsListedTogether(t *testing.T) {
	e := newEnv(t)
	r := e.run("init", "--name", "Bad", "--repos", "a", "--tasks", "bogus")
	if r.code != ExitValidation || !strings.Contains(r.err, "tasks:") || !strings.Contains(r.err, "name:") {
		t.Fatalf("%+v", r)
	}
}

func TestInitTaskURLAlone(t *testing.T) {
	e := newEnv(t)
	r := e.mustRun("init", "--name", "p", "--repos", "a", "--task-url", "https://x/{task}")
	if !strings.Contains(r.err, "task numbering was not chosen") {
		t.Fatalf("stderr: %q", r.err)
	}
}

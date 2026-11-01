package cli

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/R0b-1ns/cairn/internal/model"
)

// gitRepo makes e.root a git repository on the given branch, isolated from the
// developer's own git configuration.
func gitRepo(t *testing.T, e *testEnv, branch, userName string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Setenv("HOME", realTemp(t))
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	g := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = e.root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	g("init", "-b", branch)
	if userName != "" {
		g("config", "user.name", userName)
	}
	g("config", "user.email", "ada@x.org")
}

func shownNote(t *testing.T, e *testEnv, id string) model.Note {
	t.Helper()
	var n model.Note
	r := e.mustRun("show", id, "--json")
	if err := json.Unmarshal([]byte(r.out), &n); err != nil {
		t.Fatalf("show --json: %v\n%s", err, r.out)
	}
	return n
}

func addNote(t *testing.T, e *testEnv, extra ...string) string {
	t.Helper()
	args := append([]string{"add", "--agent", "builder", "--repo", "server", "--model", "m", "--title", "t",
		"--status", "done", "--change", "x", "--none", "decision,open,note,helper"}, extra...)
	id, _ := noteIDs(e.mustRun(args...).out)
	return id
}

func TestAddNewTaskAssignsAndContinues(t *testing.T) {
	e := newEnv(t)
	e.initProject("--tasks", "auto")
	a := e.mustRun("add", "--agent", "b", "--repo", "server", "--model", "m", "--title", "t", "--status", "in_progress",
		"--change", "x", "--none", "decision,open,note,helper", "--new-task")
	if !strings.Contains(a.out, "(T-1, in_progress)") {
		t.Fatalf("first: %s", a.out)
	}
	b := addNote(t, e, "--new-task")
	if got := shownNote(t, e, b).Task; got != "T-2" {
		t.Errorf("second task = %q", got)
	}
	c := addNote(t, e, "--task", "T-1")
	if got := shownNote(t, e, c).Task; got != "T-1" {
		t.Errorf("continued task = %q", got)
	}
}

func TestAddNewTaskErrors(t *testing.T) {
	e := newEnv(t)
	e.initProject("--tasks", "auto")
	r := e.run("add", "--agent", "b", "--repo", "server", "--model", "m", "--title", "t", "--status", "done",
		"--change", "x", "--none", "decision,open,note,helper", "--new-task", "--task", "T-1")
	if r.code != 2 {
		t.Errorf("with --task: code %d", r.code)
	}
	e2 := newEnv(t)
	e2.initProject()
	r = e2.run("add", "--agent", "b", "--repo", "server", "--model", "m", "--title", "t", "--status", "done",
		"--change", "x", "--none", "decision,open,note,helper", "--new-task")
	if r.code != 2 || !strings.Contains(r.err, "custom") {
		t.Errorf("legacy project: code %d, stderr %q", r.code, r.err)
	}
}

func TestAddRecordsBranchAndAuthor(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	gitRepo(t, e, "feat/x", "Ada Lovelace")
	n := shownNote(t, e, addNote(t, e))
	if n.Branch != "feat/x" || n.AuthorName != "Ada Lovelace" || n.AuthorEmail != "ada@x.org" {
		t.Errorf("git values: %+v", n)
	}
	n = shownNote(t, e, addNote(t, e, "--author", "Bob <b@x.org>", "--branch", "hotfix"))
	if n.Branch != "hotfix" || n.AuthorName != "Bob" || n.AuthorEmail != "b@x.org" {
		t.Errorf("flag values: %+v", n)
	}
	e.env["CAIRN_AUTHOR"] = "Eve <e@x.org>"
	n = shownNote(t, e, addNote(t, e))
	if n.AuthorName != "Eve" || n.AuthorEmail != "e@x.org" || n.Branch != "feat/x" {
		t.Errorf("env value: %+v", n)
	}
}

func TestAddDetachedHeadStoresNoBranch(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	gitRepo(t, e, "main", "Ada")
	for _, args := range [][]string{{"-c", "commit.gpgsign=false", "-c", "user.name=a", "-c", "user.email=a@b", "commit", "--allow-empty", "-m", "x"}, {"checkout", "--detach"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = e.root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	if n := shownNote(t, e, addNote(t, e)); n.Branch != "" {
		t.Errorf("branch = %q", n.Branch)
	}
}

func TestAddDropsInvalidGitValues(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	gitRepo(t, e, "main", "Ada\tLovelace")
	args := []string{"add", "--agent", "b", "--repo", "server", "--model", "m", "--title", "t", "--status", "done",
		"--change", "x", "--none", "decision,open,note,helper"}
	r := e.run(args...)
	if r.code != 0 {
		t.Fatalf("code %d: %s", r.code, r.err)
	}
	if strings.Count(r.err, "warning") != 1 {
		t.Errorf("stderr: %q", r.err)
	}
	id, _ := noteIDs(r.out)
	if n := shownNote(t, e, id); n.AuthorName != "" {
		t.Errorf("stored author %q", n.AuthorName)
	}
	r = e.run(append(args, "--author", "Ada\tLovelace")...)
	if r.code != 3 {
		t.Errorf("explicit: code %d", r.code)
	}
	r = e.run(append(args, "--branch", "a\tb")...)
	if r.code != 3 {
		t.Errorf("explicit branch: code %d", r.code)
	}
}

func TestAmendOnlyRefIsValid(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	id := addNote(t, e)
	r := e.run("amend", id, "--agent", "human", "--model", "none", "--ref", "https://github.com/a/b/issues/1")
	if r.code != 0 {
		t.Fatalf("code %d: %s", r.code, r.err)
	}
	if r = e.run("amend", id, "--agent", "human", "--model", "none", "--ref", "ftp://x"); r.code != 3 {
		t.Errorf("ftp: code %d", r.code)
	}
	r = e.mustRun("amend", id, "--agent", "human", "--model", "none", "--status", "done", "--author", "Bob <b@x.org>", "--branch", "hotfix", "--json")
	var a model.Addendum
	if err := json.Unmarshal([]byte(r.out), &a); err != nil {
		t.Fatal(err)
	}
	if a.AuthorName != "Bob" || a.Branch != "hotfix" {
		t.Errorf("%+v", a)
	}
}

func TestStdinNewTask(t *testing.T) {
	e := newEnv(t)
	e.initProject("--tasks", "auto")
	body := func(extra string) string {
		return `{"agent":"b","repo":"server","model":"m","title":"t","status":"done",` + extra +
			`"entries":[{"type":"change","text":"x"}],"none":["decision","open","note","helper"]}`
	}
	r := e.runIn(e.root, body(`"new_task":true,`), "add", "--stdin")
	if r.code != 0 || !strings.Contains(r.out, "(T-1, done)") {
		t.Fatalf("code %d: %s %s", r.code, r.out, r.err)
	}
	r = e.runIn(e.root, body(`"new_task":true,"task":"T-1",`), "add", "--stdin")
	if r.code != 3 {
		t.Errorf("with task: code %d", r.code)
	}
	e2 := newEnv(t)
	e2.initProject()
	r = e2.runIn(e2.root, body(`"new_task":true,`), "add", "--stdin")
	if r.code != 2 || !strings.Contains(r.err, "custom") {
		t.Errorf("legacy: code %d %s", r.code, r.err)
	}
	r = e.runIn(e.root, body(`"ref":"https://x.org/1","author_name":"Zed",`), "add", "--stdin", "--json")
	var n model.Note
	if err := json.Unmarshal([]byte(r.out), &n); err != nil || n.Ref != "https://x.org/1" || n.AuthorName != "Zed" {
		t.Errorf("stdin fields: %v %+v", err, n)
	}
}

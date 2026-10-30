package cli

import (
	"strings"
	"testing"
)

// addMinimal adds a one-change note without --repo.
func addMinimal(e *testEnv, extra ...string) result {
	args := []string{"add", "--agent", "builder", "--model", "claude-test", "--title", "Wire ingest",
		"--status", "done", "--change", "x", "--none", "decision,open,note,helper"}
	return e.run(append(args, extra...)...)
}

func TestSingleRepoProjectGetsItAsDefault(t *testing.T) {
	e := newEnv(t)
	e.mustRun("init", "--name", "tidepool", "--repos", "server")
	if out := e.mustRun("context").out; !strings.Contains(out, "default repo: server") {
		t.Fatalf("context must show the default repo:\n%s", out)
	}
	r := addMinimal(e)
	if r.code != ExitOK {
		t.Fatalf("add without --repo: %+v", r)
	}
	id, _ := noteIDs(r.out)
	if n := storeNote(t, e, id); n.Repo != "server" {
		t.Fatalf("repo = %q", n.Repo)
	}
}

func TestInitDefaultRepoFlag(t *testing.T) {
	e := newEnv(t)
	e.mustRun("init", "--name", "tidepool", "--repos", "server,client", "--default-repo", "client")
	r := addMinimal(e)
	id, _ := noteIDs(r.out)
	if r.code != ExitOK || storeNote(t, e, id).Repo != "client" {
		t.Fatalf("%+v", r)
	}
	// An explicit --repo still wins.
	r = addMinimal(e, "--repo", "server")
	id, _ = noteIDs(r.out)
	if r.code != ExitOK || storeNote(t, e, id).Repo != "server" {
		t.Fatalf("explicit repo: %+v", r)
	}
}

func TestSeveralReposHaveNoDefaultUnlessAsked(t *testing.T) {
	e := newEnv(t)
	e.mustRun("init", "--name", "tidepool", "--repos", "server,client")
	if strings.Contains(e.mustRun("context").out, "default repo") {
		t.Fatal("no default repo expected")
	}
	if r := addMinimal(e); r.code != ExitValidation || !strings.Contains(r.err, "repo:") || !strings.Contains(r.err, "--default-repo") {
		t.Fatalf("add without --repo and no default must list the repo field: %+v", r)
	}
}

func TestInitDefaultRepoMustBeOneOfTheRepos(t *testing.T) {
	e := newEnv(t)
	r := e.run("init", "--name", "tidepool", "--repos", "server,client", "--default-repo", "other")
	if r.code != ExitValidation || !strings.Contains(r.err, "default_repo:") {
		t.Fatalf("%+v", r)
	}
	if exists(e.root + "/.cairn") {
		t.Fatal("invalid init created files")
	}
}

func TestStdinNoteWithoutRepoUsesTheDefault(t *testing.T) {
	e := newEnv(t)
	e.mustRun("init", "--name", "tidepool", "--repos", "server")
	fixClock(t)
	in := strings.Replace(validJSON(), `"repo":"server",`, "", 1)
	r := e.runIn(e.root, in, "add", "--stdin")
	id, _ := noteIDs(r.out)
	if r.code != ExitOK || storeNote(t, e, id).Repo != "server" {
		t.Fatalf("%+v", r)
	}
}

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/R0b-1ns/cairn/internal/locate"
)

func TestOutdatedChildWarningNamesValidCommand(t *testing.T) {
	f := newChildFixture(t, true)
	// the advised form is a valid command
	if r := f.e.runIn(f.e.root, "", "context", "--project", "web"); r.code != 0 {
		t.Fatalf("advised command failed: %+v", r)
	}
	db := filepath.Join(f.child, ".cairn", "cairn.db")
	raw, err := sqlOpenRaw(db)
	if err != nil {
		t.Fatal(err)
	}
	raw.Exec("PRAGMA user_version = 4")
	raw.Close()
	_, warn := scopeRun(t, f.e)
	if !strings.Contains(warn, "cairn context --project web") {
		t.Errorf("valid command missing: %q", warn)
	}
	if strings.Contains(warn, "cairn --project") || strings.Contains(warn, "project link") {
		t.Errorf("invalid command or link hint in: %q", warn)
	}
}

func TestExportKeepsStoredDefaultRepo(t *testing.T) {
	e := newEnv(t)
	e.mustRun("init", "--name", "tidepool", "--repos", "api", "--default-repo", "api")
	child := realTemp(t)
	if r := e.runIn(child, "", "init", "--project", "tidepool", "--repo", "web"); r.code != 0 {
		t.Fatalf("%+v", r)
	}
	r := e.runIn(child, "", "export", "--format", "json")
	if r.code != 0 {
		t.Fatalf("%+v", r)
	}
	var doc struct {
		Project struct {
			DefaultRepo string `json:"default_repo"`
		} `json:"project"`
	}
	if err := json.Unmarshal([]byte(r.out), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Project.DefaultRepo != "api" {
		t.Errorf("export default_repo = %q, want api", doc.Project.DefaultRepo)
	}
	if out := e.runIn(child, "", "context").out; !strings.Contains(out, "default repo: web") {
		t.Errorf("context lost the effective default: %s", out)
	}
	a := e.runIn(child, "", "add", "--agent", "a", "--model", "m", "--title", "T", "--status", "done",
		"--change", "x", "--none", "decision,open,note,helper")
	if a.code != 0 {
		t.Fatalf("add: %+v", a)
	}
	id, _ := noteIDs(a.out)
	if n := storeNote(t, e, id); n.Repo != "web" {
		t.Errorf("add stored repo %q", n.Repo)
	}
}

func TestHintPathsAreShellQuoted(t *testing.T) {
	if got := locate.ShellQuote("/a b/it's"); got != `'/a b/it'\''s'` {
		t.Errorf("quote = %s", got)
	}
	f := newChildFixture(t, false)
	stray := filepath.Join(realTemp(t), "it's here")
	os.MkdirAll(stray, 0o755)
	if r := f.e.runIn(f.e.root, "", "project", "unlink", "--project", "tidepool", "web"); r.code != 0 {
		t.Fatalf("%+v", r)
	}
	// pointer in stray without registry link
	if err := locate.WritePointer(stray, locate.Pointer{Parent: "tidepool", Repo: "web"}); err != nil {
		t.Fatal(err)
	}
	r := f.e.runIn(stray, "", "context")
	want := "cairn project link --project tidepool web " + locate.ShellQuote(stray)
	if r.code == 0 || !strings.Contains(r.err, want) {
		t.Errorf("want %q in %+v", want, r)
	}
	if strings.Contains(r.err, "run 'cairn init'") {
		t.Errorf("generic init advice on a pointer error: %s", r.err)
	}
}

func TestInitRefusesFolderWithPointer(t *testing.T) {
	f := newChildFixture(t, false)
	r := f.e.runIn(f.child, "", "init", "--name", "other", "--repos", "x")
	if r.code == 0 || !strings.Contains(r.err, "cairn project unlink") {
		t.Errorf("%+v", r)
	}
	if _, err := os.Stat(filepath.Join(f.child, ".cairn", "cairn.db")); err == nil {
		t.Error("a database was created")
	}
}

func TestInitLocalRefusesAdoptingAParent(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	d := realTemp(t)
	if r := e.runIn(d, "", "init", "--name", "solo", "--repos", "web"); r.code != 0 {
		t.Fatalf("%+v", r)
	}
	kid := realTemp(t)
	if r := e.runIn(kid, "", "init", "--project", "solo", "--repo", "kid"); r.code != 0 {
		t.Fatalf("%+v", r)
	}
	r := e.runIn(d, "", "init", "--project", "tidepool", "--repo", "web", "--local", "--name", "solo")
	if r.code == 0 || !strings.Contains(r.err, "nesting is not supported") {
		t.Errorf("%+v", r)
	}
}

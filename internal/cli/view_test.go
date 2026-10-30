package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/R0b-1ns/cairn/internal/model"
)

// seedView adds two notes of different tasks, one with an addendum, and returns their IDs.
func seedView(t *testing.T, e *testEnv) (string, string) {
	t.Helper()
	a, ents := seed(t, e, "2026-10-30T10:00:00Z", noteArgs("Wire ingest", "TP-03", "server", "in_progress",
		append([]string{"--change", "Started ingest", "--open", "Prune old buckets?"}, noneBut("change", "open")...)...)...)
	mustAddendum(t, e, model.Addendum{
		NoteID: a, Agent: "reviewer", Model: "claude-test", At: "2026-10-31T09:00:00Z",
		Status:   model.StatusDone,
		Resolves: []string{ents[1]},
		Entries:  []model.Entry{{Type: model.TypeChange, Text: "Verified ingest"}},
	})
	b, _ := seed(t, e, "2026-11-01T10:00:00Z", noteArgs("Draw tide chart", "TP-04", "client", "done",
		append([]string{"--decision", "user:Use hourly points."}, noneBut("decision")...)...)...)
	return a, b
}

// noBrowser makes openBrowser fail the test if called, and restores it at the end.
func noBrowser(t *testing.T) {
	t.Helper()
	old := openBrowser
	openBrowser = func(url string) error { t.Errorf("openBrowser(%q) called", url); return nil }
	t.Cleanup(func() { openBrowser = old })
}

func TestViewWritesSiteInProjectMode(t *testing.T) {
	e := newAddEnv(t)
	a, b := seedView(t, e)
	r := e.run("view")
	if r.code != ExitOK {
		t.Fatalf("view: %+v", r)
	}
	dir := filepath.Join(e.root, ".cairn", "view")
	for _, f := range []string{"index.html", "notes/" + a + ".html", "notes/" + a + ".md", "notes/" + b + ".md"} {
		if !exists(filepath.Join(dir, filepath.FromSlash(f))) {
			t.Errorf("missing %s", f)
		}
	}
	if !strings.Contains(r.out, "wrote "+dir) || !strings.Contains(r.out, "open "+filepath.Join(dir, "index.html")) {
		t.Errorf("output does not name the folder:\n%s", r.out)
	}
	if strings.Contains(r.out, "serving") {
		t.Errorf("serving without --serve:\n%s", r.out)
	}
}

func TestViewMachineModeFolder(t *testing.T) {
	e := newEnv(t)
	fixClock(t)
	e.initProject("--mode", "machine")
	r := e.run("view")
	if r.code != ExitOK {
		t.Fatalf("view: %+v", r)
	}
	dir := filepath.Join(e.home, "projects", "tidepool", "view")
	if !exists(filepath.Join(dir, "index.html")) {
		t.Fatalf("no site under %s", dir)
	}
	if !strings.Contains(r.out, dir) {
		t.Errorf("output does not name %s:\n%s", dir, r.out)
	}
}

func dbSum(t *testing.T, path string) [32]byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(b)
}

func TestViewLeavesDatabaseUntouched(t *testing.T) {
	e := newAddEnv(t)
	seedView(t, e)
	db := filepath.Join(e.root, ".cairn", "cairn.db")
	before := dbSum(t, db)
	e.mustRun("view")
	if dbSum(t, db) != before {
		t.Fatal("cairn view changed the database file")
	}
	e.mustRun("check")
	e.mustRun("list", "--all")
}

func TestViewIsIdempotent(t *testing.T) {
	e := newAddEnv(t)
	a, _ := seedView(t, e)
	e.mustRun("view")
	f := filepath.Join(e.root, ".cairn", "view", "notes", a+".md")
	first, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	e.mustRun("view")
	second, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("second run wrote a different note file")
	}
}

func TestViewDownloadMatchesExport(t *testing.T) {
	e := newAddEnv(t)
	a, b := seedView(t, e)
	e.mustRun("view")
	for id, task := range map[string]string{a: "TP-03", b: "TP-04"} {
		got, err := os.ReadFile(filepath.Join(e.root, ".cairn", "view", "notes", id+".md"))
		if err != nil {
			t.Fatal(err)
		}
		want := e.mustRun("export", "--format", "md", "--task", task).out
		if string(got) != want {
			t.Errorf("%s.md differs from export --task %s\n--- got\n%s\n--- want\n%s", id, task, got, want)
		}
	}
}

func TestViewJSON(t *testing.T) {
	e := newAddEnv(t)
	r := e.run("view", "--json")
	if r.code != ExitOK {
		t.Fatalf("view --json: %+v", r)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(r.out), &got); err != nil {
		t.Fatalf("not one JSON object: %v\n%s", err, r.out)
	}
	dir := filepath.Join(e.root, ".cairn", "view")
	want := map[string]string{"dir": dir, "index": filepath.Join(dir, "index.html"), "url": ""}
	if len(got) != len(want) {
		t.Fatalf("keys: %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestViewUsageErrors(t *testing.T) {
	e := newAddEnv(t)
	for _, args := range [][]string{
		{"view", "extra"},
		{"view", "--serve", "--port", "70000"},
		{"view", "--serve", "--port", "-1"},
		{"view", "--no-open"},
		{"view", "--port", "8080"},
		{"view", "--port", "0"},
	} {
		if r := e.run(args...); r.code != ExitUsage {
			t.Errorf("%v: exit %d, want %d (%s)", args, r.code, ExitUsage, r.err)
		}
	}
	if exists(filepath.Join(e.root, ".cairn", "view")) {
		t.Error("a usage error wrote the site")
	}
}

func TestViewNoProject(t *testing.T) {
	e := newEnv(t)
	if r := e.run("view"); r.code != ExitNoProject {
		t.Fatalf("exit %d, want %d", r.code, ExitNoProject)
	}
}

// fakeServe replaces serveContext and openBrowser. With cancelOnOpen the
// context ends when the browser is opened; otherwise it is already cancelled.
func fakeServe(t *testing.T, cancelOnOpen bool) *[]string {
	t.Helper()
	oldCtx, oldOpen := serveContext, openBrowser
	t.Cleanup(func() { serveContext, openBrowser = oldCtx, oldOpen })
	var mu sync.Mutex
	var opened []string
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if !cancelOnOpen {
		cancel()
	}
	serveContext = func() (context.Context, context.CancelFunc) { return ctx, cancel }
	openBrowser = func(url string) error {
		mu.Lock()
		opened = append(opened, url)
		mu.Unlock()
		cancel()
		return nil
	}
	return &opened
}

func TestViewServe(t *testing.T) {
	e := newAddEnv(t)
	opened := fakeServe(t, true)
	r := e.run("view", "--serve")
	if r.code != ExitOK {
		t.Fatalf("view --serve: %+v", r)
	}
	if len(*opened) != 1 || !strings.HasPrefix((*opened)[0], "http://127.0.0.1:") {
		t.Fatalf("browser opened with %v", *opened)
	}
	if !strings.Contains(r.out, "serving "+(*opened)[0]+" (press Ctrl-C to stop)") {
		t.Errorf("no serving line for %s:\n%s", (*opened)[0], r.out)
	}

	opened = fakeServe(t, false)
	r = e.run("view", "--serve", "--no-open", "--json")
	if r.code != ExitOK {
		t.Fatalf("view --serve --no-open: %+v", r)
	}
	if len(*opened) != 0 {
		t.Errorf("--no-open opened the browser: %v", *opened)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(r.out), &got); err != nil {
		t.Fatalf("not one JSON object: %v\n%s", err, r.out)
	}
	if !strings.HasPrefix(got["url"], "http://127.0.0.1:") {
		t.Errorf("url = %q", got["url"])
	}
}

func TestViewBrowserFailureWarns(t *testing.T) {
	e := newAddEnv(t)
	fakeServe(t, false)
	openBrowser = func(string) error { return os.ErrNotExist }
	r := e.run("view", "--serve")
	if r.code != ExitOK {
		t.Fatalf("view --serve: %+v", r)
	}
	if !strings.Contains(r.err, "cairn: warning: could not open the browser: ") {
		t.Errorf("no warning:\n%s", r.err)
	}
}

func TestViewSymlinkedViewDir(t *testing.T) {
	e := newAddEnv(t)
	noBrowser(t)
	target := realTemp(t)
	if err := os.Symlink(target, filepath.Join(e.root, ".cairn", "view")); err != nil {
		t.Fatal(err)
	}
	if r := e.run("view"); r.code != ExitRuntime {
		t.Fatalf("exit %d, want %d", r.code, ExitRuntime)
	}
	if ents, _ := os.ReadDir(target); len(ents) != 0 {
		t.Errorf("wrote through the symlink: %v", ents)
	}
}

func TestViewServeBindFailure(t *testing.T) {
	e := newAddEnv(t)
	fakeServe(t, true)
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	if r := e.run("view", "--serve", "--port", port); r.code != ExitRuntime {
		t.Fatalf("exit %d, want %d (%s)", r.code, ExitRuntime, r.err)
	}
}

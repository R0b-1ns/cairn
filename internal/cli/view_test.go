package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

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

// Only a hostile cloned database can hold escape sequences or direction overrides;
// the site must drop them as the terminal output does.
func TestViewSanitizesSiteText(t *testing.T) {
	e := newAddEnv(t)
	s := openStore(t, e)
	n, err := s.InsertNote(model.Note{
		Agent: "build\u202eer", Repo: "server", Task: "TP-03", Model: "claude\x1b[31m-test",
		At: "2026-10-30T10:00:00Z", Title: "Evil \x1b[2Jtitle\u202e", Status: model.StatusDone,
		Entries: []model.Entry{{Type: model.TypeDecision, By: "user", Text: "Bad \x1b]0;pwn\x07 text\u202e\x1b[31m"}},
		None:    []model.EntryType{model.TypeChange, model.TypeOpen, model.TypeNote, model.TypeHelper},
	})
	if err != nil {
		t.Fatalf("insert raw note: %v", err)
	}
	mustAddendum(t, e, model.Addendum{NoteID: n.ID, Agent: "rev\x1b[1miewer", Model: "m", At: "2026-10-31T09:00:00Z",
		Entries: []model.Entry{{Type: model.TypeNote, Text: "late \u202enote"}}})
	s.Close()
	noBrowser(t)
	if r := e.run("view"); r.code != ExitOK {
		t.Fatalf("view: %+v", r)
	}
	dir := filepath.Join(e.root, ".cairn", "view")
	for _, f := range []string{"index.html", "decisions.html", "notes/" + n.ID + ".html", "notes/" + n.ID + ".md", "data.js"} {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f)))
		if err != nil {
			t.Fatal(err)
		}
		body := string(b)
		for _, bad := range []string{"\x1b", "\\u001b", "\u202e", "\x07", "\\u0007", "[2J", "[31m", "[1m", "0;pwn"} {
			if strings.Contains(body, bad) {
				t.Errorf("%s contains %q", f, bad)
			}
		}
		for _, want := range []string{"Evil title", "builder"} {
			if f != "decisions.html" && !strings.Contains(body, want) {
				t.Errorf("%s lacks %q", f, want)
			}
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "decisions.html")); !strings.Contains(string(b), "Bad  text") {
		t.Errorf("decisions.html lacks the cleaned decision text")
	}
}

// Once Ctrl-C has been received, the signal handling is given back at once, so a
// second Ctrl-C kills the process instead of waiting out the graceful shutdown.
func TestViewServeRestoresSignalsOnStop(t *testing.T) {
	e := newAddEnv(t)
	oldCtx, oldOpen := serveContext, openBrowser
	t.Cleanup(func() { serveContext, openBrowser = oldCtx, oldOpen })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	stopped := make(chan struct{})
	var once sync.Once
	serveContext = func() (context.Context, context.CancelFunc) {
		return ctx, func() { once.Do(func() { close(stopped) }); cancel() }
	}
	var conn net.Conn
	t.Cleanup(func() {
		if conn != nil {
			conn.Close()
		}
	})
	openBrowser = func(url string) error {
		// A request still being read keeps the graceful shutdown waiting.
		c, err := net.Dial("tcp", strings.TrimSuffix(strings.TrimPrefix(url, "http://"), "/"))
		if err != nil {
			t.Error(err)
		} else {
			conn = c
			c.Write([]byte("GET / HTTP/1.1\r\n"))
			// Connections are accepted in order: once a later request is answered,
			// the held one has been accepted too.
			if resp, err := http.Get(url); err != nil {
				t.Error(err)
			} else {
				resp.Body.Close()
			}
		}
		cancel() // the first Ctrl-C
		return nil
	}
	done := make(chan result, 1)
	go func() { done <- e.run("view", "--serve") }()
	select {
	case <-stopped:
	case r := <-done:
		t.Fatalf("signal handling given back only after the server stopped: %+v", r)
	case <-time.After(3 * time.Second):
		t.Fatal("signal handling not given back while shutting down")
	}
	select {
	case r := <-done:
		t.Fatalf("server returned before the stop function was checked: %+v", r)
	default:
	}
	conn.Close()
	conn = nil
	if r := <-done; r.code != ExitOK {
		t.Fatalf("view --serve: %+v", r)
	}
}

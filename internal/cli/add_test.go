package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
)

var (
	noteIDRe  = regexp.MustCompile(`n-[0-9a-hjkmnp-tv-z]{6}`)
	entryIDRe = regexp.MustCompile(`e-[0-9a-hjkmnp-tv-z]{6}`)
)

// noteIDs returns the note ID on the first line of add output and the entry IDs
// in the order they are listed.
func noteIDs(out string) (string, []string) {
	note := noteIDRe.FindString(out)
	return note, entryIDRe.FindAllString(out, -1)
}

// fixClock pins the tool clock for the duration of the test.
func fixClock(t *testing.T) {
	t.Helper()
	old := now
	now = func() time.Time { return time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { now = old })
}

// newAddEnv returns an initialized tidepool project with a fixed clock.
func newAddEnv(t *testing.T) *testEnv {
	t.Helper()
	e := newEnv(t)
	e.initProject()
	fixClock(t)
	return e
}

// validJSON returns a valid note for tidepool with one entry of every type.
func validJSON() string {
	return `{"agent":"builder","repo":"server","task":"TP-03","model":"claude-test","title":"Wire ingest","status":"done","entries":[` +
		`{"type":"change","text":"Added ingest loop"},` +
		`{"type":"decision","text":"Keep batches small","by":"user"},` +
		`{"type":"open","text":"Retry policy unknown"},` +
		`{"type":"note","text":"Measured latency"},` +
		`{"type":"helper","text":"Ran the bench"}]}`
}

// baseFlags are the flags every flag-mode add needs besides its entries.
func baseFlags() []string {
	return []string{"add", "--agent", "builder", "--repo", "server", "--task", "TP-03",
		"--model", "claude-test", "--title", "Wire ingest", "--status", "done"}
}

// noneBut declares every section empty except the given types.
func noneBut(keep ...string) []string {
	skip := map[string]bool{}
	for _, k := range keep {
		skip[k] = true
	}
	var rest []string
	for _, t := range []string{"change", "decision", "open", "note", "helper"} {
		if !skip[t] {
			rest = append(rest, t)
		}
	}
	return []string{"--none", strings.Join(rest, ",")}
}

func withFlags(extra ...string) []string {
	return append(baseFlags(), extra...)
}

// storeNote opens the tidepool database and returns the note with the given ID.
func storeNote(t *testing.T, e *testEnv, id string) model.Note {
	t.Helper()
	s, err := store.Open(filepath.Join(e.root, ".cairn", "cairn.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()
	n, err := s.GetNote(id)
	if err != nil {
		t.Fatalf("get note %s: %v", id, err)
	}
	return n
}

// countNotes returns how many notes the tidepool database holds.
func countNotes(t *testing.T, e *testEnv) int {
	t.Helper()
	s, err := store.Open(filepath.Join(e.root, ".cairn", "cairn.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()
	n, err := s.CountNotes()
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAddFlags(t *testing.T) {
	e := newAddEnv(t)
	r := e.run(withFlags(
		"--note", "Measured latency",
		"--change", "Added ingest loop",
		"--impact", "server", "--impact", "client",
		"--commit", "abc1234", "--commit", "def5678",
		"--open", "Retry policy unknown",
		"--decision", "user:Keep batches small",
		"--none", "helper",
	)...)
	if r.code != ExitOK {
		t.Fatalf("add: %+v", r)
	}
	id, entries := noteIDs(r.out)
	if id == "" || !strings.HasPrefix(strings.SplitN(r.out, "\n", 2)[0], "note "+id+" added") {
		t.Fatalf("first line should be 'note n-xxxxxx added':\n%s", r.out)
	}
	if len(entries) != 4 {
		t.Fatalf("want 4 entry lines, got %d:\n%s", len(entries), r.out)
	}
	n := storeNote(t, e, id)
	if n.Agent != "builder" || n.Repo != "server" || n.Task != "TP-03" || n.Model != "claude-test" ||
		n.Title != "Wire ingest" || n.Status != model.StatusDone {
		t.Fatalf("stored fields wrong: %+v", n)
	}
	if n.At != "2026-10-30T10:00:00Z" {
		t.Fatalf("At = %q", n.At)
	}
	want := []struct {
		typ  model.EntryType
		text string
		by   string
	}{
		{model.TypeChange, "Added ingest loop", ""},
		{model.TypeDecision, "Keep batches small", "user"},
		{model.TypeOpen, "Retry policy unknown", ""},
		{model.TypeNote, "Measured latency", ""},
	}
	if len(n.Entries) != len(want) {
		t.Fatalf("entries: %+v", n.Entries)
	}
	for i, w := range want {
		got := n.Entries[i]
		if got.Type != w.typ || got.Text != w.text || got.By != w.by {
			t.Errorf("entries[%d] = %+v, want %+v", i, got, w)
		}
	}
	if len(n.None) != 1 || n.None[0] != model.TypeHelper {
		t.Errorf("None = %v", n.None)
	}
	if strings.Join(n.Impacts, ",") != "server,client" {
		t.Errorf("Impacts = %v", n.Impacts)
	}
	if strings.Join(n.Commits, ",") != "abc1234,def5678" {
		t.Errorf("Commits = %v", n.Commits)
	}
}

func TestAddStdin(t *testing.T) {
	e := newAddEnv(t)
	r := e.runIn(e.root, validJSON(), "add", "--stdin")
	if r.code != ExitOK {
		t.Fatalf("add --stdin: %+v", r)
	}
	id, entries := noteIDs(r.out)
	if id == "" || len(entries) != 5 {
		t.Fatalf("unexpected output:\n%s", r.out)
	}
	n := storeNote(t, e, id)
	if n.Title != "Wire ingest" || len(n.Entries) != 5 {
		t.Fatalf("stored note: %+v", n)
	}
}

func TestAddJSONOutput(t *testing.T) {
	e := newAddEnv(t)
	r := e.run(withFlags(append([]string{"--change", "Added ingest loop", "--json"}, noneBut("change")...)...)...)
	if r.code != ExitOK {
		t.Fatalf("add --json: %+v", r)
	}
	var n model.Note
	if err := json.Unmarshal([]byte(r.out), &n); err != nil {
		t.Fatalf("output is not a note: %v\n%s", err, r.out)
	}
	if !noteIDRe.MatchString(n.ID) || n.Title != "Wire ingest" || n.At != "2026-10-30T10:00:00Z" {
		t.Fatalf("note fields: %+v", n)
	}
	if len(n.Entries) != 1 || !entryIDRe.MatchString(n.Entries[0].ID) || n.Entries[0].Text != "Added ingest loop" {
		t.Fatalf("entries: %+v", n.Entries)
	}
}

func TestAddStdinMalformed(t *testing.T) {
	valid := validJSON()
	cases := map[string]string{
		"truncated":     `{"agent":`,
		"unknown field": strings.Replace(valid, `"agent"`, `"bogus":1,"agent"`, 1),
		"trailing doc":  valid + ` {}`,
		"wrong type":    strings.Replace(valid, `"title":"Wire ingest"`, `"title":5`, 1),
		"empty input":   ``,
		"null":          `null`,
		"array":         `[]`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			e := newAddEnv(t)
			r := e.runIn(e.root, input, "add", "--stdin")
			if r.code != ExitValidation {
				t.Fatalf("want exit 3, got %+v", r)
			}
			if n := countNotes(t, e); n != 0 {
				t.Fatalf("%d notes stored", n)
			}
		})
	}
}

func TestAddStdinRejectsIDsAndAt(t *testing.T) {
	valid := validJSON()
	cases := map[string]struct{ input, field string }{
		"note id":  {strings.Replace(valid, `{"agent"`, `{"id":"n-abcdef","agent"`, 1), "id"},
		"entry id": {strings.Replace(valid, `{"type":"change"`, `{"id":"e-abcdef","type":"change"`, 1), "id"},
		"note at":  {strings.Replace(valid, `{"agent"`, `{"at":"2026-10-30T10:00:00Z","agent"`, 1), "at"},
		"entry at": {strings.Replace(valid, `{"type":"change"`, `{"at":"2026-10-30T10:00:00Z","type":"change"`, 1), "at"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			e := newAddEnv(t)
			r := e.runIn(e.root, c.input, "add", "--stdin")
			if r.code != ExitValidation || !strings.Contains(r.err, c.field) {
				t.Fatalf("want exit 3 mentioning %q, got %+v", c.field, r)
			}
			if n := countNotes(t, e); n != 0 {
				t.Fatalf("%d notes stored", n)
			}
		})
	}
}

func TestStdinOversize(t *testing.T) {
	e := newAddEnv(t)
	input := validJSON() + strings.Repeat(" ", 1048576)
	r := e.runIn(e.root, input, "add", "--stdin")
	if r.code != ExitValidation || !strings.Contains(r.err, "too large") {
		t.Fatalf("want exit 3 'too large', got %+v", r)
	}
	if n := countNotes(t, e); n != 0 {
		t.Fatalf("%d notes stored", n)
	}
}

func TestAddBOMAndCRLF(t *testing.T) {
	e := newAddEnv(t)
	input := "\ufeff" + strings.Replace(validJSON(), `"Added ingest loop"`, `"line one\r\nline two"`, 1)
	r := e.runIn(e.root, input, "add", "--stdin")
	if r.code != ExitOK {
		t.Fatalf("add --stdin with BOM: %+v", r)
	}
	id, _ := noteIDs(r.out)
	n := storeNote(t, e, id)
	if got := n.Entries[0].Text; got != "line one\nline two" {
		t.Fatalf("entries[0].text = %q, want LF line endings", got)
	}
}

func TestAddRejectsEscapeAndBadUTF8(t *testing.T) {
	cases := map[string]struct {
		run   func(e *testEnv) result
		field string
	}{
		"escape in flag": {func(e *testEnv) result {
			return e.run(withFlags("--change", "bad\x1bthing")...)
		}, "entries[0].text"},
		"bad utf-8 in flag": {func(e *testEnv) result {
			return e.run(append(withFlags("--change", "ok"), "--title", "\xff")...)
		}, "title"},
		"escape in json": {func(e *testEnv) result {
			return e.runIn(e.root, strings.Replace(validJSON(), `"Added ingest loop"`, `"Added\u001b"`, 1), "add", "--stdin")
		}, "entries[0].text"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			e := newAddEnv(t)
			r := c.run(e)
			if r.code != ExitValidation || !strings.Contains(r.err, c.field) {
				t.Fatalf("want exit 3 naming %q, got %+v", c.field, r)
			}
			if n := countNotes(t, e); n != 0 {
				t.Fatalf("%d notes stored", n)
			}
		})
	}
}

func TestAddListsAllInvalidFields(t *testing.T) {
	e := newAddEnv(t)
	r := e.run("add", "--agent", "builder", "--repo", "nope", "--task", "TP-03",
		"--model", "claude-test", "--title", "", "--status", "x")
	if r.code != ExitValidation {
		t.Fatalf("want exit 3, got %+v", r)
	}
	for _, want := range []string{"repo:", "title:", "status:", "none:"} {
		if !strings.Contains(r.err, want) {
			t.Errorf("stderr missing %q:\n%s", want, r.err)
		}
	}
}

func TestAddSecretWarns(t *testing.T) {
	e := newAddEnv(t)
	secret := "password = " + "hunter2" + "hunter2"
	r := e.run(withFlags(append([]string{"--change", "Rotated config " + secret}, noneBut("change")...)...)...)
	if r.code != ExitOK {
		t.Fatalf("secret must not block: %+v", r)
	}
	if !strings.Contains(r.err, "possible secret") || !strings.Contains(r.err, "credential-assignment") {
		t.Fatalf("stderr should warn with rule name:\n%s", r.err)
	}
	if strings.Contains(r.err, "hunter2hunter2") || strings.Contains(r.out, "hunter2hunter2") {
		t.Fatalf("secret value printed:\nstdout: %s\nstderr: %s", r.out, r.err)
	}
	id, _ := noteIDs(r.out)
	if n := storeNote(t, e, id); len(n.Entries) != 1 {
		t.Fatalf("note not saved: %+v", n)
	}
}

func TestAddFlagsCombinedWithStdinIsUsageError(t *testing.T) {
	e := newAddEnv(t)
	for _, extra := range [][]string{
		{"--agent", "builder"},
		{"--title", "x"},
		{"--change", "x"},
	} {
		r := e.runIn(e.root, validJSON(), append([]string{"add", "--stdin"}, extra...)...)
		if r.code != ExitUsage {
			t.Errorf("--stdin with %v: want exit 2, got %+v", extra, r)
		}
	}
	if n := countNotes(t, e); n != 0 {
		t.Fatalf("%d notes stored", n)
	}
	if r := e.runIn(e.root, validJSON(), "add", "--stdin", "--project", "tidepool", "--json"); r.code != ExitOK {
		t.Fatalf("--stdin with --project and --json must work: %+v", r)
	}
}

func TestAddDecisionFlagNeedsPrefix(t *testing.T) {
	e := newAddEnv(t)
	r := e.run(withFlags("--decision", "Use WAL mode")...)
	if r.code != ExitValidation || !strings.Contains(r.err, "entries[0].by") {
		t.Fatalf("want exit 3 naming entries[0].by, got %+v", r)
	}
}

func TestAddTaskMustMatchPattern(t *testing.T) {
	e := newAddEnv(t)
	r := e.run("add", "--agent", "builder", "--repo", "server", "--task", "bogus",
		"--model", "claude-test", "--title", "Wire ingest", "--status", "done", "--change", "x", "--none", "decision,open,note,helper")
	if r.code != ExitValidation || !strings.Contains(r.err, "task:") {
		t.Fatalf("want exit 3 naming task, got %+v", r)
	}
}

func TestAddResolvesAcrossNotes(t *testing.T) {
	e := newAddEnv(t)
	first := e.mustRun(withFlags(append([]string{"--open", "Retry policy unknown"}, noneBut("open")...)...)...)
	_, openIDs := noteIDs(first.out)
	if len(openIDs) != 1 {
		t.Fatalf("first note entries: %s", first.out)
	}
	open := openIDs[0]

	second := e.run(withFlags(append([]string{"--change", "Fixed retry", "--resolves", open}, noneBut("change")...)...)...)
	if second.code != ExitOK {
		t.Fatalf("resolve: %+v", second)
	}
	id2, _ := noteIDs(second.out)
	if n := storeNote(t, e, id2); len(n.Resolves) != 1 || n.Resolves[0] != open {
		t.Fatalf("Resolves = %v, want [%s]", n.Resolves, open)
	}

	third := e.run(withFlags(append([]string{"--change", "Again", "--resolves", open}, noneBut("change")...)...)...)
	if third.code != ExitValidation || !strings.Contains(third.err, "already resolved") {
		t.Fatalf("second resolve: want exit 3 'already resolved', got %+v", third)
	}
}

func TestAddResolveWrongTypeRefused(t *testing.T) {
	e := newAddEnv(t)
	first := e.mustRun(withFlags(append([]string{"--decision", "user:Use WAL mode", "--open", "Retry policy unknown"}, noneBut("decision", "open")...)...)...)
	_, ids := noteIDs(first.out)
	if len(ids) != 2 {
		t.Fatalf("first note entries: %s", first.out)
	}
	decision, open := ids[0], ids[1]

	if r := e.run(withFlags(append([]string{"--change", "x", "--resolves", decision}, noneBut("change")...)...)...); r.code != ExitValidation || !strings.Contains(r.err, "resolves") {
		t.Fatalf("resolving a decision: want exit 3, got %+v", r)
	}
	if r := e.run(withFlags(append([]string{"--change", "x", "--supersedes", open}, noneBut("change")...)...)...); r.code != ExitValidation || !strings.Contains(r.err, "supersedes") {
		t.Fatalf("superseding an open entry: want exit 3, got %+v", r)
	}
}

func TestAddNoProject(t *testing.T) {
	e := newEnv(t)
	dir := realTemp(t)
	r := e.runIn(dir, "", withFlags("--change", "x")...)
	if r.code != ExitNoProject {
		t.Fatalf("want exit 4, got %+v", r)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("files created: %v", entries)
	}
	if exists(e.home) {
		t.Fatal("data home created")
	}
}

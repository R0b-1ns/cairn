package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
)

// setNow moves the tool clock to the given RFC3339 time until the test ends.
func setNow(t *testing.T, at string) {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, at)
	if err != nil {
		t.Fatal(err)
	}
	old := now
	now = func() time.Time { return ts }
	t.Cleanup(func() { now = old })
}

// noteArgs is an add command for a tidepool note; extra holds entries and other flags.
func noteArgs(title, task, repo, status string, extra ...string) []string {
	args := []string{"add", "--agent", "builder", "--repo", repo, "--model", "claude-test",
		"--title", title, "--status", status}
	if task != "" {
		args = append(args, "--task", task)
	}
	return append(args, extra...)
}

// seed adds a note at the given time and returns its ID and entry IDs.
func seed(t *testing.T, e *testEnv, at string, args ...string) (string, []string) {
	t.Helper()
	setNow(t, at)
	return noteIDs(e.mustRun(args...).out)
}

// openStore opens the tidepool database for direct writes; closed at test end.
func openStore(t *testing.T, e *testEnv) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(e.root, ".cairn", "cairn.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// mustAddendum stores an addendum directly, since amend is not implemented yet.
func mustAddendum(t *testing.T, e *testEnv, a model.Addendum) model.Addendum {
	t.Helper()
	got, err := openStore(t, e).InsertAddendum(a)
	if err != nil {
		t.Fatalf("insert addendum: %v", err)
	}
	return got
}

// lineWith returns the first line of out containing sub, failing the test if none does.
func lineWith(t *testing.T, out, sub string) string {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, sub) {
			return l
		}
	}
	t.Fatalf("no line contains %q:\n%s", sub, out)
	return ""
}

// anyLine reports whether some line of out contains every given substring.
func anyLine(out string, subs ...string) bool {
	for _, l := range strings.Split(out, "\n") {
		ok := true
		for _, s := range subs {
			if !strings.Contains(l, s) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func TestShowNote(t *testing.T) {
	e := newAddEnv(t)
	long := strings.Repeat("abcdefghij", 50) // 500 characters
	id, _ := seed(t, e, "2026-10-30T10:00:00Z", noteArgs("Wire ingest", "TP-03", "server", "done",
		append([]string{"--change", long, "--impact", "client", "--commit", "abc1234"}, noneBut("change")...)...)...)

	r := e.run("show", id)
	if r.code != ExitOK {
		t.Fatalf("show: %+v", r)
	}
	if !strings.Contains(r.out, long) {
		t.Fatalf("full 500-character text not shown:\n%s", r.out)
	}
	if strings.Contains(r.out, "…") {
		t.Errorf("show must not clip text:\n%s", r.out)
	}
	if !anyLine(r.out, id, "Wire ingest") {
		t.Errorf("no header line with ID and title:\n%s", r.out)
	}
	for _, want := range []string{"builder", "claude-test", "TP-03", "server", "2026-10-30", "done"} {
		if !strings.Contains(r.out, want) {
			t.Errorf("output missing %q:\n%s", want, r.out)
		}
	}
	if !strings.Contains(r.out, "client") || !strings.Contains(r.out, "abc1234") {
		t.Errorf("impacts or commits missing:\n%s", r.out)
	}
	none := lineWith(t, r.out, "none:")
	for _, typ := range []string{"decision", "open", "note", "helper"} {
		if !strings.Contains(none, typ) {
			t.Errorf("none line %q lacks %s", none, typ)
		}
	}

	rj := e.run("show", id, "--json")
	if rj.code != ExitOK {
		t.Fatalf("show --json: %+v", rj)
	}
	var n model.Note
	if err := json.Unmarshal([]byte(rj.out), &n); err != nil {
		t.Fatalf("show --json is not JSON: %v\n%s", err, rj.out)
	}
	if n.ID != id || n.Title != "Wire ingest" || len(n.Entries) != 1 || n.Entries[0].Text != long {
		t.Fatalf("show --json object: %+v", n)
	}
}

func TestShowNoteWithAddenda(t *testing.T) {
	e := newAddEnv(t)
	id, _ := seed(t, e, "2026-10-30T10:00:00Z", noteArgs("Wire ingest", "TP-03", "server", "in_progress",
		append([]string{"--change", "Started ingest"}, noneBut("change")...)...)...)
	a := mustAddendum(t, e, model.Addendum{
		NoteID: id, Agent: "reviewer", Model: "claude-test", At: "2026-10-31T09:00:00Z",
		Status:  model.StatusDone,
		Entries: []model.Entry{{Type: model.TypeChange, Text: "Verified ingest"}},
	})

	r := e.run("show", id)
	if r.code != ExitOK {
		t.Fatalf("show: %+v", r)
	}
	for _, want := range []string{a.ID, a.Entries[0].ID, "Verified ingest", "reviewer", "2026-10-31",
		"in_progress", "done"} {
		if !strings.Contains(r.out, want) {
			t.Errorf("output missing %q:\n%s", want, r.out)
		}
	}
}

func TestShowAddendum(t *testing.T) {
	e := newAddEnv(t)
	id, _ := seed(t, e, "2026-10-30T10:00:00Z", noteArgs("Wire ingest", "TP-03", "server", "done",
		append([]string{"--change", "Started ingest"}, noneBut("change")...)...)...)
	a := mustAddendum(t, e, model.Addendum{
		NoteID: id, Agent: "reviewer", Model: "claude-test", At: "2026-10-31T09:00:00Z",
		Entries: []model.Entry{{Type: model.TypeChange, Text: "Verified ingest"}},
	})

	r := e.run("show", a.ID)
	if r.code != ExitOK {
		t.Fatalf("show addendum: %+v", r)
	}
	for _, want := range []string{a.ID, a.Entries[0].ID, "Verified ingest", id} {
		if !strings.Contains(r.out, want) {
			t.Errorf("output missing %q:\n%s", want, r.out)
		}
	}
}

func TestShowEntry(t *testing.T) {
	e := newAddEnv(t)
	id, ents := seed(t, e, "2026-10-30T10:00:00Z", noteArgs("Wire ingest", "TP-03", "server", "in_progress",
		append([]string{"--open", "Retry policy unknown"}, noneBut("open")...)...)...)
	open := ents[0]

	r := e.run("show", open)
	if r.code != ExitOK {
		t.Fatalf("show entry: %+v", r)
	}
	for _, want := range []string{open, "Retry policy unknown", id} {
		if !strings.Contains(r.out, want) {
			t.Errorf("output missing %q:\n%s", want, r.out)
		}
	}

	closer := mustAddendum(t, e, model.Addendum{
		NoteID: id, Agent: "builder", Model: "claude-test", At: "2026-10-31T09:00:00Z",
		Resolves: []string{open},
	})
	r = e.run("show", open)
	if !strings.Contains(r.out, "resolved") || !strings.Contains(r.out, closer.ID) {
		t.Errorf("closed entry should show its closing addendum:\n%s", r.out)
	}

	fixed := mustAddendum(t, e, model.Addendum{
		NoteID: id, Agent: "builder", Model: "claude-test", At: "2026-11-01T09:00:00Z",
		Entries: []model.Entry{{Type: model.TypeChange, Text: "Fixed retry"}},
	})
	fixedEntry := fixed.Entries[0].ID
	r = e.run("show", fixedEntry)
	if r.code != ExitOK {
		t.Fatalf("show addendum entry: %+v", r)
	}
	for _, want := range []string{fixedEntry, "Fixed retry", fixed.ID, id} {
		if !strings.Contains(r.out, want) {
			t.Errorf("addendum entry output missing %q:\n%s", want, r.out)
		}
	}
}

func TestShowUnknownAndMalformed(t *testing.T) {
	e := newAddEnv(t)
	for _, id := range []string{"n-abcdef", "a-abcdef", "e-abcdef"} {
		r := e.run("show", id)
		if r.code != ExitRuntime || !strings.Contains(r.err, "not found") {
			t.Errorf("show %s: want exit 1 with 'not found', got %+v", id, r)
		}
	}
	for _, id := range []string{"junk", "n-12345", "x-abcdef"} {
		if r := e.run("show", id); r.code != ExitUsage {
			t.Errorf("show %s: want exit 2, got %+v", id, r)
		}
	}
	if r := e.run("show"); r.code != ExitUsage {
		t.Errorf("show without ID: want exit 2, got %+v", r)
	}
}

func TestShowNeverEmitsEscape(t *testing.T) {
	e := newAddEnv(t)
	s := openStore(t, e)
	n, err := s.InsertNote(model.Note{
		Agent: "builder", Repo: "server", Task: "TP-03", Model: "claude-test",
		At: "2026-10-30T10:00:00Z", Title: "Evil \x1b[2Jtitle", Status: model.StatusDone,
		Entries: []model.Entry{{Type: model.TypeChange, Text: "Bad \x1b]0;pwn\x07 text\x1b[31m"}},
		None:    []model.EntryType{model.TypeDecision, model.TypeOpen, model.TypeNote, model.TypeHelper},
	})
	if err != nil {
		t.Fatalf("insert raw note: %v", err)
	}
	s.Close()

	for _, args := range [][]string{{"show", n.ID}, {"show", n.ID, "--json"}, {"list", "--all"}} {
		r := e.run(args...)
		if r.code != ExitOK {
			t.Fatalf("%v: %+v", args, r)
		}
		if strings.Contains(r.out, "\x1b") {
			t.Errorf("%v emitted an ESC byte:\n%q", args, r.out)
		}
	}
}

func TestShowShowsSupersededDecisionText(t *testing.T) {
	e := newAddEnv(t)
	oldID, oldEnts := seed(t, e, "2026-10-01T10:00:00Z", noteArgs("Storage", "TP-03", "server", "done",
		append([]string{"--decision", "user:Use WAL mode"}, noneBut("decision")...)...)...)
	newID, _ := seed(t, e, "2026-10-05T10:00:00Z", noteArgs("Storage v2", "TP-03", "server", "done",
		append([]string{"--decision", "user:Use rollback journal", "--supersedes", oldEnts[0]}, noneBut("decision")...)...)...)

	r := e.run("show", oldID)
	if r.code != ExitOK {
		t.Fatalf("show: %+v", r)
	}
	if !strings.Contains(r.out, "Use WAL mode") {
		t.Errorf("superseded decision text must still be shown:\n%s", r.out)
	}
	if !strings.Contains(r.out, "[user]") {
		t.Errorf("decision should show [user]:\n%s", r.out)
	}
	if !anyLine(r.out, "superseded by", newID) {
		t.Errorf("superseded decision should name %s:\n%s", newID, r.out)
	}
}

func TestListHidesResolvedOpen(t *testing.T) {
	e := newAddEnv(t)
	id, ents := seed(t, e, "2026-10-01T10:00:00Z", noteArgs("Ingest", "TP-03", "server", "in_progress",
		append([]string{"--open", "Retry policy unknown", "--open", "Cache eviction unclear"}, noneBut("open")...)...)...)
	resolved, pending := ents[0], ents[1]
	mustAddendum(t, e, model.Addendum{
		NoteID: id, Agent: "builder", Model: "claude-test", At: "2026-10-02T10:00:00Z",
		Resolves: []string{resolved},
	})

	r := e.run("list")
	if r.code != ExitOK {
		t.Fatalf("list: %+v", r)
	}
	if strings.Contains(r.out, resolved) || strings.Contains(r.out, "Retry policy unknown") {
		t.Errorf("resolved open entry shown by default:\n%s", r.out)
	}
	if !strings.Contains(r.out, pending) || !strings.Contains(r.out, "Cache eviction unclear") {
		t.Errorf("still-open entry missing:\n%s", r.out)
	}
}

func TestListResolvedShowsClosingAddendum(t *testing.T) {
	e := newAddEnv(t)
	id, ents := seed(t, e, "2026-10-01T10:00:00Z", noteArgs("Ingest", "TP-03", "server", "in_progress",
		append([]string{"--open", "Retry policy unknown", "--open", "Cache eviction unclear"}, noneBut("open")...)...)...)
	resolved, pending := ents[0], ents[1]
	closer := mustAddendum(t, e, model.Addendum{
		NoteID: id, Agent: "builder", Model: "claude-test", At: "2026-10-02T10:00:00Z",
		Resolves: []string{resolved},
	})

	r := e.run("list", "--resolved")
	if r.code != ExitOK {
		t.Fatalf("list --resolved: %+v", r)
	}
	line := lineWith(t, r.out, resolved)
	if !strings.Contains(line, "(resolved by "+closer.ID+")") {
		t.Errorf("line should carry the closing marker: %q", line)
	}
	if !strings.Contains(line, id) {
		t.Errorf("line should carry the parent note ID: %q", line)
	}
	if strings.Contains(r.out, pending) {
		t.Errorf("--resolved must show only closed entries:\n%s", r.out)
	}
}

func TestListSupersededDecisions(t *testing.T) {
	e := newAddEnv(t)
	_, oldEnts := seed(t, e, "2026-10-01T10:00:00Z", noteArgs("Storage", "TP-03", "server", "done",
		append([]string{"--decision", "user:Use WAL mode", "--decision", "user:Keep batches small"}, noneBut("decision")...)...)...)
	wal, batches := oldEnts[0], oldEnts[1]
	newID, newEnts := seed(t, e, "2026-10-05T10:00:00Z", noteArgs("Storage v2", "TP-03", "server", "done",
		append([]string{"--decision", "user:Use rollback journal", "--supersedes", wal}, noneBut("decision")...)...)...)
	rollback := newEnts[0]

	r := e.run("list")
	if r.code != ExitOK {
		t.Fatalf("list: %+v", r)
	}
	if !strings.Contains(r.out, rollback) || !strings.Contains(r.out, batches) {
		t.Errorf("default list should show decisions in force:\n%s", r.out)
	}
	if strings.Contains(r.out, wal) || strings.Contains(r.out, "Use WAL mode") {
		t.Errorf("default list should hide superseded decisions:\n%s", r.out)
	}

	r = e.run("list", "--resolved")
	line := lineWith(t, r.out, wal)
	if !strings.Contains(line, "(superseded by "+newID+")") {
		t.Errorf("--resolved line should carry the superseding note: %q", line)
	}
	if strings.Contains(r.out, rollback) || strings.Contains(r.out, batches) {
		t.Errorf("--resolved must show only closed entries:\n%s", r.out)
	}

	r = e.run("list", "--all")
	for _, want := range []string{wal, rollback, batches} {
		if !strings.Contains(r.out, want) {
			t.Errorf("--all missing %s:\n%s", want, r.out)
		}
	}
	if !strings.Contains(lineWith(t, r.out, wal), "(superseded by "+newID+")") {
		t.Errorf("--all should mark the superseded decision:\n%s", r.out)
	}
}

func TestListFiltersCombine(t *testing.T) {
	e := newAddEnv(t)
	seed(t, e, "2026-10-01T10:00:00Z", noteArgs("Alpha", "TP-03", "server", "done",
		append([]string{"--change", "Alpha change", "--impact", "client"}, noneBut("change")...)...)...)
	seed(t, e, "2026-10-15T10:00:00Z", noteArgs("Beta", "TP-03", "server", "done",
		append([]string{"--change", "Beta change", "--open", "Beta open"}, noneBut("change", "open")...)...)...)
	seed(t, e, "2026-10-20T10:00:00Z", noteArgs("Gamma", "TP-04", "client", "done",
		append([]string{"--change", "Gamma change", "--impact", "server"}, noneBut("change")...)...)...)
	seed(t, e, "2026-10-25T10:00:00Z", noteArgs("Delta", "TP-03", "client", "done",
		append([]string{"--change", "Delta change"}, noneBut("change")...)...)...)

	r := e.run("list", "--type", "change", "--task", "TP-03", "--repo", "server", "--impacts", "client",
		"--since", "2026-10-01", "--until", "2026-10-10")
	if r.code != ExitOK || !strings.Contains(r.out, "Alpha change") ||
		strings.Contains(r.out, "Beta") || strings.Contains(r.out, "Gamma") || strings.Contains(r.out, "Delta") {
		t.Errorf("type+task+repo+impacts+dates: %+v", r)
	}

	r = e.run("list", "--task", "TP-03", "--repo", "server", "--since", "2026-10-15", "--until", "2026-10-15")
	if !strings.Contains(r.out, "Beta change") || !strings.Contains(r.out, "Beta open") ||
		strings.Contains(r.out, "Alpha change") || strings.Contains(r.out, "Delta change") {
		t.Errorf("same-day window must include the whole day:\n%s", r.out)
	}

	r = e.run("list", "--impacts", "server")
	if !strings.Contains(r.out, "Gamma change") || strings.Contains(r.out, "Alpha change") {
		t.Errorf("--impacts server:\n%s", r.out)
	}

	r = e.run("list", "--since", "2026-10-20", "--until", "2026-10-20")
	if !strings.Contains(r.out, "Gamma change") || strings.Contains(r.out, "Beta change") ||
		strings.Contains(r.out, "Delta change") {
		t.Errorf("--until must include the whole day:\n%s", r.out)
	}
}

func TestListTaskExtension(t *testing.T) {
	e := newAddEnv(t)
	seed(t, e, "2026-10-01T10:00:00Z", noteArgs("Base", "TP-03", "server", "done",
		append([]string{"--change", "Base change"}, noneBut("change")...)...)...)
	seed(t, e, "2026-10-02T10:00:00Z", noteArgs("Ext", "TP-03.1", "server", "done",
		append([]string{"--change", "Ext change"}, noneBut("change")...)...)...)
	seed(t, e, "2026-10-03T10:00:00Z", noteArgs("Other", "TP-04", "server", "done",
		append([]string{"--change", "Other change"}, noneBut("change")...)...)...)

	r := e.run("list", "--task", "TP-03")
	if !strings.Contains(r.out, "Base change") || !strings.Contains(r.out, "Ext change") ||
		strings.Contains(r.out, "Other change") {
		t.Errorf("TP-03 must include TP-03.1:\n%s", r.out)
	}
	r = e.run("list", "--task", "TP-03.1")
	if !strings.Contains(r.out, "Ext change") || strings.Contains(r.out, "Base change") ||
		strings.Contains(r.out, "Other change") {
		t.Errorf("TP-03.1 must match exactly:\n%s", r.out)
	}
}

func TestListOrderNewestFirst(t *testing.T) {
	e := newAddEnv(t)
	seed(t, e, "2026-10-01T10:00:00Z", noteArgs("First", "TP-03", "server", "done",
		append([]string{"--change", "First change"}, noneBut("change")...)...)...)
	seed(t, e, "2026-10-05T10:00:00Z", noteArgs("Second", "TP-03", "server", "done",
		append([]string{"--change", "Second change"}, noneBut("change")...)...)...)
	seed(t, e, "2026-10-03T10:00:00Z", noteArgs("Third", "TP-03", "server", "done",
		append([]string{"--change", "Third change"}, noneBut("change")...)...)...)
	setNow(t, "2026-10-06T10:00:00Z")
	r := e.runIn(e.root, `{"agent":"builder","repo":"server","task":"TP-03","model":"claude-test","title":"Fourth","status":"done",`+
		`"entries":[{"type":"change","text":"Fourth A"},{"type":"change","text":"Fourth B"}],`+
		`"none":["decision","open","note","helper"]}`, "add", "--stdin")
	if r.code != ExitOK {
		t.Fatalf("add fourth: %+v", r)
	}

	r = e.run("list")
	if r.code != ExitOK {
		t.Fatalf("list: %+v", r)
	}
	order := []string{"Fourth B", "Fourth A", "Second change", "Third change", "First change"}
	last := -1
	for _, want := range order {
		i := strings.Index(r.out, want)
		if i < 0 {
			t.Fatalf("missing %q:\n%s", want, r.out)
		}
		if i < last {
			t.Errorf("%q listed out of order:\n%s", want, r.out)
		}
		last = i
	}
}

func TestListClipsText(t *testing.T) {
	e := newAddEnv(t)
	text := "START " + strings.Repeat("x", 300) + " END"
	id, _ := seed(t, e, "2026-10-01T10:00:00Z", noteArgs("Long", "TP-03", "server", "done",
		append([]string{"--change", text + "\nsecond line"}, noneBut("change")...)...)...)

	r := e.run("list")
	if r.code != ExitOK {
		t.Fatalf("list: %+v", r)
	}
	if strings.Contains(r.out, "END") || strings.Contains(r.out, "second line") {
		t.Errorf("text not clipped:\n%s", r.out)
	}
	if !strings.Contains(r.out, id) {
		t.Errorf("parent note missing:\n%s", r.out)
	}
	line := lineWith(t, r.out, "START")
	seg := line[strings.Index(line, "START"):]
	cut := strings.Index(seg, "…")
	if cut < 0 {
		t.Fatalf("clipped text must end with an ellipsis: %q", line)
	}
	if n := len([]rune(seg[:cut+len("…")])); n != 160 {
		t.Errorf("clipped text is %d runes, want 160: %q", n, line)
	}
}

func TestMaxCapsOutput(t *testing.T) {
	e := newAddEnv(t)
	times := []string{"2026-10-01T10:00:00Z", "2026-10-02T10:00:00Z", "2026-10-03T10:00:00Z", "2026-10-04T10:00:00Z"}
	for i, at := range times {
		seed(t, e, at, noteArgs("Cap", "TP-03", "server", "done",
			append([]string{"--change", fmt.Sprintf("Cap change %d", i)}, noneBut("change")...)...)...)
	}

	r := e.run("list")
	if n := len(entryIDRe.FindAllString(r.out, -1)); n != 4 || strings.Contains(r.out, "more (raise --max)") {
		t.Fatalf("default list: want 4 entries and no 'more' line:\n%s", r.out)
	}

	r = e.run("list", "--max=2")
	if r.code != ExitOK {
		t.Fatalf("list --max=2: %+v", r)
	}
	if n := len(entryIDRe.FindAllString(r.out, -1)); n != 2 {
		t.Errorf("want 2 entry lines, got %d:\n%s", n, r.out)
	}
	if !strings.Contains(r.out, "... 2 more (raise --max)") {
		t.Errorf("missing final '... 2 more' line:\n%s", r.out)
	}
}

func TestMaxInvalid(t *testing.T) {
	e := newAddEnv(t)
	for _, v := range []string{"--max=0", "--max=-1", "--max=abc"} {
		if r := e.run("list", v); r.code != ExitUsage {
			t.Errorf("list %s: want exit 2, got %+v", v, r)
		}
	}
}

func TestBadDateExit2(t *testing.T) {
	e := newAddEnv(t)
	for _, args := range [][]string{
		{"list", "--since", "2026-13-45"},
		{"list", "--until", "notadate"},
		{"list", "--since", "2026/10/01"},
	} {
		if r := e.run(args...); r.code != ExitUsage {
			t.Errorf("%v: want exit 2, got %+v", args, r)
		}
	}
}

func TestListBadTypeExit2(t *testing.T) {
	e := newAddEnv(t)
	if r := e.run("list", "--type", "bogus"); r.code != ExitUsage {
		t.Fatalf("want exit 2, got %+v", r)
	}
}

func TestListJSON(t *testing.T) {
	e := newAddEnv(t)
	long := "JSON " + strings.Repeat("y", 300)
	seed(t, e, "2026-10-01T10:00:00Z", noteArgs("Json", "TP-03", "server", "done",
		append([]string{"--change", long}, noneBut("change")...)...)...)

	r := e.run("list", "--json")
	if r.code != ExitOK {
		t.Fatalf("list --json: %+v", r)
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(r.out), &items); err != nil {
		t.Fatalf("not a JSON array: %v\n%s", err, r.out)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 object, got %d", len(items))
	}
	for _, k := range []string{"id", "type", "text", "by", "note_id", "addendum_id", "task", "repo", "at",
		"closed", "closed_by", "closed_at"} {
		if _, ok := items[0][k]; !ok {
			t.Errorf("object missing key %q: %v", k, items[0])
		}
	}
	if got, _ := items[0]["text"].(string); got != long {
		t.Errorf("JSON text must not be clipped: %q", got)
	}
}

func TestListEmpty(t *testing.T) {
	e := newAddEnv(t)
	r := e.run("list")
	if r.code != ExitOK || !strings.Contains(r.out, "no entries") {
		t.Fatalf("want exit 0 with 'no entries', got %+v", r)
	}
}

func TestListNoProject(t *testing.T) {
	e := newEnv(t)
	if r := e.run("list"); r.code != ExitNoProject {
		t.Fatalf("want exit 4, got %+v", r)
	}
}

func TestRecentSubjects(t *testing.T) {
	e := newAddEnv(t)
	firstID, _ := seed(t, e, "2026-10-01T10:00:00Z", noteArgs("First base", "TP-03", "server", "done",
		append([]string{"--change", "First change"}, noneBut("change")...)...)...)
	seed(t, e, "2026-10-02T10:00:00Z", noteArgs("Ext base", "TP-03.1", "server", "in_progress",
		append([]string{"--change", "Ext change"}, noneBut("change")...)...)...)
	looseID, _ := seed(t, e, "2026-10-03T10:00:00Z", noteArgs("Loose note", "", "server", "blocked",
		append([]string{"--change", "Loose change"}, noneBut("change")...)...)...)
	seed(t, e, "2026-10-04T10:00:00Z", noteArgs("Other", "TP-04", "server", "done",
		append([]string{"--change", "Other change"}, noneBut("change")...)...)...)
	// A later addendum on the TP-03 note makes it the most recently active subject.
	mustAddendum(t, e, model.Addendum{
		NoteID: firstID, Agent: "builder", Model: "claude-test", At: "2026-10-05T10:00:00Z",
		Entries: []model.Entry{{Type: model.TypeChange, Text: "Late change"}},
	})

	r := e.run("recent")
	if r.code != ExitOK {
		t.Fatalf("recent: %+v", r)
	}
	tp03 := lineWith(t, r.out, "TP-03")
	if !strings.Contains(tp03, "2026-10-05") || !strings.Contains(tp03, "done") ||
		!strings.Contains(tp03, "First base") || strings.Contains(tp03, "in_progress") {
		t.Errorf("TP-03 line should use the latest active note: %q", tp03)
	}
	if !regexp.MustCompile(`(^|[^0-9])2 notes?`).MatchString(tp03) {
		t.Errorf("TP-03 line should count 2 notes: %q", tp03)
	}
	tp04 := lineWith(t, r.out, "TP-04")
	if !strings.Contains(tp04, "2026-10-04") || !strings.Contains(tp04, "Other") {
		t.Errorf("TP-04 line: %q", tp04)
	}
	loose := lineWith(t, r.out, looseID)
	if !strings.Contains(loose, "2026-10-03") || !strings.Contains(loose, "blocked") ||
		!strings.Contains(loose, "Loose note") {
		t.Errorf("loose note line: %q", loose)
	}
	i03, i04, iLoose := strings.Index(r.out, "TP-03"), strings.Index(r.out, "TP-04"), strings.Index(r.out, looseID)
	if !(i03 < i04 && i04 < iLoose) {
		t.Errorf("subjects must be ordered by latest activity, newest first:\n%s", r.out)
	}
}

func TestRecentInvalid(t *testing.T) {
	e := newAddEnv(t)
	for _, v := range []string{"0", "-3", "abc"} {
		if r := e.run("recent", v); r.code != ExitUsage {
			t.Errorf("recent %s: want exit 2, got %+v", v, r)
		}
	}
}

func TestRecentDefaultTen(t *testing.T) {
	e := newAddEnv(t)
	for i := 1; i <= 12; i++ {
		seed(t, e, fmt.Sprintf("2026-10-%02dT10:00:00Z", i),
			noteArgs(fmt.Sprintf("Task %d", i), fmt.Sprintf("TP-%02d", i), "server", "done",
				append([]string{"--change", "Work"}, noneBut("change")...)...)...)
	}

	r := e.run("recent")
	if r.code != ExitOK {
		t.Fatalf("recent: %+v", r)
	}
	if n := len(regexp.MustCompile(`TP-\d{2}`).FindAllString(r.out, -1)); n != 10 {
		t.Errorf("default recent: want 10 subjects, got %d:\n%s", n, r.out)
	}
	lineWith(t, r.out, "TP-12")
	if strings.Contains(r.out, "TP-01") || strings.Contains(r.out, "TP-02") {
		t.Errorf("oldest subjects must be cut:\n%s", r.out)
	}

	r = e.run("recent", "3")
	if n := len(regexp.MustCompile(`TP-\d{2}`).FindAllString(r.out, -1)); n != 3 {
		t.Errorf("recent 3: want 3 subjects, got %d:\n%s", n, r.out)
	}
	if strings.Contains(r.out, "TP-09") {
		t.Errorf("recent 3 must stop at TP-10:\n%s", r.out)
	}
}

func TestRecentJSON(t *testing.T) {
	e := newAddEnv(t)
	seed(t, e, "2026-10-01T10:00:00Z", noteArgs("First base", "TP-03", "server", "done",
		append([]string{"--change", "First change"}, noneBut("change")...)...)...)
	seed(t, e, "2026-10-02T10:00:00Z", noteArgs("Other", "TP-04", "server", "done",
		append([]string{"--change", "Other change"}, noneBut("change")...)...)...)

	r := e.run("recent", "--json")
	if r.code != ExitOK {
		t.Fatalf("recent --json: %+v", r)
	}
	var items []any
	if err := json.Unmarshal([]byte(r.out), &items); err != nil {
		t.Fatalf("not a JSON array: %v\n%s", err, r.out)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 subjects, got %d:\n%s", len(items), r.out)
	}
	if !strings.Contains(r.out, "TP-03") || !strings.Contains(r.out, "First base") {
		t.Errorf("JSON lacks subject or title:\n%s", r.out)
	}
}

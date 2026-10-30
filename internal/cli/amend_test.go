package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
)

var addendumIDRe = regexp.MustCompile(`a-[0-9a-hjkmnp-tv-z]{6}`)

// clockAt sets the tool clock to the given hour of 2026-10-30 UTC. fixClock restores it.
func clockAt(hour int) {
	now = func() time.Time { return time.Date(2026, 10, 30, hour, 0, 0, 0, time.UTC) }
}

// amendCmd builds an amend command line for the given note with agent and model set.
func amendCmd(note string, extra ...string) []string {
	return append([]string{"amend", note, "--agent", "builder", "--model", "claude-test"}, extra...)
}

// amendValidJSON returns a valid addendum for amend --stdin.
func amendValidJSON() string {
	return `{"agent":"builder","model":"claude-test","status":"blocked","entries":[` +
		`{"type":"change","text":"Added retry"},` +
		`{"type":"decision","text":"Cap retries at 5","by":"agent"}]}`
}

// plainNote adds a note with one change entry and returns its ID.
func plainNote(t *testing.T, e *testEnv) string {
	t.Helper()
	r := e.mustRun(withFlags(append([]string{"--change", "Initial change"}, noneBut("change")...)...)...)
	id, _ := noteIDs(r.out)
	return id
}

// openTidepool opens the tidepool database for inspection.
func openTidepool(t *testing.T, e *testEnv) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(e.root, ".cairn", "cairn.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// addendaOf returns the addenda stored on a note.
func addendaOf(t *testing.T, e *testEnv, noteID string) []model.Addendum {
	t.Helper()
	as, err := openTidepool(t, e).Addenda(noteID)
	if err != nil {
		t.Fatalf("addenda of %s: %v", noteID, err)
	}
	return as
}

// addendumID returns the addendum ID found in amend output.
func addendumID(out string) string { return addendumIDRe.FindString(out) }

func TestAmendAddsAddendum(t *testing.T) {
	e := newAddEnv(t)
	note := plainNote(t, e)
	clockAt(11)
	r := e.mustRun(amendCmd(note,
		"--change", "Fixed retry",
		"--open", "Cache size unknown",
		"--note", "Checked logs",
		"--helper", "Ran the script",
		"--decision", "agent:Use a bounded queue")...)

	addID := addendumID(r.out)
	if addID == "" || strings.SplitN(r.out, "\n", 2)[0] != "addendum "+addID+" added to "+note {
		t.Fatalf("first line should be 'addendum a-xxxxxx added to %s':\n%s", note, r.out)
	}
	lines := strings.Split(strings.TrimRight(r.out, "\n"), "\n")
	if len(lines) != 6 {
		t.Fatalf("want header plus 5 entry lines:\n%s", r.out)
	}
	types := map[string]bool{}
	for _, line := range lines[1:] {
		f := strings.Fields(line)
		if len(f) != 2 || !entryIDRe.MatchString(f[0]) {
			t.Fatalf("bad entry line %q", line)
		}
		types[f[1]] = true
	}
	for _, typ := range model.EntryTypes {
		if !types[string(typ)] {
			t.Errorf("no output line for type %s:\n%s", typ, r.out)
		}
	}

	as := addendaOf(t, e, note)
	if len(as) != 1 {
		t.Fatalf("addenda: %+v", as)
	}
	a := as[0]
	if a.ID != addID || a.NoteID != note || a.Agent != "builder" || a.Model != "claude-test" ||
		a.Status != "" || a.At != "2026-10-30T11:00:00Z" {
		t.Fatalf("stored addendum: %+v", a)
	}
	want := map[model.EntryType]struct{ text, by string }{
		model.TypeChange:   {"Fixed retry", ""},
		model.TypeOpen:     {"Cache size unknown", ""},
		model.TypeNote:     {"Checked logs", ""},
		model.TypeHelper:   {"Ran the script", ""},
		model.TypeDecision: {"Use a bounded queue", "agent"},
	}
	if len(a.Entries) != len(want) {
		t.Fatalf("entries: %+v", a.Entries)
	}
	for _, en := range a.Entries {
		w, ok := want[en.Type]
		if !ok || en.Text != w.text || en.By != w.by || !entryIDRe.MatchString(en.ID) {
			t.Errorf("entry %+v, want %+v", en, w)
		}
	}
}

func TestAmendNeverChangesNote(t *testing.T) {
	e := newAddEnv(t)
	note := plainNote(t, e)
	before := storeNote(t, e, note)
	clockAt(11)
	e.mustRun(amendCmd(note, "--change", "Fixed retry", "--status", "blocked")...)
	after := storeNote(t, e, note)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("note changed:\nbefore: %+v\nafter:  %+v", before, after)
	}
	if n := len(addendaOf(t, e, note)); n != 1 {
		t.Fatalf("want 1 addendum attached to the note, got %d", n)
	}
}

func TestAmendStatusOnly(t *testing.T) {
	e := newAddEnv(t)
	note := plainNote(t, e)
	clockAt(11)
	r := e.mustRun(amendCmd(note, "--status", "blocked")...)
	addID := addendumID(r.out)
	if addID == "" {
		t.Fatalf("no addendum ID in output:\n%s", r.out)
	}
	as := addendaOf(t, e, note)
	if len(as) != 1 || as[0].ID != addID || as[0].Status != model.StatusBlocked || len(as[0].Entries) != 0 {
		t.Fatalf("stored addenda: %+v", as)
	}
	st, err := openTidepool(t, e).CurrentStatus(note)
	if err != nil || st != model.StatusBlocked {
		t.Fatalf("CurrentStatus = %q, %v; want blocked", st, err)
	}
	if got := storeNote(t, e, note).Status; got != model.StatusDone {
		t.Fatalf("note status changed to %q", got)
	}
}

func TestAmendStdin(t *testing.T) {
	e := newAddEnv(t)
	note := plainNote(t, e)
	clockAt(11)
	r := e.runIn(e.root, amendValidJSON(), "amend", note, "--stdin")
	if r.code != ExitOK {
		t.Fatalf("amend --stdin: %+v", r)
	}
	addID := addendumID(r.out)
	as := addendaOf(t, e, note)
	if len(as) != 1 || as[0].ID != addID || as[0].Status != model.StatusBlocked || len(as[0].Entries) != 2 ||
		as[0].Entries[1].By != "agent" {
		t.Fatalf("stored addenda: %+v", as)
	}
}

func TestAmendStdinMalformed(t *testing.T) {
	valid := amendValidJSON()
	cases := map[string]string{
		"truncated":     `{"agent":`,
		"unknown field": strings.Replace(valid, `"agent"`, `"bogus":1,"agent"`, 1),
		"trailing doc":  valid + ` {}`,
		"wrong type":    strings.Replace(valid, `"status":"blocked"`, `"status":5`, 1),
		"empty input":   ``,
		"null":          `null`,
		"array":         `[]`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			e := newAddEnv(t)
			note := plainNote(t, e)
			r := e.runIn(e.root, input, "amend", note, "--stdin")
			if r.code != ExitValidation {
				t.Fatalf("want exit 3, got %+v", r)
			}
			if n := len(addendaOf(t, e, note)); n != 0 {
				t.Fatalf("%d addenda stored", n)
			}
		})
	}
}

func TestAmendStdinRejectsAssignedFields(t *testing.T) {
	valid := amendValidJSON()
	cases := map[string]struct{ input, field string }{
		"addendum id": {strings.Replace(valid, `{"agent"`, `{"id":"a-abcdef","agent"`, 1), "id"},
		"note id":     {strings.Replace(valid, `{"agent"`, `{"note_id":"n-abcdef","agent"`, 1), "note_id"},
		"at":          {strings.Replace(valid, `{"agent"`, `{"at":"2026-10-30T10:00:00Z","agent"`, 1), "at"},
		"entry id":    {strings.Replace(valid, `{"type":"change"`, `{"id":"e-abcdef","type":"change"`, 1), "id"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			e := newAddEnv(t)
			note := plainNote(t, e)
			r := e.runIn(e.root, c.input, "amend", note, "--stdin")
			if r.code != ExitValidation || !strings.Contains(r.err, c.field) {
				t.Fatalf("want exit 3 mentioning %q, got %+v", c.field, r)
			}
			if n := len(addendaOf(t, e, note)); n != 0 {
				t.Fatalf("%d addenda stored", n)
			}
		})
	}
}

func TestAmendFlagsCombinedWithStdinIsUsageError(t *testing.T) {
	e := newAddEnv(t)
	note := plainNote(t, e)
	for _, extra := range [][]string{
		{"--agent", "builder"},
		{"--status", "done"},
		{"--change", "x"},
	} {
		r := e.runIn(e.root, amendValidJSON(), append([]string{"amend", note, "--stdin"}, extra...)...)
		if r.code != ExitUsage {
			t.Errorf("--stdin with %v: want exit 2, got %+v", extra, r)
		}
	}
	if n := len(addendaOf(t, e, note)); n != 0 {
		t.Fatalf("%d addenda stored", n)
	}
	if r := e.runIn(e.root, amendValidJSON(), "amend", note, "--stdin", "--project", "tidepool", "--json"); r.code != ExitOK {
		t.Fatalf("--stdin with --project and --json must work: %+v", r)
	}
}

func TestAmendEmptyRefused(t *testing.T) {
	e := newAddEnv(t)
	note := plainNote(t, e)
	if r := e.run(amendCmd(note)...); r.code != ExitValidation || !strings.Contains(r.err, "addendum") {
		t.Fatalf("flags with no content: want exit 3 mentioning addendum, got %+v", r)
	}
	empty := `{"agent":"builder","model":"claude-test","entries":[]}`
	if r := e.runIn(e.root, empty, "amend", note, "--stdin"); r.code != ExitValidation || !strings.Contains(r.err, "addendum") {
		t.Fatalf("empty stdin addendum: want exit 3 mentioning addendum, got %+v", r)
	}
	if n := len(addendaOf(t, e, note)); n != 0 {
		t.Fatalf("%d addenda stored", n)
	}
}

func TestAmendUnknownNote(t *testing.T) {
	e := newAddEnv(t)
	r := e.run(amendCmd("n-zzzzzz", "--change", "x")...)
	if r.code != ExitRuntime || !strings.Contains(r.err, "not found") {
		t.Fatalf("want exit 1 'not found', got %+v", r)
	}
}

func TestAmendBadNoteID(t *testing.T) {
	e := newAddEnv(t)
	r := e.run(amendCmd("junk", "--change", "x")...)
	if r.code != ExitValidation || !strings.Contains(r.err, "note_id") {
		t.Fatalf("want exit 3 mentioning note_id, got %+v", r)
	}
}

func TestAmendMissingArgument(t *testing.T) {
	e := newAddEnv(t)
	r := e.run("amend", "--agent", "builder", "--model", "claude-test", "--change", "x")
	if r.code != ExitUsage {
		t.Fatalf("want exit 2, got %+v", r)
	}
}

func TestAmendResolvesAcrossNotes(t *testing.T) {
	e := newAddEnv(t)
	first := e.mustRun(withFlags(append([]string{"--open", "Retry policy unknown"}, noneBut("open")...)...)...)
	_, openIDs := noteIDs(first.out)
	if len(openIDs) != 1 {
		t.Fatalf("first note entries: %s", first.out)
	}
	open := openIDs[0]
	other := plainNote(t, e)
	clockAt(11)

	r := e.run(amendCmd(other, "--resolves", open, "--change", "Fixed retry")...)
	if r.code != ExitOK {
		t.Fatalf("resolve from another note: %+v", r)
	}
	addID := addendumID(r.out)
	info, err := openTidepool(t, e).Entry(open)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Closed || info.ClosedBy != addID {
		t.Fatalf("open entry state: %+v, want closed by %s", info, addID)
	}

	again := e.run(amendCmd(other, "--resolves", open, "--change", "Again")...)
	if again.code != ExitValidation || !strings.Contains(again.err, "already resolved") {
		t.Fatalf("second resolve: want exit 3 'already resolved', got %+v", again)
	}
}

func TestAmendSupersedesDecision(t *testing.T) {
	e := newAddEnv(t)
	first := e.mustRun(withFlags(append([]string{"--decision", "user:Use WAL mode"}, noneBut("decision")...)...)...)
	_, ids := noteIDs(first.out)
	if len(ids) != 1 {
		t.Fatalf("first note entries: %s", first.out)
	}
	old := ids[0]
	other := plainNote(t, e)
	clockAt(11)

	r := e.run(amendCmd(other, "--decision", "agent:Use WAL with a busy timeout", "--supersedes", old)...)
	if r.code != ExitOK {
		t.Fatalf("supersede: %+v", r)
	}
	addID := addendumID(r.out)
	info, err := openTidepool(t, e).Entry(old)
	if err != nil {
		t.Fatal(err)
	}
	if info.Text != "Use WAL mode" || !info.Closed || info.ClosedBy != addID {
		t.Fatalf("superseded decision: %+v, want text kept and closed by %s", info, addID)
	}
}

func TestAmendResolveWrongTypeRefused(t *testing.T) {
	e := newAddEnv(t)
	first := e.mustRun(withFlags(append([]string{"--decision", "user:Use WAL mode", "--open", "Retry policy unknown"}, noneBut("decision", "open")...)...)...)
	_, ids := noteIDs(first.out)
	if len(ids) != 2 {
		t.Fatalf("first note entries: %s", first.out)
	}
	decision, open := ids[0], ids[1]
	other := plainNote(t, e)

	if r := e.run(amendCmd(other, "--resolves", decision, "--change", "x")...); r.code != ExitValidation || !strings.Contains(r.err, "resolves") {
		t.Fatalf("resolving a decision: want exit 3, got %+v", r)
	}
	if r := e.run(amendCmd(other, "--supersedes", open, "--change", "x")...); r.code != ExitValidation || !strings.Contains(r.err, "supersedes") {
		t.Fatalf("superseding an open entry: want exit 3, got %+v", r)
	}
}

func TestAmendDecisionNeedsPrefix(t *testing.T) {
	e := newAddEnv(t)
	note := plainNote(t, e)
	r := e.run(amendCmd(note, "--decision", "Use WAL mode")...)
	if r.code != ExitValidation || !strings.Contains(r.err, "entries[0].by") {
		t.Fatalf("want exit 3 naming entries[0].by, got %+v", r)
	}
	if n := len(addendaOf(t, e, note)); n != 0 {
		t.Fatalf("%d addenda stored", n)
	}
}

func TestAmendSecretWarns(t *testing.T) {
	e := newAddEnv(t)
	note := plainNote(t, e)
	clockAt(11)
	secret := "password = " + "hunter2" + "hunter2"
	r := e.run(amendCmd(note, "--change", "Rotated config "+secret)...)
	if r.code != ExitOK {
		t.Fatalf("secret must not block: %+v", r)
	}
	if !strings.Contains(r.err, "possible secret") || !strings.Contains(r.err, "credential-assignment") {
		t.Fatalf("stderr should warn with rule name:\n%s", r.err)
	}
	if strings.Contains(r.err, "hunter2hunter2") || strings.Contains(r.out, "hunter2hunter2") {
		t.Fatalf("secret value printed:\nstdout: %s\nstderr: %s", r.out, r.err)
	}
	if n := len(addendaOf(t, e, note)); n != 1 {
		t.Fatalf("addendum not saved: %d stored", n)
	}
}

func TestAmendJSONOutput(t *testing.T) {
	e := newAddEnv(t)
	note := plainNote(t, e)
	clockAt(11)
	r := e.run(amendCmd(note, "--change", "Fixed retry", "--json")...)
	if r.code != ExitOK {
		t.Fatalf("amend --json: %+v", r)
	}
	var a model.Addendum
	if err := json.Unmarshal([]byte(r.out), &a); err != nil {
		t.Fatalf("output is not an addendum: %v\n%s", err, r.out)
	}
	if !addendumIDRe.MatchString(a.ID) || a.NoteID != note || a.At != "2026-10-30T11:00:00Z" {
		t.Fatalf("addendum fields: %+v", a)
	}
	if len(a.Entries) != 1 || !entryIDRe.MatchString(a.Entries[0].ID) || a.Entries[0].Text != "Fixed retry" {
		t.Fatalf("entries: %+v", a.Entries)
	}
}

func TestAmendNoProject(t *testing.T) {
	e := newEnv(t)
	dir := realTemp(t)
	r := e.runIn(dir, "", amendCmd("n-abcdef", "--change", "x")...)
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

func TestCurrentStatusLatestAddendumCLI(t *testing.T) {
	e := newAddEnv(t)
	note := plainNote(t, e)
	clockAt(12)
	later := e.mustRun(amendCmd(note, "--status", "blocked")...)
	laterID := addendumID(later.out)
	clockAt(11)
	e.mustRun(amendCmd(note, "--status", "in_progress")...)

	st, err := openTidepool(t, e).CurrentStatus(note)
	if err != nil || st != model.StatusBlocked {
		t.Fatalf("CurrentStatus = %q, %v; want blocked (the 12:00 addendum)", st, err)
	}
	as := addendaOf(t, e, note)
	if len(as) != 2 || as[0].Status != model.StatusInProgress || as[1].ID != laterID {
		t.Fatalf("addenda in time order: %+v", as)
	}
}

package validate

import (
	"strings"
	"testing"
	"time"

	"github.com/R0b-1ns/cairn/internal/model"
)

// validNote returns a fully valid note for project "tidepool".
// Entries[0] is change, [1] decision, [2] open, [3] note, [4] helper.
func validNote() model.Note {
	return model.Note{
		Agent:  "builder",
		Repo:   "server",
		At:     "2026-10-30T10:00:00Z",
		Task:   "TP-03",
		Model:  "claude-test",
		Title:  "Add tidal index",
		Status: model.StatusDone,
		Entries: []model.Entry{
			{Type: model.TypeChange, Text: "Added the tidal index table."},
			{Type: model.TypeDecision, Text: "Index buckets are hourly.", By: "user"},
			{Type: model.TypeOpen, Text: "Decide retention for old buckets."},
			{Type: model.TypeNote, Text: "Benchmarks run on the staging host."},
			{Type: model.TypeHelper, Text: "Use the index helper from the client repo."},
		},
	}
}

// tidepool returns a valid project configuration.
func tidepool() model.Project {
	return model.Project{
		Name:        "tidepool",
		Mode:        "project",
		Context:     "Tide data service.",
		Repos:       []string{"server", "client"},
		TaskPattern: model.DefaultTaskPattern,
	}
}

// validAddendum returns a fully valid addendum on note n-abcdef.
func validAddendum() model.Addendum {
	return model.Addendum{
		NoteID:  "n-abcdef",
		Agent:   "builder",
		Model:   "claude-test",
		At:      "2026-10-30T10:00:00Z",
		Entries: []model.Entry{{Type: model.TypeNote, Text: "Follow-up benchmark numbers."}},
	}
}

func findField(errs []FieldError, field string) *FieldError {
	for i := range errs {
		if errs[i].Field == field {
			return &errs[i]
		}
	}
	return nil
}

func hasField(errs []FieldError, field string) bool {
	return findField(errs, field) != nil
}

func TestRejectsControlChars(t *testing.T) {
	for _, s := range []string{"\x1b[31mred", "a\x00b", "a\rb"} {
		if err := Text("f", s, 100, true); err == nil {
			t.Errorf("Text(%q) accepted, want rejection", s)
		}
	}
	if err := Text("f", "line1\nline2\ttab", 100, true); err != nil {
		t.Errorf("multiline Text with newline and tab rejected: %v", err)
	}
	if err := Text("f", "a\nb", 100, false); err == nil {
		t.Error(`single-line Text("a\nb") accepted, want rejection`)
	}
}

func TestRejectsInvalidUTF8(t *testing.T) {
	if err := Text("f", "\xff\xfe", 100, false); err == nil {
		t.Error(`Text("\xff\xfe") accepted, want rejection`)
	}
}

func TestLimits(t *testing.T) {
	p := tidepool()

	n := validNote()
	n.Title = strings.Repeat("t", 201)
	if !hasField(Note(n, p, Options{}), "title") {
		t.Error("201-rune title not rejected with Field \"title\"")
	}

	n = validNote()
	n.Entries[0].Text = strings.Repeat("x", 4001)
	if !hasField(Note(n, p, Options{}), "entries[0].text") {
		t.Error("4001-rune entry text not rejected with Field \"entries[0].text\"")
	}

	n = validNote()
	n.Entries = nil
	for i := 0; i < 101; i++ {
		n.Entries = append(n.Entries, model.Entry{Type: model.TypeNote, Text: "item"})
	}
	if !hasField(Note(n, p, Options{}), "entries") {
		t.Error("101 entries not rejected with Field \"entries\"")
	}

	n = validNote()
	n.Impacts = make([]string, 51)
	for i := range n.Impacts {
		n.Impacts[i] = "server"
	}
	if len(Note(n, p, Options{})) == 0 {
		t.Error("51 impacts not rejected")
	}

	pr := tidepool()
	pr.Context = strings.Repeat("c", 2001)
	if !hasField(Project(pr), "context") {
		t.Error("2001-rune project context not rejected with Field \"context\"")
	}

	n = validNote()
	n.Entries[0].Text = strings.Repeat("x", 4000)
	if errs := Note(n, p, Options{}); len(errs) != 0 {
		t.Errorf("4000-rune entry text rejected: %v", errs)
	}
}

func TestEmptySectionsNeedNone(t *testing.T) {
	p := tidepool()

	// The helper entry is the last one in validNote.
	n := validNote()
	n.Entries = n.Entries[:4]
	e := findField(Note(n, p, Options{}), "none")
	if e == nil || !strings.Contains(e.Message, "helper") {
		t.Errorf("missing helper section without None: want Field \"none\" mentioning helper, got %v", e)
	}

	n = validNote()
	n.Entries = n.Entries[:4]
	n.None = []model.EntryType{model.TypeHelper}
	if errs := Note(n, p, Options{}); len(errs) != 0 {
		t.Errorf("helper declared in None still rejected: %v", errs)
	}

	n = validNote()
	n.None = []model.EntryType{model.TypeChange}
	if !hasField(Note(n, p, Options{}), "none") {
		t.Error("type present as entry and also in None not rejected")
	}
}

func TestUnknownRepoRejected(t *testing.T) {
	p := tidepool()

	n := validNote()
	n.Repo = "other"
	if !hasField(Note(n, p, Options{}), "repo") {
		t.Error(`Repo "other" not rejected with Field "repo"`)
	}

	n = validNote()
	n.Impacts = []string{"other"}
	if !hasField(Note(n, p, Options{}), "impacts") {
		t.Error(`Impacts ["other"] not rejected with Field "impacts"`)
	}
}

func TestTaskMustMatchPattern(t *testing.T) {
	p := tidepool()

	for _, task := range []string{"tp-3", "TP-03.1.1"} {
		n := validNote()
		n.Task = task
		if !hasField(Note(n, p, Options{}), "task") {
			t.Errorf("Task %q not rejected with Field \"task\"", task)
		}
	}

	n := validNote()
	n.Task = "TP-03.1"
	if hasField(Note(n, p, Options{}), "task") {
		t.Error(`Task "TP-03.1" rejected, want accepted`)
	}

	n = validNote()
	n.Task = ""
	if hasField(Note(n, p, Options{}), "task") {
		t.Error("empty Task rejected, want accepted")
	}
}

func TestDecisionNeedsBy(t *testing.T) {
	p := tidepool()

	// entries[1] is the decision.
	n := validNote()
	n.Entries[1].By = ""
	if !hasField(Note(n, p, Options{}), "entries[1].by") {
		t.Error("decision without By not rejected")
	}

	n = validNote()
	n.Entries[1].By = "robot"
	if !hasField(Note(n, p, Options{}), "entries[1].by") {
		t.Error(`decision with By "robot" not rejected`)
	}

	n = validNote()
	n.Entries[1].By = "agent"
	if errs := Note(n, p, Options{}); len(errs) != 0 {
		t.Errorf(`decision with By "agent" rejected: %v`, errs)
	}

	n = validNote()
	n.Entries[0].By = "user" // entries[0] is a change
	if !hasField(Note(n, p, Options{}), "entries[0].by") {
		t.Error("non-decision entry with By set not rejected")
	}
}

func TestStatusValues(t *testing.T) {
	p := tidepool()

	for _, s := range []model.Status{model.StatusDone, model.StatusInProgress, model.StatusBlocked} {
		n := validNote()
		n.Status = s
		if hasField(Note(n, p, Options{}), "status") {
			t.Errorf("status %q rejected, want accepted", s)
		}
	}

	for _, s := range []model.Status{"finished", ""} {
		n := validNote()
		n.Status = s
		if !hasField(Note(n, p, Options{}), "status") {
			t.Errorf("note status %q not rejected with Field \"status\"", s)
		}
	}

	a := validAddendum()
	a.Status = ""
	if hasField(Addendum(a, Options{}), "status") {
		t.Error("addendum with empty status rejected, want accepted")
	}

	a.Status = "finished"
	if !hasField(Addendum(a, Options{}), "status") {
		t.Error(`addendum status "finished" not rejected with Field "status"`)
	}
}

func TestEntryTextNoneRejected(t *testing.T) {
	p := tidepool()
	for _, text := range []string{"none", "NONE", "  None  "} {
		n := validNote()
		n.Entries[0].Text = text
		if len(Note(n, p, Options{})) == 0 {
			t.Errorf("entry text %q accepted, want rejection", text)
		}
	}
}

func TestModelUnknown(t *testing.T) {
	p := tidepool()

	n := validNote()
	n.Model = "unknown"
	if len(Note(n, p, Options{})) == 0 {
		t.Error(`Model "unknown" accepted by default, want rejection`)
	}
	if errs := Note(n, p, Options{AllowUnknownModel: true}); len(errs) != 0 {
		t.Errorf(`Model "unknown" rejected with AllowUnknownModel: %v`, errs)
	}

	n = validNote()
	n.Model = ""
	if len(Note(n, p, Options{})) == 0 {
		t.Error("empty Model accepted, want rejection")
	}
}

func TestMultipleErrorsReported(t *testing.T) {
	n := validNote()
	n.Repo = "bad"
	n.Status = "finished"
	n.Title = ""
	if errs := Note(n, tidepool(), Options{}); len(errs) < 3 {
		t.Errorf("got %d errors, want at least 3: %v", len(errs), errs)
	}
}

func TestBadLinksRejected(t *testing.T) {
	p := tidepool()

	for _, id := range []string{"n-abcdef", "junk"} {
		n := validNote()
		n.Resolves = []string{id}
		if !hasField(Note(n, p, Options{}), "resolves") {
			t.Errorf("Resolves %q not rejected with Field \"resolves\"", id)
		}

		n = validNote()
		n.Supersedes = []string{id}
		if !hasField(Note(n, p, Options{}), "supersedes") {
			t.Errorf("Supersedes %q not rejected with Field \"supersedes\"", id)
		}
	}

	n := validNote()
	n.Resolves = []string{"e-abcdef"}
	if hasField(Note(n, p, Options{}), "resolves") {
		t.Error(`Resolves ["e-abcdef"] rejected, want accepted`)
	}

	n = validNote()
	n.Commits = []string{"xyz"}
	if !hasField(Note(n, p, Options{}), "commits") {
		t.Error(`Commits ["xyz"] not rejected with Field "commits"`)
	}

	n = validNote()
	n.Commits = []string{"abc1234"}
	if hasField(Note(n, p, Options{}), "commits") {
		t.Error(`Commits ["abc1234"] rejected, want accepted`)
	}

	n = validNote()
	n.At = "yesterday"
	if !hasField(Note(n, p, Options{}), "at") {
		t.Error(`At "yesterday" not rejected with Field "at"`)
	}

	n = validNote()
	n.ID = "junk"
	if !hasField(Note(n, p, Options{}), "id") {
		t.Error(`ID "junk" not rejected with Field "id"`)
	}
}

func TestAddendumRules(t *testing.T) {
	if errs := Addendum(validAddendum(), Options{}); len(errs) != 0 {
		t.Errorf("valid addendum rejected: %v", errs)
	}

	empty := model.Addendum{NoteID: "n-abcdef", Agent: "builder", Model: "claude-test", At: "2026-10-30T10:00:00Z"}
	if len(Addendum(empty, Options{})) == 0 {
		t.Error("empty addendum (no entries, status, resolves, supersedes) accepted")
	}

	a := validAddendum()
	a.NoteID = ""
	if !hasField(Addendum(a, Options{}), "note_id") {
		t.Error("missing note_id not rejected with Field \"note_id\"")
	}

	a = validAddendum()
	a.NoteID = "junk"
	if !hasField(Addendum(a, Options{}), "note_id") {
		t.Error(`note_id "junk" not rejected with Field "note_id"`)
	}
}

func TestProjectRules(t *testing.T) {
	if errs := Project(tidepool()); len(errs) != 0 {
		t.Errorf("valid project rejected: %v", errs)
	}

	for _, name := range []string{"../x", "UPPER", ""} {
		p := tidepool()
		p.Name = name
		if !hasField(Project(p), "name") {
			t.Errorf("name %q not rejected with Field \"name\"", name)
		}
	}

	p := tidepool()
	p.Mode = "cloud"
	if len(Project(p)) == 0 {
		t.Error(`mode "cloud" accepted, want rejection`)
	}

	p = tidepool()
	p.Repos = nil
	if len(Project(p)) == 0 {
		t.Error("project with no repos accepted, want rejection")
	}

	p = tidepool()
	p.Repos = []string{"a b"}
	if len(Project(p)) == 0 {
		t.Error(`repo "a b" accepted, want rejection`)
	}

	p = tidepool()
	p.TaskPattern = `^[A-Z]+-\d+$`
	if !hasField(Project(p), "task_pattern") {
		t.Error("pattern without named group base not rejected with Field \"task_pattern\"")
	}

	p = tidepool()
	p.TaskPattern = "("
	if !hasField(Project(p), "task_pattern") {
		t.Error(`pattern "(" not rejected with Field "task_pattern"`)
	}
}

func TestTaskPatternLimits(t *testing.T) {
	p := tidepool()
	p.TaskPattern = "^(?P<base>" + strings.Repeat("a", 200) + ")$"
	if !hasField(Project(p), "task_pattern") {
		t.Error("pattern longer than 200 chars not rejected with Field \"task_pattern\"")
	}

	// RE2 has no backtracking, so this must compile and match quickly.
	p = tidepool()
	p.TaskPattern = `^(?P<base>(a+)+)$`
	if hasField(Project(p), "task_pattern") {
		t.Error("catastrophic-looking but RE2-valid pattern rejected")
	}

	start := time.Now()
	_, _, ok := TaskParts(p, strings.Repeat("a", 30)+"!")
	if ok {
		t.Error("TaskParts matched 30 'a' followed by '!'")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("TaskParts took %v, want quick return", elapsed)
	}
}

func TestTaskParts(t *testing.T) {
	p := tidepool()

	base, ext, ok := TaskParts(p, "TP-03.1")
	if !ok || base != "TP-03" || ext != "1" {
		t.Errorf(`TaskParts("TP-03.1") = %q, %q, %v; want "TP-03", "1", true`, base, ext, ok)
	}

	base, ext, ok = TaskParts(p, "TP-03")
	if !ok || base != "TP-03" || ext != "" {
		t.Errorf(`TaskParts("TP-03") = %q, %q, %v; want "TP-03", "", true`, base, ext, ok)
	}

	if _, _, ok := TaskParts(p, "nope"); ok {
		t.Error(`TaskParts("nope") ok = true, want false`)
	}
}

// Invisible and direction-changing characters can hide or reorder text; they are refused.
func TestRejectsInvisibleAndBidiCharacters(t *testing.T) {
	for _, s := range []string{"a\u202eb", "a\u200bb", "a\u2066b", "a\ufeffb", "a\U000e0049b", "a\u2060b", "a\u200fb"} {
		if err := Text("f", s, 100, true); err == nil {
			t.Errorf("Text(%q) accepted, want rejection", s)
		}
	}
	for _, s := range []string{"é漢字", "emoji 🙂", "naïve café"} {
		if err := Text("f", s, 100, true); err != nil {
			t.Errorf("Text(%q) rejected: %v", s, err)
		}
	}
}

func TestEmptyNoteRefused(t *testing.T) {
	n := validNote()
	n.Entries = nil
	n.None = []model.EntryType{model.TypeChange, model.TypeDecision, model.TypeOpen, model.TypeNote, model.TypeHelper}
	if !hasField(Note(n, tidepool(), Options{}), "entries") {
		t.Fatal("a note with no entry at all must be refused")
	}
}

func TestTaskPartsRequiresFullMatchAndBase(t *testing.T) {
	p := tidepool()
	p.TaskPattern = `(?P<base>[A-Z]+-\d+)`
	if _, _, ok := TaskParts(p, "xx TP-03 yy"); ok {
		t.Error("unanchored pattern must still match the whole ID")
	}
	if _, _, ok := TaskParts(p, "TP-03"); !ok {
		t.Error("full match must work")
	}
	p.TaskPattern = `^(?P<base>[A-Z]*)-?(?P<ext>\d*)$`
	if _, _, ok := TaskParts(p, "-"); ok {
		t.Error("an empty base must be refused")
	}
}

func TestCommitErrorDoesNotEchoValue(t *testing.T) {
	n := validNote()
	secret := "ghp_" + "abcdefghijklmnopqrstuvwxyz0123456789"
	n.Commits = []string{secret}
	for _, e := range Note(n, tidepool(), Options{}) {
		if strings.Contains(e.Message, secret) {
			t.Fatalf("error echoes the value: %v", e)
		}
	}
}

func TestProjectDefaultRepo(t *testing.T) {
	p := tidepool()
	p.DefaultRepo = "client"
	if errs := Project(p); len(errs) != 0 {
		t.Errorf("default repo in repos rejected: %v", errs)
	}
	p.DefaultRepo = "other"
	if !hasField(Project(p), "default_repo") {
		t.Error("default repo outside repos must be refused with Field \"default_repo\"")
	}
	p.DefaultRepo = ""
	if errs := Project(p); len(errs) != 0 {
		t.Errorf("an empty default repo is allowed: %v", errs)
	}
}

package view

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
)

const hostile = `</script><img src=x onerror=alert(1)>"&'`

// sampleSite builds a site by hand: two notes, the second closing entries of the first through an addendum and a note.
func sampleSite() *Site {
	n1 := model.Note{
		ID: "n-aaaaaa", Agent: "builder", Repo: "server", At: "2026-10-28T10:00:00Z", Task: "TP-01", Model: "opus",
		Title: "Add the index", Status: model.StatusInProgress,
		Entries: []model.Entry{
			{ID: "e-aaaaa1", Type: model.TypeChange, Text: "added the b-tree"},
			{ID: "e-aaaaa2", Type: model.TypeDecision, Text: "use a b-tree", By: "user"},
			{ID: "e-aaaaa3", Type: model.TypeOpen, Text: "what about deletes?"},
		},
		None:    []model.EntryType{model.TypeNote, model.TypeHelper},
		Impacts: []string{"storage", "api"},
		Commits: []string{"abc1234"},
	}
	a1 := model.Addendum{ID: "a-aaaaa1", NoteID: "n-aaaaaa", Agent: "reviewer", Model: "sonnet", At: "2026-10-29T09:00:00Z",
		Status: model.StatusDone, Entries: []model.Entry{{ID: "e-aaaaa4", Type: model.TypeNote, Text: "reviewed"}},
		Resolves: []string{"e-aaaaa3"}}
	n2 := model.Note{
		ID: "n-bbbbbb", Agent: "planner", Repo: "web", At: "2026-10-30T10:00:00Z", Model: "haiku",
		Title: "Switch to hash", Status: model.StatusBlocked,
		Entries: []model.Entry{
			{ID: "e-bbbbb1", Type: model.TypeDecision, Text: "use a hash", By: "agent"},
			{ID: "e-bbbbb2", Type: model.TypeHelper, Text: "run make bench"},
		},
		None:       []model.EntryType{model.TypeChange, model.TypeOpen, model.TypeNote},
		Supersedes: []string{"e-aaaaa2"},
	}
	info := func(e model.Entry, note, add, task, repo, at string, closedBy string) store.EntryInfo {
		return store.EntryInfo{Entry: e, NoteID: note, AddendumID: add, Task: task, Repo: repo, At: at,
			Closed: closedBy != "", ClosedBy: closedBy}
	}
	i1 := info(n1.Entries[0], n1.ID, "", "TP-01", "server", n1.At, "")
	i2 := info(n1.Entries[1], n1.ID, "", "TP-01", "server", n1.At, "n-bbbbbb")
	i3 := info(n1.Entries[2], n1.ID, "", "TP-01", "server", n1.At, "a-aaaaa1")
	i4 := info(a1.Entries[0], n1.ID, a1.ID, "TP-01", "server", a1.At, "")
	j1 := info(n2.Entries[0], n2.ID, "", "", "web", n2.At, "")
	j2 := info(n2.Entries[1], n2.ID, "", "", "web", n2.At, "")
	return &Site{
		Project:        model.Project{Name: "tidepool", Mode: model.ModeProject, Context: "Tide data service."},
		Generated:      "2026-10-31T00:00:00Z",
		MarkdownHeader: "# tidepool\n\nTide data service.\n\n",
		Notes: []NoteView{
			{Header: store.NoteHeader{ID: n2.ID, Repo: "web", Agent: "planner", Model: "haiku", At: n2.At, Title: n2.Title, Status: model.StatusBlocked},
				Written: model.StatusBlocked, Note: n2, OpenPoints: 0,
				States:   map[string]store.EntryInfo{j1.ID: j1, j2.ID: j2},
				Markdown: "## n-bbbbbb Switch to hash\n\n"},
			{Header: store.NoteHeader{ID: n1.ID, Task: "TP-01", Repo: "server", Agent: "builder", Model: "opus", At: n1.At, Title: n1.Title, Status: model.StatusDone},
				Written: model.StatusInProgress, Note: n1, Addenda: []model.Addendum{a1}, OpenPoints: 0,
				States:   map[string]store.EntryInfo{i1.ID: i1, i2.ID: i2, i3.ID: i3, i4.ID: i4},
				Markdown: "## n-aaaaaa Add the index\n\n"},
		},
		Entries: []EntryRow{
			{EntryInfo: j1, Agent: "planner"}, {EntryInfo: j2, Agent: "planner"},
			{EntryInfo: i4, Agent: "reviewer"},
			{EntryInfo: i1, Agent: "builder"}, {EntryInfo: i2, Agent: "builder"}, {EntryInfo: i3, Agent: "builder"},
		},
	}
}

var siteFiles = []string{
	".gitignore", "index.html", "decisions.html", "open-points.html", "helpers.html",
	"notes/n-aaaaaa.html", "notes/n-aaaaaa.md", "notes/n-bbbbbb.html", "notes/n-bbbbbb.md",
	"assets/app.css", "assets/app.js", "data.js",
}

func writeSite(t *testing.T, s *Site) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "view")
	if err := Write(dir, s); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestWriteCreatesFiles(t *testing.T) {
	dir := writeSite(t, sampleSite())
	for _, f := range siteFiles {
		fi, err := os.Lstat(filepath.Join(dir, f))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if !fi.Mode().IsRegular() || fi.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode %v", f, fi.Mode())
		}
	}
	for _, d := range []string{"", "notes", "assets"} {
		fi, err := os.Lstat(filepath.Join(dir, d))
		if err != nil {
			t.Fatal(err)
		}
		if !fi.IsDir() || fi.Mode().Perm() != 0o700 {
			t.Fatalf("%q mode %v", d, fi.Mode())
		}
	}
	if got := readFile(t, dir, ".gitignore"); got != "*\n" {
		t.Fatalf(".gitignore %q", got)
	}
}

func snapshot(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.Mode().IsRegular() {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(dir, p)
			out[rel] = b
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestWriteIdempotent(t *testing.T) {
	s := sampleSite()
	dir := writeSite(t, s)
	first := snapshot(t, dir)
	// a pre-existing file with loose permissions is tightened on rewrite
	if err := os.Chmod(filepath.Join(dir, "index.html"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir, s); err != nil {
		t.Fatal(err)
	}
	second := snapshot(t, dir)
	if len(first) != len(second) {
		t.Fatalf("files %d then %d", len(first), len(second))
	}
	for k, v := range first {
		if !bytes.Equal(v, second[k]) {
			t.Fatalf("%s changed", k)
		}
	}
	fi, _ := os.Stat(filepath.Join(dir, "index.html"))
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode())
	}
}

func TestWriteRefusesSymlinkDir(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "view")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	err := Write(link, sampleSite())
	if err == nil || !strings.Contains(err.Error(), link) {
		t.Fatalf("err %v", err)
	}
	ents, _ := os.ReadDir(target)
	if len(ents) != 0 {
		t.Fatalf("target touched: %d entries", len(ents))
	}
}

func TestWriteRefusesSymlinkFile(t *testing.T) {
	dir := writeSite(t, sampleSite())
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	idx := filepath.Join(dir, "index.html")
	os.Remove(idx)
	if err := os.Symlink(outside, idx); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir, sampleSite()); err == nil || !strings.Contains(err.Error(), idx) {
		t.Fatalf("err %v", err)
	}
	if b, _ := os.ReadFile(outside); string(b) != "keep" {
		t.Fatalf("outside changed: %q", b)
	}
}

func TestWriteRefusesRegularFileDir(t *testing.T) {
	p := filepath.Join(t.TempDir(), "view")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Write(p, sampleSite())
	if err == nil || !strings.Contains(err.Error(), p) {
		t.Fatalf("err %v", err)
	}
}

func TestWriteRequiresParent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "missing", "view")
	if err := Write(p, sampleSite()); err == nil {
		t.Fatal("no error")
	}
}

func TestWriteRefusesBadNoteID(t *testing.T) {
	s := sampleSite()
	s.Notes[0].Header.ID = "../evil"
	if err := Write(filepath.Join(t.TempDir(), "view"), s); err == nil {
		t.Fatal("no error")
	}
}

func TestWriteEscapesHostileText(t *testing.T) {
	s := sampleSite()
	s.Notes[0].Header.Title = hostile
	s.Notes[0].Note.Title = hostile
	s.Notes[0].Header.Agent = hostile
	s.Notes[0].Note.Agent = hostile
	s.Notes[0].Note.Entries[0].Text = hostile
	s.Notes[0].Markdown = hostile
	s.MarkdownHeader = hostile
	for i := range s.Entries {
		if s.Entries[i].ID == "e-bbbbb1" {
			s.Entries[i].Text = hostile
			s.Entries[i].Agent = hostile
		}
	}
	dir := writeSite(t, s)
	escaped := "&lt;/script&gt;&lt;img src=x onerror=alert(1)&gt;&#34;&amp;&#39;"
	for _, f := range []string{"index.html", "decisions.html", "notes/n-bbbbbb.html"} {
		page := readFile(t, dir, f)
		if strings.Contains(page, hostile) || strings.Contains(page, "<img") {
			t.Fatalf("%s contains raw hostile text", f)
		}
		if !strings.Contains(page, escaped) {
			t.Fatalf("%s lacks escaped text", f)
		}
	}
	data := readFile(t, dir, "data.js")
	if strings.Contains(strings.ToLower(data), "</script") || strings.Contains(data, "<") {
		t.Fatalf("data.js not escaped: %s", data)
	}
}

func TestWriteEmptySite(t *testing.T) {
	s := &Site{Project: model.Project{Name: "empty"}, Generated: "2026-10-31T00:00:00Z", MarkdownHeader: "# empty\n\n",
		Notes: []NoteView{}, Entries: []EntryRow{}}
	dir := writeSite(t, s)
	want := map[string]string{
		"index.html": "No notes yet.", "decisions.html": "No decisions yet.",
		"open-points.html": "No open points yet.", "helpers.html": "No helpers yet.",
	}
	for f, msg := range want {
		if page := readFile(t, dir, f); !strings.Contains(page, msg) {
			t.Fatalf("%s lacks %q", f, msg)
		}
	}
	if !strings.Contains(readFile(t, dir, "index.html"), "No matching notes.") {
		t.Fatal("no matching message missing")
	}
}

func TestWriteAllClosedEntriesAndAllNone(t *testing.T) {
	s := sampleSite()
	// keep only the closed decision
	s.Entries = []EntryRow{s.Entries[4]}
	nv := &s.Notes[0]
	nv.Note.Entries = nil
	nv.Note.None = []model.EntryType{model.TypeChange, model.TypeDecision, model.TypeOpen, model.TypeNote, model.TypeHelper}
	dir := writeSite(t, s)
	page := readFile(t, dir, "decisions.html")
	if !strings.Contains(page, "use a b-tree") || !strings.Contains(page, `data-closed="true"`) {
		t.Fatal("closed decision not listed")
	}
	if !strings.Contains(page, `id="show-closed"`) || !strings.Contains(page, "Show closed") {
		t.Fatal("show closed control missing")
	}
	note := readFile(t, dir, "notes/n-bbbbbb.html")
	if got := strings.Count(note, `class="none"`); got != 5 {
		t.Fatalf("none count %d", got)
	}
}

func TestNotePageContents(t *testing.T) {
	dir := writeSite(t, sampleSite())
	page := readFile(t, dir, "notes/n-aaaaaa.html")
	for _, want := range []string{"builder", "opus", "server", "storage", "api", "TP-01", "2026-10-28",
		"abc1234", "status-done", "Written status", "in_progress", "← All notes", "reviewer", "reviewed"} {
		if !strings.Contains(page, want) {
			t.Fatalf("note page lacks %q", want)
		}
	}
	last := -1
	for _, h := range []string{"<h2>Changes</h2>", "<h2>Decisions</h2>", "<h2>Open points</h2>", "<h2>Notes</h2>", "<h2>Helpers</h2>", "<h2>Addenda</h2>"} {
		i := strings.Index(page, h)
		if i < 0 || i < last {
			t.Fatalf("heading %s at %d after %d", h, i, last)
		}
		last = i
	}
	// superseded decision struck and linking to the closing note; resolved open point linking to the addendum
	if !regexp.MustCompile(`<s>use a b-tree</s>`).MatchString(page) {
		t.Fatal("decision not struck")
	}
	if !strings.Contains(page, `href="n-bbbbbb.html"`) || !strings.Contains(page, `href="n-aaaaaa.html#a-aaaaa1"`) {
		t.Fatal("closer links missing")
	}
	if !regexp.MustCompile(`<a [^>]*href="n-aaaaaa\.md"[^>]*download`).MatchString(page) {
		t.Fatal("download link missing")
	}
	// written status is shown only when it differs
	other := readFile(t, dir, "notes/n-bbbbbb.html")
	if strings.Contains(other, "Written status") {
		t.Fatal("written status shown though equal")
	}
}

func TestNotesTableRowAttributes(t *testing.T) {
	dir := writeSite(t, sampleSite())
	page := readFile(t, dir, "index.html")
	row := regexp.MustCompile(`(?s)<tr [^>]*data-id="n-bbbbbb"[^>]*>.*?</tr>`).FindString(page)
	if row == "" {
		t.Fatal("row missing")
	}
	for _, want := range []string{`data-status="blocked"`, `data-task=""`, `data-repo="web"`, `data-agent="planner"`,
		`data-at="2026-10-30T10:00:00Z"`, `data-text="`, `<td>-</td>`, `href="notes/n-bbbbbb.html"`} {
		if !strings.Contains(row, want) {
			t.Fatalf("row lacks %q: %s", want, row)
		}
	}
	if !strings.Contains(page, `data-task="TP-01"`) {
		t.Fatal("task attribute missing")
	}
}

func TestNoteMarkdownFileIsHeaderPlusNote(t *testing.T) {
	s := sampleSite()
	dir := writeSite(t, s)
	if got := readFile(t, dir, "notes/n-aaaaaa.md"); got != s.MarkdownHeader+s.Notes[1].Markdown {
		t.Fatalf("md %q", got)
	}
}

func TestDataJSCarriesAgentAndMarkdown(t *testing.T) {
	s := sampleSite()
	dir := writeSite(t, s)
	data := readFile(t, dir, "data.js")
	const pre = "window.CAIRN = "
	if !strings.HasPrefix(data, pre) || !strings.HasSuffix(data, ";\n") {
		t.Fatalf("data.js shape: %q", data)
	}
	var got struct {
		Header  string            `json:"header"`
		Notes   map[string]string `json:"notes"`
		Entries map[string]struct {
			Text, By, Note, Agent string
		} `json:"entries"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimPrefix(data, pre), ";\n")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Header != s.MarkdownHeader || got.Notes["n-aaaaaa"] != s.Notes[1].Markdown {
		t.Fatalf("data %+v", got)
	}
	e := got.Entries["e-aaaaa4"]
	if e.Text != "reviewed" || e.Note != "n-aaaaaa" || e.Agent != "reviewer" {
		t.Fatalf("entry %+v", e)
	}
	if got.Entries["e-aaaaa2"].By != "user" {
		t.Fatalf("by %+v", got.Entries["e-aaaaa2"])
	}
}

func TestPagesAreEnglishAndSelfContained(t *testing.T) {
	dir := writeSite(t, sampleSite())
	inline := regexp.MustCompile(`<script(\s[^>]*)?>`)
	for name, b := range snapshot(t, dir) {
		body := string(b)
		if strings.Contains(body, "http://") || strings.Contains(body, "https://") {
			t.Fatalf("%s has a URL", name)
		}
		if strings.HasSuffix(name, ".html") {
			for _, tag := range inline.FindAllString(body, -1) {
				if !strings.Contains(tag, " src=") {
					t.Fatalf("%s has inline script %s", name, tag)
				}
			}
			if strings.Contains(body, " style=") || strings.Contains(body, "<style") {
				t.Fatalf("%s has inline style", name)
			}
			if !strings.Contains(body, "read-only") || !strings.Contains(body, "source of truth") ||
				!strings.Contains(body, "2026-10-31T00:00:00Z") {
				t.Fatalf("%s lacks the banner", name)
			}
		}
	}
}

// TestAppJSLogic runs the pure functions of app.js through node, when node is installed.
func TestAppJSLogic(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed")
	}
	script := `
const assert = require("assert");
const app = require(process.argv[1]);
const any = {text: "", status: "", task: "*", repo: "*", agent: "*", from: "", to: "", showClosed: false};
const row = {status: "done", task: "", repo: "web", agent: "planner", at: "2026-10-30T10:00:00Z", text: "n-1 switch to hash", closed: false};
assert.ok(app.matches(row, any));
assert.ok(app.matches(row, {...any, task: ""}));
assert.ok(!app.matches(row, {...any, task: "TP-01"}));
assert.ok(!app.matches(row, {...any, status: "blocked"}));
assert.ok(app.matches(row, {...any, status: "done", repo: "web", agent: "planner"}));
assert.ok(!app.matches(row, {...any, agent: "builder"}));
assert.ok(app.matches(row, {...any, from: "2026-10-30", to: "2026-10-30"}));
assert.ok(!app.matches(row, {...any, from: "2026-10-31"}));
assert.ok(!app.matches(row, {...any, to: "2026-10-29"}));
assert.ok(app.matches(row, {...any, text: "  HASH switch "}));
assert.ok(!app.matches(row, {...any, text: "hash tree"}));
assert.ok(!app.matches({...row, closed: true}, any));
assert.ok(app.matches({...row, closed: true}, {...any, showClosed: true}));
const data = {header: "# p\n\n", notes: {"n-1": "## one\n\n", "n-2": "## two\n\n"},
  entries: {"e-1": {text: "use\na hash", by: "user", note: "n-1", agent: "a"}, "e-2": {text: "run it", by: "", note: "n-2", agent: "b"}}};
assert.strictEqual(app.notesMarkdown(data, ["n-2", "n-1"]), "# p\n\n## two\n\n## one\n\n");
assert.strictEqual(app.notesMarkdown(data, []), "# p\n\n");
assert.strictEqual(app.entriesMarkdown(data, ["e-1", "e-2"]),
  "- ` + "`e-1`" + ` [user] use a hash (note n-1)\n- ` + "`e-2`" + ` run it (note n-2)\n");
assert.ok(app.compare("2", "10", true) < 0);
assert.ok(app.compare("b", "A", false) > 0);
`
	abs, err := filepath.Abs(filepath.Join("assets", "app.js"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, "-e", script, abs).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
}

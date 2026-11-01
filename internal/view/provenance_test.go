package view

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
)

// originSite is the sample site where n-aaaaaa (builder) is on feat/x by Ada, its
// addendum a-aaaaa1 on review/y by Bob, and n-bbbbbb on main by Cy.
func originSite() *Site {
	s := sampleSite()
	b, a := &s.Notes[0], &s.Notes[1] // n-bbbbbb, n-aaaaaa
	b.Note.Branch, b.Note.AuthorName, b.Note.AuthorEmail = "main", "Cy", "cy@example.com"
	a.Note.Branch, a.Note.AuthorName, a.Note.AuthorEmail = "feat/x", "Ada", "ada@example.com"
	a.Note.Ref = "https://tracker.example/TP-01"
	a.Addenda[0].Branch, a.Addenda[0].AuthorName, a.Addenda[0].AuthorEmail = "review/y", "Bob", "bob@example.com"
	a.Addenda[0].Ref = "https://tracker.example/review/1"
	for i := range s.Entries {
		e := &s.Entries[i]
		switch {
		case e.AddendumID == "a-aaaaa1":
			e.Branch, e.AuthorName, e.AuthorEmail = "review/y", "Bob", "bob@example.com"
		case e.NoteID == "n-aaaaaa":
			e.Branch, e.AuthorName, e.AuthorEmail = "feat/x", "Ada", "ada@example.com"
		default:
			e.Branch, e.AuthorName, e.AuthorEmail = "main", "Cy", "cy@example.com"
		}
	}
	return s
}

func rowOf(t *testing.T, page, id string) string {
	t.Helper()
	row := regexp.MustCompile(`(?s)<tr [^>]*data-id="` + id + `"[^>]*>.*?</tr>`).FindString(page)
	if row == "" {
		t.Fatalf("row %s missing", id)
	}
	return row
}

func TestSiteHasBranchAndAuthorColumns(t *testing.T) {
	dir := writeSite(t, originSite())
	page := readFile(t, dir, "index.html")
	for _, want := range []string{
		`<th scope="col" data-sort="branch"><button type="button">Branch</button></th>`,
		`<th scope="col" data-sort="author"><button type="button">Author</button></th>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("index.html lacks %s", want)
		}
	}
	row := rowOf(t, page, "n-aaaaaa")
	// the cell shows the note's own value, then the addendum's other values
	for _, want := range []string{`data-branch="feat/x"`, `data-author="Ada"`,
		`data-branches="feat/x` + "\n" + `review/y"`, `data-authors="Ada` + "\n" + `ada@example.com` + "\n" + `Bob` + "\n" + `bob@example.com"`,
		`<td>feat/x <span class="more">+ review/y</span></td>`, `<td>Ada <span class="more">+ Bob</span></td>`} {
		if !strings.Contains(row, want) {
			t.Errorf("row lacks %q: %s", want, row)
		}
	}
	other := rowOf(t, page, "n-bbbbbb")
	if !strings.Contains(other, "<td>main</td>") || !strings.Contains(other, "<td>Cy</td>") || strings.Contains(other, "more") {
		t.Errorf("single-origin row: %s", other)
	}
}

// Every header the page script sorts by carries data-sort, and the script reads
// the matching row attribute.
func TestSortAttributesPresent(t *testing.T) {
	dir := writeSite(t, originSite())
	page := readFile(t, dir, "index.html")
	for _, key := range []string{"branch", "author"} {
		if !strings.Contains(page, `data-sort="`+key+`"`) || !strings.Contains(page, ` data-`+key+`="`) {
			t.Errorf("sort key %s not on header and rows", key)
		}
	}
	css := readFile(t, dir, "assets/app.css")
	if !strings.Contains(css, `th[aria-sort="ascending"] button::after`) || !strings.Contains(css, `th[aria-sort="descending"] button::after`) {
		t.Error("sort indicator missing from the stylesheet")
	}
}

// A filter offers only what the rows of its view hold: the Notes view lists the
// addendum's branch and author as the note's own; the open-points view lists only
// the note holding the open point.
func TestBranchFilterOptionsOnlyFromListedRows(t *testing.T) {
	dir := writeSite(t, originSite())
	cases := []struct{ file, sel, want string }{
		{"index.html", "f-branch", "*|Any, feat/x|feat/x, main|main, review/y|review/y"},
		{"index.html", "f-author", "*|Any, Ada|Ada, Bob|Bob, Cy|Cy"},
		{"open-points.html", "f-branch", "*|Any, feat/x|feat/x"},
		{"open-points.html", "f-author", "*|Any, Ada|Ada"},
		{"helpers.html", "f-branch", "*|Any, main|main"},
	}
	for _, c := range cases {
		got := strings.Join(selectOptions(t, readFile(t, dir, c.file), c.sel), ", ")
		if got != c.want {
			t.Errorf("%s %s:\n got %s\nwant %s", c.file, c.sel, got, c.want)
		}
	}
}

// An entry written in an addendum carries the addendum's branch and author, not the note's.
func TestEntryViewUsesHolderBranch(t *testing.T) {
	dir := writeSite(t, originSite())
	dec := readFile(t, dir, "decisions.html")
	if row := rowOf(t, dec, "e-aaaaa2"); !strings.Contains(row, `data-branches="feat/x"`) || !strings.Contains(row, `data-authors="Ada`+"\n"+`ada@example.com"`) {
		t.Errorf("note entry: %s", row)
	}
	s := originSite()
	s.Entries = append(s.Entries, EntryRow{EntryInfo: store.EntryInfo{Entry: model.Entry{ID: "e-aaaaa9", Type: model.TypeDecision, Text: "late"},
		NoteID: "n-aaaaaa", AddendumID: "a-aaaaa1", Repo: "server", At: "2026-10-29T09:00:00Z"},
		Agent: "reviewer", Branch: "review/y", AuthorName: "Bob", AuthorEmail: "bob@example.com"})
	dec = readFile(t, writeSite(t, s), "decisions.html")
	if row := rowOf(t, dec, "e-aaaaa9"); !strings.Contains(row, `data-branches="review/y"`) || !strings.Contains(row, `data-authors="Bob`+"\n"+`bob@example.com"`) {
		t.Errorf("addendum entry: %s", row)
	}
}

func TestNoteCardShowsOriginAndRefs(t *testing.T) {
	page := readFile(t, writeSite(t, originSite()), "notes/n-aaaaaa.html")
	for _, want := range []string{
		`<dt>Author</dt><dd>Ada &lt;ada@example.com&gt;</dd>`, `<dt>Branch</dt><dd>feat/x</dd>`,
		`<a href="https://tracker.example/TP-01" rel="noopener noreferrer">https://tracker.example/TP-01</a>`,
		`Author Bob &lt;bob@example.com&gt;, branch review/y, ref <a href="https://tracker.example/review/1" rel="noopener noreferrer">`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("note page lacks %q", want)
		}
	}
}

// Validation refuses a ref that is not http or https, so none is stored; should one
// reach the template anyway it is plain escaped text, never a link.
func TestRefRenderedAsHTTPLinkOnly(t *testing.T) {
	s := originSite()
	s.Notes[1].Note.Ref = `javascript:alert("x")<b>`
	s.Notes[1].TaskRefs = []string{`HTTPS://tracker.example/a?x=1&y=<2>`}
	page := readFile(t, writeSite(t, s), "notes/n-aaaaaa.html")
	if strings.Contains(page, `href="javascript`) || strings.Contains(page, "<b>") {
		t.Errorf("unsafe ref became markup: %s", page)
	}
	if !strings.Contains(page, `javascript:alert(&#34;x&#34;)&lt;b&gt;`) {
		t.Error("ref text not escaped")
	}
	if !strings.Contains(page, `rel="noopener noreferrer"`) || !strings.Contains(page, `HTTPS://tracker.example/a?x=1&amp;y=%3c2%3e`) {
		t.Errorf("https ref not linked: %s", page)
	}
}

func TestComputedTaskLinkOnlyWithoutStoredRef(t *testing.T) {
	s := originSite()
	s.Project.TaskURL = "https://tracker.example/browse/{task}"
	s.Notes[1].Note.Ref = ""
	s.Notes[1].Addenda[0].Ref = ""
	page := readFile(t, writeSite(t, s), "notes/n-aaaaaa.html")
	if !strings.Contains(page, `href="https://tracker.example/browse/TP-01"`) {
		t.Error("computed task link missing")
	}
	s.Notes[1].TaskRefs = []string{"https://other.example/1"}
	page = readFile(t, writeSite(t, s), "notes/n-aaaaaa.html")
	if strings.Contains(page, "browse/TP-01") || !strings.Contains(page, "https://other.example/1") {
		t.Error("computed link shown despite a stored ref")
	}
	// no task: no link
	if strings.Contains(readFile(t, writeSite(t, s), "notes/n-bbbbbb.html"), "browse/") {
		t.Error("link for a note without task")
	}
}

func TestSearchTextIncludesOrigin(t *testing.T) {
	s := originSite()
	got := noteSearchText(s.Notes[1])
	for _, w := range []string{"feat/x", "ada", "ada@example.com", "bob", "review/y", "https://tracker.example/review/1"} {
		if !strings.Contains(got, w) {
			t.Errorf("search text lacks %q", w)
		}
	}
}

// A project from before the change has no branch, author or ref: nothing is offered,
// cells are dashes and cards show dashes.
func TestSiteWithoutProvenance(t *testing.T) {
	dir := writeSite(t, sampleSite())
	index := readFile(t, dir, "index.html")
	for _, sel := range []string{"f-branch", "f-author"} {
		if got := strings.Join(selectOptions(t, index, sel), ", "); got != "*|Any" {
			t.Errorf("%s offers %s", sel, got)
		}
	}
	if row := rowOf(t, index, "n-bbbbbb"); !strings.Contains(row, `data-branches=""`) || !strings.Contains(row, "<td>-</td>\n<td>-</td>") {
		t.Errorf("row: %s", row)
	}
	note := readFile(t, dir, "notes/n-aaaaaa.html")
	if !strings.Contains(note, `<dt>Branch</dt><dd>-</dd>`) || !strings.Contains(note, `<dt>Refs</dt><dd>-</dd>`) || strings.Contains(note, `class="origin"`) {
		t.Error("note card without provenance")
	}
}

// The task-filter selects keep their shape; the Notes view's new rows must not break TestAppJSLogic's cases.
func TestAppJSBranchAndAuthorFilters(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed")
	}
	script := `
const assert = require("assert");
const app = require(process.argv[1]);
const any = {text: "", status: "", task: "*", repo: "*", agent: "*", branch: "*", author: "*", from: "", to: "", showClosed: false};
const row = {status: "done", task: "", repo: "web", agent: "p", at: "2026-10-30T10:00:00Z", text: "x", closed: false,
  branches: ["feat/x", "review/y"], authors: ["Ada", "ada@example.com", "Bob"]};
assert.ok(app.matches(row, any));
assert.ok(app.matches(row, {...any, branch: "feat/x"}));
assert.ok(app.matches(row, {...any, branch: "review/y"}));
assert.ok(!app.matches(row, {...any, branch: "main"}));
assert.ok(app.matches(row, {...any, author: "Bob"}));
assert.ok(app.matches(row, {...any, author: "ada@example.com"}));
assert.ok(!app.matches(row, {...any, author: "Cy"}));
assert.ok(!app.matches({...row, branches: [], authors: []}, {...any, branch: "main"}));
assert.ok(app.matches({...row, branches: [], authors: []}, any));
assert.ok(app.matches(row, {...any, branch: "feat/x", author: "Ada", repo: "web"}));
assert.ok(!app.matches(row, {...any, branch: "feat/x", author: "Cy"}));
// rows from an older page have no such fields
assert.ok(app.matches({status: "", task: "", repo: "", agent: "", at: "", text: "", closed: false}, any));
`
	abs, err := filepath.Abs(filepath.Join("assets", "app.js"))
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, "-e", script, abs).CombinedOutput(); err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
}

package view

import (
	"errors"
	"strings"
	"testing"

	"github.com/R0b-1ns/cairn/internal/model"
)

func childProject() model.Project {
	p := testProject()
	p.Name, p.Repos = "web-app", []string{"web"}
	return p
}

func TestCollectAllMergesNewestFirstAndLabelsChildren(t *testing.T) {
	parent, child := newStore(t), newStore(t)
	p1 := insert(t, parent, note("2026-10-28T10:00:00Z", "parent old", model.Entry{Type: model.TypeDecision, Text: "d-parent", By: "user"}))
	c1 := insert(t, child, note("2026-10-29T10:00:00Z", "child mid", model.Entry{Type: model.TypeDecision, Text: "d-child", By: "user"}))
	p2 := insert(t, parent, note("2026-10-30T10:00:00Z", "parent new"))
	site, err := CollectAll([]Source{{St: parent, Proj: testProject()}, {Child: "web", St: child, Proj: childProject()}}, "g")
	if err != nil {
		t.Fatal(err)
	}
	if site.Project.Name != "tidepool" {
		t.Errorf("project %q", site.Project.Name)
	}
	var ids, kids []string
	for _, n := range site.Notes {
		ids, kids = append(ids, n.Header.ID), append(kids, n.Child)
	}
	if strings.Join(ids, ",") != p2.ID+","+c1.ID+","+p1.ID || strings.Join(kids, ",") != ",web," {
		t.Errorf("notes %v children %v", ids, kids)
	}
	var texts []string
	for _, e := range site.Entries {
		texts = append(texts, e.Text+"/"+e.Child)
	}
	if strings.Join(texts, ",") != "d-child/web,d-parent/" {
		t.Errorf("entries %v", texts)
	}
}

func TestCollectAllSingleSourceEqualsCollect(t *testing.T) {
	s := newStore(t)
	insert(t, s, note("2026-10-28T10:00:00Z", "a", model.Entry{Type: model.TypeDecision, Text: "d"}))
	insert(t, s, note("2026-10-29T10:00:00Z", "b"))
	one := collect(t, s)
	all, err := CollectAll([]Source{{St: s, Proj: testProject()}}, "2026-10-31T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(one.Notes) != len(all.Notes) || one.Notes[0].Header.ID != all.Notes[0].Header.ID || len(one.Entries) != len(all.Entries) {
		t.Errorf("differs: %+v vs %+v", one, all)
	}
}

func TestCollectAllDuplicateNoteID(t *testing.T) {
	s := newStore(t)
	n := insert(t, s, note("2026-10-28T10:00:00Z", "a"))
	_, err := CollectAll([]Source{{St: s, Proj: testProject()}, {Child: "web", St: s, Proj: childProject()}}, "g")
	var d *DuplicateNoteError
	if !errors.As(err, &d) || d.ID != n.ID || d.First != "" || d.Second != "web" {
		t.Fatalf("err %v", err)
	}
}

func childSite() *Site {
	s := sampleSite()
	s.Notes[0].Child = "web"
	for i := range s.Entries {
		if s.Entries[i].NoteID == "n-bbbbbb" {
			s.Entries[i].Child = "web"
		}
	}
	return s
}

func TestWriteShowsChildColumnAndFilterOnlyWithChildren(t *testing.T) {
	plain := writeSite(t, sampleSite())
	for _, f := range []string{"index.html", "decisions.html", "open-points.html", "helpers.html", "data.js"} {
		if b := readFile(t, plain, f); strings.Contains(strings.ToLower(b), "child") {
			t.Errorf("%s mentions child without children", f)
		}
	}
	dir := writeSite(t, childSite())
	idx := readFile(t, dir, "index.html")
	for _, want := range []string{`data-sort="child"`, `id="f-child"`, `data-child="web"`, `<td>web</td>`} {
		if !strings.Contains(idx, want) {
			t.Errorf("index.html lacks %q", want)
		}
	}
	if got := selectOptions(t, idx, "f-child"); strings.Join(got, "|") != "*|" && !strings.Contains(strings.Join(got, "|"), "web") {
		t.Errorf("child options %v", got)
	}
	dec := readFile(t, dir, "decisions.html")
	for _, want := range []string{`data-sort="child"`, `id="f-child"`, `data-child="web"`} {
		if !strings.Contains(dec, want) {
			t.Errorf("decisions.html lacks %q", want)
		}
	}
	if !strings.Contains(readFile(t, dir, "assets/app.js"), "child") {
		t.Errorf("app.js does not filter by child")
	}
}

func TestWriteEscapesHostileChildName(t *testing.T) {
	s := childSite()
	s.Notes[0].Child = hostile
	for i := range s.Entries {
		if s.Entries[i].Child != "" {
			s.Entries[i].Child = hostile
		}
	}
	dir := writeSite(t, s)
	escaped := "&lt;/script&gt;&lt;img src=x onerror=alert(1)&gt;&#34;&amp;&#39;"
	for _, f := range []string{"index.html", "decisions.html", "helpers.html", "notes/n-bbbbbb.html"} {
		page := readFile(t, dir, f)
		if strings.Contains(page, hostile) || strings.Contains(page, "<img") {
			t.Fatalf("%s contains the raw child name", f)
		}
		if f != "notes/n-bbbbbb.html" && !strings.Contains(page, escaped) {
			t.Errorf("%s lacks the escaped child name", f)
		}
	}
	data := readFile(t, dir, "data.js")
	if strings.Contains(data, "<") || strings.Contains(strings.ToLower(data), "</script") {
		t.Fatalf("data.js not escaped: %s", data)
	}
}

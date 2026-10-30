package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// secondProject returns a fresh machine with an empty tidepool project.
func secondProject(t *testing.T) *testEnv {
	t.Helper()
	b := newEnv(t)
	b.initProject()
	return b
}

func listJSON(t *testing.T, e *testEnv) string {
	t.Helper()
	return e.mustRun("list", "--all", "--json", "--max", "500").out
}

func TestExportJSONRoundTrip(t *testing.T) {
	a, f := newBriefFixture(t)
	doc := a.mustRun("export", "--format", "json").out
	b := secondProject(t)
	r := b.runIn(b.root, doc, "import", "-")
	if r.code != ExitOK || !strings.Contains(r.out, "imported 4 notes") {
		t.Fatalf("import: %+v", r)
	}
	if listJSON(t, a) != listJSON(t, b) {
		t.Fatal("entries differ after round trip")
	}
	if a.mustRun("show", f.note1).out != b.mustRun("show", f.note1).out {
		t.Fatal("show differs after round trip")
	}
	if got := b.mustRun("brief", "TP-03").out; got != a.mustRun("brief", "TP-03").out {
		t.Fatalf("brief differs:\n%s", got)
	}
}

func TestExportMarkdownReadable(t *testing.T) {
	e, f := newBriefFixture(t)
	out := e.mustRun("export").out // markdown is the default
	for _, want := range []string{"# tidepool", f.note1, "Work on TP-03", "Added the tidal index.", "Use a b-tree.",
		"superseded by", "resolved by", "Client reads the index."} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown missing %q:\n%s", want, out)
		}
	}
	if r := e.run("export", "--format", "xml"); r.code != ExitUsage {
		t.Fatalf("bad format: %+v", r)
	}
}

func TestExportTaskFilter(t *testing.T) {
	e, f := newBriefFixture(t)
	out := e.mustRun("export", "--format", "json", "--task", "TP-04").out
	if !strings.Contains(out, f.other) || strings.Contains(out, f.note1) {
		t.Fatalf("task filter:\n%s", out)
	}
	if !strings.Contains(e.mustRun("export", "--format", "json", "--task", "TP-03").out, f.note2) {
		t.Fatal("base task must include extensions")
	}
}

func TestExportOutNoOverwrite(t *testing.T) {
	e, _ := newBriefFixture(t)
	dir := realTemp(t)
	path := filepath.Join(dir, "notes.json")
	e.mustRun("export", "--format", "json", "--out", path)
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("export file: %v %v", fi, err)
	}
	if r := e.run("export", "--format", "json", "--out", path); r.code != ExitRuntime {
		t.Fatalf("overwrite must be refused: %+v", r)
	}
	e.mustRun("export", "--format", "json", "--out", path, "--force")
	link := filepath.Join(dir, "link.json")
	os.Symlink(path, link)
	if r := e.run("export", "--format", "json", "--out", link, "--force"); r.code != ExitRuntime {
		t.Fatalf("symlink target must be refused: %+v", r)
	}
}

func TestImportRunsValidation(t *testing.T) {
	a, _ := newBriefFixture(t)
	doc := a.mustRun("export", "--format", "json").out
	bad := strings.Replace(doc, `"repo": "server"`, `"repo": "nope"`, 1)
	bad = strings.Replace(bad, `"type": "change"`, `"type": "weird"`, 1)
	b := secondProject(t)
	r := b.runIn(b.root, bad, "import", "-")
	if r.code != ExitValidation || !strings.Contains(r.err, "notes[0].repo") || !strings.Contains(r.err, "type") {
		t.Fatalf("%+v", r)
	}
	if countNotes(t, b) != 0 {
		t.Fatal("nothing may be imported when validation fails")
	}
}

func TestImportConflictAborts(t *testing.T) {
	a, f := newBriefFixture(t)
	doc := a.mustRun("export", "--format", "json").out
	b := secondProject(t)
	b.runIn(b.root, doc, "import", "-")
	tampered := strings.Replace(doc, "Work on TP-03", "Rewritten history", 1)
	r := b.runIn(b.root, tampered, "import", "-")
	if r.code != ExitRuntime || !strings.Contains(r.err, "different content") {
		t.Fatalf("%+v", r)
	}
	if n := storeNote(t, b, f.note1); n.Title != "Work on TP-03" {
		t.Fatalf("immutability broken by import: %q", n.Title)
	}
}

func TestImportIdempotent(t *testing.T) {
	a, _ := newBriefFixture(t)
	doc := a.mustRun("export", "--format", "json").out
	b := secondProject(t)
	b.runIn(b.root, doc, "import", "-")
	r := b.runIn(b.root, doc, "import", "-")
	if r.code != ExitOK || !strings.Contains(r.out, "imported 0 notes") || !strings.Contains(r.out, "already present") {
		t.Fatalf("%+v", r)
	}
}

func TestImportFromFile(t *testing.T) {
	a, _ := newBriefFixture(t)
	path := filepath.Join(realTemp(t), "x.json")
	a.mustRun("export", "--format", "json", "--out", path)
	b := secondProject(t)
	if r := b.run("import", path); r.code != ExitOK {
		t.Fatalf("%+v", r)
	}
	if r := b.run("import", filepath.Join(path, "missing")); r.code != ExitRuntime {
		t.Fatalf("%+v", r)
	}
}

func TestImportMalformed(t *testing.T) {
	a, _ := newBriefFixture(t)
	doc := a.mustRun("export", "--format", "json").out
	cases := map[string]string{
		"unknown field": strings.Replace(doc, `"format"`, `"bogus": 1, "format"`, 1),
		"trailing":      doc + "{}",
		"wrong format":  strings.Replace(doc, "cairn-export", "other", 1),
		"wrong version": strings.Replace(doc, `"version": 1`, `"version": 9`, 1),
		"truncated":     doc[:len(doc)/2],
		"empty":         "",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			b := secondProject(t)
			if r := b.runIn(b.root, in, "import", "-"); r.code != ExitValidation || countNotes(t, b) != 0 {
				t.Fatalf("%+v", r)
			}
		})
	}
}

func TestImportOversize(t *testing.T) {
	b := secondProject(t)
	r := b.runIn(b.root, strings.Repeat(" ", 32<<20+1), "import", "-")
	if r.code != ExitValidation || !strings.Contains(r.err, "too large") {
		t.Fatalf("%+v", r)
	}
}

func TestImportUnknownModelOnlyHere(t *testing.T) {
	a, _ := newBriefFixture(t)
	doc := strings.Replace(a.mustRun("export", "--format", "json").out, "claude-test", "unknown", 1)
	b := secondProject(t)
	if r := b.runIn(b.root, doc, "import", "-"); r.code != ExitOK {
		t.Fatalf("import must accept model unknown: %+v", r)
	}
	if r := b.run(append(withFlags("--model", "unknown", "--change", "x"), noneBut("change")...)...); r.code != ExitValidation {
		t.Fatalf("add must refuse model unknown: %+v", r)
	}
}

func TestImportBadLinksAndMissingIDs(t *testing.T) {
	a, _ := newBriefFixture(t)
	doc := a.mustRun("export", "--format", "json").out
	var m map[string]any
	json.Unmarshal([]byte(doc), &m)
	notes := m["notes"].([]any)
	first := notes[0].(map[string]any)
	first["resolves"] = []any{"e-zzzzzz"} // no such entry
	b1, _ := json.Marshal(m)
	b := secondProject(t)
	if r := b.runIn(b.root, string(b1), "import", "-"); r.code != ExitValidation || countNotes(t, b) != 0 {
		t.Fatalf("dangling link: %+v", r)
	}
	delete(first, "resolves")
	delete(first, "id")
	b2, _ := json.Marshal(m)
	if r := b.runIn(b.root, string(b2), "import", "-"); r.code != ExitValidation || !strings.Contains(r.err, "notes[0].id") {
		t.Fatalf("missing id: %+v", r)
	}
}

func TestImportNoProject(t *testing.T) {
	if r := newEnv(t).run("import", "-"); r.code != ExitNoProject {
		t.Fatalf("%+v", r)
	}
}

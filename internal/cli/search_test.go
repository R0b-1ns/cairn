package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSearchFindsAcrossNotesAndAddenda(t *testing.T) {
	e, f := newBriefFixture(t)
	r := e.mustRun("search", "tidal")
	if !strings.Contains(r.out, "Added the tidal index.") || !strings.Contains(r.out, f.note1) {
		t.Fatalf("entry hit missing:\n%s", r.out)
	}
	e.mustRun("amend", f.note1, "--agent", "builder", "--model", "claude-test", "--note", "The zebra cache is warm.")
	r = e.mustRun("search", "zebra")
	if !strings.Contains(r.out, "zebra") || !strings.Contains(r.out, f.note1) || !strings.Contains(r.out, "a-") {
		t.Fatalf("addendum hit missing:\n%s", r.out)
	}
	if r := e.mustRun("search", "Work", "on", "TP-04"); !strings.Contains(r.out, f.other) {
		t.Fatalf("title search: all terms must match:\n%s", r.out)
	}
}

func TestSearchAllTermsRequired(t *testing.T) {
	e, _ := newBriefFixture(t)
	if r := e.mustRun("search", "tidal", "index"); strings.Contains(r.out, "no matches") {
		t.Fatalf("tidal index must match:\n%s", r.out)
	}
	if r := e.mustRun("search", "tidal", "zebra"); !strings.Contains(r.out, "no matches") {
		t.Fatalf("tidal zebra must not match:\n%s", r.out)
	}
}

func TestSearchOperatorsInert(t *testing.T) {
	e, _ := newBriefFixture(t)
	for _, q := range []string{`"`, `*`, `NEAR(`, `-x`, `a OR b`, `)`, `AND`, `foo"bar`, `:`, `^x`, `tidal*`, `title:Work`, `NOT`} {
		args := []string{"search", q}
		if strings.HasPrefix(q, "-") {
			args = []string{"search", "--", q} // "--" ends flag parsing
		}
		r := e.run(args...)
		if r.code != ExitOK || strings.Contains(strings.ToLower(r.err), "syntax") || strings.Contains(strings.ToLower(r.err), "fts5") {
			t.Errorf("search %q: %+v", q, r)
		}
	}
	for _, args := range [][]string{{"search"}, {"search", ""}, {"search", "  "}} {
		if r := e.run(args...); r.code != ExitUsage {
			t.Errorf("%q: want exit 2, got %+v", args, r)
		}
	}
}

func TestSearchMaxAndJSON(t *testing.T) {
	e, _ := newBriefFixture(t)
	r := e.mustRun("search", "TP", "--max", "1")
	if !strings.Contains(r.out, "more (raise --max)") {
		t.Fatalf("no cap marker:\n%s", r.out)
	}
	if r := e.run("search", "TP", "--max", "0"); r.code != ExitUsage {
		t.Fatalf("--max 0: %+v", r)
	}
	var hits []struct{ Kind, ID, NoteID, Snippet string }
	if err := json.Unmarshal([]byte(e.mustRun("search", "tidal", "--json").out), &hits); err != nil || len(hits) == 0 || hits[0].ID == "" {
		t.Fatalf("json: %v %+v", err, hits)
	}
}

func TestSearchNoProject(t *testing.T) {
	if r := newEnv(t).run("search", "x"); r.code != ExitNoProject {
		t.Fatalf("%+v", r)
	}
}

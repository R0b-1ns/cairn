package cli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const (
	issue1 = "https://github.com/a/b/issues/1"
	issue2 = "https://github.com/a/b/issues/2"
)

func originNote(t *testing.T, e *testEnv, task string, extra ...string) string {
	t.Helper()
	id, _ := addTaskNote(t, e, task, append([]string{"--change", "Did " + task + ".", "--none", "decision,open,note,helper"}, extra...)...)
	return id
}

func amendNote(t *testing.T, e *testEnv, id string, extra ...string) {
	t.Helper()
	e.mustRun(append([]string{"amend", id, "--agent", "human", "--model", "none", "--status", "done"}, extra...)...)
}

func TestShowCardHasAuthorBranchRef(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	setClock(t, "2026-10-30T10:00:00Z")
	id := originNote(t, e, "TP-03", "--author", "Ada <ada@x.org>", "--branch", "main", "--ref", issue1)
	setClock(t, "2026-10-31T10:00:00Z")
	amendNote(t, e, id, "--author", "Bob <bob@x.org>", "--branch", "fix/x", "--ref", issue2)
	out := e.mustRun("show", id).out
	for _, want := range []string{"by Ada <ada@x.org> on main\n", "ref: " + issue1 + "\n", "ref: " + issue2 + "\n", "  by Bob <bob@x.org> on fix/x\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("show missing %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "ref: "+issue1) > strings.Index(out, "ref: "+issue2) {
		t.Errorf("own ref must come first:\n%s", out)
	}
	if !strings.Contains(e.mustRun("show", id, "--json").out, `"ref": "`+issue1+`"`) {
		t.Error("json lacks ref")
	}
	// an addendum on its own
	var aid string
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "addendum ") {
			aid = strings.Fields(l)[1]
		}
	}
	if o := e.mustRun("show", aid).out; !strings.Contains(o, "by Bob <bob@x.org> on fix/x") || !strings.Contains(o, "ref: "+issue2) {
		t.Errorf("show addendum:\n%s", o)
	}
}

func TestShowRefsFromTaskAndComputedLink(t *testing.T) {
	e := newEnv(t)
	e.initProject("--tasks", "jira", "--task-url", "https://x.atlassian.net/browse/{task}")
	setClock(t, "2026-10-30T10:00:00Z")
	a := originNote(t, e, "PROJ-12.1", "--author", "Ada", "--branch", "main")
	if o := e.mustRun("show", a).out; !strings.Contains(o, "ref: https://x.atlassian.net/browse/PROJ-12\n") {
		t.Errorf("computed link:\n%s", o)
	}
	setClock(t, "2026-10-31T10:00:00Z")
	originNote(t, e, "PROJ-12", "--author", "Ada", "--branch", "main", "--ref", issue1)
	o := e.mustRun("show", a).out
	if !strings.Contains(o, "ref: "+issue1) || strings.Contains(o, "atlassian") {
		t.Errorf("stored ref from a sibling must win over the template:\n%s", o)
	}
}

// isolateGit hides the machine's git identity so notes carry no origin.
func isolateGit(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", realTemp(t))
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
}

func TestBriefShowsAllRefsNewestFirst(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	setClock(t, "2026-10-30T10:00:00Z")
	id := originNote(t, e, "TP-03", "--author", "Ada <ada@x.org>", "--branch", "main", "--ref", issue1)
	setClock(t, "2026-10-31T10:00:00Z")
	amendNote(t, e, id, "--author", "Ada", "--branch", "main", "--ref", issue2)
	out := e.mustRun("brief", "TP-03").out
	want := "refs: " + issue2 + ", " + issue1 + "\n"
	if !strings.Contains(out, want) {
		t.Errorf("want %q in:\n%s", want, out)
	}
	if !strings.Contains(out, "(server, builder) by Ada <ada@x.org> on main\n") {
		t.Errorf("note line lacks origin:\n%s", out)
	}
}

func TestBriefComputedLinkWhenNoRef(t *testing.T) {
	e := newEnv(t)
	e.initProject("--tasks", "jira", "--task-url", "https://x/browse/{task}")
	setClock(t, "2026-10-30T10:00:00Z")
	originNote(t, e, "PROJ-12.1", "--author", "Ada", "--branch", "main")
	if out := e.mustRun("brief", "PROJ-12").out; !strings.Contains(out, "refs: https://x/browse/PROJ-12\n") {
		t.Errorf("computed:\n%s", out)
	}
	if out := e.mustRun("brief", "PROJ-12.1").out; !strings.Contains(out, "refs: https://x/browse/PROJ-12\n") {
		t.Errorf("computed from extension:\n%s", out)
	}
	originNote(t, e, "PROJ-12", "--author", "Ada", "--branch", "main", "--ref", issue1)
	out := e.mustRun("brief", "PROJ-12").out
	if !strings.Contains(out, "refs: "+issue1+"\n") || strings.Contains(out, "browse") {
		t.Errorf("stored must win:\n%s", out)
	}
}

// TestBriefWithoutProvenanceOmitsKeys checks a project without authors,
// branches or refs. Byte identity with the old build on a migrated project is
// checked against golden files by TestLegacyOutputGolden (internal/store).
func TestBriefWithoutProvenanceOmitsKeys(t *testing.T) {
	isolateGit(t)
	e, _ := newBriefFixture(t)
	out := e.mustRun("brief", "TP-03").out
	if strings.Contains(out, "refs:") {
		t.Errorf("legacy brief must not print refs:\n%s", out)
	}
	j := e.mustRun("brief", "TP-03", "--json").out
	for _, k := range []string{`"ref"`, `"branch"`, `"author_name"`, `"author_email"`} {
		if strings.Contains(j, k) {
			t.Errorf("empty key %s emitted:\n%s", k, j)
		}
	}
}

func TestListFiltersBranchAuthorRef(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	setClock(t, "2026-10-30T10:00:00Z")
	n1 := originNote(t, e, "TP-03", "--author", "Ada <ada@x.org>", "--branch", "main", "--ref", issue1)
	setClock(t, "2026-10-31T10:00:00Z")
	amendNote(t, e, n1, "--author", "Bob <bob@x.org>", "--branch", "fix/x")
	n2 := originNote(t, e, "TP-03.1", "--author", "Ada", "--branch", "main")
	n3 := originNote(t, e, "TP-04", "--author", "Cy", "--branch", "other")
	has := func(out string, ids ...string) {
		t.Helper()
		for _, id := range []string{n1, n2, n3} {
			want := false
			for _, w := range ids {
				want = want || w == id
			}
			if got := strings.Contains(out, id); got != want {
				t.Errorf("note %s present=%v want %v in:\n%s", id, got, want, out)
			}
		}
	}
	has(e.mustRun("list", "--branch", "fix/x").out, n1)
	has(e.mustRun("list", "--author", "bob@x.org").out, n1)
	has(e.mustRun("list", "--author", "Cy").out, n3)
	has(e.mustRun("list", "--ref", issue1).out, n1, n2)
	has(e.mustRun("list", "--branch", "main", "--author", "Ada").out, n1, n2)
	if !strings.Contains(e.mustRun("brief", "TP-03", "--branch", "fix/x").out, n1) {
		t.Error("brief --branch")
	}
	if out := e.mustRun("brief", "TP-03", "--author", "Cy").out; strings.Contains(out, n1) {
		t.Errorf("brief --author must filter:\n%s", out)
	}
}

func TestExportMarkdownShowsOriginAndRefs(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	setClock(t, "2026-10-30T10:00:00Z")
	id := originNote(t, e, "TP-03", "--author", "Ada <ada@x.org>", "--branch", "main", "--ref", issue1)
	setClock(t, "2026-10-31T10:00:00Z")
	amendNote(t, e, id, "--author", "Bob", "--branch", "fix/x", "--ref", issue2)
	out := e.mustRun("export").out
	for _, want := range []string{"by Ada <ada@x.org> on main\n", "ref: " + issue1 + "\n", "- by Bob on fix/x\n", "- ref: " + issue2 + "\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown missing %q:\n%s", want, out)
		}
	}
}

func TestExportImportRoundTripNewFields(t *testing.T) {
	a := newEnv(t)
	a.initProject()
	setClock(t, "2026-10-30T10:00:00Z")
	id := originNote(t, a, "TP-03", "--author", "Ada <ada@x.org>", "--branch", "main", "--ref", issue1)
	setClock(t, "2026-10-31T10:00:00Z")
	amendNote(t, a, id, "--author", "Bob <bob@x.org>", "--branch", "fix/x", "--ref", issue2)
	doc := a.mustRun("export", "--format", "json").out
	b := secondProject(t)
	if r := b.runIn(b.root, doc, "import", "-"); r.code != 0 || !strings.Contains(r.out, "imported 1 notes and 1 addenda") {
		t.Fatalf("%+v", r)
	}
	if a.mustRun("show", id, "--json").out != b.mustRun("show", id, "--json").out {
		t.Fatal("show --json differs")
	}
	if r := b.runIn(b.root, doc, "import", "-"); r.code != 0 || !strings.Contains(r.out, "imported 0 notes and 0 addenda (2 already present)") {
		t.Fatalf("re-import: %+v", r)
	}
}

func TestOldExportStillImports(t *testing.T) {
	isolateGit(t)
	a, _ := newBriefFixture(t)
	doc := a.mustRun("export", "--format", "json").out
	for _, k := range []string{"branch", "author_name", "author_email", "ref"} {
		if strings.Contains(doc, `"`+k+`"`) {
			t.Fatalf("an export without origin must not carry %s", k)
		}
	}
	b := secondProject(t)
	if r := b.runIn(b.root, doc, "import", "-"); r.code != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestImportRejectsNewTask(t *testing.T) {
	a := newEnv(t)
	a.initProject()
	id := originNote(t, a, "TP-03", "--author", "Ada", "--branch", "main")
	doc := a.mustRun("export", "--format", "json").out
	bad := strings.Replace(doc, `"id": "`+id+`"`, `"id": "`+id+`", "new_task": true`, 1)
	b := secondProject(t)
	r := b.runIn(b.root, bad, "import", "-")
	if r.code != ExitValidation || !strings.Contains(r.err, "new_task") {
		t.Fatalf("%+v", r)
	}
}

func jsonRefs(t *testing.T, out string) ([]string, bool) {
	t.Helper()
	var v struct {
		Refs *[]string `json:"refs"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if v.Refs == nil {
		return nil, false
	}
	return *v.Refs, true
}

func TestJSONRefs(t *testing.T) {
	e := newEnv(t)
	e.initProject("--tasks", "jira", "--task-url", "https://x.atlassian.net/browse/{task}")
	setClock(t, "2026-10-30T10:00:00Z")
	a := originNote(t, e, "PROJ-12.1")
	computed := []string{"https://x.atlassian.net/browse/PROJ-12"}
	if got, _ := jsonRefs(t, e.mustRun("brief", "PROJ-12", "--json").out); !reflect.DeepEqual(got, computed) {
		t.Errorf("brief computed: %v", got)
	}
	if got, _ := jsonRefs(t, e.mustRun("show", a, "--json").out); !reflect.DeepEqual(got, computed) {
		t.Errorf("show computed: %v", got)
	}
	setClock(t, "2026-10-31T10:00:00Z")
	b := originNote(t, e, "PROJ-12", "--ref", issue1)
	setClock(t, "2026-11-01T10:00:00Z")
	amendNote(t, e, a, "--ref", issue2)
	if got, _ := jsonRefs(t, e.mustRun("brief", "PROJ-12", "--json").out); !reflect.DeepEqual(got, []string{issue2, issue1}) {
		t.Errorf("brief stored: %v", got)
	}
	if got, _ := jsonRefs(t, e.mustRun("show", b, "--json").out); !reflect.DeepEqual(got, []string{issue1, issue2}) {
		t.Errorf("show stored (own first): %v", got)
	}
	e2 := newEnv(t)
	e2.initProject()
	c := originNote(t, e2, "TP-03")
	if _, ok := jsonRefs(t, e2.mustRun("brief", "TP-03", "--json").out); ok {
		t.Error("brief: refs must be omitted when empty")
	}
	if _, ok := jsonRefs(t, e2.mustRun("show", c, "--json").out); ok {
		t.Error("show: refs must be omitted when empty")
	}
}

func TestComputedLinkUsesPatternBase(t *testing.T) {
	e := newEnv(t)
	e.initProject("--task-pattern", `^(?P<base>REL-\d+\.\d+)(?:-(?P<ext>\d+))?$`, "--task-url", "https://x.example/r/{task}")
	setClock(t, "2026-10-30T10:00:00Z")
	id := originNote(t, e, "REL-1.2-3")
	want := "https://x.example/r/REL-1.2"
	if o := e.mustRun("show", id).out; !strings.Contains(o, "ref: "+want+"\n") {
		t.Errorf("show:\n%s", o)
	}
	if o := e.mustRun("brief", "REL-1.2").out; !strings.Contains(o, "refs: "+want+"\n") {
		t.Errorf("brief:\n%s", o)
	}
}

func TestOriginLineFormat(t *testing.T) {
	for _, c := range []struct{ name, email, branch, want string }{
		{"Ada", "ada@x.org", "main", "by Ada <ada@x.org> on main"},
		{"Ada", "", "main", "by Ada on main"},
		{"", "ada@x.org", "", "by <ada@x.org>"},
		{"", "", "main", "on main"},
		{"", "", "", ""},
	} {
		if got := originLine(c.name, c.email, c.branch); got != c.want {
			t.Errorf("originLine(%q, %q, %q) = %q, want %q", c.name, c.email, c.branch, got, c.want)
		}
	}
}

func TestBriefOriginWithEmailOnly(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	setClock(t, "2026-10-30T10:00:00Z")
	originNote(t, e, "TP-03", "--author", "<ada@x.org>", "--branch", "main")
	if out := e.mustRun("brief", "TP-03").out; !strings.Contains(out, "(server, builder) by <ada@x.org> on main\n") {
		t.Errorf("brief:\n%s", out)
	}
}

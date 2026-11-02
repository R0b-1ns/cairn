package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/R0b-1ns/cairn/internal/store"
)

// scopeRun opens the parent in e.root and returns its scope and what openScope printed.
func scopeRun(t *testing.T, e *testEnv) (*scope, string) {
	t.Helper()
	var errb bytes.Buffer
	c := &Ctx{Stdout: &bytes.Buffer{}, Stderr: &errb, Getenv: e.getenv}
	t.Chdir(e.root)
	s, code := openProject(c, "")
	if s == nil {
		t.Fatalf("openProject: %d %s", code, errb.String())
	}
	t.Cleanup(s.Close)
	errb.Reset()
	sc := openScope(c, s)
	t.Cleanup(sc.Close)
	return sc, errb.String()
}

func addLocalChild(t *testing.T, e *testEnv, name string) string {
	t.Helper()
	dir := realTemp(t)
	if r := e.runIn(dir, "", "init", "--project", "tidepool", "--repo", name, "--local"); r.code != 0 {
		t.Fatalf("init %s: %d %s", name, r.code, r.err)
	}
	return dir
}

func dirNames(t *testing.T, dir string) string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var n []string
	for _, x := range es {
		n = append(n, x.Name())
	}
	return strings.Join(n, ",")
}

func TestScopeOpensLocalChildren(t *testing.T) {
	f := newChildFixture(t, true)
	sc, warn := scopeRun(t, f.e)
	if len(sc.Sources) != 2 || sc.Sources[0].Child != "" || sc.Sources[1].Child != "web" || len(sc.Missing) != 0 || warn != "" {
		t.Fatalf("sources %+v missing %v warn %q", sc.Sources, sc.Missing, warn)
	}
	if sc.Sources[1].Proj.Name != "web" {
		t.Errorf("child project %+v", sc.Sources[1].Proj)
	}
}

func TestScopeSkipsAttachedChildren(t *testing.T) {
	f := newChildFixture(t, false)
	sc, warn := scopeRun(t, f.e)
	if len(sc.Sources) != 1 || warn != "" {
		t.Fatalf("sources %d warn %q", len(sc.Sources), warn)
	}
}

func TestScopeMissingChild(t *testing.T) {
	cases := map[string]func(t *testing.T, f childFixture) string{
		"no registry path": func(t *testing.T, f childFixture) string {
			reg, _ := openRegistry(&Ctx{Getenv: f.e.getenv})
			if err := reg.RemoveChild("tidepool", "web"); err != nil {
				t.Fatal(err)
			}
			return "<path>"
		},
		"no database": func(t *testing.T, f childFixture) string {
			empty := realTemp(t)
			reg, _ := openRegistry(&Ctx{Getenv: f.e.getenv})
			reg.SetChild("tidepool", "web", empty)
			return empty
		},
		"another project": func(t *testing.T, f childFixture) string {
			other := realTemp(t)
			if r := f.e.runIn(other, "", "init", "--name", "other", "--repos", "x"); r.code != 0 {
				t.Fatalf("init other: %d %s", r.code, r.err)
			}
			reg, _ := openRegistry(&Ctx{Getenv: f.e.getenv})
			reg.SetChild("tidepool", "web", other)
			return other
		},
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			f := newChildFixture(t, true)
			path := mk(t, f)
			sc, warn := scopeRun(t, f.e)
			if len(sc.Sources) != 1 || len(sc.Missing) != 1 || sc.Missing[0] != "web" {
				t.Fatalf("sources %d missing %v", len(sc.Sources), sc.Missing)
			}
			lines := strings.Split(strings.TrimRight(warn, "\n"), "\n")
			if len(lines) != 1 || !strings.HasPrefix(lines[0], "cairn: warning") {
				t.Fatalf("warning %q", warn)
			}
			want := "cairn project link --project tidepool web "
			if !strings.Contains(lines[0], want) {
				t.Errorf("warning lacks %q: %q", want, lines[0])
			}
			_ = path
		})
	}
}

func TestScopeSkipsOutdatedChild(t *testing.T) {
	f := newChildFixture(t, true)
	api := addLocalChild(t, f.e, "api")
	db := filepath.Join(f.child, ".cairn", "cairn.db")
	raw, err := sqlOpenRaw(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("PRAGMA user_version = 4"); err != nil {
		t.Fatal(err)
	}
	raw.Close()
	before, _ := os.ReadFile(db)
	listing := dirNames(t, filepath.Join(f.child, ".cairn"))

	sc, warn := scopeRun(t, f.e)
	if len(sc.Missing) != 1 || sc.Missing[0] != "web" {
		t.Fatalf("missing %v", sc.Missing)
	}
	if len(sc.Sources) != 2 || sc.Sources[1].Child != "api" {
		t.Fatalf("api not read: %+v", sc.Sources)
	}
	if !strings.Contains(warn, "web") || !strings.Contains(warn, "cairn context --project web") || !strings.HasPrefix(warn, "cairn: warning") {
		t.Errorf("warning %q", warn)
	}
	after, _ := os.ReadFile(db)
	if !bytes.Equal(before, after) {
		t.Error("the child database was written")
	}
	if got := dirNames(t, filepath.Join(f.child, ".cairn")); got != listing {
		t.Errorf("listing changed: %q -> %q", listing, got)
	}
	_ = api
}

func TestOpenCurrentNeverWrites(t *testing.T) {
	f := newChildFixture(t, true)
	db := filepath.Join(f.child, ".cairn", "cairn.db")
	st, err := store.OpenCurrent(db)
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	st.Close()
	raw, _ := sqlOpenRaw(db)
	raw.Exec("PRAGMA user_version = 4")
	raw.Close()
	before, _ := os.ReadFile(db)
	fi0, _ := os.Stat(db)
	_, err = store.OpenCurrent(db)
	nm, ok := err.(*store.NeedsMigrationError)
	if !ok || nm.Version != 4 {
		t.Fatalf("err %T %v", err, err)
	}
	after, _ := os.ReadFile(db)
	fi1, _ := os.Stat(db)
	if !bytes.Equal(before, after) || !fi0.ModTime().Equal(fi1.ModTime()) {
		t.Error("file changed")
	}
}

func TestScopeWithoutChildrenIsSilent(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	sc, warn := scopeRun(t, e)
	if len(sc.Sources) != 1 || sc.Sources[0].Child != "" || warn != "" || len(sc.Missing) != 0 {
		t.Fatalf("%+v %q", sc, warn)
	}
}

func TestScopeLocalChildAggregatesNothing(t *testing.T) {
	f := newChildFixture(t, true)
	var errb bytes.Buffer
	c := &Ctx{Stdout: &bytes.Buffer{}, Stderr: &errb, Getenv: f.e.getenv}
	t.Chdir(f.child)
	s, code := openProject(c, "")
	if s == nil {
		t.Fatalf("%d %s", code, errb.String())
	}
	defer s.Close()
	sc := openScope(c, s)
	defer sc.Close()
	if len(sc.Sources) != 1 || errb.Len() != 0 {
		t.Fatalf("%d %q", len(sc.Sources), errb.String())
	}
}

func TestEntryInfoJSONUnchangedWithoutChild(t *testing.T) {
	e := store.EntryInfo{NoteID: "n", Repo: "r"}
	e.ID, e.Type, e.Text, e.By = "e1", "note", "t", "a"
	b, _ := json.Marshal(e)
	golden := `{"id":"e1","type":"note","text":"t","by":"a","note_id":"n","addendum_id":"","task":"","repo":"r","at":"","closed":false,"closed_by":"","closed_at":""}`
	if string(b) != golden {
		t.Fatalf("got %s", b)
	}
	e.Child = "web"
	b, _ = json.Marshal(e)
	if !strings.Contains(string(b), `"child":"web"`) {
		t.Fatalf("got %s", b)
	}
}

func TestChildJSONOmittedWhenEmpty(t *testing.T) {
	b, _ := json.Marshal(store.Hit{})
	h, _ := json.Marshal(store.NoteHeader{})
	if strings.Contains(string(b), "child") || strings.Contains(string(h), "child") {
		t.Fatalf("%s %s", b, h)
	}
	b, _ = json.Marshal(store.Hit{Child: "w"})
	h, _ = json.Marshal(store.NoteHeader{Child: "w"})
	if !strings.Contains(string(b), `"child":"w"`) || !strings.Contains(string(h), `"child":"w"`) {
		t.Fatalf("%s %s", b, h)
	}
}

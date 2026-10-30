package cli

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// hostileDB inserts rows straight into the database, bypassing validation, the
// way a tampered clone could: escape sequences and short values in every text column.
func hostileDB(t *testing.T, e *testEnv) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(e.root, ".cairn", "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	esc := "\x1b[31m\x1b]0;pwned\x07\x1b[2J"
	exec := func(q string, a ...any) {
		t.Helper()
		if _, err := db.Exec(q, a...); err != nil {
			t.Fatal(err)
		}
	}
	for i, id := range []string{"n-aaaaaa", "n-" + esc} {
		exec(`INSERT INTO notes(id,agent,repo,at,task,task_base,task_ext,model,title,status,none,impacts,commits)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, "agent"+esc, "repo"+esc, []string{"2026-01-01T00:00:00Z", "x"}[i],
			"TP-03"+esc, "TP-03", "", "model"+esc, "title"+esc, "done", `["`+"open"+esc+`"]`, `["`+"client"+esc+`"]`, `["abc`+esc+`"]`)
		exec(`INSERT INTO entries(id,note_id,pos,type,text,decided_by) VALUES (?,?,0,'decision',?,?)`,
			"e-"+[]string{"aaaaaa", esc}[i], id, "text"+esc, "by"+esc)
		exec(`INSERT INTO entries(id,note_id,pos,type,text,decided_by) VALUES (?,?,1,'open',?,'')`,
			"e-"+[]string{"bbbbbb", esc + "b"}[i], id, "open"+esc)
		exec(`INSERT INTO search(text,kind,id,note_id) VALUES (?,?,?,?)`, "needle"+esc, "entry", "e-"+esc, id)
	}
	exec(`INSERT INTO addenda(id,note_id,agent,model,at,status) VALUES ('a-aaaaaa','n-aaaaaa',?,?,'short','done')`, "ag"+esc, "mo"+esc)
	exec(`INSERT INTO links(kind,target_entry_id,source_addendum_id) VALUES ('resolves','e-bbbbbb','a-aaaaaa')`)
}

func TestHostileDatabaseNeverReachesTerminal(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	hostileDB(t, e)
	cmds := [][]string{
		{"show", "n-aaaaaa"}, {"show", "a-aaaaaa"}, {"show", "e-aaaaaa"}, {"show", "e-bbbbbb"},
		{"list", "--all"}, {"list", "--resolved"}, {"recent"}, {"brief", "TP-03"},
		{"search", "needle"}, {"check", "--older-than", "0", "--done", "TP-03"}, {"context"}, {"projects"},
	}
	for _, args := range cmds {
		r := e.run(args...)
		for _, out := range []string{r.out, r.err} {
			for _, b := range []byte(out) {
				if b != '\n' && b != '\t' && (b < 0x20 || b == 0x7f) {
					t.Fatalf("cairn %v wrote control byte %#x:\nstdout: %q\nstderr: %q", args, b, r.out, r.err)
				}
			}
		}
		if strings.Contains(r.err, "panic") {
			t.Fatalf("cairn %v panicked: %s", args, r.err)
		}
	}
}

func TestImportRefusesFutureTimestamps(t *testing.T) {
	a, _ := newBriefFixture(t)
	doc := a.mustRun("export", "--format", "json").out
	forged := strings.Replace(doc, `"at": "2026-10-30T10:00:00Z"`, `"at": "9999-12-31T23:59:59Z"`, 1)
	b := secondProject(t)
	r := b.runIn(b.root, forged, "import", "-")
	if r.code != ExitValidation || !strings.Contains(r.err, "notes[0].at") || countNotes(t, b) != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestMachineDatabaseMissingIsNoProject(t *testing.T) {
	e := newEnv(t)
	e.initProject("--mode", "machine")
	os.Remove(filepath.Join(e.home, "projects", "tidepool", "cairn.db"))
	if r := e.run("context"); r.code != ExitNoProject || strings.Contains(r.err, "lstat") {
		t.Fatalf("want a clear exit 4, got %+v", r)
	}
}

func TestUnregisteredStoreIsAnnounced(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	clone := realTemp(t)
	copyDir(t, filepath.Join(e.root, ".cairn"), filepath.Join(clone, ".cairn"))
	other := newEnv(t) // a machine that never registered this clone
	r := other.runIn(clone, "", "context")
	if r.code != ExitOK || !strings.Contains(r.err, "not registered") {
		t.Fatalf("want a stderr notice about the unregistered store: %+v", r)
	}
	if r := e.run("context"); strings.Contains(r.err, "not registered") {
		t.Fatalf("a registered project must stay quiet: %+v", r)
	}
}

// Files written by export bypass the terminal filter, so they are cleaned themselves.
func TestExportFilesNeverCarryControlBytes(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	db, err := sql.Open("sqlite", filepath.Join(e.root, ".cairn", "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	esc := "\x1b[31m\x1b]0;pwned\x07\u009b2J"
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO notes(id,agent,repo,at,task,task_base,task_ext,model,title,status,none,impacts,commits)
			VALUES ('n-aaaaaa',?,?,?,?,'TP-03','',?,?,'done','[]','[]','[]')`, []any{"ag" + esc, "re" + esc, "2026-01-01T00:00:00Z", "TP-03" + esc, "mo" + esc, "ti" + esc}},
		{`INSERT INTO entries(id,note_id,pos,type,text,decided_by) VALUES ('e-aaaaaa','n-aaaaaa',0,'decision',?,?)`, []any{"tx" + esc, "by" + esc}},
		{`INSERT INTO addenda(id,note_id,agent,model,at,status) VALUES ('a-aaaaaa','n-aaaaaa',?,?,'2026-01-02T00:00:00Z','done')`, []any{"ag" + esc, "mo" + esc}},
		{`INSERT INTO entries(id,addendum_id,pos,type,text,decided_by) VALUES ('e-bbbbbb','a-aaaaaa',0,'note',?,'')`, []any{"tx" + esc}},
		{`INSERT INTO links(kind,target_entry_id,source_addendum_id) VALUES ('supersedes','e-aaaaaa','a-aaaaaa')`, nil},
	} {
		if _, err := db.Exec(q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	dir := realTemp(t)
	for _, format := range []string{"md", "json"} {
		path := filepath.Join(dir, "out."+format)
		if r := e.run("export", "--format", format, "--out", path); r.code != ExitOK {
			t.Fatalf("export %s: %+v", format, r)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range string(b) {
			if r != '\n' && r != '\t' && (r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f)) {
				t.Fatalf("%s export contains control character %U", format, r)
			}
		}
	}
}

package cli

import (
	"database/sql"
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

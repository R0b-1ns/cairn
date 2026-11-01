package store_test

import (
	"bytes"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/R0b-1ns/cairn/internal/cli"
	"github.com/R0b-1ns/cairn/internal/store"
)

// legacyFixture is a version 3 project with fixed IDs and timestamps: two
// tasks (one with an extension), a decision superseded later, an open point
// resolved by an addendum, one still open, and a note without a task.
var legacyFixture = []string{
	`INSERT INTO project(id,name,mode,context,repos,default_repo,task_pattern) VALUES
		(1,'tidepool','project','Tide data service.','["server","client"]','server','^(?P<base>[A-Z]{2,5}-\d{1,4})(?:\.(?P<ext>\d{1,3}))?$')`,
	`INSERT INTO notes(id,agent,repo,at,task,task_base,task_ext,model,title,status,none,impacts,commits) VALUES
		('n-0aaaaa','builder','server','2026-10-28T09:00:00Z','TP-03','TP-03','','m-1','Tide import','in_progress','["helper"]','["client"]','["abc1234"]')`,
	`INSERT INTO entries(id,note_id,addendum_id,pos,type,text,decided_by) VALUES
		('e-0aaaa1','n-0aaaaa',NULL,0,'change','Wrote the tide importer.',''),
		('e-0aaaa2','n-0aaaaa',NULL,1,'decision','Store heights in millimetres.','user'),
		('e-0aaaa3','n-0aaaaa',NULL,2,'open','Which station is the reference?',''),
		('e-0aaaa4','n-0aaaaa',NULL,3,'note','Data arrives hourly.','')`,
	`INSERT INTO notes(id,agent,repo,at,task,task_base,task_ext,model,title,status,none,impacts,commits) VALUES
		('n-0bbbbb','reviewer','client','2026-10-29T10:30:00Z','TP-03.1','TP-03','1','m-2','Chart for tides','done','["decision","helper"]','[]','[]')`,
	`INSERT INTO entries(id,note_id,addendum_id,pos,type,text,decided_by) VALUES
		('e-0bbbb1','n-0bbbbb',NULL,0,'change','Drew the tide chart.',''),
		('e-0bbbb2','n-0bbbbb',NULL,1,'open','Colour scheme for spring tides?',''),
		('e-0bbbb3','n-0bbbbb',NULL,2,'note','Uses the importer output.','')`,
	`INSERT INTO addenda(id,note_id,agent,model,at,status) VALUES
		('a-0ccccc','n-0aaaaa','builder','m-1','2026-10-30T08:15:00Z','done')`,
	`INSERT INTO entries(id,note_id,addendum_id,pos,type,text,decided_by) VALUES
		('e-0cccc1',NULL,'a-0ccccc',0,'decision','Station Brest is the reference.','agent')`,
	`INSERT INTO links(kind,target_entry_id,source_note_id,source_addendum_id) VALUES
		('resolves','e-0aaaa3',NULL,'a-0ccccc')`,
	`INSERT INTO notes(id,agent,repo,at,task,task_base,task_ext,model,title,status,none,impacts,commits) VALUES
		('n-0ddddd','builder','server','2026-10-31T12:00:00Z','TP-04','TP-04','','m-1','Units rework','blocked','["open","note","helper"]','["client","server"]','[]')`,
	`INSERT INTO entries(id,note_id,addendum_id,pos,type,text,decided_by) VALUES
		('e-0dddd1','n-0ddddd',NULL,0,'change','Switched to centimetres.',''),
		('e-0dddd2','n-0ddddd',NULL,1,'decision','Store heights in centimetres.','user')`,
	`INSERT INTO links(kind,target_entry_id,source_note_id,source_addendum_id) VALUES
		('supersedes','e-0aaaa2','n-0ddddd',NULL)`,
	`INSERT INTO notes(id,agent,repo,at,task,task_base,task_ext,model,title,status,none,impacts,commits) VALUES
		('n-0eeeee','builder','server','2026-11-01T07:00:00Z','','','','m-1','Housekeeping','done','["decision","open","helper"]','[]','[]')`,
	`INSERT INTO entries(id,note_id,addendum_id,pos,type,text,decided_by) VALUES
		('e-0eeee1','n-0eeeee',NULL,0,'change','Renamed the tide module.',''),
		('e-0eeee2','n-0eeeee',NULL,1,'note','No behaviour change.','')`,
	`INSERT INTO search(text,kind,id,note_id) VALUES
		('Tide import','note','n-0aaaaa','n-0aaaaa'),
		('Wrote the tide importer.','entry','e-0aaaa1','n-0aaaaa'),
		('Store heights in millimetres.','entry','e-0aaaa2','n-0aaaaa'),
		('Which station is the reference?','entry','e-0aaaa3','n-0aaaaa'),
		('Data arrives hourly.','entry','e-0aaaa4','n-0aaaaa'),
		('Chart for tides','note','n-0bbbbb','n-0bbbbb'),
		('Drew the tide chart.','entry','e-0bbbb1','n-0bbbbb'),
		('Colour scheme for spring tides?','entry','e-0bbbb2','n-0bbbbb'),
		('Uses the importer output.','entry','e-0bbbb3','n-0bbbbb'),
		('Station Brest is the reference.','entry','e-0cccc1','n-0aaaaa'),
		('Units rework','note','n-0ddddd','n-0ddddd'),
		('Switched to centimetres.','entry','e-0dddd1','n-0ddddd'),
		('Store heights in centimetres.','entry','e-0dddd2','n-0ddddd'),
		('Housekeeping','note','n-0eeeee','n-0eeeee'),
		('Renamed the tide module.','entry','e-0eeee1','n-0eeeee'),
		('No behaviour change.','entry','e-0eeee2','n-0eeeee')`,
	`PRAGMA user_version = 3`,
}

// legacyCommands are compared byte for byte (standard output and exit code)
// with the output of the build that predates schema version 4.
var legacyCommands = []struct {
	golden string
	args   []string
}{
	{"show-note.txt", []string{"show", "n-0aaaaa"}},
	{"show-note.json", []string{"show", "n-0aaaaa", "--json"}},
	{"show-ext-note.txt", []string{"show", "n-0bbbbb"}},
	{"show-addendum.txt", []string{"show", "a-0ccccc"}},
	{"show-addendum.json", []string{"show", "a-0ccccc", "--json"}},
	{"show-entry.txt", []string{"show", "e-0aaaa2"}},
	{"show-entry.json", []string{"show", "e-0aaaa2", "--json"}},
	{"list.txt", []string{"list"}},
	{"list-all.txt", []string{"list", "--all"}},
	{"list-all.json", []string{"list", "--all", "--json"}},
	{"list-task.txt", []string{"list", "--task", "TP-03"}},
	{"list-ext.txt", []string{"list", "--task", "TP-03.1", "--all"}},
	{"list-decision.txt", []string{"list", "--type", "decision", "--all"}},
	{"list-resolved.txt", []string{"list", "--resolved"}},
	{"list-repo.txt", []string{"list", "--repo", "client", "--all"}},
	{"list-impacts-since.txt", []string{"list", "--impacts", "client", "--since", "2026-10-29", "--all"}},
	{"brief.txt", []string{"brief", "TP-03"}},
	{"brief.json", []string{"brief", "TP-03", "--json"}},
	{"brief-ext.txt", []string{"brief", "TP-03.1"}},
	{"brief-ext.json", []string{"brief", "TP-03.1", "--json"}},
	{"brief-other.txt", []string{"brief", "TP-04"}},
	{"export.md", []string{"export"}},
	{"export.json", []string{"export", "--format", "json"}},
	{"export-task.md", []string{"export", "--task", "TP-03"}},
	{"context.txt", []string{"context"}},
	{"context.json", []string{"context", "--json"}},
	{"check.txt", []string{"check", "--done", "TP-03,TP-04,TP-09", "--older-than", "0"}},
	{"check.json", []string{"check", "--done", "TP-03,TP-04,TP-09", "--older-than", "0", "--json"}},
	{"search.txt", []string{"search", "tide"}},
	{"search.json", []string{"search", "--json", "station"}},
}

// buildLegacyFixture writes the version 3 fixture at root/.cairn/cairn.db,
// the way migrate_test.go builds one: baseline tables, triggers, rows.
func buildLegacyFixture(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, ".cairn")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stmts := []string{store.TableSQLv3}
	for _, s := range store.TriggerSQL() {
		stmts = append(stmts, s)
	}
	for _, s := range append(stmts, legacyFixture...) {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("%v\n%s", err, s)
		}
	}
}

// TestLegacyOutputGolden opens the version 3 fixture through the CLI (which
// migrates it) and compares the output of the read commands with golden files
// made by the build before schema version 4.
//
// To regenerate the golden files, build cairn at the commit before task modes
// (f896777) and run:
//
//	CAIRN_GOLDEN_OLD_BIN=/path/to/old/cairn go test ./internal/store -run TestLegacyOutputGolden
//
// Never regenerate them from the current code.
func TestLegacyOutputGolden(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(t.TempDir(), "home")
	buildLegacyFixture(t, root)
	if old := os.Getenv("CAIRN_GOLDEN_OLD_BIN"); old != "" {
		writeGoldens(t, old, root, home)
		return
	}
	getenv := func(k string) string {
		if k == "CAIRN_HOME" {
			return home
		}
		return ""
	}
	golden, err := filepath.Abs(filepath.Join("testdata", "legacy"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range legacyCommands {
		want, err := os.ReadFile(filepath.Join(golden, c.golden))
		if err != nil {
			t.Fatal(err)
		}
		t.Chdir(root)
		var out, errb bytes.Buffer
		code := cli.Run(c.args, strings.NewReader(""), &out, &errb, getenv)
		got := append([]byte{byte('0' + code), '\n'}, out.Bytes()...)
		if !bytes.Equal(got, want) {
			t.Errorf("cairn %s: output differs from %s\n--- got ---\n%s\n--- want ---\n%s\n--- stderr ---\n%s",
				strings.Join(c.args, " "), c.golden, got, want, errb.String())
		}
	}
}

// writeGoldens runs the old binary on the fixture and stores exit code and
// standard output of every command.
func writeGoldens(t *testing.T, bin, root, home string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(wd, "testdata", "legacy")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range legacyCommands {
		cmd := exec.Command(bin, c.args...)
		cmd.Dir = root
		cmd.Env = []string{"CAIRN_HOME=" + home, "CAIRN_NO_UPDATE_CHECK=1", "HOME=" + home, "PATH=/usr/bin:/bin"}
		var out bytes.Buffer
		cmd.Stdout = &out
		code := 0
		if err := cmd.Run(); err != nil {
			ee, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatal(err)
			}
			code = ee.ExitCode()
		}
		data := append([]byte{byte('0' + code), '\n'}, out.Bytes()...)
		if err := os.WriteFile(filepath.Join(dir, c.golden), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("wrote %d golden files with %s", len(legacyCommands), bin)
}

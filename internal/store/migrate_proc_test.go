package store

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Environment of the helper process started by TestMigrateMultiProcess.
const (
	helperDBEnv  = "CAIRN_TEST_MIGRATE_DB"
	helperGoEnv  = "CAIRN_TEST_MIGRATE_GO"
	helperRounds = 20
	helperProcs  = 4
)

// TestMigrateHelperProcess is not a test: run as a subprocess by
// TestMigrateMultiProcess, it waits for the start signal and opens the database.
func TestMigrateHelperProcess(t *testing.T) {
	path := os.Getenv(helperDBEnv)
	if path == "" {
		t.Skip("helper process only")
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(os.Getenv(helperGoEnv)); err == nil {
			break
		}
		if time.Now().After(deadline) {
			fmt.Fprintln(os.Stderr, "no start signal")
			os.Exit(2)
		}
		time.Sleep(time.Millisecond)
	}
	s, err := Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := s.CountNotes(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	s.Close()
	os.Exit(0)
}

// TestMigrateMultiProcess opens one version 3 file from several processes at
// once. Goroutines share the process's POSIX locks, so only separate processes
// can show a migration that loses its SQLite lock.
func TestMigrateMultiProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("starts subprocesses")
	}
	for round := 0; round < helperRounds; round++ {
		path := makeV3(t)
		goFile := filepath.Join(filepath.Dir(path), "go")
		cmds := make([]*exec.Cmd, helperProcs)
		outs := make([]*bytes.Buffer, helperProcs)
		for i := range cmds {
			cmd := exec.Command(os.Args[0], "-test.run=^TestMigrateHelperProcess$", "-test.count=1")
			cmd.Env = append(os.Environ(), helperDBEnv+"="+path, helperGoEnv+"="+goFile)
			outs[i] = &bytes.Buffer{}
			cmd.Stdout, cmd.Stderr = outs[i], outs[i]
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			cmds[i] = cmd
		}
		time.Sleep(50 * time.Millisecond) // let every helper reach the wait loop
		if err := os.WriteFile(goFile, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		for i, cmd := range cmds {
			if err := cmd.Wait(); err != nil {
				t.Errorf("round %d process %d: %v: %s", round, i, err, outs[i].String())
			}
		}
		if t.Failed() {
			t.FailNow()
		}
		checkMigrated(t, path)
	}
}

func checkMigrated(t *testing.T, path string) {
	t.Helper()
	if v := userVersion(t, path); v != SchemaVersion {
		t.Fatalf("user_version = %d, want %d", v, SchemaVersion)
	}
	db := rawDB(t, path)
	for table, cols := range map[string][]string{
		"notes":   {"author_name", "author_email", "branch", "ref"},
		"addenda": {"author_name", "author_email", "branch", "ref"},
		"project": {"task_mode", "task_prefix", "task_url"},
	} {
		for _, col := range cols {
			var n int
			if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, table, col).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 1 {
				t.Fatalf("%s.%s present %d times, want 1", table, col, n)
			}
		}
	}
	for _, p := range []string{path, path + ".v3.bak"} {
		var check string
		if err := rawDB(t, p).QueryRow("PRAGMA integrity_check").Scan(&check); err != nil || check != "ok" {
			t.Fatalf("%s integrity_check: %v %s", p, err, check)
		}
	}
	if v := userVersion(t, path+".v3.bak"); v != 3 {
		t.Fatalf("backup user_version = %d, want 3", v)
	}
	db.Close()
}

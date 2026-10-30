package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

type result struct {
	code     int
	out, err string
}

// testEnv is an isolated machine: its own data home and project root.
type testEnv struct {
	t    *testing.T
	home string
	root string
	env  map[string]string
}

func realTemp(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func newEnv(t *testing.T) *testEnv {
	t.Helper()
	e := &testEnv{t: t, home: filepath.Join(realTemp(t), "home"), root: realTemp(t), env: map[string]string{}}
	return e
}

func (e *testEnv) getenv(k string) string {
	if k == "CAIRN_HOME" {
		return e.home
	}
	return e.env[k]
}

// runIn runs cairn with the given working directory and standard input.
func (e *testEnv) runIn(dir, stdin string, args ...string) result {
	e.t.Helper()
	e.t.Chdir(dir)
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(stdin), &out, &errb, e.getenv)
	return result{code, out.String(), errb.String()}
}

// run runs cairn in the project root.
func (e *testEnv) run(args ...string) result { e.t.Helper(); return e.runIn(e.root, "", args...) }

// mustRun fails the test unless the command exits 0.
func (e *testEnv) mustRun(args ...string) result {
	e.t.Helper()
	r := e.run(args...)
	if r.code != 0 {
		e.t.Fatalf("cairn %v: exit %d\nstdout: %s\nstderr: %s", args, r.code, r.out, r.err)
	}
	return r
}

// initProject creates the standard fictional project in the root.
func (e *testEnv) initProject(extra ...string) {
	e.t.Helper()
	args := append([]string{"init", "--name", "tidepool", "--repos", "server,client",
		"--context", "Tide data service."}, extra...)
	e.mustRun(args...)
}

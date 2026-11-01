package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runTTY runs cairn as if standard input and standard error were a terminal.
func (e *testEnv) runTTY(stdin string, args ...string) result {
	e.t.Helper()
	e.t.Chdir(e.root)
	var out, errb bytes.Buffer
	c := &Ctx{Stdin: strings.NewReader(stdin), Stdout: &out, Stderr: &errb, Getenv: e.getenv, Interactive: true}
	code := runCtx(c, args)
	return result{code, out.String(), errb.String()}
}

func (e *testEnv) projectJSON() string {
	e.t.Helper()
	return e.mustRun("context", "--json").out
}

func TestInteractiveInitAllDefaults(t *testing.T) {
	e := newEnv(t)
	// name, then Enter for: storage, root; repos; Enter for context, mode, tag
	r := e.runTTY("demo\n\n\nserver\n\n\n\n", "init")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.out, r.err)
	}
	j := e.projectJSON()
	for _, want := range []string{`"task_mode": "auto"`, `"task_prefix": "T"`, `"name": "demo"`, `"mode": "project"`} {
		if !strings.Contains(j, want) {
			t.Errorf("missing %s in %s", want, j)
		}
	}
	if strings.Contains(r.err, "task numbering was not chosen") {
		t.Errorf("legacy note printed: %s", r.err)
	}
}

func TestInteractiveSkipsGivenFlags(t *testing.T) {
	e := newEnv(t)
	// Remaining questions: storage, root, context, URL template (jira).
	r := e.runTTY("\n\n\n\n", "init", "--name", "demo", "--repos", "a", "--tasks", "jira")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.err)
	}
	for _, bad := range []string{"Project name", "Repos,", "How do you number", "Tag"} {
		if strings.Contains(r.err, bad) {
			t.Errorf("asked %q: %s", bad, r.err)
		}
	}
	if !strings.Contains(r.err, "Ticket URL") {
		t.Errorf("URL template not asked: %s", r.err)
	}
	if !strings.Contains(e.projectJSON(), `"task_mode": "jira"`) {
		t.Errorf("mode not jira")
	}
}

func TestInteractiveSkipsAllTaskQuestionsWithTaskPattern(t *testing.T) {
	e := newEnv(t)
	r := e.runTTY("\n\n\n", "init", "--name", "demo", "--repos", "a", "--task-pattern", `^(?P<base>TP-\d{2})$`)
	if r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.err)
	}
	if strings.Contains(r.err, "Tag") || strings.Contains(r.err, "numbering") {
		t.Errorf("asked task questions: %s", r.err)
	}
}

func TestInteractiveCustomPrefix(t *testing.T) {
	e := newEnv(t)
	r := e.runTTY("demo\n\n\nserver\n\n\nfeat\nFEAT\n", "init")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.err)
	}
	if !strings.Contains(r.err, "task_prefix") && !strings.Contains(r.err, "capital") {
		t.Errorf("invalid tag not explained: %s", r.err)
	}
	j := e.projectJSON()
	if !strings.Contains(j, `"task_prefix": "FEAT"`) || !strings.Contains(j, `FEAT-`) {
		t.Errorf("prefix not FEAT: %s", j)
	}
}

func TestInteractiveCustomRegexAskedAgain(t *testing.T) {
	e := newEnv(t)
	r := e.runTTY("demo\n\n\nserver\n\n5\nnot a (regex\n^(?P<base>TP-\\d{2})$\n", "init")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.err)
	}
	if !strings.Contains(e.projectJSON(), `"task_mode": "custom"`) {
		t.Errorf("not custom")
	}
	if !strings.Contains(r.err, "regular expression") {
		t.Errorf("bad regex not explained: %s", r.err)
	}
}

func TestInteractiveGitFolderSuggestsRepoAndSeveralAsksDefault(t *testing.T) {
	e := newEnv(t)
	if err := os.Mkdir(filepath.Join(e.root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := e.runTTY("demo\n\n\n\n\n\n\n", "init")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.err)
	}
	if !strings.Contains(r.err, "Repos, comma-separated [demo]") || !strings.Contains(e.projectJSON(), `"repos": [`) {
		t.Errorf("repo suggestion missing: %s", r.err)
	}
	e2 := newEnv(t)
	r = e2.runTTY("two\n\n\na, b\nb\n\n\n\n", "init")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.err)
	}
	if !strings.Contains(e2.projectJSON(), `"default_repo": "b"`) {
		t.Errorf("default repo not b: %s", e2.projectJSON())
	}
}

func TestInteractiveInvalidNameAskedAgain(t *testing.T) {
	e := newEnv(t)
	r := e.runTTY("Bad Name\ndemo\n\n\nserver\n\n\n\n", "init")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.err)
	}
	if !strings.Contains(r.err, "name:") {
		t.Errorf("no explanation: %s", r.err)
	}
}

func TestNoPromptWithoutTerminal(t *testing.T) {
	e := newEnv(t)
	e.t.Chdir(e.root)
	var out, errb bytes.Buffer
	code := Run([]string{"init", "--name", "demo", "--repos", "a"}, failReader{t}, &out, &errb, e.getenv)
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "--tasks") {
		t.Errorf("legacy note missing: %s", errb.String())
	}
}

type failReader struct{ t *testing.T }

func (f failReader) Read([]byte) (int, error) {
	f.t.Helper()
	f.t.Error("standard input was read")
	return 0, os.ErrClosed
}

func TestNoInputFlag(t *testing.T) {
	e := newEnv(t)
	e.t.Chdir(e.root)
	var out, errb bytes.Buffer
	c := &Ctx{Stdin: failReader{t}, Stdout: &out, Stderr: &errb, Getenv: e.getenv, Interactive: true}
	code := runCtx(c, []string{"init", "--name", "demo", "--repos", "a", "--no-input"})
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, errb.String())
	}
	if strings.Contains(errb.String(), "Your choice") || !strings.Contains(errb.String(), "--tasks") {
		t.Errorf("stderr: %s", errb.String())
	}
}

func TestCancelWritesNothing(t *testing.T) {
	e := newEnv(t)
	r := e.runTTY("demo\n", "init")
	if r.code != 1 || !strings.Contains(r.err, "cairn: init canceled") {
		t.Fatalf("exit %d\n%s", r.code, r.err)
	}
	if _, err := os.Stat(filepath.Join(e.root, ".cairn")); !os.IsNotExist(err) {
		t.Errorf(".cairn exists: %v", err)
	}
	if _, err := os.Stat(e.home); !os.IsNotExist(err) {
		t.Errorf("data home was created: %v", err)
	}
	if r := e.run("projects"); strings.Contains(r.out, "demo") {
		t.Errorf("registered: %s", r.out)
	}
}

func TestCancelInTaskQuestionsWritesNothing(t *testing.T) {
	e := newEnv(t)
	r := e.runTTY("demo\n\n\nserver\n\n\n1\n", "init") // EOF at the tag question
	if r.code != 1 {
		t.Fatalf("exit %d\n%s", r.code, r.err)
	}
	if _, err := os.Stat(filepath.Join(e.root, ".cairn")); !os.IsNotExist(err) {
		t.Errorf(".cairn exists")
	}
}

func TestInteractivePrintsEquivalentFlags(t *testing.T) {
	e := newEnv(t)
	r := e.runTTY("demo\n\n\nserver\nit's mine\n1\nFEAT\n", "init")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.err)
	}
	want := `equivalent command: cairn init --name demo --repos server --context 'it'\''s mine' --tasks auto --task-prefix FEAT`
	if !strings.Contains(r.err, want) {
		t.Errorf("want %q in:\n%s", want, r.err)
	}
	if strings.Contains(r.out, "equivalent") {
		t.Errorf("equivalent line on stdout")
	}
}

func TestInteractiveEquivalentKeepsGivenFlags(t *testing.T) {
	e := newEnv(t)
	r := e.runTTY("\n\n\n", "init", "--name", "demo", "--repos", "a", "--tasks", "jira", "--task-url", "https://x/{task}")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.err)
	}
	want := `equivalent command: cairn init --name demo --repos a --tasks jira --task-url 'https://x/{task}'`
	if !strings.Contains(r.err, want) {
		t.Errorf("want %q in:\n%s", want, r.err)
	}
}

func TestInterruptCancelsInitAndWritesNothing(t *testing.T) {
	sig := make(chan os.Signal, 1)
	stopped := false
	old := interruptSignals
	interruptSignals = func() (<-chan os.Signal, func()) { return sig, func() { stopped = true } }
	t.Cleanup(func() { interruptSignals = old })
	e := newEnv(t)
	e.t.Chdir(e.root)
	in, w := io.Pipe() // nobody types: the first question waits until Ctrl-C
	defer w.Close()
	sig <- os.Interrupt
	var out, errb bytes.Buffer
	c := &Ctx{Stdin: in, Stdout: &out, Stderr: &errb, Getenv: e.getenv, Interactive: true}
	if code := runCtx(c, []string{"init"}); code != 1 || !strings.Contains(errb.String(), "cairn: init canceled") {
		t.Fatalf("exit %d\n%s", code, errb.String())
	}
	if !stopped {
		t.Error("signal handling not stopped after the questions")
	}
	if _, err := os.Stat(filepath.Join(e.root, ".cairn")); !os.IsNotExist(err) {
		t.Errorf(".cairn exists: %v", err)
	}
	if _, err := os.Stat(e.home); !os.IsNotExist(err) {
		t.Errorf("data home was created: %v", err)
	}
}

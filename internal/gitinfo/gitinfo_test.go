package gitinfo

import (
	"os"
	"os/exec"
	"testing"
)

func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.org")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.org")
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func newRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	isolate(t)
	dir := t.TempDir()
	git(t, dir, "init", "-b", "main")
	git(t, dir, "config", "user.name", "Ada Lovelace")
	git(t, dir, "config", "user.email", "ada@x.org")
	git(t, dir, "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "x")
	return dir
}

func TestBranch(t *testing.T) {
	dir := newRepo(t)
	if got := Branch(dir); got != "main" {
		t.Errorf("fresh repo: got %q, want main", got)
	}
	git(t, dir, "checkout", "-b", "feat/x")
	if got := Branch(dir); got != "feat/x" {
		t.Errorf("after checkout: got %q, want feat/x", got)
	}
	git(t, dir, "checkout", "--detach")
	if got := Branch(dir); got != "" {
		t.Errorf("detached: got %q, want empty", got)
	}
	plain := t.TempDir()
	if got := Branch(plain); got != "" {
		t.Errorf("plain dir: got %q, want empty", got)
	}
	if got := Branch(plain + "/missing"); got != "" {
		t.Errorf("missing dir: got %q, want empty", got)
	}
}

func TestIdentity(t *testing.T) {
	dir := newRepo(t)
	name, email := Identity(dir)
	if name != "Ada Lovelace" || email != "ada@x.org" {
		t.Errorf("got %q, %q", name, email)
	}

	git(t, dir, "config", "--unset", "user.email")
	name, email = Identity(dir)
	if name != "Ada Lovelace" || email != "" {
		t.Errorf("name only: got %q, %q", name, email)
	}

	plain := t.TempDir()
	name, email = Identity(plain)
	if name != "" || email != "" {
		t.Errorf("plain dir: got %q, %q", name, email)
	}
	if _, err := os.Stat(plain); err != nil {
		t.Fatal(err)
	}
}

func TestParseAuthor(t *testing.T) {
	cases := []struct{ in, name, email string }{
		{"Ada <ada@x.org>", "Ada", "ada@x.org"},
		{"  Ada Lovelace  <  ada@x.org > ", "Ada Lovelace", "ada@x.org"},
		{"Ada", "Ada", ""},
		{"  Ada  ", "Ada", ""},
		{"<a@b>", "", "a@b"},
		{"", "", ""},
	}
	for _, c := range cases {
		name, email := ParseAuthor(c.in)
		if name != c.name || email != c.email {
			t.Errorf("ParseAuthor(%q) = %q, %q; want %q, %q", c.in, name, email, c.name, c.email)
		}
	}
}

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func gitignoreInitArgs(extra ...string) []string {
	return append([]string{"init", "--name", "tidepool", "--repos", "server,client"}, extra...)
}

const bakLine = ".cairn/*.bak"

func TestEnsureGitignoreCreatesFile(t *testing.T) {
	root := t.TempDir()
	changed, err := ensureGitignore(root)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	b, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if string(b) != bakLine+"\n" {
		t.Errorf("content %q", b)
	}
	fi, _ := os.Stat(filepath.Join(root, ".gitignore"))
	if fi.Mode().Perm() != 0o644 {
		t.Errorf("mode %v", fi.Mode().Perm())
	}
}

func TestEnsureGitignoreAppends(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"node_modules\n", "node_modules\n" + bakLine + "\n"},
		{"node_modules", "node_modules\n" + bakLine + "\n"},
	} {
		root := t.TempDir()
		p := filepath.Join(root, ".gitignore")
		os.WriteFile(p, []byte(tc.in), 0o644)
		if changed, err := ensureGitignore(root); err != nil || !changed {
			t.Fatalf("changed=%v err=%v", changed, err)
		}
		b, _ := os.ReadFile(p)
		if string(b) != tc.want {
			t.Errorf("content %q, want %q", b, tc.want)
		}
		if changed, err := ensureGitignore(root); err != nil || changed {
			t.Errorf("second run changed=%v err=%v", changed, err)
		}
		if b2, _ := os.ReadFile(p); string(b2) != tc.want {
			t.Errorf("second run rewrote %q", b2)
		}
	}
	// an identical line with a carriage return counts as present
	root := t.TempDir()
	p := filepath.Join(root, ".gitignore")
	os.WriteFile(p, []byte("a\r\n"+bakLine+"\r\n"), 0o644)
	if changed, err := ensureGitignore(root); err != nil || changed {
		t.Errorf("crlf changed=%v err=%v", changed, err)
	}
}

func TestEnsureGitignoreRefusesSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	os.WriteFile(target, []byte("keep\n"), 0o644)
	if err := os.Symlink(target, filepath.Join(root, ".gitignore")); err != nil {
		t.Skip(err)
	}
	if _, err := ensureGitignore(root); err == nil {
		t.Fatal("symlink accepted")
	}
	if b, _ := os.ReadFile(target); string(b) != "keep\n" {
		t.Errorf("target changed: %q", b)
	}
}

func TestInitProjectModeWritesGitignore(t *testing.T) {
	e := newEnv(t)
	r := e.run(gitignoreInitArgs()...)
	b, _ := os.ReadFile(filepath.Join(e.root, ".gitignore"))
	if !strings.Contains(string(b), bakLine+"\n") {
		t.Errorf(".gitignore %q", b)
	}
	if !strings.Contains(r.out, ".gitignore") || !strings.Contains(r.out, bakLine) {
		t.Errorf("stdout does not mention it:\n%s", r.out)
	}
	m := newEnv(t)
	m.initProject("--mode", "machine")
	if exists(filepath.Join(m.root, ".gitignore")) {
		t.Error("machine mode created .gitignore")
	}
}

func TestInitLocalChildWritesGitignore(t *testing.T) {
	e := newEnv(t)
	e.initProject()
	local := realTemp(t)
	if r := e.runIn(local, "", "init", "--project", "tidepool", "--repo", "web", "--local"); r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.err)
	}
	b, _ := os.ReadFile(filepath.Join(local, ".gitignore"))
	if string(b) != bakLine+"\n" {
		t.Errorf("local .gitignore %q", b)
	}
	att := realTemp(t)
	if r := e.runIn(att, "", "init", "--project", "tidepool", "--repo", "api"); r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.err)
	}
	if exists(filepath.Join(att, ".gitignore")) {
		t.Error("attached child got a .gitignore")
	}
}

func TestInitSurvivesGitignoreFailure(t *testing.T) {
	e := newEnv(t)
	// a folder in place of the file cannot be used (not a regular file)
	os.Mkdir(filepath.Join(e.root, ".gitignore"), 0o755)
	r := e.run(gitignoreInitArgs()...)
	if r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.err)
	}
	if !strings.Contains(r.err, "cairn: warning") || !strings.Contains(r.err, bakLine) {
		t.Errorf("stderr:\n%s", r.err)
	}
	if !exists(filepath.Join(e.root, ".cairn", "cairn.db")) {
		t.Error("no database")
	}
}

func TestInitChildRollbackKeepsUserGitignore(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permissions do not bind root")
	}
	e := newEnv(t)
	e.initProject()
	// an unwritable registry folder makes the local child fail after its .gitignore
	rp := e.home
	for name, pre := range map[string]string{"created": "", "existing": "dist\n"} {
		child := realTemp(t)
		gi := filepath.Join(child, ".gitignore")
		if pre != "" {
			os.WriteFile(gi, []byte(pre), 0o644)
		}
		os.Chmod(rp, 0o500)
		r := e.runIn(child, "", "init", "--project", "tidepool", "--repo", "web-"+name, "--local")
		os.Chmod(rp, 0o700)
		if r.code != 1 {
			t.Fatalf("%s: exit %d, want 1\n%s", name, r.code, r.err)
		}
		b, err := os.ReadFile(gi)
		if name == "created" && err == nil {
			t.Errorf("created .gitignore left behind: %q", b)
		}
		if name == "existing" && !strings.HasPrefix(string(b), pre) {
			t.Errorf("user .gitignore lost lines: %q", b)
		}
	}
}

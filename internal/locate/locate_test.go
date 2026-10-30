package locate

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/R0b-1ns/cairn/internal/registry"
)

// tempBase returns t.TempDir() with symlinks resolved (macOS /var is a symlink).
// Each test calls it once at the start.
func tempBase(t *testing.T) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks(TempDir): %v", err)
	}
	return base
}

// newReg opens a registry in <base>/home. Nothing is created until a write.
func newReg(base string) *registry.Registry {
	return registry.Open(filepath.Join(base, "home"))
}

func mkdirAll(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", p, err)
	}
}

func stubFile(t *testing.T, p string) {
	t.Helper()
	mkdirAll(t, filepath.Dir(p))
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", p, err)
	}
}

// stubDB creates an empty <root>/.cairn/cairn.db.
func stubDB(t *testing.T, root string) {
	t.Helper()
	stubFile(t, filepath.Join(root, ".cairn", "cairn.db"))
}

// registerMachine registers a machine-mode project and stubs its machine database.
func registerMachine(t *testing.T, reg *registry.Registry, name, path string) {
	t.Helper()
	if err := reg.Register(registry.Entry{Name: name, Mode: "machine", Path: path}); err != nil {
		t.Fatalf("Register %s: %v", name, err)
	}
	stubFile(t, reg.MachineDBPath(name))
}

// snapshot lists every path under root, for detecting created files or dirs.
func snapshot(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		out = append(out, p)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir(%s): %v", root, err)
	}
	return out
}

func TestWalkUp(t *testing.T) {
	base := tempBase(t)
	root := filepath.Join(base, "root")
	stubDB(t, root)
	cwd := filepath.Join(root, "a", "b", "c")
	mkdirAll(t, cwd)

	loc, err := Find(cwd, "", newReg(base))
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if loc.Mode != "project" {
		t.Errorf("Mode = %q, want project", loc.Mode)
	}
	if loc.Root != root {
		t.Errorf("Root = %q, want %q", loc.Root, root)
	}
	if want := filepath.Join(root, ".cairn", "cairn.db"); loc.DBPath != want {
		t.Errorf("DBPath = %q, want %q", loc.DBPath, want)
	}
}

func TestWalkUpStopsAtRoot(t *testing.T) {
	base := tempBase(t)
	tree := filepath.Join(base, "tree")
	cwd := filepath.Join(tree, "a", "b")
	mkdirAll(t, cwd)
	reg := newReg(base)

	before := snapshot(t, base)
	_, err := Find(cwd, "", reg)
	if !errors.Is(err, ErrNoProject) {
		t.Fatalf("Find error = %v, want ErrNoProject", err)
	}
	after := snapshot(t, base)

	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("Find created or changed files under %s\nbefore: %v\nafter:  %v", base, before, after)
	}
	if _, err := os.Stat(filepath.Join(base, "home")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("registry home was created (stat err = %v)", err)
	}
}

func TestRejectsSymlinkedStore(t *testing.T) {
	base := tempBase(t)

	t.Run("symlinked .cairn folder", func(t *testing.T) {
		root := filepath.Join(base, "a")
		other := filepath.Join(root, "elsewhere")
		mkdirAll(t, other)
		stubFile(t, filepath.Join(other, "cairn.db"))
		if err := os.Symlink(other, filepath.Join(root, ".cairn")); err != nil {
			t.Fatalf("Symlink: %v", err)
		}
		sub := filepath.Join(root, "sub")
		mkdirAll(t, sub)

		_, err := Find(sub, "", newReg(base))
		if err == nil {
			t.Fatalf("Find returned nil error for symlinked .cairn folder")
		}
		if errors.Is(err, ErrNoProject) {
			t.Errorf("Find error = %v, must not be ErrNoProject (symlink must be an error, not skipped)", err)
		}
		if !strings.Contains(strings.ToLower(err.Error()), "symlink") {
			t.Errorf("error %q does not mention symlink", err)
		}
	})

	t.Run("symlinked cairn.db file", func(t *testing.T) {
		root := filepath.Join(base, "b")
		store := filepath.Join(root, ".cairn")
		mkdirAll(t, store)
		target := filepath.Join(root, "elsewhere.db")
		stubFile(t, target)
		if err := os.Symlink(target, filepath.Join(store, "cairn.db")); err != nil {
			t.Fatalf("Symlink: %v", err)
		}
		sub := filepath.Join(root, "sub")
		mkdirAll(t, sub)

		_, err := Find(sub, "", newReg(base))
		if err == nil {
			t.Fatalf("Find returned nil error for symlinked cairn.db")
		}
		if errors.Is(err, ErrNoProject) {
			t.Errorf("Find error = %v, must not be ErrNoProject (symlink must be an error, not skipped)", err)
		}
		if !strings.Contains(strings.ToLower(err.Error()), "symlink") {
			t.Errorf("error %q does not mention symlink", err)
		}
	})
}

func TestLongestMatch(t *testing.T) {
	base := tempBase(t)
	a := filepath.Join(base, "A")
	b := filepath.Join(a, "b")
	mkdirAll(t, filepath.Join(b, "c"))
	mkdirAll(t, filepath.Join(a, "x"))
	reg := newReg(base)
	registerMachine(t, reg, "tidepool", a)
	registerMachine(t, reg, "harbor", b)

	loc, err := Find(filepath.Join(b, "c"), "", reg)
	if err != nil {
		t.Fatalf("Find in A/b/c: %v", err)
	}
	if loc.Name != "harbor" || loc.Root != b {
		t.Errorf("Find A/b/c = (name %q, root %q), want (harbor, %q)", loc.Name, loc.Root, b)
	}

	loc, err = Find(filepath.Join(a, "x"), "", reg)
	if err != nil {
		t.Fatalf("Find in A/x: %v", err)
	}
	if loc.Name != "tidepool" || loc.Root != a {
		t.Errorf("Find A/x = (name %q, root %q), want (tidepool, %q)", loc.Name, loc.Root, a)
	}

	outside := tempBase(t)
	if _, err := Find(outside, "", reg); !errors.Is(err, ErrNoProject) {
		t.Errorf("Find outside error = %v, want ErrNoProject", err)
	}
}

func TestSymlinkedCwdSameProject(t *testing.T) {
	base := tempBase(t)
	p := filepath.Join(base, "P")
	sub := filepath.Join(p, "sub")
	mkdirAll(t, sub)
	link := filepath.Join(base, "L")
	if err := os.Symlink(p, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	check := func(t *testing.T, reg *registry.Registry) {
		t.Helper()
		want, err := Find(filepath.Join(p, "sub"), "", reg)
		if err != nil {
			t.Fatalf("Find(P/sub): %v", err)
		}
		got, err := Find(filepath.Join(link, "sub"), "", reg)
		if err != nil {
			t.Fatalf("Find(L/sub): %v", err)
		}
		if got != want {
			t.Errorf("Find(L/sub) = %+v, want same as Find(P/sub) = %+v", got, want)
		}
		if got.Root != p {
			t.Errorf("Root = %q, want %q", got.Root, p)
		}
	}

	t.Run("plain path", func(t *testing.T) {
		reg := newReg(base)
		registerMachine(t, reg, "tidepool", p)
		check(t, reg)
	})

	t.Run("trailing slash", func(t *testing.T) {
		reg := registry.Open(filepath.Join(base, "home-slash"))
		registerMachine(t, reg, "tidepool", p+"/")
		check(t, reg)
	})
}

func TestDeeperWins(t *testing.T) {
	base := tempBase(t)

	t.Run("project deeper than machine", func(t *testing.T) {
		a := filepath.Join(base, "one")
		cwd := filepath.Join(a, "b", "c")
		mkdirAll(t, cwd)
		reg := registry.Open(filepath.Join(base, "home1"))
		registerMachine(t, reg, "tidepool", a)
		stubDB(t, filepath.Join(a, "b"))

		loc, err := Find(cwd, "", reg)
		if err != nil {
			t.Fatalf("Find: %v", err)
		}
		if loc.Mode != "project" || loc.Root != filepath.Join(a, "b") {
			t.Errorf("Find = (mode %q, root %q), want (project, %q)", loc.Mode, loc.Root, filepath.Join(a, "b"))
		}
	})

	t.Run("machine deeper than project", func(t *testing.T) {
		a := filepath.Join(base, "two")
		m := filepath.Join(a, "b", "c")
		cwd := filepath.Join(m, "d")
		mkdirAll(t, cwd)
		reg := registry.Open(filepath.Join(base, "home2"))
		registerMachine(t, reg, "harbor", m)
		stubDB(t, a)

		loc, err := Find(cwd, "", reg)
		if err != nil {
			t.Fatalf("Find: %v", err)
		}
		if loc.Mode != "machine" || loc.Root != m {
			t.Errorf("Find = (mode %q, root %q), want (machine, %q)", loc.Mode, loc.Root, m)
		}
	})

	t.Run("same depth project wins", func(t *testing.T) {
		r := filepath.Join(base, "three")
		sub := filepath.Join(r, "sub")
		mkdirAll(t, sub)
		stubDB(t, r)
		reg := registry.Open(filepath.Join(base, "home3"))
		registerMachine(t, reg, "tidepool", r)

		loc, err := Find(sub, "", reg)
		if err != nil {
			t.Fatalf("Find: %v", err)
		}
		if loc.Mode != "project" {
			t.Errorf("Mode = %q, want project at equal depth", loc.Mode)
		}
	})
}

func TestProjectNameOverrides(t *testing.T) {
	base := tempBase(t)
	other := filepath.Join(base, "elsewhere")
	mkdirAll(t, other)
	reg := newReg(base)

	h := filepath.Join(base, "harbor-dir")
	mkdirAll(t, h)
	registerMachine(t, reg, "harbor", h)

	root := filepath.Join(base, "proj-root")
	stubDB(t, root)
	if err := reg.Register(registry.Entry{Name: "tidepool", Mode: "project", Path: root}); err != nil {
		t.Fatalf("Register tidepool: %v", err)
	}

	for _, cwd := range []string{other, h} {
		loc, err := Find(cwd, "harbor", reg)
		if err != nil {
			t.Fatalf("Find(%s, harbor): %v", cwd, err)
		}
		if loc.Mode != "machine" || loc.Root != h || loc.Name != "harbor" {
			t.Errorf("Find(%s, harbor) = %+v, want machine harbor at %s", cwd, loc, h)
		}
		if want := reg.MachineDBPath("harbor"); loc.DBPath != want {
			t.Errorf("DBPath = %q, want %q", loc.DBPath, want)
		}
	}

	if _, err := Find(other, "nosuch", reg); !errors.Is(err, ErrNoProject) {
		t.Errorf("Find unknown name error = %v, want ErrNoProject", err)
	}

	loc, err := Find(other, "tidepool", reg)
	if err != nil {
		t.Fatalf("Find(tidepool): %v", err)
	}
	if want := filepath.Join(root, ".cairn", "cairn.db"); loc.DBPath != want {
		t.Errorf("project-mode DBPath = %q, want %q", loc.DBPath, want)
	}
	if loc.Name != "tidepool" {
		t.Errorf("Name = %q, want tidepool", loc.Name)
	}
}

func TestCaseInsensitivePath(t *testing.T) {
	base := tempBase(t)
	projDir := filepath.Join(base, "Proj")
	mkdirAll(t, filepath.Join(projDir, "sub"))

	if _, err := os.Stat(filepath.Join(base, "proj")); err != nil {
		t.Skip("case-sensitive filesystem: lowercase variant does not exist")
	}

	reg := newReg(base)
	registerMachine(t, reg, "tidepool", projDir)

	loc, err := Find(filepath.Join(base, "proj", "sub"), "", reg)
	if err != nil {
		t.Fatalf("Find with lowercase cwd: %v", err)
	}
	if loc.Mode != "machine" {
		t.Errorf("Mode = %q, want machine", loc.Mode)
	}
	if !strings.EqualFold(loc.Root, projDir) {
		t.Errorf("Root = %q, want %q (case-insensitively)", loc.Root, projDir)
	}
}

func TestUnreadableAncestorSkipped(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits are not enforced")
	}
	base := tempBase(t)
	stubDB(t, base)
	locked := filepath.Join(base, "locked")
	cwd := filepath.Join(locked, "sub", "deep")
	mkdirAll(t, cwd)

	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	loc, err := Find(cwd, "", newReg(base))
	if err != nil && !errors.Is(err, ErrNoProject) {
		t.Fatalf("Find under unreadable ancestor error = %v, want nil or ErrNoProject", err)
	}
	if err == nil && loc.Mode == "" {
		t.Errorf("Find returned an empty Location without error")
	}
}

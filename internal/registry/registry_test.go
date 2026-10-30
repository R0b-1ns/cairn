package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

const registryFile = "registry.json"

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

func evalPath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", p, err)
	}
	return r
}

func mkdirAll(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", p, err)
	}
}

func writeFile(t *testing.T, p string, data []byte) {
	t.Helper()
	mkdirAll(t, filepath.Dir(p))
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", p, err)
	}
}

// assertNothingWritten checks that home is missing or empty.
func assertNothingWritten(t *testing.T, home string) {
	t.Helper()
	entries, err := os.ReadDir(home)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", home, err)
	}
	if len(entries) != 0 {
		t.Errorf("home %s has %d entries after rejected write, want none", home, len(entries))
	}
}

func TestNameTraversalRejected(t *testing.T) {
	base := tempBase(t)
	home := filepath.Join(base, "home")
	proj := filepath.Join(base, "proj")
	mkdirAll(t, proj)
	reg := Open(home)

	for _, name := range []string{"../x", "a/b", "", "UPPER", ".hidden"} {
		err := reg.Register(Entry{Name: name, Mode: "machine", Path: proj})
		if err == nil {
			t.Errorf("Register(%q) succeeded, want error", name)
		}
	}

	list, err := reg.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List has %d entries after rejected names, want 0", len(list))
	}
	assertNothingWritten(t, home)
}

func TestPathNormalized(t *testing.T) {
	base := tempBase(t)
	home := filepath.Join(base, "home")
	proj := filepath.Join(base, "proj")
	sub := filepath.Join(proj, "sub")
	other := filepath.Join(base, "other")
	link := filepath.Join(base, "link")
	mkdirAll(t, sub)
	mkdirAll(t, other)
	if err := os.Symlink(other, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	wantProj := evalPath(t, proj)
	wantOther := evalPath(t, other)

	reg := Open(home)
	messy := proj + "/sub/../" // trailing slash and a sub/.. segment
	if err := reg.Register(Entry{Name: "tidepool", Mode: "machine", Path: messy}); err != nil {
		t.Fatalf("Register tidepool: %v", err)
	}
	if err := reg.Register(Entry{Name: "harbor", Mode: "machine", Path: link}); err != nil {
		t.Fatalf("Register harbor via symlink: %v", err)
	}

	list, err := reg.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := map[string]string{}
	for _, e := range list {
		got[e.Name] = e.Path
	}
	if len(got) != 2 {
		t.Fatalf("List returned %d entries, want 2", len(got))
	}
	if got["tidepool"] != wantProj {
		t.Errorf("tidepool path = %q, want %q", got["tidepool"], wantProj)
	}
	if got["harbor"] != wantOther {
		t.Errorf("harbor path = %q, want symlink target %q", got["harbor"], wantOther)
	}

	// A relative path must be stored as an absolute path.
	t.Chdir(base)
	rel := filepath.Join(base, "rel")
	mkdirAll(t, rel)
	reg2 := Open(filepath.Join(base, "home2"))
	if err := reg2.Register(Entry{Name: "harbor", Mode: "machine", Path: "rel"}); err != nil {
		t.Fatalf("Register relative path: %v", err)
	}
	e, err := reg2.Get("harbor")
	if err != nil {
		t.Fatalf("Get harbor: %v", err)
	}
	if !filepath.IsAbs(e.Path) {
		t.Errorf("relative path stored as %q, want absolute", e.Path)
	}
	if want := evalPath(t, rel); e.Path != want {
		t.Errorf("relative path stored as %q, want %q", e.Path, want)
	}
}

func TestDuplicateNameOrPath(t *testing.T) {
	base := tempBase(t)
	a := filepath.Join(base, "a")
	b := filepath.Join(base, "b")
	link := filepath.Join(base, "link")
	mkdirAll(t, a)
	mkdirAll(t, b)
	if err := os.Symlink(a, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	cases := []struct {
		name   string
		second Entry
	}{
		{"same name", Entry{Name: "tidepool", Mode: "machine", Path: b}},
		{"same path", Entry{Name: "harbor", Mode: "machine", Path: a}},
		{"same path via symlink", Entry{Name: "harbor", Mode: "machine", Path: link}},
		{"same path trailing slash", Entry{Name: "harbor", Mode: "machine", Path: a + "/"}},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := Open(filepath.Join(base, fmt.Sprintf("home%d", i)))
			if err := reg.Register(Entry{Name: "tidepool", Mode: "machine", Path: a}); err != nil {
				t.Fatalf("first Register: %v", err)
			}
			err := reg.Register(tc.second)
			if !errors.Is(err, ErrExists) {
				t.Errorf("second Register error = %v, want ErrExists", err)
			}
		})
	}
}

func TestMoveRemove(t *testing.T) {
	base := tempBase(t)
	home := filepath.Join(base, "home")
	a := filepath.Join(base, "a")
	b := filepath.Join(base, "b")
	c := filepath.Join(base, "c")
	mkdirAll(t, a)
	mkdirAll(t, b)
	mkdirAll(t, c)
	reg := Open(home)

	if err := reg.Register(Entry{Name: "tidepool", Mode: "machine", Path: a}); err != nil {
		t.Fatalf("Register tidepool: %v", err)
	}
	if err := reg.Register(Entry{Name: "harbor", Mode: "machine", Path: c}); err != nil {
		t.Fatalf("Register harbor: %v", err)
	}

	if err := reg.Move("tidepool", b); err != nil {
		t.Fatalf("Move: %v", err)
	}
	e, err := reg.Get("tidepool")
	if err != nil {
		t.Fatalf("Get after Move: %v", err)
	}
	if want := evalPath(t, b); e.Path != want {
		t.Errorf("Path after Move = %q, want %q", e.Path, want)
	}

	if err := reg.Move("nope", b); !errors.Is(err, ErrNotFound) {
		t.Errorf("Move unknown name error = %v, want ErrNotFound", err)
	}
	if err := reg.Move("tidepool", c); !errors.Is(err, ErrExists) {
		t.Errorf("Move onto another project's path error = %v, want ErrExists", err)
	}

	keep := filepath.Join(b, "keep.txt")
	db := filepath.Join(b, ".cairn", "cairn.db")
	writeFile(t, keep, []byte("keep"))
	writeFile(t, db, nil)

	if err := reg.Remove("tidepool"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := reg.Get("tidepool"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Remove error = %v, want ErrNotFound", err)
	}
	for _, p := range []string{keep, db} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("Remove touched the project folder: %s: %v", p, err)
		}
	}
	if err := reg.Remove("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Remove unknown error = %v, want ErrNotFound", err)
	}
}

func TestConcurrentRegister(t *testing.T) {
	base := tempBase(t)
	home := filepath.Join(base, "home")
	const n = 20

	paths := make([]string, n)
	for i := range paths {
		paths[i] = filepath.Join(base, fmt.Sprintf("p%02d", i))
		mkdirAll(t, paths[i])
	}

	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("p%02d", i)
			errs[i] = Open(home).Register(Entry{Name: name, Mode: "machine", Path: paths[i]})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("Register p%02d: %v", i, err)
		}
	}

	list, err := Open(home).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != n {
		t.Fatalf("List returned %d entries, want %d", len(list), n)
	}
	names := map[string]bool{}
	for _, e := range list {
		names[e.Name] = true
	}
	for i := 0; i < n; i++ {
		if name := fmt.Sprintf("p%02d", i); !names[name] {
			t.Errorf("missing entry %s after concurrent Register", name)
		}
	}

	data, err := os.ReadFile(filepath.Join(home, registryFile))
	if err != nil {
		t.Fatalf("ReadFile registry.json: %v", err)
	}
	if !json.Valid(data) {
		t.Errorf("registry.json is not valid JSON after concurrent writes: %s", data)
	}
}

func TestFileModes(t *testing.T) {
	base := tempBase(t)
	home := filepath.Join(base, "newhome")
	proj := filepath.Join(base, "proj")
	mkdirAll(t, proj)

	if err := Open(home).Register(Entry{Name: "tidepool", Mode: "machine", Path: proj}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	fi, err := os.Stat(home)
	if err != nil {
		t.Fatalf("Stat home: %v", err)
	}
	if got := fi.Mode().Perm(); got != 0o700 {
		t.Errorf("home mode = %o, want 0700", got)
	}

	fi, err = os.Stat(filepath.Join(home, registryFile))
	if err != nil {
		t.Fatalf("Stat registry.json: %v", err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("registry.json mode = %o, want 0600", got)
	}
}

func TestListEmptyWhenNoFile(t *testing.T) {
	base := tempBase(t)
	home := filepath.Join(base, "missing")

	list, err := Open(home).List()
	if err != nil {
		t.Fatalf("List on missing home: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List returned %d entries, want 0", len(list))
	}
	if _, err := os.Stat(home); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("List created the home directory (stat err = %v)", err)
	}
}

func TestCorruptRegistryNotOverwritten(t *testing.T) {
	base := tempBase(t)
	home := filepath.Join(base, "home")
	proj := filepath.Join(base, "proj")
	mkdirAll(t, proj)
	garbage := []byte("{this is not json")
	file := filepath.Join(home, registryFile)
	writeFile(t, file, garbage)
	reg := Open(home)

	if _, err := reg.List(); err == nil {
		t.Errorf("List on corrupt registry returned nil error")
	}
	if err := reg.Register(Entry{Name: "tidepool", Mode: "machine", Path: proj}); err == nil {
		t.Errorf("Register on corrupt registry returned nil error")
	}

	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(garbage) {
		t.Errorf("corrupt registry.json was overwritten: got %q, want %q", got, garbage)
	}
}

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestHome(t *testing.T) {
	base := tempBase(t)

	t.Run("CAIRN_HOME wins", func(t *testing.T) {
		got, err := Home(envFrom(map[string]string{"CAIRN_HOME": "/x/y", "HOME": base}))
		if err != nil {
			t.Fatalf("Home: %v", err)
		}
		if got != "/x/y" {
			t.Errorf("Home = %q, want %q", got, "/x/y")
		}
	})

	if runtime.GOOS == "darwin" {
		t.Run("darwin default", func(t *testing.T) {
			got, err := Home(envFrom(map[string]string{"HOME": base}))
			if err != nil {
				t.Fatalf("Home: %v", err)
			}
			if want := base + "/Library/Application Support/cairn"; got != want {
				t.Errorf("Home = %q, want %q", got, want)
			}
		})
		return
	}

	t.Run("XDG_DATA_HOME", func(t *testing.T) {
		got, err := Home(envFrom(map[string]string{"HOME": base, "XDG_DATA_HOME": "/xdg"}))
		if err != nil {
			t.Fatalf("Home: %v", err)
		}
		if got != "/xdg/cairn" {
			t.Errorf("Home = %q, want %q", got, "/xdg/cairn")
		}
	})

	t.Run("HOME fallback", func(t *testing.T) {
		got, err := Home(envFrom(map[string]string{"HOME": base}))
		if err != nil {
			t.Fatalf("Home: %v", err)
		}
		if want := base + "/.local/share/cairn"; got != want {
			t.Errorf("Home = %q, want %q", got, want)
		}
	})
}

func TestMachineDBPath(t *testing.T) {
	base := tempBase(t)
	home := filepath.Join(base, "home")
	reg := Open(home)

	want := filepath.Join(home, "projects", "tidepool", "cairn.db")
	if got := reg.MachineDBPath("tidepool"); got != want {
		t.Errorf("MachineDBPath(tidepool) = %q, want %q", got, want)
	}
}

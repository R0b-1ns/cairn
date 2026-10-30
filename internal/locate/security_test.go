package locate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/R0b-1ns/cairn/internal/registry"
)

func TestRejectsForeignOwnerStore(t *testing.T) {
	base := tempBase(t)
	root := filepath.Join(base, "shared")
	stubDB(t, root)
	mkdirAll(t, filepath.Join(root, "work"))
	orig := uidFunc
	t.Cleanup(func() { uidFunc = orig })
	uidFunc = func() int { return os.Geteuid() + 4242 } // pretend another user runs cairn
	_, err := Find(filepath.Join(root, "work"), "", newReg(base))
	if err == nil || errors.Is(err, ErrNoProject) || !strings.Contains(err.Error(), "owned by") {
		t.Fatalf("want ownership error, got %v", err)
	}
}

func TestRootMayUseAnyOwner(t *testing.T) {
	base := tempBase(t)
	stubDB(t, base)
	orig := uidFunc
	t.Cleanup(func() { uidFunc = orig })
	uidFunc = func() int { return 0 }
	if _, err := Find(base, "", newReg(base)); err != nil {
		t.Fatalf("root must be accepted: %v", err)
	}
}

func TestRejectsWorldWritableStore(t *testing.T) {
	base := tempBase(t)
	stubDB(t, base)
	if err := os.Chmod(filepath.Join(base, ".cairn"), 0o777); err != nil {
		t.Fatal(err)
	}
	if _, err := Find(base, "", newReg(base)); err == nil || !strings.Contains(err.Error(), "writable") {
		t.Fatalf("want world-writable error, got %v", err)
	}
	if err := os.Chmod(filepath.Join(base, ".cairn"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(base, ".cairn", "cairn.db"), 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := Find(base, "", newReg(base)); err == nil || !strings.Contains(err.Error(), "writable") {
		t.Fatalf("want world-writable db error, got %v", err)
	}
}

func TestProjectNameChecksSymlinkedStore(t *testing.T) {
	base := tempBase(t)
	real := filepath.Join(base, "real")
	stubDB(t, real)
	proj := filepath.Join(base, "proj")
	mkdirAll(t, proj)
	if err := os.Symlink(filepath.Join(real, ".cairn"), filepath.Join(proj, ".cairn")); err != nil {
		t.Fatal(err)
	}
	reg := newReg(base)
	if err := reg.Register(registry.Entry{Name: "tidepool", Mode: "project", Path: proj}); err != nil {
		t.Fatal(err)
	}
	if _, err := Find(base, "tidepool", reg); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("want symlink error by name, got %v", err)
	}
	// A registered project-mode folder without a store is "no project", not a path to nowhere.
	empty := filepath.Join(base, "empty")
	mkdirAll(t, empty)
	if err := reg.Register(registry.Entry{Name: "harbor", Mode: "project", Path: empty}); err != nil {
		t.Fatal(err)
	}
	if _, err := Find(base, "harbor", reg); !errors.Is(err, ErrNoProject) {
		t.Fatalf("want ErrNoProject, got %v", err)
	}
}

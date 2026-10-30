// Package locate finds the cairn project for the current directory.
package locate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/registry"
)

// ErrNoProject means no project was found; commands must not create one.
var ErrNoProject = errors.New("no cairn project found")

// Location says where a project's database is.
type Location struct {
	Name   string // registry name when known, else ""
	Mode   string // "project" or "machine"
	Root   string // project root folder
	DBPath string
}

// uidFunc returns the effective user ID; tests replace it.
var uidFunc = os.Geteuid

// checkTrusted refuses a store entry that another user could have planted or
// could modify: it must be owned by the current user (root excepted) and must
// not be world-writable.
func checkTrusted(path string, fi os.FileInfo) error {
	if fi.Mode().Perm()&0o002 != 0 {
		return fmt.Errorf("refusing %s: it is world-writable", path)
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		if uid := uidFunc(); uid != 0 && int(st.Uid) != uid {
			return fmt.Errorf("refusing %s: owned by another user (uid %d)", path, st.Uid)
		}
	}
	return nil
}

// CheckStore reports the project-mode database of a project root. found is false
// when root has no .cairn folder (or it cannot be read); an error is returned when
// .cairn or cairn.db exists but is a symlink or not the expected kind of file.
func CheckStore(root string) (dbPath string, found bool, err error) {
	dir := filepath.Join(root, ".cairn")
	fi, err := os.Lstat(dir)
	if err != nil {
		return "", false, nil
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return "", false, fmt.Errorf("refusing symlinked store folder %s", dir)
	}
	if !fi.IsDir() {
		return "", false, fmt.Errorf("%s exists but is not a folder", dir)
	}
	if err := checkTrusted(dir, fi); err != nil {
		return "", false, err
	}
	db := filepath.Join(dir, "cairn.db")
	fi, err = os.Lstat(db)
	if err != nil {
		return "", false, nil
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return "", false, fmt.Errorf("refusing symlinked database %s", db)
	}
	if !fi.Mode().IsRegular() {
		return "", false, fmt.Errorf("%s is not a regular file", db)
	}
	if err := checkTrusted(db, fi); err != nil {
		return "", false, err
	}
	return db, true, nil
}

func samePath(a, b string) bool {
	if a == b {
		return true
	}
	ai, err1 := os.Stat(a)
	bi, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ai, bi)
}

func fromEntry(reg *registry.Registry, e registry.Entry) (Location, error) {
	if e.Mode == model.ModeProject {
		db, found, err := CheckStore(e.Path)
		if err != nil {
			return Location{}, err
		}
		if !found {
			return Location{}, fmt.Errorf("%w: no .cairn/cairn.db at %s (project %q)", ErrNoProject, e.Path, e.Name)
		}
		return Location{Name: e.Name, Mode: e.Mode, Root: e.Path, DBPath: db}, nil
	}
	return Location{Name: e.Name, Mode: e.Mode, Root: e.Path, DBPath: reg.MachineDBPath(e.Name)}, nil
}

// Find locates the project for cwd. A non-empty name (from --project or
// CAIRN_PROJECT) selects a registered project directly. Otherwise it walks up
// from cwd and the deepest match between a real .cairn/cairn.db and a
// registered machine-mode project path wins; at equal depth .cairn wins.
// A symlinked .cairn folder or cairn.db is an error, never skipped.
func Find(cwd, name string, reg *registry.Registry) (Location, error) {
	if name != "" {
		e, err := reg.Get(name)
		if errors.Is(err, registry.ErrNotFound) {
			return Location{}, fmt.Errorf("%w: %q is not a registered project", ErrNoProject, name)
		}
		if err != nil {
			return Location{}, err
		}
		return fromEntry(reg, e)
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return Location{}, err
	}
	dir, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// e.g. an unreadable ancestor: walk the plain absolute path instead.
		dir = filepath.Clean(abs)
	}
	entries, listErr := reg.List()
	for {
		if db, found, err := CheckStore(dir); err != nil {
			return Location{}, err
		} else if found {
			loc := Location{Mode: model.ModeProject, Root: dir, DBPath: db}
			for _, e := range entries {
				if e.Mode == model.ModeProject && samePath(e.Path, dir) {
					loc.Name = e.Name
				}
			}
			return loc, nil
		}
		for _, e := range entries {
			if e.Mode == model.ModeMachine && samePath(e.Path, dir) {
				return fromEntry(reg, e)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if listErr != nil {
		return Location{}, listErr
	}
	return Location{}, fmt.Errorf("%w from %s", ErrNoProject, cwd)
}

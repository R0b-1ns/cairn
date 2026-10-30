// Package locate finds the cairn project for the current directory.
package locate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

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

func fromEntry(reg *registry.Registry, e registry.Entry) Location {
	db := reg.MachineDBPath(e.Name)
	if e.Mode == model.ModeProject {
		db = filepath.Join(e.Path, ".cairn", "cairn.db")
	}
	return Location{Name: e.Name, Mode: e.Mode, Root: e.Path, DBPath: db}
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
		return fromEntry(reg, e), nil
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
				return fromEntry(reg, e), nil
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

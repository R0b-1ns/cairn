// Package locate finds the cairn project for the current directory.
package locate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

	// Set only for an attached child: Name, Mode, Root and DBPath above are
	// the parent's; Repo and ChildRoot describe the child folder.
	Attached  bool
	Repo      string // the child's repo name
	ChildRoot string // folder holding the child's pointer
	ParentID  string // parent_id recorded in the child's pointer (may be empty)
}

// uidFunc returns the effective user ID; tests replace it.
var uidFunc = os.Geteuid

// sudoUID returns the invoking user when running under sudo; tests replace it.
var sudoUID = func() string { return os.Getenv("SUDO_UID") }

// checkTrusted refuses a store entry that another user could have planted or
// could modify: it must be owned by the current user (root excepted) and must
// not be world-writable.
func checkTrusted(path string, fi os.FileInfo) error {
	if fi.Mode().Perm()&0o002 != 0 {
		return fmt.Errorf("refusing %s: it is world-writable", path)
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		uid := uidFunc()
		owner := int(st.Uid)
		trusted := owner == uid
		if uid == 0 && !trusted {
			// Root, for example under sudo, trusts only the invoking user's stores.
			trusted = sudoUID() == strconv.Itoa(owner)
		}
		if !trusted {
			return fmt.Errorf("refusing %s: owned by another user (uid %d); run cairn as that user", path, owner)
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
	// Anyone who can write to the project root can swap the store between our
	// checks and the open, unless the sticky bit protects our entries.
	if rfi, err := os.Lstat(root); err == nil && rfi.Mode().Perm()&0o002 != 0 && rfi.Mode()&os.ModeSticky == 0 {
		return "", false, fmt.Errorf("refusing %s: the project folder is world-writable", dir)
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

// Pointer is the content of <child root>/.cairn/links.json: it names the parent
// project an attached child belongs to. It never holds a path.
type Pointer struct {
	Parent   string `json:"parent"`
	ParentID string `json:"parent_id"`
	Repo     string `json:"repo"`
}

func pointerPaths(root string) (dir, file string) {
	dir = filepath.Join(root, ".cairn")
	return dir, filepath.Join(dir, "links.json")
}

// ReadPointer reads the attached-child pointer of a folder. found is false when
// there is none. It applies the same trust rules as CheckStore: a symlink, a
// foreign owner or a world-writable file or folder is an error.
func ReadPointer(root string) (p Pointer, found bool, err error) {
	dir, file := pointerPaths(root)
	fi, err := os.Lstat(dir)
	if err != nil {
		return Pointer{}, false, nil
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return Pointer{}, false, fmt.Errorf("refusing symlinked store folder %s", dir)
	}
	if !fi.IsDir() {
		return Pointer{}, false, fmt.Errorf("%s exists but is not a folder", dir)
	}
	if err := checkTrusted(dir, fi); err != nil {
		return Pointer{}, false, err
	}
	fi, err = os.Lstat(file)
	if err != nil {
		return Pointer{}, false, nil
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return Pointer{}, false, fmt.Errorf("refusing symlinked pointer %s", file)
	}
	if !fi.Mode().IsRegular() {
		return Pointer{}, false, fmt.Errorf("%s is not a regular file", file)
	}
	if err := checkTrusted(file, fi); err != nil {
		return Pointer{}, false, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return Pointer{}, false, err
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return Pointer{}, false, fmt.Errorf("%s is malformed: %w", file, err)
	}
	if p.Parent == "" || p.Repo == "" {
		return Pointer{}, false, fmt.Errorf("%s is malformed: parent and repo are required", file)
	}
	return p, true, nil
}

// WritePointer writes the pointer, creating .cairn (0700) when needed and
// replacing links.json atomically (0600).
func WritePointer(root string, p Pointer) error {
	dir, file := pointerPaths(root)
	if fi, err := os.Lstat(dir); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlinked store folder %s", dir)
		}
		if !fi.IsDir() {
			return fmt.Errorf("%s exists but is not a folder", dir)
		}
		if err := checkTrusted(dir, fi); err != nil {
			return err
		}
	} else if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".links-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name) // no-op after a successful rename
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, file)
}

// RemovePointer deletes the pointer; an absent one is not an error.
func RemovePointer(root string) error {
	_, file := pointerPaths(root)
	if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
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
	db := reg.MachineDBPath(e.Name)
	if _, err := os.Lstat(db); err != nil {
		return Location{}, fmt.Errorf("%w: the database of machine project %q is missing (%s)", ErrNoProject, e.Name, db)
	}
	return Location{Name: e.Name, Mode: e.Mode, Root: e.Path, DBPath: db}, nil
}

// fromPointer resolves an attached child to its parent's location.
func fromPointer(reg *registry.Registry, childRoot string, p Pointer) (Location, error) {
	e, err := reg.Get(p.Parent)
	if errors.Is(err, registry.ErrNotFound) {
		return Location{}, fmt.Errorf("%w: the parent project %q of the repo %q is not registered; run `cairn project add <path>` for it", ErrNoProject, p.Parent, p.Repo)
	}
	if err != nil {
		return Location{}, err
	}
	loc, err := fromEntry(reg, e)
	if err != nil {
		return Location{}, err
	}
	loc.Attached, loc.Repo, loc.ChildRoot, loc.ParentID = true, p.Repo, childRoot, p.ParentID
	return loc, nil
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
		if p, ok, err := ReadPointer(dir); err != nil {
			return Location{}, err
		} else if ok {
			return fromPointer(reg, dir, p)
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

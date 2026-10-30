// Package registry keeps the machine-wide list of cairn projects.
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"syscall"

	"github.com/R0b-1ns/cairn/internal/validate"
)

// Sentinel errors.
var (
	ErrNotFound = errors.New("project not found in registry")
	ErrExists   = errors.New("project name or path already registered")
)

// Entry is one registered project. Path is the project root folder.
type Entry struct {
	Name string `json:"name"`
	Mode string `json:"mode"` // "project" or "machine"
	Path string `json:"path"`
}

type fileFormat struct {
	Version  int     `json:"version"`
	Projects []Entry `json:"projects"`
}

// Registry is the registry file inside a data directory (home).
type Registry struct{ home string }

// Home returns the data directory: $CAIRN_HOME, else the platform default
// (macOS: ~/Library/Application Support/cairn; elsewhere $XDG_DATA_HOME/cairn
// or ~/.local/share/cairn). It does not create anything.
func Home(getenv func(string) string) (string, error) {
	if h := getenv("CAIRN_HOME"); h != "" {
		return filepath.Abs(h) // a relative path would make the registry depend on the working directory
	}
	home := getenv("HOME")
	if home == "" {
		var err error
		if home, err = os.UserHomeDir(); err != nil {
			return "", fmt.Errorf("cannot find a data directory: set CAIRN_HOME: %w", err)
		}
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "cairn"), nil
	}
	if x := getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "cairn"), nil
	}
	return filepath.Join(home, ".local", "share", "cairn"), nil
}

// Open returns a Registry for a data directory. Nothing is created until a write.
func Open(home string) *Registry { return &Registry{home: home} }

// Home returns the data directory of this registry.
func (r *Registry) Home() string { return r.home }

// MachineDBPath returns <home>/projects/<name>/cairn.db for a validated name.
func (r *Registry) MachineDBPath(name string) string {
	return filepath.Join(r.home, "projects", name, "cairn.db")
}

// NormalizePath returns an absolute, cleaned path with symlinks resolved when the path exists.
// For a path that does not exist, the longest existing ancestor is resolved.
func NormalizePath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	var tail []string
	cur := abs
	for {
		if real, err := filepath.EvalSymlinks(cur); err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				real = filepath.Join(real, tail[i])
			}
			return real, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs, nil
		}
		tail = append(tail, filepath.Base(cur))
		cur = parent
	}
}

func (r *Registry) file() string { return filepath.Join(r.home, "registry.json") }

func (r *Registry) load() ([]Entry, error) {
	b, err := os.ReadFile(r.file())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var f fileFormat
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("registry %s is corrupt (not modified): %w", r.file(), err)
	}
	if f.Version != 1 {
		return nil, fmt.Errorf("registry %s has unsupported version %d", r.file(), f.Version)
	}
	return f.Projects, nil
}

// List returns every registered project sorted by name. A missing file means no projects.
func (r *Registry) List() ([]Entry, error) {
	es, err := r.load()
	sort.Slice(es, func(i, j int) bool { return es[i].Name < es[j].Name })
	return es, err
}

// Get returns the entry with the given name or ErrNotFound.
func (r *Registry) Get(name string) (Entry, error) {
	es, err := r.load()
	if err != nil {
		return Entry{}, err
	}
	for _, e := range es {
		if e.Name == name {
			return e, nil
		}
	}
	return Entry{}, fmt.Errorf("%w: %s", ErrNotFound, name)
}

// update runs fn on the entries under an exclusive lock and writes the result atomically.
func (r *Registry) update(fn func([]Entry) ([]Entry, error)) error {
	if err := os.MkdirAll(r.home, 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(r.home, "registry.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	es, err := r.load()
	if err != nil {
		return err
	}
	es, err = fn(es)
	if err != nil {
		return err
	}
	sort.Slice(es, func(i, j int) bool { return es[i].Name < es[j].Name })
	b, err := json.MarshalIndent(fileFormat{Version: 1, Projects: es}, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(r.home, "registry-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), r.file())
}

func samePath(a, b string) bool {
	if a == b {
		return true
	}
	ai, err1 := os.Stat(a)
	bi, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ai, bi)
}

// Register adds a project. Name must be valid; name and path must be unique (ErrExists).
// The path is normalized before storing. Creates the home directory (0700) and file (0600).
func (r *Registry) Register(e Entry) error {
	if !validate.ValidName(e.Name) {
		return fmt.Errorf("invalid project name %q", e.Name)
	}
	if e.Mode != "project" && e.Mode != "machine" {
		return fmt.Errorf("invalid mode %q", e.Mode)
	}
	p, err := NormalizePath(e.Path)
	if err != nil {
		return err
	}
	e.Path = p
	return r.update(func(es []Entry) ([]Entry, error) {
		for _, o := range es {
			if o.Name == e.Name {
				return nil, fmt.Errorf("%w: name %q is used by %s", ErrExists, e.Name, o.Path)
			}
			if samePath(o.Path, e.Path) {
				return nil, fmt.Errorf("%w: path %s belongs to %q", ErrExists, e.Path, o.Name)
			}
		}
		return append(es, e), nil
	})
}

// Move changes the path of a project (ErrNotFound / ErrExists).
func (r *Registry) Move(name, newPath string) error {
	p, err := NormalizePath(newPath)
	if err != nil {
		return err
	}
	return r.update(func(es []Entry) ([]Entry, error) {
		idx := -1
		for i, o := range es {
			if o.Name == name {
				idx = i
			}
		}
		if idx < 0 {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		for i, o := range es {
			if i != idx && samePath(o.Path, p) {
				return nil, fmt.Errorf("%w: path %s belongs to %q", ErrExists, p, o.Name)
			}
		}
		es[idx].Path = p
		return es, nil
	})
}

// Remove unregisters a project. It never deletes any database.
func (r *Registry) Remove(name string) error {
	return r.update(func(es []Entry) ([]Entry, error) {
		for i, o := range es {
			if o.Name == name {
				return append(es[:i], es[i+1:]...), nil
			}
		}
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	})
}

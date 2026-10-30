package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/R0b-1ns/cairn/internal/locate"
	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/registry"
	"github.com/R0b-1ns/cairn/internal/store"
	"github.com/R0b-1ns/cairn/internal/validate"
)

func init() {
	register(Command{Name: "init", Summary: "create a project (project or machine mode)", Define: defineInit})
	register(Command{Name: "context", Summary: "print the project name and context", Define: defineContext})
	register(Command{Name: "projects", Summary: "list every project registered on this machine", Define: defineProjects})
	register(Command{Name: "project move", Summary: "update the path of a registered project", Define: defineProjectMove})
	register(Command{Name: "project add", Summary: "register a project-mode database that came from elsewhere", Define: defineProjectAdd})
	register(Command{Name: "project remove", Summary: "unregister a project (never deletes its database)", Define: defineProjectRemove})
}

func defineInit(fs *flag.FlagSet) Handler {
	var p model.Project
	var repos, path string
	fs.StringVar(&p.Name, "name", "", "project name: lowercase letters, digits, - and _ (required)")
	fs.StringVar(&p.Mode, "mode", model.ModeProject, "storage mode: project (database in .cairn/) or machine (database in the data directory)")
	fs.StringVar(&path, "path", "", "project root folder (default: current directory)")
	fs.StringVar(&p.Context, "context", "", "a few lines describing the project")
	fs.StringVar(&repos, "repos", "", "comma-separated allowed repo names (required)")
	fs.StringVar(&p.TaskPattern, "task-pattern", model.DefaultTaskPattern, "regular expression with a named group (?P<base>...) and optionally (?P<ext>...)")
	return func(c *Ctx, args []string) int {
		if len(args) != 0 {
			return fail(c, ExitUsage, "init takes no arguments")
		}
		p.Repos = splitList(repos)
		if errs := validate.Project(p); len(errs) != 0 {
			return reportFieldErrors(c, errs)
		}
		if path == "" {
			path = "."
		}
		root, err := registry.NormalizePath(path)
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
			return fail(c, ExitRuntime, "%s is not a folder", root)
		}
		reg, err := openRegistry(c)
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		entries, err := reg.List()
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		for _, e := range entries {
			if e.Name == p.Name {
				return fail(c, ExitRuntime, "project %q is already registered at %s", e.Name, e.Path)
			}
			if sameFolder(e.Path, root) {
				return fail(c, ExitRuntime, "project %q is already registered for %s", e.Name, root)
			}
		}
		var dbPath string
		if p.Mode == model.ModeProject {
			existing, found, err := locate.CheckStore(root)
			if err != nil {
				return fail(c, ExitRuntime, "%v", err)
			}
			if found {
				return fail(c, ExitRuntime, "already initialized: %s exists", existing)
			}
			dbPath = filepath.Join(root, ".cairn", "cairn.db")
		} else {
			dbPath = reg.MachineDBPath(p.Name)
			if _, err := os.Lstat(dbPath); err == nil {
				return fail(c, ExitRuntime, "already initialized: %s exists", dbPath)
			}
		}
		st, err := store.Create(dbPath, p)
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		st.Close()
		if err := reg.Register(registry.Entry{Name: p.Name, Mode: p.Mode, Path: root}); err != nil {
			os.Remove(dbPath)
			os.Remove(filepath.Dir(dbPath))
			return fail(c, ExitRuntime, "%v", err)
		}
		fmt.Fprintf(c.Stdout, "initialized project %s (%s mode) at %s\ndatabase: %s\n", p.Name, p.Mode, root, dbPath)
		return ExitOK
	}
}

func sameFolder(a, b string) bool {
	if a == b {
		return true
	}
	ai, err1 := os.Stat(a)
	bi, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ai, bi)
}

func defineContext(fs *flag.FlagSet) Handler {
	var f common
	f.projectFlag(fs)
	f.jsonFlag(fs)
	return func(c *Ctx, args []string) int {
		if len(args) != 0 {
			return fail(c, ExitUsage, "context takes no arguments")
		}
		s, code := openProject(c, f.project)
		if s == nil {
			return code
		}
		defer s.Close()
		if f.json {
			printJSON(c, s.Proj)
			return ExitOK
		}
		p := s.Proj
		fmt.Fprintf(c.Stdout, "project: %s (%s mode)\nrepos: %s\ntask pattern: %s\n", p.Name, p.Mode, strings.Join(p.Repos, ", "), p.TaskPattern)
		if p.Context != "" {
			fmt.Fprintf(c.Stdout, "\n%s\n", p.Context)
		}
		return ExitOK
	}
}

type projectRow struct {
	Name    string `json:"name"`
	Mode    string `json:"mode"`
	Path    string `json:"path"`
	Notes   int    `json:"notes"` // -1 when the database cannot be read
	Missing bool   `json:"missing"`
}

func defineProjects(fs *flag.FlagSet) Handler {
	var f common
	f.jsonFlag(fs)
	return func(c *Ctx, args []string) int {
		if len(args) != 0 {
			return fail(c, ExitUsage, "projects takes no arguments")
		}
		reg, err := openRegistry(c)
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		entries, err := reg.List()
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		rows := []projectRow{}
		for _, e := range entries {
			row := projectRow{Name: e.Name, Mode: e.Mode, Path: e.Path, Notes: -1}
			if fi, err := os.Stat(e.Path); err != nil || !fi.IsDir() {
				row.Missing = true
			}
			db := reg.MachineDBPath(e.Name)
			if e.Mode == model.ModeProject {
				var found bool
				var err error
				if db, found, err = locate.CheckStore(e.Path); err != nil || !found {
					db = ""
				}
			}
			if db != "" {
				if st, err := store.Open(db); err == nil {
					if n, err := st.CountNotes(); err == nil {
						row.Notes = n
					}
					st.Close()
				}
			}
			rows = append(rows, row)
		}
		if f.json {
			printJSON(c, rows)
			return ExitOK
		}
		if len(rows) == 0 {
			fmt.Fprintln(c.Stdout, "no projects registered")
			return ExitOK
		}
		w := tabwriter.NewWriter(c.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tMODE\tNOTES\tPATH")
		for _, r := range rows {
			notes := "?"
			if r.Notes >= 0 {
				notes = fmt.Sprint(r.Notes)
			}
			path := r.Path
			if r.Missing {
				path += "  [missing]"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Name, r.Mode, notes, path)
		}
		w.Flush()
		return ExitOK
	}
}

func regErr(c *Ctx, err error) int { return fail(c, ExitRuntime, "%v", err) }

func defineProjectMove(fs *flag.FlagSet) Handler {
	return func(c *Ctx, args []string) int {
		if len(args) != 2 {
			return fail(c, ExitUsage, "usage: cairn project move <name> <new-path>")
		}
		reg, err := openRegistry(c)
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		entry, err := reg.Get(args[0])
		if err != nil {
			return regErr(c, err)
		}
		np, err := registry.NormalizePath(args[1])
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		if fi, err := os.Stat(np); err != nil || !fi.IsDir() {
			return fail(c, ExitRuntime, "%s is not an existing folder", np)
		}
		if entry.Mode == model.ModeProject {
			if _, found, err := locate.CheckStore(np); err != nil || !found {
				return fail(c, ExitRuntime, "no .cairn/cairn.db at %s", np)
			}
		}
		if err := reg.Move(args[0], np); err != nil {
			return regErr(c, err)
		}
		fmt.Fprintf(c.Stdout, "moved %s to %s\n", args[0], np)
		return ExitOK
	}
}

func defineProjectAdd(fs *flag.FlagSet) Handler {
	return func(c *Ctx, args []string) int {
		if len(args) != 1 {
			return fail(c, ExitUsage, "usage: cairn project add <path>")
		}
		root, err := registry.NormalizePath(args[0])
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		dbPath, found, err := locate.CheckStore(root)
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		if !found {
			return fail(c, ExitRuntime, "no .cairn/cairn.db at %s", root)
		}
		st, err := store.Open(dbPath)
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		p, err := st.Project()
		st.Close()
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		if p.Mode != model.ModeProject {
			return fail(c, ExitRuntime, "database at %s is not a project-mode database", root)
		}
		reg, err := openRegistry(c)
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		if err := reg.Register(registry.Entry{Name: p.Name, Mode: model.ModeProject, Path: root}); err != nil {
			return regErr(c, err)
		}
		fmt.Fprintf(c.Stdout, "registered %s at %s\n", p.Name, root)
		return ExitOK
	}
}

func defineProjectRemove(fs *flag.FlagSet) Handler {
	return func(c *Ctx, args []string) int {
		if len(args) != 1 {
			return fail(c, ExitUsage, "usage: cairn project remove <name>")
		}
		reg, err := openRegistry(c)
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		if err := reg.Remove(args[0]); err != nil {
			return regErr(c, err)
		}
		fmt.Fprintf(c.Stdout, "unregistered %s (its database was not deleted)\n", args[0])
		return ExitOK
	}
}

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/R0b-1ns/cairn/internal/locate"
	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/registry"
	"github.com/R0b-1ns/cairn/internal/store"
	"github.com/R0b-1ns/cairn/internal/validate"
)

// initChild joins the project parentName as a child. An attached child (the
// default) gets a pointer file and uses the parent's database; a local child
// gets its own database. Nothing is asked: the child takes the parent's task
// settings and its own repo.
//
// Effects are ordered so that a failure leaves nothing half done: every check
// comes first, then the child's database (local), the parent's link events,
// the pointer or the child's parent event, and the registry last. Link events
// cannot be deleted, so a later failure records child_detached for the link.
func initChild(c *Ctx, parentName, repo, name string, local bool, path, context string) int {
	if repo == "" {
		return fail(c, ExitUsage, "--project needs --repo (the repo name of this folder)")
	}
	nameFromRepo := false
	if !local {
		name = repo // an attached child is named after its repo
	} else if name == "" {
		name, nameFromRepo = repo, true
	}
	var errs []validate.FieldError
	if !validate.ValidRepo(repo) {
		errs = append(errs, validate.FieldError{Field: "repo", Message: fmt.Sprintf("repo %s must be 1-40 characters of a-z, 0-9, '.', '_' or '-', starting with a letter or digit", sanitize(repo))})
	}
	if local && !validate.ValidName(name) {
		msg := fmt.Sprintf("project name %s must be lowercase letters, digits, '-' or '_', starting with a letter or digit, 1-40 characters", sanitize(name))
		if nameFromRepo {
			msg = fmt.Sprintf("the repo %s is not a valid project name: give the project name of the local child with --name", sanitize(repo))
		}
		errs = append(errs, validate.FieldError{Field: "name", Message: msg})
	}
	if !validate.ValidName(parentName) {
		errs = append(errs, validate.FieldError{Field: "project", Message: fmt.Sprintf("project name %s is not a valid name", sanitize(parentName))})
	}
	if len(errs) != 0 {
		return reportFieldErrors(c, errs)
	}
	if path == "" {
		path = "."
	}
	root, err := registry.NormalizePath(path)
	if err != nil {
		return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
	}
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return fail(c, ExitRuntime, "%s is not a folder", sanitize(root))
	}
	// A local child can adopt a standalone database left by `project unlink`.
	var cs *store.Store
	adopt := false
	childDB := filepath.Join(root, ".cairn", "cairn.db")
	if local {
		if db, found, err := locate.CheckStore(root); err != nil {
			return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
		} else if found {
			var code int
			if cs, code = adoptableDatabase(c, db, name, repo); cs == nil {
				return code
			}
			defer func() { cs.Close() }()
			adopt = true
		}
	}
	reg, err := openRegistry(c)
	if err != nil {
		return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
	}
	entries, err := reg.List()
	if err != nil {
		return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
	}
	var pe *registry.Entry
	registered := false
	for i, e := range entries {
		if e.Name == parentName {
			pe = &entries[i]
		}
		if adopt && e.Name == name && sameFolder(e.Path, root) {
			registered = true
			continue
		}
		if sameFolder(e.Path, root) {
			if len(e.Children) != 0 {
				return fail(c, ExitRuntime, "%s is already the parent project %s; nesting is not supported", sanitize(root), sanitize(e.Name))
			}
			return fail(c, ExitRuntime, "project %s is already registered for %s", sanitize(e.Name), sanitize(root))
		}
		if local && e.Name == name {
			return fail(c, ExitRuntime, "project %s is already registered at %s", sanitize(e.Name), sanitize(e.Path))
		}
	}
	if pe == nil {
		return fail(c, ExitRuntime, "project %s is not registered on this machine; if it is a project-mode database, run: cairn project add <path>", sanitize(parentName))
	}
	loc, err := locate.Find(root, parentName, reg)
	if err != nil {
		return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
	}
	parent, err := store.Open(loc.DBPath)
	if err != nil {
		return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
	}
	defer parent.Close()
	pp, err := parent.Project()
	if err != nil {
		return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
	}
	pl, err := parent.Links()
	if err != nil {
		return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
	}
	if pl.Parent != nil {
		return fail(c, ExitRuntime, "project %s is itself a child of %s; nesting is not supported", sanitize(parentName), sanitize(pl.Parent.Name))
	}
	for _, ch := range pl.Children {
		if ch.Name == name {
			return fail(c, ExitRuntime, "project %s already has an active child named %s", sanitize(parentName), sanitize(name))
		}
	}
	// The folder must hold neither a database (unless adopted) nor a pointer.
	if db, found, err := locate.CheckStore(root); err != nil {
		return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
	} else if found && !adopt {
		return fail(c, ExitRuntime, "already initialized: %s exists", sanitize(db))
	}
	if _, found, err := locate.ReadPointer(root); err != nil {
		return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
	} else if found {
		return fail(c, ExitRuntime, "already initialized: %s already holds a pointer to a parent project", sanitize(root))
	}

	var child model.Project
	if local {
		child = model.Project{
			Name: name, Mode: model.ModeProject, Context: context,
			Repos: []string{repo}, DefaultRepo: repo,
			TaskPattern: pp.TaskPattern, TaskMode: pp.TaskMode, TaskPrefix: pp.TaskPrefix, TaskURL: pp.TaskURL,
		}
		if errs := validate.Project(child); len(errs) != 0 {
			return reportFieldErrors(c, errs)
		}
	}

	dotCairn := filepath.Join(root, ".cairn")
	_, statErr := os.Lstat(dotCairn)
	createdDir := statErr != nil
	linked := false
	parentSet := false
	gitignoreCreated := false
	cleanup := func() {
		if gitignoreCreated {
			removeCreatedGitignore(root)
		}
		if adopt && parentSet {
			if err := cs.ClearParent(); err != nil {
				fmt.Fprintf(c.Stderr, "cairn: warning: could not clear the parent of %s (%s); run in that folder: cairn project unlink\n", sanitize(name), sanitize(err.Error()))
			}
		}
		if cs != nil {
			cs.Close()
		}
		if adopt {
			// the standalone database stays
		} else if local {
			for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
				os.Remove(childDB + suffix)
			}
		} else {
			locate.RemovePointer(root)
		}
		if createdDir && !adopt {
			os.Remove(dotCairn)
		}
		if linked {
			if err := parent.DetachChild(name); err != nil {
				fmt.Fprintf(c.Stderr, "cairn: warning: could not record that child %s of %s is detached (%s); run: cairn project unlink %s\n",
					sanitize(name), sanitize(parentName), sanitize(err.Error()), sanitize(name))
			}
		}
	}
	failRun := func(err error) int {
		cleanup()
		return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
	}

	if local && !adopt {
		if cs, err = store.Create(childDB, child); err != nil {
			cleanup()
			return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
		}
	}
	var gitignoreNote string
	if local {
		// The line is printed with the other output, after the steps that can still fail.
		changed, created, gerr := applyGitignore(root)
		gitignoreCreated = created
		switch {
		case gerr != nil:
			warnf(c, "could not update .gitignore (%s); add the line %s to %s by hand so database backups are not committed",
				sanitize(gerr.Error()), bakIgnoreLine, sanitize(filepath.Join(root, ".gitignore")))
		case changed:
			gitignoreNote = fmt.Sprintf(".gitignore now ignores %s\n", bakIgnoreLine)
		}
	}
	pid, err := parent.EnsureProjectID()
	if err != nil {
		return failRun(err)
	}
	if err := parent.AddRepo(repo); err != nil {
		return failRun(err)
	}
	kind := model.ChildAttached
	if local {
		kind = model.ChildLocal
	}
	if err := parent.LinkChild(model.Child{Name: name, Repo: repo, Kind: kind}); err != nil {
		return failRun(err)
	}
	linked = true

	var undoReg []func()
	rollback := func(err error) int {
		for i := len(undoReg) - 1; i >= 0; i-- {
			undoReg[i]()
		}
		return failRun(err)
	}
	if pe.ID != pid {
		if err := reg.SetID(parentName, pid); err != nil {
			return rollback(err)
		}
	}
	if local {
		if err := cs.SetParent(model.ParentRef{Name: parentName, ID: pid}); err != nil {
			return rollback(err)
		}
		parentSet = true
		cid, err := cs.EnsureProjectID()
		if err != nil {
			return rollback(err)
		}
		if registered {
			if err := reg.SetID(name, cid); err != nil {
				return rollback(err)
			}
		} else {
			if err := reg.Register(registry.Entry{Name: name, Mode: model.ModeProject, Path: root, ID: cid}); err != nil {
				return rollback(err)
			}
			undoReg = append(undoReg, func() { reg.Remove(name) })
		}
	}
	if err := reg.SetChild(parentName, name, root); err != nil {
		return rollback(err)
	}
	undoReg = append(undoReg, func() { reg.RemoveChild(parentName, name) })
	if !local {
		// The registry entry above must exist first: locate refuses a pointer
		// the parent does not know.
		if err := locate.WritePointer(root, locate.Pointer{Parent: parentName, ParentID: pid, Repo: repo}); err != nil {
			return rollback(err)
		}
	}
	if cs != nil {
		cs.Close()
	}

	if adopt {
		fmt.Fprintf(c.Stdout, "adopted the existing database of project %s at %s as a local child of %s (repo %s); no note was changed\ndatabase: %s\n",
			sanitize(name), sanitize(root), sanitize(parentName), sanitize(repo), sanitize(childDB))
	} else if local {
		fmt.Fprintf(c.Stdout, "initialized project %s (project mode) at %s as a local child of %s (repo %s)\ndatabase: %s\n",
			sanitize(name), sanitize(root), sanitize(parentName), sanitize(repo), sanitize(childDB))
	} else {
		fmt.Fprintf(c.Stdout, "attached %s to project %s as repo %s\npointer: %s\n",
			sanitize(root), sanitize(parentName), sanitize(repo), sanitize(filepath.Join(dotCairn, "links.json")))
	}
	fmt.Fprint(c.Stdout, gitignoreNote)
	if loc.Mode == model.ModeProject {
		fmt.Fprintf(c.Stdout, "the parent database %s changed (link events); commit it so your team gets the link\n", sanitize(loc.DBPath))
	}
	noticeNewVersion(c)
	return ExitOK
}

// adoptableDatabase opens the database in an existing folder as a candidate
// for a local child: a standalone project with the chosen name and repo. It
// prints why it refuses and returns the exit code.
func adoptableDatabase(c *Ctx, db, name, repo string) (*store.Store, int) {
	cs, err := store.OpenCurrent(db)
	if err != nil {
		return nil, fail(c, ExitRuntime, "already initialized: %s exists and cannot be adopted: %s", sanitize(db), sanitize(err.Error()))
	}
	refuse := func(format string, a ...any) (*store.Store, int) {
		cs.Close()
		return nil, fail(c, ExitRuntime, "already initialized: %s exists and cannot be adopted: "+format, append([]any{sanitize(db)}, a...)...)
	}
	cp, err := cs.Project()
	if err != nil {
		return refuse("%s", sanitize(err.Error()))
	}
	if cp.Name != name {
		return refuse("it belongs to project %s, not %s (use --name)", sanitize(cp.Name), sanitize(name))
	}
	if !containsRepo(cp.Repos, repo) {
		return refuse("repo %s is not one of its repos", sanitize(repo))
	}
	cl, err := cs.Links()
	if err != nil {
		return refuse("%s", sanitize(err.Error()))
	}
	if cl.Parent != nil {
		return refuse("it is already a child of project %s; unlink it first", sanitize(cl.Parent.Name))
	}
	return cs, ExitOK
}

func containsRepo(repos []string, r string) bool {
	for _, x := range repos {
		if x == r {
			return true
		}
	}
	return false
}

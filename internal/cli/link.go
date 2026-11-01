package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/R0b-1ns/cairn/internal/locate"
	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/registry"
	"github.com/R0b-1ns/cairn/internal/store"
)

func init() {
	register(Command{Name: "project link", Usage: "[--project NAME] <child> <path>", Summary: "record where a child of a project lives on this machine", Define: defineProjectLink})
	register(Command{Name: "project unlink", Usage: "[--project NAME] [<child>]", Summary: "detach a child from its parent (never deletes a database or a note)", Define: defineProjectUnlink})
}

// parentSession opens the parent project for link and unlink: found from the
// current directory or --project, never through a child's pointer (an unlinked
// attached folder cannot be resolved that way). The parent must be registered,
// because the registry is where the child's path lives.
func parentSession(c *Ctx, projectFlag string) (*session, store.LinkState, int) {
	s, code := openProject(c, projectFlag)
	if s == nil {
		return nil, store.LinkState{}, code
	}
	if s.Loc.Name == "" {
		s.Close()
		return nil, store.LinkState{}, fail(c, ExitRuntime, "project at %s is not registered on this machine; run: cairn project add %s", sanitize(s.Loc.Root), sanitize(s.Loc.Root))
	}
	pl, err := s.St.Links()
	if err != nil {
		s.Close()
		return nil, store.LinkState{}, fail(c, ExitRuntime, "%s", sanitize(err.Error()))
	}
	return s, pl, ExitOK
}

func activeChild(pl store.LinkState, name string) (model.Child, bool) {
	for _, ch := range pl.Children {
		if ch.Name == name {
			return ch, true
		}
	}
	return model.Child{}, false
}

func defineProjectLink(fs *flag.FlagSet) Handler {
	var f common
	f.projectFlag(fs)
	return func(c *Ctx, args []string) int {
		if len(args) != 2 {
			return fail(c, ExitUsage, "usage: cairn project link [--project NAME] <child> <path>")
		}
		childName := args[0]
		root, err := registry.NormalizePath(args[1])
		if err != nil {
			return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
		}
		if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
			return fail(c, ExitRuntime, "%s is not an existing folder", sanitize(root))
		}
		s, pl, code := parentSession(c, f.project)
		if s == nil {
			return code
		}
		defer s.Close()
		parent := s.Loc.Name
		ch, ok := activeChild(pl, childName)
		if !ok {
			return fail(c, ExitRuntime, "project %s has no active child named %s", sanitize(parent), sanitize(childName))
		}
		if err := verifyChildFolder(root, parent, pl.ID, ch); err != nil {
			return fail(c, ExitRuntime, "%s is not the child %s of project %s: %s", sanitize(root), sanitize(childName), sanitize(parent), sanitize(err.Error()))
		}
		if err := s.Reg.SetChild(parent, childName, root); err != nil {
			return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
		}
		fmt.Fprintf(c.Stdout, "linked child %s of project %s at %s\n", sanitize(childName), sanitize(parent), sanitize(root))
		return ExitOK
	}
}

// verifyChildFolder checks that root really is the child ch of the parent: a
// local child's database names this parent (name and id); an attached child's
// pointer matches parent, repo and id.
func verifyChildFolder(root, parent, parentID string, ch model.Child) error {
	if ch.Kind == model.ChildLocal {
		db, found, err := locate.CheckStore(root)
		if err != nil {
			return err
		}
		if !found {
			return errors.New("it holds no .cairn/cairn.db")
		}
		cs, err := store.Open(db)
		if err != nil {
			return err
		}
		defer cs.Close()
		cp, err := cs.Project()
		if err != nil {
			return err
		}
		cl, err := cs.Links()
		if err != nil {
			return err
		}
		if cp.Name != ch.Name {
			return fmt.Errorf("its database belongs to project %q", cp.Name)
		}
		if cl.Parent == nil || cl.Parent.Name != parent || cl.Parent.ID != parentID {
			return errors.New("its database does not name this project as its parent")
		}
		return nil
	}
	p, found, err := locate.ReadPointer(root)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("it holds no pointer file (.cairn/links.json)")
	}
	if p.Parent != parent || p.Repo != ch.Repo || p.ParentID != parentID {
		return errors.New("its pointer names another parent, repo or project id")
	}
	return nil
}

func defineProjectUnlink(fs *flag.FlagSet) Handler {
	var f common
	f.projectFlag(fs)
	return func(c *Ctx, args []string) int {
		switch len(args) {
		case 0:
			return unlinkFromChild(c)
		case 1:
			return unlinkFromParent(c, f.project, args[0])
		}
		return fail(c, ExitUsage, "usage: cairn project unlink [--project NAME] [<child>]")
	}
}

func warnf(c *Ctx, format string, a ...any) {
	fmt.Fprintf(c.Stderr, "cairn: warning: "+format+"\n", a...)
}

// unlinkFromParent detaches the child on the parent's side and, when the
// child's folder is known on this machine, on the child's side.
func unlinkFromParent(c *Ctx, projectFlag, childName string) int {
	s, pl, code := parentSession(c, projectFlag)
	if s == nil {
		return code
	}
	defer s.Close()
	parent := s.Loc.Name
	ch, ok := activeChild(pl, childName)
	if !ok {
		return fail(c, ExitRuntime, "project %s has no active child named %s", sanitize(parent), sanitize(childName))
	}
	entry, err := s.Reg.Get(parent)
	if err != nil {
		return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
	}
	// Child side first, so a failure here leaves the link intact for a retry.
	if path := entry.Children[childName]; path != "" {
		clearChildSide(c, ch, path, parent, pl.ID)
	} else {
		warnf(c, "the folder of child %s is not known on this machine; its own link is untouched; run in that folder: cairn project unlink", sanitize(childName))
	}
	if err := s.St.DetachChild(childName); err != nil {
		return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
	}
	if err := s.Reg.RemoveChild(parent, childName); err != nil {
		return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
	}
	fmt.Fprintf(c.Stdout, "unlinked child %s from project %s (repo %s stays in the project; no database or note was deleted)\n",
		sanitize(childName), sanitize(parent), sanitize(ch.Repo))
	commitHint(c, s)
	return ExitOK
}

func commitHint(c *Ctx, s *session) {
	if s.Loc.Mode == model.ModeProject {
		fmt.Fprintf(c.Stdout, "the parent database %s changed (link events); commit it so your team gets the change\n", sanitize(s.Loc.DBPath))
	}
}

// clearChildSide removes the child's own link at path when it still names this
// parent. Failures only warn: the parent side is the authority.
func clearChildSide(c *Ctx, ch model.Child, path, parent, parentID string) {
	if ch.Kind == model.ChildLocal {
		db, found, err := locate.CheckStore(path)
		if err != nil || !found {
			warnf(c, "cannot reach the database of child %s at %s; run in that folder: cairn project unlink", sanitize(ch.Name), sanitize(path))
			return
		}
		cs, err := store.Open(db)
		if err != nil {
			warnf(c, "cannot open the database of child %s (%s); run in that folder: cairn project unlink", sanitize(ch.Name), sanitize(err.Error()))
			return
		}
		defer cs.Close()
		cl, err := cs.Links()
		if err != nil {
			warnf(c, "cannot read the links of child %s (%s)", sanitize(ch.Name), sanitize(err.Error()))
			return
		}
		if cl.Parent == nil || cl.Parent.Name != parent || cl.Parent.ID != parentID {
			return // not (or no longer) this parent's child
		}
		if err := cs.ClearParent(); err != nil {
			warnf(c, "could not clear the parent of child %s (%s); run in that folder: cairn project unlink", sanitize(ch.Name), sanitize(err.Error()))
		}
		return
	}
	p, found, err := locate.ReadPointer(path)
	if err != nil {
		warnf(c, "cannot read the pointer of child %s (%s); run in that folder: cairn project unlink", sanitize(ch.Name), sanitize(err.Error()))
		return
	}
	if !found || p.Parent != parent || p.Repo != ch.Repo {
		return
	}
	if err := locate.RemovePointer(path); err != nil {
		warnf(c, "could not remove the pointer of child %s (%s)", sanitize(ch.Name), sanitize(err.Error()))
	}
}

// nearestChildFolder walks up from dir to the first folder holding a database
// or a pointer. For a pointer it returns the pointer.
func nearestChildFolder(dir string) (root string, ptr *locate.Pointer, dbFound bool, err error) {
	if r, e := filepath.EvalSymlinks(dir); e == nil {
		dir = r
	}
	for {
		if _, found, err := locate.CheckStore(dir); err != nil {
			return "", nil, false, err
		} else if found {
			return dir, nil, true, nil
		}
		if p, found, err := locate.ReadPointer(dir); err != nil {
			return "", nil, false, err
		} else if found {
			return dir, &p, false, nil
		}
		up := filepath.Dir(dir)
		if up == dir {
			return "", nil, false, locate.ErrNoProject
		}
		dir = up
	}
}

// unlinkFromChild runs inside a child folder. It reads the child's own link
// (not through locate, which refuses an unlinked attached folder), clears it,
// and detaches from the parent when the parent is reachable on this machine.
func unlinkFromChild(c *Ctx) int {
	cwd, err := os.Getwd()
	if err != nil {
		return fail(c, ExitRuntime, "cannot read the current directory: %s", sanitize(err.Error()))
	}
	root, ptr, _, err := nearestChildFolder(cwd)
	if err != nil {
		if errors.Is(err, locate.ErrNoProject) {
			return fail(c, ExitNoProject, "no cairn project or attached repo found from %s", sanitize(cwd))
		}
		return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
	}
	reg, err := openRegistry(c)
	if err != nil {
		return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
	}

	var childName, parentName, parentID string
	var kind model.ChildKind
	var cs *store.Store
	if ptr != nil {
		kind, childName, parentName, parentID = model.ChildAttached, ptr.Repo, ptr.Parent, ptr.ParentID
	} else {
		db, _, _ := locate.CheckStore(root)
		if cs, err = store.Open(db); err != nil {
			return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
		}
		defer cs.Close()
		cp, err := cs.Project()
		if err != nil {
			return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
		}
		cl, err := cs.Links()
		if err != nil {
			return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
		}
		if cl.Parent == nil {
			return fail(c, ExitRuntime, "project %s at %s has no parent project; nothing to unlink", sanitize(cp.Name), sanitize(root))
		}
		kind, childName, parentName, parentID = model.ChildLocal, cp.Name, cl.Parent.Name, cl.Parent.ID
	}

	// Detach from the parent when it is reachable and is the very project the
	// child recorded; otherwise only the child's own link is cleared.
	detached, reason := false, ""
	var parentSt *store.Store
	if ent, err := reg.Get(parentName); err != nil {
		reason = fmt.Sprintf("project %s is not registered on this machine", sanitize(parentName))
	} else if ent.ID != "" && parentID != "" && ent.ID != parentID {
		reason = fmt.Sprintf("the registered project %s is not the one this folder was linked to", sanitize(parentName))
	} else if loc, err := locate.Find("", parentName, reg); err != nil {
		reason = sanitize(err.Error())
	} else if parentSt, err = store.Open(loc.DBPath); err != nil {
		reason = sanitize(err.Error())
	} else {
		defer parentSt.Close()
		if pl, err := parentSt.Links(); err != nil {
			reason = sanitize(err.Error())
		} else if pl.ID != "" && parentID != "" && pl.ID != parentID {
			reason = fmt.Sprintf("the database of project %s is not the one this folder was linked to", sanitize(parentName))
		} else {
			detached = true
			if _, ok := activeChild(pl, childName); ok {
				if err := parentSt.DetachChild(childName); err != nil {
					return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
				}
			}
			if p := childPath(reg, parentName, childName); p == "" || sameFolder(p, root) {
				if err := reg.RemoveChild(parentName, childName); err != nil {
					return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
				}
			}
		}
	}

	if kind == model.ChildLocal {
		if err := cs.ClearParent(); err != nil {
			return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
		}
	} else if err := locate.RemovePointer(root); err != nil {
		return fail(c, ExitRuntime, "%s", sanitize(err.Error()))
	}
	if detached {
		fmt.Fprintf(c.Stdout, "unlinked %s from project %s (no database or note was deleted)\n", sanitize(childName), sanitize(parentName))
	} else {
		fmt.Fprintf(c.Stdout, "cleared the link of %s to project %s (no database or note was deleted)\n", sanitize(childName), sanitize(parentName))
		warnf(c, "%s; the parent still lists %s as a child; run on the parent: cairn project unlink --project %s %s",
			reason, sanitize(childName), sanitize(parentName), sanitize(childName))
	}
	return ExitOK
}

func childPath(reg *registry.Registry, parent, child string) string {
	e, _ := reg.Get(parent)
	return e.Children[child]
}

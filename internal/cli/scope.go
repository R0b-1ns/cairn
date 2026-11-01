package cli

import (
	"errors"
	"fmt"

	"github.com/R0b-1ns/cairn/internal/locate"
	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
	"github.com/R0b-1ns/cairn/internal/validate"
)

// source is one database a read command looks in. Child is empty for the
// current project and the child's name otherwise.
type source struct {
	Child string
	St    *store.Store
	Proj  model.Project
}

// scope is every database a read command aggregates: the current project
// first, then each active local child in link order.
type scope struct {
	Sources []source
	Missing []string // children that could not be read, each already warned about
}

// Close closes the child databases; the session owns the current one.
func (sc *scope) Close() {
	for _, s := range sc.Sources {
		if s.Child != "" {
			s.St.Close()
		}
	}
}

// openScope opens the local children of the session's project for reading
// only. Attached children are skipped (their notes are in this database), and
// a session of a child has no children of its own. A child that cannot be
// read is reported on standard error and listed in Missing.
func openScope(c *Ctx, s *session) *scope {
	sc := &scope{Sources: []source{{St: s.St, Proj: s.Proj}}}
	pl, err := s.St.Links()
	if err != nil || len(pl.Children) == 0 || pl.Parent != nil {
		return sc
	}
	parent := s.Proj.Name
	var paths map[string]string
	if s.Reg != nil {
		if e, err := s.Reg.Get(parent); err == nil {
			paths = e.Children
		}
	}
	for _, ch := range pl.Children {
		if ch.Kind != model.ChildLocal || !validate.ValidName(ch.Name) {
			continue
		}
		path := paths[ch.Name]
		st, why := openChild(path, parent, pl.ID, ch.Name)
		if st == nil {
			sc.Missing = append(sc.Missing, ch.Name)
			warnf(c, "%s", missingChild(parent, ch.Name, path, why))
			continue
		}
		p, err := st.Project()
		if err != nil {
			st.Close()
			sc.Missing = append(sc.Missing, ch.Name)
			warnf(c, "%s", missingChild(parent, ch.Name, path, sanitize(err.Error())))
			continue
		}
		sc.Sources = append(sc.Sources, source{Child: ch.Name, St: st, Proj: p})
	}
	return sc
}

// openChild opens a child database read-only in effect and checks that it
// really is that child of this parent. It returns a reason when it cannot.
func openChild(path, parent, parentID, name string) (*store.Store, string) {
	if path == "" {
		return nil, "its folder is not known on this machine"
	}
	db, found, err := locate.CheckStore(path)
	if err != nil {
		return nil, sanitize(err.Error())
	}
	if !found {
		return nil, "no cairn database there"
	}
	st, err := store.OpenCurrent(db)
	if err != nil {
		var nm *store.NeedsMigrationError
		if errors.As(err, &nm) {
			return nil, fmt.Sprintf("its database has schema version %d, older than this build; run `cairn --project %s context` from that repo once to upgrade it", nm.Version, sanitize(name))
		}
		return nil, sanitize(err.Error())
	}
	cl, err := st.Links()
	if err != nil {
		st.Close()
		return nil, sanitize(err.Error())
	}
	if cl.Parent == nil || cl.Parent.Name != parent || cl.Parent.ID != parentID || parentID == "" {
		st.Close()
		return nil, "that folder holds a different project"
	}
	return st, ""
}

func missingChild(parent, name, path, why string) string {
	p := path
	if p == "" {
		p = "<path>"
	}
	return fmt.Sprintf("local child %s of %s was not read: %s; to point to it, run: cairn project link --project %s %s %s",
		sanitize(name), sanitize(parent), why, sanitize(parent), sanitize(name), sanitize(p))
}

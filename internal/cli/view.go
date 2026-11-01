package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
	"github.com/R0b-1ns/cairn/internal/view"
)

func init() {
	register(Command{Name: "view", Usage: "[flags]", Summary: "write a read-only website of the notes, to browse and filter them", Define: defineView})
}

// openBrowser opens url in the default browser; tests replace it.
var openBrowser = defaultOpenBrowser

func defaultOpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // reap the launcher without blocking the server
	return nil
}

// serveContext is the lifetime of `view --serve`: until Ctrl-C or SIGTERM. Tests replace it.
var serveContext = func() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// sanitizeSite runs every stored value of the site's structured fields through
// sanitize, as terminal output is: a hostile database must not put escape
// sequences or invisible and direction-changing characters into the pages or
// data.js. The Markdown is rendered before this, from the stored values, by
// the export renderer, which cleans each value as it prints it.
func sanitizeSite(site *view.Site) {
	list := func(l []string) {
		for i := range l {
			l[i] = sanitize(l[i])
		}
	}
	entry := func(e *model.Entry) {
		e.ID, e.Text, e.By = sanitize(e.ID), sanitize(e.Text), sanitize(e.By)
		e.Type = model.EntryType(sanitize(string(e.Type)))
	}
	info := func(e *store.EntryInfo) {
		entry(&e.Entry)
		for _, f := range []*string{&e.NoteID, &e.AddendumID, &e.Task, &e.Repo, &e.At, &e.ClosedBy, &e.ClosedAt} {
			*f = sanitize(*f)
		}
	}
	p := &site.Project
	p.Name, p.Context = sanitize(p.Name), sanitize(p.Context)
	for i := range site.Notes {
		nv := &site.Notes[i]
		nv.Child = sanitize(nv.Child)
		h := &nv.Header
		for _, f := range []*string{&h.ID, &h.Task, &h.Repo, &h.Agent, &h.Model, &h.At, &h.Title, &h.Branch, &h.AuthorName, &h.AuthorEmail, &h.Ref} {
			*f = sanitize(*f)
		}
		h.Status = model.Status(sanitize(string(h.Status)))
		nv.Written = model.Status(sanitize(string(nv.Written)))
		n := &nv.Note
		for _, f := range []*string{&n.ID, &n.Agent, &n.Repo, &n.At, &n.Task, &n.Model, &n.Title, &n.Branch, &n.AuthorName, &n.AuthorEmail, &n.Ref} {
			*f = sanitize(*f)
		}
		n.Status = model.Status(sanitize(string(n.Status)))
		for j := range n.Entries {
			entry(&n.Entries[j])
		}
		for j := range n.None {
			n.None[j] = model.EntryType(sanitize(string(n.None[j])))
		}
		list(nv.TaskRefs)
		list(n.Impacts)
		list(n.Commits)
		list(n.Resolves)
		list(n.Supersedes)
		for j := range nv.Addenda {
			a := &nv.Addenda[j]
			for _, f := range []*string{&a.ID, &a.NoteID, &a.Agent, &a.Model, &a.At, &a.Branch, &a.AuthorName, &a.AuthorEmail, &a.Ref} {
				*f = sanitize(*f)
			}
			a.Status = model.Status(sanitize(string(a.Status)))
			for k := range a.Entries {
				entry(&a.Entries[k])
			}
			list(a.Resolves)
			list(a.Supersedes)
		}
		states := make(map[string]store.EntryInfo, len(nv.States))
		for _, st := range nv.States {
			info(&st)
			states[st.ID] = st
		}
		nv.States = states
	}
	for i := range site.Entries {
		info(&site.Entries[i].EntryInfo)
		site.Entries[i].Child = sanitize(site.Entries[i].Child)
		for _, f := range []*string{&site.Entries[i].Agent, &site.Entries[i].Branch, &site.Entries[i].AuthorName, &site.Entries[i].AuthorEmail} {
			*f = sanitize(*f)
		}
	}
}

type viewResult struct {
	Dir   string `json:"dir"`
	Index string `json:"index"`
	URL   string `json:"url"`
}

func defineView(fs *flag.FlagSet) Handler {
	var f common
	f.projectFlag(fs)
	f.jsonFlag(fs)
	var serve, noOpen bool
	var port int
	fs.BoolVar(&serve, "serve", false, "also serve the site on http://127.0.0.1 until Ctrl-C")
	fs.IntVar(&port, "port", 0, "with --serve: the port to listen on (0 picks a free port)")
	fs.BoolVar(&noOpen, "no-open", false, "with --serve: do not open the browser")
	return func(c *Ctx, args []string) int {
		if len(args) != 0 {
			return fail(c, ExitUsage, "view takes no arguments")
		}
		set := map[string]bool{}
		fs.Visit(func(fl *flag.Flag) { set[fl.Name] = true })
		if !serve && (set["port"] || set["no-open"]) {
			return fail(c, ExitUsage, "--port and --no-open need --serve")
		}
		if port < 0 || port > 65535 {
			return fail(c, ExitUsage, "--port must be between 0 and 65535")
		}
		s, code := openProject(c, f.project)
		if s == nil {
			return code
		}
		open := true
		defer func() {
			if open {
				s.Close()
			}
		}()
		sc := openScope(c, s)
		defer sc.Close()
		site, err := collectViewSite(s, sc.Sources)
		if err != nil {
			var dup *view.DuplicateNoteError
			if errors.As(err, &dup) {
				name := func(child string) string {
					if child == "" {
						return "project " + sanitize(s.Proj.Name)
					}
					return "child " + sanitize(child)
				}
				return fail(c, ExitRuntime, "cannot write the site: note %s is in both %s and %s",
					sanitize(dup.ID), name(dup.First), name(dup.Second))
			}
			return fail(c, ExitRuntime, "%v", err)
		}
		sanitizeSite(site)
		dir := filepath.Join(filepath.Dir(s.St.Path()), "view")
		if err := view.Write(dir, site); err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		res := viewResult{Dir: dir, Index: filepath.Join(dir, "index.html")}
		if !f.json {
			fmt.Fprintf(c.Stdout, "wrote %s\nopen %s\n", res.Dir, res.Index)
		}
		if !serve {
			if f.json {
				printJSON(c, res)
			}
			return ExitOK
		}
		// The databases are not needed while serving the written files.
		sc.Close()
		s.Close()
		open = false
		ctx, cancel := serveContext()
		defer cancel()
		// Give the signals back as soon as the first one arrives, so a second
		// Ctrl-C kills the process instead of waiting out the graceful shutdown.
		go func() { <-ctx.Done(); cancel() }()
		err = view.Serve(ctx, dir, port, func(url string) {
			if f.json {
				res.URL = url
				printJSON(c, res)
			} else {
				fmt.Fprintf(c.Stdout, "serving %s (press Ctrl-C to stop)\n", url)
			}
			if !noOpen {
				if err := openBrowser(url); err != nil {
					fmt.Fprintf(c.Stderr, "cairn: warning: could not open the browser: %v\n", err)
				}
			}
		})
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		return ExitOK
	}
}

// collectViewSite reads every source into one site and fills the Markdown of
// each note from its own source, as export does. A note of a child starts with
// the child's name, as in `export --format md`.
func collectViewSite(s *session, sources []source) (*view.Site, error) {
	srcs := make([]view.Source, len(sources))
	byChild := map[string]source{}
	for i, src := range sources {
		srcs[i] = view.Source{Child: src.Child, St: src.St, Proj: src.Proj}
		byChild[src.Child] = src
	}
	site, err := view.CollectAll(srcs, timestamp())
	if err != nil {
		return nil, err
	}
	// The Markdown is rendered from the stored values, exactly as export does: it
	// looks entries up by their stored IDs and cleans each value it prints.
	site.MarkdownHeader = renderMarkdownHeader(s.Proj)
	for i := range site.Notes {
		nv := &site.Notes[i]
		src := byChild[nv.Child]
		ps := &session{St: src.St, Proj: src.Proj, Reg: s.Reg, Loc: s.Loc}
		doc := exportDoc{Project: src.Proj, Notes: []model.Note{nv.Note}, Addenda: nv.Addenda}
		md, err := renderMarkdownNotes(ps, doc)
		if err != nil {
			return nil, err
		}
		if nv.Child != "" {
			md = "# child: " + sanitize(nv.Child) + "\n\n" + md
		}
		nv.Markdown = md
	}
	return site, nil
}

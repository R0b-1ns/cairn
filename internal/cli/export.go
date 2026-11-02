package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
	"github.com/R0b-1ns/cairn/internal/validate"
)

func init() {
	register(Command{Name: "export", Usage: "[flags]", Summary: "write notes as Markdown (to read) or JSON (to import)", Define: defineExport})
	register(Command{Name: "import", Usage: "<file|-> [flags]", Summary: "import a JSON export, validated and without ever changing existing data", Define: defineImport})
}

const (
	exportFormat  = "cairn-export"
	exportVersion = 1
)

type exportDoc struct {
	Format  string           `json:"format"`
	Version int              `json:"version"`
	Project model.Project    `json:"project"`
	Notes   []model.Note     `json:"notes"`
	Addenda []model.Addendum `json:"addenda"`
}

func defineExport(fs *flag.FlagSet) Handler {
	var f common
	f.projectFlag(fs)
	var format, task, out string
	var force bool
	fs.StringVar(&format, "format", "md", "md (readable) or json (importable)")
	fs.StringVar(&task, "task", "", "only this task (a base task includes its extensions)")
	fs.StringVar(&out, "out", "", "write to this file instead of standard output (mode 0600)")
	fs.BoolVar(&force, "force", false, "with --out: replace an existing regular file")
	return func(c *Ctx, args []string) int {
		if len(args) != 0 {
			return fail(c, ExitUsage, "export takes no arguments")
		}
		if format != "md" && format != "json" {
			return fail(c, ExitUsage, "--format must be md or json")
		}
		s, code := openProject(c, f.project)
		if s == nil {
			return code
		}
		defer s.Close()
		// JSON stays on the current project so it remains importable; only the
		// Markdown reads the local children.
		sources := []source{{St: s.St, Proj: s.Proj}}
		if format == "md" {
			sc := openScope(c, s)
			defer sc.Close()
			sources = sc.Sources
		}
		type part struct {
			src source
			doc exportDoc
		}
		var parts []part
		for _, src := range sources {
			var nf store.NoteFilter
			if task != "" {
				flt, ok := taskFilter(src.Proj, task)
				if !ok {
					continue // this source numbers its tasks differently
				}
				nf = store.NoteFilter{TaskBase: flt.TaskBase, Task: flt.Task}
			}
			doc, err := collectExport(src, nf)
			if err != nil {
				return fail(c, ExitRuntime, "%v", err)
			}
			parts = append(parts, part{src, doc})
		}
		if len(parts) == 0 {
			return fail(c, ExitUsage, "%q does not match the task pattern %s", task, s.Proj.TaskPattern)
		}
		count := 0
		for _, p := range parts {
			count += len(p.doc.Notes)
		}
		var body string
		var err error
		if format == "json" {
			var sb strings.Builder
			w := &Ctx{Stdout: &sb}
			printJSON(w, parts[0].doc)
			body = sb.String()
		} else {
			var b strings.Builder
			for _, p := range parts {
				ps := &session{St: p.src.St, Proj: p.src.Proj, Reg: s.Reg, Loc: s.Loc}
				var md string
				if p.src.Child == "" {
					md, err = renderMarkdown(ps, p.doc)
				} else {
					b.WriteString("# child: " + sanitize(p.src.Child) + "\n\n")
					md, err = renderMarkdownNotes(ps, p.doc)
				}
				if err != nil {
					return fail(c, ExitRuntime, "%v", err)
				}
				b.WriteString(md)
			}
			body = b.String()
		}
		// A file is read later by tools that do not filter terminal output, so the
		// whole body is cleaned here, whatever the database contained.
		body = sanitize(body)
		if out == "" {
			io.WriteString(c.Stdout, body)
			return ExitOK
		}
		if err := writeNew(out, body, force); err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		fmt.Fprintf(c.Stdout, "exported %d note(s) to %s\n", count, out)
		return ExitOK
	}
}

// lstat is os.Lstat; tests replace it to simulate a swap between check and open.
var lstat = os.Lstat

// writeNew writes an owner-only file. It never follows a symlink, and replaces
// an existing file only with --force and only if it is a regular file.
func writeNew(path, body string, force bool) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL | syscall.O_NOFOLLOW
	var checked os.FileInfo
	if fi, err := lstat(path); err == nil {
		if !force {
			return fmt.Errorf("%s already exists (use --force to replace it)", path)
		}
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file; refusing to replace it", path)
		}
		checked = fi
		flags = os.O_WRONLY | syscall.O_NOFOLLOW // truncate only after the handle is verified
	}
	fh, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return err
	}
	defer fh.Close()
	if checked != nil {
		opened, err := fh.Stat()
		if err != nil || !opened.Mode().IsRegular() || !os.SameFile(checked, opened) {
			return fmt.Errorf("%s changed while opening; refusing to write", path)
		}
		if st, ok := opened.Sys().(*syscall.Stat_t); ok && st.Nlink > 1 {
			return fmt.Errorf("%s has several hard links; refusing to truncate a file shared under another name", path)
		}
		if err := fh.Truncate(0); err != nil {
			return err
		}
	}
	if err := fh.Chmod(0o600); err != nil {
		return err
	}
	_, err = io.WriteString(fh, body)
	return err
}

// collectExport reads the notes and addenda of one source.
func collectExport(src source, nf store.NoteFilter) (exportDoc, error) {
	// The document carries the stored project: an attached child's in-memory
	// default repo must not leak into an export that has to stay importable.
	proj, err := src.St.Project()
	if err != nil {
		return exportDoc{}, err
	}
	doc := exportDoc{Format: exportFormat, Version: exportVersion, Project: proj, Notes: []model.Note{}, Addenda: []model.Addendum{}}
	hs, err := src.St.NoteHeaders(nf)
	if err != nil {
		return doc, err
	}
	for _, h := range hs {
		n, err := src.St.GetNote(h.ID)
		if err != nil {
			return doc, err
		}
		as, err := src.St.Addenda(h.ID)
		if err != nil {
			return doc, err
		}
		doc.Notes = append(doc.Notes, n)
		doc.Addenda = append(doc.Addenda, as...)
	}
	return doc, nil
}

func renderMarkdown(s *session, doc exportDoc) (string, error) {
	notes, err := renderMarkdownNotes(s, doc)
	if err != nil {
		return "", err
	}
	return renderMarkdownHeader(doc.Project) + notes, nil
}

// renderMarkdownHeader is the project name line and the context paragraph.
func renderMarkdownHeader(p model.Project) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", sanitize(p.Name))
	if p.Context != "" {
		fmt.Fprintf(&b, "%s\n\n", sanitize(p.Context))
	}
	return b.String()
}

// joinSanitized cleans each stored value on its own, so an unterminated escape
// sequence in one value cannot swallow the values after it, then joins them.
func joinSanitized(l []string) string {
	out := make([]string, len(l))
	for i, v := range l {
		out[i] = sanitize(v)
	}
	return strings.Join(out, ", ")
}

// renderMarkdownNotes is everything after the header: the notes and their addenda.
func renderMarkdownNotes(s *session, doc exportDoc) (string, error) {
	var b strings.Builder
	entry := func(e model.Entry) (string, error) {
		info, err := s.St.Entry(e.ID)
		if err != nil {
			return "", err
		}
		by := ""
		if e.By != "" {
			by = "[" + sanitize(e.By) + "] "
		}
		return fmt.Sprintf("- `%s` %s%s%s\n", sanitize(e.ID), by, indent(e.Text, "  "), stateMarker(info)), nil
	}
	byAdd := map[string][]model.Addendum{}
	for _, a := range doc.Addenda {
		byAdd[a.NoteID] = append(byAdd[a.NoteID], a)
	}
	for _, n := range doc.Notes {
		fmt.Fprintf(&b, "## %s  %s\n\n", sanitize(n.ID), sanitize(n.Title))
		task := n.Task
		if task == "" {
			task = "-"
		}
		cur, err := s.St.CurrentStatus(n.ID)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "task: %s · repo: %s · agent: %s (%s) · at: %s · status: %s\n\n", sanitize(task), sanitize(n.Repo), sanitize(n.Agent), sanitize(n.Model), sanitize(n.At), sanitize(string(cur)))
		if o := originLine(n.AuthorName, n.AuthorEmail, n.Branch); o != "" {
			fmt.Fprintf(&b, "%s\n\n", o)
		}
		if n.Ref != "" {
			fmt.Fprintf(&b, "ref: %s\n\n", sanitize(n.Ref))
		}
		for _, t := range model.EntryTypes {
			wrote := false
			for _, e := range n.Entries {
				if e.Type != t {
					continue
				}
				if !wrote {
					fmt.Fprintf(&b, "### %s\n\n", t)
					wrote = true
				}
				line, err := entry(e)
				if err != nil {
					return "", err
				}
				b.WriteString(line)
			}
			if wrote {
				b.WriteString("\n")
			}
		}
		if len(n.None) > 0 {
			var ts []string
			for _, t := range n.None {
				ts = append(ts, sanitize(string(t)))
			}
			fmt.Fprintf(&b, "none: %s\n\n", strings.Join(ts, ", "))
		}
		if len(n.Resolves) > 0 {
			fmt.Fprintf(&b, "resolves: %s\n\n", joinSanitized(n.Resolves))
		}
		if len(n.Supersedes) > 0 {
			fmt.Fprintf(&b, "supersedes: %s\n\n", joinSanitized(n.Supersedes))
		}
		if len(n.Impacts) > 0 {
			fmt.Fprintf(&b, "impacts: %s\n\n", joinSanitized(n.Impacts))
		}
		if len(n.Commits) > 0 {
			fmt.Fprintf(&b, "commits: %s\n\n", joinSanitized(n.Commits))
		}
		for _, a := range byAdd[n.ID] {
			fmt.Fprintf(&b, "#### addendum %s · %s (%s) · %s", sanitize(a.ID), sanitize(a.Agent), sanitize(a.Model), sanitize(a.At))
			if a.Status != "" {
				fmt.Fprintf(&b, " · status: %s", sanitize(string(a.Status)))
			}
			b.WriteString("\n\n")
			if o := originLine(a.AuthorName, a.AuthorEmail, a.Branch); o != "" {
				fmt.Fprintf(&b, "- %s\n", o)
			}
			if a.Ref != "" {
				fmt.Fprintf(&b, "- ref: %s\n", sanitize(a.Ref))
			}
			for _, e := range a.Entries {
				line, err := entry(e)
				if err != nil {
					return "", err
				}
				b.WriteString(line)
			}
			if len(a.Resolves) > 0 {
				fmt.Fprintf(&b, "- resolves: %s\n", joinSanitized(a.Resolves))
			}
			if len(a.Supersedes) > 0 {
				fmt.Fprintf(&b, "- supersedes: %s\n", joinSanitized(a.Supersedes))
			}
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

func defineImport(fs *flag.FlagSet) Handler {
	var f common
	f.projectFlag(fs)
	return func(c *Ctx, args []string) int {
		if len(args) != 1 {
			return fail(c, ExitUsage, "usage: cairn import <file|->")
		}
		s, code := openProject(c, f.project)
		if s == nil {
			return code
		}
		defer s.Close()
		var r io.Reader = c.Stdin
		if args[0] != "-" {
			fh, err := os.Open(args[0])
			if err != nil {
				return fail(c, ExitRuntime, "%v", err)
			}
			defer fh.Close()
			r = fh
		}
		data, err := readLimitedN(r, "import", maxImport)
		if err != nil {
			return fail(c, ExitValidation, "%v", err)
		}
		var doc exportDoc
		if err := decodeStrict(data, "import", &doc); err != nil {
			return fail(c, ExitValidation, "%v", err)
		}
		if doc.Format != exportFormat || doc.Version != exportVersion {
			return fail(c, ExitValidation, "import: not a cairn export (format %q version %d; expected %q version %d)", doc.Format, doc.Version, exportFormat, exportVersion)
		}
		var errs []validate.FieldError
		prefix := func(p string, es []validate.FieldError) {
			for _, e := range es {
				errs = append(errs, validate.FieldError{Field: p + "." + e.Field, Message: e.Message})
			}
		}
		needID := func(p, id string, entries []model.Entry) {
			var missing []validate.FieldError
			if id == "" {
				missing = append(missing, validate.FieldError{Field: "id", Message: "is required in an import"})
			}
			for i, e := range entries {
				if e.ID == "" {
					missing = append(missing, validate.FieldError{Field: fmt.Sprintf("entries[%d].id", i), Message: "is required in an import"})
				}
			}
			prefix(p, missing)
		}
		var items []store.ImportItem
		opt := validate.Options{AllowUnknownModel: true}
		// An import must not carry timestamps from the future: they would pin a
		// note's status and ordering ahead of everything written later.
		latest := now().Add(time.Hour)
		notFuture := func(p, at string) {
			if t, err := time.Parse(time.RFC3339, at); err == nil && t.After(latest) {
				errs = append(errs, validate.FieldError{Field: p + ".at", Message: "is in the future"})
			}
		}
		for i := range doc.Notes {
			n := &doc.Notes[i]
			p := fmt.Sprintf("notes[%d]", i)
			needID(p, n.ID, n.Entries)
			if n.NewTask {
				prefix(p, []validate.FieldError{{Field: "new_task", Message: "is input for add only and cannot be imported"}})
			}
			notFuture(p, n.At)
			prefix(p, validate.Note(*n, s.Proj, opt))
			items = append(items, store.ImportItem{Note: n})
			warnSecrets(c, prefixed(p, entryFields(n.Title, n.Entries)))
		}
		for i := range doc.Addenda {
			a := &doc.Addenda[i]
			p := fmt.Sprintf("addenda[%d]", i)
			needID(p, a.ID, a.Entries)
			notFuture(p, a.At)
			prefix(p, validate.Addendum(*a, opt))
			items = append(items, store.ImportItem{Addendum: a})
			warnSecrets(c, prefixed(p, entryFields("", a.Entries)))
		}
		if len(errs) != 0 {
			return reportFieldErrors(c, errs)
		}
		res, err := s.St.Import(items)
		if err != nil {
			if errors.Is(err, store.ErrConflict) {
				return fail(c, ExitRuntime, "import refused, nothing was imported: %v", err)
			}
			return storeFailure(c, err)
		}
		fmt.Fprintf(c.Stdout, "imported %d notes and %d addenda (%d already present)\n", res.Notes, res.Addenda, res.Skipped)
		return ExitOK
	}
}

func prefixed(p string, m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[p+"."+k] = v
	}
	return out
}

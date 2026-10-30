package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
	"github.com/R0b-1ns/cairn/internal/validate"
)

func init() {
	register(Command{Name: "export", Summary: "write notes as Markdown (to read) or JSON (to import)", Define: defineExport})
	register(Command{Name: "import", Summary: "import a JSON export, validated and without ever changing existing data", Define: defineImport})
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
		var nf store.NoteFilter
		if task != "" {
			flt, ok := taskFilter(s.Proj, task)
			if !ok {
				return fail(c, ExitUsage, "%q does not match the task pattern %s", task, s.Proj.TaskPattern)
			}
			nf = store.NoteFilter{TaskBase: flt.TaskBase, Task: flt.Task}
		}
		doc := exportDoc{Format: exportFormat, Version: exportVersion, Project: s.Proj, Notes: []model.Note{}, Addenda: []model.Addendum{}}
		hs, err := s.St.NoteHeaders(nf)
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		for _, h := range hs {
			n, err := s.St.GetNote(h.ID)
			if err == nil {
				var as []model.Addendum
				if as, err = s.St.Addenda(h.ID); err == nil {
					doc.Notes = append(doc.Notes, n)
					doc.Addenda = append(doc.Addenda, as...)
				}
			}
			if err != nil {
				return fail(c, ExitRuntime, "%v", err)
			}
		}
		var body string
		if format == "json" {
			var sb strings.Builder
			w := &Ctx{Stdout: &sb}
			printJSON(w, doc)
			body = sb.String()
		} else if body, err = renderMarkdown(s, doc); err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		if out == "" {
			io.WriteString(c.Stdout, body)
			return ExitOK
		}
		if err := writeNew(out, body, force); err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		fmt.Fprintf(c.Stdout, "exported %d note(s) to %s\n", len(doc.Notes), out)
		return ExitOK
	}
}

// writeNew writes a 0600 file, refusing to follow symlinks or replace anything but a regular file with --force.
func writeNew(path, body string, force bool) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if fi, err := os.Lstat(path); err == nil {
		if !force {
			return fmt.Errorf("%s already exists (use --force to replace it)", path)
		}
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file; refusing to replace it", path)
		}
		flags = os.O_WRONLY | os.O_TRUNC
	}
	fh, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(fh, body); err != nil {
		fh.Close()
		return err
	}
	return fh.Close()
}

func renderMarkdown(s *session, doc exportDoc) (string, error) {
	var b strings.Builder
	p := doc.Project
	fmt.Fprintf(&b, "# %s\n\n", sanitize(p.Name))
	if p.Context != "" {
		fmt.Fprintf(&b, "%s\n\n", sanitize(p.Context))
	}
	entry := func(e model.Entry) (string, error) {
		info, err := s.St.Entry(e.ID)
		if err != nil {
			return "", err
		}
		by := ""
		if e.By != "" {
			by = "[" + e.By + "] "
		}
		return fmt.Sprintf("- `%s` %s%s%s\n", e.ID, by, indent(e.Text, "  "), stateMarker(info)), nil
	}
	byAdd := map[string][]model.Addendum{}
	for _, a := range doc.Addenda {
		byAdd[a.NoteID] = append(byAdd[a.NoteID], a)
	}
	for _, n := range doc.Notes {
		fmt.Fprintf(&b, "## %s  %s\n\n", n.ID, sanitize(n.Title))
		task := n.Task
		if task == "" {
			task = "-"
		}
		cur, err := s.St.CurrentStatus(n.ID)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "task: %s · repo: %s · agent: %s (%s) · at: %s · status: %s\n\n", sanitize(task), sanitize(n.Repo), sanitize(n.Agent), sanitize(n.Model), n.At, cur)
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
				ts = append(ts, string(t))
			}
			fmt.Fprintf(&b, "none: %s\n\n", strings.Join(ts, ", "))
		}
		if len(n.Impacts) > 0 {
			fmt.Fprintf(&b, "impacts: %s\n\n", sanitize(strings.Join(n.Impacts, ", ")))
		}
		if len(n.Commits) > 0 {
			fmt.Fprintf(&b, "commits: %s\n\n", sanitize(strings.Join(n.Commits, ", ")))
		}
		for _, a := range byAdd[n.ID] {
			fmt.Fprintf(&b, "#### addendum %s · %s (%s) · %s", a.ID, sanitize(a.Agent), sanitize(a.Model), a.At)
			if a.Status != "" {
				fmt.Fprintf(&b, " · status: %s", a.Status)
			}
			b.WriteString("\n\n")
			for _, e := range a.Entries {
				line, err := entry(e)
				if err != nil {
					return "", err
				}
				b.WriteString(line)
			}
			if len(a.Resolves) > 0 {
				fmt.Fprintf(&b, "- resolves: %s\n", strings.Join(a.Resolves, ", "))
			}
			if len(a.Supersedes) > 0 {
				fmt.Fprintf(&b, "- supersedes: %s\n", strings.Join(a.Supersedes, ", "))
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
		for i := range doc.Notes {
			n := &doc.Notes[i]
			p := fmt.Sprintf("notes[%d]", i)
			needID(p, n.ID, n.Entries)
			prefix(p, validate.Note(*n, s.Proj, opt))
			n.Title = crlf(n.Title)
			items = append(items, store.ImportItem{Note: n})
			warnSecrets(c, prefixed(p, entryFields(n.Title, n.Entries)))
		}
		for i := range doc.Addenda {
			a := &doc.Addenda[i]
			p := fmt.Sprintf("addenda[%d]", i)
			needID(p, a.ID, a.Entries)
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

package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
	"github.com/R0b-1ns/cairn/internal/tasks"
	"github.com/R0b-1ns/cairn/internal/validate"
)

func init() {
	register(Command{Name: "brief", Usage: "<task-id> [flags]", Summary: "one merged summary of a task: changes, decisions in force, open points", Define: defineBrief})
}

// textWidth is how much of an entry the compact outputs show; `show` prints it all.
const textWidth = 160

// taskFilter turns a task ID into a store filter: a base task includes its
// extensions, an extension matches exactly.
func taskFilter(p model.Project, task string) (store.Filter, bool) {
	base, ext, ok := validate.TaskParts(p, task)
	if !ok {
		return store.Filter{}, false
	}
	if ext == "" {
		return store.Filter{TaskBase: base}, true
	}
	return store.Filter{Task: task}, true
}

// where names the note (and addendum) that holds an entry.
func where(e store.EntryInfo) string {
	if e.AddendumID != "" {
		return e.NoteID + " via " + e.AddendumID
	}
	return e.NoteID
}

func entryLine(e store.EntryInfo) string {
	by := ""
	if e.Type == model.TypeDecision && e.By != "" {
		by = "[" + e.By + "] "
	}
	return fmt.Sprintf("  %s  %s%s  (%s)", e.ID, by, clip(sanitize(e.Text), textWidth), where(e))
}

func defineBrief(fs *flag.FlagSet) Handler {
	var f common
	f.all(fs)
	var branch, author string
	fs.StringVar(&branch, "branch", "", "only notes written on this branch (the note or one of its addenda)")
	fs.StringVar(&author, "author", "", "only notes by this author name or email (the note or one of its addenda)")
	return func(c *Ctx, args []string) int {
		if len(args) != 1 {
			return fail(c, ExitUsage, "usage: cairn brief <task-id>")
		}
		if f.max < 1 {
			return fail(c, ExitUsage, "--max must be 1 or more")
		}
		s, code := openProject(c, f.project)
		if s == nil {
			return code
		}
		defer s.Close()
		flt, ok := taskFilter(s.Proj, args[0])
		if !ok {
			return fail(c, ExitUsage, "%q does not match the task pattern %s", args[0], s.Proj.TaskPattern)
		}
		headers, err := s.St.NoteHeaders(store.NoteFilter{TaskBase: flt.TaskBase, Task: flt.Task, Branch: branch, Author: author})
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		query := func(t model.EntryType, st store.StateFilter) ([]store.EntryInfo, error) {
			q := flt
			q.Branch, q.Author = branch, author
			q.Types, q.State = []model.EntryType{t}, st
			items, _, err := s.St.ListEntries(q)
			for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 { // oldest first reads best
				items[i], items[j] = items[j], items[i]
			}
			return items, err
		}
		changes, err1 := query(model.TypeChange, store.StateAll)
		decisions, err2 := query(model.TypeDecision, store.StateActive)
		opens, err3 := query(model.TypeOpen, store.StateActive)
		for _, err := range []error{err1, err2, err3} {
			if err != nil {
				return fail(c, ExitRuntime, "%v", err)
			}
		}
		refs, err := briefRefs(s, args[0], flt)
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		if f.json {
			out := map[string]any{"task": args[0],
				"notes": nonNil(capped(headers, f.max)), "notes_total": len(headers),
				"changes": nonNil(capped(changes, f.max)), "changes_total": len(changes),
				"decisions": nonNil(capped(decisions, f.max)), "decisions_total": len(decisions),
				"open": nonNil(capped(opens, f.max)), "open_total": len(opens)}
			if len(refs) > 0 {
				out["refs"] = refs // omitted when empty, so older output is unchanged
			}
			printJSON(c, out)
			return ExitOK
		}
		if len(headers) == 0 {
			fmt.Fprintf(c.Stdout, "no notes for task %s\n", args[0])
			return ExitOK
		}
		fmt.Fprintf(c.Stdout, "%s: %d note(s)\n", args[0], len(headers))
		if len(refs) > 0 {
			var clean []string
			for _, r := range refs {
				clean = append(clean, sanitize(r))
			}
			fmt.Fprintf(c.Stdout, "refs: %s\n", strings.Join(clean, ", "))
		}
		fmt.Fprintln(c.Stdout, "notes:")
		var lines []string
		for _, h := range headers {
			task := h.Task
			if task == "" {
				task = "-"
			}
			line := fmt.Sprintf("  %s  %-8s %-11s %s  %s (%s, %s)", h.ID, task, h.Status, day(h.At),
				clip(sanitize(h.Title), textWidth), sanitize(h.Repo), sanitize(h.Agent))
			if o := originLine(h.AuthorName, h.AuthorEmail, h.Branch); o != "" {
				line += " " + o
			}
			lines = append(lines, line)
		}
		printSection(c, lines, f.max)
		for _, sec := range []struct {
			name  string
			items []store.EntryInfo
		}{{"changes", changes}, {"decisions in force", decisions}, {"open points", opens}} {
			fmt.Fprintf(c.Stdout, "%s:\n", sec.name)
			var ls []string
			for _, e := range sec.items {
				ls = append(ls, entryLine(e))
			}
			printSection(c, ls, f.max)
		}
		return ExitOK
	}
}

// briefRefs is every ref stored for the task (newest first), or the project's
// link for the task when none is stored.
func briefRefs(s *session, task string, flt store.Filter) ([]string, error) {
	base := flt.TaskBase
	if base == "" {
		base, _, _ = validate.TaskParts(s.Proj, task)
	}
	refs, err := s.St.TaskRefs(base)
	if err != nil || len(refs) > 0 {
		return refs, err
	}
	if u := tasks.URL(s.Proj.TaskURL, base); u != "" {
		return []string{u}, nil
	}
	return nil, nil
}

func nonNil[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}

// printSection prints at most max lines and says how many were left out.
func printSection(c *Ctx, lines []string, max int) {
	if len(lines) == 0 {
		fmt.Fprintln(c.Stdout, "  none")
		return
	}
	for i, l := range lines {
		if i == max {
			fmt.Fprintf(c.Stdout, "  ... %d more (raise --max)\n", len(lines)-max)
			break
		}
		fmt.Fprintln(c.Stdout, l)
	}
}

// capped returns at most n items.
func capped[T any](v []T, n int) []T {
	if len(v) > n {
		return v[:n]
	}
	return v
}

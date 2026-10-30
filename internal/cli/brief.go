package cli

import (
	"flag"
	"fmt"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
	"github.com/R0b-1ns/cairn/internal/validate"
)

func init() {
	register(Command{Name: "brief", Summary: "one merged summary of a task: changes, decisions in force, open points", Define: defineBrief})
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
		headers, err := s.St.NoteHeaders(store.NoteFilter{TaskBase: flt.TaskBase, Task: flt.Task})
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		query := func(t model.EntryType, st store.StateFilter) ([]store.EntryInfo, error) {
			q := flt
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
		if f.json {
			printJSON(c, map[string]any{"task": args[0], "notes": nonNil(headers), "changes": nonNil(changes),
				"decisions": nonNil(decisions), "open": nonNil(opens)})
			return ExitOK
		}
		if len(headers) == 0 {
			fmt.Fprintf(c.Stdout, "no notes for task %s\n", args[0])
			return ExitOK
		}
		fmt.Fprintf(c.Stdout, "%s: %d note(s)\n", args[0], len(headers))
		fmt.Fprintln(c.Stdout, "notes:")
		var lines []string
		for _, h := range headers {
			task := h.Task
			if task == "" {
				task = "-"
			}
			lines = append(lines, fmt.Sprintf("  %s  %-8s %-11s %s  %s (%s, %s)", h.ID, task, h.Status, h.At[:10],
				clip(sanitize(h.Title), textWidth), sanitize(h.Repo), sanitize(h.Agent)))
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

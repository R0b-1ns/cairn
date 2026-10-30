package cli

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/R0b-1ns/cairn/internal/ids"
	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
)

func init() {
	register(Command{Name: "show", Usage: "<id> [flags]", Summary: "print one note, addendum or entry in full", Define: defineShow})
	register(Command{Name: "list", Usage: "[flags]", Summary: "list entries with combinable filters", Define: defineList})
	register(Command{Name: "recent", Usage: "[N] [flags]", Summary: "the last N subjects handled", Define: defineRecent})
}

// indent prefixes every line of s (already sanitized) so multi-line text stays readable.
func indent(s, prefix string) string {
	return strings.ReplaceAll(sanitize(s), "\n", "\n"+prefix)
}

func stateMarker(e store.EntryInfo) string {
	if !e.Closed {
		return ""
	}
	if e.Type == model.TypeDecision {
		return "  (superseded by " + e.ClosedBy + ")"
	}
	return "  (resolved by " + e.ClosedBy + ")"
}

func fullEntry(e store.EntryInfo, pad string) string {
	by := ""
	if e.Type == model.TypeDecision && e.By != "" {
		by = "[" + e.By + "] "
	}
	return fmt.Sprintf("%s%s  %-8s %s%s%s", pad, e.ID, e.Type, by, indent(e.Text, pad+"    "), stateMarker(e))
}

func defineShow(fs *flag.FlagSet) Handler {
	var f common
	f.projectFlag(fs)
	f.jsonFlag(fs)
	return func(c *Ctx, args []string) int {
		if len(args) != 1 {
			return fail(c, ExitUsage, "usage: cairn show <id>")
		}
		kind := ids.Kind(args[0])
		if kind == 0 {
			return fail(c, ExitUsage, "%q is not a note, addendum or entry ID (n-..., a-..., e-...)", args[0])
		}
		s, code := openProject(c, f.project)
		if s == nil {
			return code
		}
		defer s.Close()
		var err error
		switch kind {
		case 'n':
			err = showNote(c, s, args[0], f.json)
		case 'a':
			err = showAddendum(c, s, args[0], f.json)
		default:
			err = showEntry(c, s, args[0], f.json)
		}
		if errors.Is(err, store.ErrNotFound) {
			return fail(c, ExitRuntime, "%s not found", args[0])
		}
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		return ExitOK
	}
}

// states resolves every entry of a list to its computed state.
func states(s *session, es []model.Entry) ([]store.EntryInfo, error) {
	out := make([]store.EntryInfo, 0, len(es))
	for _, e := range es {
		info, err := s.St.Entry(e.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, info)
	}
	return out, nil
}

func links(c *Ctx, pad string, resolves, supersedes []string) {
	if len(resolves) > 0 {
		fmt.Fprintf(c.Stdout, "%sresolves: %s\n", pad, strings.Join(resolves, ", "))
	}
	if len(supersedes) > 0 {
		fmt.Fprintf(c.Stdout, "%ssupersedes: %s\n", pad, strings.Join(supersedes, ", "))
	}
}

func showNote(c *Ctx, s *session, id string, asJSON bool) error {
	n, err := s.St.GetNote(id)
	if err != nil {
		return err
	}
	cur, err := s.St.CurrentStatus(id)
	if err != nil {
		return err
	}
	add, err := s.St.Addenda(id)
	if err != nil {
		return err
	}
	entries, err := states(s, n.Entries)
	if err != nil {
		return err
	}
	if asJSON {
		printJSON(c, struct {
			model.Note
			CurrentStatus model.Status     `json:"current_status"`
			Addenda       []model.Addendum `json:"addenda"`
		}{n, cur, nonNil(add)})
		return nil
	}
	fmt.Fprintf(c.Stdout, "%s  %s\n", n.ID, sanitize(n.Title))
	task := n.Task
	if task == "" {
		task = "-"
	}
	fmt.Fprintf(c.Stdout, "task: %s   repo: %s   agent: %s   model: %s   at: %s\n", sanitize(task), sanitize(n.Repo), sanitize(n.Agent), sanitize(n.Model), n.At)
	status := "status: " + string(cur)
	if cur != n.Status {
		status += " (set when written: " + string(n.Status) + ")"
	}
	fmt.Fprintln(c.Stdout, status)
	if len(n.Impacts) > 0 {
		fmt.Fprintf(c.Stdout, "impacts: %s\n", sanitize(strings.Join(n.Impacts, ", ")))
	}
	if len(n.Commits) > 0 {
		fmt.Fprintf(c.Stdout, "commits: %s\n", sanitize(strings.Join(n.Commits, ", ")))
	}
	fmt.Fprintln(c.Stdout, "entries:")
	for _, e := range entries {
		fmt.Fprintln(c.Stdout, fullEntry(e, "  "))
	}
	if len(n.None) > 0 {
		var t []string
		for _, x := range n.None {
			t = append(t, string(x))
		}
		fmt.Fprintf(c.Stdout, "none: %s\n", strings.Join(t, ", "))
	}
	links(c, "", n.Resolves, n.Supersedes)
	for _, a := range add {
		fmt.Fprintf(c.Stdout, "\naddendum %s  %s  %s (%s)", a.ID, a.At, sanitize(a.Agent), sanitize(a.Model))
		if a.Status != "" {
			fmt.Fprintf(c.Stdout, "  status: %s", a.Status)
		}
		fmt.Fprintln(c.Stdout)
		es, err := states(s, a.Entries)
		if err != nil {
			return err
		}
		for _, e := range es {
			fmt.Fprintln(c.Stdout, fullEntry(e, "  "))
		}
		links(c, "  ", a.Resolves, a.Supersedes)
	}
	return nil
}

func showAddendum(c *Ctx, s *session, id string, asJSON bool) error {
	a, err := s.St.GetAddendum(id)
	if err != nil {
		return err
	}
	if asJSON {
		printJSON(c, a)
		return nil
	}
	fmt.Fprintf(c.Stdout, "%s  addendum to note %s\n", a.ID, a.NoteID)
	fmt.Fprintf(c.Stdout, "agent: %s   model: %s   at: %s\n", sanitize(a.Agent), sanitize(a.Model), a.At)
	if a.Status != "" {
		fmt.Fprintf(c.Stdout, "status: %s\n", a.Status)
	}
	es, err := states(s, a.Entries)
	if err != nil {
		return err
	}
	for _, e := range es {
		fmt.Fprintln(c.Stdout, fullEntry(e, ""))
	}
	links(c, "", a.Resolves, a.Supersedes)
	return nil
}

func showEntry(c *Ctx, s *session, id string, asJSON bool) error {
	e, err := s.St.Entry(id)
	if err != nil {
		return err
	}
	if asJSON {
		printJSON(c, e)
		return nil
	}
	loc := "in note " + e.NoteID
	if e.AddendumID != "" {
		loc += " via addendum " + e.AddendumID
	}
	fmt.Fprintf(c.Stdout, "%s  %s  %s\n", e.ID, e.Type, loc)
	fmt.Fprintln(c.Stdout, fullEntry(e, ""))
	return nil
}

// parseTime reads YYYY-MM-DD or RFC3339 and returns the exclusive-bound string used by the store.
func parseTime(v string, until bool) (string, error) {
	if d, err := time.Parse("2006-01-02", v); err == nil {
		if until {
			d = d.AddDate(0, 0, 1)
		}
		return d.UTC().Format(time.RFC3339), nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return "", fmt.Errorf("%q is not a date (YYYY-MM-DD or RFC3339)", v)
	}
	if until {
		t = t.Add(time.Second)
	}
	return t.UTC().Format(time.RFC3339), nil
}

func defineList(fs *flag.FlagSet) Handler {
	var f common
	f.all(fs)
	var typ, task, repo, impacts, since, until string
	var resolved, all bool
	fs.StringVar(&typ, "type", "", "entry type: change, decision, open, note or helper")
	fs.StringVar(&task, "task", "", "task ID; a base task includes its extensions, an extension matches exactly")
	fs.StringVar(&repo, "repo", "", "repo the note was written in")
	fs.StringVar(&impacts, "impacts", "", "repo listed in the note's impacts")
	fs.StringVar(&since, "since", "", "only entries at or after this date (YYYY-MM-DD or RFC3339)")
	fs.StringVar(&until, "until", "", "only entries up to and including this date (YYYY-MM-DD or RFC3339)")
	fs.BoolVar(&resolved, "resolved", false, "show only resolved open points and superseded decisions, with what closes them")
	fs.BoolVar(&all, "all", false, "show everything, closed or not")
	return func(c *Ctx, args []string) int {
		if len(args) != 0 {
			return fail(c, ExitUsage, "list takes no arguments (use filters)")
		}
		if f.max < 1 {
			return fail(c, ExitUsage, "--max must be 1 or more")
		}
		if resolved && all {
			return fail(c, ExitUsage, "--resolved and --all cannot be combined")
		}
		var flt store.Filter
		flt.Limit, flt.Repo, flt.Impacts = f.max, repo, impacts
		if typ != "" {
			ok := false
			for _, t := range model.EntryTypes {
				ok = ok || string(t) == typ
			}
			if !ok {
				return fail(c, ExitUsage, "--type must be one of change, decision, open, note, helper")
			}
			flt.Types = []model.EntryType{model.EntryType(typ)}
		}
		switch {
		case resolved:
			flt.State = store.StateClosed
		case all:
			flt.State = store.StateAll
		}
		var err error
		if since != "" {
			if flt.Since, err = parseTime(since, false); err != nil {
				return fail(c, ExitUsage, "--since: %v", err)
			}
		}
		if until != "" {
			if flt.Until, err = parseTime(until, true); err != nil {
				return fail(c, ExitUsage, "--until: %v", err)
			}
		}
		s, code := openProject(c, f.project)
		if s == nil {
			return code
		}
		defer s.Close()
		if task != "" {
			tf, ok := taskFilter(s.Proj, task)
			if !ok {
				return fail(c, ExitUsage, "%q does not match the task pattern %s", task, s.Proj.TaskPattern)
			}
			flt.TaskBase, flt.Task = tf.TaskBase, tf.Task
		}
		items, total, err := s.St.ListEntries(flt)
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		if f.json {
			printJSON(c, nonNil(items))
			return ExitOK
		}
		if len(items) == 0 {
			fmt.Fprintln(c.Stdout, "no entries")
			return ExitOK
		}
		for _, e := range items {
			t := e.Task
			if t == "" {
				t = "-"
			}
			by := ""
			if e.Type == model.TypeDecision && e.By != "" {
				by = "[" + e.By + "] "
			}
			fmt.Fprintf(c.Stdout, "%s  %-8s %s  %s  %s  %s%s%s  %s\n", e.ID, e.Type, sanitize(t), sanitize(e.Repo),
				day(e.At), by, clip(sanitize(e.Text), textWidth), stateMarker(e), e.NoteID)
		}
		if total > len(items) {
			fmt.Fprintf(c.Stdout, "... %d more (raise --max)\n", total-len(items))
		}
		return ExitOK
	}
}

func defineRecent(fs *flag.FlagSet) Handler {
	var f common
	f.projectFlag(fs)
	f.jsonFlag(fs)
	return func(c *Ctx, args []string) int {
		n := 10
		if len(args) > 1 {
			return fail(c, ExitUsage, "usage: cairn recent [N]")
		}
		if len(args) == 1 {
			var err error
			if n, err = strconv.Atoi(args[0]); err != nil || n < 1 {
				return fail(c, ExitUsage, "N must be a positive integer")
			}
		}
		s, code := openProject(c, f.project)
		if s == nil {
			return code
		}
		defer s.Close()
		subs, err := s.St.Subjects(n)
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		type row struct {
			Subject string       `json:"subject"`
			LastAt  string       `json:"last_at"`
			Status  model.Status `json:"status"`
			Title   string       `json:"title"`
			NoteID  string       `json:"note_id"`
			Notes   int          `json:"notes"`
		}
		rows := []row{}
		for _, sub := range subs {
			hs, err := s.St.NoteHeaders(store.NoteFilter{ID: sub.NoteID})
			if err != nil || len(hs) != 1 {
				return fail(c, ExitRuntime, "cannot read note %s: %v", sub.NoteID, err)
			}
			rows = append(rows, row{sub.Key, sub.LastAt, hs[0].Status, hs[0].Title, sub.NoteID, sub.Notes})
		}
		if f.json {
			printJSON(c, rows)
			return ExitOK
		}
		if len(rows) == 0 {
			fmt.Fprintln(c.Stdout, "no notes yet")
			return ExitOK
		}
		for _, r := range rows {
			word := "notes"
			if r.Notes == 1 {
				word = "note"
			}
			fmt.Fprintf(c.Stdout, "%s  %s  %-11s %s  (%d %s)\n", r.Subject, day(r.LastAt), r.Status,
				clip(sanitize(r.Title), textWidth), r.Notes, word)
		}
		return ExitOK
	}
}

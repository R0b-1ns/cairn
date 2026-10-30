package cli

import (
	"flag"
	"fmt"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/validate"
)

func init() {
	register(Command{Name: "add", Usage: "[flags]", Summary: "create a note from flags or JSON on standard input", Define: defineAdd})
}

func defineAdd(fs *flag.FlagSet) Handler {
	var f common
	f.projectFlag(fs)
	f.jsonFlag(fs)
	var agent, repo, task, mdl, title, status, none string
	var change, decision, open, note, helper, impact, commit, resolves, supersedes multi
	var stdin bool
	fs.StringVar(&agent, "agent", "", "agent name (required)")
	fs.StringVar(&repo, "repo", "", "repo the work was done in; must be one of the project's repos (default: the project's default repo)")
	fs.StringVar(&task, "task", "", "task ID matching the project's task pattern (optional)")
	fs.StringVar(&mdl, "model", "", "model that did the work (required)")
	fs.StringVar(&title, "title", "", "short title (required)")
	fs.StringVar(&status, "status", "", "done, in_progress or blocked (required)")
	fs.Var(&change, "change", "summary of what was done (repeatable)")
	fs.Var(&decision, "decision", `a choice with its reason, prefixed by who made it: "user:..." or "agent:..." (repeatable)`)
	fs.Var(&open, "open", "an open point or unresolved question (repeatable)")
	fs.Var(&note, "note", "something worth knowing: pitfall or fact (repeatable)")
	fs.Var(&helper, "helper", "a reusable building block: path and purpose (repeatable)")
	fs.StringVar(&none, "none", "", "comma-separated entry types declared empty, e.g. open,helper")
	fs.Var(&impact, "impact", "repo affected by this work (repeatable or comma-separated)")
	fs.Var(&commit, "commit", "commit hash attached to the note (repeatable or comma-separated)")
	fs.Var(&resolves, "resolves", "ID of an open entry this note resolves (repeatable)")
	fs.Var(&supersedes, "supersedes", "ID of a decision entry this note replaces (repeatable)")
	fs.BoolVar(&stdin, "stdin", false, "read the note as one JSON object from standard input instead of flags")
	return func(c *Ctx, args []string) int {
		if len(args) != 0 {
			return fail(c, ExitUsage, "add takes no arguments")
		}
		if stdin && !onlyAllowed(setFlags(fs), "stdin", "project", "json") {
			return fail(c, ExitUsage, "--stdin cannot be combined with note flags (only --project and --json)")
		}
		s, code := openProject(c, f.project)
		if s == nil {
			return code
		}
		defer s.Close()

		var n model.Note
		var errs []validate.FieldError
		if stdin {
			data, err := readLimited(c.Stdin, "stdin")
			if err != nil {
				return fail(c, ExitValidation, "%v", err)
			}
			if err := decodeStrict(data, "stdin", &n); err != nil {
				return fail(c, ExitValidation, "%v", err)
			}
			errs = assignedByCairn(n.ID, n.At, n.Entries)
			n.ID, n.At = "", ""
			for i := range n.Entries {
				n.Entries[i].ID = ""
			}
		} else {
			n = model.Note{Agent: agent, Repo: repo, Task: task, Model: mdl, Title: title,
				Status: model.Status(status), Resolves: resolves, Supersedes: supersedes}
			n.Entries = entriesFromFlags(change, decision, open, note, helper)
			for _, t := range splitList(none) {
				n.None = append(n.None, model.EntryType(t))
			}
			for _, v := range impact {
				n.Impacts = append(n.Impacts, splitList(v)...)
			}
			for _, v := range commit {
				n.Commits = append(n.Commits, splitList(v)...)
			}
		}
		if n.Repo == "" {
			n.Repo = s.Proj.DefaultRepo
		}
		n.Title = crlf(n.Title)
		normalizeEntries(n.Entries)
		n.At = timestamp()
		errs = append(errs, validate.Note(n, s.Proj, validate.Options{})...)
		if len(errs) != 0 {
			return reportFieldErrors(c, errs)
		}
		warnSecrets(c, entryFields(n.Title, n.Entries))
		out, err := s.St.InsertNote(n)
		if err != nil {
			return storeFailure(c, err)
		}
		if f.json {
			printJSON(c, out)
			return ExitOK
		}
		task = ""
		if out.Task != "" {
			task = out.Task + ", "
		}
		fmt.Fprintf(c.Stdout, "note %s added (%s%s)\n", out.ID, task, out.Status)
		for _, e := range out.Entries {
			fmt.Fprintf(c.Stdout, "  %s %s\n", e.ID, e.Type)
		}
		return ExitOK
	}
}

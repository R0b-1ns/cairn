package cli

import (
	"errors"
	"flag"
	"fmt"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
	"github.com/R0b-1ns/cairn/internal/validate"
)

func init() {
	register(Command{Name: "amend", Summary: "add an addendum to a note (the note itself never changes)", Define: defineAmend})
}

func defineAmend(fs *flag.FlagSet) Handler {
	var f common
	f.projectFlag(fs)
	f.jsonFlag(fs)
	var agent, mdl, status string
	var change, decision, open, note, helper, resolves, supersedes multi
	var stdin bool
	fs.StringVar(&agent, "agent", "", "agent name (required)")
	fs.StringVar(&mdl, "model", "", "model that did the work (required)")
	fs.StringVar(&status, "status", "", "new status: done, in_progress or blocked (optional)")
	fs.Var(&change, "change", "summary of what was done (repeatable)")
	fs.Var(&decision, "decision", `a choice with its reason, prefixed by who made it: "user:..." or "agent:..." (repeatable)`)
	fs.Var(&open, "open", "an open point or unresolved question (repeatable)")
	fs.Var(&note, "note", "something worth knowing (repeatable)")
	fs.Var(&helper, "helper", "a reusable building block (repeatable)")
	fs.Var(&resolves, "resolves", "ID of an open entry this addendum resolves, in any note (repeatable)")
	fs.Var(&supersedes, "supersedes", "ID of a decision entry this addendum replaces, in any note (repeatable)")
	fs.BoolVar(&stdin, "stdin", false, "read the addendum as one JSON object from standard input instead of flags")
	return func(c *Ctx, args []string) int {
		if len(args) != 1 {
			return fail(c, ExitUsage, "usage: cairn amend <note-id> [flags]")
		}
		if stdin && !onlyAllowed(setFlags(fs), "stdin", "project", "json") {
			return fail(c, ExitUsage, "--stdin cannot be combined with addendum flags (only --project and --json)")
		}
		s, code := openProject(c, f.project)
		if s == nil {
			return code
		}
		defer s.Close()

		var a model.Addendum
		var errs []validate.FieldError
		if stdin {
			data, err := readLimited(c.Stdin, "stdin")
			if err != nil {
				return fail(c, ExitValidation, "%v", err)
			}
			if err := decodeStrict(data, "stdin", &a); err != nil {
				return fail(c, ExitValidation, "%v", err)
			}
			errs = assignedByCairn(a.ID, a.At, a.Entries)
			if a.NoteID != "" {
				errs = append(errs, validate.FieldError{Field: "note_id", Message: "is given as the command argument; remove it"})
			}
			a.ID, a.At = "", ""
			for i := range a.Entries {
				a.Entries[i].ID = ""
			}
		} else {
			a = model.Addendum{Agent: agent, Model: mdl, Status: model.Status(status),
				Entries: entriesFromFlags(change, decision, open, note, helper), Resolves: resolves, Supersedes: supersedes}
		}
		a.NoteID = args[0]
		a.At = timestamp()
		normalizeEntries(a.Entries)
		errs = append(errs, validate.Addendum(a, validate.Options{})...)
		if len(errs) != 0 {
			return reportFieldErrors(c, errs)
		}
		warnSecrets(c, entryFields("", a.Entries))
		out, err := s.St.InsertAddendum(a)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return fail(c, ExitRuntime, "note %s not found", a.NoteID)
			}
			return storeFailure(c, err)
		}
		if f.json {
			printJSON(c, out)
			return ExitOK
		}
		fmt.Fprintf(c.Stdout, "addendum %s added to %s\n", out.ID, out.NoteID)
		for _, e := range out.Entries {
			fmt.Fprintf(c.Stdout, "  %s %s\n", e.ID, e.Type)
		}
		return ExitOK
	}
}

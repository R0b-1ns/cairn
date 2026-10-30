package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
)

func init() {
	register(Command{Name: "check", Usage: "[flags]", Summary: "tasks given as done without notes, and old open points", Define: defineCheck})
}

// readDoneFile reads task IDs, one per line; blank lines and # comments are ignored.
func readDoneFile(c *Ctx, path string) ([]string, error) {
	var r io.Reader = c.Stdin
	if path != "-" {
		fh, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer fh.Close()
		r = fh
	}
	data, err := readLimited(r, "done list")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out, nil
}

type taskProblem struct {
	Task   string `json:"task"`
	Detail string `json:"detail"`
}

func defineCheck(fs *flag.FlagSet) Handler {
	var f common
	f.all(fs)
	var done, doneFile string
	var older int
	fs.StringVar(&done, "done", "", "comma-separated task IDs the caller considers done")
	fs.StringVar(&doneFile, "done-file", "", "file of done task IDs, one per line ('-' for standard input; # starts a comment)")
	fs.IntVar(&older, "older-than", 14, "report unresolved open points older than this many days (0 or more)")
	return func(c *Ctx, args []string) int {
		if len(args) != 0 {
			return fail(c, ExitUsage, "check takes no arguments")
		}
		if older < 0 || f.max < 1 {
			return fail(c, ExitUsage, "--older-than must be 0 or more and --max 1 or more")
		}
		tasks := splitList(done)
		if doneFile != "" {
			more, err := readDoneFile(c, doneFile)
			if err != nil {
				return fail(c, ExitRuntime, "%v", err)
			}
			tasks = append(tasks, more...)
		}
		s, code := openProject(c, f.project)
		if s == nil {
			return code
		}
		defer s.Close()
		var missing, notDone []taskProblem
		for _, t := range tasks {
			flt, ok := taskFilter(s.Proj, t)
			if !ok {
				return fail(c, ExitUsage, "%q does not match the task pattern %s", t, s.Proj.TaskPattern)
			}
			hs, err := s.St.NoteHeaders(store.NoteFilter{TaskBase: flt.TaskBase, Task: flt.Task})
			if err != nil {
				return fail(c, ExitRuntime, "%v", err)
			}
			if len(hs) == 0 {
				missing = append(missing, taskProblem{t, "no note for this task"})
			} else if last := hs[len(hs)-1]; last.Status != model.StatusDone {
				notDone = append(notDone, taskProblem{t, fmt.Sprintf("latest note %s is %s", last.ID, last.Status)})
			}
		}
		cutoff := now().UTC().AddDate(0, 0, -older).Format(time.RFC3339)
		old, total, err := s.St.ListEntries(store.Filter{Types: []model.EntryType{model.TypeOpen}, State: store.StateActive, Until: cutoff, Limit: f.max})
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		findings := len(missing) + len(notDone) + total
		if f.json {
			printJSON(c, map[string]any{"missing_notes": nonNil(missing), "not_done": nonNil(notDone), "old_open": nonNil(old), "ok": findings == 0})
		} else {
			printCheck(c, missing, notDone, old, total, older)
		}
		if findings > 0 {
			return ExitFindings
		}
		return ExitOK
	}
}

func printCheck(c *Ctx, missing, notDone []taskProblem, old []store.EntryInfo, total, days int) {
	if len(missing)+len(notDone)+total == 0 {
		fmt.Fprintln(c.Stdout, "ok: nothing to report")
		return
	}
	section := func(title string, ps []taskProblem) {
		if len(ps) == 0 {
			return
		}
		fmt.Fprintf(c.Stdout, "%s:\n", title)
		for _, p := range ps {
			fmt.Fprintf(c.Stdout, "  %s  %s\n", p.Task, p.Detail)
		}
	}
	section("done tasks without notes", missing)
	section("done tasks whose latest note is not done", notDone)
	if total > 0 {
		fmt.Fprintf(c.Stdout, "open points older than %d days:\n", days)
		for _, e := range old {
			fmt.Fprintf(c.Stdout, "  %s  %s  %s  (%s)\n", e.ID, day(e.At), clip(sanitize(e.Text), textWidth), where(e))
		}
		if total > len(old) {
			fmt.Fprintf(c.Stdout, "  ... %d more (raise --max)\n", total-len(old))
		}
	}
}

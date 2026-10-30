package cli

import (
	"flag"
	"fmt"
	"strings"
)

func init() {
	register(Command{Name: "search", Usage: "[flags] [--] <terms...>", Summary: "full-text search over note titles and entries", Define: defineSearch})
}

func defineSearch(fs *flag.FlagSet) Handler {
	var f common
	f.all(fs)
	return func(c *Ctx, args []string) int {
		var terms []string
		for _, a := range args {
			terms = append(terms, strings.Fields(a)...)
		}
		if len(terms) == 0 {
			return fail(c, ExitUsage, "usage: cairn search <terms...> (all terms must match)")
		}
		if f.max < 1 {
			return fail(c, ExitUsage, "--max must be 1 or more")
		}
		s, code := openProject(c, f.project)
		if s == nil {
			return code
		}
		defer s.Close()
		hits, total, err := s.St.Search(terms, f.max)
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		if f.json {
			printJSON(c, nonNil(hits))
			return ExitOK
		}
		if len(hits) == 0 {
			fmt.Fprintln(c.Stdout, "no matches")
			return ExitOK
		}
		for _, h := range hits {
			kind := h.Type
			if h.Kind == "note" {
				kind = "title"
			}
			loc := h.NoteID
			if h.AddendumID != "" {
				loc += " via " + h.AddendumID
			}
			fmt.Fprintf(c.Stdout, "%s  %-8s %s  (%s)\n", h.ID, kind, clip(sanitize(h.Text), textWidth), loc)
		}
		if total > len(hits) {
			fmt.Fprintf(c.Stdout, "... %d more (raise --max)\n", total-len(hits))
		}
		return ExitOK
	}
}

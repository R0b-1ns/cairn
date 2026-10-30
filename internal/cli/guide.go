package cli

import (
	"flag"
	"io"

	"github.com/R0b-1ns/cairn/docs"
)

func init() {
	register(Command{Name: "guide", Usage: "", Summary: "print the one-page usage guide for agents", Define: func(fs *flag.FlagSet) Handler {
		return func(c *Ctx, args []string) int {
			if len(args) != 0 {
				return fail(c, ExitUsage, "guide takes no arguments")
			}
			io.WriteString(c.Stdout, docs.Guide)
			return ExitOK
		}
	}})
}

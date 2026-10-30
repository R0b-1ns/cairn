// Package cli wires command-line flags to the cairn packages.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Version is set at build time with -ldflags "-X ...cli.Version=...".
var Version = "dev"

// Exit codes, documented in docs/exit-codes.md.
const (
	ExitOK         = 0
	ExitRuntime    = 1
	ExitUsage      = 2
	ExitValidation = 3
	ExitNoProject  = 4
	ExitFindings   = 5 // check found problems
)

// Ctx is what every command handler receives.
type Ctx struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Getenv func(string) string
}

// Handler runs a command with its positional arguments (flags already parsed).
type Handler func(c *Ctx, args []string) int

// Command describes one CLI command. Define registers its flags on fs and
// returns the handler; it is also called on a throwaway FlagSet so that tests
// and documentation checks can list every flag.
type Command struct {
	Name    string // may contain one space for sub-commands, e.g. "project move"
	Summary string
	Define  func(fs *flag.FlagSet) Handler
}

var commands []Command

func register(c Command) { commands = append(commands, c) }

// Commands returns the registered commands sorted by name.
func Commands() []Command {
	out := append([]Command(nil), commands...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Run executes cairn and returns the process exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	c := &Ctx{Stdin: stdin, Stdout: safeWriter{stdout}, Stderr: safeWriter{stderr}, Getenv: getenv}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		usage(c)
		return ExitOK
	}
	if args[0] == "--version" || args[0] == "version" {
		fmt.Fprintf(c.Stdout, "cairn %s\n", Version)
		return ExitOK
	}
	cmd, rest := find(args)
	if cmd == nil {
		fmt.Fprintf(c.Stderr, "cairn: unknown command %q (run 'cairn --help')\n", args[0])
		return ExitUsage
	}
	fs := flag.NewFlagSet(cmd.Name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	h := cmd.Define(fs)
	pos, err := parseInterleaved(fs, rest)
	if err != nil {
		if err == flag.ErrHelp {
			commandHelp(c, cmd, fs)
			return ExitOK
		}
		fmt.Fprintf(c.Stderr, "cairn %s: %v\n", cmd.Name, err)
		return ExitUsage
	}
	return h(c, pos)
}

func find(args []string) (*Command, []string) {
	if len(args) >= 2 {
		two := args[0] + " " + args[1]
		for i := range commands {
			if commands[i].Name == two {
				return &commands[i], args[2:]
			}
		}
	}
	for i := range commands {
		if commands[i].Name == args[0] {
			return &commands[i], args[1:]
		}
	}
	return nil, nil
}

// parseInterleaved parses flags that may appear before, between or after
// positional arguments. A literal "--" ends flag parsing.
func parseInterleaved(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		if args[0] == "--" {
			return append(pos, args[1:]...), nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func usage(c *Ctx) {
	fmt.Fprintln(c.Stdout, "cairn: immutable work notes in SQLite, queried in small slices.")
	fmt.Fprintln(c.Stdout, "\nUsage: cairn <command> [flags]\n\nCommands:")
	for _, cmd := range Commands() {
		fmt.Fprintf(c.Stdout, "  %-16s %s\n", cmd.Name, cmd.Summary)
	}
	fmt.Fprintln(c.Stdout, "\nRun 'cairn guide' for the usage guide, 'cairn <command> --help' for flags.")
}

func commandHelp(c *Ctx, cmd *Command, fs *flag.FlagSet) {
	fmt.Fprintf(c.Stdout, "cairn %s: %s\n\nFlags:\n", cmd.Name, cmd.Summary)
	fs.SetOutput(c.Stdout)
	fs.PrintDefaults()
	fs.SetOutput(io.Discard)
}

func osGetenv(k string) string { return os.Getenv(k) }

// Main is the entry point used by cmd/cairn.
func Main() int {
	return Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, osGetenv)
}

func fail(c *Ctx, code int, format string, a ...any) int {
	fmt.Fprintf(c.Stderr, "cairn: "+strings.TrimRight(format, "\n")+"\n", a...)
	return code
}

func errorsIs(err, target error) bool { return errors.Is(err, target) }

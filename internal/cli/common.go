package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/R0b-1ns/cairn/internal/locate"
	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/registry"
	"github.com/R0b-1ns/cairn/internal/store"
	"github.com/R0b-1ns/cairn/internal/validate"
)

// common holds the flags shared by commands that read or write a project.
type common struct {
	project string
	json    bool
	max     int
}

// defaultMax caps list-like output unless --max is given.
const defaultMax = 50

func (f *common) projectFlag(fs *flag.FlagSet) {
	fs.StringVar(&f.project, "project", "", "registered project name (default: found from the current directory; env CAIRN_PROJECT)")
}

func (f *common) jsonFlag(fs *flag.FlagSet) {
	fs.BoolVar(&f.json, "json", false, "print JSON instead of compact text")
}

func (f *common) maxFlag(fs *flag.FlagSet) {
	fs.IntVar(&f.max, "max", defaultMax, "maximum number of items to print (1 or more)")
}

func (f *common) all(fs *flag.FlagSet) {
	f.projectFlag(fs)
	f.jsonFlag(fs)
	f.maxFlag(fs)
}

// session is an opened project.
type session struct {
	St   *store.Store
	Proj model.Project
	Reg  *registry.Registry
}

func (s *session) Close() { s.St.Close() }

func openRegistry(c *Ctx) (*registry.Registry, error) {
	home, err := registry.Home(c.Getenv)
	if err != nil {
		return nil, err
	}
	return registry.Open(home), nil
}

// open finds and opens the project for the current directory. It never creates anything.
func openProject(c *Ctx, projectFlag string) (*session, int) {
	reg, err := openRegistry(c)
	if err != nil {
		return nil, fail(c, ExitRuntime, "%v", err)
	}
	name := projectFlag
	if name == "" {
		name = c.Getenv("CAIRN_PROJECT")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fail(c, ExitRuntime, "cannot read the current directory: %v", err)
	}
	loc, err := locate.Find(cwd, name, reg)
	if err != nil {
		if isNoProject(err) {
			return nil, fail(c, ExitNoProject, "%v; run 'cairn init' to create one, or pass --project NAME", err)
		}
		return nil, fail(c, ExitRuntime, "%v", err)
	}
	st, err := store.Open(loc.DBPath)
	if err != nil {
		return nil, fail(c, ExitRuntime, "%v", err)
	}
	p, err := st.Project()
	if err != nil {
		st.Close()
		return nil, fail(c, ExitRuntime, "%v", err)
	}
	return &session{St: st, Proj: p, Reg: reg}, ExitOK
}

func isNoProject(err error) bool { return errorsIs(err, locate.ErrNoProject) }

// reportFieldErrors prints every invalid field and returns the validation exit code.
func reportFieldErrors(c *Ctx, errs []validate.FieldError) int {
	fmt.Fprintf(c.Stderr, "cairn: invalid input, %d field(s) to fix:\n", len(errs))
	for _, e := range errs {
		fmt.Fprintf(c.Stderr, "  %s: %s\n", e.Field, e.Message)
	}
	return ExitValidation
}

func printJSON(c *Ctx, v any) {
	enc := json.NewEncoder(c.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	enc.Encode(v)
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

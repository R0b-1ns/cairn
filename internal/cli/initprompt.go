package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/prompt"
	"github.com/R0b-1ns/cairn/internal/registry"
	"github.com/R0b-1ns/cairn/internal/tasks"
	"github.com/R0b-1ns/cairn/internal/validate"
)

// initValues points at the values init collects, so the questions can fill
// the ones whose flag was not given.
type initValues struct {
	P        *model.Project
	Repos    *string // comma-separated, as the --repos flag
	Path     *string
	TaskMode *string
}

var errInitCanceled = errors.New("init canceled")

// interruptSignals starts catching Ctrl-C and returns the channel and the
// function that stops it. Tests replace it so no real signal is sent.
var interruptSignals = func() (<-chan os.Signal, func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	return ch, func() { signal.Stop(ch) }
}

// askInit asks, on standard error, the questions whose flag is not in given,
// stores the answers in v and marks the flags it answered in given, so that
// init then treats them like flags. Nothing is written to disk. It ends by
// printing the equivalent command line.
func askInit(c *Ctx, given map[string]bool, v *initValues) error {
	// Ctrl-C during the questions cancels like the end of input: nothing is
	// written. Signals are caught only while the questions run.
	sig, stop := interruptSignals()
	defer stop()
	pr := &prompt.Prompter{In: bufio.NewReader(c.Stdin), Out: c.Stderr, Cancel: sig}
	orig := map[string]bool{}
	for k, b := range given {
		orig[k] = b
	}
	asked := map[string]bool{}
	cwd, _ := os.Getwd()
	ask := func(flagName, label, def string, check func(string) error) (string, error) {
		ans, err := pr.Ask(label, def, check)
		if err != nil {
			return "", errInitCanceled
		}
		asked[flagName], given[flagName] = true, true
		return ans, nil
	}
	p := v.P

	if !orig["name"] {
		def := ""
		base := cwd
		if *v.Path != "" {
			if abs, err := filepath.Abs(*v.Path); err == nil {
				base = abs
			}
		}
		if n := strings.ToLower(filepath.Base(base)); validate.ValidName(n) {
			def = n
		}
		ans, err := ask("name", "Project name", def, func(s string) error {
			if err := projectField("name", func(q *model.Project) { q.Name = s }); err != nil {
				return err
			}
			return nameFree(c, s)
		})
		if err != nil {
			return err
		}
		p.Name = ans
	}
	if !orig["mode"] {
		choices := []prompt.Choice{
			{Value: model.ModeProject, Help: "the database lives in .cairn/ inside the project folder"},
			{Value: model.ModeMachine, Help: "the database lives in cairn's data directory, outside the folder"},
		}
		ans, err := pr.Choose("Where should the database live?", choices, model.ModeProject)
		if err != nil {
			return errInitCanceled
		}
		asked["mode"], given["mode"] = true, true
		p.Mode = ans
	}
	if !orig["path"] {
		ans, err := ask("path", "Project root folder", cwd, func(s string) error {
			root, err := registry.NormalizePath(s)
			if err != nil {
				return err
			}
			if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
				return fmt.Errorf("%s is not a folder", root)
			}
			return nil
		})
		if err != nil {
			return err
		}
		*v.Path = ans
	}
	if !orig["repos"] {
		def := ""
		root := *v.Path
		if root == "" {
			root = cwd
		}
		if _, err := os.Lstat(filepath.Join(root, ".git")); err == nil {
			if projectField("repos", func(q *model.Project) { q.Repos = []string{p.Name} }) == nil {
				def = p.Name
			}
		}
		ans, err := ask("repos", "Repos, comma-separated", def, func(s string) error {
			return projectField("repos", func(q *model.Project) { q.Repos = splitList(s) })
		})
		if err != nil {
			return err
		}
		*v.Repos = ans
	}
	if repos := splitList(*v.Repos); !orig["default-repo"] && len(repos) > 1 {
		ans, err := ask("default-repo", "Default repo, optional (used when add gets no --repo)", "", func(s string) error {
			return projectField("default_repo", func(q *model.Project) { q.Repos, q.DefaultRepo = repos, s })
		})
		if err != nil {
			return err
		}
		p.DefaultRepo = ans
	}
	if !orig["context"] {
		ans, err := ask("context", "Context, one line, optional", "", func(s string) error {
			return projectField("context", func(q *model.Project) { q.Context = s })
		})
		if err != nil {
			return err
		}
		p.Context = ans
	}
	if err := askTasks(pr, given, orig, asked, v); err != nil {
		return err
	}
	fmt.Fprintln(c.Stderr, equivalentCommand(orig, asked, cwd, v))
	return nil
}

func askTasks(pr *prompt.Prompter, given, orig, asked map[string]bool, v *initValues) error {
	p := v.P
	anyTaskFlag := orig["tasks"] || orig["task-pattern"] || orig["task-prefix"] || orig["task-url"]
	if !anyTaskFlag {
		var choices []prompt.Choice
		for _, m := range tasks.Modes() {
			choices = append(choices, prompt.Choice{Value: m, Help: tasks.Describe(m)})
		}
		ans, err := pr.Choose("How do you number your tasks?", choices, tasks.Auto)
		if err != nil {
			return errInitCanceled
		}
		asked["tasks"], given["tasks"] = true, true
		*v.TaskMode = ans
	}
	mode := *v.TaskMode
	if orig["task-pattern"] || !given["tasks"] {
		return nil // the mode comes from the flags, whose rules apply unchanged
	}
	ask := func(flagName, label, def string, check func(string) error) (string, error) {
		ans, err := pr.Ask(label, def, check)
		if err != nil {
			return "", errInitCanceled
		}
		asked[flagName], given[flagName] = true, true
		return ans, nil
	}
	switch mode {
	case tasks.Auto:
		if !orig["task-prefix"] {
			fmt.Fprintln(pr.Out, "Which tag do you want? Enter keeps T (T-1, T-2, ...), or type your own letters, for example FEAT (FEAT-1).")
			ans, err := ask("task-prefix", "Tag", "T", func(s string) error {
				return projectField("task_prefix", func(q *model.Project) { q.TaskMode, q.TaskPrefix = tasks.Auto, s })
			})
			if err != nil {
				return err
			}
			p.TaskPrefix = ans
		}
	case tasks.Custom:
		ans, err := ask("task-pattern", "Task format, a regular expression with a named group (?P<base>...)", "", func(s string) error {
			return projectField("task_pattern", func(q *model.Project) { q.TaskPattern = s })
		})
		if err != nil {
			return err
		}
		p.TaskPattern = ans
	case tasks.Jira, tasks.Linear, tasks.GitHub:
		if !orig["task-url"] {
			example := map[string]string{
				tasks.Jira:   "https://example.atlassian.net/browse/{task}",
				tasks.Linear: "https://linear.app/yourteam/issue/{task}",
				tasks.GitHub: "https://github.com/you/repo/issues/{task}",
			}[mode]
			ans, err := ask("task-url", "Ticket URL template, optional, for example "+example, "", func(s string) error {
				return projectField("task_url", func(q *model.Project) { q.TaskMode, q.TaskURL = tasks.Jira, s })
			})
			if err != nil {
				return err
			}
			p.TaskURL = ans
		}
	}
	return nil
}

// projectField checks one field with validate.Project, so the prompts apply
// exactly the rules of init: set changes a valid baseline project and only
// the errors of field are reported.
func projectField(field string, set func(*model.Project)) error {
	q := model.Project{Name: "demo", Mode: model.ModeProject, Repos: []string{"demo"}, TaskPattern: model.DefaultTaskPattern}
	set(&q)
	for _, e := range validate.Project(q) {
		if e.Field == field {
			return e
		}
	}
	return nil
}

// nameFree refuses a name that is already registered on this machine.
func nameFree(c *Ctx, name string) error {
	reg, err := openRegistry(c)
	if err != nil {
		return nil
	}
	entries, err := reg.List()
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.Name == name {
			return fmt.Errorf("name: project %q is already registered at %s", name, e.Path)
		}
	}
	return nil
}

// equivalentCommand rebuilds the command line that gives the same project:
// the flags that were given, and the answers that differ from a flag's default.
func equivalentCommand(orig, asked map[string]bool, cwd string, v *initValues) string {
	p := v.P
	type kv struct{ flag, val string }
	all := []kv{
		{"name", p.Name}, {"mode", p.Mode}, {"path", *v.Path}, {"repos", strings.Join(splitList(*v.Repos), ",")},
		{"default-repo", p.DefaultRepo}, {"context", p.Context}, {"tasks", *v.TaskMode},
		{"task-prefix", p.TaskPrefix}, {"task-url", p.TaskURL}, {"task-pattern", p.TaskPattern},
	}
	parts := []string{"cairn", "init"}
	for _, f := range all {
		show := orig[f.flag] || (asked[f.flag] && f.val != "")
		if asked[f.flag] && (f.flag == "mode" && f.val == model.ModeProject || f.flag == "path" && f.val == cwd) {
			show = false
		}
		if show {
			parts = append(parts, "--"+f.flag, shellQuote(f.val))
		}
	}
	return "equivalent command: " + strings.Join(parts, " ")
}

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

func shellQuote(s string) string {
	if shellSafe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

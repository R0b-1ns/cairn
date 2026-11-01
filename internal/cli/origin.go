package cli

import (
	"fmt"
	"os"

	"github.com/R0b-1ns/cairn/internal/gitinfo"
	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/tasks"
	"github.com/R0b-1ns/cairn/internal/validate"
)

// resolveOrigin works out who wrote a note and on which branch. Explicit
// values (the flags, then CAIRN_AUTHOR for the author) win and are returned
// as errors when invalid. Otherwise git is asked about the current directory;
// a git value that fails validation is dropped with a warning.
func resolveOrigin(c *Ctx, authorFlag, branchFlag string) (name, email, branch string, errs []validate.FieldError) {
	return resolve(c, authorFlag, branchFlag, true, true)
}

// resolve is resolveOrigin limited to the parts the caller still needs.
func resolve(c *Ctx, authorFlag, branchFlag string, wantAuthor, wantBranch bool) (name, email, branch string, errs []validate.FieldError) {
	author := authorFlag
	if author == "" && wantAuthor {
		author = c.Getenv("CAIRN_AUTHOR")
	}
	if author != "" {
		name, email = gitinfo.ParseAuthor(author)
		errs = append(errs, checkLine("author_name", name, validate.MaxAuthorName)...)
		errs = append(errs, checkLine("author_email", email, validate.MaxAuthorEmail)...)
	}
	if branchFlag != "" {
		branch = branchFlag
		errs = append(errs, checkLine("branch", branch, validate.MaxBranch)...)
	}
	if len(errs) != 0 {
		return "", "", "", errs
	}
	dir, _ := os.Getwd()
	if author == "" && wantAuthor {
		gn, ge := gitinfo.Identity(dir)
		name = gitValue(c, "author_name", gn, validate.MaxAuthorName)
		email = gitValue(c, "author_email", ge, validate.MaxAuthorEmail)
	}
	if branch == "" && wantBranch {
		branch = gitValue(c, "branch", gitinfo.Branch(dir), validate.MaxBranch)
	}
	return name, email, branch, nil
}

func checkLine(field, s string, max int) []validate.FieldError {
	if s == "" {
		return nil
	}
	if e := validate.Text(field, s, max, false); e != nil {
		return []validate.FieldError{*e}
	}
	return nil
}

// gitValue returns v, or "" with a warning when v is not a valid value for field.
func gitValue(c *Ctx, field, v string, max int) string {
	if errs := checkLine(field, v, max); len(errs) != 0 {
		fmt.Fprintf(c.Stderr, "cairn: warning: ignoring the git value for %s: %s\n", field, errs[0].Message)
		return ""
	}
	return v
}

// fillOrigin sets the author and branch of a note or addendum that does not
// carry them yet, using resolveOrigin. It returns the errors to report.
func fillOrigin(c *Ctx, name, email, branch *string, authorFlag, branchFlag string) []validate.FieldError {
	wantAuthor := *name == "" && *email == ""
	wantBranch := *branch == ""
	n, e, b, errs := resolve(c, authorFlag, branchFlag, wantAuthor, wantBranch)
	if len(errs) != 0 {
		return errs
	}
	if wantAuthor {
		*name, *email = n, e
	}
	if wantBranch {
		*branch = b
	}
	return nil
}

// notAutoMessage names the project's task mode for a refused --new-task.
func notAutoMessage(p model.Project) string {
	mode := p.TaskMode
	if mode == "" {
		mode = tasks.Custom
	}
	return fmt.Sprintf("--new-task needs a project in auto mode; this project's task mode is %s", mode)
}

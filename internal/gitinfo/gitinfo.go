// Package gitinfo reads the git branch and identity of a directory.
// Every function returns empty values on any failure.
package gitinfo

import (
	"os"
	"os/exec"
	"strings"
)

// run executes git with an argument list in dir and returns trimmed standard
// output. Standard error is discarded and git never prompts.
func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Branch returns the current branch of the repository containing dir. It is
// empty outside a repository, on a detached HEAD, or when git fails.
func Branch(dir string) string {
	out, err := run(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || out == "HEAD" {
		return ""
	}
	return out
}

// Identity returns git's user.name and user.email as seen from dir.
func Identity(dir string) (name, email string) {
	name, _ = run(dir, "config", "user.name")
	email, _ = run(dir, "config", "user.email")
	return name, email
}

// ParseAuthor splits "Name <email>". Without angle brackets the whole
// string is the name.
func ParseAuthor(s string) (name, email string) {
	s = strings.TrimSpace(s)
	open := strings.Index(s, "<")
	closing := strings.LastIndex(s, ">")
	if open < 0 || closing < open {
		return s, ""
	}
	return strings.TrimSpace(s[:open]), strings.TrimSpace(s[open+1 : closing])
}

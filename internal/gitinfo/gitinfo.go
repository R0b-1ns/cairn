// Package gitinfo reads the git branch and identity of a directory.
// Every function returns empty values on any failure.
package gitinfo

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

// timeout bounds every git call, so a hung git (a stale lock, a slow network
// file system) cannot hang cairn; a call that times out gives empty values.
var timeout = 5 * time.Second

// broken is set when a git call times out: git is then taken as unusable for
// the rest of the process, so a hung git costs one timeout, not one per call.
var broken atomic.Bool

// Reset forgets that git timed out. It exists for tests.
func Reset() { broken.Store(false) }

// run executes git with an argument list in dir and returns trimmed standard
// output. Standard error is discarded and git never prompts.
func run(dir string, args ...string) (string, error) {
	if broken.Load() {
		return "", context.DeadlineExceeded
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	// After the kill, stop waiting for output still held open by git's children.
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			broken.Store(true)
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Branch returns the current branch of the repository containing dir. It is
// empty outside a repository, on a detached HEAD, or when git fails.
func Branch(dir string) string {
	// symbolic-ref works before the first commit and exits 1 on a detached HEAD.
	out, err := run(dir, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
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

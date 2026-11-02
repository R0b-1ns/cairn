package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// bakIgnoreLine keeps the migration backups (.cairn/cairn.db.vN.bak) out of git.
const bakIgnoreLine = ".cairn/*.bak"

// ensureGitignore makes <root>/.gitignore contain the line .cairn/*.bak. It
// never removes or reorders existing lines and refuses a .gitignore that is a
// symlink or not a regular file.
func ensureGitignore(root string) (changed bool, err error) {
	changed, _, err = applyGitignore(root)
	return changed, err
}

// applyGitignore is ensureGitignore that also reports whether it created the file.
func applyGitignore(root string) (changed, created bool, err error) {
	path := filepath.Join(root, ".gitignore")
	fi, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return false, false, err
		}
		if _, err := f.WriteString(bakIgnoreLine + "\n"); err != nil {
			f.Close()
			os.Remove(path)
			return false, false, err
		}
		if err := f.Close(); err != nil {
			os.Remove(path)
			return false, false, err
		}
		return true, true, nil
	case err != nil:
		return false, false, err
	case !fi.Mode().IsRegular():
		return false, false, fmt.Errorf("%s is not a regular file", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, false, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSuffix(line, "\r") == bakIgnoreLine {
			return false, false, nil
		}
	}
	add := bakIgnoreLine + "\n"
	if len(data) > 0 && data[len(data)-1] != '\n' {
		add = "\n" + add
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return false, false, err
	}
	if _, err := f.WriteString(add); err != nil {
		f.Close()
		return false, false, err
	}
	return true, false, f.Close()
}

// removeCreatedGitignore undoes a .gitignore this call created: the file goes
// only while it still holds exactly the line this call wrote.
func removeCreatedGitignore(root string) {
	path := filepath.Join(root, ".gitignore")
	if fi, err := os.Lstat(path); err != nil || !fi.Mode().IsRegular() {
		return
	}
	if data, err := os.ReadFile(path); err == nil && string(data) == bakIgnoreLine+"\n" {
		os.Remove(path)
	}
}

// ignoreBackups runs ensureGitignore for init: a failure only warns.
func ignoreBackups(c *Ctx, root string) {
	changed, _, err := applyGitignore(root)
	if err != nil {
		warnf(c, "could not update .gitignore (%s); add the line %s to %s by hand so database backups are not committed",
			sanitize(err.Error()), bakIgnoreLine, sanitize(filepath.Join(root, ".gitignore")))
		return
	}
	if changed {
		fmt.Fprintf(c.Stdout, ".gitignore now ignores %s\n", bakIgnoreLine)
	}
}

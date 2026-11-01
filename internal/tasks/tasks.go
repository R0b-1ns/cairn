// Package tasks holds the pure logic of task modes: presets that compile to a
// task pattern, automatic numbering and the task URL template.
package tasks

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Task modes.
const (
	Auto   = "auto"
	Jira   = "jira"
	Linear = "linear"
	GitHub = "github"
	Custom = "custom"
)

// MaxNumber is the highest automatic task number the auto pattern accepts.
const MaxNumber = 999999

const ext = `(?:\.(?P<ext>\d{1,3}))?$`

var prefixRe = regexp.MustCompile(`^[A-Z]{1,5}$`)

// Modes lists every mode in the order the init prompt shows them.
func Modes() []string { return []string{Auto, Jira, Linear, GitHub, Custom} }

// Describe returns the one-sentence meaning of a mode, or "" for an unknown mode.
func Describe(mode string) string {
	switch mode {
	case Auto:
		return "cairn numbers your work for you (T-1, T-2, ...). Pick this if you have no ticket tracker, or you work alone."
	case Jira:
		return "Use your Jira keys (PROJ-123). cairn does not talk to Jira; it only checks the format and can link to the ticket."
	case Linear:
		return "Use your Linear IDs (ENG-42)."
	case GitHub:
		return "Use GitHub or GitLab issue numbers (#128)."
	case Custom:
		return "Use your own format, given as a regular expression."
	}
	return ""
}

// Pattern returns the task pattern a mode compiles to. It fails for custom, for
// an unknown mode, and for an auto prefix that is not 1 to 5 capital letters.
func Pattern(mode, prefix string) (string, error) {
	switch mode {
	case Auto:
		if !prefixRe.MatchString(prefix) {
			return "", fmt.Errorf("task prefix %q must match %s", prefix, prefixRe)
		}
		return `^(?P<base>` + prefix + `-\d{1,6})` + ext, nil
	case Jira:
		return `^(?P<base>[A-Z][A-Z0-9]{1,9}-\d{1,6})` + ext, nil
	case Linear:
		return `^(?P<base>[A-Z]{1,10}-\d{1,6})` + ext, nil
	case GitHub:
		return `^(?P<base>#\d{1,7})` + ext, nil
	case Custom:
		return "", fmt.Errorf("mode custom has no preset pattern")
	}
	return "", fmt.Errorf("unknown task mode %q", mode)
}

// Format builds a task ID such as T-7.
func Format(prefix string, n int) string { return prefix + "-" + strconv.Itoa(n) }

// Next returns one more than the highest number used by bases of the given
// prefix, or 1 when there is none.
func Next(prefix string, bases []string) int {
	max := 0
	for _, b := range bases {
		rest, ok := strings.CutPrefix(b, prefix+"-")
		if !ok {
			continue
		}
		if i := strings.IndexByte(rest, '.'); i >= 0 {
			rest = rest[:i]
		}
		if n, err := strconv.Atoi(rest); err == nil && n > max {
			max = n
		}
	}
	return max + 1
}

// URL fills the {task} placeholder of tmpl with the base of task (no extension,
// no leading #). It returns "" when tmpl is empty.
func URL(tmpl, task string) string {
	if tmpl == "" {
		return ""
	}
	base := task
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	base = strings.TrimPrefix(base, "#")
	return strings.ReplaceAll(tmpl, "{task}", base)
}

package validate

import (
	"testing"

	"github.com/R0b-1ns/cairn/internal/model"
)

// TestTaskPartsFrozen runs a fixed set of IDs through the task code path and
// compares with frozen results. The expected values come from the examples
// in docs/configuration.md (default pattern, custom pattern with ext) and were
// checked against the TaskParts of the build before task modes existed; they
// must never be regenerated from the current code. Existing projects depend
// on these results staying identical.
func TestTaskPartsFrozen(t *testing.T) {
	type want struct {
		id, base, ext string
		ok            bool
	}
	cases := []struct {
		name    string
		pattern string
		ids     []want
	}{
		{"default", model.DefaultTaskPattern, []want{
			{"TP-03", "TP-03", "", true},
			{"TP-3", "TP-3", "", true},
			{"TP-0003", "TP-0003", "", true},
			{"TP-03.1", "TP-03", "1", true},
			{"TP-03.001", "TP-03", "001", true},
			{"ABCDE-1234.999", "ABCDE-1234", "999", true},
			{"tp-03", "", "", false},
			{"TP03", "", "", false},
			{"T-03", "", "", false},
			{"ABCDEF-03", "", "", false},
			{"TP-12345", "", "", false},
			{"TP-03.1000", "", "", false},
			{"TP-03.", "", "", false},
			{"TP-03.1.2", "", "", false},
			{" TP-03", "", "", false},
			{"", "", "", false},
		}},
		{"custom with ext", `^(?P<base>[A-Z]+-\d+)(?:/(?P<ext>[a-z]+))?$`, []want{
			{"TP-3", "TP-3", "", true},
			{"TP-3/api", "TP-3", "api", true},
			{"TP-3/API", "", "", false},
			{"tp-3", "", "", false},
			{"TP-3.1", "", "", false},
		}},
		{"custom without ext", `^(?P<base>[A-Z]+-\d+)$`, []want{
			{"TP-3", "TP-3", "", true},
			{"ABC-0042", "ABC-0042", "", true},
			{"TP-3/api", "", "", false},
			{"TP-3.1", "", "", false},
			{"tp-3", "", "", false},
			{"TP-", "", "", false},
		}},
		{"custom unanchored", `(?P<base>[A-Z]+-\d+)`, []want{
			{"TP-3", "TP-3", "", true},
			{"xTP-3", "", "", false}, // the pattern must cover the whole ID
		}},
	}
	for _, c := range cases {
		p := model.Project{TaskPattern: c.pattern, Repos: []string{"server"}}
		for _, w := range c.ids {
			base, ext, ok := TaskParts(p, w.id)
			if base != w.base || ext != w.ext || ok != w.ok {
				t.Errorf("%s: TaskParts(%q) = (%q, %q, %v), want (%q, %q, %v)",
					c.name, w.id, base, ext, ok, w.base, w.ext, w.ok)
			}
			if w.id == "" {
				continue // an empty task means "no task" and is not checked
			}
			n := model.Note{Task: w.id}
			refused := false
			for _, e := range Note(n, p, Options{}) {
				if e.Field == "task" {
					refused = true
				}
			}
			if refused == w.ok {
				t.Errorf("%s: validate.Note task %q refused=%v, want %v", c.name, w.id, refused, !w.ok)
			}
		}
	}
}

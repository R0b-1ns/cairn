package tasks_test

import (
	"testing"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/tasks"
	"github.com/R0b-1ns/cairn/internal/validate"
)

func TestPatternPresets(t *testing.T) {
	cases := []struct {
		mode, prefix, id string
		ok               bool
	}{
		{"jira", "", "PROJ-123", true},
		{"jira", "", "proj-1", false},
		{"github", "", "#128", true},
		{"github", "", "128", false},
		{"github", "", "#128.1", true},
		{"linear", "", "ENG-42", true},
		{"auto", "FEAT", "FEAT-3.1", true},
		{"auto", "FEAT", "T-3", false},
	}
	for _, c := range cases {
		pat, err := tasks.Pattern(c.mode, c.prefix)
		if err != nil {
			t.Fatalf("Pattern(%s,%s): %v", c.mode, c.prefix, err)
		}
		_, _, ok := validate.TaskParts(model.Project{TaskPattern: pat}, c.id)
		if ok != c.ok {
			t.Errorf("%s %q: got %v, want %v", c.mode, c.id, ok, c.ok)
		}
	}
	for _, bad := range [][2]string{{"custom", ""}, {"nope", ""}, {"auto", "t"}, {"auto", "TOOLONG"}, {"auto", ""}} {
		if _, err := tasks.Pattern(bad[0], bad[1]); err == nil {
			t.Errorf("Pattern(%q,%q) should fail", bad[0], bad[1])
		}
	}
}

func TestNextAndFormat(t *testing.T) {
	if got := tasks.Next("T", []string{"T-1", "T-9", "TP-30", "T-3"}); got != 10 {
		t.Errorf("Next = %d, want 10", got)
	}
	if got := tasks.Next("T", nil); got != 1 {
		t.Errorf("Next(nil) = %d, want 1", got)
	}
	if got := tasks.Format("T", 7); got != "T-7" {
		t.Errorf("Format = %q", got)
	}
}

func TestURL(t *testing.T) {
	for _, c := range []struct{ tmpl, base, want string }{
		{"https://x/browse/{task}", "PROJ-12", "https://x/browse/PROJ-12"},
		{"https://g/issues/{task}", "#128", "https://g/issues/128"},
		// the base is used as given: a dot in a custom base is kept
		{"https://x/r/{task}", "REL-1.2", "https://x/r/REL-1.2"},
		// the value is escaped as one path segment
		{"https://x/t/{task}", "A B/c?d#e", "https://x/t/A%20B%2Fc%3Fd%23e"},
		{"", "T-1", ""},
	} {
		if got := tasks.URL(c.tmpl, c.base); got != c.want {
			t.Errorf("URL(%q, %q) = %q, want %q", c.tmpl, c.base, got, c.want)
		}
	}
}

func TestModesAndDescribe(t *testing.T) {
	m := tasks.Modes()
	if len(m) != 5 || m[0] != "auto" || m[4] != "custom" {
		t.Fatalf("Modes = %v", m)
	}
	for _, x := range m {
		if tasks.Describe(x) == "" {
			t.Errorf("no description for %s", x)
		}
	}
	if tasks.Describe("zzz") != "" {
		t.Error("unknown mode should describe as empty")
	}
}

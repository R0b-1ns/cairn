package cli

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
)

// hostile carries a terminal escape (CSI and OSC), a bell and direction and
// invisible characters.
const hostile = "\x1b[31m\x1b]0;pwned\x07\u202e\u200b"

// unsafeRune reports a character that must never reach the output.
func unsafeRune(r rune) bool {
	return r != '\n' && r != '\t' && (r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) ||
		r == 0x2028 || r == 0x2029 || (unicode.Is(unicode.Cf, r) && r != 0x200d))
}

func assertClean(t *testing.T, what, out string) {
	t.Helper()
	for _, r := range out {
		if unsafeRune(r) {
			t.Fatalf("%s carries %U:\n%q", what, r, out)
		}
	}
}

// TestOriginAndRefsAreSanitized stores author, branch, ref and task values
// that validation would refuse, straight through the store, and checks that
// show, brief and export md print them cleaned exactly as sanitize cleans
// every other field, the computed ticket link included.
func TestOriginAndRefsAreSanitized(t *testing.T) {
	e := newEnv(t)
	e.initProject("--task-pattern", `^(?P<base>[^.]+)(?:\.(?P<ext>\d+))?$`, "--task-url", "https://x.example/browse/{task}")
	name, email, branch := "Ada"+hostile+"L", "ada"+hostile+"@x.org", "feat/"+hostile+"x"
	ref := "https://x.example/r/" + hostile + "1"
	task := "TP-" + hostile + "9" // no ref stored: its link is computed

	st, err := store.Open(filepath.Join(e.root, ".cairn", "cairn.db"))
	if err != nil {
		t.Fatal(err)
	}
	ins := func(n model.Note) string {
		t.Helper()
		n.Agent, n.Repo, n.Model, n.Title, n.Status = "builder", "server", "m", "t", model.StatusDone
		n.At = "2026-10-30T10:00:00Z"
		n.Entries = []model.Entry{{Type: model.TypeChange, Text: "x"}}
		out, err := st.InsertNote(n)
		if err != nil {
			t.Fatal(err)
		}
		return out.ID
	}
	withRef := ins(model.Note{Task: "TP-1", AuthorName: name, AuthorEmail: email, Branch: branch, Ref: ref})
	computed := ins(model.Note{Task: task})
	st.Close()

	origin := "by " + sanitize(name) + " <" + sanitize(email) + "> on " + sanitize(branch)
	link := "https://x.example/browse/" + url.PathEscape(task)
	if sanitize(name) != "AdaL" || sanitize(ref) != "https://x.example/r/1" {
		t.Fatalf("sanitize changed: %q %q", sanitize(name), sanitize(ref))
	}

	show := e.mustRun("show", withRef).out
	assertClean(t, "show", show)
	for _, want := range []string{origin + "\n", "ref: " + sanitize(ref) + "\n"} {
		if !strings.Contains(show, want) {
			t.Errorf("show lacks %q:\n%s", want, show)
		}
	}
	show = e.mustRun("show", computed).out
	assertClean(t, "show computed", show)
	if !strings.Contains(show, "ref: "+link+"\n") {
		t.Errorf("show lacks the escaped computed link %q:\n%s", link, show)
	}

	brief := e.mustRun("brief", "TP-1").out
	assertClean(t, "brief", brief)
	for _, want := range []string{"refs: " + sanitize(ref) + "\n", " by " + sanitize(name) + " on " + sanitize(branch) + "\n"} {
		if !strings.Contains(brief, want) {
			t.Errorf("brief lacks %q:\n%s", want, brief)
		}
	}
	brief = e.mustRun("brief", task).out
	assertClean(t, "brief computed", brief)
	if !strings.Contains(brief, "refs: "+link+"\n") {
		t.Errorf("brief lacks the escaped computed link %q:\n%s", link, brief)
	}

	// Markdown goes to a file too, where the terminal filter does not apply.
	path := filepath.Join(realTemp(t), "out.md")
	e.mustRun("export", "--out", path)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for what, md := range map[string]string{"export stdout": e.mustRun("export").out, "export file": string(b)} {
		assertClean(t, what, md)
		for _, want := range []string{"\n" + origin + "\n", "\nref: " + sanitize(ref) + "\n"} {
			if !strings.Contains(md, want) {
				t.Errorf("%s lacks %q:\n%s", what, want, md)
			}
		}
	}
}

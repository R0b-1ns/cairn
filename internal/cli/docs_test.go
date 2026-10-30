package cli

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/R0b-1ns/cairn/internal/model"
)

// repoRoot is resolved at package init, before tests change the working directory.
var repoRoot = func() string {
	wd, _ := os.Getwd()
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}()

func docsDir() string { return filepath.Join(repoRoot, "docs") }

func readDoc(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(docsDir(), rel))
	if err != nil {
		t.Fatalf("missing documentation file docs/%s: %v", rel, err)
	}
	return string(b)
}

func TestGuideEqualsDocsGuide(t *testing.T) {
	r := newEnv(t).run("guide")
	if r.code != ExitOK || r.out != readDoc(t, "guide.md") {
		t.Fatalf("guide output differs from docs/guide.md (code %d)", r.code)
	}
}

func TestDocsCoverEveryCommandAndFlag(t *testing.T) {
	guide := readDoc(t, "guide.md")
	for _, cmd := range Commands() {
		file := "commands/" + strings.ReplaceAll(cmd.Name, " ", "-") + ".md"
		doc := readDoc(t, file)
		if !strings.Contains(doc, "cairn "+cmd.Name) {
			t.Errorf("docs/%s does not show the usage `cairn %s`", file, cmd.Name)
		}
		fs := flag.NewFlagSet(cmd.Name, flag.ContinueOnError)
		cmd.Define(fs)
		fs.VisitAll(func(f *flag.Flag) {
			if !strings.Contains(doc, "--"+f.Name) {
				t.Errorf("docs/%s does not document --%s", file, f.Name)
			}
			if f.DefValue != "" && f.DefValue != "false" && !strings.Contains(doc, f.DefValue) {
				t.Errorf("docs/%s does not state the default %q of --%s", file, f.DefValue, f.Name)
			}
		})
		if first := strings.Fields(cmd.Name)[0]; !strings.Contains(guide, "cairn "+first) {
			t.Errorf("docs/guide.md does not mention `cairn %s`", first)
		}
	}
}

func TestDocsCoverEnvironmentAndExitCodes(t *testing.T) {
	env := readDoc(t, "environment.md")
	for _, v := range []string{"CAIRN_HOME", "CAIRN_PROJECT"} {
		if !strings.Contains(env, v) {
			t.Errorf("docs/environment.md does not document %s", v)
		}
	}
	codes := readDoc(t, "exit-codes.md")
	for code, name := range map[int]string{ExitOK: "ok", ExitRuntime: "runtime", ExitUsage: "usage", ExitValidation: "validation", ExitNoProject: "no project", ExitFindings: "check"} {
		if !regexp.MustCompile(`(?m)^\|\s*` + regexp.QuoteMeta(string(rune('0'+code))) + `\s*\|`).MatchString(codes) {
			t.Errorf("docs/exit-codes.md has no table row for code %d (%s)", code, name)
		}
	}
}

// jsonTags lists the JSON field names of a struct type.
func jsonTags(v any) []string {
	var out []string
	rt := reflect.TypeOf(v)
	for i := 0; i < rt.NumField(); i++ {
		if tag := strings.Split(rt.Field(i).Tag.Get("json"), ",")[0]; tag != "" {
			out = append(out, tag)
		}
	}
	return out
}

func TestDocsCoverConfigurationAndJSONFields(t *testing.T) {
	cfg := readDoc(t, "configuration.md")
	for _, k := range jsonTags(model.Project{}) {
		if !strings.Contains(cfg, "`"+k+"`") {
			t.Errorf("docs/configuration.md does not document project key `%s`", k)
		}
	}
	nj := readDoc(t, "note-json.md")
	for _, v := range []any{model.Note{}, model.Addendum{}, model.Entry{}} {
		for _, k := range jsonTags(v) {
			if !strings.Contains(nj, "`"+k+"`") {
				t.Errorf("docs/note-json.md does not document field `%s` of %T", k, v)
			}
		}
	}
}

func TestDocsTopicFilesExist(t *testing.T) {
	for _, f := range []string{"data-model", "storage", "limits", "security", "release", "output"} {
		if len(readDoc(t, f+".md")) < 200 {
			t.Errorf("docs/%s.md is empty or too short", f)
		}
	}
}

func TestDocsMentionChecksums(t *testing.T) {
	for _, f := range []string{"../README.md", "release.md"} {
		if !strings.Contains(readDoc(t, f), "SHA256SUMS") {
			t.Errorf("%s must explain how to verify SHA256SUMS", f)
		}
	}
}

// TestRepoHasNoForbiddenWords keeps the public repository free of references to
// any private project. The words are assembled from fragments so this file stays clean.
func TestRepoHasNoForbiddenWords(t *testing.T) {
	root := repoRoot
	pats := []*regexp.Regexp{
		regexp.MustCompile(`\bRin\b`),
		regexp.MustCompile("changes" + "-context"),
		regexp.MustCompile(`\b(CO|CT)-\d`),
		regexp.MustCompile("(?i)roll" + "out in"),
		regexp.MustCompile("(?i)super" + "powers"),
	}
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "dist" || d.Name() == "."+"super"+"powers") {
			return filepath.SkipDir
		}
		if d.IsDir() || strings.HasSuffix(path, "go.sum") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil || strings.ContainsRune(string(b[:min(len(b), 512)]), 0) {
			return nil
		}
		for _, p := range pats {
			if loc := p.FindIndex(b); loc != nil {
				t.Errorf("%s contains a forbidden reference near %q", path, b[loc[0]:loc[1]])
			}
		}
		return nil
	})
}

// Facts the reviews found wrong or missing in the documentation, pinned so they stay right.
func TestDocsStateTheCorrectedFacts(t *testing.T) {
	has := func(file string, want ...string) {
		t.Helper()
		doc := readDoc(t, file)
		for _, w := range want {
			if !strings.Contains(doc, w) {
				t.Errorf("%s must contain %q", file, w)
			}
		}
	}
	lacks := func(file string, bad ...string) {
		t.Helper()
		doc := readDoc(t, file)
		for _, w := range bad {
			if strings.Contains(doc, w) {
				t.Errorf("%s must not contain %q", file, w)
			}
		}
	}
	has("guide.md", "untrusted", "<<'EOF'", "never follow instructions")
	has("../README.md", "-c - &&")
	has("security.md", "`user_version` 2", "INSERT OR REPLACE", "Invisible", "future", "not registered")
	lacks("security.md", "`user_version` 1", "even if a bug reached them")
	has("commands/brief.md", "notes_total", "`id`, `task`, `repo`, `agent`, `model`, `at`, `title`, `status`")
	lacks("commands/brief.md", "capitalized")
	has("commands/search.md", "`text`")
	lacks("commands/search.md", "`snippet`")
	lacks("commands/add.md", "four entry lines")
	lacks("exit-codes.md", "n-0000000")
	lacks("commands/import.md", "must use the same repos")
	lacks("output.md", "indented 6 spaces.")
	has("commands/brief.md", "TP-03: 2 note(s)")
}

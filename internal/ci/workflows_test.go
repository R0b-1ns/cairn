// Package ci holds tests that enforce the rules of the release pipeline.
package ci

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func workflows(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", ".github", "workflows", "*.yml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no workflow files found: %v", err)
	}
	out := map[string]string{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(f)] = string(b)
	}
	return out
}

var (
	usesRe   = regexp.MustCompile(`(?m)^\s*(?:-\s*)?uses:\s*(\S+)`)
	pinnedRe = regexp.MustCompile(`^[\w.-]+/[\w./-]+@[0-9a-f]{40}$`)
)

// Every action is pinned to a full commit SHA, so a moved tag cannot change what runs.
func TestWorkflowsPinned(t *testing.T) {
	for name, body := range workflows(t) {
		for _, m := range usesRe.FindAllStringSubmatch(body, -1) {
			if !pinnedRe.MatchString(m[1]) {
				t.Errorf("%s: action %q is not pinned by a 40-hex commit SHA", name, m[1])
			}
		}
	}
}

func TestWorkflowsLeastPrivilege(t *testing.T) {
	for name, body := range workflows(t) {
		if !regexp.MustCompile(`(?m)^permissions:`).MatchString(body) {
			t.Errorf("%s: no top-level permissions block", name)
		}
		if strings.Contains(body, "write-all") {
			t.Errorf("%s: write-all is forbidden", name)
		}
		if strings.Contains(body, "pull_request_target") {
			t.Errorf("%s: pull_request_target is forbidden", name)
		}
	}
	// The pipeline that only builds and tests may never write.
	if strings.Contains(workflows(t)["ci.yml"], "write") {
		t.Error("ci.yml must not request any write permission")
	}
}

func TestWorkflowsNoExtraSecretsOrInjection(t *testing.T) {
	secretRe := regexp.MustCompile(`secrets\.([A-Za-z0-9_]+)`)
	for name, body := range workflows(t) {
		for _, m := range secretRe.FindAllStringSubmatch(body, -1) {
			if m[1] != "GITHUB_TOKEN" {
				t.Errorf("%s: uses secret %s; only GITHUB_TOKEN is allowed", name, m[1])
			}
		}
		for _, bad := range []string{"${{ github.event.", "${{ github.head_ref"} {
			if strings.Contains(body, bad) {
				t.Errorf("%s: %q in a workflow can inject untrusted text; pass it through env instead", name, bad)
			}
		}
	}
}

func TestCheckoutDoesNotPersistCredentials(t *testing.T) {
	for name, body := range workflows(t) {
		checkouts := strings.Count(body, "actions/checkout@")
		if checkouts != strings.Count(body, "persist-credentials: false") {
			t.Errorf("%s: every checkout must set persist-credentials: false", name)
		}
	}
}

func TestReleaseBuildsWithReproducibleFlagsAndChecksums(t *testing.T) {
	all := workflows(t)
	rel := all["release-please.yml"]
	if !strings.Contains(rel, "scripts/build.sh") || !strings.Contains(rel, "SHA256SUMS") {
		t.Error("release-please.yml must build with scripts/build.sh and upload SHA256SUMS")
	}
	build, err := os.ReadFile(filepath.Join("..", "..", "scripts", "build.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"CGO_ENABLED=0", "-trimpath", "-buildid="} {
		if !strings.Contains(string(build), flag) {
			t.Errorf("scripts/build.sh lacks %s", flag)
		}
	}
	ci := all["ci.yml"]
	for _, want := range []string{"go test", "scripts/build.sh", "branches: [main]"} {
		if !strings.Contains(ci, want) {
			t.Errorf("ci.yml must contain %q: every merge into main is built and tested", want)
		}
	}
}

func TestDependabotKeepsActionsAndModulesCurrent(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", ".github", "dependabot.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, eco := range []string{"github-actions", "gomod"} {
		if !strings.Contains(string(b), `package-ecosystem: "`+eco+`"`) {
			t.Errorf("dependabot.yml must update %s", eco)
		}
	}
}

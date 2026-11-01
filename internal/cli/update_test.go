package cli

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeRelease serves a release page whose latest tag is latest and whose
// binary has the given content, listed in SHA256SUMS under sumOf.
func fakeRelease(t *testing.T, latest, binary, sumOf string) string {
	t.Helper()
	asset := "cairn-" + runtime.GOOS + "-" + runtime.GOARCH
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(sumOf)))
	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/tag/"+latest, http.StatusFound)
	})
	mux.HandleFunc("/download/"+latest+"/"+asset, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, binary) })
	mux.HandleFunc("/download/"+latest+"/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n%s  cairn-other\n", sum, asset, strings.Repeat("0", 64))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

// updateSetup points the updater at url and at a stand-in for the running binary.
func updateSetup(t *testing.T, url, version string) string {
	t.Helper()
	exe := filepath.Join(realTemp(t), "cairn")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldBase, oldExe, oldVer := updateBase, updateExe, Version
	updateBase, updateExe, Version = url, func() (string, error) { return exe, nil }, version
	t.Cleanup(func() { updateBase, updateExe, Version = oldBase, oldExe, oldVer })
	return exe
}

func TestUpdateReplacesTheBinary(t *testing.T) {
	exe := updateSetup(t, fakeRelease(t, "v1.2.0", "new binary", "new binary"), "1.0.0")
	r := newEnv(t).run("update")
	if r.code != ExitOK || !strings.Contains(r.out, "updated cairn 1.0.0 -> 1.2.0") {
		t.Fatalf("update: %+v", r)
	}
	b, _ := os.ReadFile(exe)
	st, _ := os.Stat(exe)
	if string(b) != "new binary" || st.Mode().Perm() != 0o755 {
		t.Fatalf("binary = %q mode %v", b, st.Mode())
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(exe), ".cairn-update-*")); len(left) != 0 {
		t.Fatalf("temporary files left behind: %v", left)
	}
}

func TestUpdateRefusesAChecksumMismatch(t *testing.T) {
	exe := updateSetup(t, fakeRelease(t, "v1.2.0", "tampered", "new binary"), "1.0.0")
	r := newEnv(t).run("update")
	if r.code != ExitRuntime || !strings.Contains(r.err, "checksum mismatch") {
		t.Fatalf("update: %+v", r)
	}
	if b, _ := os.ReadFile(exe); string(b) != "old" {
		t.Fatalf("binary was replaced despite the mismatch: %q", b)
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(exe), ".cairn-update-*")); len(left) != 0 {
		t.Fatalf("temporary files left behind: %v", left)
	}
}

func TestUpdateWhenCurrent(t *testing.T) {
	exe := updateSetup(t, fakeRelease(t, "v1.2.0", "new binary", "new binary"), "1.2.0")
	r := newEnv(t).run("update")
	if r.code != ExitOK || !strings.Contains(r.out, "up to date") {
		t.Fatalf("update: %+v", r)
	}
	if b, _ := os.ReadFile(exe); string(b) != "old" {
		t.Fatal("binary changed although already current")
	}
}

func TestUpdateCheckInstallsNothing(t *testing.T) {
	exe := updateSetup(t, fakeRelease(t, "v1.2.0", "new binary", "new binary"), "1.0.0")
	r := newEnv(t).run("update", "--check")
	if r.code != ExitOK || !strings.Contains(r.out, "1.2.0 is available") {
		t.Fatalf("update --check: %+v", r)
	}
	if b, _ := os.ReadFile(exe); string(b) != "old" {
		t.Fatal("--check changed the binary")
	}
}

func TestUpdateRefusesBadInput(t *testing.T) {
	updateSetup(t, fakeRelease(t, "v1.2.0", "x", "x"), "1.0.0")
	e := newEnv(t)
	for _, args := range [][]string{{"update", "--version", "../etc"}, {"update", "--version", "v1.2"}, {"update", "now"}} {
		if r := e.run(args...); r.code != ExitUsage {
			t.Errorf("%v: code %d, want %d (%s)", args, r.code, ExitUsage, r.err)
		}
	}
}

func TestUpdateRefusesADevBuild(t *testing.T) {
	updateSetup(t, fakeRelease(t, "v1.2.0", "x", "x"), "dev")
	if r := newEnv(t).run("update"); r.code != ExitRuntime || !strings.Contains(r.err, "development build") {
		t.Fatalf("update: %+v", r)
	}
}

func initArgs(e *testEnv) []string {
	return []string{"init", "--name", "tidepool", "--repos", "server"}
}

func TestInitSaysWhenANewVersionExists(t *testing.T) {
	updateSetup(t, fakeRelease(t, "v1.2.0", "x", "x"), "1.0.0")
	e := newEnv(t)
	r := e.run(initArgs(e)...)
	if r.code != ExitOK || !strings.Contains(r.out, "initialized project tidepool") {
		t.Fatalf("init: %+v", r)
	}
	if want := "a new version is available (1.2.0, installed: 1.0.0); install it with 'cairn update'"; !strings.Contains(r.err, want) {
		t.Fatalf("stderr = %q, want it to contain %q", r.err, want)
	}
	if strings.Contains(r.out, "new version") {
		t.Fatalf("the notice must stay off standard output: %q", r.out)
	}
}

func TestInitStaysQuietWhenNothingToSay(t *testing.T) {
	cases := map[string]func(t *testing.T, e *testEnv){
		"current": func(t *testing.T, e *testEnv) { updateSetup(t, fakeRelease(t, "v1.2.0", "x", "x"), "1.2.0") },
		"dev":     func(t *testing.T, e *testEnv) { updateSetup(t, fakeRelease(t, "v1.2.0", "x", "x"), "dev") },
		"opt out": func(t *testing.T, e *testEnv) {
			updateSetup(t, fakeRelease(t, "v1.2.0", "x", "x"), "1.0.0")
			e.env["CAIRN_NO_UPDATE_CHECK"] = "1"
		},
		"unreachable": func(t *testing.T, e *testEnv) {
			srv := httptest.NewServer(http.NotFoundHandler())
			srv.Close()
			updateSetup(t, srv.URL, "1.0.0")
		},
		"no redirect": func(t *testing.T, e *testEnv) {
			srv := httptest.NewServer(http.NotFoundHandler())
			t.Cleanup(srv.Close)
			updateSetup(t, srv.URL, "1.0.0")
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			setup(t, e)
			if r := e.run(initArgs(e)...); r.code != ExitOK || r.err != "" {
				t.Fatalf("init: %+v", r)
			}
		})
	}
}

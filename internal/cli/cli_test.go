package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(""), &out, &errb, func(string) string { return "" })
	return code, out.String(), errb.String()
}

func TestVersionFlag(t *testing.T) {
	code, out, _ := run(t, "--version")
	if code != 0 || !strings.HasPrefix(out, "cairn ") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestUnknownCommandExit2(t *testing.T) {
	code, _, errs := run(t, "frobnicate")
	if code != 2 || !strings.Contains(errs, "unknown command") {
		t.Fatalf("code=%d err=%q", code, errs)
	}
}

func TestIsTerminalRefusesNonTerminals(t *testing.T) {
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	if isTerminal(null) {
		t.Error("/dev/null counted as a terminal")
	}
	reg, err := os.Create(filepath.Join(t.TempDir(), "f"))
	if err != nil {
		t.Fatal(err)
	}
	if isTerminal(reg) {
		t.Error("a regular file counted as a terminal")
	}
	reg.Close()
	if isTerminal(reg) {
		t.Error("a closed file counted as a terminal")
	}
	if isTerminal(nil) {
		t.Error("nil counted as a terminal")
	}
	// A real terminal cannot be created portably in a test, so the positive
	// case is not covered here.
}

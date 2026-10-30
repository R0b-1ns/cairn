package cli

import (
	"bytes"
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

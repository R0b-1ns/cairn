package cli

import (
	"strings"
	"testing"
)

func TestSanitizeStripsANSI(t *testing.T) {
	cases := map[string]string{
		"a\x1b[31mred\x1b[0m b": "ared b",
		"\x1b]0;title\x07x":     "x",
		"x\x1b":                 "x",
		"\x00\x07\x08\x7f":      "",
		"a\x00\x07\x08\x7fb":    "ab",
		"a\rb":                  "ab",
		"line1\nline2\tx":       "line1\nline2\tx",
		"é漢 plain text":         "é漢 plain text",
	}
	for in, want := range cases {
		if got := sanitize(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClip(t *testing.T) {
	if got := clip("a\nb\tc  d", 100); got != "a b c d" {
		t.Errorf("clip whitespace = %q, want %q", got, "a b c d")
	}
	if got := clip(strings.Repeat("x", 10), 5); got != "xxxx…" {
		t.Errorf("clip 10 to 5 = %q, want %q", got, "xxxx…")
	}
	if got := clip("éééééé", 4); got != "ééé…" {
		t.Errorf("clip runes = %q, want %q", got, "ééé…")
	}
	for _, s := range []string{"hello", "éé"} {
		if got := clip(s, len([]rune(s))); got != s {
			t.Errorf("clip(%q, len) = %q, want unchanged", s, got)
		}
		if got := clip(s, 100); got != s {
			t.Errorf("clip(%q, 100) = %q, want unchanged", s, got)
		}
	}
}

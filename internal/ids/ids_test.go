package ids

import (
	"regexp"
	"testing"
)

func TestIDsFormat(t *testing.T) {
	for _, kind := range []byte{'n', 'a', 'e'} {
		re := regexp.MustCompile("^" + string(kind) + `-[0-9a-hjkmnp-tv-z]{6}$`)
		for i := 0; i < 100; i++ {
			if id := New(kind); !re.MatchString(id) {
				t.Fatalf("New(%q) = %q, want match of %s", kind, id, re)
			}
		}
	}

	for i := 0; i < 1000; i++ {
		if id := New('e'); !Valid('e', id) {
			t.Fatalf("New('e') = %q, Valid reports false", id)
		}
	}

	if !Valid('n', "n-abcdef") {
		t.Error(`Valid('n', "n-abcdef") = false, want true`)
	}
	for _, s := range []string{"a-abcdef", "n-abc", "n-ABCDEF", "n-abcdef\n"} {
		if Valid('n', s) {
			t.Errorf("Valid('n', %q) = true, want false", s)
		}
	}

	if got := Kind("a-abcdef"); got != 'a' {
		t.Errorf(`Kind("a-abcdef") = %q, want 'a'`, got)
	}
	if got := Kind("zzz"); got != 0 {
		t.Errorf(`Kind("zzz") = %q, want 0`, got)
	}
}

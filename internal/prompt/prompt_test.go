package prompt

import (
	"bufio"
	"errors"
	"strings"
	"testing"
)

func newP(in string) (*Prompter, *strings.Builder) {
	var out strings.Builder
	return &Prompter{In: bufio.NewReader(strings.NewReader(in)), Out: &out}, &out
}

func TestAskDefaultOnEmptyAndSpaces(t *testing.T) {
	p, out := newP("\n   \n")
	for i := 0; i < 2; i++ {
		got, err := p.Ask("Name", "demo", nil)
		if err != nil || got != "demo" {
			t.Fatalf("got %q, %v", got, err)
		}
	}
	if !strings.Contains(out.String(), "Name [demo]: ") {
		t.Fatalf("prompt = %q", out.String())
	}
}

func TestAskRepeatsOnInvalid(t *testing.T) {
	p, out := newP("bad\nworse\ngood\n")
	check := func(s string) error {
		if s != "good" {
			return errors.New("not good enough")
		}
		return nil
	}
	got, err := p.Ask("Word", "", check)
	if err != nil || got != "good" {
		t.Fatalf("got %q, %v", got, err)
	}
	if n := strings.Count(out.String(), "not good enough"); n != 2 {
		t.Fatalf("error shown %d times: %q", n, out.String())
	}
}

func TestAskEmptyWithoutDefaultRepeats(t *testing.T) {
	p, _ := newP("\n\nx\n")
	got, err := p.Ask("Word", "", nil)
	if err != nil || got != "x" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestAskOptionalAcceptsEmpty(t *testing.T) {
	p, _ := newP("\n")
	got, err := p.Ask("Note", "", func(string) error { return nil })
	if err != nil || got != "" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestAskEOFCancels(t *testing.T) {
	p, _ := newP("")
	if _, err := p.Ask("Name", "demo", nil); !errors.Is(err, ErrCanceled) {
		t.Fatalf("err = %v", err)
	}
	p, _ = newP("last")
	if got, err := p.Ask("Name", "", nil); err != nil || got != "last" {
		t.Fatalf("unterminated last line: %q, %v", got, err)
	}
}

func TestChooseByNumberAndValue(t *testing.T) {
	cs := []Choice{{"a", "first"}, {"b", "second"}}
	p, out := newP("2\nb\n\nzzz\n9\nb\n")
	for _, want := range []string{"b", "b", "a", "b"} {
		def := "a"
		got, err := p.Choose("Pick", cs, def)
		if err != nil || got != want {
			t.Fatalf("got %q, %v want %q", got, err, want)
		}
	}
	if !strings.Contains(out.String(), "1) a") || !strings.Contains(out.String(), "second") {
		t.Fatalf("choices not shown: %q", out.String())
	}
	if _, err := p.Choose("Pick", cs, "a"); !errors.Is(err, ErrCanceled) {
		t.Fatalf("err = %v", err)
	}
}

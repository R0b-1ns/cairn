// Package prompt asks questions on a terminal: one line per answer, a default
// in brackets, validation at the prompt. It uses only the standard library.
package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ErrCanceled is returned when the input ends or cannot be read.
var ErrCanceled = errors.New("canceled")

// Prompter reads answers from In and writes prompts and messages to Out.
// One Prompter, with one reader, serves every question of a session so that
// buffered input is not lost between questions.
type Prompter struct {
	In  *bufio.Reader
	Out io.Writer
	// Cancel, when set, ends the question being asked with ErrCanceled as soon
	// as it delivers a value or is closed (Ctrl-C), even while In blocks.
	Cancel <-chan os.Signal

	pending chan result // read still blocked on In after a cancel
}

// Choice is one option of Choose: Value is what the answer returns and Help
// says what it is for.
type Choice struct{ Value, Help string }

type result struct {
	line string
	err  error
}

func (p *Prompter) readLine() (string, error) {
	if p.Cancel == nil {
		return p.read()
	}
	// A read left blocked by an earlier cancel is awaited again rather than
	// replaced, so at most one goroutine ever reads In.
	if p.pending == nil {
		done := make(chan result, 1)
		p.pending = done
		go func() {
			line, err := p.read()
			done <- result{line, err}
		}()
	}
	select {
	case r := <-p.pending:
		p.pending = nil
		return r.line, r.err
	case <-p.Cancel:
		return "", ErrCanceled
	}
}

func (p *Prompter) read() (string, error) {
	line, err := p.In.ReadString('\n')
	if err != nil && (err != io.EOF || line == "") {
		return "", ErrCanceled
	}
	return strings.TrimSpace(line), nil
}

// Ask prints "label [def]: " and returns the answer. An empty or blank line
// gives def. Without a default, an empty answer is accepted only when check
// accepts it (an optional question); otherwise the question is asked again,
// as it is while check rejects the answer, with the reason printed.
func (p *Prompter) Ask(label, def string, check func(string) error) (string, error) {
	for {
		if def != "" {
			fmt.Fprintf(p.Out, "%s [%s]: ", label, def)
		} else {
			fmt.Fprintf(p.Out, "%s: ", label)
		}
		ans, err := p.readLine()
		if err != nil {
			fmt.Fprintln(p.Out)
			return "", err
		}
		if ans == "" && def != "" {
			return def, nil
		}
		if check == nil {
			if ans == "" {
				fmt.Fprintln(p.Out, "an answer is required")
				continue
			}
			return ans, nil
		}
		if err := check(ans); err != nil {
			fmt.Fprintf(p.Out, "%v\n", err)
			continue
		}
		return ans, nil
	}
}

// Choose prints the numbered choices with their help and returns the Value of
// the one picked by number or by value. An empty answer gives def.
func (p *Prompter) Choose(label string, choices []Choice, def string) (string, error) {
	fmt.Fprintln(p.Out, label)
	for i, c := range choices {
		fmt.Fprintf(p.Out, "  %d) %s: %s\n", i+1, c.Value, c.Help)
	}
	ans, err := p.Ask("Your choice", def, func(s string) error {
		if _, ok := pick(choices, s); !ok {
			return fmt.Errorf("choose a number from 1 to %d or one of the names", len(choices))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	v, _ := pick(choices, ans)
	return v, nil
}

func pick(choices []Choice, s string) (string, bool) {
	if n, err := strconv.Atoi(s); err == nil {
		if n >= 1 && n <= len(choices) {
			return choices[n-1].Value, true
		}
		return "", false
	}
	for _, c := range choices {
		if c.Value == s {
			return c.Value, true
		}
	}
	return "", false
}

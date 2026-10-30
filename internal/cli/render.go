package cli

import (
	"strings"
	"unicode/utf8"
)

// sanitize removes ANSI escape sequences and every control character except
// newline and tab, so stored text can never drive the reader's terminal.
func sanitize(s string) string {
	var b strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == 0x1b:
			i = skipEscape(rs, i)
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) || r == 0x2028 || r == 0x2029:
			// dropped
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// skipEscape returns the index of the last rune of the escape sequence starting at i.
func skipEscape(rs []rune, i int) int {
	if i+1 >= len(rs) {
		return i
	}
	switch c := rs[i+1]; {
	case c == '[': // CSI: parameters, then a final byte 0x40-0x7e
		for j := i + 2; j < len(rs); j++ {
			if rs[j] >= 0x40 && rs[j] <= 0x7e {
				return j
			}
		}
		return len(rs) - 1
	case c == ']' || c == 'P' || c == 'X' || c == '^' || c == '_': // string sequences end at BEL or ESC \
		for j := i + 2; j < len(rs); j++ {
			if rs[j] == 0x07 {
				return j
			}
			if rs[j] == 0x1b && j+1 < len(rs) && rs[j+1] == '\\' {
				return j + 1
			}
		}
		return len(rs) - 1
	case c >= 0x40 && c <= 0x5f: // two-character sequence
		return i + 1
	}
	return i // bare ESC: drop it only
}

// clip makes s a single line (newlines and tabs become spaces, runs of spaces
// collapse) and cuts it to at most n runes, ending with "…" when cut.
func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	if n < 1 {
		return ""
	}
	return string([]rune(s)[:n-1]) + "…"
}

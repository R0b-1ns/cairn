// Package ids generates and checks short stable identifiers.
package ids

import (
	"crypto/rand"
	"strings"
)

// alphabet is Crockford base32, lowercase, without i, l, o, u.
const alphabet = "0123456789abcdefghjkmnpqrstvwxyz"

const idLen = 6

// New returns a new random ID for kind 'n' (note), 'a' (addendum) or 'e' (entry),
// for example "n-4k7q2m".
func New(kind byte) string {
	var raw [idLen]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic("cairn: no randomness available: " + err.Error())
	}
	b := make([]byte, 0, idLen+2)
	b = append(b, kind, '-')
	for _, r := range raw {
		b = append(b, alphabet[int(r)&31])
	}
	return string(b)
}

// Valid reports whether s is a well-formed ID of the given kind.
func Valid(kind byte, s string) bool {
	return len(s) == idLen+2 && s[0] == kind && Kind(s) == kind
}

// Kind returns the kind byte of a well-formed ID, or 0.
func Kind(s string) byte {
	if len(s) != idLen+2 || s[1] != '-' {
		return 0
	}
	if s[0] != 'n' && s[0] != 'a' && s[0] != 'e' {
		return 0
	}
	for i := 2; i < len(s); i++ {
		if !strings.ContainsRune(alphabet, rune(s[i])) {
			return 0
		}
	}
	return s[0]
}

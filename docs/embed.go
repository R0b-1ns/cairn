// Package docs embeds the usage guide so the binary and the documentation cannot drift apart.
package docs

import _ "embed"

// Guide is the one-page usage guide printed by `cairn guide`.
//
//go:embed guide.md
var Guide string

// Package secrets detects common secret patterns so the tool can warn.
package secrets

import (
	"fmt"
	"regexp"
	"sort"
)

// Finding says which rule matched in which field. It never carries the match.
type Finding struct{ Rule, Field string }

var rules = []struct {
	name string
	re   *regexp.Regexp
}{
	{"aws-access-key", regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{"private-key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{"github-token", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,}\b|\bgithub_pat_[A-Za-z0-9_]{22,}\b`)},
	{"slack-token", regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`)},
	{"jwt", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`)},
	{"credential-assignment", regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:[a-z0-9]+[_-])*(?:password|passwd|secret|token|api[_-]?key)(?:[_-][a-z0-9]+)*["']?\s*[:=]\s*["']?[^\s"']{8,}`)},
	{"url-credentials", regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]*://[^\s/:@]+:[^\s/@]{3,}@`)},
	{"api-key", regexp.MustCompile(`\b(?:sk-ant-|sk_live_|sk_test_)[A-Za-z0-9_-]{16,}|\bsk-[A-Za-z0-9]{32,}`)},
}

// Scan checks each field value and returns findings sorted by field then rule.
func Scan(fields map[string]string) []Finding {
	var out []Finding
	for field, value := range fields {
		for _, r := range rules {
			if r.re.MatchString(value) {
				out = append(out, Finding{Rule: r.name, Field: field})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Field != out[j].Field {
			return out[i].Field < out[j].Field
		}
		return out[i].Rule < out[j].Rule
	})
	return out
}

// Warning renders a finding for stderr without echoing the matched text.
func Warning(f Finding) string {
	return fmt.Sprintf("warning: possible secret (%s) in %s; notes are stored unencrypted and may be shared", f.Rule, f.Field)
}

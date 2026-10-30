package secrets

import (
	"strings"
	"testing"
)

func hasRule(fs []Finding, rule string) bool {
	for _, f := range fs {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

// Token-looking literals are built from fragments so the source does not
// contain a complete secret-shaped string.
func TestDetectsKnownPatterns(t *testing.T) {
	cases := []struct {
		rule, value string
	}{
		{"aws-access-key", "AKIA" + "IOSFODNN7EXAMPLE"},
		{"private-key", "-----BEGIN " + "RSA PRIVATE KEY-----"},
		{"github-token", "ghp_" + "abcdefghijklmnopqrstuvwxyz0123456789"},
		{"slack-token", "xox" + "b-123456789012-abcdefABCDEF"},
		{"jwt", "eyJhbGciOiJIUzI1NiJ9" + ".eyJzdWIiOiIxMjM0NTY3ODkwIn0" + ".abc123def456"},
		{"credential-assignment", "password = " + "hunter2hunter2"},
		{"credential-assignment", "api_key: " + "abcd1234efgh5678"},
	}
	for _, c := range cases {
		fs := Scan(map[string]string{"field": c.value})
		if !hasRule(fs, c.rule) {
			t.Errorf("Scan(%s value) = %v, want rule %q", c.rule, fs, c.rule)
		}
	}
}

func TestNoFalsePositives(t *testing.T) {
	for _, v := range []string{"we chose a token bucket design", "password=short"} {
		if fs := Scan(map[string]string{"field": v}); len(fs) != 0 {
			t.Errorf("Scan(%q) = %v, want no findings", v, fs)
		}
	}
}

func TestFindingsSortedAndFieldNamed(t *testing.T) {
	fs := Scan(map[string]string{
		"notes": "password=" + "hunter2hunter2",
		"body":  "api_key: " + "abcd1234efgh5678",
	})
	if len(fs) < 2 {
		t.Fatalf("Scan returned %d findings, want at least 2: %v", len(fs), fs)
	}
	for i := 1; i < len(fs); i++ {
		if fs[i-1].Field > fs[i].Field {
			t.Errorf("findings not sorted by field: %v", fs)
		}
	}
	if fs[0].Field != "body" {
		t.Errorf("first finding field = %q, want %q", fs[0].Field, "body")
	}
	if fs[len(fs)-1].Field != "notes" {
		t.Errorf("last finding field = %q, want %q", fs[len(fs)-1].Field, "notes")
	}
}

func TestWarningDoesNotEchoSecret(t *testing.T) {
	secret := "hunter2hunter2"
	fs := Scan(map[string]string{"notes": "password = " + secret})
	if len(fs) == 0 {
		t.Fatal("Scan found nothing, cannot check Warning")
	}
	for _, f := range fs {
		w := Warning(f)
		if !strings.Contains(w, f.Rule) {
			t.Errorf("Warning(%v) = %q, missing rule %q", f, w, f.Rule)
		}
		if !strings.Contains(w, f.Field) {
			t.Errorf("Warning(%v) = %q, missing field %q", f, w, f.Field)
		}
		if strings.Contains(w, secret) {
			t.Errorf("Warning(%v) = %q, echoes the secret", f, w)
		}
	}
}

func TestDetectsPrefixedAndUrlCredentials(t *testing.T) {
	cases := map[string]string{
		"credential-assignment": "DB_" + "PASSWORD=hunter2hunter2",
	}
	for rule, v := range cases {
		if !hasRule(Scan(map[string]string{"f": v}), rule) {
			t.Errorf("%q not detected as %s", v, rule)
		}
	}
	for _, v := range []string{"GITHUB_" + "TOKEN: abcdef123456", "AWS_SECRET_" + "ACCESS_KEY=abcdefgh12345678", "export API_" + "KEY=abcdefgh12345678"} {
		if !hasRule(Scan(map[string]string{"f": v}), "credential-assignment") {
			t.Errorf("%q not detected", v)
		}
	}
	if !hasRule(Scan(map[string]string{"f": "postgres://user:" + "s3cretpw@db.example.com/x"}), "url-credentials") {
		t.Error("URL credentials not detected")
	}
	for _, v := range []string{"sk-ant-" + "api03-abcdefghijklmnopqrstuvwx", "sk_live_" + "abcdefghijklmnopqrstuvwx"} {
		if !hasRule(Scan(map[string]string{"f": v}), "api-key") {
			t.Errorf("%q not detected as api-key", v)
		}
	}
	for _, v := range []string{"the token bucket resets", "https://example.com/path", "user@example.com", "mailto:a@b.c"} {
		if fs := Scan(map[string]string{"f": v}); len(fs) != 0 {
			t.Errorf("%q wrongly flagged: %v", v, fs)
		}
	}
}

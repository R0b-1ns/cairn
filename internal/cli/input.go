package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/secrets"
	"github.com/R0b-1ns/cairn/internal/store"
	"github.com/R0b-1ns/cairn/internal/validate"
)

// maxInput is the largest standard input or import file cairn will read.
const maxInput = 1 << 20

// maxImport is the largest export file `import` will read: whole projects are bigger than one note.
const maxImport = 32 << 20

// multi is a repeatable string flag.
type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

// readLimited reads at most maxInput bytes and fails when there is more.
func readLimited(r io.Reader, what string) ([]byte, error) { return readLimitedN(r, what, maxInput) }

// readLimitedN is readLimited with an explicit limit.
func readLimitedN(r io.Reader, what string, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %v", what, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s: input too large (max %d bytes)", what, limit)
	}
	return data, nil
}

// decodeStrict decodes exactly one JSON object into v. Unknown fields, trailing
// data, a leading non-object value and empty input are all refused.
func decodeStrict(data []byte, what string, v any) error {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return fmt.Errorf("%s: empty input, expected a JSON object", what)
	}
	if data[0] != '{' {
		return fmt.Errorf("%s: expected a JSON object", what)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%s: invalid JSON: %v", what, err)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s: unexpected data after the JSON document", what)
	}
	return nil
}

func crlf(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

func normalizeEntries(es []model.Entry) {
	for i := range es {
		es[i].Text = crlf(es[i].Text)
	}
}

// assignedByCairn lists ids and timestamps that callers may not supply on add/amend.
func assignedByCairn(id, at string, entries []model.Entry) []validate.FieldError {
	var errs []validate.FieldError
	if id != "" {
		errs = append(errs, validate.FieldError{Field: "id", Message: "is assigned by cairn; remove it"})
	}
	if at != "" {
		errs = append(errs, validate.FieldError{Field: "at", Message: "is set by cairn; remove it"})
	}
	for i, e := range entries {
		if e.ID != "" {
			errs = append(errs, validate.FieldError{Field: fmt.Sprintf("entries[%d].id", i), Message: "is assigned by cairn; remove it"})
		}
	}
	return errs
}

// warnSecrets prints a value-free warning for each possible secret.
func warnSecrets(c *Ctx, fields map[string]string) {
	for _, f := range secrets.Scan(fields) {
		fmt.Fprintln(c.Stderr, "cairn: "+secrets.Warning(f))
	}
}

func entryFields(title string, entries []model.Entry) map[string]string {
	m := map[string]string{}
	if title != "" {
		m["title"] = title
	}
	for i, e := range entries {
		m[fmt.Sprintf("entries[%d].text", i)] = e.Text
	}
	return m
}

// storeFailure converts a store error to an exit code, listing link problems as fields.
func storeFailure(c *Ctx, err error) int {
	var le *store.LinkError
	if errors.As(err, &le) {
		return reportFieldErrors(c, []validate.FieldError{{Field: le.Field, Message: le.Target + ": " + le.Reason}})
	}
	return fail(c, ExitRuntime, "%v", err)
}

// setFlags returns the names of the flags that were given on the command line.
func setFlags(fs *flag.FlagSet) map[string]bool {
	m := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { m[f.Name] = true })
	return m
}

// onlyAllowed reports whether every given flag is in allowed.
func onlyAllowed(given map[string]bool, allowed ...string) bool {
	ok := map[string]bool{}
	for _, a := range allowed {
		ok[a] = true
	}
	for k := range given {
		if !ok[k] {
			return false
		}
	}
	return true
}

// entriesFromFlags orders flag entries: changes, decisions, open points, notes, helpers.
// A decision text carries its author as a "user:" or "agent:" prefix.
func entriesFromFlags(change, decision, open, note, helper []string) []model.Entry {
	var out []model.Entry
	for _, t := range change {
		out = append(out, model.Entry{Type: model.TypeChange, Text: t})
	}
	for _, t := range decision {
		e := model.Entry{Type: model.TypeDecision, Text: t}
		for _, by := range []string{"user", "agent"} {
			if strings.HasPrefix(t, by+":") {
				e.By, e.Text = by, strings.TrimSpace(strings.TrimPrefix(t, by+":"))
			}
		}
		out = append(out, e)
	}
	for _, t := range open {
		out = append(out, model.Entry{Type: model.TypeOpen, Text: t})
	}
	for _, t := range note {
		out = append(out, model.Entry{Type: model.TypeNote, Text: t})
	}
	for _, t := range helper {
		out = append(out, model.Entry{Type: model.TypeHelper, Text: t})
	}
	return out
}

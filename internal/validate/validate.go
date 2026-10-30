// Package validate checks every piece of untrusted input before it reaches storage.
package validate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/R0b-1ns/cairn/internal/ids"
	"github.com/R0b-1ns/cairn/internal/model"
)

// Limits applied to input.
const (
	MaxTitle      = 200
	MaxEntryText  = 4000
	MaxEntries    = 100
	MaxListItems  = 50
	MaxContext    = 2000
	MaxLine       = 80
	MaxPatternLen = 200
	MaxRepos      = 50
)

// FieldError names an invalid field and says what is wrong.
type FieldError struct{ Field, Message string }

func (e FieldError) Error() string { return e.Field + ": " + e.Message }

// Options relaxes rules for trusted-format imports.
type Options struct{ AllowUnknownModel bool }

var (
	nameRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)
	repoRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,39}$`)
	commitRe = regexp.MustCompile(`^[0-9a-f]{7,64}$`)
)

// Text checks UTF-8, control characters and length. Newline and tab are allowed
// only when multiline is true. Returns nil when s is acceptable.
func Text(field, s string, max int, multiline bool) *FieldError {
	if !utf8.ValidString(s) {
		return &FieldError{field, "must be valid UTF-8"}
	}
	if utf8.RuneCountInString(s) > max {
		return &FieldError{field, fmt.Sprintf("too long (max %d characters)", max)}
	}
	for _, r := range s {
		if multiline && (r == '\n' || r == '\t') {
			continue
		}
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return &FieldError{field, "contains a control character"}
		}
		// Invisible and direction-changing characters (bidi marks, zero-width
		// spaces, tag characters) can hide or reorder text; ZWJ is kept for emoji.
		if unicode.Is(unicode.Cf, r) && r != '\u200d' {
			return &FieldError{field, "contains an invisible or direction-changing character"}
		}
	}
	return nil
}

type collector []FieldError

func (c *collector) add(e *FieldError) {
	if e != nil {
		*c = append(*c, *e)
	}
}

func (c *collector) addf(field, format string, a ...any) {
	*c = append(*c, FieldError{field, fmt.Sprintf(format, a...)})
}

func (c *collector) line(field, s string, max int) {
	if strings.TrimSpace(s) == "" {
		c.addf(field, "is required")
		return
	}
	c.add(Text(field, s, max, false))
}

func (c *collector) idList(field string, kind byte, list []string) {
	if len(list) > MaxListItems {
		c.addf(field, "too many items (max %d)", MaxListItems)
		return
	}
	for _, id := range list {
		if !ids.Valid(kind, id) {
			c.addf(field, "%s is not a valid entry ID", quote(id))
		}
	}
}

func validStatus(s model.Status) bool {
	return s == model.StatusDone || s == model.StatusInProgress || s == model.StatusBlocked
}

func validType(t model.EntryType) bool {
	for _, v := range model.EntryTypes {
		if v == t {
			return true
		}
	}
	return false
}

func (c *collector) entries(entries []model.Entry) {
	if len(entries) > MaxEntries {
		c.addf("entries", "too many entries (max %d)", MaxEntries)
		return
	}
	for i, e := range entries {
		f := fmt.Sprintf("entries[%d]", i)
		if e.ID != "" && !ids.Valid('e', e.ID) {
			c.addf(f+".id", "%s is not a valid entry ID", quote(e.ID))
		}
		if !validType(e.Type) {
			c.addf(f+".type", "%s is not one of change, decision, open, note, helper", quote(string(e.Type)))
		}
		switch strings.ToLower(strings.TrimSpace(e.Text)) {
		case "":
			c.addf(f+".text", "is required")
		case "none":
			c.addf(f+".text", `"none" is not an entry: declare empty sections in the none field`)
		default:
			c.add(Text(f+".text", e.Text, MaxEntryText, true))
		}
		switch {
		case e.Type == model.TypeDecision && e.By != "user" && e.By != "agent":
			c.addf(f+".by", `a decision needs by "user" or "agent"`)
		case e.Type != model.TypeDecision && e.By != "":
			c.addf(f+".by", "only decisions carry by")
		}
	}
}

func (c *collector) timestamp(at string) {
	if _, err := time.Parse(time.RFC3339, at); err != nil {
		c.addf("at", "must be an RFC3339 timestamp, e.g. 2026-10-30T10:00:00Z")
	}
}

func (c *collector) actor(a, m string, o Options) {
	c.line("agent", a, MaxLine)
	c.line("model", m, MaxLine)
	if m == model.UnknownModel && !o.AllowUnknownModel {
		c.addf("model", `"unknown" is reserved for imported notes`)
	}
}

// Note validates a note against its project and returns every problem found.
func Note(n model.Note, p model.Project, o Options) []FieldError {
	var c collector
	if n.ID != "" && !ids.Valid('n', n.ID) {
		c.addf("id", "%s is not a valid note ID", quote(n.ID))
	}
	c.actor(n.Agent, n.Model, o)
	c.line("title", n.Title, MaxTitle)
	c.timestamp(n.At)
	if !validStatus(n.Status) {
		c.addf("status", "must be done, in_progress or blocked")
	}
	if n.Repo == "" {
		c.addf("repo", "is required: pass --repo (allowed: %s), or set a default with `cairn init --default-repo`", strings.Join(p.Repos, ", "))
	} else if !contains(p.Repos, n.Repo) {
		c.addf("repo", "%s is not an allowed repo (allowed: %s)", quote(n.Repo), strings.Join(p.Repos, ", "))
	}
	if n.Task != "" {
		if err := Text("task", n.Task, MaxLine, false); err != nil {
			c.add(err)
		} else if _, _, ok := TaskParts(p, n.Task); !ok {
			c.addf("task", "%s does not match the project task pattern %s", quote(n.Task), p.TaskPattern)
		}
	}
	if len(n.Impacts) > MaxListItems {
		c.addf("impacts", "too many items (max %d)", MaxListItems)
	}
	for _, r := range n.Impacts {
		if !contains(p.Repos, r) {
			c.addf("impacts", "%s is not an allowed repo", quote(r))
		}
	}
	if len(n.Commits) > MaxListItems {
		c.addf("commits", "too many items (max %d)", MaxListItems)
	}
	for _, h := range n.Commits {
		if !commitRe.MatchString(h) {
			c.addf("commits", "entry is not a lowercase hex commit hash (7-64 characters)")
		}
	}
	c.idList("resolves", 'e', n.Resolves)
	c.idList("supersedes", 'e', n.Supersedes)
	c.entries(n.Entries)
	if len(n.Entries) == 0 {
		c.addf("entries", "a note needs at least one entry")
	}
	c.sections(n)
	return c
}

// sections enforces that every entry type has an entry or is declared in none.
func (c *collector) sections(n model.Note) {
	have := map[model.EntryType]bool{}
	for _, e := range n.Entries {
		have[e.Type] = true
	}
	none := map[model.EntryType]bool{}
	for _, t := range n.None {
		if !validType(t) {
			c.addf("none", "%s is not an entry type", quote(string(t)))
			continue
		}
		none[t] = true
		if have[t] {
			c.addf("none", "%s is declared empty but has entries", t)
		}
	}
	for _, t := range model.EntryTypes {
		if !have[t] && !none[t] {
			c.addf("none", `section %s has no entry: add one or list it in none`, t)
		}
	}
}

// Addendum validates an addendum and returns every problem found.
func Addendum(a model.Addendum, o Options) []FieldError {
	var c collector
	if a.ID != "" && !ids.Valid('a', a.ID) {
		c.addf("id", "%s is not a valid addendum ID", quote(a.ID))
	}
	if !ids.Valid('n', a.NoteID) {
		c.addf("note_id", "%s is not a valid note ID", quote(a.NoteID))
	}
	c.actor(a.Agent, a.Model, o)
	c.timestamp(a.At)
	if a.Status != "" && !validStatus(a.Status) {
		c.addf("status", "must be done, in_progress or blocked")
	}
	c.idList("resolves", 'e', a.Resolves)
	c.idList("supersedes", 'e', a.Supersedes)
	c.entries(a.Entries)
	if len(a.Entries) == 0 && a.Status == "" && len(a.Resolves) == 0 && len(a.Supersedes) == 0 {
		c.addf("addendum", "is empty: give entries, a status, resolves or supersedes")
	}
	return c
}

// Project validates project configuration (name, mode, context, repos, task pattern).
func Project(p model.Project) []FieldError {
	var c collector
	if !nameRe.MatchString(p.Name) {
		c.addf("name", "must match %s", nameRe)
	}
	if p.Mode != model.ModeProject && p.Mode != model.ModeMachine {
		c.addf("mode", "must be project or machine")
	}
	c.add(Text("context", p.Context, MaxContext, true))
	if len(p.Repos) == 0 {
		c.addf("repos", "at least one repo is required")
	}
	if len(p.Repos) > MaxRepos {
		c.addf("repos", "too many repos (max %d)", MaxRepos)
	}
	for _, r := range p.Repos {
		if !repoRe.MatchString(r) {
			c.addf("repos", "%s must match %s", quote(r), repoRe)
		}
	}
	if p.DefaultRepo != "" && !contains(p.Repos, p.DefaultRepo) {
		c.addf("default_repo", "%s is not one of the repos", quote(p.DefaultRepo))
	}
	if len(p.TaskPattern) > MaxPatternLen {
		c.addf("task_pattern", "too long (max %d characters)", MaxPatternLen)
	} else if re, err := regexp.Compile(p.TaskPattern); err != nil {
		c.addf("task_pattern", "is not a valid regular expression: %v", err)
	} else if re.SubexpIndex("base") < 0 {
		c.addf("task_pattern", "needs a named group (?P<base>...)")
	}
	return c
}

// TaskParts splits a task ID using the project's pattern (named groups base, ext).
func TaskParts(p model.Project, id string) (base, ext string, ok bool) {
	if len(p.TaskPattern) > MaxPatternLen {
		return "", "", false
	}
	re, err := regexp.Compile(p.TaskPattern)
	if err != nil || re.SubexpIndex("base") < 0 {
		return "", "", false
	}
	loc := re.FindStringSubmatchIndex(id)
	if loc == nil || loc[0] != 0 || loc[1] != len(id) {
		return "", "", false // the pattern must cover the whole ID, anchored or not
	}
	m := re.FindStringSubmatch(id)
	base = m[re.SubexpIndex("base")]
	if base == "" {
		return "", "", false
	}
	if i := re.SubexpIndex("ext"); i >= 0 {
		ext = m[i]
	}
	return base, ext, true
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ValidName reports whether s is an acceptable project name. Names become folder
// names in the data directory, so path separators and dots are impossible.
func ValidName(s string) bool { return nameRe.MatchString(s) }

// quote shows a user-supplied value in an error message, shortened so that a
// secret pasted into the wrong field is not repeated in full.
func quote(v string) string {
	r := []rune(v)
	if len(r) > 24 {
		return strconv.Quote(string(r[:24])) + "…"
	}
	return strconv.Quote(v)
}

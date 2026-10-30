package view

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/R0b-1ns/cairn/internal/ids"
	"github.com/R0b-1ns/cairn/internal/model"
)

//go:embed templates assets
var embedded embed.FS

var funcs = template.FuncMap{
	"date": dateOf,
	"dash": orDash,
	"join": strings.Join,
}

// pages are parsed once: each page is the layout plus its own content template.
var pages = func() map[string]*template.Template {
	base := template.Must(template.New("layout.html").Funcs(funcs).ParseFS(embedded, "templates/layout.html"))
	out := map[string]*template.Template{}
	for _, name := range []string{"notes.html", "entries.html", "note.html"} {
		out[name] = template.Must(template.Must(base.Clone()).ParseFS(embedded, "templates/"+name))
	}
	return out
}()

// option is one value offered by a select filter.
type option struct{ Value, Label string }

// filters are the values offered by the sidebar selects.
type filters struct {
	Statuses, Tasks, Repos, Agents []option
}

// page is what the layout needs, plus the content of one page.
type page struct {
	Title     string
	Project   string
	Generated string
	Root      string // "" at the top, "../" on note pages
	Active    string // notes, decisions, open, helpers, or note
	Listing   bool   // shows filters, count and download, and loads data.js
	Notes     bool   // the Notes view (status filter)
	Entries   bool   // an entry view (Show closed)
	Filters   filters
	Content   any
}

type notesContent struct{ Rows []noteRow }

type noteRow struct {
	ID, Task, Title, Repo, Agent, At, Text string
	Status                                 model.Status
	OpenPoints                             int
}

type entriesContent struct {
	Noun      string // "decisions", "open points", "helpers"
	Author    bool   // decisions only
	Rows      []entryRow
	Download  string // file name of the view download
	AllClosed bool   // there are rows and every one is closed
}

type entryRow struct {
	ID, Task, Repo, At, Text, By, Agent, NoteID, State, Search string
	Closed                                                     bool
}

type noteContent struct {
	ID, Agent, Model, Repo, Task, At, Title string
	Impacts, Commits                        []string
	Status, Written                         model.Status
	ShowWritten                             bool
	Sections                                []section
	Addenda                                 []addendumView
}

type section struct {
	Title   string
	None    bool
	Entries []entryView
}

type entryView struct {
	ID, Text, By, Label string
	Type                model.EntryType
	Closed              bool
	CloserVerb, Closer  string // closer is the link to the note or addendum that closed it
	CloserID            string
}

type addendumView struct {
	ID, Agent, Model, At string
	Status               model.Status
	Entries              []entryView
	Resolves, Supersedes []string
}

// dataEntry is the shape of an entry in data.js. Never marshal EntryRow or
// store.EntryInfo directly: their promoted MarshalJSON drops the agent.
type dataEntry struct {
	Text  string `json:"text"`
	By    string `json:"by"`
	Note  string `json:"note"`
	Agent string `json:"agent"`
}

type dataFile struct {
	Header  string               `json:"header"`
	Notes   map[string]string    `json:"notes"`
	Entries map[string]dataEntry `json:"entries"`
}

var sectionTitles = map[model.EntryType]string{
	model.TypeChange: "Changes", model.TypeDecision: "Decisions", model.TypeOpen: "Open points",
	model.TypeNote: "Notes", model.TypeHelper: "Helpers",
}

var typeLabels = map[model.EntryType]string{
	model.TypeChange: "change", model.TypeDecision: "decision", model.TypeOpen: "open point",
	model.TypeNote: "note", model.TypeHelper: "helper",
}

// Write renders the site into dir. dir must not exist (its parent must) or be a
// real directory. Files are owner-only and never written through a symlink.
func Write(dir string, s *Site) error {
	for _, nv := range s.Notes {
		if !ids.Valid('n', nv.Header.ID) {
			return fmt.Errorf("invalid note ID %q", nv.Header.ID)
		}
	}
	if err := ensureDir(dir); err != nil {
		return err
	}
	for _, sub := range []string{"notes", "assets"} {
		if err := ensureDir(filepath.Join(dir, sub)); err != nil {
			return err
		}
	}
	files, err := render(s)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := writeFile(filepath.Join(dir, filepath.FromSlash(name)), files[name]); err != nil {
			return err
		}
	}
	return nil
}

// render builds every file of the site in memory, keyed by slash path.
func render(s *Site) (map[string][]byte, error) {
	files := map[string][]byte{".gitignore": []byte("*\n")}
	for _, a := range []string{"app.css", "app.js"} {
		b, err := embedded.ReadFile("assets/" + a)
		if err != nil {
			return nil, err
		}
		files["assets/"+a] = b
	}
	f := buildFilters(s)
	base := func(title, active string) page {
		return page{Title: title, Project: s.Project.Name, Generated: s.Generated, Active: active, Filters: f}
	}

	p := base("Notes", "notes")
	p.Listing, p.Notes = true, true
	rows := []noteRow{}
	for _, nv := range s.Notes {
		rows = append(rows, noteRow{
			ID: nv.Header.ID, Task: nv.Header.Task, Title: nv.Header.Title, Repo: nv.Header.Repo,
			Agent: nv.Header.Agent, At: nv.Header.At, Status: nv.Header.Status, OpenPoints: nv.OpenPoints,
			Text: noteSearchText(nv),
		})
	}
	p.Content = notesContent{Rows: rows}
	if err := renderPage(files, "index.html", "notes.html", p); err != nil {
		return nil, err
	}

	views := []struct {
		file, title, active, noun string
		typ                       model.EntryType
	}{
		{"decisions.html", "Decisions", "decisions", "decisions", model.TypeDecision},
		{"open-points.html", "Open points", "open", "open points", model.TypeOpen},
		{"helpers.html", "Helpers", "helpers", "helpers", model.TypeHelper},
	}
	for _, v := range views {
		p := base(v.title, v.active)
		p.Listing, p.Entries = true, true
		c := entriesContent{Noun: v.noun, Author: v.typ == model.TypeDecision, Rows: []entryRow{},
			Download: strings.TrimSuffix(v.file, ".html") + ".md"}
		for _, e := range s.EntriesOf(v.typ) {
			c.Rows = append(c.Rows, entryRow{
				ID: e.ID, Task: e.Task, Repo: e.Repo, At: e.At, Text: e.Text, By: e.By, Agent: e.Agent,
				NoteID: e.NoteID, Closed: e.Closed, State: stateLabel(e.Type, e.Closed),
				Search: strings.ToLower(strings.Join([]string{e.ID, e.Text, e.Task, e.Repo, e.Agent, e.NoteID}, " ")),
			})
		}
		c.AllClosed = len(c.Rows) > 0
		for _, r := range c.Rows {
			if !r.Closed {
				c.AllClosed = false
			}
		}
		p.Content = c
		if err := renderPage(files, v.file, "entries.html", p); err != nil {
			return nil, err
		}
	}

	owner := map[string]string{} // addendum ID -> note ID
	for _, nv := range s.Notes {
		for _, a := range nv.Addenda {
			owner[a.ID] = nv.Header.ID
		}
	}
	for _, nv := range s.Notes {
		p := base(nv.Header.Title, "note")
		p.Root = "../"
		p.Content = buildNote(nv, owner)
		id := nv.Header.ID
		if err := renderPage(files, "notes/"+id+".html", "note.html", p); err != nil {
			return nil, err
		}
		files["notes/"+id+".md"] = []byte(s.MarkdownHeader + nv.Markdown)
	}

	data, err := buildData(s)
	if err != nil {
		return nil, err
	}
	files["data.js"] = data
	return files, nil
}

func renderPage(files map[string][]byte, out, tmpl string, p page) error {
	var b bytes.Buffer
	if err := pages[tmpl].ExecuteTemplate(&b, "layout.html", p); err != nil {
		return fmt.Errorf("render %s: %w", out, err)
	}
	files[out] = b.Bytes()
	return nil
}

func buildData(s *Site) ([]byte, error) {
	d := dataFile{Header: s.MarkdownHeader, Notes: map[string]string{}, Entries: map[string]dataEntry{}}
	for _, nv := range s.Notes {
		d.Notes[nv.Header.ID] = nv.Markdown
	}
	for _, e := range s.Entries {
		d.Entries[e.ID] = dataEntry{Text: e.Text, By: e.By, Note: e.NoteID, Agent: e.Agent}
	}
	// json.Marshal escapes <, > and &, so no "</script" can appear.
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return []byte("window.CAIRN = " + string(b) + ";\n"), nil
}

func buildFilters(s *Site) filters {
	tasks, repos, agents := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, nv := range s.Notes {
		tasks[nv.Header.Task] = true
		repos[nv.Header.Repo] = true
		agents[nv.Header.Agent] = true
	}
	for _, e := range s.Entries {
		tasks[e.Task] = true
		repos[e.Repo] = true
		agents[e.Agent] = true
	}
	opts := func(m map[string]bool) []option {
		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := []option{}
		for _, k := range keys {
			out = append(out, option{Value: k, Label: orDash(k)})
		}
		return out
	}
	return filters{
		Statuses: []option{
			{string(model.StatusDone), "done"}, {string(model.StatusInProgress), "in_progress"},
			{string(model.StatusBlocked), "blocked"},
		},
		Tasks: opts(tasks), Repos: opts(repos), Agents: opts(agents),
	}
}

func noteSearchText(nv NoteView) string {
	parts := []string{nv.Header.ID, nv.Header.Title, nv.Header.Task, nv.Header.Repo, nv.Header.Agent}
	for _, e := range nv.Note.Entries {
		parts = append(parts, e.Text)
	}
	for _, a := range nv.Addenda {
		parts = append(parts, a.Agent)
		for _, e := range a.Entries {
			parts = append(parts, e.Text)
		}
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func buildNote(nv NoteView, owner map[string]string) noteContent {
	n := nv.Note
	c := noteContent{
		ID: nv.Header.ID, Agent: n.Agent, Model: n.Model, Repo: n.Repo, Task: n.Task, At: n.At, Title: n.Title,
		Impacts: n.Impacts, Commits: n.Commits, Status: nv.Header.Status, Written: nv.Written,
		ShowWritten: nv.Written != "" && nv.Written != nv.Header.Status,
	}
	none := map[model.EntryType]bool{}
	for _, t := range n.None {
		none[t] = true
	}
	view := func(e model.Entry) entryView {
		ev := entryView{ID: e.ID, Text: e.Text, By: e.By, Type: e.Type, Label: typeLabels[e.Type]}
		if st, ok := nv.States[e.ID]; ok && st.Closed {
			ev.Closed = true
			ev.CloserVerb = closerVerb(e.Type)
			ev.CloserID = st.ClosedBy
			ev.Closer = closerHref(st.ClosedBy, owner)
		}
		return ev
	}
	for _, t := range model.EntryTypes {
		sec := section{Title: sectionTitles[t]}
		for _, e := range n.Entries {
			if e.Type == t {
				sec.Entries = append(sec.Entries, view(e))
			}
		}
		sec.None = len(sec.Entries) == 0 || none[t]
		c.Sections = append(c.Sections, sec)
	}
	for _, a := range nv.Addenda {
		av := addendumView{ID: a.ID, Agent: a.Agent, Model: a.Model, At: a.At, Status: a.Status,
			Resolves: a.Resolves, Supersedes: a.Supersedes}
		for _, e := range a.Entries {
			av.Entries = append(av.Entries, view(e))
		}
		c.Addenda = append(c.Addenda, av)
	}
	return c
}

// closerHref links, from a note page, to the note or addendum that closed an entry.
func closerHref(id string, owner map[string]string) string {
	switch ids.Kind(id) {
	case 'n':
		return id + ".html"
	case 'a':
		if n, ok := owner[id]; ok {
			return n + ".html#" + id
		}
	}
	return ""
}

func closerVerb(t model.EntryType) string {
	if t == model.TypeDecision {
		return "superseded by"
	}
	return "resolved by"
}

func stateLabel(t model.EntryType, closed bool) string {
	switch {
	case closed && t == model.TypeDecision:
		return "superseded"
	case closed:
		return "resolved"
	case t == model.TypeOpen:
		return "open"
	default:
		return "active"
	}
}

func dateOf(at string) string {
	if len(at) >= 10 {
		return at[:10]
	}
	return at
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// ensureDir creates dir (0700, parent must exist) or checks it is a real directory.
func ensureDir(dir string) error {
	fi, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.Mkdir(dir, 0o700); err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		return fmt.Errorf("%s is not a directory (symlinks are refused)", dir)
	}
	return nil
}

// writeFile writes an owner-only file, never through a symlink, and replaces
// only an existing regular file.
func writeFile(path string, body []byte) error {
	flags := os.O_WRONLY | os.O_CREATE | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
	if fi, err := os.Lstat(path); err == nil {
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file; refusing to replace it", path)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	fh, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	defer fh.Close()
	opened, err := fh.Stat()
	if err != nil || !opened.Mode().IsRegular() {
		return fmt.Errorf("%s changed while opening; refusing to write", path)
	}
	if err := fh.Truncate(0); err != nil {
		return err
	}
	if err := fh.Chmod(0o600); err != nil {
		return err
	}
	if _, err := io.Copy(fh, bytes.NewReader(body)); err != nil {
		return err
	}
	return fh.Close()
}

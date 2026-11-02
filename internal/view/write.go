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
	"github.com/R0b-1ns/cairn/internal/tasks"
	"github.com/R0b-1ns/cairn/internal/validate"
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

// option is one value offered by a select filter. A base task option (Base)
// selects the base and its extensions; any other option selects exactly its value.
type option struct {
	Value, Label string
	Base         bool
}

// filters are the values offered by the sidebar selects.
type filters struct {
	Statuses, Tasks, Repos, Agents, Branches, Authors []option
	Children                                          []option // only when a note comes from a child
}

// page is what the layout needs, plus the content of one page.
type page struct {
	Title     string
	Project   string
	Generated string
	Root      string // "" at the top, "../" on note pages
	Active    string // notes, decisions, open, helpers, or note
	Children  bool   // some note comes from a local child: show the Child column and filter
	Listing   bool   // shows filters, count and download, and loads data.js
	Notes     bool   // the Notes view (status filter)
	Entries   bool   // an entry view (Show closed)
	Filters   filters
	Content   any
}

type notesContent struct{ Rows []noteRow }

type noteRow struct {
	ID, Task, TaskBase, Title, Repo, Child, Agent, At, Text string
	Status                                                  model.Status
	OpenPoints                                              int
	// Branch and Author are the note's own; More* are the other values of its addenda.
	// Branches and Authors list everything the filters match, one value per line.
	Branch, Author, Branches, Authors string
	MoreBranches, MoreAuthors         []string
}

type entriesContent struct {
	Noun      string // "decisions", "open points", "helpers"
	Author    bool   // decisions only
	Rows      []entryRow
	Download  string // file name of the view download
	AllClosed bool   // there are rows and every one is closed
}

type entryRow struct {
	ID, Task, TaskBase, Repo, Child, At, Text, By, Agent, NoteID, State, Search string
	Branches, Authors                                                           string // of the holder, for the filters
	Closed                                                                      bool
}

type noteContent struct {
	ID, Agent, Model, Repo, Task, At, Title string
	Author, Branch, Child                   string
	Refs                                    []refView
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

// refView is a ref shown on a card; only an http or https ref is a link.
type refView struct {
	URL  string
	Link bool
}

type addendumView struct {
	ID, Agent, Model, At string
	Author, Branch       string
	Refs                 []refView
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
	Child string `json:"child,omitempty"`
	// Closed is "superseded by <id>" or "resolved by <id>" for a closed entry, else empty,
	// so a downloaded view never shows a closed entry as current.
	Closed string `json:"closed,omitempty"`
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
	base := func(title, active string) page {
		return page{Title: title, Project: s.Project.Name, Generated: s.Generated, Active: active, Children: s.hasChildren()}
	}

	p := base("Notes", "notes")
	p.Listing, p.Notes = true, true
	rows := []noteRow{}
	var nf filterSet
	for _, nv := range s.Notes {
		nf.add(nv.Header.Task, nv.Header.Repo, nv.Header.Agent)
		nf.addChild(nv.Child)
		own := origin{nv.Note.Branch, nv.Note.AuthorName, nv.Note.AuthorEmail}
		all := []origin{own}
		for _, a := range nv.Addenda {
			all = append(all, origin{a.Branch, a.AuthorName, a.AuthorEmail})
		}
		nf.addOrigins(all...)
		branches, authors := matchValues(all...)
		moreB, moreA := others(own.branch, branchesOf(all[1:])), others(own.display(), displaysOf(all[1:]))
		rows = append(rows, noteRow{
			Branch: own.branch, Author: own.display(), Branches: branches, Authors: authors,
			MoreBranches: moreB, MoreAuthors: moreA,
			ID: nv.Header.ID, Task: nv.Header.Task, TaskBase: taskBase(s.Project, nv.Header.Task),
			Title: nv.Header.Title, Repo: nv.Header.Repo, Child: nv.Child,
			Agent: nv.Header.Agent, At: nv.Header.At, Status: nv.Header.Status, OpenPoints: nv.OpenPoints,
			Text: noteSearchText(nv),
		})
	}
	p.Content = notesContent{Rows: rows}
	p.Filters = nf.build(s.Project, true)
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
		var ef filterSet
		for _, e := range s.EntriesOf(v.typ) {
			ef.add(e.Task, e.Repo, e.Agent)
			ef.addChild(e.Child)
			holder := origin{e.Branch, e.AuthorName, e.AuthorEmail}
			ef.addOrigins(holder)
			branches, authors := matchValues(holder)
			c.Rows = append(c.Rows, entryRow{
				ID: e.ID, Task: e.Task, TaskBase: taskBase(s.Project, e.Task), Repo: e.Repo, Child: e.Child, At: e.At, Text: e.Text, By: e.By, Agent: e.Agent,
				NoteID: e.NoteID, Closed: e.Closed, State: stateLabel(e.Type, e.Closed),
				Branches: branches, Authors: authors,
				Search: strings.ToLower(strings.Join([]string{e.ID, e.Text, e.Task, e.Repo, e.Child, e.Agent, e.NoteID,
					e.Branch, e.AuthorName, e.AuthorEmail}, " ")),
			})
		}
		c.AllClosed = len(c.Rows) > 0
		for _, r := range c.Rows {
			if !r.Closed {
				c.AllClosed = false
			}
		}
		p.Content = c
		p.Filters = ef.build(s.Project, false)
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
		p.Content = buildNote(nv, owner, s.Project)
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
		de := dataEntry{Text: e.Text, By: e.By, Note: e.NoteID, Agent: e.Agent, Child: e.Child}
		if e.Closed {
			de.Closed = closerVerb(e.Type) + " " + e.ClosedBy
		}
		d.Entries[e.ID] = de
	}
	// json.Marshal escapes <, > and &, so no "</script" can appear.
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return []byte("window.CAIRN = " + string(b) + ";\n"), nil
}

// origin is who wrote a note or addendum, and on which branch.
type origin struct{ branch, name, email string }

// display is the name, or the email when there is no name.
func (o origin) display() string {
	if o.name != "" {
		return o.name
	}
	return o.email
}

// matchValues are the values a branch and an author filter match against, one per line:
// the branches, and the names and emails (as `cairn list --author` matches them).
func matchValues(list ...origin) (branches, authors string) {
	var b, a []string
	for _, o := range list {
		b = append(b, o.branch)
		a = append(a, o.name, o.email)
	}
	return strings.Join(distinct(b), "\n"), strings.Join(distinct(a), "\n")
}

func branchesOf(list []origin) []string {
	var out []string
	for _, o := range list {
		out = append(out, o.branch)
	}
	return out
}

func displaysOf(list []origin) []string {
	var out []string
	for _, o := range list {
		out = append(out, o.display())
	}
	return out
}

// distinct drops empty values and repeats, keeping the first-seen order.
func distinct(l []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range l {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// others are the distinct values of l that differ from own.
func others(own string, l []string) []string {
	out := []string{}
	for _, v := range distinct(l) {
		if v != own {
			out = append(out, v)
		}
	}
	return out
}

// filterSet gathers the values of the rows of one view.
type filterSet struct{ tasks, repos, agents, branches, authors, children map[string]bool }

// addChild records the child a row comes from; the current project (empty) is not offered.
func (f *filterSet) addChild(child string) {
	if child == "" {
		return
	}
	if f.children == nil {
		f.children = map[string]bool{}
	}
	f.children[child] = true
}

// addOrigins records the branch and author of a row; empty values are not offered.
func (f *filterSet) addOrigins(list ...origin) {
	for _, o := range list {
		if o.branch != "" {
			f.branches[o.branch] = true
		}
		if d := o.display(); d != "" {
			f.authors[d] = true
		}
	}
}

func (f *filterSet) add(task, repo, agent string) {
	if f.tasks == nil {
		f.tasks, f.repos, f.agents = map[string]bool{}, map[string]bool{}, map[string]bool{}
		f.branches, f.authors = map[string]bool{}, map[string]bool{}
	}
	f.tasks[task], f.repos[repo], f.agents[agent] = true, true, true
}

// build turns the values into select options; status is offered on the Notes view only.
func (f *filterSet) build(p model.Project, status bool) filters {
	out := filters{Tasks: taskOptions(p, f.tasks), Repos: plainOptions(f.repos), Agents: plainOptions(f.agents),
		Branches: plainOptions(f.branches), Authors: plainOptions(f.authors)}
	if len(f.children) > 0 {
		out.Children = plainOptions(f.children)
	}
	if status {
		out.Statuses = []option{
			{Value: string(model.StatusDone), Label: "done"}, {Value: string(model.StatusInProgress), Label: "in_progress"},
			{Value: string(model.StatusBlocked), Label: "blocked"},
		}
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func plainOptions(m map[string]bool) []option {
	out := []option{}
	for _, k := range sortedKeys(m) {
		out = append(out, option{Value: k, Label: orDash(k)})
	}
	return out
}

// taskOptions offers, as the CLI does, each base task (selecting it and its
// extensions, and labelled so when the view has some) and each extension
// (selecting exactly itself). A base is offered even when only its extensions
// have rows. "-" (no task) is exact.
func taskOptions(p model.Project, tasks map[string]bool) []option {
	opts := map[string]option{}
	extended := map[string]bool{} // base -> the view has one of its extensions
	for task := range tasks {
		if task == "" {
			opts[task] = option{Value: "", Label: "-"}
			continue
		}
		b := taskBase(p, task)
		if b != task {
			extended[b] = true
			if _, ok := opts[task]; !ok {
				opts[task] = option{Value: task, Label: task}
			}
		}
		opts[b] = option{Value: b, Label: b, Base: true}
	}
	for b := range extended {
		opts[b] = option{Value: b, Label: b + " (incl. extensions)", Base: true}
	}
	keys := make([]string, 0, len(opts))
	for k := range opts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []option{}
	for _, k := range keys {
		out = append(out, opts[k])
	}
	return out
}

// taskBase is the base of a task under the project's pattern (as `cairn list
// --task` uses it), or the task itself when it does not parse.
func taskBase(p model.Project, task string) string {
	if task == "" {
		return ""
	}
	if b, _, ok := validate.TaskParts(p, task); ok {
		return b
	}
	return task
}

func noteSearchText(nv NoteView) string {
	n := nv.Note
	parts := []string{nv.Header.ID, nv.Header.Title, nv.Header.Task, nv.Header.Repo, nv.Child, nv.Header.Agent,
		n.Branch, n.AuthorName, n.AuthorEmail, n.Ref}
	parts = append(parts, nv.TaskRefs...)
	for _, e := range nv.Note.Entries {
		parts = append(parts, e.Text)
	}
	for _, a := range nv.Addenda {
		parts = append(parts, a.Agent, a.Branch, a.AuthorName, a.AuthorEmail, a.Ref)
		for _, e := range a.Entries {
			parts = append(parts, e.Text)
		}
	}
	return strings.ToLower(strings.Join(parts, " "))
}

// refViews turns refs into card values, dropping empty ones and repeats.
func refViews(refs ...string) []refView {
	var out []refView
	for _, r := range distinct(refs) {
		l := strings.ToLower(r)
		out = append(out, refView{URL: r, Link: strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://")})
	}
	return out
}

// authorLine is "Name <email>", or whichever of the two is set.
func authorLine(name, email string) string {
	switch {
	case name != "" && email != "":
		return name + " <" + email + ">"
	case name != "":
		return name
	default:
		return email
	}
}

func buildNote(nv NoteView, owner map[string]string, proj model.Project) noteContent {
	n := nv.Note
	c := noteContent{
		ID: nv.Header.ID, Agent: n.Agent, Model: n.Model, Repo: n.Repo, Task: n.Task, At: n.At, Title: n.Title,
		Impacts: n.Impacts, Commits: n.Commits, Status: nv.Header.Status, Written: nv.Written,
		ShowWritten: nv.Written != "" && nv.Written != nv.Header.Status,
		Author:      authorLine(n.AuthorName, n.AuthorEmail), Branch: n.Branch, Child: nv.Child,
	}
	refs := append([]string{n.Ref}, nv.TaskRefs...)
	if len(distinct(refs)) == 0 && n.Task != "" {
		// no ref stored for the task: offer the one the project's URL template builds
		refs = append(refs, tasks.URL(proj.TaskURL, taskBase(proj, n.Task)))
	}
	c.Refs = refViews(refs...)
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
			Author: authorLine(a.AuthorName, a.AuthorEmail), Branch: a.Branch, Refs: refViews(a.Ref),
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

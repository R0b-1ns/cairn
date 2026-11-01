// Package view collects a project's notes for the static website of `cairn view`.
// It only reads through the store's existing read APIs.
package view

import (
	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/store"
)

// EntryRow is an entry with where it lives, its state and its author.
type EntryRow struct {
	store.EntryInfo
	Agent string // agent that wrote the entry (the addendum's agent for addendum entries)
	// Branch and the author are those of the note or addendum that holds the entry.
	Branch, AuthorName, AuthorEmail string
}

// NoteView is one note with everything the site shows about it.
type NoteView struct {
	Header     store.NoteHeader // Status is the current status
	Written    model.Status     // the note's own status
	Note       model.Note
	Addenda    []model.Addendum
	OpenPoints int                        // open entries of the note and its addenda not yet resolved
	States     map[string]store.EntryInfo // entry ID -> state, for every entry of the note and its addenda
	Markdown   string                     // filled by the caller (cli), notes part only
	// TaskRefs are the refs stored anywhere for the note's task (base), newest first.
	// The note's own branch, author and ref are in Note and Header; the addenda's in Addenda.
	TaskRefs []string
}

// Site is the data behind the whole website.
type Site struct {
	Project        model.Project
	Generated      string     // RFC3339 UTC, given by the caller
	MarkdownHeader string     // filled by the caller
	Notes          []NoteView // newest first
	Entries        []EntryRow // every entry, all states, newest first
}

// Collect reads every note, addendum and entry of the project.
func Collect(st *store.Store, proj model.Project, generated string) (*Site, error) {
	site := &Site{Project: proj, Generated: generated, Notes: []NoteView{}, Entries: []EntryRow{}}
	headers, err := st.NoteHeaders(store.NoteFilter{})
	if err != nil {
		return nil, err
	}
	infos, _, err := st.ListEntries(store.Filter{State: store.StateAll})
	if err != nil {
		return nil, err
	}
	byNote := map[string]map[string]store.EntryInfo{}
	for _, e := range infos {
		if byNote[e.NoteID] == nil {
			byNote[e.NoteID] = map[string]store.EntryInfo{}
		}
		byNote[e.NoteID][e.ID] = e
	}
	agents := map[string]string{} // addendum ID -> agent
	type origin struct{ branch, name, email string }
	holders := map[string]origin{}    // note or addendum ID -> who wrote it, on which branch
	taskRefs := map[string][]string{} // task base -> refs, read once per base
	// headers are oldest first; build newest first.
	for i := len(headers) - 1; i >= 0; i-- {
		h := headers[i]
		n, err := st.GetNote(h.ID)
		if err != nil {
			return nil, err
		}
		add, err := st.Addenda(h.ID)
		if err != nil {
			return nil, err
		}
		holders[h.ID] = origin{n.Branch, n.AuthorName, n.AuthorEmail}
		for _, a := range add {
			agents[a.ID] = a.Agent
			holders[a.ID] = origin{a.Branch, a.AuthorName, a.AuthorEmail}
		}
		var refs []string
		if base := taskBase(proj, n.Task); base != "" {
			var ok bool
			if refs, ok = taskRefs[base]; !ok {
				if refs, err = st.TaskRefs(base); err != nil {
					return nil, err
				}
				taskRefs[base] = refs
			}
		}
		states := byNote[h.ID]
		if states == nil {
			states = map[string]store.EntryInfo{}
		}
		open := 0
		for _, e := range states {
			if e.Type == model.TypeOpen && !e.Closed {
				open++
			}
		}
		site.Notes = append(site.Notes, NoteView{
			Header: h, Written: n.Status, Note: n, Addenda: add, OpenPoints: open, States: states, TaskRefs: refs,
		})
	}
	noteAgent := map[string]string{}
	for _, h := range headers {
		noteAgent[h.ID] = h.Agent
	}
	for _, e := range infos {
		agent, holder := noteAgent[e.NoteID], e.NoteID
		if e.AddendumID != "" {
			agent, holder = agents[e.AddendumID], e.AddendumID
		}
		o := holders[holder]
		site.Entries = append(site.Entries, EntryRow{EntryInfo: e, Agent: agent,
			Branch: o.branch, AuthorName: o.name, AuthorEmail: o.email})
	}
	return site, nil
}

// EntriesOf returns the entries of one type, keeping the newest-first order.
func (s *Site) EntriesOf(t model.EntryType) []EntryRow {
	out := []EntryRow{}
	for _, e := range s.Entries {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

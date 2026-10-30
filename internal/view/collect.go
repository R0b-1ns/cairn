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
		for _, a := range add {
			agents[a.ID] = a.Agent
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
			Header: h, Written: n.Status, Note: n, Addenda: add, OpenPoints: open, States: states,
		})
	}
	noteAgent := map[string]string{}
	for _, h := range headers {
		noteAgent[h.ID] = h.Agent
	}
	for _, e := range infos {
		agent := noteAgent[e.NoteID]
		if e.AddendumID != "" {
			agent = agents[e.AddendumID]
		}
		site.Entries = append(site.Entries, EntryRow{EntryInfo: e, Agent: agent})
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

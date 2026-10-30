// Package model holds the plain data types shared by every cairn package.
package model

// Status is the progress state of a note.
type Status string

// Note and addendum statuses.
const (
	StatusDone       Status = "done"
	StatusInProgress Status = "in_progress"
	StatusBlocked    Status = "blocked"
)

// EntryType is the kind of a note entry.
type EntryType string

// Entry types.
const (
	TypeChange   EntryType = "change"
	TypeDecision EntryType = "decision"
	TypeOpen     EntryType = "open"
	TypeNote     EntryType = "note"
	TypeHelper   EntryType = "helper"
)

// EntryTypes lists every entry type in display order.
var EntryTypes = []EntryType{TypeChange, TypeDecision, TypeOpen, TypeNote, TypeHelper}

// Entry is one typed piece of content. By ("user" or "agent") is set for decisions only.
type Entry struct {
	ID   string    `json:"id,omitempty"`
	Type EntryType `json:"type"`
	Text string    `json:"text"`
	By   string    `json:"by,omitempty"`
}

// Note is an immutable work note.
type Note struct {
	ID         string      `json:"id,omitempty"`
	Agent      string      `json:"agent"`
	Repo       string      `json:"repo"`
	At         string      `json:"at,omitempty"` // RFC3339, UTC
	Task       string      `json:"task,omitempty"`
	Model      string      `json:"model"`
	Title      string      `json:"title"`
	Status     Status      `json:"status"`
	Entries    []Entry     `json:"entries"`
	None       []EntryType `json:"none,omitempty"` // sections explicitly declared empty
	Impacts    []string    `json:"impacts,omitempty"`
	Commits    []string    `json:"commits,omitempty"`
	Resolves   []string    `json:"resolves,omitempty"`   // IDs of open entries closed by this note
	Supersedes []string    `json:"supersedes,omitempty"` // IDs of decision entries replaced by this note
}

// Addendum is an immutable addition to a note.
type Addendum struct {
	ID         string   `json:"id,omitempty"`
	NoteID     string   `json:"note_id,omitempty"`
	Agent      string   `json:"agent"`
	Model      string   `json:"model"`
	At         string   `json:"at,omitempty"`
	Status     Status   `json:"status,omitempty"` // optional new status
	Entries    []Entry  `json:"entries,omitempty"`
	Resolves   []string `json:"resolves,omitempty"`
	Supersedes []string `json:"supersedes,omitempty"`
}

// Project is the configuration stored with the data.
type Project struct {
	Name        string   `json:"name"`
	Mode        string   `json:"mode"` // "project" or "machine"
	Context     string   `json:"context"`
	Repos       []string `json:"repos"`
	DefaultRepo string   `json:"default_repo"` // repo used by add when --repo is omitted; "" = none
	TaskPattern string   `json:"task_pattern"`
}

// Modes of storage.
const (
	ModeProject = "project"
	ModeMachine = "machine"
)

// DefaultTaskPattern is used when init is given no pattern.
const DefaultTaskPattern = `^(?P<base>[A-Z]{2,5}-\d{1,4})(?:\.(?P<ext>\d{1,3}))?$`

// UnknownModel is the model name reserved for imported notes.
const UnknownModel = "unknown"

package store

import (
	"encoding/json"
	"fmt"

	"github.com/R0b-1ns/cairn/internal/ids"
	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/validate"
)

// Link event kinds, as stored in link_events.kind.
const (
	evID            = "id"
	evRepoAdded     = "repo_added"
	evChildLinked   = "child_linked"
	evChildDetached = "child_detached"
	evParentSet     = "parent_set"
	evParentCleared = "parent_cleared"
)

// LinkState is the result of replaying the link_events table.
type LinkState struct {
	ID         string
	ExtraRepos []string
	Children   []model.Child // active children, in link order
	Parent     *model.ParentRef
}

// Links replays every link event in order.
func (s *Store) Links() (LinkState, error) { return replayLinks(s.db) }

func replayLinks(q querier) (LinkState, error) {
	var st LinkState
	rows, err := q.Query(`SELECT kind,name,repo,child_kind,parent_id FROM link_events ORDER BY seq`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, name, repo, ck, pid string
		if err := rows.Scan(&kind, &name, &repo, &ck, &pid); err != nil {
			return st, err
		}
		switch kind {
		case evID:
			if st.ID == "" {
				st.ID = name
			}
		case evRepoAdded:
			if !containsString(st.ExtraRepos, repo) {
				st.ExtraRepos = append(st.ExtraRepos, repo)
			}
		case evChildLinked:
			st.Children = append(st.Children, model.Child{Name: name, Repo: repo, Kind: model.ChildKind(ck)})
		case evChildDetached:
			for i, c := range st.Children {
				if c.Name == name {
					st.Children = append(st.Children[:i:i], st.Children[i+1:]...)
					break
				}
			}
		case evParentSet:
			st.Parent = &model.ParentRef{Name: name, ID: pid}
		case evParentCleared:
			st.Parent = nil
		}
	}
	return st, rows.Err()
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// mergeRepos returns base followed by extra, without duplicates.
func mergeRepos(base, extra []string) []string {
	out := append([]string(nil), base...)
	for _, r := range extra {
		if !containsString(out, r) {
			out = append(out, r)
		}
	}
	return out
}

// addedRepos returns the repos added by repo_added events, in order.
func (s *Store) addedRepos(q querier) ([]string, error) {
	st, err := replayLinks(q)
	return st.ExtraRepos, err
}

// EnsureProjectID returns the project ID, writing one id event the first time.
func (s *Store) EnsureProjectID() (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	st, err := replayLinks(tx)
	if err != nil {
		return "", err
	}
	if st.ID != "" {
		return st.ID, nil
	}
	id := ids.NewProject()
	if _, err := tx.Exec(`INSERT INTO link_events(kind,name) VALUES (?,?)`, evID, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}

// AddRepo adds a repo to the project. It does nothing when the repo is already
// effective, and fails for an invalid name or when the project would exceed
// validate.MaxRepos repos.
func (s *Store) AddRepo(repo string) error {
	if !validate.ValidRepo(repo) {
		return fmt.Errorf("repo %q must be 1-40 characters of a-z, 0-9, '.', '_' or '-', starting with a letter or digit", repo)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw string
	if err := tx.QueryRow(`SELECT repos FROM project WHERE id=1`).Scan(&raw); err != nil {
		return err
	}
	var base []string
	if err := json.Unmarshal([]byte(raw), &base); err != nil {
		return err
	}
	st, err := replayLinks(tx)
	if err != nil {
		return err
	}
	eff := mergeRepos(base, st.ExtraRepos)
	if containsString(eff, repo) {
		return nil
	}
	if len(eff) >= validate.MaxRepos {
		return fmt.Errorf("too many repos (max %d)", validate.MaxRepos)
	}
	if _, err := tx.Exec(`INSERT INTO link_events(kind,repo) VALUES (?,?)`, evRepoAdded, repo); err != nil {
		return err
	}
	return tx.Commit()
}

// LinkChild records a child. It fails when a child of that name is active.
func (s *Store) LinkChild(c model.Child) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	st, err := replayLinks(tx)
	if err != nil {
		return err
	}
	for _, have := range st.Children {
		if have.Name == c.Name {
			return fmt.Errorf("a child named %q is already linked", c.Name)
		}
	}
	if _, err := tx.Exec(`INSERT INTO link_events(kind,name,repo,child_kind) VALUES (?,?,?,?)`,
		evChildLinked, c.Name, c.Repo, string(c.Kind)); err != nil {
		return err
	}
	return tx.Commit()
}

// DetachChild records that an active child is no longer linked.
func (s *Store) DetachChild(name string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	st, err := replayLinks(tx)
	if err != nil {
		return err
	}
	found := false
	for _, c := range st.Children {
		found = found || c.Name == name
	}
	if !found {
		return fmt.Errorf("%w: no linked child named %q", ErrNotFound, name)
	}
	if _, err := tx.Exec(`INSERT INTO link_events(kind,name) VALUES (?,?)`, evChildDetached, name); err != nil {
		return err
	}
	return tx.Commit()
}

// SetParent records the parent of this project.
func (s *Store) SetParent(p model.ParentRef) error {
	_, err := s.db.Exec(`INSERT INTO link_events(kind,name,parent_id) VALUES (?,?,?)`, evParentSet, p.Name, p.ID)
	return err
}

// ClearParent records that this project has no parent.
func (s *Store) ClearParent() error {
	_, err := s.db.Exec(`INSERT INTO link_events(kind) VALUES (?)`, evParentCleared)
	return err
}

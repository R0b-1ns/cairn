package store

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// migration upgrades a database from schema version `from` to from+1.
// Steps must only add things (ALTER TABLE ... ADD COLUMN), never rewrite rows.
type migration struct {
	from int
	sql  string
}

var migrations = []migration{
	{from: 3, sql: `
ALTER TABLE notes ADD COLUMN author_name TEXT NOT NULL DEFAULT '';
ALTER TABLE notes ADD COLUMN author_email TEXT NOT NULL DEFAULT '';
ALTER TABLE notes ADD COLUMN branch TEXT NOT NULL DEFAULT '';
ALTER TABLE notes ADD COLUMN ref TEXT NOT NULL DEFAULT '';
ALTER TABLE addenda ADD COLUMN author_name TEXT NOT NULL DEFAULT '';
ALTER TABLE addenda ADD COLUMN author_email TEXT NOT NULL DEFAULT '';
ALTER TABLE addenda ADD COLUMN branch TEXT NOT NULL DEFAULT '';
ALTER TABLE addenda ADD COLUMN ref TEXT NOT NULL DEFAULT '';
ALTER TABLE project ADD COLUMN task_mode TEXT NOT NULL DEFAULT '';
ALTER TABLE project ADD COLUMN task_prefix TEXT NOT NULL DEFAULT '';
ALTER TABLE project ADD COLUMN task_url TEXT NOT NULL DEFAULT '';
`},
}

// stmtExecer runs statements; satisfied by *sql.DB and *sql.Tx.
type stmtExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// buildSchema creates the schema of the given version: the version 3 baseline,
// the triggers, then every migration below version, so a fresh database equals
// a migrated one.
func buildSchema(db stmtExecer, version int) error {
	if _, err := db.Exec(tableSQLv3); err != nil {
		return err
	}
	for _, stmt := range triggerSQL() {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	for _, m := range migrations {
		if m.from >= version {
			continue
		}
		if _, err := db.Exec(m.sql); err != nil {
			return err
		}
	}
	return nil
}

func migrationFrom(v int) (migration, bool) {
	for _, m := range migrations {
		if m.from == v {
			return m, true
		}
	}
	return migration{}, false
}

// migrate upgrades an older database in place. It does nothing for a current
// database or one whose version has no migration (verify reports those).
//
// Everything runs in one write transaction (BEGIN IMMEDIATE): re-read the
// version, verify the old schema, copy the file to a backup, run the steps and
// set the new version. Holding the write lock across the check means a process
// that loses the race sees the new version and does nothing, instead of
// comparing a freshly migrated schema against the old one.
//
// The copy reads the file through a separate descriptor. On POSIX, closing any
// descriptor of a file drops every lock the process holds on it, SQLite's
// included, so that descriptor stays open until the transaction has ended.
func (s *Store) migrate() (err error) {
	var v int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return nil // verify reports it
	}
	if _, ok := migrationFrom(v); !ok {
		return nil
	}
	tx, err := s.db.Begin() // BEGIN IMMEDIATE
	if err != nil {
		return err
	}
	var src *os.File
	defer func() {
		tx.Rollback() // no-op after a successful Commit
		if src != nil {
			src.Close() // only now: the transaction no longer holds a lock
		}
	}()
	if err := tx.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return err
	}
	m, ok := migrationFrom(v)
	if !ok {
		return nil // another process migrated first
	}
	// Refuse a hostile or corrupt old database before copying or altering it.
	if err := verifyShape(tx, v); err != nil {
		return err
	}
	if src, err = os.Open(s.path); err != nil {
		return fmt.Errorf("backup before migration: %w", err)
	}
	if err := backupFile(src, fmt.Sprintf("%s.v%d.bak", s.path, v)); err != nil {
		return fmt.Errorf("backup before migration: %w", err)
	}
	for ok {
		if _, err := tx.Exec(m.sql); err != nil {
			return fmt.Errorf("migrate from version %d: %w", m.from, err)
		}
		v = m.from + 1
		m, ok = migrationFrom(v)
	}
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", v)); err != nil {
		return err
	}
	return tx.Commit()
}

// backupFile copies the open file in to dst (mode 0600): written to a temporary
// file in the same directory, synced, then renamed over dst. It does not close
// in (see migrate).
func backupFile(in *os.File, dst string) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if err = tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err = io.Copy(tmp, in); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

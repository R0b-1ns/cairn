package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/tasks"
)

// ErrNotAuto is returned when a new task number is requested in a project whose
// task mode is not "auto".
var ErrNotAuto = errors.New("project is not in auto mode")

// TaskError reports a task that cannot be stored: it does not match the task
// pattern, or automatic numbering has run out. It is a validation failure.
type TaskError struct{ Msg string }

func (e *TaskError) Error() string { return e.Msg }

// likeEscape escapes the LIKE wildcards of s (to be used with ESCAPE '\').
func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// nextTask computes the next automatic task ID inside tx. It counts every note
// whose task base carries the project prefix, extensions included. Callers run
// inside a write transaction, so concurrent writers serialize.
func nextTask(tx *sql.Tx, p model.Project) (string, error) {
	if p.TaskMode != tasks.Auto {
		return "", ErrNotAuto
	}
	if p.TaskPrefix == "" {
		return "", errors.New("project in auto mode has no task prefix")
	}
	rows, err := tx.Query(`SELECT DISTINCT task_base FROM notes WHERE task_base LIKE ? ESCAPE '\'`,
		likeEscape(p.TaskPrefix)+"-%")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var bases []string
	for rows.Next() {
		var b string
		if err := rows.Scan(&b); err != nil {
			return "", err
		}
		bases = append(bases, b)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	n := tasks.Next(p.TaskPrefix, bases)
	if n > tasks.MaxNumber {
		return "", &TaskError{fmt.Sprintf("task numbers exhausted for prefix %s (max %d)", p.TaskPrefix, tasks.MaxNumber)}
	}
	return tasks.Format(p.TaskPrefix, n), nil
}

// TaskRefs returns the distinct refs carried by the notes and addenda of a
// task, the most recently written first.
func (s *Store) TaskRefs(taskBase string) ([]string, error) {
	rows, err := s.db.Query(`SELECT ref FROM (
		SELECT ref, at FROM notes WHERE task_base = ? AND ref <> ''
		UNION ALL
		SELECT a.ref, a.at FROM addenda a JOIN notes n ON n.id = a.note_id
		WHERE n.task_base = ? AND a.ref <> '')
		GROUP BY ref ORDER BY MAX(at) DESC, ref`, taskBase, taskBase)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// provenanceWhere returns conditions on notes aliased n for the branch, author
// and ref filters. All values are bound parameters.
func provenanceWhere(branch, author, ref string) (conds []string, args []any) {
	if branch != "" {
		conds = append(conds, `(n.branch = ? OR EXISTS (SELECT 1 FROM addenda x WHERE x.note_id = n.id AND x.branch = ?))`)
		args = append(args, branch, branch)
	}
	if author != "" {
		conds = append(conds, `(n.author_name = ? OR n.author_email = ? OR EXISTS (
			SELECT 1 FROM addenda x WHERE x.note_id = n.id AND (x.author_name = ? OR x.author_email = ?)))`)
		args = append(args, author, author, author, author)
	}
	if ref != "" {
		conds = append(conds, `(n.ref = ? OR EXISTS (SELECT 1 FROM addenda x WHERE x.note_id = n.id AND x.ref = ?)
			OR (n.task_base <> '' AND n.task_base IN (
				SELECT m.task_base FROM notes m WHERE m.ref = ?
				UNION SELECT m.task_base FROM addenda x JOIN notes m ON m.id = x.note_id WHERE x.ref = ?)))`)
		args = append(args, ref, ref, ref, ref)
	}
	return conds, args
}

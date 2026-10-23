package store

import (
	"database/sql"
	"strings"
)

func (s *Store) enqueueSessionAuthorTx(tx *sql.Tx, id string) error {
	var p syncSessionPayload
	if err := tx.QueryRow(`SELECT id, ifnull(project, ''), ifnull(ownership_mode, ''), directory, started_at, ended_at, summary, ifnull(author, '') FROM sessions WHERE id = ?`, id).Scan(&p.ID, &p.Project, &p.OwnershipMode, &p.Directory, &p.StartedAt, &p.EndedAt, &p.Summary, &p.Author); err != nil {
		return err
	}
	return s.enqueueSyncMutationTx(tx, SyncEntitySession, id, SyncOpUpsert, p)
}

// setSessionAuthorTx refreshes live runtime provenance without allowing an empty
// registration to erase it. Call only after ownership/terminal-state validation.
func (s *Store) setSessionAuthorTx(tx *sql.Tx, id string, authors []string) (bool, error) {
	if len(authors) == 0 || strings.TrimSpace(authors[0]) == "" {
		return false, nil
	}
	author := strings.TrimSpace(authors[0])
	result, err := s.execHook(tx, `UPDATE sessions SET author = ? WHERE id = ? AND ifnull(author, '') != ?`, author, id, author)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
}

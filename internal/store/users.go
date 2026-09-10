package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrProtectedAdmin = errors.New("the super-admin account is protected")
var ErrInvalidUserState = errors.New("the account is not in a valid state for this action")
var ErrLibraryNotArchived = errors.New("only an archived account's library can be deleted")
var ErrLibraryTrashed = errors.New("restore the library from the recycle bin before restoring this account")
var ErrLibraryBusy = errors.New("the library still has active upload jobs")
var ErrPasswordChanged = errors.New("the password changed before it could be updated")

// UpdateUser changes account settings without changing the account or library IDs.
// Keeping those IDs stable means existing folders and public clip links still work.
func (s *Store) UpdateUser(ctx context.Context, id int64, username, normalized string, limit int64) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `
		UPDATE users
		SET username = ?, username_normalized = ?, stored_file_limit_bytes = ?, updated_at = ?
		WHERE id = ?`, username, normalized, limit, now, id)
	if err != nil {
		return User{}, classifyConstraint(err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return User{}, err
	}
	if changed == 0 {
		return User{}, sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE folders SET name = ?, name_normalized = ?, updated_at = ?
		WHERE owner_user_id = ? AND is_root = 1`, username, normalized, now, id); err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return s.UserByID(ctx, id)
}

func (s *Store) SetUserPassword(ctx context.Context, id int64, passwordHash string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		passwordHash, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// PasswordHashForActiveUser returns the hash used to authenticate the current
// session holder. The caller must verify it before passing it to ChangeOwnPassword.
func (s *Store) PasswordHashForActiveUser(ctx context.Context, id int64) (string, error) {
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ? AND state = 'active'`, id).Scan(&hash)
	return hash, err
}

// ChangeOwnPassword uses the verified hash as a compare-and-swap guard so an
// administrator reset or another self-service change cannot be overwritten.
func (s *Store) ChangeOwnPassword(ctx context.Context, id int64, verifiedHash, replacementHash string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE users SET password_hash = ?, updated_at = ?
		WHERE id = ? AND state = 'active' AND password_hash = ?`,
		replacementHash, time.Now().UTC().Format(time.RFC3339Nano), id, verifiedHash)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return ErrPasswordChanged
	}
	return nil
}

func (s *Store) DisableUser(ctx context.Context, id int64) (User, error) {
	return s.changeUserState(ctx, id, []string{"active", "disabled"}, "disabled", false)
}

func (s *Store) EnableUser(ctx context.Context, id int64) (User, error) {
	return s.changeUserState(ctx, id, []string{"active", "disabled"}, "active", false)
}

func (s *Store) ArchiveUser(ctx context.Context, id int64) (User, error) {
	return s.changeUserState(ctx, id, []string{"active", "disabled"}, "archived", true)
}

// RestoreUser assigns a replacement password in the same transaction that makes
// an archived account active again, so it is never restored with stale credentials.
func (s *Store) RestoreUser(ctx context.Context, id int64, passwordHash string) (User, error) {
	var role, state string
	if err := s.db.QueryRowContext(ctx, "SELECT role, state FROM users WHERE id = ?", id).Scan(&role, &state); err != nil {
		return User{}, err
	}
	if role == "admin" {
		return User{}, ErrProtectedAdmin
	}
	if state != "archived" {
		return User{}, ErrInvalidUserState
	}
	var trashed bool
	if err := s.db.QueryRowContext(ctx, `SELECT deleted_at IS NOT NULL FROM folders WHERE owner_user_id = ? AND is_root = 1`, id).Scan(&trashed); err != nil {
		return User{}, err
	}
	if trashed {
		return User{}, ErrLibraryTrashed
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET state = 'active', password_hash = ?, archived_at = NULL, updated_at = ?
		WHERE id = ? AND state = 'archived'`,
		passwordHash, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return User{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return User{}, err
	}
	if changed == 0 {
		return User{}, ErrInvalidUserState
	}
	return s.UserByID(ctx, id)
}

// TrashArchivedLibrary is deliberately separate from ArchiveUser. It turns the
// account root into a recoverable trash item, which makes every clip URL in its
// subtree unavailable in the same transaction while preserving the user record.
func (s *Store) TrashArchivedLibrary(ctx context.Context, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var role, state string
	if err := tx.QueryRowContext(ctx, `SELECT role, state FROM users WHERE id = ?`, userID).Scan(&role, &state); err != nil {
		return err
	}
	if role == "admin" {
		return ErrProtectedAdmin
	}
	if state != "archived" {
		return ErrLibraryNotArchived
	}
	var rootID int64
	var deletedAt sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT id, deleted_at FROM folders WHERE owner_user_id = ? AND is_root = 1`, userID).Scan(&rootID, &deletedAt); err != nil {
		return err
	}
	if deletedAt.Valid {
		return ErrTrashNotFound
	}
	var activeJobs int
	if err := tx.QueryRowContext(ctx, `WITH RECURSIVE subtree(id) AS (SELECT ? UNION ALL SELECT f.id FROM folders f JOIN subtree s ON f.parent_folder_id = s.id) SELECT COUNT(*) FROM jobs j JOIN clips c ON c.id = j.clip_id WHERE c.parent_folder_id IN (SELECT id FROM subtree) AND j.state IN ('queued', 'validating', 'processing')`, rootID).Scan(&activeJobs); err != nil {
		return err
	}
	if activeJobs != 0 {
		return ErrLibraryBusy
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `WITH RECURSIVE subtree(id) AS (SELECT ? UNION ALL SELECT f.id FROM folders f JOIN subtree s ON f.parent_folder_id = s.id) UPDATE folders SET deleted_at = ?, original_parent_folder_id = CASE WHEN id = ? THEN id ELSE original_parent_folder_id END, updated_at = ? WHERE id IN (SELECT id FROM subtree)`, rootID, now, rootID, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `WITH RECURSIVE subtree(id) AS (SELECT ? UNION ALL SELECT f.id FROM folders f JOIN subtree s ON f.parent_folder_id = s.id) UPDATE clips SET deleted_at = ?, original_parent_folder_id = parent_folder_id, updated_at = ? WHERE parent_folder_id IN (SELECT id FROM subtree) AND deleted_at IS NULL`, rootID, now, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) changeUserState(ctx context.Context, id int64, allowed []string, next string, archived bool) (User, error) {
	var role, state string
	if err := s.db.QueryRowContext(ctx, "SELECT role, state FROM users WHERE id = ?", id).Scan(&role, &state); err != nil {
		return User{}, err
	}
	if role == "admin" {
		return User{}, ErrProtectedAdmin
	}
	valid := false
	for _, candidate := range allowed {
		if state == candidate {
			valid = true
			break
		}
	}
	if !valid {
		return User{}, ErrInvalidUserState
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	var archivedAt any
	if archived {
		archivedAt = now
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE users SET state = ?, archived_at = ?, updated_at = ? WHERE id = ?`,
		next, archivedAt, now, id); err != nil {
		return User{}, err
	}
	return s.UserByID(ctx, id)
}

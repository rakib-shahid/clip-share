package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrTrashConflict = errors.New("trash restore name conflict")
var ErrTrashNotFound = errors.New("trash item not found")

type TrashItem struct {
	ID          int64   `json:"id"`
	Kind        string  `json:"kind"`
	Name        string  `json:"name"`
	OwnerUserID int64   `json:"ownerUserId"`
	OwnerName   string  `json:"ownerUsername"`
	DeletedAt   string  `json:"deletedAt"`
	PublicID    *string `json:"publicId"`
}

// ListTrash returns top-level deleted folders and individually deleted clips.
// Normal users receive only their own last 30 days; an admin receives all owners'
// last 90 days.
func (s *Store) ListTrash(ctx context.Context, owner *int64) ([]TrashItem, error) {
	retention := 90 * 24 * time.Hour
	if owner != nil {
		retention = 30 * 24 * time.Hour
	}
	cutoff := time.Now().UTC().Add(-retention).Format(time.RFC3339Nano)
	query := `
		SELECT f.id AS id, 'folder' AS kind, f.name AS name,
		       f.owner_user_id AS owner_user_id, u.username AS owner_username,
		       f.deleted_at AS deleted_at, NULL AS public_id
		FROM folders f JOIN users u ON u.id = f.owner_user_id
		WHERE f.deleted_at IS NOT NULL AND f.original_parent_folder_id IS NOT NULL AND f.deleted_at >= ?
		UNION ALL
		SELECT c.id, 'clip', c.title, c.owner_user_id, u.username, c.deleted_at, c.public_id
		FROM clips c JOIN users u ON u.id = c.owner_user_id
		WHERE c.deleted_at IS NOT NULL AND c.deleted_at >= ?
		AND NOT EXISTS (
			SELECT 1 FROM folders f
			WHERE f.id = c.parent_folder_id AND f.deleted_at IS NOT NULL
		)
	`
	args := []any{cutoff, cutoff}
	if owner != nil {
		query = `SELECT * FROM (` + query + `) WHERE owner_user_id = ?`
		args = append(args, *owner)
	}
	query += ` ORDER BY deleted_at DESC, id DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]TrashItem, 0)
	for rows.Next() {
		var item TrashItem
		if err := rows.Scan(&item.ID, &item.Kind, &item.Name, &item.OwnerUserID, &item.OwnerName, &item.DeletedAt, &item.PublicID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// TrashClipForPreview returns a ready clip only while the actor may still see it
// in trash. The caller uses StorageID to serve authenticated preview media; the
// public clip lookup deliberately remains disabled.
func (s *Store) TrashClipForPreview(ctx context.Context, id int64, owner *int64) (PublicClip, error) {
	retention := 90 * 24 * time.Hour
	if owner != nil {
		retention = 30 * 24 * time.Hour
	}
	cutoff := time.Now().UTC().Add(-retention).Format(time.RFC3339Nano)
	query := `
		SELECT c.id, c.public_id, c.storage_id, c.title, c.size_bytes, c.duration_seconds
		FROM clips c
		WHERE c.id = ? AND c.state = 'ready' AND c.deleted_at IS NOT NULL AND c.deleted_at >= ?
		AND NOT EXISTS (SELECT 1 FROM folders f WHERE f.id = c.parent_folder_id AND f.deleted_at IS NOT NULL)`
	args := []any{id, cutoff}
	if owner != nil {
		query += ` AND c.owner_user_id = ?`
		args = append(args, *owner)
	}
	var clip PublicClip
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&clip.ID, &clip.PublicID, &clip.StorageID, &clip.Title, &clip.SizeBytes, &clip.DurationSeconds)
	if errors.Is(err, sql.ErrNoRows) {
		return PublicClip{}, ErrTrashNotFound
	}
	return clip, err
}

// PurgeTrashItem removes one top-level trash item. removeAsset runs while the
// database transaction is open, preventing a concurrent restore from racing
// physical media removal. Missing files should be treated as success by the
// callback so a partially completed purge is safe to retry.
func (s *Store) PurgeTrashItem(ctx context.Context, kind string, id int64, before *time.Time, removeAsset func(string) error) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	cutoffClause := ""
	args := []any{id}
	if before != nil {
		cutoffClause = " AND deleted_at <= ?"
		args = append(args, before.UTC().Format(time.RFC3339Nano))
	}

	storageIDs := make([]string, 0)
	var purgedRoot bool
	var rootOwnerID int64
	switch kind {
	case "clip":
		query := `SELECT c.storage_id FROM clips c WHERE c.id = ? AND c.deleted_at IS NOT NULL` + cutoffClause + `
			AND NOT EXISTS (SELECT 1 FROM folders f WHERE f.id = c.parent_folder_id AND f.deleted_at IS NOT NULL)`
		var storageID string
		if err := tx.QueryRowContext(ctx, query, args...).Scan(&storageID); errors.Is(err, sql.ErrNoRows) {
			return 0, ErrTrashNotFound
		} else if err != nil {
			return 0, err
		}
		storageIDs = append(storageIDs, storageID)
	case "folder":
		query := `SELECT id, is_root, owner_user_id FROM folders WHERE id = ? AND deleted_at IS NOT NULL AND original_parent_folder_id IS NOT NULL` + cutoffClause
		var found int64
		var isRoot bool
		if err := tx.QueryRowContext(ctx, query, args...).Scan(&found, &isRoot, &rootOwnerID); errors.Is(err, sql.ErrNoRows) {
			return 0, ErrTrashNotFound
		} else if err != nil {
			return 0, err
		}
		purgedRoot = isRoot
		rows, err := tx.QueryContext(ctx, `WITH RECURSIVE subtree(id) AS (SELECT ? UNION ALL SELECT f.id FROM folders f JOIN subtree s ON f.parent_folder_id = s.id) SELECT c.storage_id FROM clips c WHERE c.parent_folder_id IN (SELECT id FROM subtree)`, id)
		if err != nil {
			return 0, err
		}
		for rows.Next() {
			var storageID string
			if err := rows.Scan(&storageID); err != nil {
				rows.Close()
				return 0, err
			}
			storageIDs = append(storageIDs, storageID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return 0, err
		}
		if err := rows.Close(); err != nil {
			return 0, err
		}
	default:
		return 0, ErrTrashNotFound
	}

	for _, storageID := range storageIDs {
		if err := removeAsset(storageID); err != nil {
			return 0, fmt.Errorf("remove stored media: %w", err)
		}
	}

	if kind == "clip" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM storage_reservations WHERE job_id IN (SELECT id FROM jobs WHERE clip_id = ?)`, id); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM jobs WHERE clip_id = ?`, id); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM clips WHERE id = ? AND deleted_at IS NOT NULL`, id); err != nil {
			return 0, err
		}
	} else {
		const subtree = `WITH RECURSIVE subtree(id) AS (SELECT ? UNION ALL SELECT f.id FROM folders f JOIN subtree s ON f.parent_folder_id = s.id) `
		if _, err := tx.ExecContext(ctx, subtree+`DELETE FROM storage_reservations WHERE job_id IN (SELECT j.id FROM jobs j JOIN clips c ON c.id = j.clip_id WHERE c.parent_folder_id IN (SELECT id FROM subtree))`, id); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, subtree+`DELETE FROM jobs WHERE clip_id IN (SELECT c.id FROM clips c WHERE c.parent_folder_id IN (SELECT id FROM subtree))`, id); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, subtree+`DELETE FROM clips WHERE parent_folder_id IN (SELECT id FROM subtree)`, id); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, subtree+`DELETE FROM folders WHERE id IN (SELECT id FROM subtree)`, id); err != nil {
			return 0, err
		}
		if purgedRoot {
			if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ? AND state = 'archived'`, rootOwnerID); err != nil {
				return 0, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(storageIDs), nil
}

// PurgeExpiredTrash permanently removes top-level trash items at or beyond the
// global 90-day retention period.
func (s *Store) PurgeExpiredTrash(ctx context.Context, before time.Time, removeAsset func(string) error) (int, error) {
	cutoff := before.UTC().Format(time.RFC3339Nano)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, 'folder' FROM folders WHERE deleted_at IS NOT NULL AND original_parent_folder_id IS NOT NULL AND deleted_at <= ?
		UNION ALL
		SELECT c.id, 'clip' FROM clips c WHERE c.deleted_at IS NOT NULL AND c.deleted_at <= ?
		AND NOT EXISTS (SELECT 1 FROM folders f WHERE f.id = c.parent_folder_id AND f.deleted_at IS NOT NULL)
		ORDER BY id`, cutoff, cutoff)
	if err != nil {
		return 0, err
	}
	type candidate struct {
		id   int64
		kind string
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.kind); err != nil {
			rows.Close()
			return 0, err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	purged := 0
	for _, item := range candidates {
		if _, err := s.PurgeTrashItem(ctx, item.kind, item.id, &before, removeAsset); errors.Is(err, ErrTrashNotFound) {
			continue
		} else if err != nil {
			return purged, err
		}
		purged++
	}
	return purged, nil
}

func (s *Store) RestoreClip(ctx context.Context, id int64, owner *int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var clipOwner, originalParent int64
	var titleNormalized string
	err = tx.QueryRowContext(ctx, `SELECT owner_user_id, original_parent_folder_id, title_normalized FROM clips WHERE id = ? AND deleted_at IS NOT NULL`, id).Scan(&clipOwner, &originalParent, &titleNormalized)
	if errors.Is(err, sql.ErrNoRows) || owner != nil && clipOwner != *owner {
		return ErrClipNotFound
	}
	if err != nil {
		return err
	}
	parent, err := activeRestoreParent(ctx, tx, clipOwner, originalParent)
	if err != nil {
		return err
	}
	var conflict int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM clips WHERE parent_folder_id = ? AND title_normalized = ? AND deleted_at IS NULL AND state NOT IN ('failed','cancelled')`, parent, titleNormalized).Scan(&conflict); err != nil {
		return err
	}
	if conflict != 0 {
		return ErrTrashConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE clips SET deleted_at = NULL, parent_folder_id = ?, original_parent_folder_id = NULL, updated_at = ? WHERE id = ?`, parent, time.Now().UTC().Format(time.RFC3339Nano), id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RestoreFolder(ctx context.Context, id int64, owner *int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var folderOwner, originalParent int64
	var isRoot bool
	var normalized string
	err = tx.QueryRowContext(ctx, `SELECT owner_user_id, original_parent_folder_id, name_normalized, is_root FROM folders WHERE id = ? AND deleted_at IS NOT NULL AND original_parent_folder_id IS NOT NULL`, id).Scan(&folderOwner, &originalParent, &normalized, &isRoot)
	if errors.Is(err, sql.ErrNoRows) || owner != nil && folderOwner != *owner {
		return ErrFolderNotFound
	}
	if err != nil {
		return err
	}
	var parent int64
	if isRoot {
		if owner != nil {
			return ErrFolderNotFound
		}
	} else {
		parent, err = activeRestoreParent(ctx, tx, folderOwner, originalParent)
		if err != nil {
			return err
		}
		var conflict int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM folders WHERE owner_user_id = ? AND parent_folder_id = ? AND name_normalized = ? AND deleted_at IS NULL`, folderOwner, parent, normalized).Scan(&conflict); err != nil {
			return err
		}
		if conflict != 0 {
			return ErrTrashConflict
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `WITH RECURSIVE subtree(id) AS (SELECT ? UNION ALL SELECT f.id FROM folders f JOIN subtree s ON f.parent_folder_id = s.id) UPDATE folders SET deleted_at = NULL, updated_at = ? WHERE id IN (SELECT id FROM subtree)`, id, now); err != nil {
		return err
	}
	if isRoot {
		_, err = tx.ExecContext(ctx, `UPDATE folders SET parent_folder_id = NULL, original_parent_folder_id = NULL WHERE id = ?`, id)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE folders SET parent_folder_id = ?, original_parent_folder_id = NULL WHERE id = ?`, parent, id)
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `WITH RECURSIVE subtree(id) AS (SELECT ? UNION ALL SELECT f.id FROM folders f JOIN subtree s ON f.parent_folder_id = s.id) UPDATE clips SET deleted_at = NULL, original_parent_folder_id = NULL, updated_at = ? WHERE parent_folder_id IN (SELECT id FROM subtree)`, id, now); err != nil {
		return err
	}
	return tx.Commit()
}

func activeRestoreParent(ctx context.Context, tx *sql.Tx, ownerID, originalParent int64) (int64, error) {
	var parent int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM folders WHERE id = ? AND owner_user_id = ? AND deleted_at IS NULL`, originalParent, ownerID).Scan(&parent)
	if err == nil {
		return parent, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT id FROM folders WHERE owner_user_id = ? AND is_root = 1 AND deleted_at IS NULL`, ownerID).Scan(&parent); err != nil {
		return 0, fmt.Errorf("find restore root: %w", err)
	}
	return parent, nil
}

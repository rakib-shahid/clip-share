package store

import "context"

// FolderDeletionSummary describes everything that would move to trash with one
// folder deletion. FolderCount includes the selected folder itself.
type FolderDeletionSummary struct {
	FolderCount int   `json:"folderCount"`
	ClipCount   int   `json:"clipCount"`
	TotalItems  int   `json:"totalItems"`
	StoredBytes int64 `json:"storedBytes"`
}

func (s *Store) FolderDeletionSummary(ctx context.Context, folderID int64, requiredOwnerID *int64) (FolderDeletionSummary, error) {
	folder, err := folderByID(ctx, s.db, folderID)
	if err != nil {
		return FolderDeletionSummary{}, err
	}
	if folder.IsRoot {
		return FolderDeletionSummary{}, ErrProtectedRoot
	}
	if requiredOwnerID != nil && folder.OwnerUserID != *requiredOwnerID {
		return FolderDeletionSummary{}, ErrFolderNotFound
	}

	var summary FolderDeletionSummary
	err = s.db.QueryRowContext(ctx, `
		WITH RECURSIVE subtree(id) AS (
			SELECT id FROM folders WHERE id = ? AND deleted_at IS NULL
			UNION ALL
			SELECT f.id FROM folders f
			JOIN subtree s ON f.parent_folder_id = s.id
			WHERE f.deleted_at IS NULL
		)
		SELECT
			(SELECT COUNT(*) FROM subtree),
			(SELECT COUNT(*) FROM clips WHERE parent_folder_id IN (SELECT id FROM subtree) AND deleted_at IS NULL AND state != 'cancelled'),
			(SELECT COALESCE(SUM(size_bytes), 0) FROM clips WHERE parent_folder_id IN (SELECT id FROM subtree) AND deleted_at IS NULL AND state != 'cancelled')`, folderID).
		Scan(&summary.FolderCount, &summary.ClipCount, &summary.StoredBytes)
	if err != nil {
		return FolderDeletionSummary{}, err
	}
	summary.TotalItems = summary.FolderCount + summary.ClipCount
	return summary, nil
}

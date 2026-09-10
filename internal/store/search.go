package store

import "context"

type SearchResult struct {
	Kind          string  `json:"kind"`
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	OwnerUserID   int64   `json:"ownerUserId"`
	OwnerUsername string  `json:"ownerUsername"`
	FolderID      int64   `json:"folderId"`
	Path          string  `json:"path"`
	PublicID      *string `json:"publicId,omitempty"`
	State         *string `json:"state,omitempty"`
	SizeBytes     *int64  `json:"sizeBytes,omitempty"`
}

type SearchResults struct {
	Results   []SearchResult `json:"results"`
	Truncated bool           `json:"truncated"`
}

// Search finds active folders and clips. requiredOwnerID is nil for the
// super-admin and points to the signed-in user's ID for a normal account.
func (s *Store) Search(ctx context.Context, normalizedQuery string, requiredOwnerID *int64) (SearchResults, error) {
	result := SearchResults{Results: []SearchResult{}}
	if normalizedQuery == "" {
		return result, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		WITH RECURSIVE folder_paths(id, path) AS (
			SELECT id, name FROM folders
			WHERE is_root = 1 AND deleted_at IS NULL
			UNION ALL
			SELECT f.id, folder_paths.path || '/' || f.name
			FROM folders f JOIN folder_paths ON f.parent_folder_id = folder_paths.id
			WHERE f.deleted_at IS NULL
		), matches(kind_order, kind, id, name, sort_name, owner_user_id, owner_username, folder_id, path, public_id, state, size_bytes, created_at) AS (
			SELECT 0, 'folder', f.id, f.name, f.name_normalized, f.owner_user_id, u.username,
			       f.id, folder_paths.path, NULL, NULL, NULL, f.created_at
			FROM folders f
			JOIN users u ON u.id = f.owner_user_id
			JOIN folder_paths ON folder_paths.id = f.id
			WHERE f.is_root = 0 AND f.deleted_at IS NULL
			  AND instr(f.name_normalized, ?) > 0
			  AND (? IS NULL OR f.owner_user_id = ?)
			UNION ALL
			SELECT 1, 'clip', c.id, c.title, c.title_normalized, c.owner_user_id, u.username,
			       c.parent_folder_id, folder_paths.path || '/' || c.title, c.public_id, c.state, c.size_bytes, c.created_at
			FROM clips c
			JOIN users u ON u.id = c.owner_user_id
			JOIN folder_paths ON folder_paths.id = c.parent_folder_id
			WHERE c.deleted_at IS NULL AND c.state != 'cancelled'
			  AND instr(c.title_normalized, ?) > 0
			  AND (? IS NULL OR c.owner_user_id = ?)
		)
		SELECT kind, id, name, owner_user_id, owner_username, folder_id, path, public_id, state, size_bytes
		FROM matches
		ORDER BY kind_order, CASE WHEN kind_order = 0 THEN sort_name END,
		         CASE WHEN kind_order = 1 THEN created_at END DESC, id
		LIMIT 101`, normalizedQuery, requiredOwnerID, requiredOwnerID, normalizedQuery, requiredOwnerID, requiredOwnerID)
	if err != nil {
		return SearchResults{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item SearchResult
		if err := rows.Scan(&item.Kind, &item.ID, &item.Name, &item.OwnerUserID, &item.OwnerUsername, &item.FolderID, &item.Path, &item.PublicID, &item.State, &item.SizeBytes); err != nil {
			return SearchResults{}, err
		}
		if len(result.Results) == 100 {
			result.Truncated = true
			break
		}
		result.Results = append(result.Results, item)
	}
	return result, rows.Err()
}

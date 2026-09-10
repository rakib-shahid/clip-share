package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	ErrFolderNotFound  = errors.New("folder not found")
	ErrFolderNameTaken = errors.New("folder name is already in use")
	ErrProtectedRoot   = errors.New("account roots cannot be changed")
	ErrInvalidMove     = errors.New("folder cannot be moved there")
	ErrFolderDepth     = errors.New("folder nesting cannot exceed 20 levels")
)

type Folder struct {
	ID             int64  `json:"id"`
	OwnerUserID    int64  `json:"ownerUserId"`
	OwnerUsername  string `json:"ownerUsername"`
	ParentFolderID *int64 `json:"parentFolderId"`
	Name           string `json:"name"`
	IsRoot         bool   `json:"isRoot"`
	FolderCount    int    `json:"folderCount"`
	ClipCount      int    `json:"clipCount"`
}

type ClipSummary struct {
	ID           int64   `json:"id"`
	Title        string  `json:"title"`
	State        string  `json:"state"`
	SizeBytes    *int64  `json:"sizeBytes"`
	CreatedAt    string  `json:"createdAt"`
	JobID        *int64  `json:"jobId"`
	Progress     *int    `json:"progress"`
	ErrorMessage *string `json:"errorMessage"`
	PublicID     string  `json:"publicId"`
}

type PublicClip struct {
	ID              int64
	PublicID        string
	StorageID       string
	Title           string
	SizeBytes       *int64
	DurationSeconds *float64
}

var ErrClipNotFound = errors.New("clip not found")

func (s *Store) PublicClipByID(ctx context.Context, publicID string) (PublicClip, error) {
	var clip PublicClip
	err := s.db.QueryRowContext(ctx, `SELECT id, public_id, storage_id, title, size_bytes, duration_seconds FROM clips WHERE public_id = ? AND state = 'ready' AND deleted_at IS NULL`, publicID).
		Scan(&clip.ID, &clip.PublicID, &clip.StorageID, &clip.Title, &clip.SizeBytes, &clip.DurationSeconds)
	if errors.Is(err, sql.ErrNoRows) {
		return PublicClip{}, ErrClipNotFound
	}
	return clip, err
}

type FolderPage struct {
	Folder      Folder        `json:"folder"`
	Breadcrumbs []Folder      `json:"breadcrumbs"`
	Folders     []Folder      `json:"folders"`
	Clips       []ClipSummary `json:"clips"`
	NextCursor  *string       `json:"nextCursor"`
}

func (s *Store) FolderPage(ctx context.Context, folderID int64, offset int) (FolderPage, error) {
	folder, err := folderByID(ctx, s.db, folderID)
	if err != nil {
		return FolderPage{}, err
	}
	page := FolderPage{Folder: folder, Breadcrumbs: []Folder{}, Folders: []Folder{}, Clips: []ClipSummary{}}

	rows, err := s.db.QueryContext(ctx, `
		WITH RECURSIVE ancestors(id, owner_user_id, parent_folder_id, name, is_root, depth) AS (
			SELECT id, owner_user_id, parent_folder_id, name, is_root, 0 FROM folders WHERE id = ? AND deleted_at IS NULL
			UNION ALL
			SELECT f.id, f.owner_user_id, f.parent_folder_id, f.name, f.is_root, a.depth + 1
			FROM folders f JOIN ancestors a ON a.parent_folder_id = f.id WHERE f.deleted_at IS NULL
		)
		SELECT a.id, a.owner_user_id, u.username, a.parent_folder_id, a.name, a.is_root
		FROM ancestors a JOIN users u ON u.id = a.owner_user_id ORDER BY a.depth DESC`, folderID)
	if err != nil {
		return FolderPage{}, err
	}
	for rows.Next() {
		item, err := scanFolder(rows)
		if err != nil {
			return FolderPage{}, err
		}
		page.Breadcrumbs = append(page.Breadcrumbs, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return FolderPage{}, err
	}
	if err := rows.Close(); err != nil {
		return FolderPage{}, err
	}

	var folderCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM folders WHERE parent_folder_id = ? AND deleted_at IS NULL`, folderID).Scan(&folderCount); err != nil {
		return FolderPage{}, err
	}
	folderOffset := offset
	if folderOffset > folderCount {
		folderOffset = folderCount
	}
	rows, err = s.db.QueryContext(ctx, `
		SELECT f.id, f.owner_user_id, u.username, f.parent_folder_id, f.name, f.is_root
		FROM folders f JOIN users u ON u.id = f.owner_user_id
		WHERE f.parent_folder_id = ? AND f.deleted_at IS NULL
		ORDER BY f.name_normalized, f.id LIMIT 61 OFFSET ?`, folderID, folderOffset)
	if err != nil {
		return FolderPage{}, err
	}
	for rows.Next() {
		item, err := scanFolder(rows)
		if err != nil {
			return FolderPage{}, err
		}
		if len(page.Folders) == 60 {
			cursor := strconv.Itoa(offset + 60)
			page.NextCursor = &cursor
			break
		}
		page.Folders = append(page.Folders, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return FolderPage{}, err
	}
	if err := rows.Close(); err != nil {
		return FolderPage{}, err
	}
	for i := range page.Folders {
		if err := s.folderCounts(ctx, &page.Folders[i]); err != nil {
			return FolderPage{}, err
		}
	}

	if page.NextCursor != nil {
		return page, nil
	}
	clipOffset := offset - folderCount
	if clipOffset < 0 {
		clipOffset = 0
	}
	remaining := 60 - len(page.Folders)
	clipRows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.title, c.state, c.size_bytes, c.created_at, j.id, j.progress, j.error_message, c.public_id
		FROM clips c LEFT JOIN jobs j ON j.clip_id = c.id
		WHERE c.parent_folder_id = ? AND c.deleted_at IS NULL AND c.state NOT IN ('uploading', 'cancelled')
		ORDER BY c.created_at DESC, c.id DESC LIMIT ? OFFSET ?`, folderID, remaining+1, clipOffset)
	if err != nil {
		return FolderPage{}, err
	}
	defer clipRows.Close()
	for clipRows.Next() {
		if len(page.Clips) == remaining {
			cursor := strconv.Itoa(offset + 60)
			page.NextCursor = &cursor
			break
		}
		var clip ClipSummary
		if err := clipRows.Scan(&clip.ID, &clip.Title, &clip.State, &clip.SizeBytes, &clip.CreatedAt, &clip.JobID, &clip.Progress, &clip.ErrorMessage, &clip.PublicID); err != nil {
			return FolderPage{}, err
		}
		page.Clips = append(page.Clips, clip)
	}
	return page, clipRows.Err()
}

func (s *Store) folderCounts(ctx context.Context, folder *Folder) error {
	return s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM folders WHERE parent_folder_id=? AND deleted_at IS NULL), (SELECT COUNT(*) FROM clips WHERE parent_folder_id=? AND deleted_at IS NULL AND state NOT IN ('uploading','failed','cancelled'))`, folder.ID, folder.ID).Scan(&folder.FolderCount, &folder.ClipCount)
}

func (s *Store) CreateFolder(ctx context.Context, parentID int64, name, normalized string, requiredOwnerID *int64) (Folder, error) {
	parent, err := folderByID(ctx, s.db, parentID)
	if err != nil {
		return Folder{}, err
	}
	if requiredOwnerID != nil && parent.OwnerUserID != *requiredOwnerID {
		return Folder{}, ErrFolderNotFound
	}
	depth, err := folderDepth(ctx, s.db, parentID)
	if err != nil {
		return Folder{}, err
	}
	if depth >= 20 {
		return Folder{}, ErrFolderDepth
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `INSERT INTO folders(owner_user_id, parent_folder_id, name, name_normalized, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, parent.OwnerUserID, parentID, name, normalized, now, now)
	if err != nil {
		return Folder{}, classifyFolderConstraint(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Folder{}, err
	}
	return folderByID(ctx, s.db, id)
}

func (s *Store) RenameFolder(ctx context.Context, folderID int64, name, normalized string, requiredOwnerID *int64) (Folder, error) {
	folder, err := folderByID(ctx, s.db, folderID)
	if err != nil {
		return Folder{}, err
	}
	if folder.IsRoot {
		return Folder{}, ErrProtectedRoot
	}
	if requiredOwnerID != nil && folder.OwnerUserID != *requiredOwnerID {
		return Folder{}, ErrFolderNotFound
	}
	_, err = s.db.ExecContext(ctx, `UPDATE folders SET name = ?, name_normalized = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, name, normalized, time.Now().UTC().Format(time.RFC3339Nano), folderID)
	if err != nil {
		return Folder{}, classifyFolderConstraint(err)
	}
	return folderByID(ctx, s.db, folderID)
}

func (s *Store) TrashFolder(ctx context.Context, folderID int64, requiredOwnerID *int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	folder, err := folderByID(ctx, tx, folderID)
	if err != nil {
		return err
	}
	if folder.IsRoot {
		return ErrProtectedRoot
	}
	if requiredOwnerID != nil && folder.OwnerUserID != *requiredOwnerID {
		return ErrFolderNotFound
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, `WITH RECURSIVE subtree(id) AS (SELECT id FROM folders WHERE id = ? UNION ALL SELECT f.id FROM folders f JOIN subtree s ON f.parent_folder_id = s.id) UPDATE folders SET deleted_at = ?, original_parent_folder_id = CASE WHEN id = ? THEN parent_folder_id ELSE original_parent_folder_id END, updated_at = ? WHERE id IN (SELECT id FROM subtree)`, folderID, now, folderID, now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `WITH RECURSIVE subtree(id) AS (SELECT id FROM folders WHERE id = ? UNION ALL SELECT f.id FROM folders f JOIN subtree s ON f.parent_folder_id = s.id) UPDATE clips SET deleted_at = ?, original_parent_folder_id = parent_folder_id, updated_at = ? WHERE parent_folder_id IN (SELECT id FROM subtree) AND deleted_at IS NULL`, folderID, now, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) MoveFolder(ctx context.Context, folderID, destinationID int64, requiredOwnerID *int64) (Folder, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Folder{}, err
	}
	defer tx.Rollback()
	folder, err := folderByID(ctx, tx, folderID)
	if err != nil {
		return Folder{}, err
	}
	if folder.IsRoot {
		return Folder{}, ErrProtectedRoot
	}
	if requiredOwnerID != nil && folder.OwnerUserID != *requiredOwnerID {
		return Folder{}, ErrFolderNotFound
	}
	destination, err := folderByID(ctx, tx, destinationID)
	if err != nil {
		return Folder{}, err
	}
	if requiredOwnerID != nil && destination.OwnerUserID != *requiredOwnerID {
		return Folder{}, ErrFolderNotFound
	}
	if folderID == destinationID {
		return Folder{}, ErrInvalidMove
	}
	var inside int
	err = tx.QueryRowContext(ctx, `WITH RECURSIVE subtree(id) AS (SELECT id FROM folders WHERE id = ? UNION ALL SELECT f.id FROM folders f JOIN subtree s ON f.parent_folder_id = s.id WHERE f.deleted_at IS NULL) SELECT COUNT(*) FROM subtree WHERE id = ?`, folderID, destinationID).Scan(&inside)
	if err != nil {
		return Folder{}, err
	}
	if inside != 0 {
		return Folder{}, ErrInvalidMove
	}
	destinationDepth, err := folderDepth(ctx, tx, destinationID)
	if err != nil {
		return Folder{}, err
	}
	var subtreeDepth int
	err = tx.QueryRowContext(ctx, `WITH RECURSIVE subtree(id, depth) AS (SELECT id, 0 FROM folders WHERE id = ? UNION ALL SELECT f.id, s.depth + 1 FROM folders f JOIN subtree s ON f.parent_folder_id = s.id WHERE f.deleted_at IS NULL) SELECT COALESCE(MAX(depth), 0) FROM subtree`, folderID).Scan(&subtreeDepth)
	if err != nil {
		return Folder{}, err
	}
	if destinationDepth+1+subtreeDepth > 20 {
		return Folder{}, ErrFolderDepth
	}

	if _, err = tx.ExecContext(ctx, `UPDATE folders SET parent_folder_id = ?, updated_at = ? WHERE id = ?`, destinationID, time.Now().UTC().Format(time.RFC3339Nano), folderID); err != nil {
		return Folder{}, classifyFolderConstraint(err)
	}
	if folder.OwnerUserID != destination.OwnerUserID {
		if _, err = tx.ExecContext(ctx, `WITH RECURSIVE subtree(id) AS (SELECT ? UNION ALL SELECT f.id FROM folders f JOIN subtree s ON f.parent_folder_id = s.id) UPDATE folders SET owner_user_id = ? WHERE id IN (SELECT id FROM subtree)`, folderID, destination.OwnerUserID); err != nil {
			return Folder{}, err
		}
		if _, err = tx.ExecContext(ctx, `WITH RECURSIVE subtree(id) AS (SELECT ? UNION ALL SELECT f.id FROM folders f JOIN subtree s ON f.parent_folder_id = s.id) UPDATE clips SET owner_user_id = ? WHERE parent_folder_id IN (SELECT id FROM subtree)`, folderID, destination.OwnerUserID); err != nil {
			return Folder{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Folder{}, err
	}
	return folderByID(ctx, s.db, folderID)
}

type rowScanner interface{ Scan(dest ...any) error }

func folderByID(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id int64) (Folder, error) {
	row := queryer.QueryRowContext(ctx, `SELECT f.id, f.owner_user_id, u.username, f.parent_folder_id, f.name, f.is_root FROM folders f JOIN users u ON u.id = f.owner_user_id WHERE f.id = ? AND f.deleted_at IS NULL`, id)
	folder, err := scanFolder(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Folder{}, ErrFolderNotFound
	}
	return folder, err
}

func scanFolder(row rowScanner) (Folder, error) {
	var folder Folder
	var isRoot int
	err := row.Scan(&folder.ID, &folder.OwnerUserID, &folder.OwnerUsername, &folder.ParentFolderID, &folder.Name, &isRoot)
	folder.IsRoot = isRoot == 1
	return folder, err
}

func folderDepth(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id int64) (int, error) {
	var depth int
	err := queryer.QueryRowContext(ctx, `WITH RECURSIVE ancestors(id, parent_folder_id, depth) AS (SELECT id, parent_folder_id, 0 FROM folders WHERE id = ? AND deleted_at IS NULL UNION ALL SELECT f.id, f.parent_folder_id, a.depth + 1 FROM folders f JOIN ancestors a ON a.parent_folder_id = f.id WHERE f.deleted_at IS NULL) SELECT COALESCE(MAX(depth), 0) FROM ancestors`, id).Scan(&depth)
	return depth, err
}

func classifyFolderConstraint(err error) error {
	if strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
		return ErrFolderNameTaken
	}
	return err
}

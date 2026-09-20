package store

import (
	"context"
	"database/sql"
	"errors"
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
	Folder           Folder        `json:"folder"`
	Breadcrumbs      []Folder      `json:"breadcrumbs"`
	Folders          []Folder      `json:"folders"`
	Clips            []ClipSummary `json:"clips"`
	NextCursor       *string       `json:"nextCursor"`
	TotalFolderCount int           `json:"totalFolderCount"`
	TotalClipCount   int           `json:"totalClipCount"`
	TotalItemCount   int           `json:"totalItemCount"`
}

type FolderPageOptions struct {
	Sort   FolderSort
	Cursor *FolderCursor
}

const folderPageSize = 60

func (s *Store) FolderPage(ctx context.Context, folderID int64, options FolderPageOptions) (FolderPage, error) {
	sort, err := ParseFolderSort(string(options.Sort))
	if err != nil {
		return FolderPage{}, err
	}
	if options.Cursor != nil && (options.Cursor.FolderID != folderID || options.Cursor.Sort != sort) {
		return FolderPage{}, ErrInvalidCursor
	}
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

	if err := s.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM folders WHERE parent_folder_id = ? AND deleted_at IS NULL),
			(SELECT COUNT(*) FROM clips WHERE parent_folder_id = ? AND deleted_at IS NULL AND state NOT IN ('uploading', 'cancelled'))`,
		folderID, folderID).Scan(&page.TotalFolderCount, &page.TotalClipCount); err != nil {
		return FolderPage{}, err
	}
	page.TotalItemCount = page.TotalFolderCount + page.TotalClipCount

	remaining := folderPageSize
	if options.Cursor == nil || options.Cursor.Phase == CursorFolders {
		var cursor *FolderCursor
		if options.Cursor != nil {
			cursor = options.Cursor
		}
		folderRows, err := s.folderPageRows(ctx, folderID, sort, cursor, remaining+1)
		if err != nil {
			return FolderPage{}, err
		}
		if len(folderRows) > remaining || (len(folderRows) == remaining && page.TotalClipCount > 0) {
			folderRows = folderRows[:remaining]
			last := folderRows[len(folderRows)-1]
			if err := setNextFolderCursor(&page, folderID, sort, last.folder.ID, last.normalized); err != nil {
				return FolderPage{}, err
			}
		}
		for _, row := range folderRows {
			page.Folders = append(page.Folders, row.folder)
		}
		for i := range page.Folders {
			if err := s.folderCounts(ctx, &page.Folders[i]); err != nil {
				return FolderPage{}, err
			}
		}
		if page.NextCursor != nil {
			return page, nil
		}
		remaining -= len(page.Folders)
	}

	if remaining == 0 {
		return page, nil
	}
	var clipCursor *FolderCursor
	if options.Cursor != nil && options.Cursor.Phase == CursorClips {
		clipCursor = options.Cursor
	}
	clipRows, err := s.clipPageRows(ctx, folderID, sort, clipCursor, remaining+1)
	if err != nil {
		return FolderPage{}, err
	}
	if len(clipRows) > remaining {
		clipRows = clipRows[:remaining]
		last := clipRows[len(clipRows)-1]
		if err := setNextClipCursor(&page, folderID, sort, last); err != nil {
			return FolderPage{}, err
		}
	}
	for _, row := range clipRows {
		page.Clips = append(page.Clips, row.clip)
	}
	return page, nil
}

type folderPageRow struct {
	folder     Folder
	normalized string
}

func (s *Store) folderPageRows(ctx context.Context, folderID int64, sort FolderSort, cursor *FolderCursor, limit int) ([]folderPageRow, error) {
	query := `SELECT f.id, f.owner_user_id, u.username, f.parent_folder_id, f.name, f.is_root, f.name_normalized
		FROM folders f JOIN users u ON u.id = f.owner_user_id
		WHERE f.parent_folder_id = ? AND f.deleted_at IS NULL`
	args := []any{folderID}
	descending := sort == SortNameDesc
	if cursor != nil {
		if cursor.Phase != CursorFolders || cursor.Name == "" {
			return nil, ErrInvalidCursor
		}
		if descending {
			query += ` AND (f.name_normalized < ? OR (f.name_normalized = ? AND f.id < ?))`
		} else {
			query += ` AND (f.name_normalized > ? OR (f.name_normalized = ? AND f.id > ?))`
		}
		args = append(args, cursor.Name, cursor.Name, cursor.ID)
	}
	if descending {
		query += ` ORDER BY f.name_normalized DESC, f.id DESC LIMIT ?`
	} else {
		query += ` ORDER BY f.name_normalized ASC, f.id ASC LIMIT ?`
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]folderPageRow, 0, limit)
	for rows.Next() {
		var row folderPageRow
		var isRoot int
		if err := rows.Scan(&row.folder.ID, &row.folder.OwnerUserID, &row.folder.OwnerUsername, &row.folder.ParentFolderID, &row.folder.Name, &isRoot, &row.normalized); err != nil {
			return nil, err
		}
		row.folder.IsRoot = isRoot == 1
		result = append(result, row)
	}
	return result, rows.Err()
}

type clipPageRow struct {
	clip       ClipSummary
	normalized string
	createdAt  time.Time
	nullRank   int
	stateRank  int
}

const clipStateRankSQL = `CASE c.state WHEN 'processing' THEN 0 WHEN 'validating' THEN 1 WHEN 'queued' THEN 2 WHEN 'failed' THEN 3 WHEN 'ready' THEN 4 ELSE 5 END`

func (s *Store) clipPageRows(ctx context.Context, folderID int64, sort FolderSort, cursor *FolderCursor, limit int) ([]clipPageRow, error) {
	query := `SELECT c.id, c.title, c.state, c.size_bytes, c.created_at, j.id, j.progress, j.error_message, c.public_id,
		c.title_normalized, CASE WHEN c.size_bytes IS NULL THEN 1 ELSE 0 END, ` + clipStateRankSQL + `
		FROM clips c LEFT JOIN jobs j ON j.clip_id = c.id
		WHERE c.parent_folder_id = ? AND c.deleted_at IS NULL AND c.state NOT IN ('uploading', 'cancelled')`
	args := []any{folderID}
	if cursor != nil && cursor.Phase != CursorClips {
		return nil, ErrInvalidCursor
	}
	if cursor != nil {
		timestamp := cursor.CreatedAt.UTC().Format(time.RFC3339Nano)
		switch sort {
		case SortLatest:
			query += ` AND (c.created_at < ? OR (c.created_at = ? AND c.id < ?))`
			args = append(args, timestamp, timestamp, cursor.ID)
		case SortOldest:
			query += ` AND (c.created_at > ? OR (c.created_at = ? AND c.id > ?))`
			args = append(args, timestamp, timestamp, cursor.ID)
		case SortNameAsc:
			query += ` AND (c.title_normalized > ? OR (c.title_normalized = ? AND c.id > ?))`
			args = append(args, cursor.Name, cursor.Name, cursor.ID)
		case SortNameDesc:
			query += ` AND (c.title_normalized < ? OR (c.title_normalized = ? AND c.id < ?))`
			args = append(args, cursor.Name, cursor.Name, cursor.ID)
		case SortSizeDesc:
			query += ` AND ((CASE WHEN c.size_bytes IS NULL THEN 1 ELSE 0 END) > ? OR ((CASE WHEN c.size_bytes IS NULL THEN 1 ELSE 0 END) = ? AND (COALESCE(c.size_bytes, 0) < ? OR (COALESCE(c.size_bytes, 0) = ? AND (c.created_at < ? OR (c.created_at = ? AND c.id < ?))))))`
			args = append(args, cursor.NullRank, cursor.NullRank, cursor.SizeBytes, cursor.SizeBytes, timestamp, timestamp, cursor.ID)
		case SortSizeAsc:
			query += ` AND ((CASE WHEN c.size_bytes IS NULL THEN 1 ELSE 0 END) > ? OR ((CASE WHEN c.size_bytes IS NULL THEN 1 ELSE 0 END) = ? AND (COALESCE(c.size_bytes, 0) > ? OR (COALESCE(c.size_bytes, 0) = ? AND (c.created_at < ? OR (c.created_at = ? AND c.id < ?))))))`
			args = append(args, cursor.NullRank, cursor.NullRank, cursor.SizeBytes, cursor.SizeBytes, timestamp, timestamp, cursor.ID)
		case SortState:
			query += ` AND (` + clipStateRankSQL + ` > ? OR (` + clipStateRankSQL + ` = ? AND (c.created_at < ? OR (c.created_at = ? AND c.id < ?))))`
			args = append(args, cursor.StateRank, cursor.StateRank, timestamp, timestamp, cursor.ID)
		}
	}
	switch sort {
	case SortLatest:
		query += ` ORDER BY c.created_at DESC, c.id DESC LIMIT ?`
	case SortOldest:
		query += ` ORDER BY c.created_at ASC, c.id ASC LIMIT ?`
	case SortNameAsc:
		query += ` ORDER BY c.title_normalized ASC, c.id ASC LIMIT ?`
	case SortNameDesc:
		query += ` ORDER BY c.title_normalized DESC, c.id DESC LIMIT ?`
	case SortSizeDesc:
		query += ` ORDER BY CASE WHEN c.size_bytes IS NULL THEN 1 ELSE 0 END ASC, COALESCE(c.size_bytes, 0) DESC, c.created_at DESC, c.id DESC LIMIT ?`
	case SortSizeAsc:
		query += ` ORDER BY CASE WHEN c.size_bytes IS NULL THEN 1 ELSE 0 END ASC, COALESCE(c.size_bytes, 0) ASC, c.created_at DESC, c.id DESC LIMIT ?`
	case SortState:
		query += ` ORDER BY ` + clipStateRankSQL + ` ASC, c.created_at DESC, c.id DESC LIMIT ?`
	}
	args = append(args, limit)
	clipRows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer clipRows.Close()
	result := make([]clipPageRow, 0, limit)
	for clipRows.Next() {
		var row clipPageRow
		if err := clipRows.Scan(&row.clip.ID, &row.clip.Title, &row.clip.State, &row.clip.SizeBytes, &row.clip.CreatedAt, &row.clip.JobID, &row.clip.Progress, &row.clip.ErrorMessage, &row.clip.PublicID, &row.normalized, &row.nullRank, &row.stateRank); err != nil {
			return nil, err
		}
		row.createdAt, err = time.Parse(time.RFC3339Nano, row.clip.CreatedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, clipRows.Err()
}

func setNextFolderCursor(page *FolderPage, folderID int64, sort FolderSort, id int64, normalized string) error {
	encoded, err := EncodeFolderCursor(FolderCursor{FolderID: folderID, Sort: sort, Phase: CursorFolders, ID: id, Name: normalized})
	if err == nil {
		page.NextCursor = &encoded
	}
	return err
}

func setNextClipCursor(page *FolderPage, folderID int64, sort FolderSort, row clipPageRow) error {
	cursor := FolderCursor{FolderID: folderID, Sort: sort, Phase: CursorClips, ID: row.clip.ID, CreatedAt: row.createdAt, Name: row.normalized, NullRank: row.nullRank, StateRank: row.stateRank}
	if row.clip.SizeBytes != nil {
		cursor.SizeBytes = *row.clip.SizeBytes
	}
	encoded, err := EncodeFolderCursor(cursor)
	if err == nil {
		page.NextCursor = &encoded
	}
	return err
}

func (s *Store) folderCounts(ctx context.Context, folder *Folder) error {
	return s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM folders WHERE parent_folder_id=? AND deleted_at IS NULL), (SELECT COUNT(*) FROM clips WHERE parent_folder_id=? AND deleted_at IS NULL AND state NOT IN ('uploading','cancelled'))`, folder.ID, folder.ID).Scan(&folder.FolderCount, &folder.ClipCount)
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

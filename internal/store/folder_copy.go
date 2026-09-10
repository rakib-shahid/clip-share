package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

var ErrInvalidCopy = errors.New("folder cannot be copied there")
var ErrCopyNotReady = errors.New("folder contains clips that are not ready")
var ErrCopyChanged = errors.New("folder changed while it was being copied")

type CopyConflict struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

type CopyConflictError struct {
	Conflicts []CopyConflict
}

func (e *CopyConflictError) Error() string { return "folder copy has name conflicts" }

type FolderCopyPlan struct {
	SourceID           int64
	DestinationID      int64
	DestinationOwnerID int64
	Folders            []FolderCopySource
	Clips              []ClipCopySource
}

type FolderCopySource struct {
	ID             int64
	ParentFolderID *int64
	Name           string
	NormalizedName string
	Depth          int
	UpdatedAt      string
}

type ClipCopySource struct {
	ID              int64
	ParentFolderID  int64
	StorageID       string
	Title           string
	NormalizedTitle string
	SizeBytes       *int64
	DurationSeconds *float64
	Width           *int
	Height          *int
	FrameRate       *float64
	UpdatedAt       string
}

type ClipCopyAssignment struct {
	SourceClipID int64
	PublicID     string
	StorageID    string
}

// PrepareFolderCopy returns an immutable description of the source. Media can be
// copied to temporary storage from this plan before the short commit transaction.
func (s *Store) PrepareFolderCopy(ctx context.Context, sourceID, destinationID int64) (FolderCopyPlan, error) {
	return prepareFolderCopy(ctx, s.db, sourceID, destinationID)
}

// CommitFolderCopy rechecks the plan inside a transaction, inserts every new
// record, publishes staged media, and only then commits visibility to readers.
func (s *Store) CommitFolderCopy(ctx context.Context, plan FolderCopyPlan, assignments []ClipCopyAssignment, publish func([]MediaLayoutEntry) error) (Folder, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Folder{}, err
	}
	defer tx.Rollback()

	current, err := prepareFolderCopy(ctx, tx, plan.SourceID, plan.DestinationID)
	if err != nil {
		return Folder{}, err
	}
	if !reflect.DeepEqual(plan, current) {
		return Folder{}, ErrCopyChanged
	}
	assignmentByClip := make(map[int64]ClipCopyAssignment, len(assignments))
	for _, assignment := range assignments {
		if assignment.SourceClipID == 0 || assignment.PublicID == "" || assignment.StorageID == "" {
			return Folder{}, ErrCopyChanged
		}
		assignmentByClip[assignment.SourceClipID] = assignment
	}
	if len(assignmentByClip) != len(plan.Clips) {
		return Folder{}, ErrCopyChanged
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	newFolderIDs := make(map[int64]int64, len(plan.Folders))
	var copiedRootID int64
	for index, source := range plan.Folders {
		parentID := plan.DestinationID
		if index != 0 {
			mapped, ok := newFolderIDs[*source.ParentFolderID]
			if !ok {
				return Folder{}, ErrCopyChanged
			}
			parentID = mapped
		}
		result, err := tx.ExecContext(ctx, `
			INSERT INTO folders(owner_user_id, parent_folder_id, name, name_normalized, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`, plan.DestinationOwnerID, parentID, source.Name, source.NormalizedName, now, now)
		if err != nil {
			return Folder{}, classifyFolderConstraint(err)
		}
		newID, err := result.LastInsertId()
		if err != nil {
			return Folder{}, err
		}
		newFolderIDs[source.ID] = newID
		if index == 0 {
			copiedRootID = newID
		}
	}

	copiedClipIDs := make([]int64, 0, len(plan.Clips))
	for _, source := range plan.Clips {
		assignment, ok := assignmentByClip[source.ID]
		if !ok {
			return Folder{}, ErrCopyChanged
		}
		parentID, ok := newFolderIDs[source.ParentFolderID]
		if !ok {
			return Folder{}, ErrCopyChanged
		}
		result, err := tx.ExecContext(ctx, `
			INSERT INTO clips(owner_user_id, parent_folder_id, public_id, storage_id, title, title_normalized,
			                  state, size_bytes, duration_seconds, width, height, frame_rate, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, 'ready', ?, ?, ?, ?, ?, ?, ?)`,
			plan.DestinationOwnerID, parentID, assignment.PublicID, assignment.StorageID,
			source.Title, source.NormalizedTitle, source.SizeBytes, source.DurationSeconds,
			source.Width, source.Height, source.FrameRate, now, now)
		if err != nil {
			return Folder{}, classifyClipConstraint(err)
		}
		clipID, err := result.LastInsertId()
		if err != nil {
			return Folder{}, err
		}
		copiedClipIDs = append(copiedClipIDs, clipID)
	}
	copied, err := folderByID(ctx, tx, copiedRootID)
	if err != nil {
		return Folder{}, err
	}
	entries := make([]MediaLayoutEntry, 0, len(copiedClipIDs))
	for _, clipID := range copiedClipIDs {
		entry, err := mediaLayoutEntry(ctx, tx, clipID)
		if err != nil {
			return Folder{}, err
		}
		entries = append(entries, entry)
	}
	if err := publish(entries); err != nil {
		return Folder{}, fmt.Errorf("publish copied media: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Folder{}, err
	}
	return copied, nil
}

type folderCopyQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func prepareFolderCopy(ctx context.Context, queryer folderCopyQueryer, sourceID, destinationID int64) (FolderCopyPlan, error) {
	source, err := folderByID(ctx, queryer, sourceID)
	if err != nil {
		return FolderCopyPlan{}, err
	}
	if source.IsRoot {
		return FolderCopyPlan{}, ErrProtectedRoot
	}
	destination, err := folderByID(ctx, queryer, destinationID)
	if err != nil {
		return FolderCopyPlan{}, err
	}

	var destinationInsideSource int
	if err := queryer.QueryRowContext(ctx, `
		WITH RECURSIVE subtree(id) AS (
			SELECT id FROM folders WHERE id = ? AND deleted_at IS NULL
			UNION ALL
			SELECT f.id FROM folders f JOIN subtree ON f.parent_folder_id = subtree.id WHERE f.deleted_at IS NULL
		)
		SELECT COUNT(*) FROM subtree WHERE id = ?`, sourceID, destinationID).Scan(&destinationInsideSource); err != nil {
		return FolderCopyPlan{}, err
	}
	if destinationInsideSource != 0 {
		return FolderCopyPlan{}, ErrInvalidCopy
	}
	destinationDepth, err := folderDepth(ctx, queryer, destinationID)
	if err != nil {
		return FolderCopyPlan{}, err
	}

	plan := FolderCopyPlan{SourceID: sourceID, DestinationID: destinationID, DestinationOwnerID: destination.OwnerUserID, Folders: []FolderCopySource{}, Clips: []ClipCopySource{}}
	rows, err := queryer.QueryContext(ctx, `
		WITH RECURSIVE subtree(id, parent_folder_id, name, name_normalized, depth, updated_at) AS (
			SELECT id, parent_folder_id, name, name_normalized, 0, updated_at
			FROM folders WHERE id = ? AND deleted_at IS NULL
			UNION ALL
			SELECT f.id, f.parent_folder_id, f.name, f.name_normalized, subtree.depth + 1, f.updated_at
			FROM folders f JOIN subtree ON f.parent_folder_id = subtree.id WHERE f.deleted_at IS NULL
		)
		SELECT id, parent_folder_id, name, name_normalized, depth, updated_at
		FROM subtree ORDER BY depth, id`, sourceID)
	if err != nil {
		return FolderCopyPlan{}, err
	}
	for rows.Next() {
		var folder FolderCopySource
		if err := rows.Scan(&folder.ID, &folder.ParentFolderID, &folder.Name, &folder.NormalizedName, &folder.Depth, &folder.UpdatedAt); err != nil {
			rows.Close()
			return FolderCopyPlan{}, err
		}
		plan.Folders = append(plan.Folders, folder)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return FolderCopyPlan{}, err
	}
	if err := rows.Close(); err != nil {
		return FolderCopyPlan{}, err
	}
	if len(plan.Folders) == 0 {
		return FolderCopyPlan{}, ErrFolderNotFound
	}
	maximumDepth := plan.Folders[len(plan.Folders)-1].Depth
	if destinationDepth+1+maximumDepth > 20 {
		return FolderCopyPlan{}, ErrFolderDepth
	}

	var conflict int
	if err := queryer.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM folders
		WHERE parent_folder_id = ? AND name_normalized = ? AND deleted_at IS NULL`,
		destinationID, plan.Folders[0].NormalizedName).Scan(&conflict); err != nil {
		return FolderCopyPlan{}, err
	}
	if conflict != 0 {
		path, err := logicalFolderPath(ctx, queryer, destinationID)
		if err != nil {
			return FolderCopyPlan{}, err
		}
		return FolderCopyPlan{}, &CopyConflictError{Conflicts: []CopyConflict{{Kind: "folder", Path: path + "/" + plan.Folders[0].Name}}}
	}

	rows, err = queryer.QueryContext(ctx, `
		WITH RECURSIVE subtree(id) AS (
			SELECT id FROM folders WHERE id = ? AND deleted_at IS NULL
			UNION ALL
			SELECT f.id FROM folders f JOIN subtree ON f.parent_folder_id = subtree.id WHERE f.deleted_at IS NULL
		)
		SELECT c.id, c.parent_folder_id, c.storage_id, c.title, c.title_normalized, c.state,
		       c.size_bytes, c.duration_seconds, c.width, c.height, c.frame_rate, c.updated_at
		FROM clips c JOIN subtree ON subtree.id = c.parent_folder_id
		WHERE c.deleted_at IS NULL AND c.state != 'cancelled'
		ORDER BY c.parent_folder_id, c.id`, sourceID)
	if err != nil {
		return FolderCopyPlan{}, err
	}
	for rows.Next() {
		var clip ClipCopySource
		var state string
		if err := rows.Scan(&clip.ID, &clip.ParentFolderID, &clip.StorageID, &clip.Title, &clip.NormalizedTitle,
			&state, &clip.SizeBytes, &clip.DurationSeconds, &clip.Width, &clip.Height, &clip.FrameRate, &clip.UpdatedAt); err != nil {
			rows.Close()
			return FolderCopyPlan{}, err
		}
		if state != "ready" {
			rows.Close()
			return FolderCopyPlan{}, ErrCopyNotReady
		}
		plan.Clips = append(plan.Clips, clip)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return FolderCopyPlan{}, err
	}
	if err := rows.Close(); err != nil {
		return FolderCopyPlan{}, err
	}
	return plan, nil
}

func logicalFolderPath(ctx context.Context, queryer folderCopyQueryer, folderID int64) (string, error) {
	var path string
	err := queryer.QueryRowContext(ctx, `
		WITH RECURSIVE paths(id, path) AS (
			SELECT id, name FROM folders WHERE is_root = 1 AND deleted_at IS NULL
			UNION ALL
			SELECT f.id, paths.path || '/' || f.name
			FROM folders f JOIN paths ON f.parent_folder_id = paths.id WHERE f.deleted_at IS NULL
		)
		SELECT path FROM paths WHERE id = ?`, folderID).Scan(&path)
	return strings.TrimSuffix(path, "/"), err
}

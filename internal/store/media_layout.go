package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode"
)

// MediaLayoutEntry describes the human-readable on-disk location for one clip.
// StorageID remains immutable; relative paths are presentation only.
type MediaLayoutEntry struct {
	ClipID      int64
	StorageID   string
	RelativeDir string
}

func (s *Store) MediaLayoutEntries(ctx context.Context) ([]MediaLayoutEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM clips WHERE state = 'ready'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	entries := make([]MediaLayoutEntry, 0, len(ids))
	for _, id := range ids {
		entry, err := s.MediaLayoutEntry(ctx, id)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *Store) MediaLayoutEntry(ctx context.Context, clipID int64) (MediaLayoutEntry, error) {
	return mediaLayoutEntry(ctx, s.db, clipID)
}

type mediaLayoutQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func mediaLayoutEntry(ctx context.Context, queryer mediaLayoutQueryer, clipID int64) (MediaLayoutEntry, error) {
	var entry MediaLayoutEntry
	var ownerID, folderID int64
	var username, title string
	err := queryer.QueryRowContext(ctx, `SELECT c.id,c.storage_id,c.owner_user_id,c.parent_folder_id,u.username,c.title FROM clips c JOIN users u ON u.id=c.owner_user_id WHERE c.id=? AND c.state IN ('processing','ready')`, clipID).Scan(&entry.ClipID, &entry.StorageID, &ownerID, &folderID, &username, &title)
	if err != nil {
		return MediaLayoutEntry{}, err
	}
	rows, err := queryer.QueryContext(ctx, `WITH RECURSIVE ancestors(id,parent_folder_id,name,is_root,depth) AS (SELECT id,parent_folder_id,name,is_root,0 FROM folders WHERE id=? UNION ALL SELECT f.id,f.parent_folder_id,f.name,f.is_root,a.depth+1 FROM folders f JOIN ancestors a ON a.parent_folder_id=f.id) SELECT id,name,is_root FROM ancestors ORDER BY depth DESC`, folderID)
	if err != nil {
		return MediaLayoutEntry{}, err
	}
	defer rows.Close()
	parts := []string{mediaComponent(username, ownerID)}
	for rows.Next() {
		var id int64
		var name string
		var root int
		if err := rows.Scan(&id, &name, &root); err != nil {
			return MediaLayoutEntry{}, err
		}
		if root == 0 {
			parts = append(parts, mediaComponent(name, id))
		}
	}
	if err := rows.Err(); err != nil {
		return MediaLayoutEntry{}, err
	}
	parts = append(parts, mediaClipComponent(title, entry.StorageID))
	entry.RelativeDir = strings.Join(parts, "/")
	return entry, nil
}

func mediaComponent(value string, id int64) string {
	return safeMediaName(value) + "--" + fmt.Sprint(id)
}
func mediaClipComponent(title, storageID string) string {
	return safeMediaName(title) + "--" + storageID
}

func safeMediaName(value string) string {
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
		if b.Len() >= 72 {
			break
		}
	}
	name := strings.Trim(strings.TrimSpace(b.String()), ".")
	if name == "" || strings.EqualFold(name, "con") || strings.EqualFold(name, "prn") || strings.EqualFold(name, "aux") || strings.EqualFold(name, "nul") {
		return "item"
	}
	return name
}

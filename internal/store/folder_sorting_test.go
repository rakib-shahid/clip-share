package store

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

type sortingClip struct {
	id        int64
	title     string
	state     string
	size      *int64
	createdAt time.Time
}

func TestFolderPageKeysetPaginatesEverySort(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "sorting.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	admin, err := db.CreateAdmin(ctx, "Admin", "admin", "hash", 500_000_000)
	if err != nil {
		t.Fatal(err)
	}

	states := []string{"processing", "validating", "queued", "failed", "ready"}
	clips := make([]sortingClip, 0, 75)
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 75; i++ {
		title := fmt.Sprintf("Title %02d", (i*17)%75)
		createdAt := base.Add(time.Duration(i/2) * time.Second)
		var size *int64
		if i%7 != 0 {
			value := int64((i % 11) * 100)
			size = &value
		}
		result, err := db.db.ExecContext(ctx, `INSERT INTO clips(owner_user_id,parent_folder_id,public_id,storage_id,title,title_normalized,state,size_bytes,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			admin.ID, admin.RootFolderID, fmt.Sprintf("public-%02d", i), fmt.Sprintf("storage-%02d", i), title, strings.ToLower(title), states[i%len(states)], size, createdAt.Format(time.RFC3339Nano), createdAt.Format(time.RFC3339Nano))
		if err != nil {
			t.Fatal(err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		clips = append(clips, sortingClip{id: id, title: strings.ToLower(title), state: states[i%len(states)], size: size, createdAt: createdAt})
	}

	for _, mode := range []FolderSort{SortLatest, SortOldest, SortNameAsc, SortNameDesc, SortSizeDesc, SortSizeAsc, SortState} {
		t.Run(string(mode), func(t *testing.T) {
			expected := append([]sortingClip(nil), clips...)
			sort.Slice(expected, func(i, j int) bool { return sortingClipLess(expected[i], expected[j], mode) })
			var cursor *FolderCursor
			var actual []int64
			for {
				page, err := db.FolderPage(ctx, admin.RootFolderID, FolderPageOptions{Sort: mode, Cursor: cursor})
				if err != nil {
					t.Fatal(err)
				}
				if page.TotalFolderCount != 0 || page.TotalClipCount != 75 || page.TotalItemCount != 75 {
					t.Fatalf("unexpected totals: folders=%d clips=%d items=%d", page.TotalFolderCount, page.TotalClipCount, page.TotalItemCount)
				}
				for _, clip := range page.Clips {
					actual = append(actual, clip.ID)
				}
				if page.NextCursor == nil {
					break
				}
				decoded, err := DecodeFolderCursor(*page.NextCursor, admin.RootFolderID, mode)
				if err != nil {
					t.Fatal(err)
				}
				cursor = &decoded
			}
			if len(actual) != len(expected) {
				t.Fatalf("got %d clips, want %d", len(actual), len(expected))
			}
			for i, clip := range expected {
				if actual[i] != clip.id {
					t.Fatalf("position %d: got ID %d, want %d", i, actual[i], clip.id)
				}
			}
		})
	}
}

func sortingClipLess(a, b sortingClip, mode FolderSort) bool {
	cmpID := func(ascending bool) bool {
		if ascending {
			return a.id < b.id
		}
		return a.id > b.id
	}
	switch mode {
	case SortLatest, SortOldest:
		if !a.createdAt.Equal(b.createdAt) {
			if mode == SortOldest {
				return a.createdAt.Before(b.createdAt)
			}
			return a.createdAt.After(b.createdAt)
		}
		return cmpID(mode == SortOldest)
	case SortNameAsc, SortNameDesc:
		if a.title != b.title {
			if mode == SortNameAsc {
				return a.title < b.title
			}
			return a.title > b.title
		}
		return cmpID(mode == SortNameAsc)
	case SortSizeAsc, SortSizeDesc:
		if (a.size == nil) != (b.size == nil) {
			return a.size != nil
		}
		if a.size != nil && *a.size != *b.size {
			if mode == SortSizeAsc {
				return *a.size < *b.size
			}
			return *a.size > *b.size
		}
	case SortState:
		aRank, bRank := testStateRank(a.state), testStateRank(b.state)
		if aRank != bRank {
			return aRank < bRank
		}
	}
	if !a.createdAt.Equal(b.createdAt) {
		return a.createdAt.After(b.createdAt)
	}
	return a.id > b.id
}

func testStateRank(state string) int {
	for rank, value := range []string{"processing", "validating", "queued", "failed", "ready"} {
		if state == value {
			return rank
		}
	}
	return 5
}

func TestFolderPagePreservesExactFolderClipBoundary(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "boundary.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	admin, err := db.CreateAdmin(ctx, "Admin", "admin", "hash", 1_000_000)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < folderPageSize; i++ {
		name := fmt.Sprintf("Folder %02d", i)
		if _, err := db.CreateFolder(ctx, admin.RootFolderID, name, strings.ToLower(name), nil); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.db.ExecContext(ctx, `INSERT INTO clips(owner_user_id,parent_folder_id,public_id,storage_id,title,title_normalized,state,created_at,updated_at) VALUES(?,?,?,?,?,?,'failed',?,?)`, admin.ID, admin.RootFolderID, "boundary-public", "boundary-storage", "Failed", "failed", now, now); err != nil {
		t.Fatal(err)
	}
	first, err := db.FolderPage(ctx, admin.RootFolderID, FolderPageOptions{Sort: SortLatest})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Folders) != folderPageSize || len(first.Clips) != 0 || first.NextCursor == nil {
		t.Fatalf("unexpected first boundary page: folders=%d clips=%d cursor=%v", len(first.Folders), len(first.Clips), first.NextCursor)
	}
	cursor, err := DecodeFolderCursor(*first.NextCursor, admin.RootFolderID, SortLatest)
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.FolderPage(ctx, admin.RootFolderID, FolderPageOptions{Sort: SortLatest, Cursor: &cursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Folders) != 0 || len(second.Clips) != 1 || second.Clips[0].State != "failed" || second.NextCursor != nil {
		t.Fatalf("unexpected second boundary page: %+v", second)
	}
}

func TestFolderPageKeysetSurvivesEarlierMutationAndCountsEveryState(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "mutation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	admin, err := db.CreateAdmin(ctx, "Admin", "admin", "hash", 500_000_000)
	if err != nil {
		t.Fatal(err)
	}
	child, err := db.CreateFolder(ctx, admin.RootFolderID, "Counts", "counts", nil)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 75; i++ {
		stamp := base.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano)
		if _, err := db.db.ExecContext(ctx, `INSERT INTO clips(owner_user_id,parent_folder_id,public_id,storage_id,title,title_normalized,state,created_at,updated_at) VALUES(?,?,?,?,?,?,'ready',?,?)`, admin.ID, admin.RootFolderID, fmt.Sprintf("mutation-public-%02d", i), fmt.Sprintf("mutation-storage-%02d", i), fmt.Sprintf("Clip %02d", i), fmt.Sprintf("clip %02d", i), stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	first, err := db.FolderPage(ctx, admin.RootFolderID, FolderPageOptions{Sort: SortLatest})
	if err != nil || first.NextCursor == nil {
		t.Fatalf("first page cursor=%v err=%v", first.NextCursor, err)
	}
	cursor, err := DecodeFolderCursor(*first.NextCursor, admin.RootFolderID, SortLatest)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := db.FolderPage(ctx, admin.RootFolderID, FolderPageOptions{Sort: SortLatest, Cursor: &cursor})
	if err != nil {
		t.Fatal(err)
	}
	baselineIDs := make([]int64, len(baseline.Clips))
	for i, clip := range baseline.Clips {
		baselineIDs[i] = clip.ID
	}

	newer := base.Add(2 * time.Hour).Format(time.RFC3339Nano)
	if _, err := db.db.ExecContext(ctx, `INSERT INTO clips(owner_user_id,parent_folder_id,public_id,storage_id,title,title_normalized,state,created_at,updated_at) VALUES(?,?,?,?,?,?,'ready',?,?)`, admin.ID, admin.RootFolderID, "mutation-new-public", "mutation-new-storage", "Newer", "newer", newer, newer); err != nil {
		t.Fatal(err)
	}
	deletedAt := base.Add(3 * time.Hour).Format(time.RFC3339Nano)
	if _, err := db.db.ExecContext(ctx, `UPDATE clips SET deleted_at=? WHERE id=?`, deletedAt, first.Clips[0].ID); err != nil {
		t.Fatal(err)
	}
	after, err := db.FolderPage(ctx, admin.RootFolderID, FolderPageOptions{Sort: SortLatest, Cursor: &cursor})
	if err != nil {
		t.Fatal(err)
	}
	afterIDs := make([]int64, len(after.Clips))
	for i, clip := range after.Clips {
		afterIDs[i] = clip.ID
	}
	if fmt.Sprint(afterIDs) != fmt.Sprint(baselineIDs) {
		t.Fatalf("later keyset page shifted: got %v want %v", afterIDs, baselineIDs)
	}
	if after.TotalFolderCount != 1 || after.TotalClipCount != 75 || after.TotalItemCount != 76 {
		t.Fatalf("mutation totals=%d/%d/%d", after.TotalFolderCount, after.TotalClipCount, after.TotalItemCount)
	}

	states := []string{"uploading", "queued", "validating", "processing", "failed", "ready", "cancelled", "unavailable"}
	for i, state := range states {
		stamp := base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339Nano)
		if _, err := db.db.ExecContext(ctx, `INSERT INTO clips(owner_user_id,parent_folder_id,public_id,storage_id,title,title_normalized,state,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, admin.ID, child.ID, "state-public-"+state, "state-storage-"+state, state, state, state, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	deletedReady := base.Add(20 * time.Minute).Format(time.RFC3339Nano)
	if _, err := db.db.ExecContext(ctx, `INSERT INTO clips(owner_user_id,parent_folder_id,public_id,storage_id,title,title_normalized,state,created_at,updated_at,deleted_at) VALUES(?,?,?,?,?,?,'ready',?,?,?)`, admin.ID, child.ID, "deleted-state-public", "deleted-state-storage", "deleted", "deleted", deletedReady, deletedReady, deletedReady); err != nil {
		t.Fatal(err)
	}
	if err := db.folderCounts(ctx, &child); err != nil {
		t.Fatal(err)
	}
	if child.ClipCount != 6 {
		t.Fatalf("visible folder-card clip count=%d, want 6", child.ClipCount)
	}
	childPage, err := db.FolderPage(ctx, child.ID, FolderPageOptions{Sort: SortLatest})
	if err != nil {
		t.Fatal(err)
	}
	if childPage.TotalClipCount != 6 || childPage.TotalItemCount != 6 {
		t.Fatalf("visible child totals=%d/%d", childPage.TotalClipCount, childPage.TotalItemCount)
	}
}

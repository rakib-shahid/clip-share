package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateAdminAndUser(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "database", "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	setup, err := db.IsSetup(ctx)
	if err != nil || setup {
		t.Fatalf("initial setup state: %v %v", setup, err)
	}
	admin, err := db.CreateAdmin(ctx, "Admin", "admin", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if admin.RootFolderID == 0 || admin.Role != "admin" {
		t.Fatalf("unexpected admin: %+v", admin)
	}
	setup, err = db.IsSetup(ctx)
	if err != nil || !setup {
		t.Fatalf("setup state: %v %v", setup, err)
	}
	if _, err := db.CreateAdmin(ctx, "Other", "other", "hash", 50_000_000); !errors.Is(err, ErrAlreadySetup) {
		t.Fatalf("second admin error = %v", err)
	}
	user, err := db.CreateUser(ctx, "Alice", "alice", "hash", 25_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if user.RootFolderID == 0 || user.Role != "user" {
		t.Fatalf("unexpected user: %+v", user)
	}
	if _, err := db.CreateUser(ctx, "ALICE", "alice", "hash", 25_000_000); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("duplicate username error = %v", err)
	}
}

func TestChangeOwnPasswordUsesVerifiedHashCompareAndSwap(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "password.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	user, err := db.CreateAdmin(ctx, "Admin", "admin", "original-hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := db.PasswordHashForActiveUser(ctx, user.ID)
	if err != nil || verified != "original-hash" {
		t.Fatalf("verified hash=%q err=%v", verified, err)
	}
	if err := db.SetUserPassword(ctx, user.ID, "administrator-reset-hash"); err != nil {
		t.Fatal(err)
	}
	if err := db.ChangeOwnPassword(ctx, user.ID, verified, "self-service-hash"); !errors.Is(err, ErrPasswordChanged) {
		t.Fatalf("compare-and-swap error=%v", err)
	}
	hash, err := db.PasswordHashForActiveUser(ctx, user.ID)
	if err != nil || hash != "administrator-reset-hash" {
		t.Fatalf("stored hash=%q err=%v", hash, err)
	}
}

func TestListUserStorageSummariesCountsFinalizedMediaOnly(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "storage-summary.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	admin, err := db.CreateAdmin(ctx, "Admin", "admin", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	alice, err := db.CreateUser(ctx, "Alice", "alice", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	ready := insertReadyClip(t, db, alice.ID, alice.RootFolderID, "ready-public", "ready-storage", "Ready")
	trashed := insertReadyClip(t, db, alice.ID, alice.RootFolderID, "trash-public", "trash-storage", "Trashed")
	if _, err := db.db.ExecContext(ctx, `UPDATE clips SET size_bytes = 1_380_000_000 WHERE id = ?`, ready); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.ExecContext(ctx, `UPDATE clips SET size_bytes = 100_000_000, deleted_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339Nano), trashed); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.db.ExecContext(ctx, `INSERT INTO clips(owner_user_id,parent_folder_id,public_id,storage_id,title,title_normalized,state,size_bytes,created_at,updated_at) VALUES(?,?,?,?,?,?,'processing',?,?,?)`, alice.ID, alice.RootFolderID, "processing-public", "processing-storage", "Processing", "processing", 999_000_000, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ArchiveUser(ctx, alice.ID); err != nil {
		t.Fatal(err)
	}
	summaries, err := db.ListUserStorageSummaries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 2 || summaries[0].ID != admin.ID || summaries[0].StoredBytes != 0 {
		t.Fatalf("admin summary=%+v summaries=%+v", summaries[0], summaries)
	}
	if summaries[1].ID != alice.ID || summaries[1].StoredBytes != 1_480_000_000 {
		t.Fatalf("alice summary=%+v", summaries[1])
	}
}

func TestAccountStateChangesKeepLibraryAndPublicClips(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "database", "users.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	admin, err := db.CreateAdmin(ctx, "Admin", "admin", "admin-hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	alice, err := db.CreateUser(ctx, "Alice", "alice", "old-hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	insertReadyClip(t, db, alice.ID, alice.RootFolderID, "public-user-state", "storage-user-state", "Winner")

	updated, err := db.UpdateUser(ctx, alice.ID, "Alicia", "alicia", 25_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != alice.ID || updated.RootFolderID != alice.RootFolderID || updated.StoredFileLimitBytes != 25_000_000 {
		t.Fatalf("identity changed during update: %+v", updated)
	}
	var rootName string
	if err := db.db.QueryRowContext(ctx, "SELECT name FROM folders WHERE id = ?", alice.RootFolderID).Scan(&rootName); err != nil || rootName != "Alicia" {
		t.Fatalf("root name=%q err=%v", rootName, err)
	}

	for _, change := range []func(context.Context, int64) (User, error){db.DisableUser, db.EnableUser, db.ArchiveUser} {
		if _, err := change(ctx, alice.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.PublicClipByID(ctx, "public-user-state"); err != nil {
			t.Fatalf("public clip unavailable after account state change: %v", err)
		}
	}
	archived, err := db.UserByID(ctx, alice.ID)
	if err != nil || archived.State != "archived" || archived.RootFolderID != alice.RootFolderID {
		t.Fatalf("archived account=%+v err=%v", archived, err)
	}
	restored, err := db.RestoreUser(ctx, alice.ID, "new-hash")
	if err != nil || restored.State != "active" || restored.RootFolderID != alice.RootFolderID {
		t.Fatalf("restored account=%+v err=%v", restored, err)
	}
	if _, err := db.DisableUser(ctx, admin.ID); !errors.Is(err, ErrProtectedAdmin) {
		t.Fatalf("disable admin error=%v", err)
	}
	if _, err := db.ArchiveUser(ctx, admin.ID); !errors.Is(err, ErrProtectedAdmin) {
		t.Fatalf("archive admin error=%v", err)
	}
}

func TestSearchScopeTrashAndResultLimit(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "database", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	admin, err := db.CreateAdmin(ctx, "Admin", "admin", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	alice, err := db.CreateUser(ctx, "Alice", "alice", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	bob, err := db.CreateUser(ctx, "Bob", "bob", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	aliceFolder, err := db.CreateFolder(ctx, alice.RootFolderID, "Game Highlights", "game highlights", nil)
	if err != nil {
		t.Fatal(err)
	}
	bobFolder, err := db.CreateFolder(ctx, bob.RootFolderID, "Game Archive", "game archive", nil)
	if err != nil {
		t.Fatal(err)
	}
	insertReadyClip(t, db, alice.ID, aliceFolder.ID, "search-alice", "search-storage-alice", "Winning Match")
	trashedID := insertReadyClip(t, db, alice.ID, aliceFolder.ID, "search-trash", "search-storage-trash", "Deleted Match")
	insertReadyClip(t, db, bob.ID, bobFolder.ID, "search-bob", "search-storage-bob", "Secret Match")
	if err := db.TrashClip(ctx, trashedID, &alice.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ArchiveUser(ctx, bob.ID); err != nil {
		t.Fatal(err)
	}

	aliceResults, err := db.Search(ctx, "match", &alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(aliceResults.Results) != 1 || aliceResults.Results[0].Name != "Winning Match" || aliceResults.Results[0].OwnerUserID != alice.ID {
		t.Fatalf("alice results = %+v", aliceResults)
	}
	if aliceResults.Results[0].FolderID != aliceFolder.ID || aliceResults.Results[0].Path != "Alice/Game Highlights/Winning Match" {
		t.Fatalf("alice result navigation = %+v", aliceResults.Results[0])
	}

	adminResults, err := db.Search(ctx, "match", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(adminResults.Results) != 2 {
		t.Fatalf("admin match results = %+v", adminResults)
	}
	gameResults, err := db.Search(ctx, "game", nil)
	if err != nil || len(gameResults.Results) != 2 || gameResults.Results[0].Kind != "folder" {
		t.Fatalf("admin folder results = %+v err=%v", gameResults, err)
	}
	empty, err := db.Search(ctx, "", nil)
	if err != nil || len(empty.Results) != 0 || empty.Truncated {
		t.Fatalf("empty results = %+v err=%v", empty, err)
	}

	for index := 0; index < 101; index++ {
		name := fmt.Sprintf("CapMatch %03d", index)
		normalized := strings.ToLower(name)
		if _, err := db.CreateFolder(ctx, admin.RootFolderID, name, normalized, nil); err != nil {
			t.Fatal(err)
		}
	}
	capped, err := db.Search(ctx, "capmatch", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(capped.Results) != 100 || !capped.Truncated {
		t.Fatalf("capped results count=%d truncated=%v", len(capped.Results), capped.Truncated)
	}
}

func TestFolderPageCountsDoNotDeadlockSingleConnection(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "database", "counts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	admin, err := db.CreateAdmin(context.Background(), "Admin", "admin", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	child, err := db.CreateFolder(context.Background(), admin.RootFolderID, "Clips", "clips", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	page, err := db.FolderPage(ctx, admin.RootFolderID, FolderPageOptions{Sort: SortLatest})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Folders) != 1 || page.Folders[0].ID != child.ID || page.Folders[0].FolderCount != 0 || page.Folders[0].ClipCount != 0 {
		t.Fatalf("unexpected counts: %+v", page.Folders)
	}
}

func TestTrashAndRestoreFolderSubtreeControlsPublicClip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "database", "trash.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	admin, err := db.CreateAdmin(ctx, "Admin", "admin", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := db.CreateFolder(ctx, admin.RootFolderID, "Games", "games", nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := db.CreateFolder(ctx, parent.ID, "Finals", "finals", nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := db.db.ExecContext(ctx, `INSERT INTO clips(owner_user_id,parent_folder_id,public_id,storage_id,title,title_normalized,state,size_bytes,created_at,updated_at) VALUES(?,?,?,?,?,?,'ready',?,?,?)`, admin.ID, child.ID, "public-trash-test", "storage-trash-test", "Winner", "winner", 1234, now, now)
	if err != nil {
		t.Fatal(err)
	}
	clipID, _ := result.LastInsertId()
	if err := db.TrashFolder(ctx, parent.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.PublicClipByID(ctx, "public-trash-test"); !errors.Is(err, ErrClipNotFound) {
		t.Fatalf("public lookup while trashed = %v", err)
	}
	items, err := db.ListTrash(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Kind != "folder" || items[0].ID != parent.ID {
		t.Fatalf("trash items = %+v", items)
	}
	if err := db.RestoreFolder(ctx, parent.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.PublicClipByID(ctx, "public-trash-test"); err != nil {
		t.Fatalf("public lookup after restore: %v", err)
	}
	var deletedAt *string
	if err := db.db.QueryRowContext(ctx, `SELECT deleted_at FROM clips WHERE id = ?`, clipID).Scan(&deletedAt); err != nil || deletedAt != nil {
		t.Fatalf("clip deleted_at = %v, err = %v", deletedAt, err)
	}
}

func TestPurgeTrashFolderIsRetrySafeAndDeletesSubtreeRecords(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "database", "purge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	admin, err := db.CreateAdmin(ctx, "Admin", "admin", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := db.CreateFolder(ctx, admin.RootFolderID, "Games", "games", nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := db.CreateFolder(ctx, parent.ID, "Finals", "finals", nil)
	if err != nil {
		t.Fatal(err)
	}
	clipID := insertReadyClip(t, db, admin.ID, child.ID, "public-purge", "storage-purge", "Winner")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	jobResult, err := db.db.ExecContext(ctx, `INSERT INTO jobs(clip_id,state,progress,created_at,updated_at) VALUES(?,'ready',100,?,?)`, clipID, now, now)
	if err != nil {
		t.Fatal(err)
	}
	jobID, _ := jobResult.LastInsertId()
	if _, err := db.db.ExecContext(ctx, `INSERT INTO storage_reservations(job_id,reservation_key,reserved_bytes,created_at) VALUES(?,?,?,?)`, jobID, "purge-reservation", 1, now); err != nil {
		t.Fatal(err)
	}
	if err := db.TrashFolder(ctx, parent.ID, nil); err != nil {
		t.Fatal(err)
	}

	removeErr := errors.New("filesystem unavailable")
	if _, err := db.PurgeTrashItem(ctx, "folder", parent.ID, nil, func(string) error { return removeErr }); !errors.Is(err, removeErr) {
		t.Fatalf("failed purge error = %v", err)
	}
	var retained int
	if err := db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM folders WHERE id = ?`, parent.ID).Scan(&retained); err != nil || retained != 1 {
		t.Fatalf("folder retained after failed purge = %d, err = %v", retained, err)
	}

	var removed []string
	mediaCount, err := db.PurgeTrashItem(ctx, "folder", parent.ID, nil, func(storageID string) error {
		removed = append(removed, storageID)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if mediaCount != 1 || len(removed) != 1 || removed[0] != "storage-purge" {
		t.Fatalf("removed media = %v, count = %d", removed, mediaCount)
	}
	for table, id := range map[string]int64{"folders": parent.ID, "clips": clipID, "jobs": jobID} {
		var count int
		if err := db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE id = ?`, id).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s count = %d, err = %v", table, count, err)
		}
	}
	var reservations int
	if err := db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM storage_reservations WHERE job_id = ?`, jobID).Scan(&reservations); err != nil || reservations != 0 {
		t.Fatalf("reservation count = %d, err = %v", reservations, err)
	}
	if _, err := db.PurgeTrashItem(ctx, "folder", parent.ID, nil, func(string) error { return nil }); !errors.Is(err, ErrTrashNotFound) {
		t.Fatalf("second purge error = %v", err)
	}
}

func TestTrashPreviewAndRetentionWindows(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "database", "retention.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	admin, err := db.CreateAdmin(ctx, "Admin", "admin", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	alice, err := db.CreateUser(ctx, "Alice", "alice", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	clipID := insertReadyClip(t, db, alice.ID, alice.RootFolderID, "public-old", "storage-old", "Old clip")
	if err := db.TrashClip(ctx, clipID, &alice.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.TrashClipForPreview(ctx, clipID, &admin.ID); !errors.Is(err, ErrTrashNotFound) {
		t.Fatalf("cross-owner preview error = %v", err)
	}
	if clip, err := db.TrashClipForPreview(ctx, clipID, &alice.ID); err != nil || clip.StorageID != "storage-old" {
		t.Fatalf("owner preview = %+v, err = %v", clip, err)
	}

	old := time.Now().UTC().Add(-31 * 24 * time.Hour)
	if _, err := db.db.ExecContext(ctx, `UPDATE clips SET deleted_at = ? WHERE id = ?`, old.Format(time.RFC3339Nano), clipID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.TrashClipForPreview(ctx, clipID, &alice.ID); !errors.Is(err, ErrTrashNotFound) {
		t.Fatalf("expired owner preview error = %v", err)
	}
	if clip, err := db.TrashClipForPreview(ctx, clipID, nil); err != nil || clip.StorageID != "storage-old" {
		t.Fatalf("admin preview before purge = %+v, err = %v", clip, err)
	}
	old = time.Now().UTC().Add(-91 * 24 * time.Hour)
	if _, err := db.db.ExecContext(ctx, `UPDATE clips SET deleted_at = ? WHERE id = ?`, old.Format(time.RFC3339Nano), clipID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.TrashClipForPreview(ctx, clipID, nil); !errors.Is(err, ErrTrashNotFound) {
		t.Fatalf("expired admin preview error = %v", err)
	}
	var removed []string
	purged, err := db.PurgeExpiredTrash(ctx, time.Now().UTC().Add(-90*24*time.Hour), func(storageID string) error {
		removed = append(removed, storageID)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if purged != 1 || len(removed) != 1 || removed[0] != "storage-old" {
		t.Fatalf("purged = %d, removed = %v", purged, removed)
	}
}

func insertReadyClip(t *testing.T, db *Store, ownerID, folderID int64, publicID, storageID, title string) int64 {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := db.db.ExecContext(context.Background(), `INSERT INTO clips(owner_user_id,parent_folder_id,public_id,storage_id,title,title_normalized,state,size_bytes,created_at,updated_at) VALUES(?,?,?,?,?,?,'ready',?,?,?)`, ownerID, folderID, publicID, storageID, title, strings.ToLower(title), 1234, now, now)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestFolderHierarchyAndCrossOwnerMove(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "database", "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	admin, err := db.CreateAdmin(ctx, "Admin", "admin", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	alice, err := db.CreateUser(ctx, "Alice", "alice", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}

	games, err := db.CreateFolder(ctx, alice.RootFolderID, "Games", "games", nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := db.CreateFolder(ctx, games.ID, "Highlights", "highlights", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.RenameFolder(ctx, games.ID, "Nope", "nope", &admin.ID); !errors.Is(err, ErrFolderNotFound) {
		t.Fatalf("required owner error = %v", err)
	}
	if _, err := db.CreateFolder(ctx, alice.RootFolderID, "GAMES", "games", nil); !errors.Is(err, ErrFolderNameTaken) {
		t.Fatalf("duplicate folder error = %v", err)
	}
	page, err := db.FolderPage(ctx, child.ID, FolderPageOptions{Sort: SortLatest})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Breadcrumbs) != 3 || page.Breadcrumbs[0].ID != alice.RootFolderID {
		t.Fatalf("breadcrumbs = %+v", page.Breadcrumbs)
	}
	if _, err := db.MoveFolder(ctx, games.ID, child.ID, nil); !errors.Is(err, ErrInvalidMove) {
		t.Fatalf("cycle move error = %v", err)
	}
	moved, err := db.MoveFolder(ctx, games.ID, admin.RootFolderID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if moved.OwnerUserID != admin.ID {
		t.Fatalf("moved owner = %d, want %d", moved.OwnerUserID, admin.ID)
	}
	childPage, err := db.FolderPage(ctx, child.ID, FolderPageOptions{Sort: SortLatest})
	if err != nil {
		t.Fatal(err)
	}
	if childPage.Folder.OwnerUserID != admin.ID {
		t.Fatalf("child owner = %d, want %d", childPage.Folder.OwnerUserID, admin.ID)
	}
	if _, err := db.RenameFolder(ctx, admin.RootFolderID, "Other", "other", nil); !errors.Is(err, ErrProtectedRoot) {
		t.Fatalf("rename root error = %v", err)
	}
}

func TestClipManagementPreservesPublicIdentityAndEnforcesOwnership(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "database", "clips.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	admin, err := db.CreateAdmin(ctx, "Admin", "admin", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	alice, err := db.CreateUser(ctx, "Alice", "alice", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	bob, err := db.CreateUser(ctx, "Bob", "bob", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	games, err := db.CreateFolder(ctx, alice.RootFolderID, "Games", "games", nil)
	if err != nil {
		t.Fatal(err)
	}
	clipID := insertReadyClip(t, db, alice.ID, alice.RootFolderID, "stable-public-id", "stable-storage-id", "Original")
	insertReadyClip(t, db, alice.ID, games.ID, "other-public-id", "other-storage-id", "Taken")
	if err := db.RenameClip(ctx, clipID, "Renamed", "renamed", &bob.ID); !errors.Is(err, ErrClipNotFound) {
		t.Fatalf("other user rename=%v", err)
	}
	if err := db.MoveClip(ctx, clipID, games.ID, &bob.ID); !errors.Is(err, ErrClipNotFound) {
		t.Fatalf("other user move=%v", err)
	}
	if err := db.MoveClip(ctx, clipID, games.ID, &alice.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.RenameClip(ctx, clipID, "Taken", "taken", &alice.ID); !errors.Is(err, ErrClipTitleTaken) {
		t.Fatalf("rename conflict=%v", err)
	}
	if err := db.RenameClip(ctx, clipID, "Highlight", "highlight", &alice.ID); err != nil {
		t.Fatal(err)
	}
	var publicID, storageID, title string
	var ownerID, parentID int64
	if err := db.db.QueryRowContext(ctx, `SELECT public_id, storage_id, title, owner_user_id, parent_folder_id FROM clips WHERE id = ?`, clipID).Scan(&publicID, &storageID, &title, &ownerID, &parentID); err != nil {
		t.Fatal(err)
	}
	if publicID != "stable-public-id" || storageID != "stable-storage-id" || title != "Highlight" || ownerID != alice.ID || parentID != games.ID {
		t.Fatalf("clip changed unexpectedly: public=%q storage=%q title=%q owner=%d parent=%d", publicID, storageID, title, ownerID, parentID)
	}
	if _, err := db.PublicClipByID(ctx, publicID); err != nil {
		t.Fatalf("renamed/moved public clip=%v", err)
	}
	if err := db.MoveClip(ctx, clipID, bob.RootFolderID, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.db.QueryRowContext(ctx, `SELECT owner_user_id, parent_folder_id, public_id, storage_id FROM clips WHERE id = ?`, clipID).Scan(&ownerID, &parentID, &publicID, &storageID); err != nil {
		t.Fatal(err)
	}
	if ownerID != bob.ID || parentID != bob.RootFolderID || publicID != "stable-public-id" || storageID != "stable-storage-id" {
		t.Fatalf("admin move changed identity: owner=%d parent=%d public=%q storage=%q", ownerID, parentID, publicID, storageID)
	}
	if err := db.TrashClip(ctx, clipID, &alice.ID); !errors.Is(err, ErrClipNotFound) {
		t.Fatalf("old owner trash=%v", err)
	}
	if err := db.TrashClip(ctx, clipID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.PublicClipByID(ctx, "stable-public-id"); !errors.Is(err, ErrClipNotFound) {
		t.Fatalf("trashed public clip=%v", err)
	}
	_ = admin
}

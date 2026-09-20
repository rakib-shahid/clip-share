package store

import (
	"context"
	"errors"
	"testing"
)

func TestFolderCopyCreatesIndependentSubtree(t *testing.T) {
	db, admin, alice := copyTestStore(t)
	defer db.Close()
	ctx := context.Background()
	source, err := db.CreateFolder(ctx, alice.RootFolderID, "Games", "games", nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := db.CreateFolder(ctx, source.ID, "Finals", "finals", nil)
	if err != nil {
		t.Fatal(err)
	}
	insertReadyClip(t, db, alice.ID, source.ID, "source-public-one", "source-storage-one", "Opening")
	insertReadyClip(t, db, alice.ID, child.ID, "source-public-two", "source-storage-two", "Winner")

	plan, err := db.PrepareFolderCopy(ctx, source.ID, admin.RootFolderID)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Folders) != 2 || len(plan.Clips) != 2 {
		t.Fatalf("copy plan folders=%d clips=%d", len(plan.Folders), len(plan.Clips))
	}
	assignments := []ClipCopyAssignment{
		{SourceClipID: plan.Clips[0].ID, PublicID: "copied-public-one", StorageID: "copied-storage-one"},
		{SourceClipID: plan.Clips[1].ID, PublicID: "copied-public-two", StorageID: "copied-storage-two"},
	}
	published := false
	copied, err := db.CommitFolderCopy(ctx, plan, assignments, func([]MediaLayoutEntry) error { published = true; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if !published || copied.ID == source.ID || copied.OwnerUserID != admin.ID || copied.Name != source.Name {
		t.Fatalf("copied root=%+v published=%v", copied, published)
	}
	page, err := db.FolderPage(ctx, copied.ID, FolderPageOptions{Sort: SortLatest})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Folders) != 1 || len(page.Clips) != 1 || page.Folders[0].ID == child.ID || page.Clips[0].PublicID == "source-public-one" {
		t.Fatalf("copied page=%+v", page)
	}
	childPage, err := db.FolderPage(ctx, page.Folders[0].ID, FolderPageOptions{Sort: SortLatest})
	if err != nil || len(childPage.Clips) != 1 || childPage.Clips[0].PublicID == "source-public-two" {
		t.Fatalf("copied child=%+v err=%v", childPage, err)
	}
	for _, publicID := range []string{"source-public-one", "source-public-two", "copied-public-one", "copied-public-two"} {
		if _, err := db.PublicClipByID(ctx, publicID); err != nil {
			t.Fatalf("public ID %q unavailable: %v", publicID, err)
		}
	}
	if _, err := db.PrepareFolderCopy(ctx, source.ID, admin.RootFolderID); !isCopyConflict(err) {
		t.Fatalf("repeat copy conflict=%v", err)
	}
}

func TestFolderCopyRejectsInvalidOrChangingSourcesAndRollsBack(t *testing.T) {
	db, admin, alice := copyTestStore(t)
	defer db.Close()
	ctx := context.Background()
	source, err := db.CreateFolder(ctx, alice.RootFolderID, "Source", "source", nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := db.CreateFolder(ctx, source.ID, "Child", "child", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.PrepareFolderCopy(ctx, source.ID, child.ID); !errors.Is(err, ErrInvalidCopy) {
		t.Fatalf("descendant copy error=%v", err)
	}
	if _, err := db.PrepareFolderCopy(ctx, alice.RootFolderID, admin.RootFolderID); !errors.Is(err, ErrProtectedRoot) {
		t.Fatalf("root copy error=%v", err)
	}

	queued, err := db.BeginUpload(ctx, child.ID, "Working", "working", "queued-public", "queued-storage", "queued-reservation", 1, 0, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.PrepareFolderCopy(ctx, source.ID, admin.RootFolderID); !errors.Is(err, ErrCopyNotReady) {
		t.Fatalf("not-ready copy error=%v", err)
	}
	if err := db.AbortUpload(ctx, queued.ClipID, "queued-reservation"); err != nil {
		t.Fatal(err)
	}

	plan, err := db.PrepareFolderCopy(ctx, source.ID, admin.RootFolderID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.RenameFolder(ctx, child.ID, "Renamed", "renamed", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CommitFolderCopy(ctx, plan, nil, func([]MediaLayoutEntry) error { return nil }); !errors.Is(err, ErrCopyChanged) {
		t.Fatalf("changed source error=%v", err)
	}

	rollbackSource, err := db.CreateFolder(ctx, alice.RootFolderID, "Rollback", "rollback", nil)
	if err != nil {
		t.Fatal(err)
	}
	rollbackPlan, err := db.PrepareFolderCopy(ctx, rollbackSource.ID, admin.RootFolderID)
	if err != nil {
		t.Fatal(err)
	}
	publishErr := errors.New("filesystem publish failed")
	if _, err := db.CommitFolderCopy(ctx, rollbackPlan, nil, func([]MediaLayoutEntry) error { return publishErr }); !errors.Is(err, publishErr) {
		t.Fatalf("publish error=%v", err)
	}
	adminPage, err := db.FolderPage(ctx, admin.RootFolderID, FolderPageOptions{Sort: SortLatest})
	if err != nil {
		t.Fatal(err)
	}
	for _, folder := range adminPage.Folders {
		if folder.Name == "Rollback" {
			t.Fatal("failed copy left a visible folder")
		}
	}
}

func copyTestStore(t *testing.T) (*Store, User, User) {
	t.Helper()
	db, err := Open(t.TempDir() + "/copy.db")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := db.CreateAdmin(context.Background(), "Admin", "admin", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	alice, err := db.CreateUser(context.Background(), "Alice", "alice", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	return db, admin, alice
}

func isCopyConflict(err error) bool {
	var conflict *CopyConflictError
	return errors.As(err, &conflict) && len(conflict.Conflicts) > 0
}

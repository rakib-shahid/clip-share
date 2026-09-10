package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestFolderDeletionSummaryCountsRecursiveActiveContents(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "summary.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	admin, err := database.CreateAdmin(ctx, "Admin", "admin", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	alice, err := database.CreateUser(ctx, "Alice", "alice", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := database.CreateFolder(ctx, alice.RootFolderID, "Games", "games", nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := database.CreateFolder(ctx, parent.ID, "Finals", "finals", nil)
	if err != nil {
		t.Fatal(err)
	}
	grandchild, err := database.CreateFolder(ctx, child.ID, "Round One", "round one", nil)
	if err != nil {
		t.Fatal(err)
	}
	insertReadyClip(t, database, alice.ID, parent.ID, "summary-parent", "summary-parent-storage", "Parent clip")
	insertReadyClip(t, database, alice.ID, grandchild.ID, "summary-nested", "summary-nested-storage", "Nested clip")
	trashedID := insertReadyClip(t, database, alice.ID, child.ID, "summary-trashed", "summary-trashed-storage", "Trashed clip")
	if err := database.TrashClip(ctx, trashedID, &alice.ID); err != nil {
		t.Fatal(err)
	}
	outside, err := database.CreateFolder(ctx, alice.RootFolderID, "Outside", "outside", nil)
	if err != nil {
		t.Fatal(err)
	}
	insertReadyClip(t, database, alice.ID, outside.ID, "summary-outside", "summary-outside-storage", "Outside clip")

	timedContext, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	summary, err := database.FolderDeletionSummary(timedContext, parent.ID, &alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.FolderCount != 3 || summary.ClipCount != 2 || summary.TotalItems != 5 || summary.StoredBytes != 2468 {
		t.Fatalf("summary = %+v", summary)
	}
	if _, err := database.FolderDeletionSummary(ctx, parent.ID, &admin.ID); !errors.Is(err, ErrFolderNotFound) {
		t.Fatalf("cross-owner summary error = %v", err)
	}
	if _, err := database.FolderDeletionSummary(ctx, alice.RootFolderID, &alice.ID); !errors.Is(err, ErrProtectedRoot) {
		t.Fatalf("root summary error = %v", err)
	}

	adminSummary, err := database.FolderDeletionSummary(ctx, parent.ID, nil)
	if err != nil || adminSummary != summary {
		t.Fatalf("admin summary = %+v, err = %v", adminSummary, err)
	}
}

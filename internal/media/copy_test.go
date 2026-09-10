package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"clip-share/internal/store"
)

func TestStageAndPublishStoredAssetCopies(t *testing.T) {
	dataDir := t.TempDir()
	for _, area := range []string{"media", "temporary"} {
		if err := os.Mkdir(filepath.Join(dataDir, area), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(dataDir, "media", "source-id")
	if err := os.Mkdir(source, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "video.mp4"), []byte("video-data"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "poster.jpg"), []byte("poster"), 0o640); err != nil {
		t.Fatal(err)
	}
	copies := []StoredAssetCopy{{Source: store.MediaLayoutEntry{StorageID: "source-id", RelativeDir: "source-id"}, DestinationStorageID: "new-id"}}
	size, err := StoredAssetCopiesSize(dataDir, copies)
	if err != nil || size != uint64(len("video-data")+len("poster")) {
		t.Fatalf("size=%d err=%v", size, err)
	}
	if err := StageStoredAssetCopies(context.Background(), dataDir, "request-id", copies); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "media", "new-id")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged media became visible early: %v", err)
	}
	destinations := []store.MediaLayoutEntry{{StorageID: "new-id", RelativeDir: "admin--1/Copied clip--new-id"}}
	if err := PublishStagedCopies(dataDir, "request-id", copies, destinations); err != nil {
		t.Fatal(err)
	}
	video, err := os.ReadFile(filepath.Join(dataDir, "media", "admin--1", "Copied clip--new-id", "video.mp4"))
	if err != nil || string(video) != "video-data" {
		t.Fatalf("copied video=%q err=%v", video, err)
	}
	if err := CleanupStagedCopy(dataDir, "request-id"); err != nil {
		t.Fatal(err)
	}
}

func TestStoredAssetCopyRequiresBothAssets(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "media", "source-id"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "media", "source-id", "video.mp4"), []byte("video"), 0o640); err != nil {
		t.Fatal(err)
	}
	copies := []StoredAssetCopy{{Source: store.MediaLayoutEntry{StorageID: "source-id", RelativeDir: "source-id"}, DestinationStorageID: "new-id"}}
	if _, err := StoredAssetCopiesSize(dataDir, copies); !errors.Is(err, ErrStoredAssetsMissing) {
		t.Fatalf("missing poster error=%v", err)
	}
}

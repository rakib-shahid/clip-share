package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"clip-share/internal/config"
	"clip-share/internal/store"
)

func TestPublicSharingContract(t *testing.T) {
	dataDir := t.TempDir()
	db, err := store.Open(filepath.Join(dataDir, "database", "public.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := config.Config{BaseURL: "http://example.test", DataDir: dataDir, SessionSecret: []byte("01234567890123456789012345678901"), SetupToken: "setup-token"}
	handler := New(cfg, db, slog.New(slog.NewTextHandler(io.Discard, nil)), http.NotFoundHandler())
	setup := performJSON(t, handler, http.MethodPost, "/api/setup", map[string]any{"setupToken": cfg.SetupToken, "username": "Admin", "password": "password123"}, "", "")
	admin := decodeTestSession(t, setup)
	adminCookie := setup.Result().Cookies()[0].Value
	clipID := insertPublicContractClip(t, db, admin.User.ID, admin.User.RootFolderID, "public-contract", "public-storage", "<Title & test>")
	mediaDir := filepath.Join(dataDir, "media", "public-storage")
	if err := os.MkdirAll(mediaDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "video.mp4"), []byte("0123456789"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "poster.jpg"), []byte("poster"), 0o640); err != nil {
		t.Fatal(err)
	}

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/c/public-contract", nil))
	body := page.Body.String()
	if page.Code != http.StatusOK || !strings.Contains(body, "&lt;Title &amp; test&gt;") || !strings.Contains(body, `preload="metadata"`) || strings.Contains(body, "owner_user_id") || page.Header().Get("Cache-Control") != "no-store" || page.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatalf("public page contract status=%d headers=%v body=%s", page.Code, page.Header(), body)
	}

	media := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/m/public-contract/video", nil)
	request.Header.Set("Range", "bytes=2-5")
	handler.ServeHTTP(media, request)
	if media.Code != http.StatusPartialContent || media.Body.String() != "2345" || media.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("range contract status=%d body=%q headers=%v", media.Code, media.Body.String(), media.Header())
	}

	unauthenticated := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/api/folders/"+strconv.FormatInt(admin.User.RootFolderID, 10), nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous folder status=%d", unauthenticated.Code)
	}
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/c/not-a-real-id", nil))
	if missing.Code != http.StatusNotFound || missing.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatalf("missing public status=%d headers=%v", missing.Code, missing.Header())
	}
	if err := db.TrashClip(context.Background(), clipID, nil); err != nil {
		t.Fatal(err)
	}
	deleted := httptest.NewRecorder()
	handler.ServeHTTP(deleted, httptest.NewRequest(http.MethodGet, "/c/public-contract", nil))
	if deleted.Code != http.StatusNotFound {
		t.Fatalf("deleted public status=%d", deleted.Code)
	}
	_ = adminCookie
}

func insertPublicContractClip(t *testing.T, db *store.Store, ownerID, folderID int64, publicID, storageID, title string) int64 {
	t.Helper()
	_ = ownerID
	record, err := db.BeginUpload(context.Background(), folderID, title, strings.ToLower(title), publicID, storageID, "reservation-"+publicID, 10, 0, 1_000_000_000, nil)
	if err != nil {
		t.Fatal(err)
	}
	job, err := db.CompleteUpload(context.Background(), record, "reservation-"+publicID, 10, store.ProbeMetadata{Container: "mp4", VideoCodec: "h264", DurationSeconds: 1, Width: 320, Height: 180, FrameRate: 30}, store.UploadOptions{StoredLimitBypassed: true, TargetSizeBytes: 50_000_000, QualityCRF: 24, MaxHeight: 1080})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := db.ClaimNextJob(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.CompleteProcessing(context.Background(), claimed, 10, store.ProbeMetadata{Container: "mp4", VideoCodec: "h264", DurationSeconds: 1, Width: 320, Height: 180, FrameRate: 30}); err != nil {
		t.Fatal(err)
	}
	return job.ClipID
}

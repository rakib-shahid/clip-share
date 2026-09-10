package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"clip-share/internal/config"
	"clip-share/internal/media"
	"clip-share/internal/store"
)

func TestRestoreTrashClipReconcilesReadableMedia(t *testing.T) {
	dataDir := t.TempDir()
	databasePath := filepath.Join(dataDir, "database", "restore-layout.db")
	database, err := store.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cfg := config.Config{BaseURL: "http://example.test", DataDir: dataDir, SessionSecret: []byte("01234567890123456789012345678901"), SetupToken: "01234567890123456789012345678901"}
	handler := New(cfg, database, slog.New(slog.NewTextHandler(io.Discard, nil)), http.NotFoundHandler())

	setup := performJSON(t, handler, http.MethodPost, "/api/setup", map[string]any{"setupToken": cfg.SetupToken, "username": "Admin", "password": "password123"}, "", "")
	admin := decodeTestSession(t, setup)
	adminCookie := setup.Result().Cookies()[0].Value
	now := time.Now().UTC().Format(time.RFC3339Nano)
	raw, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	result, err := raw.Exec(`INSERT INTO clips(owner_user_id,parent_folder_id,public_id,storage_id,title,title_normalized,state,size_bytes,created_at,updated_at,deleted_at,original_parent_folder_id) VALUES(?,?,?,?,?,?,'ready',?,?,?,?,?)`, admin.User.ID, admin.User.RootFolderID, "restored-public-id", "restored-storage-id", "Restored clip", "restored clip", 5, now, now, now, admin.User.RootFolderID)
	if err != nil {
		t.Fatal(err)
	}
	clipID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	legacyDirectory := filepath.Join(dataDir, "media", "restored-storage-id")
	if err := os.MkdirAll(legacyDirectory, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"video.mp4": "video", "poster.jpg": "poster"} {
		if err := os.WriteFile(filepath.Join(legacyDirectory, name), []byte(content), 0o640); err != nil {
			t.Fatal(err)
		}
	}

	restored := performJSON(t, handler, http.MethodPost, "/api/trash/clip/"+strconv.FormatInt(clipID, 10)+"/restore", nil, adminCookie, admin.CSRFToken)
	if restored.Code != http.StatusNoContent {
		t.Fatalf("restore status=%d body=%s", restored.Code, restored.Body.String())
	}
	entry, err := database.MediaLayoutEntry(context.Background(), clipID)
	if err != nil {
		t.Fatal(err)
	}
	directory, err := media.AssetDirectory(dataDir, entry)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(directory) == filepath.Clean(legacyDirectory) {
		t.Fatal("restored media remained in the legacy flat path")
	}
	publicVideo := httptest.NewRecorder()
	handler.ServeHTTP(publicVideo, httptest.NewRequest(http.MethodGet, "/m/restored-public-id/video", nil))
	if publicVideo.Code != http.StatusOK || publicVideo.Body.String() != "video" {
		t.Fatalf("public media status=%d body=%q", publicVideo.Code, publicVideo.Body.String())
	}
}

func TestTrashPreviewAndAdminPurge(t *testing.T) {
	dataDir := t.TempDir()
	databasePath := filepath.Join(dataDir, "database", "trash-api.db")
	database, err := store.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cfg := config.Config{BaseURL: "http://example.test", DataDir: dataDir, SessionSecret: []byte("01234567890123456789012345678901"), SetupToken: "01234567890123456789012345678901"}
	handler := New(cfg, database, slog.New(slog.NewTextHandler(io.Discard, nil)), http.NotFoundHandler())

	setup := performJSON(t, handler, http.MethodPost, "/api/setup", map[string]any{"setupToken": cfg.SetupToken, "username": "Admin", "password": "password123"}, "", "")
	adminCookie := setup.Result().Cookies()[0]
	var adminSession struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(setup.Body.Bytes(), &adminSession); err != nil {
		t.Fatal(err)
	}
	created := performJSON(t, handler, http.MethodPost, "/api/users", map[string]any{"username": "Alice", "password": "password456", "storedFileLimitMb": 50}, adminCookie.Value, adminSession.CSRFToken)
	var alice store.User
	if err := json.Unmarshal(created.Body.Bytes(), &alice); err != nil {
		t.Fatal(err)
	}
	aliceLogin := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "alice", "password": "password456"}, "", "")
	aliceCookie := aliceLogin.Result().Cookies()[0]
	var aliceSession struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(aliceLogin.Body.Bytes(), &aliceSession); err != nil {
		t.Fatal(err)
	}

	raw, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := raw.Exec(`INSERT INTO clips(owner_user_id,parent_folder_id,public_id,storage_id,title,title_normalized,state,size_bytes,created_at,updated_at,deleted_at,original_parent_folder_id) VALUES(?,?,?,?,?,?,'ready',?,?,?,?,?)`, alice.ID, alice.RootFolderID, "trashed-public-id", "trashed-storage-id", "Trashed clip", "trashed clip", 5, now, now, now, alice.RootFolderID)
	if err != nil {
		t.Fatal(err)
	}
	clipID, _ := result.LastInsertId()
	mediaDir := filepath.Join(dataDir, "media", "trashed-storage-id")
	if err := os.MkdirAll(mediaDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "video.mp4"), []byte("video"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "poster.jpg"), []byte("poster"), 0o640); err != nil {
		t.Fatal(err)
	}

	previewRequest := httptest.NewRequest(http.MethodGet, "/api/trash/clip/"+strconv.FormatInt(clipID, 10)+"/video", nil)
	previewRequest.AddCookie(aliceCookie)
	previewResponse := httptest.NewRecorder()
	handler.ServeHTTP(previewResponse, previewRequest)
	if previewResponse.Code != http.StatusOK || previewResponse.Body.String() != "video" || previewResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("preview status=%d cache=%q body=%q", previewResponse.Code, previewResponse.Header().Get("Cache-Control"), previewResponse.Body.String())
	}

	publicRequest := httptest.NewRequest(http.MethodGet, "/c/trashed-public-id", nil)
	publicResponse := httptest.NewRecorder()
	handler.ServeHTTP(publicResponse, publicRequest)
	if publicResponse.Code != http.StatusNotFound {
		t.Fatalf("trashed public status=%d", publicResponse.Code)
	}

	userPurge := performJSON(t, handler, http.MethodDelete, "/api/trash/clip/"+strconv.FormatInt(clipID, 10), nil, aliceCookie.Value, aliceSession.CSRFToken)
	if userPurge.Code != http.StatusNotFound {
		t.Fatalf("normal user purge status=%d", userPurge.Code)
	}
	adminPurge := performJSON(t, handler, http.MethodDelete, "/api/trash/clip/"+strconv.FormatInt(clipID, 10), nil, adminCookie.Value, adminSession.CSRFToken)
	if adminPurge.Code != http.StatusNoContent {
		t.Fatalf("admin purge status=%d body=%s", adminPurge.Code, adminPurge.Body.String())
	}
	if _, err := os.Stat(mediaDir); !os.IsNotExist(err) {
		t.Fatalf("media directory still exists, err=%v", err)
	}
	var clips int
	if err := raw.QueryRow(`SELECT COUNT(*) FROM clips WHERE id = ?`, clipID).Scan(&clips); err != nil || clips != 0 {
		t.Fatalf("clip count=%d err=%v", clips, err)
	}
}

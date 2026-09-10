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
	"strings"
	"testing"
	"time"

	"clip-share/internal/config"
	"clip-share/internal/media"
	"clip-share/internal/store"
	_ "modernc.org/sqlite"
)

func TestAdminFolderCopyAPIAndMediaIndependence(t *testing.T) {
	dataDir := t.TempDir()
	for _, area := range []string{"media", "temporary"} {
		if err := os.MkdirAll(filepath.Join(dataDir, area), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	databasePath := filepath.Join(dataDir, "database", "copy-api.db")
	database, err := store.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cfg := config.Config{BaseURL: "http://example.test", DataDir: dataDir, SessionSecret: []byte("01234567890123456789012345678901"), SetupToken: "01234567890123456789012345678901"}
	handler := newHandler(cfg, database, slog.New(slog.NewTextHandler(io.Discard, nil)), http.NotFoundHandler(), nil, func(string) (uint64, error) { return 20_000_000_000, nil })

	setup := performJSON(t, handler, http.MethodPost, "/api/setup", map[string]any{"setupToken": cfg.SetupToken, "username": "Admin", "password": "password123"}, "", "")
	admin := decodeTestSession(t, setup)
	adminCookie := setup.Result().Cookies()[0].Value
	createdUser := performJSON(t, handler, http.MethodPost, "/api/users", map[string]any{"username": "Alice", "password": "password456", "storedFileLimitMb": 50}, adminCookie, admin.CSRFToken)
	var alice store.User
	if err := json.Unmarshal(createdUser.Body.Bytes(), &alice); err != nil {
		t.Fatal(err)
	}
	sourceResponse := performJSON(t, handler, http.MethodPost, "/api/folders", map[string]any{"parentFolderId": alice.RootFolderID, "name": "Games"}, adminCookie, admin.CSRFToken)
	var source store.Folder
	if err := json.Unmarshal(sourceResponse.Body.Bytes(), &source); err != nil {
		t.Fatal(err)
	}
	childResponse := performJSON(t, handler, http.MethodPost, "/api/folders", map[string]any{"parentFolderId": source.ID, "name": "Finals"}, adminCookie, admin.CSRFToken)
	var child store.Folder
	if err := json.Unmarshal(childResponse.Body.Bytes(), &child); err != nil {
		t.Fatal(err)
	}

	raw, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := raw.Exec(`INSERT INTO clips(owner_user_id,parent_folder_id,public_id,storage_id,title,title_normalized,state,size_bytes,duration_seconds,width,height,frame_rate,created_at,updated_at) VALUES(?,?,?,?,?,?,'ready',?,?,?,?,?,?,?)`, alice.ID, child.ID, "original-public", "original-storage", "Winner", "winner", 5, 1.5, 1280, 720, 60, now, now); err != nil {
		t.Fatal(err)
	}
	originalMedia := filepath.Join(dataDir, "media", "original-storage")
	if err := os.Mkdir(originalMedia, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(originalMedia, "video.mp4"), []byte("independent-video"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(originalMedia, "poster.jpg"), []byte("independent-poster"), 0o640); err != nil {
		t.Fatal(err)
	}

	aliceLogin := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "alice", "password": "password456"}, "", "")
	aliceSession := decodeTestSession(t, aliceLogin)
	aliceCookie := aliceLogin.Result().Cookies()[0].Value
	copyPath := "/api/folders/" + strconv.FormatInt(source.ID, 10) + "/copy"
	userCopy := performJSON(t, handler, http.MethodPost, copyPath, map[string]any{"destinationFolderId": admin.User.RootFolderID}, aliceCookie, aliceSession.CSRFToken)
	if userCopy.Code != http.StatusNotFound {
		t.Fatalf("normal user copy status=%d body=%s", userCopy.Code, userCopy.Body.String())
	}

	adminCopy := performJSON(t, handler, http.MethodPost, copyPath, map[string]any{"destinationFolderId": admin.User.RootFolderID}, adminCookie, admin.CSRFToken)
	if adminCopy.Code != http.StatusCreated {
		t.Fatalf("admin copy status=%d body=%s", adminCopy.Code, adminCopy.Body.String())
	}
	var copied store.Folder
	if err := json.Unmarshal(adminCopy.Body.Bytes(), &copied); err != nil {
		t.Fatal(err)
	}
	if copied.ID == source.ID || copied.OwnerUserID != admin.User.ID || copied.Name != "Games" {
		t.Fatalf("copied folder=%+v", copied)
	}

	var copiedID int64
	var copiedPublicID, copiedStorageID string
	if err := raw.QueryRow(`WITH RECURSIVE subtree(id) AS (SELECT ? UNION ALL SELECT f.id FROM folders f JOIN subtree ON f.parent_folder_id=subtree.id) SELECT c.id,c.public_id,c.storage_id FROM clips c WHERE c.parent_folder_id IN (SELECT id FROM subtree)`, copied.ID).Scan(&copiedID, &copiedPublicID, &copiedStorageID); err != nil {
		t.Fatal(err)
	}
	if copiedPublicID == "original-public" || copiedStorageID == "original-storage" {
		t.Fatalf("copy reused identity public=%q storage=%q", copiedPublicID, copiedStorageID)
	}
	entry, err := database.MediaLayoutEntry(context.Background(), copiedID)
	if err != nil {
		t.Fatal(err)
	}
	copiedDirectory, err := media.AssetDirectory(dataDir, entry)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(copiedDirectory) == filepath.Join(dataDir, "media", copiedStorageID) {
		t.Fatal("copied media was published into the legacy flat path")
	}
	copiedVideo, err := os.ReadFile(filepath.Join(copiedDirectory, "video.mp4"))
	if err != nil || string(copiedVideo) != "independent-video" {
		t.Fatalf("copied video=%q err=%v", copiedVideo, err)
	}
	publicVideo := httptest.NewRequest(http.MethodGet, "/m/"+copiedPublicID+"/video", nil)
	publicResponse := httptest.NewRecorder()
	handler.ServeHTTP(publicResponse, publicVideo)
	if publicResponse.Code != http.StatusOK || publicResponse.Body.String() != "independent-video" {
		t.Fatalf("copied public media status=%d body=%q", publicResponse.Code, publicResponse.Body.String())
	}

	repeat := performJSON(t, handler, http.MethodPost, copyPath, map[string]any{"destinationFolderId": admin.User.RootFolderID}, adminCookie, admin.CSRFToken)
	if repeat.Code != http.StatusConflict || !strings.Contains(repeat.Body.String(), "copy_conflict") || !strings.Contains(repeat.Body.String(), "Admin/Games") {
		t.Fatalf("repeat copy status=%d body=%s", repeat.Code, repeat.Body.String())
	}
	if original, err := os.ReadFile(filepath.Join(originalMedia, "video.mp4")); err != nil || string(original) != "independent-video" {
		t.Fatalf("original media changed=%q err=%v", original, err)
	}
}

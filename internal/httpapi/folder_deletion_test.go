package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"

	"clip-share/internal/config"
	"clip-share/internal/store"
)

func TestFolderDeletionSummaryAuthorizationAndRecursiveCounts(t *testing.T) {
	database, err := store.Open(filepath.Join(t.TempDir(), "summary.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cfg := config.Config{BaseURL: "http://example.test", SessionSecret: []byte("01234567890123456789012345678901"), SetupToken: "01234567890123456789012345678901"}
	handler := New(cfg, database, slog.New(slog.NewTextHandler(io.Discard, nil)), http.NotFoundHandler())

	setup := performJSON(t, handler, http.MethodPost, "/api/setup", map[string]any{"setupToken": cfg.SetupToken, "username": "Admin", "password": "password123"}, "", "")
	adminCookie := setup.Result().Cookies()[0]
	var adminSession struct {
		User      store.User `json:"user"`
		CSRFToken string     `json:"csrfToken"`
	}
	if err := json.Unmarshal(setup.Body.Bytes(), &adminSession); err != nil {
		t.Fatal(err)
	}

	createAlice := performJSON(t, handler, http.MethodPost, "/api/users", map[string]any{"username": "Alice", "password": "password456", "storedFileLimitMb": 50}, adminCookie.Value, adminSession.CSRFToken)
	var alice store.User
	if err := json.Unmarshal(createAlice.Body.Bytes(), &alice); err != nil {
		t.Fatal(err)
	}
	createBob := performJSON(t, handler, http.MethodPost, "/api/users", map[string]any{"username": "Bob", "password": "password789", "storedFileLimitMb": 50}, adminCookie.Value, adminSession.CSRFToken)
	if createBob.Code != http.StatusCreated {
		t.Fatalf("create Bob status=%d body=%s", createBob.Code, createBob.Body.String())
	}

	aliceLogin := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "alice", "password": "password456"}, "", "")
	aliceCookie := aliceLogin.Result().Cookies()[0]
	var aliceSession struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(aliceLogin.Body.Bytes(), &aliceSession); err != nil {
		t.Fatal(err)
	}
	parentResponse := performJSON(t, handler, http.MethodPost, "/api/folders", map[string]any{"parentFolderId": alice.RootFolderID, "name": "Games"}, aliceCookie.Value, aliceSession.CSRFToken)
	var parent store.Folder
	if err := json.Unmarshal(parentResponse.Body.Bytes(), &parent); err != nil {
		t.Fatal(err)
	}
	childResponse := performJSON(t, handler, http.MethodPost, "/api/folders", map[string]any{"parentFolderId": parent.ID, "name": "Finals"}, aliceCookie.Value, aliceSession.CSRFToken)
	if childResponse.Code != http.StatusCreated {
		t.Fatalf("create child status=%d body=%s", childResponse.Code, childResponse.Body.String())
	}

	path := "/api/folders/" + strconv.FormatInt(parent.ID, 10) + "/deletion-summary"
	aliceSummary := performJSON(t, handler, http.MethodGet, path, nil, aliceCookie.Value, "")
	if aliceSummary.Code != http.StatusOK {
		t.Fatalf("Alice summary status=%d body=%s", aliceSummary.Code, aliceSummary.Body.String())
	}
	var summary store.FolderDeletionSummary
	if err := json.Unmarshal(aliceSummary.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.FolderCount != 2 || summary.ClipCount != 0 || summary.TotalItems != 2 || summary.StoredBytes != 0 {
		t.Fatalf("summary = %+v", summary)
	}

	adminSummary := performJSON(t, handler, http.MethodGet, path, nil, adminCookie.Value, "")
	if adminSummary.Code != http.StatusOK {
		t.Fatalf("admin summary status=%d body=%s", adminSummary.Code, adminSummary.Body.String())
	}
	bobLogin := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "bob", "password": "password789"}, "", "")
	bobSummary := performJSON(t, handler, http.MethodGet, path, nil, bobLogin.Result().Cookies()[0].Value, "")
	if bobSummary.Code != http.StatusNotFound {
		t.Fatalf("cross-owner summary status=%d body=%s", bobSummary.Code, bobSummary.Body.String())
	}
	rootSummary := performJSON(t, handler, http.MethodGet, "/api/folders/"+strconv.FormatInt(alice.RootFolderID, 10)+"/deletion-summary", nil, aliceCookie.Value, "")
	if rootSummary.Code != http.StatusConflict {
		t.Fatalf("root summary status=%d body=%s", rootSummary.Code, rootSummary.Body.String())
	}
}

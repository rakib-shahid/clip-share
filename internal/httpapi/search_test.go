package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"clip-share/internal/config"
	"clip-share/internal/store"
)

func TestSearchAuthorizationAndValidation(t *testing.T) {
	database, err := store.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cfg := config.Config{BaseURL: "http://example.test", SessionSecret: []byte("01234567890123456789012345678901"), SetupToken: "01234567890123456789012345678901"}
	handler := New(cfg, database, slog.New(slog.NewTextHandler(io.Discard, nil)), http.NotFoundHandler())

	setup := performJSON(t, handler, http.MethodPost, "/api/setup", map[string]any{"setupToken": cfg.SetupToken, "username": "Admin", "password": "password123"}, "", "")
	admin := decodeTestSession(t, setup)
	adminCookie := setup.Result().Cookies()[0].Value
	createAlice := performJSON(t, handler, http.MethodPost, "/api/users", map[string]any{"username": "Alice", "password": "password456", "storedFileLimitMb": 50}, adminCookie, admin.CSRFToken)
	createBob := performJSON(t, handler, http.MethodPost, "/api/users", map[string]any{"username": "Bob", "password": "password789", "storedFileLimitMb": 50}, adminCookie, admin.CSRFToken)
	var alice, bob store.User
	if err := json.Unmarshal(createAlice.Body.Bytes(), &alice); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(createBob.Body.Bytes(), &bob); err != nil {
		t.Fatal(err)
	}
	for _, root := range []int64{alice.RootFolderID, bob.RootFolderID} {
		response := performJSON(t, handler, http.MethodPost, "/api/folders", map[string]any{"parentFolderId": root, "name": "Private Match Folder"}, adminCookie, admin.CSRFToken)
		if response.Code != http.StatusCreated {
			t.Fatalf("create search folder status=%d body=%s", response.Code, response.Body.String())
		}
	}

	login := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "alice", "password": "password456"}, "", "")
	aliceCookie := login.Result().Cookies()[0].Value
	userResults := authenticatedRequest(handler, http.MethodGet, "/api/search?q=MATCH", aliceCookie)
	assertSearchOwners(t, userResults, alice.ID, 1)
	adminResults := authenticatedRequest(handler, http.MethodGet, "/api/search?q=match", adminCookie)
	if adminResults.Code != http.StatusOK {
		t.Fatalf("admin search status=%d body=%s", adminResults.Code, adminResults.Body.String())
	}
	var all struct {
		Results []store.SearchResult `json:"results"`
	}
	if err := json.Unmarshal(adminResults.Body.Bytes(), &all); err != nil || len(all.Results) != 2 {
		t.Fatalf("admin results=%+v err=%v", all.Results, err)
	}
	empty := authenticatedRequest(handler, http.MethodGet, "/api/search?q=++", aliceCookie)
	if empty.Code != http.StatusOK || !strings.Contains(empty.Body.String(), `"results":[]`) {
		t.Fatalf("empty search status=%d body=%s", empty.Code, empty.Body.String())
	}
	tooLong := authenticatedRequest(handler, http.MethodGet, "/api/search?q="+url.QueryEscape(strings.Repeat("x", 201)), aliceCookie)
	if tooLong.Code != http.StatusBadRequest || !strings.Contains(tooLong.Body.String(), "search_query_too_long") {
		t.Fatalf("long search status=%d body=%s", tooLong.Code, tooLong.Body.String())
	}
	unauthenticated := authenticatedRequest(handler, http.MethodGet, "/api/search?q=match", "")
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated search status=%d", unauthenticated.Code)
	}
}

func assertSearchOwners(t *testing.T, response *httptest.ResponseRecorder, ownerID int64, count int) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("search status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Results []store.SearchResult `json:"results"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Results) != count {
		t.Fatalf("results=%+v", body.Results)
	}
	for _, result := range body.Results {
		if result.OwnerUserID != ownerID {
			t.Fatalf("cross-owner result=%+v", result)
		}
	}
}

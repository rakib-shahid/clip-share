package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"clip-share/internal/config"
	"clip-share/internal/store"
)

func TestSetupLoginAndCreateUser(t *testing.T) {
	database, err := store.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cfg := config.Config{BaseURL: "http://example.test", SessionSecret: []byte("01234567890123456789012345678901"), SetupToken: "01234567890123456789012345678901"}
	handler := New(cfg, database, slog.New(slog.NewTextHandler(io.Discard, nil)), http.NotFoundHandler())

	setup := performJSON(t, handler, http.MethodPost, "/api/setup", map[string]any{"setupToken": cfg.SetupToken, "username": "Admin", "password": "password123"}, "", "")
	if setup.Code != http.StatusCreated {
		t.Fatalf("setup status=%d body=%s", setup.Code, setup.Body.String())
	}
	cookies := setup.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("setup cookies=%d", len(cookies))
	}
	var session struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(setup.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}

	create := performJSON(t, handler, http.MethodPost, "/api/users", map[string]any{"username": "Alice", "password": "password456", "storedFileLimitMb": 50}, cookies[0].Value, session.CSRFToken)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}

	duplicateSetup := performJSON(t, handler, http.MethodPost, "/api/setup", map[string]any{"setupToken": cfg.SetupToken, "username": "Other", "password": "password789"}, "", "")
	if duplicateSetup.Code != http.StatusConflict {
		t.Fatalf("duplicate setup status=%d", duplicateSetup.Code)
	}

	login := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "alice", "password": "password456"}, "", "")
	if login.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}

	listAsUser := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	listAsUser.AddCookie(login.Result().Cookies()[0])
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listAsUser)
	if listResponse.Code != http.StatusNotFound {
		t.Fatalf("normal user list status=%d", listResponse.Code)
	}
}

func TestFolderAuthorizationAndMutations(t *testing.T) {
	database, err := store.Open(filepath.Join(t.TempDir(), "db.sqlite"))
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
	createUser := performJSON(t, handler, http.MethodPost, "/api/users", map[string]any{"username": "Alice", "password": "password456", "storedFileLimitMb": 50}, adminCookie.Value, adminSession.CSRFToken)
	var alice store.User
	if err := json.Unmarshal(createUser.Body.Bytes(), &alice); err != nil {
		t.Fatal(err)
	}

	login := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "alice", "password": "password456"}, "", "")
	aliceCookie := login.Result().Cookies()[0]
	var aliceSession struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &aliceSession); err != nil {
		t.Fatal(err)
	}
	created := performJSON(t, handler, http.MethodPost, "/api/folders", map[string]any{"parentFolderId": alice.RootFolderID, "name": "Game Clips"}, aliceCookie.Value, aliceSession.CSRFToken)
	if created.Code != http.StatusCreated {
		t.Fatalf("create folder status=%d body=%s", created.Code, created.Body.String())
	}
	var folder store.Folder
	if err := json.Unmarshal(created.Body.Bytes(), &folder); err != nil {
		t.Fatal(err)
	}

	renamed := performJSON(t, handler, http.MethodPatch, "/api/folders/"+strconv.FormatInt(folder.ID, 10), map[string]any{"name": "Highlights"}, aliceCookie.Value, aliceSession.CSRFToken)
	if renamed.Code != http.StatusOK {
		t.Fatalf("rename status=%d body=%s", renamed.Code, renamed.Body.String())
	}

	adminRoot := httptest.NewRequest(http.MethodGet, "/api/folders/"+strconv.FormatInt(adminSession.User.RootFolderID, 10), nil)
	adminRoot.AddCookie(aliceCookie)
	adminRootResponse := httptest.NewRecorder()
	handler.ServeHTTP(adminRootResponse, adminRoot)
	if adminRootResponse.Code != http.StatusNotFound {
		t.Fatalf("cross-user read status=%d", adminRootResponse.Code)
	}

	moveCrossUser := performJSON(t, handler, http.MethodPost, "/api/folders/"+strconv.FormatInt(folder.ID, 10)+"/move", map[string]any{"destinationFolderId": adminSession.User.RootFolderID}, aliceCookie.Value, aliceSession.CSRFToken)
	if moveCrossUser.Code != http.StatusNotFound {
		t.Fatalf("cross-user move status=%d", moveCrossUser.Code)
	}

	moveAsAdmin := performJSON(t, handler, http.MethodPost, "/api/folders/"+strconv.FormatInt(folder.ID, 10)+"/move", map[string]any{"destinationFolderId": adminSession.User.RootFolderID}, adminCookie.Value, adminSession.CSRFToken)
	if moveAsAdmin.Code != http.StatusOK {
		t.Fatalf("admin move status=%d body=%s", moveAsAdmin.Code, moveAsAdmin.Body.String())
	}
}

func performJSON(t *testing.T, handler http.Handler, method, path string, body any, session, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://example.test")
	if session != "" {
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session})
	}
	if csrf != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

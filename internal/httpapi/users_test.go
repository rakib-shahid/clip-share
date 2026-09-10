package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"clip-share/internal/config"
	"clip-share/internal/store"
)

type testSession struct {
	User      store.User `json:"user"`
	CSRFToken string     `json:"csrfToken"`
}

func TestChangeOwnPassword(t *testing.T) {
	database, err := store.Open(filepath.Join(t.TempDir(), "password.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cfg := config.Config{BaseURL: "http://example.test", SessionSecret: []byte("01234567890123456789012345678901"), SetupToken: "01234567890123456789012345678901"}
	handler := New(cfg, database, slog.New(slog.NewTextHandler(io.Discard, nil)), http.NotFoundHandler())
	setup := performJSON(t, handler, http.MethodPost, "/api/setup", map[string]any{"setupToken": cfg.SetupToken, "username": "Admin", "password": "password123"}, "", "")
	session := decodeTestSession(t, setup)
	cookie := setup.Result().Cookies()[0].Value

	path := "/api/me/password"
	if response := performJSON(t, handler, http.MethodPost, path, map[string]any{"currentPassword": "password123", "newPassword": "replacement456", "confirmPassword": "replacement456"}, "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated change status=%d body=%s", response.Code, response.Body.String())
	}
	for name, test := range map[string]struct {
		body map[string]any
		code int
		want string
	}{
		"missing csrf":  {map[string]any{"currentPassword": "password123", "newPassword": "replacement456", "confirmPassword": "replacement456"}, http.StatusForbidden, "csrf_failed"},
		"wrong current": {map[string]any{"currentPassword": "wrong-password", "newPassword": "replacement456", "confirmPassword": "replacement456"}, http.StatusUnauthorized, "current_password_incorrect"},
		"unchanged":     {map[string]any{"currentPassword": "password123", "newPassword": "password123", "confirmPassword": "password123"}, http.StatusConflict, "new_password_unchanged"},
		"mismatch":      {map[string]any{"currentPassword": "password123", "newPassword": "replacement456", "confirmPassword": "different456"}, http.StatusBadRequest, "password_confirmation_mismatch"},
		"invalid new":   {map[string]any{"currentPassword": "password123", "newPassword": "short", "confirmPassword": "short"}, http.StatusBadRequest, "invalid_password"},
	} {
		csrf := session.CSRFToken
		if name == "missing csrf" {
			csrf = ""
		}
		response := performJSON(t, handler, http.MethodPost, path, test.body, cookie, csrf)
		if response.Code != test.code || !strings.Contains(response.Body.String(), test.want) {
			t.Fatalf("%s status=%d body=%s", name, response.Code, response.Body.String())
		}
	}
	changed := performJSON(t, handler, http.MethodPost, path, map[string]any{"currentPassword": "password123", "newPassword": "replacement456", "confirmPassword": "replacement456"}, cookie, session.CSRFToken)
	if changed.Code != http.StatusNoContent || changed.Body.Len() != 0 {
		t.Fatalf("change status=%d body=%s", changed.Code, changed.Body.String())
	}
	if retained := authenticatedRequest(handler, http.MethodGet, "/api/auth/me", cookie); retained.Code != http.StatusOK {
		t.Fatalf("existing session status=%d body=%s", retained.Code, retained.Body.String())
	}
	if login := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "admin", "password": "password123"}, "", ""); login.Code != http.StatusUnauthorized {
		t.Fatalf("old password login=%d", login.Code)
	}
	if login := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "admin", "password": "replacement456"}, "", ""); login.Code != http.StatusOK {
		t.Fatalf("new password login=%d body=%s", login.Code, login.Body.String())
	}
}

func TestUserAdministrationLifecycle(t *testing.T) {
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
	created := performJSON(t, handler, http.MethodPost, "/api/users", map[string]any{"username": "Alice", "password": "password456", "storedFileLimitMb": 50}, adminCookie, admin.CSRFToken)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var alice store.User
	if err := json.Unmarshal(created.Body.Bytes(), &alice); err != nil {
		t.Fatal(err)
	}

	login := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "alice", "password": "password456"}, "", "")
	aliceSession := decodeTestSession(t, login)
	aliceCookie := login.Result().Cookies()[0].Value
	userPath := "/api/users/" + strconv.FormatInt(alice.ID, 10)
	listedSummary := authenticatedRequest(handler, http.MethodGet, "/api/users", adminCookie)
	if listedSummary.Code != http.StatusOK || !strings.Contains(listedSummary.Body.String(), `"storedBytes":0`) || !strings.Contains(listedSummary.Body.String(), `"storedFileLimitBytes":50000000`) {
		t.Fatalf("admin storage summary status=%d body=%s", listedSummary.Code, listedSummary.Body.String())
	}
	if response := authenticatedRequest(handler, http.MethodGet, "/api/users", aliceCookie); response.Code != http.StatusNotFound {
		t.Fatalf("normal user storage summary status=%d body=%s", response.Code, response.Body.String())
	}

	updated := performJSON(t, handler, http.MethodPatch, userPath, map[string]any{"username": "Alicia", "storedFileLimitMb": 25}, adminCookie, admin.CSRFToken)
	if updated.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", updated.Code, updated.Body.String())
	}
	var updatedUser store.User
	if err := json.Unmarshal(updated.Body.Bytes(), &updatedUser); err != nil {
		t.Fatal(err)
	}
	if updatedUser.Username != "Alicia" || updatedUser.StoredFileLimitBytes != 25_000_000 || updatedUser.RootFolderID != alice.RootFolderID {
		t.Fatalf("unexpected updated user: %+v", updatedUser)
	}
	root := authenticatedRequest(handler, http.MethodGet, "/api/folders/"+strconv.FormatInt(alice.RootFolderID, 10), adminCookie)
	if root.Code != http.StatusOK || !strings.Contains(root.Body.String(), `"name":"Alicia"`) {
		t.Fatalf("renamed root status=%d body=%s", root.Code, root.Body.String())
	}
	if response := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "alice", "password": "password456"}, "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("old username login status=%d", response.Code)
	}

	reset := performJSON(t, handler, http.MethodPost, userPath+"/password", map[string]any{"password": "replacement789"}, adminCookie, admin.CSRFToken)
	if reset.Code != http.StatusNoContent || reset.Body.Len() != 0 {
		t.Fatalf("password reset status=%d body=%s", reset.Code, reset.Body.String())
	}
	if response := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "alicia", "password": "password456"}, "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("old password login status=%d", response.Code)
	}
	if response := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "alicia", "password": "replacement789"}, "", ""); response.Code != http.StatusOK {
		t.Fatalf("replacement password login status=%d body=%s", response.Code, response.Body.String())
	}

	disabled := performJSON(t, handler, http.MethodPost, userPath+"/disable", map[string]any{}, adminCookie, admin.CSRFToken)
	assertUserState(t, disabled, "disabled")
	if response := authenticatedRequest(handler, http.MethodGet, "/api/auth/me", aliceCookie); response.Code != http.StatusUnauthorized {
		t.Fatalf("disabled existing session status=%d body=%s", response.Code, response.Body.String())
	}
	if response := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "alicia", "password": "replacement789"}, "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("disabled login status=%d", response.Code)
	}

	enabled := performJSON(t, handler, http.MethodPost, userPath+"/enable", map[string]any{}, adminCookie, admin.CSRFToken)
	assertUserState(t, enabled, "active")
	login = performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "alicia", "password": "replacement789"}, "", "")
	if login.Code != http.StatusOK {
		t.Fatalf("enabled login status=%d body=%s", login.Code, login.Body.String())
	}

	archived := performJSON(t, handler, http.MethodPost, userPath+"/archive", map[string]any{}, adminCookie, admin.CSRFToken)
	assertUserState(t, archived, "archived")
	if response := authenticatedRequest(handler, http.MethodGet, "/api/auth/me", login.Result().Cookies()[0].Value); response.Code != http.StatusUnauthorized {
		t.Fatalf("archived existing session status=%d", response.Code)
	}
	listed := authenticatedRequest(handler, http.MethodGet, "/api/users", adminCookie)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"state":"archived"`) || !strings.Contains(listed.Body.String(), `"rootFolderId":`+strconv.FormatInt(alice.RootFolderID, 10)) {
		t.Fatalf("archived user missing from admin list: %s", listed.Body.String())
	}

	restored := performJSON(t, handler, http.MethodPost, userPath+"/restore", map[string]any{"password": "restored-password"}, adminCookie, admin.CSRFToken)
	assertUserState(t, restored, "active")
	if response := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "alicia", "password": "restored-password"}, "", ""); response.Code != http.StatusOK {
		t.Fatalf("restored login status=%d body=%s", response.Code, response.Body.String())
	}

	adminPath := "/api/users/" + strconv.FormatInt(admin.User.ID, 10)
	for _, action := range []string{"disable", "archive"} {
		response := performJSON(t, handler, http.MethodPost, adminPath+"/"+action, map[string]any{}, adminCookie, admin.CSRFToken)
		if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "protected_admin") {
			t.Fatalf("admin %s status=%d body=%s", action, response.Code, response.Body.String())
		}
	}

	_ = aliceSession // The first signed session is intentionally reused above.
}

func TestUserAdministrationValidationAndAuthorization(t *testing.T) {
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
	created := performJSON(t, handler, http.MethodPost, "/api/users", map[string]any{"username": "Alice", "password": "password456", "storedFileLimitMb": 50}, adminCookie, admin.CSRFToken)
	var alice store.User
	if err := json.Unmarshal(created.Body.Bytes(), &alice); err != nil {
		t.Fatal(err)
	}
	path := "/api/users/" + strconv.FormatInt(alice.ID, 10)

	for name, test := range map[string]struct {
		response *httptest.ResponseRecorder
		code     int
	}{
		"bad limit":    {performJSON(t, handler, http.MethodPatch, path, map[string]any{"username": "Alice", "storedFileLimitMb": 501}, adminCookie, admin.CSRFToken), http.StatusBadRequest},
		"bad password": {performJSON(t, handler, http.MethodPost, path+"/password", map[string]any{"password": "short"}, adminCookie, admin.CSRFToken), http.StatusBadRequest},
		"missing user": {performJSON(t, handler, http.MethodPost, "/api/users/99999/disable", map[string]any{}, adminCookie, admin.CSRFToken), http.StatusNotFound},
	} {
		if test.response.Code != test.code {
			t.Fatalf("%s status=%d body=%s", name, test.response.Code, test.response.Body.String())
		}
	}

	login := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "alice", "password": "password456"}, "", "")
	user := decodeTestSession(t, login)
	response := performJSON(t, handler, http.MethodPatch, path, map[string]any{"username": "Alicia", "storedFileLimitMb": 25}, login.Result().Cookies()[0].Value, user.CSRFToken)
	if response.Code != http.StatusNotFound {
		t.Fatalf("normal user update status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestArchivedLibraryDeletionCanRestoreThenPurge(t *testing.T) {
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
	created := performJSON(t, handler, http.MethodPost, "/api/users", map[string]any{"username": "Alice", "password": "password456", "storedFileLimitMb": 50}, adminCookie, admin.CSRFToken)
	var alice store.User
	if err := json.Unmarshal(created.Body.Bytes(), &alice); err != nil {
		t.Fatal(err)
	}
	path := "/api/users/" + strconv.FormatInt(alice.ID, 10)
	if response := performJSON(t, handler, http.MethodDelete, path+"/library", nil, adminCookie, admin.CSRFToken); response.Code != http.StatusConflict {
		t.Fatalf("active library delete status=%d", response.Code)
	}
	if response := performJSON(t, handler, http.MethodPost, path+"/archive", map[string]any{}, adminCookie, admin.CSRFToken); response.Code != http.StatusOK {
		t.Fatalf("archive=%d %s", response.Code, response.Body.String())
	}
	deleted := performJSON(t, handler, http.MethodDelete, path+"/library", nil, adminCookie, admin.CSRFToken)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete library=%d %s", deleted.Code, deleted.Body.String())
	}
	if response := authenticatedRequest(handler, http.MethodGet, "/api/folders/"+strconv.FormatInt(alice.RootFolderID, 10), adminCookie); response.Code != http.StatusNotFound {
		t.Fatalf("trashed root status=%d", response.Code)
	}
	listed := authenticatedRequest(handler, http.MethodGet, "/api/users", adminCookie)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"libraryTrashed":true`) {
		t.Fatalf("library state=%s", listed.Body.String())
	}
	if response := performJSON(t, handler, http.MethodPost, path+"/restore", map[string]any{"password": "newpassword123"}, adminCookie, admin.CSRFToken); response.Code != http.StatusConflict {
		t.Fatalf("restore while trashed=%d %s", response.Code, response.Body.String())
	}
	if response := performJSON(t, handler, http.MethodPost, "/api/trash/folder/"+strconv.FormatInt(alice.RootFolderID, 10)+"/restore", map[string]any{}, adminCookie, admin.CSRFToken); response.Code != http.StatusNoContent {
		t.Fatalf("restore root=%d %s", response.Code, response.Body.String())
	}
	if response := performJSON(t, handler, http.MethodPost, path+"/restore", map[string]any{"password": "newpassword123"}, adminCookie, admin.CSRFToken); response.Code != http.StatusOK {
		t.Fatalf("restore user=%d %s", response.Code, response.Body.String())
	}
	if response := performJSON(t, handler, http.MethodPost, path+"/archive", map[string]any{}, adminCookie, admin.CSRFToken); response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	if response := performJSON(t, handler, http.MethodDelete, path+"/library", nil, adminCookie, admin.CSRFToken); response.Code != http.StatusNoContent {
		t.Fatal(response.Body.String())
	}
	if _, err := database.PurgeTrashItem(context.Background(), "folder", alice.RootFolderID, nil, func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := database.UserByID(context.Background(), alice.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("purged user err=%v", err)
	}
}

func decodeTestSession(t *testing.T, response *httptest.ResponseRecorder) testSession {
	t.Helper()
	if response.Code < 200 || response.Code >= 300 {
		t.Fatalf("session response status=%d body=%s", response.Code, response.Body.String())
	}
	var session testSession
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	return session
}

func authenticatedRequest(handler http.Handler, method, path, session string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertUserState(t *testing.T, response *httptest.ResponseRecorder, want string) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("state change status=%d body=%s", response.Code, response.Body.String())
	}
	var user store.User
	if err := json.Unmarshal(response.Body.Bytes(), &user); err != nil {
		t.Fatal(err)
	}
	if user.State != want {
		t.Fatalf("state=%q want=%q", user.State, want)
	}
}

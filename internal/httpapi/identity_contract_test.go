package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"clip-share/internal/config"
	"clip-share/internal/store"
)

func TestIdentityAndLogSafetyContract(t *testing.T) {
	database, err := store.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var logs bytes.Buffer
	secret := "setup-secret-value"
	cfg := config.Config{BaseURL: "http://example.test", SessionSecret: []byte("01234567890123456789012345678901"), SetupToken: secret}
	handler := New(cfg, database, slog.New(slog.NewTextHandler(&logs, nil)), http.NotFoundHandler())
	wrong := performJSON(t, handler, http.MethodPost, "/api/setup", map[string]any{"setupToken": "wrong", "username": "Admin", "password": "password123"}, "", "")
	if wrong.Code != http.StatusUnauthorized || !strings.Contains(wrong.Body.String(), "setup_denied") {
		t.Fatalf("wrong setup=%d %s", wrong.Code, wrong.Body.String())
	}
	setup := performJSON(t, handler, http.MethodPost, "/api/setup", map[string]any{"setupToken": secret, "username": "Admin", "password": "password123"}, "", "")
	if setup.Code != http.StatusCreated {
		t.Fatalf("setup=%d %s", setup.Code, setup.Body.String())
	}
	cookie := setup.Result().Cookies()[0]
	if cookie.MaxAge != 0 || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie=%+v", cookie)
	}
	var session struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(setup.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	logout := performJSON(t, handler, http.MethodPost, "/api/auth/logout", map[string]any{}, cookie.Value, session.CSRFToken)
	if logout.Code != http.StatusNoContent || logout.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatalf("logout=%d cookies=%v", logout.Code, logout.Result().Cookies())
	}
	badLogin := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "Admin", "password": "wrong-password"}, "", "")
	if badLogin.Code != http.StatusUnauthorized || !strings.Contains(badLogin.Body.String(), "incorrect") {
		t.Fatalf("bad login=%d %s", badLogin.Code, badLogin.Body.String())
	}
	if strings.Contains(logs.String(), secret) || strings.Contains(logs.String(), "password123") || strings.Contains(logs.String(), cookie.Value) || strings.Contains(logs.String(), session.CSRFToken) {
		t.Fatalf("sensitive value leaked in logs: %s", logs.String())
	}
}

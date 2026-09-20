package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"clip-share/internal/auth"
	"clip-share/internal/config"
	"clip-share/internal/media"
	"clip-share/internal/store"
)

const sessionCookieName = "clip_share_session"

type API struct {
	cfg       config.Config
	store     *store.Store
	logger    *slog.Logger
	sessions  *auth.SessionManager
	web       http.Handler
	probe     media.Prober
	freeBytes func(string) (uint64, error)
}

type contextKey string

const userContextKey contextKey = "user"

func New(cfg config.Config, data *store.Store, logger *slog.Logger, web http.Handler) http.Handler {
	return newHandler(cfg, data, logger, web, media.FFProbe{Command: "ffprobe"}, media.FreeBytes)
}

func newHandler(cfg config.Config, data *store.Store, logger *slog.Logger, web http.Handler, probe media.Prober, freeBytes func(string) (uint64, error)) http.Handler {
	api := &API{cfg: cfg, store: data, logger: logger, sessions: auth.NewSessionManager(cfg.SessionSecret), web: web, probe: probe, freeBytes: freeBytes}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("GET /readyz", api.ready)
	mux.HandleFunc("GET /c/{publicID}", api.publicClipPage)
	mux.HandleFunc("GET /m/{publicID}/video", api.publicMedia)
	mux.HandleFunc("GET /m/{publicID}/poster", api.publicMedia)
	mux.HandleFunc("GET /api/setup/status", api.setupStatus)
	mux.HandleFunc("POST /api/setup", api.setup)
	mux.HandleFunc("POST /api/auth/login", api.login)
	mux.HandleFunc("POST /api/auth/logout", api.logout)
	mux.Handle("GET /api/auth/me", api.requireUser(http.HandlerFunc(api.me)))
	mux.Handle("POST /api/me/password", api.requireUser(http.HandlerFunc(api.changeOwnPassword)))
	mux.Handle("GET /api/users", api.requireAdmin(http.HandlerFunc(api.listUsers)))
	mux.Handle("POST /api/users", api.requireAdmin(http.HandlerFunc(api.createUser)))
	mux.Handle("PATCH /api/users/{userID}", api.requireAdmin(http.HandlerFunc(api.updateUser)))
	mux.Handle("POST /api/users/{userID}/password", api.requireAdmin(http.HandlerFunc(api.resetUserPassword)))
	mux.Handle("POST /api/users/{userID}/disable", api.requireAdmin(http.HandlerFunc(api.disableUser)))
	mux.Handle("POST /api/users/{userID}/enable", api.requireAdmin(http.HandlerFunc(api.enableUser)))
	mux.Handle("POST /api/users/{userID}/archive", api.requireAdmin(http.HandlerFunc(api.archiveUser)))
	mux.Handle("POST /api/users/{userID}/restore", api.requireAdmin(http.HandlerFunc(api.restoreUser)))
	mux.Handle("DELETE /api/users/{userID}/library", api.requireAdmin(http.HandlerFunc(api.deleteArchivedLibrary)))
	mux.Handle("GET /api/folders/{folderID}", api.requireUser(http.HandlerFunc(api.getFolder)))
	mux.Handle("POST /api/folders", api.requireUser(http.HandlerFunc(api.createFolder)))
	mux.Handle("PATCH /api/folders/{folderID}", api.requireUser(http.HandlerFunc(api.renameFolder)))
	mux.Handle("POST /api/folders/{folderID}/move", api.requireUser(http.HandlerFunc(api.moveFolder)))
	mux.Handle("POST /api/folders/{folderID}/copy", api.requireAdmin(http.HandlerFunc(api.copyFolder)))
	mux.Handle("GET /api/folders/{folderID}/deletion-summary", api.requireUser(http.HandlerFunc(api.folderDeletionSummary)))
	mux.Handle("DELETE /api/folders/{folderID}", api.requireUser(http.HandlerFunc(api.trashFolder)))
	mux.Handle("POST /api/uploads", api.requireUser(http.HandlerFunc(api.upload)))
	mux.Handle("GET /api/uploads/resumable", api.requireUser(http.HandlerFunc(api.getResumableEditorSession)))
	mux.Handle("GET /api/uploads/{uploadID}", api.requireUser(http.HandlerFunc(api.getEditorSession)))
	mux.Handle("PUT /api/uploads/{uploadID}/edit", api.requireUser(http.HandlerFunc(api.saveEditorRecipe)))
	mux.Handle("PATCH /api/uploads/{uploadID}/metadata", api.requireUser(http.HandlerFunc(api.updateEditorMetadata)))
	mux.Handle("POST /api/uploads/{uploadID}/heartbeat", api.requireUser(http.HandlerFunc(api.heartbeatEditorSession)))
	mux.Handle("POST /api/uploads/{uploadID}/finalize", api.requireUser(http.HandlerFunc(api.finalizeEditorSession)))
	mux.Handle("POST /api/uploads/{uploadID}/preview", api.requireUser(http.HandlerFunc(api.renderEditorPreview)))
	mux.Handle("GET /api/uploads/{uploadID}/preview", api.requireUser(http.HandlerFunc(api.getEditorPreview)))
	mux.Handle("DELETE /api/uploads/{uploadID}", api.requireUser(http.HandlerFunc(api.discardEditorSession)))
	mux.Handle("GET /api/jobs/statuses", api.requireUser(http.HandlerFunc(api.getJobStatuses)))
	mux.Handle("GET /api/jobs/{jobID}", api.requireUser(http.HandlerFunc(api.getJob)))
	mux.Handle("DELETE /api/jobs/{jobID}", api.requireUser(http.HandlerFunc(api.cancelJob)))
	mux.Handle("DELETE /api/jobs/{jobID}/failure", api.requireUser(http.HandlerFunc(api.dismissFailedJob)))
	mux.Handle("PATCH /api/clips/{clipID}", api.requireUser(http.HandlerFunc(api.renameClip)))
	mux.Handle("DELETE /api/clips/{clipID}", api.requireUser(http.HandlerFunc(api.trashClip)))
	mux.Handle("POST /api/clips/{clipID}/move", api.requireUser(http.HandlerFunc(api.moveClip)))
	mux.Handle("GET /api/trash", api.requireUser(http.HandlerFunc(api.listTrash)))
	mux.Handle("GET /api/search", api.requireUser(http.HandlerFunc(api.search)))
	mux.Handle("GET /api/trash/clip/{id}/{asset}", api.requireUser(http.HandlerFunc(api.previewTrashClip)))
	mux.Handle("POST /api/trash/clip/{id}/restore", api.requireUser(http.HandlerFunc(api.restoreTrashClip)))
	mux.Handle("POST /api/trash/folder/{id}/restore", api.requireUser(http.HandlerFunc(api.restoreTrashFolder)))
	mux.Handle("DELETE /api/trash/{kind}/{id}", api.requireAdmin(http.HandlerFunc(api.purgeTrashItem)))
	mux.HandleFunc("/api/", api.notFound)
	mux.HandleFunc("/app/", api.app)
	mux.HandleFunc("/", api.frontend)
	return api.securityHeaders(api.requestLog(mux))
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.store.Ready(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "The service is not ready.", "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *API) setupStatus(w http.ResponseWriter, r *http.Request) {
	setup, err := a.store.IsSetup(r.Context())
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"setupRequired": !setup})
}

type setupRequest struct {
	SetupToken string `json:"setupToken"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

func (a *API) setup(w http.ResponseWriter, r *http.Request) {
	if !a.validOrigin(r) {
		writeError(w, http.StatusForbidden, "invalid_origin", "The request origin is not allowed.", "")
		return
	}
	var input setupRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.SetupToken != a.cfg.SetupToken {
		writeError(w, http.StatusUnauthorized, "setup_denied", "The setup token is invalid.", "setupToken")
		return
	}
	normalized, err := auth.NormalizeUsername(input.Username)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_username", err.Error(), "username")
		return
	}
	if err := auth.ValidatePassword(input.Password); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_password", err.Error(), "password")
		return
	}
	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	user, err := a.store.CreateAdmin(r.Context(), input.Username, normalized, hash, 50_000_000)
	if errors.Is(err, store.ErrAlreadySetup) {
		writeError(w, http.StatusConflict, "already_setup", "Setup has already been completed.", "")
		return
	}
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	token := sessionCookieValue(a.sessions, user.ID)
	a.writeSessionCookie(w, r, token)
	a.logger.Info("setup_completed", "userId", user.ID)
	writeJSON(w, http.StatusCreated, userResponse(user, a.sessions.CSRF(token), a.cfg.BaseURL))
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	if !a.validOrigin(r) {
		writeError(w, http.StatusForbidden, "invalid_origin", "The request origin is not allowed.", "")
		return
	}
	var input loginRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	normalized, err := auth.NormalizeUsername(input.Username)
	if err != nil {
		a.loginFailed(w)
		return
	}
	user, hash, err := a.store.UserForLogin(r.Context(), normalized)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			a.logger.Error("login_lookup_failed", "error", err)
		}
		a.loginFailed(w)
		return
	}
	valid, err := auth.VerifyPassword(input.Password, hash)
	if err != nil || !valid || user.State != "active" {
		a.loginFailed(w)
		return
	}
	token := sessionCookieValue(a.sessions, user.ID)
	a.writeSessionCookie(w, r, token)
	a.logger.Info("login_succeeded", "userId", user.ID)
	writeJSON(w, http.StatusOK, userResponse(user, a.sessions.CSRF(token), a.cfg.BaseURL))
}

func (a *API) loginFailed(w http.ResponseWriter) {
	a.logger.Info("login_failed")
	writeError(w, http.StatusUnauthorized, "invalid_credentials", "The username or password is incorrect.", "")
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil && !a.validMutation(r, cookie.Value) {
		writeError(w, http.StatusForbidden, "csrf_failed", "The request could not be verified.", "")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", HttpOnly: true, Secure: a.secureCookie(r), SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	cookie, _ := r.Cookie(sessionCookieName)
	writeJSON(w, http.StatusOK, userResponse(user, a.sessions.CSRF(cookie.Value), a.cfg.BaseURL))
}

func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.store.ListUserStorageSummaries(r.Context())
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

type createUserRequest struct {
	Username          string `json:"username"`
	Password          string `json:"password"`
	StoredFileLimitMB int64  `json:"storedFileLimitMb"`
}

func (a *API) createUser(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(sessionCookieName)
	if !a.validMutation(r, cookie.Value) {
		writeError(w, http.StatusForbidden, "csrf_failed", "The request could not be verified.", "")
		return
	}
	var input createUserRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	normalized, err := auth.NormalizeUsername(input.Username)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_username", err.Error(), "username")
		return
	}
	if err := auth.ValidatePassword(input.Password); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_password", err.Error(), "password")
		return
	}
	if input.StoredFileLimitMB < 1 || input.StoredFileLimitMB > 500 {
		writeError(w, http.StatusBadRequest, "invalid_limit", "Stored file limit must be 1-500 MB.", "storedFileLimitMb")
		return
	}
	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	user, err := a.store.CreateUser(r.Context(), input.Username, normalized, hash, input.StoredFileLimitMB*1_000_000)
	if errors.Is(err, store.ErrUsernameTaken) {
		writeError(w, http.StatusConflict, "username_taken", "That username is already in use.", "username")
		return
	}
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	a.logger.Info("user_created", "userId", user.ID, "actorUserId", userFromContext(r.Context()).ID)
	writeJSON(w, http.StatusCreated, user)
}

func (a *API) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := a.authenticate(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication_required", "Please log in.", "")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}

func (a *API) requireAdmin(next http.Handler) http.Handler {
	return a.requireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if userFromContext(r.Context()).Role != "admin" {
			a.notFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func (a *API) authenticate(r *http.Request) (store.User, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return store.User{}, false
	}
	userID, err := a.sessions.Parse(cookie.Value)
	if err != nil {
		return store.User{}, false
	}
	user, err := a.store.UserByID(r.Context(), userID)
	if err != nil || user.State != "active" {
		return store.User{}, false
	}
	return user, true
}

func (a *API) app(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.authenticate(r); !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	a.web.ServeHTTP(w, r)
}

func (a *API) frontend(w http.ResponseWriter, r *http.Request) { a.web.ServeHTTP(w, r) }
func (a *API) notFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "Not found.", "")
}

func (a *API) validOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if a.cfg.AllowedOrigin != "" && origin == strings.TrimRight(a.cfg.AllowedOrigin, "/") {
		return true
	}
	base, err := url.Parse(a.cfg.BaseURL)
	if err != nil {
		return false
	}
	want := base.Scheme + "://" + base.Host
	return origin == want
}

func (a *API) validMutation(r *http.Request, sessionToken string) bool {
	return a.validOrigin(r) && a.sessions.VerifyCSRF(sessionToken, r.Header.Get("X-CSRF-Token"))
}

func (a *API) writeSessionCookie(w http.ResponseWriter, r *http.Request, value string) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: value, Path: "/", HttpOnly: true, Secure: a.secureCookie(r), SameSite: http.SameSiteLaxMode})
}

// The public deployment uses HTTPS and Secure cookies. The explicitly allowed
// LAN origin is HTTP, so its browser session must omit Secure or the browser
// would silently refuse to send it. No other HTTP host receives this exception.
func (a *API) secureCookie(r *http.Request) bool {
	if !a.cfg.SecureCookies || a.cfg.AllowedOrigin == "" {
		return a.cfg.SecureCookies
	}
	allowed, err := url.Parse(strings.TrimRight(a.cfg.AllowedOrigin, "/"))
	if err == nil && allowed.Scheme == "http" && r.TLS == nil && r.Host == allowed.Host {
		return false
	}
	return true
}
func sessionCookieValue(manager *auth.SessionManager, userID int64) string {
	return manager.Create(userID)
}

func userResponse(user store.User, csrf, baseURL string) map[string]any {
	return map[string]any{"user": user, "csrfToken": csrf, "publicBaseURL": strings.TrimRight(baseURL, "/")}
}
func userFromContext(ctx context.Context) store.User {
	user, _ := ctx.Value(userContextKey).(store.User)
	return user
}

func (a *API) internalError(w http.ResponseWriter, r *http.Request, err error) {
	a.logger.Error("request_failed", "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong.", "")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "The request body is invalid.", "")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message, field string) {
	errorValue := map[string]any{"code": code, "message": message}
	if field != "" {
		errorValue["field"] = field
	}
	writeJSON(w, status, map[string]any{"error": errorValue})
}

func (a *API) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; media-src 'self' blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func (a *API) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		a.logger.Info("http_request", "method", r.Method, "path", r.URL.Path, "durationMs", time.Since(started).Milliseconds())
	})
}

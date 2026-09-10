package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"clip-share/internal/config"
	"clip-share/internal/media"
	"clip-share/internal/store"
)

func TestJobControlEndpointsAuthorizeCancelCleanAndDismiss(t *testing.T) {
	dataDir := t.TempDir()
	database, err := store.Open(filepath.Join(dataDir, "database", "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cfg := config.Config{BaseURL: "http://example.test", DataDir: dataDir, SessionSecret: []byte("01234567890123456789012345678901"), SetupToken: "01234567890123456789012345678901"}
	probe := fakeProbe{metadata: store.ProbeMetadata{Container: "mp4", VideoCodec: "h264", DurationSeconds: 2, Width: 640, Height: 360, FrameRate: 30}}
	handler := newHandler(cfg, database, slog.New(slog.NewTextHandler(io.Discard, nil)), http.NotFoundHandler(), probe, func(string) (uint64, error) { return 20_000_000_000, nil })

	setup := performJSON(t, handler, http.MethodPost, "/api/setup", map[string]any{"setupToken": cfg.SetupToken, "username": "Admin", "password": "password123"}, "", "")
	adminCookie := setup.Result().Cookies()[0]
	var adminSession struct {
		CSRFToken string `json:"csrfToken"`
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
		t.Fatalf("create Bob status=%d", createBob.Code)
	}
	aliceLogin := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "alice", "password": "password456"}, "", "")
	aliceCookie := aliceLogin.Result().Cookies()[0]
	var aliceSession struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(aliceLogin.Body.Bytes(), &aliceSession); err != nil {
		t.Fatal(err)
	}
	bobLogin := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "bob", "password": "password789"}, "", "")
	bobCookie := bobLogin.Result().Cookies()[0]
	var bobSession struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(bobLogin.Body.Bytes(), &bobSession); err != nil {
		t.Fatal(err)
	}

	queued := performUpload(t, handler, alice.RootFolderID, "Cancel me", false, []byte("fake video"), aliceCookie, aliceSession.CSRFToken)
	var queuedResponse struct {
		Job store.Job `json:"job"`
	}
	if err := json.Unmarshal(queued.Body.Bytes(), &queuedResponse); err != nil {
		t.Fatal(err)
	}
	queuedResponse.Job = finalizeUploadedEditor(t, handler, queued.Body.Bytes(), aliceCookie, aliceSession.CSRFToken)
	jobPath := "/api/jobs/" + strconv.FormatInt(queuedResponse.Job.ID, 10)
	crossOwner := performJSON(t, handler, http.MethodDelete, jobPath, nil, bobCookie.Value, bobSession.CSRFToken)
	if crossOwner.Code != http.StatusNotFound {
		t.Fatalf("cross-owner cancel status=%d body=%s", crossOwner.Code, crossOwner.Body.String())
	}
	cancelled := performJSON(t, handler, http.MethodDelete, jobPath, nil, aliceCookie.Value, aliceSession.CSRFToken)
	if cancelled.Code != http.StatusNoContent {
		t.Fatalf("cancel status=%d body=%s", cancelled.Code, cancelled.Body.String())
	}
	if retry := performJSON(t, handler, http.MethodDelete, jobPath, nil, aliceCookie.Value, aliceSession.CSRFToken); retry.Code != http.StatusNoContent {
		t.Fatalf("retry cancel status=%d body=%s", retry.Code, retry.Body.String())
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, "temporary"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary entries after cancel=%d", len(entries))
	}
	storedJob, err := database.JobByID(context.Background(), queuedResponse.Job.ID)
	if err != nil || storedJob.State != "cancelled" {
		t.Fatalf("cancelled job=%+v err=%v", storedJob, err)
	}

	failedUpload := performUpload(t, handler, alice.RootFolderID, "Dismiss me", false, []byte("fake video"), aliceCookie, aliceSession.CSRFToken)
	var failedResponse struct {
		Job store.Job `json:"job"`
	}
	if err := json.Unmarshal(failedUpload.Body.Bytes(), &failedResponse); err != nil {
		t.Fatal(err)
	}
	failedResponse.Job = finalizeUploadedEditor(t, handler, failedUpload.Body.Bytes(), aliceCookie, aliceSession.CSRFToken)
	processing, err := database.ClaimNextJob(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := media.RemoveStoredAssets(dataDir, processing.StorageID); err != nil {
		t.Fatal(err)
	}
	if err := database.FailProcessing(context.Background(), processing, "processing_failed", "Could not process video."); err != nil {
		t.Fatal(err)
	}
	if _, err := database.JobByID(context.Background(), failedResponse.Job.ID); !errors.Is(err, store.ErrJobNotFound) {
		t.Fatalf("editor failures must be fully removed, lookup error=%v", err)
	}

	queuedAgain := performUpload(t, handler, alice.RootFolderID, "Still queued", false, []byte("fake video"), aliceCookie, aliceSession.CSRFToken)
	var queuedAgainResponse struct {
		Job store.Job `json:"job"`
	}
	if err := json.Unmarshal(queuedAgain.Body.Bytes(), &queuedAgainResponse); err != nil {
		t.Fatal(err)
	}
	queuedAgainResponse.Job = finalizeUploadedEditor(t, handler, queuedAgain.Body.Bytes(), aliceCookie, aliceSession.CSRFToken)
	notFailedPath := "/api/jobs/" + strconv.FormatInt(queuedAgainResponse.Job.ID, 10) + "/failure"
	if response := performJSON(t, handler, http.MethodDelete, notFailedPath, nil, aliceCookie.Value, aliceSession.CSRFToken); response.Code != http.StatusConflict {
		t.Fatalf("dismiss queued status=%d body=%s", response.Code, response.Body.String())
	}
}

func finalizeUploadedEditor(t *testing.T, handler http.Handler, body []byte, cookie *http.Cookie, csrf string) store.Job {
	t.Helper()
	var editor store.EditorSession
	if err := json.Unmarshal(body, &editor); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/uploads/"+editor.ID+"/finalize", nil)
	request.Header.Set("Origin", "http://example.test")
	request.Header.Set("X-CSRF-Token", csrf)
	request.Header.Set("If-Match", strconv.Itoa(editor.EditRevision))
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("finalize status=%d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		Job store.Job `json:"job"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result.Job
}

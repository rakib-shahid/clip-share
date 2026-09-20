package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
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
)

type fakeProbe struct {
	metadata store.ProbeMetadata
	err      error
}

func (p fakeProbe) Probe(context.Context, string) (store.ProbeMetadata, error) {
	return p.metadata, p.err
}

func TestUploadIntakeCreatesPrivateEditorSession(t *testing.T) {
	dataDir := t.TempDir()
	database, err := store.Open(filepath.Join(dataDir, "database", "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cfg := config.Config{BaseURL: "http://example.test", DataDir: dataDir, SessionSecret: []byte("01234567890123456789012345678901"), SetupToken: "01234567890123456789012345678901"}
	probe := fakeProbe{metadata: store.ProbeMetadata{Container: "mov,mp4", VideoCodec: "h264", AudioCodec: "aac", DurationSeconds: 12.5, Width: 1920, Height: 1080, FrameRate: 60}}
	handler := newHandler(cfg, database, slog.New(slog.NewTextHandler(io.Discard, nil)), http.NotFoundHandler(), probe, func(string) (uint64, error) { return 20_000_000_000, nil })

	setup := performJSON(t, handler, http.MethodPost, "/api/setup", map[string]any{"setupToken": cfg.SetupToken, "username": "Admin", "password": "password123"}, "", "")
	var session struct {
		User      store.User `json:"user"`
		CSRFToken string     `json:"csrfToken"`
	}
	if err := json.Unmarshal(setup.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	cookie := setup.Result().Cookies()[0]

	upload := performUpload(t, handler, session.User.RootFolderID, "First clip", false, []byte("fake video"), cookie, session.CSRFToken)
	if upload.Code != http.StatusCreated {
		t.Fatalf("upload status=%d body=%s", upload.Code, upload.Body.String())
	}
	var response store.EditorSession
	if err := json.Unmarshal(upload.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.State != "editing_session" || response.ID == "" || response.Edit.TrimEndMS != 12500 {
		t.Fatalf("session = %+v", response)
	}

	page, err := database.FolderPage(context.Background(), session.User.RootFolderID, store.FolderPageOptions{Sort: store.SortLatest})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Clips) != 0 {
		t.Fatalf("clips = %+v", page.Clips)
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, "temporary"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary entries=%d err=%v", len(entries), err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "temporary", entries[0].Name(), "source")); err != nil {
		t.Fatal(err)
	}

	editorRequest := httptest.NewRequest(http.MethodGet, "/api/uploads/"+response.ID, nil)
	editorRequest.AddCookie(cookie)
	editorResponse := httptest.NewRecorder()
	handler.ServeHTTP(editorResponse, editorRequest)
	if editorResponse.Code != http.StatusOK {
		t.Fatalf("editor status=%d", editorResponse.Code)
	}
	if policy := editorResponse.Header().Get("Content-Security-Policy"); !strings.Contains(policy, "media-src 'self' blob:") {
		t.Fatalf("editor policy blocks local media blobs: %q", policy)
	}

	pending, err := database.BeginEditorPreview(context.Background(), response.ID, session.User.ID, response.EditRevision, 0, true)
	if err != nil || !pending.PreviewRendering {
		t.Fatalf("begin preview session=%+v err=%v", pending, err)
	}
	if err := database.FinishEditorPreview(context.Background(), response.ID, session.User.ID, response.EditRevision, true); err != nil {
		t.Fatal(err)
	}
	saved, err := database.SaveEditorRecipe(context.Background(), response.ID, session.User.ID, response.EditRevision, store.EditorRecipe{TrimStartMS: 1000, TrimEndMS: 11000})
	if err != nil {
		t.Fatal(err)
	}
	if saved.PreviewState != "ready" || saved.PreviewRevision != response.EditRevision {
		t.Fatalf("saved edit discarded usable preview: %+v", saved)
	}
	if _, err := database.BeginEditorPreview(context.Background(), response.ID, session.User.ID, saved.EditRevision, 5000, false); err != nil {
		t.Fatal(err)
	}
	duringReplacement, err := database.EditorSessionByID(context.Background(), response.ID, session.User.ID)
	if err != nil || !duringReplacement.PreviewRendering || duringReplacement.PreviewState != "ready" || duringReplacement.PreviewRevision != response.EditRevision {
		t.Fatalf("replacement did not preserve ready preview: %+v err=%v", duringReplacement, err)
	}
	recoveredStorage, err := database.RecoverInterruptedEditorPreviews(context.Background())
	if err != nil || len(recoveredStorage) != 1 {
		t.Fatalf("recovered preview storage=%v err=%v", recoveredStorage, err)
	}
	afterFailure, err := database.EditorSessionByID(context.Background(), response.ID, session.User.ID)
	if err != nil || afterFailure.PreviewRendering || afterFailure.PreviewState != "ready" {
		t.Fatalf("failed replacement removed ready preview: %+v err=%v", afterFailure, err)
	}

	resumeRequest := httptest.NewRequest(http.MethodGet, "/api/uploads/resumable?destinationFolderId="+strconv.FormatInt(session.User.RootFolderID, 10)+"&sourceSizeBytes=10&title=FIRST%20CLIP", nil)
	resumeRequest.AddCookie(cookie)
	resumeResponse := httptest.NewRecorder()
	handler.ServeHTTP(resumeResponse, resumeRequest)
	if resumeResponse.Code != http.StatusOK {
		t.Fatalf("resume status=%d body=%s", resumeResponse.Code, resumeResponse.Body.String())
	}
	var resumed store.EditorSession
	if err := json.Unmarshal(resumeResponse.Body.Bytes(), &resumed); err != nil {
		t.Fatal(err)
	}
	if resumed.ID != response.ID || resumed.Edit.TrimStartMS != 1000 || resumed.Edit.TrimEndMS != 11000 {
		t.Fatalf("resumed session=%+v", resumed)
	}

	second := performUpload(t, handler, session.User.RootFolderID, "Second clip", false, []byte("another fake video"), cookie, session.CSRFToken)
	if second.Code != http.StatusCreated {
		t.Fatalf("second concurrent editor status=%d body=%s", second.Code, second.Body.String())
	}

	duplicate := performUpload(t, handler, session.User.RootFolderID, "FIRST CLIP", false, []byte("fake video"), cookie, session.CSRFToken)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status=%d body=%s", duplicate.Code, duplicate.Body.String())
	}
}

func TestUploadFailureCleansTemporaryState(t *testing.T) {
	dataDir := t.TempDir()
	database, err := store.Open(filepath.Join(dataDir, "database", "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cfg := config.Config{BaseURL: "http://example.test", DataDir: dataDir, SessionSecret: []byte("01234567890123456789012345678901"), SetupToken: "01234567890123456789012345678901"}
	handler := newHandler(cfg, database, slog.New(slog.NewTextHandler(io.Discard, nil)), http.NotFoundHandler(), fakeProbe{err: media.ErrUnsupportedMedia}, func(string) (uint64, error) { return 20_000_000_000, nil })
	setup := performJSON(t, handler, http.MethodPost, "/api/setup", map[string]any{"setupToken": cfg.SetupToken, "username": "Admin", "password": "password123"}, "", "")
	var session struct {
		User      store.User `json:"user"`
		CSRFToken string     `json:"csrfToken"`
	}
	if err := json.Unmarshal(setup.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	upload := performUpload(t, handler, session.User.RootFolderID, "Broken", false, []byte("not a video"), setup.Result().Cookies()[0], session.CSRFToken)
	if upload.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", upload.Code, upload.Body.String())
	}
	page, err := database.FolderPage(context.Background(), session.User.RootFolderID, store.FolderPageOptions{Sort: store.SortLatest})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Clips) != 0 {
		t.Fatalf("clips = %+v", page.Clips)
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, "temporary"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary entries = %d", len(entries))
	}
}

func TestUploadRejectsDeclaredOversizeBeforeReading(t *testing.T) {
	dataDir := t.TempDir()
	database, err := store.Open(filepath.Join(dataDir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cfg := config.Config{BaseURL: "http://example.test", DataDir: dataDir, SessionSecret: []byte("01234567890123456789012345678901"), SetupToken: "01234567890123456789012345678901"}
	api := newHandler(cfg, database, slog.New(slog.NewTextHandler(io.Discard, nil)), http.NotFoundHandler(), fakeProbe{}, func(string) (uint64, error) { return 20_000_000_000, nil })
	setup := performJSON(t, api, http.MethodPost, "/api/setup", map[string]any{"setupToken": cfg.SetupToken, "username": "Admin", "password": "password123"}, "", "")
	var session struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(setup.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/uploads", bytes.NewReader(nil))
	request.ContentLength = maximumRequestBytes + 1
	request.Header.Set("Origin", cfg.BaseURL)
	request.AddCookie(setup.Result().Cookies()[0])
	request.Header.Set("X-CSRF-Token", session.CSRFToken)
	response := httptest.NewRecorder()
	api.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestCancelledUploadRequestCleansTemporaryState(t *testing.T) {
	dataDir := t.TempDir()
	database, err := store.Open(filepath.Join(dataDir, "database", "cancel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cfg := config.Config{BaseURL: "http://example.test", DataDir: dataDir, SessionSecret: []byte("01234567890123456789012345678901"), SetupToken: "01234567890123456789012345678901"}
	handler := newHandler(cfg, database, slog.New(slog.NewTextHandler(io.Discard, nil)), http.NotFoundHandler(), fakeProbe{}, func(string) (uint64, error) { return 20_000_000_000, nil })
	setup := performJSON(t, handler, http.MethodPost, "/api/setup", map[string]any{"setupToken": cfg.SetupToken, "username": "Admin", "password": "password123"}, "", "")
	var session struct {
		User      store.User `json:"user"`
		CSRFToken string     `json:"csrfToken"`
	}
	if err := json.Unmarshal(setup.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}

	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	requestContext, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodPost, "/api/uploads", pipeReader).WithContext(requestContext)
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	request.Header.Set("Origin", cfg.BaseURL)
	request.Header.Set("X-CSRF-Token", session.CSRFToken)
	request.AddCookie(setup.Result().Cookies()[0])
	response := httptest.NewRecorder()
	handlerDone := make(chan struct{})
	go func() {
		handler.ServeHTTP(response, request)
		close(handlerDone)
	}()

	videoStarted := make(chan struct{})
	releaseWriter := make(chan struct{})
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		fields := map[string]string{"title": "Cancelled request", "destinationFolderId": strconv.FormatInt(session.User.RootFolderID, 10), "compressionRequested": "false", "qualityCrf": "24", "maxHeight": "1080"}
		for _, name := range []string{"title", "destinationFolderId", "compressionRequested", "qualityCrf", "maxHeight"} {
			if err := multipartWriter.WriteField(name, fields[name]); err != nil {
				return
			}
		}
		part, err := multipartWriter.CreateFormFile("video", "cancel.mp4")
		if err != nil {
			return
		}
		if _, err := part.Write([]byte("partial video bytes")); err != nil {
			return
		}
		close(videoStarted)
		<-releaseWriter
		_ = pipeWriter.CloseWithError(context.Canceled)
	}()

	select {
	case <-videoStarted:
	case <-time.After(time.Second):
		t.Fatal("upload did not begin")
	}
	deadline := time.Now().Add(time.Second)
	for {
		entries, readErr := os.ReadDir(filepath.Join(dataDir, "temporary"))
		if readErr == nil && len(entries) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("uploading reservation was not created")
		}
		time.Sleep(5 * time.Millisecond)
	}

	cancel()
	close(releaseWriter)
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("cancelled upload handler did not return")
	}
	<-writerDone
	page, err := database.FolderPage(context.Background(), session.User.RootFolderID, store.FolderPageOptions{Sort: store.SortLatest})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Clips) != 0 {
		t.Fatalf("cancelled upload clips=%+v", page.Clips)
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, "temporary"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary entries after request cancellation=%d", len(entries))
	}
}

func performUpload(t *testing.T, handler http.Handler, destination int64, title string, compress bool, contents []byte, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{"title": title, "destinationFolderId": strconv.FormatInt(destination, 10), "compressionRequested": strconv.FormatBool(compress), "qualityCrf": "24", "maxHeight": "1080", "editingRequested": "true"}
	for _, name := range []string{"title", "destinationFolderId", "compressionRequested", "qualityCrf", "maxHeight", "editingRequested"} {
		if err := writer.WriteField(name, fields[name]); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("video", "clip.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/uploads", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Origin", "http://example.test")
	request.Header.Set("X-CSRF-Token", csrf)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

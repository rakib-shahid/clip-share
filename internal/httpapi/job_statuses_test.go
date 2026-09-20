package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"clip-share/internal/config"
	"clip-share/internal/store"
)

func TestParseJobIDs(t *testing.T) {
	invalid := [][]string{nil, {}, {""}, {"0"}, {"-1"}, {"+1"}, {"01x"}, {"1.0"}, {"x"}, {"1,1"}, {"1,,2"}, {"1", "2"}, {" 1"}}
	tooMany := "1"
	for id := 2; id <= 101; id++ {
		tooMany += fmt.Sprintf(",%d", id)
	}
	invalid = append(invalid, []string{tooMany})
	for _, values := range invalid {
		if _, ok := parseJobIDs(values); ok {
			t.Fatalf("accepted %#v", values)
		}
	}
	ids, ok := parseJobIDs([]string{"3,1,2"})
	if !ok || len(ids) != 3 || ids[0] != 3 || ids[2] != 2 {
		t.Fatalf("ids=%v ok=%v", ids, ok)
	}
	hundred := "1"
	for id := 2; id <= 100; id++ {
		hundred += fmt.Sprintf(",%d", id)
	}
	if ids, ok := parseJobIDs([]string{hundred}); !ok || len(ids) != 100 {
		t.Fatalf("100 ids=%d ok=%v", len(ids), ok)
	}
}

func TestJobStatusesHTTPAuthorizationOrderAndNulls(t *testing.T) {
	dataDir := t.TempDir()
	database, err := store.Open(filepath.Join(dataDir, "statuses.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cfg := config.Config{BaseURL: "http://example.test", DataDir: dataDir, SessionSecret: []byte("01234567890123456789012345678901"), SetupToken: "01234567890123456789012345678901"}
	handler := newHandler(cfg, database, slog.New(slog.NewTextHandler(io.Discard, nil)), http.NotFoundHandler(), fakeProbe{metadata: store.ProbeMetadata{Container: "mp4", VideoCodec: "h264", DurationSeconds: 2, Width: 640, Height: 360, FrameRate: 30}}, func(string) (uint64, error) { return 20_000_000_000, nil })

	setup := performJSON(t, handler, http.MethodPost, "/api/setup", map[string]any{"setupToken": cfg.SetupToken, "username": "Admin", "password": "password123"}, "", "")
	adminCookie := setup.Result().Cookies()[0]
	var adminSession struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(setup.Body.Bytes(), &adminSession); err != nil {
		t.Fatal(err)
	}
	createAlice := performJSON(t, handler, http.MethodPost, "/api/users", map[string]any{"username": "Alice", "password": "password456", "storedFileLimitMb": 50}, adminCookie.Value, adminSession.CSRFToken)
	createBob := performJSON(t, handler, http.MethodPost, "/api/users", map[string]any{"username": "Bob", "password": "password789", "storedFileLimitMb": 50}, adminCookie.Value, adminSession.CSRFToken)
	var alice, bob store.User
	if err := json.Unmarshal(createAlice.Body.Bytes(), &alice); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(createBob.Body.Bytes(), &bob); err != nil {
		t.Fatal(err)
	}
	aliceLogin := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "alice", "password": "password456"}, "", "")
	bobLogin := performJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"username": "bob", "password": "password789"}, "", "")
	aliceCookie := aliceLogin.Result().Cookies()[0]
	bobCookie := bobLogin.Result().Cookies()[0]
	var aliceAuth, bobAuth struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(aliceLogin.Body.Bytes(), &aliceAuth); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(bobLogin.Body.Bytes(), &bobAuth); err != nil {
		t.Fatal(err)
	}
	aliceUpload := performUpload(t, handler, alice.RootFolderID, "Alice status", false, []byte("video"), aliceCookie, aliceAuth.CSRFToken)
	bobUpload := performUpload(t, handler, bob.RootFolderID, "Bob status", false, []byte("video"), bobCookie, bobAuth.CSRFToken)
	aliceJob := finalizeUploadedEditor(t, handler, aliceUpload.Body.Bytes(), aliceCookie, aliceAuth.CSRFToken)
	bobJob := finalizeUploadedEditor(t, handler, bobUpload.Body.Bytes(), bobCookie, bobAuth.CSRFToken)

	path := "/api/jobs/statuses?ids=" + strconv.FormatInt(bobJob.ID, 10) + "," + strconv.FormatInt(aliceJob.ID, 10) + ",999999"
	adminResponse := performJSON(t, handler, http.MethodGet, path, nil, adminCookie.Value, "")
	if adminResponse.Code != http.StatusOK {
		t.Fatalf("admin status=%d body=%s", adminResponse.Code, adminResponse.Body.String())
	}
	var adminResult struct {
		Jobs []store.JobStatus `json:"jobs"`
	}
	if err := json.Unmarshal(adminResponse.Body.Bytes(), &adminResult); err != nil {
		t.Fatal(err)
	}
	if len(adminResult.Jobs) != 2 || adminResult.Jobs[0].JobID != bobJob.ID || adminResult.Jobs[1].JobID != aliceJob.ID {
		t.Fatalf("admin jobs=%+v", adminResult.Jobs)
	}
	if adminResult.Jobs[0].SizeBytes != nil || adminResult.Jobs[0].ErrorMessage != nil {
		t.Fatalf("nullable fields=%+v", adminResult.Jobs[0])
	}

	aliceResponse := performJSON(t, handler, http.MethodGet, path, nil, aliceCookie.Value, "")
	var aliceResult struct {
		Jobs []store.JobStatus `json:"jobs"`
	}
	if err := json.Unmarshal(aliceResponse.Body.Bytes(), &aliceResult); err != nil {
		t.Fatal(err)
	}
	if aliceResponse.Code != http.StatusOK || len(aliceResult.Jobs) != 1 || aliceResult.Jobs[0].JobID != aliceJob.ID {
		t.Fatalf("alice status=%d jobs=%+v", aliceResponse.Code, aliceResult.Jobs)
	}
	if unauthenticated := performJSON(t, handler, http.MethodGet, path, nil, "", ""); unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated=%d", unauthenticated.Code)
	}
	if invalid := performJSON(t, handler, http.MethodGet, "/api/jobs/statuses?ids=+1", nil, aliceCookie.Value, ""); invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), `"code":"invalid_job_ids"`) || !strings.Contains(invalid.Body.String(), `"field":"ids"`) {
		t.Fatalf("invalid=%d body=%s", invalid.Code, invalid.Body.String())
	}
}

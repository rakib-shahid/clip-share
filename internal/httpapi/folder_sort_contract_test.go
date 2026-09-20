package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"clip-share/internal/config"
	"clip-share/internal/store"
)

func TestFolderSortHTTPContract(t *testing.T) {
	database, err := store.Open(filepath.Join(t.TempDir(), "sort-http.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cfg := config.Config{BaseURL: "http://example.test", SessionSecret: []byte("01234567890123456789012345678901"), SetupToken: "01234567890123456789012345678901"}
	handler := New(cfg, database, slog.New(slog.NewTextHandler(io.Discard, nil)), http.NotFoundHandler())
	setup := performJSON(t, handler, http.MethodPost, "/api/setup", map[string]any{"setupToken": cfg.SetupToken, "username": "Admin", "password": "password123"}, "", "")
	if setup.Code != http.StatusCreated {
		t.Fatalf("setup=%d %s", setup.Code, setup.Body.String())
	}
	cookie := setup.Result().Cookies()[0]
	var session struct {
		User      store.User `json:"user"`
		CSRFToken string     `json:"csrfToken"`
	}
	if err := json.Unmarshal(setup.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 61; i++ {
		response := performJSON(t, handler, http.MethodPost, "/api/folders", map[string]any{"parentFolderId": session.User.RootFolderID, "name": fmt.Sprintf("Folder %02d", i)}, cookie.Value, session.CSRFToken)
		if response.Code != http.StatusCreated {
			t.Fatalf("create folder %d=%d %s", i, response.Code, response.Body.String())
		}
	}

	rootPath := "/api/folders/" + strconv.FormatInt(session.User.RootFolderID, 10)
	for _, mode := range []store.FolderSort{store.SortLatest, store.SortOldest, store.SortNameAsc, store.SortNameDesc, store.SortSizeDesc, store.SortSizeAsc, store.SortState} {
		t.Run(string(mode), func(t *testing.T) {
			path := rootPath + "?sort=" + url.QueryEscape(string(mode))
			first := authenticatedGET(handler, path, cookie)
			if first.Code != http.StatusOK {
				t.Fatalf("first=%d %s", first.Code, first.Body.String())
			}
			var page store.FolderPage
			if err := json.Unmarshal(first.Body.Bytes(), &page); err != nil {
				t.Fatal(err)
			}
			if page.TotalFolderCount != 61 || page.TotalClipCount != 0 || page.TotalItemCount != 61 || len(page.Folders) != 60 || page.NextCursor == nil {
				t.Fatalf("unexpected first page: totals=%d/%d/%d folders=%d cursor=%v", page.TotalFolderCount, page.TotalClipCount, page.TotalItemCount, len(page.Folders), page.NextCursor)
			}
			second := authenticatedGET(handler, path+"&cursor="+url.QueryEscape(*page.NextCursor), cookie)
			if second.Code != http.StatusOK {
				t.Fatalf("second=%d %s", second.Code, second.Body.String())
			}
			var next store.FolderPage
			if err := json.Unmarshal(second.Body.Bytes(), &next); err != nil {
				t.Fatal(err)
			}
			if len(next.Folders) != 1 || next.NextCursor != nil {
				t.Fatalf("unexpected second page: folders=%d cursor=%v", len(next.Folders), next.NextCursor)
			}
			firstName, lastName := page.Folders[0].Name, next.Folders[0].Name
			if mode == store.SortNameDesc {
				if firstName != "Folder 60" || lastName != "Folder 00" {
					t.Fatalf("descending names=%q...%q", firstName, lastName)
				}
			} else if firstName != "Folder 00" || lastName != "Folder 60" {
				t.Fatalf("ascending names=%q...%q", firstName, lastName)
			}
		})
	}

	defaultResponse := authenticatedGET(handler, rootPath, cookie)
	encodedSort := authenticatedGET(handler, rootPath+"?sort=%6catest", cookie)
	if defaultResponse.Code != http.StatusOK || encodedSort.Code != http.StatusOK {
		t.Fatalf("default=%d encoded=%d", defaultResponse.Code, encodedSort.Code)
	}

	latest := authenticatedGET(handler, rootPath+"?sort=latest", cookie)
	var latestPage store.FolderPage
	if err := json.Unmarshal(latest.Body.Bytes(), &latestPage); err != nil || latestPage.NextCursor == nil {
		t.Fatalf("latest page cursor=%v err=%v", latestPage.NextCursor, err)
	}
	invalidCases := []struct {
		path  string
		code  string
		field string
	}{
		{rootPath + "?sort=LATEST", "invalid_sort", "sort"},
		{rootPath + "?sort=oldest&cursor=" + url.QueryEscape(*latestPage.NextCursor), "invalid_cursor", "cursor"},
		{rootPath + "?cursor=not-a-cursor", "invalid_cursor", "cursor"},
		{rootPath + "?cursor=" + url.QueryEscape(base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"folderId":1,"sort":"latest","phase":"unknown","id":1,"key":{"name":"x"}}`))), "invalid_cursor", "cursor"},
	}
	for _, test := range invalidCases {
		response := authenticatedGET(handler, test.path, cookie)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) || !strings.Contains(response.Body.String(), `"field":"`+test.field+`"`) {
			t.Errorf("%s: status=%d body=%s", test.path, response.Code, response.Body.String())
		}
	}
}

func authenticatedGET(handler http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

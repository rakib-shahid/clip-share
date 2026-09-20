package store

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func TestParseFolderSort(t *testing.T) {
	for _, value := range []string{"", "latest", "oldest", "name_asc", "name_desc", "size_desc", "size_asc", "state"} {
		if _, err := ParseFolderSort(value); err != nil {
			t.Fatalf("%q: %v", value, err)
		}
	}
	for _, value := range []string{"Latest", " latest", "unknown"} {
		if _, err := ParseFolderSort(value); !errors.Is(err, ErrInvalidSort) {
			t.Fatalf("%q accepted", value)
		}
	}
}

func TestFolderCursorRoundTrips(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 123, time.UTC)
	cases := []FolderCursor{
		{FolderID: 4, Sort: SortLatest, Phase: CursorFolders, ID: 8, Name: "folder"},
		{FolderID: 4, Sort: SortLatest, Phase: CursorClips, ID: 8, CreatedAt: now},
		{FolderID: 4, Sort: SortNameDesc, Phase: CursorClips, ID: 8, Name: "title"},
		{FolderID: 4, Sort: SortSizeAsc, Phase: CursorClips, ID: 8, NullRank: 1, SizeBytes: 0, CreatedAt: now},
		{FolderID: 4, Sort: SortState, Phase: CursorClips, ID: 8, StateRank: 3, CreatedAt: now},
	}
	for _, input := range cases {
		encoded, err := EncodeFolderCursor(input)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeFolderCursor(encoded, input.FolderID, input.Sort)
		if err != nil {
			t.Fatal(err)
		}
		if decoded.ID != input.ID || decoded.Phase != input.Phase {
			t.Fatalf("bad round trip: %#v", decoded)
		}
	}
}

func TestFolderCursorRejectsMalformedAndMismatchedValues(t *testing.T) {
	valid, _ := EncodeFolderCursor(FolderCursor{FolderID: 4, Sort: SortLatest, Phase: CursorFolders, ID: 8, Name: "folder"})
	for _, test := range []struct {
		value  string
		folder int64
		sort   FolderSort
	}{{"8", 4, SortLatest}, {base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"folderId":4,"sort":"latest","phase":"folders","id":8,"key":{"name":"x"}} trailing`)), 4, SortLatest}, {valid + "=", 4, SortLatest}, {valid, 5, SortLatest}, {valid, 4, SortOldest}} {
		if _, err := DecodeFolderCursor(test.value, test.folder, test.sort); !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("accepted %q", test.value)
		}
	}
}

func FuzzDecodeFolderCursor(f *testing.F) {
	f.Add("not-a-cursor")
	f.Fuzz(func(t *testing.T, value string) { _, _ = DecodeFolderCursor(value, 1, SortLatest) })
}

package store

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

type FolderSort string

const (
	SortLatest   FolderSort = "latest"
	SortOldest   FolderSort = "oldest"
	SortNameAsc  FolderSort = "name_asc"
	SortNameDesc FolderSort = "name_desc"
	SortSizeDesc FolderSort = "size_desc"
	SortSizeAsc  FolderSort = "size_asc"
	SortState    FolderSort = "state"
)

var ErrInvalidSort = errors.New("invalid sort")
var ErrInvalidCursor = errors.New("invalid cursor")

func ParseFolderSort(value string) (FolderSort, error) {
	if value == "" {
		return SortLatest, nil
	}
	sort := FolderSort(value)
	switch sort {
	case SortLatest, SortOldest, SortNameAsc, SortNameDesc, SortSizeDesc, SortSizeAsc, SortState:
		return sort, nil
	}
	return "", ErrInvalidSort
}

type CursorPhase string

const (
	CursorFolders CursorPhase = "folders"
	CursorClips   CursorPhase = "clips"
)

type FolderCursor struct {
	FolderID  int64
	Sort      FolderSort
	Phase     CursorPhase
	ID        int64
	Name      string
	CreatedAt time.Time
	NullRank  int
	SizeBytes int64
	StateRank int
}

type cursorEnvelope struct {
	Version  int             `json:"v"`
	FolderID int64           `json:"folderId"`
	Sort     FolderSort      `json:"sort"`
	Phase    CursorPhase     `json:"phase"`
	ID       int64           `json:"id"`
	Key      json.RawMessage `json:"key"`
}
type nameKey struct {
	Name string `json:"name"`
}
type timeKey struct {
	CreatedAt string `json:"createdAt"`
}
type sizeKey struct {
	NullRank  int    `json:"nullRank"`
	SizeBytes int64  `json:"sizeBytes"`
	CreatedAt string `json:"createdAt"`
}
type stateKey struct {
	StateRank int    `json:"stateRank"`
	CreatedAt string `json:"createdAt"`
}

func EncodeFolderCursor(cursor FolderCursor) (string, error) {
	if err := validateCursorBase(cursor); err != nil {
		return "", err
	}
	var key any
	if cursor.Phase == CursorFolders {
		key = nameKey{cursor.Name}
	} else {
		switch cursor.Sort {
		case SortLatest, SortOldest:
			key = timeKey{cursor.CreatedAt.UTC().Format(time.RFC3339Nano)}
		case SortNameAsc, SortNameDesc:
			key = nameKey{cursor.Name}
		case SortSizeAsc, SortSizeDesc:
			key = sizeKey{cursor.NullRank, cursor.SizeBytes, cursor.CreatedAt.UTC().Format(time.RFC3339Nano)}
		case SortState:
			key = stateKey{cursor.StateRank, cursor.CreatedAt.UTC().Format(time.RFC3339Nano)}
		}
	}
	rawKey, err := json.Marshal(key)
	if err != nil {
		return "", ErrInvalidCursor
	}
	raw, err := json.Marshal(cursorEnvelope{1, cursor.FolderID, cursor.Sort, cursor.Phase, cursor.ID, rawKey})
	if err != nil {
		return "", ErrInvalidCursor
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeFolderCursor(value string, folderID int64, sort FolderSort) (FolderCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return FolderCursor{}, ErrInvalidCursor
	}
	var envelope cursorEnvelope
	if err := strictJSON(raw, &envelope); err != nil || envelope.Version != 1 || envelope.FolderID != folderID || envelope.Sort != sort {
		return FolderCursor{}, ErrInvalidCursor
	}
	cursor := FolderCursor{FolderID: envelope.FolderID, Sort: envelope.Sort, Phase: envelope.Phase, ID: envelope.ID}
	if cursor.FolderID <= 0 || cursor.ID <= 0 || (cursor.Phase != CursorFolders && cursor.Phase != CursorClips) {
		return FolderCursor{}, ErrInvalidCursor
	}
	if cursor.Phase == CursorFolders {
		var key nameKey
		if strictJSON(envelope.Key, &key) != nil || key.Name == "" {
			return FolderCursor{}, ErrInvalidCursor
		}
		cursor.Name = key.Name
		return cursor, nil
	}
	switch sort {
	case SortLatest, SortOldest:
		var key timeKey
		if strictJSON(envelope.Key, &key) != nil {
			return FolderCursor{}, ErrInvalidCursor
		}
		cursor.CreatedAt, err = parseCursorTime(key.CreatedAt)
	case SortNameAsc, SortNameDesc:
		var key nameKey
		if strictJSON(envelope.Key, &key) != nil || key.Name == "" {
			return FolderCursor{}, ErrInvalidCursor
		}
		cursor.Name = key.Name
	case SortSizeAsc, SortSizeDesc:
		var key sizeKey
		if strictJSON(envelope.Key, &key) != nil || key.NullRank < 0 || key.NullRank > 1 || key.SizeBytes < 0 {
			return FolderCursor{}, ErrInvalidCursor
		}
		cursor.NullRank, cursor.SizeBytes = key.NullRank, key.SizeBytes
		cursor.CreatedAt, err = parseCursorTime(key.CreatedAt)
	case SortState:
		var key stateKey
		if strictJSON(envelope.Key, &key) != nil || key.StateRank < 0 || key.StateRank > 100 {
			return FolderCursor{}, ErrInvalidCursor
		}
		cursor.StateRank = key.StateRank
		cursor.CreatedAt, err = parseCursorTime(key.CreatedAt)
	default:
		return FolderCursor{}, ErrInvalidCursor
	}
	if err != nil {
		return FolderCursor{}, ErrInvalidCursor
	}
	return cursor, nil
}

func validateCursorBase(cursor FolderCursor) error {
	if cursor.FolderID <= 0 || cursor.ID <= 0 {
		return ErrInvalidCursor
	}
	if _, err := ParseFolderSort(string(cursor.Sort)); err != nil || cursor.Sort == "" {
		return ErrInvalidCursor
	}
	if cursor.Phase != CursorFolders && cursor.Phase != CursorClips {
		return ErrInvalidCursor
	}
	if cursor.Phase == CursorFolders && cursor.Name == "" {
		return ErrInvalidCursor
	}
	if cursor.Phase == CursorClips && (cursor.Sort == SortLatest || cursor.Sort == SortOldest || cursor.Sort == SortSizeAsc || cursor.Sort == SortSizeDesc || cursor.Sort == SortState) && cursor.CreatedAt.IsZero() {
		return ErrInvalidCursor
	}
	return nil
}

func strictJSON(raw []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}

func parseCursorTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.Year() < 1 || parsed.Year() > 9999 {
		return time.Time{}, ErrInvalidCursor
	}
	return parsed, nil
}

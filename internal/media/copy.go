package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"clip-share/internal/store"
)

var ErrStoredAssetsMissing = errors.New("stored clip assets are missing")

type StoredAssetCopy struct {
	Source               store.MediaLayoutEntry
	DestinationStorageID string
}

func StoredAssetCopiesSize(dataDir string, copies []StoredAssetCopy) (uint64, error) {
	var total uint64
	for _, item := range copies {
		if !validStorageID(item.Source.StorageID) || !validStorageID(item.DestinationStorageID) {
			return 0, errors.New("invalid storage ID")
		}
		sourceDirectory, err := AssetDirectory(dataDir, item.Source)
		if errors.Is(err, os.ErrNotExist) {
			return 0, ErrStoredAssetsMissing
		}
		if err != nil {
			return 0, err
		}
		for _, name := range []string{"video.mp4", "poster.jpg"} {
			info, err := os.Stat(filepath.Join(sourceDirectory, name))
			if errors.Is(err, os.ErrNotExist) {
				return 0, ErrStoredAssetsMissing
			}
			if err != nil {
				return 0, err
			}
			total += uint64(info.Size())
		}
	}
	return total, nil
}

// StageStoredAssetCopies duplicates immutable ready media under /temporary.
// Nothing in this directory is publicly addressable.
func StageStoredAssetCopies(ctx context.Context, dataDir, requestID string, copies []StoredAssetCopy) error {
	if !validStorageID(requestID) {
		return errors.New("invalid copy request ID")
	}
	stageRoot := copyStageRoot(dataDir, requestID)
	if err := os.Mkdir(stageRoot, 0o750); err != nil {
		return fmt.Errorf("create copy staging directory: %w", err)
	}
	for _, item := range copies {
		if !validStorageID(item.Source.StorageID) || !validStorageID(item.DestinationStorageID) {
			return errors.New("invalid storage ID")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		destination := filepath.Join(stageRoot, item.DestinationStorageID)
		if err := os.Mkdir(destination, 0o750); err != nil {
			return err
		}
		sourceDirectory, err := AssetDirectory(dataDir, item.Source)
		if errors.Is(err, os.ErrNotExist) {
			return ErrStoredAssetsMissing
		}
		if err != nil {
			return err
		}
		for _, name := range []string{"video.mp4", "poster.jpg"} {
			source := filepath.Join(sourceDirectory, name)
			if err := copyStoredFile(ctx, source, filepath.Join(destination, name)); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return ErrStoredAssetsMissing
				}
				return err
			}
		}
	}
	return nil
}

// PublishStagedCopies atomically renames each staged directory into its resolved
// readable /media destination.
// The caller invokes this while its SQLite transaction is still uncommitted.
func PublishStagedCopies(dataDir, requestID string, copies []StoredAssetCopy, destinations []store.MediaLayoutEntry) error {
	destinationByStorageID := make(map[string]store.MediaLayoutEntry, len(destinations))
	for _, entry := range destinations {
		if !validStorageID(entry.StorageID) || entry.RelativeDir == "" {
			return errors.New("invalid destination media layout")
		}
		destinationByStorageID[entry.StorageID] = entry
	}
	stageRoot := copyStageRoot(dataDir, requestID)
	for _, item := range copies {
		entry, ok := destinationByStorageID[item.DestinationStorageID]
		if !ok {
			return errors.New("missing destination media layout")
		}
		destination := filepath.Join(dataDir, "media", filepath.FromSlash(entry.RelativeDir))
		if _, err := os.Stat(destination); err == nil {
			return errors.New("destination media already exists")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(stageRoot, item.DestinationStorageID), destination); err != nil {
			return err
		}
	}
	return nil
}

func CleanupStagedCopy(dataDir, requestID string) error {
	if !validStorageID(requestID) {
		return errors.New("invalid copy request ID")
	}
	return os.RemoveAll(copyStageRoot(dataDir, requestID))
}

func RemovePublishedCopies(dataDir string, copies []StoredAssetCopy) error {
	var errs []error
	for _, item := range copies {
		if err := RemoveStoredAssets(dataDir, item.DestinationStorageID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func copyStoredFile(ctx context.Context, source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, contextReader{ctx: ctx, reader: input})
	closeErr := output.Close()
	return errors.Join(copyErr, closeErr)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

func copyStageRoot(dataDir, requestID string) string {
	return filepath.Join(dataDir, "temporary", "copy-"+requestID)
}

func validStorageID(value string) bool {
	return value != "" && value != "." && filepath.Base(value) == value
}

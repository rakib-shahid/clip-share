package media

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RemoveStoredAssets permanently removes both final and temporary data for one
// server-generated storage ID. RemoveAll succeeds when a directory is already
// absent, which makes retention safe to retry after an interrupted pass.
func RemoveStoredAssets(dataDir, storageID string) error {
	if storageID == "" || storageID == "." || filepath.Base(storageID) != storageID {
		return fmt.Errorf("invalid storage ID")
	}
	var errs []error
	if err := removeMediaDirectory(dataDir, storageID); err != nil {
		errs = append(errs, err)
	}
	if err := os.RemoveAll(filepath.Join(dataDir, "temporary", storageID)); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func removeMediaDirectory(dataDir, storageID string) error {
	root := filepath.Join(dataDir, "media")
	var matches []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == storageID || strings.HasSuffix(d.Name(), "--"+storageID)) {
			matches = append(matches, path)
			return filepath.SkipDir
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, path := range matches {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

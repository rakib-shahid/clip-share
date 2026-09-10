package media

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"clip-share/internal/store"
)

func AssetDirectory(dataDir string, entry store.MediaLayoutEntry) (string, error) {
	desired := filepath.Join(dataDir, "media", filepath.FromSlash(entry.RelativeDir))
	if info, err := os.Stat(desired); err == nil && info.IsDir() {
		return desired, nil
	}
	var found string
	err := filepath.WalkDir(filepath.Join(dataDir, "media"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if name == entry.StorageID || strings.HasSuffix(name, "--"+entry.StorageID) {
			found = path
			return fs.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", os.ErrNotExist
	}
	return found, nil
}

// ReconcileLayout moves ready asset directories into their readable locations.
// It preflights every source and rolls back moves made by this call on failure.
func ReconcileLayout(dataDir string, entries []store.MediaLayoutEntry) error {
	type move struct{ from, to string }
	moves := make([]move, 0, len(entries))
	for _, entry := range entries {
		from, err := AssetDirectory(dataDir, entry)
		if err != nil {
			return fmt.Errorf("locate %s: %w", entry.StorageID, err)
		}
		to := filepath.Join(dataDir, "media", filepath.FromSlash(entry.RelativeDir))
		if filepath.Clean(from) == filepath.Clean(to) {
			continue
		}
		if _, err := os.Stat(to); err == nil {
			return fmt.Errorf("destination already exists: %s", to)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		moves = append(moves, move{from, to})
	}
	movedItems := []move{}
	for _, item := range moves {
		from, to := item.from, item.to
		if err := os.MkdirAll(filepath.Dir(to), 0o750); err != nil {
			for i := len(movedItems) - 1; i >= 0; i-- {
				_ = os.Rename(movedItems[i].to, movedItems[i].from)
			}
			return err
		}
		if err := os.Rename(from, to); err != nil {
			for i := len(movedItems) - 1; i >= 0; i-- {
				_ = os.Rename(movedItems[i].to, movedItems[i].from)
			}
			return err
		}
		movedItems = append(movedItems, move{from, to})
	}
	return nil
}

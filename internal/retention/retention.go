package retention

import (
	"context"
	"log/slog"
	"time"

	"clip-share/internal/media"
	"clip-share/internal/store"
)

const (
	retentionAge = 90 * 24 * time.Hour
	passInterval = time.Hour
)

// Run performs one retention pass at startup, then repeats hourly until Docker
// asks the application to stop.
func Run(ctx context.Context, data *store.Store, dataDir string, logger *slog.Logger) {
	pass := func() {
		started := time.Now()
		count, err := PurgeOnce(ctx, data, dataDir, time.Now())
		if err != nil && ctx.Err() == nil {
			logger.Error("retention_purge_failed", "error", err, "durationMs", time.Since(started).Milliseconds())
			return
		}
		if ctx.Err() == nil {
			logger.Info("retention_purge_completed", "items", count, "durationMs", time.Since(started).Milliseconds())
		}
	}

	pass()
	ticker := time.NewTicker(passInterval)
	editorTicker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	defer editorTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pass()
		case <-editorTicker.C:
			storageIDs, err := data.ExpireEditorSessions(ctx, time.Now())
			if err != nil && ctx.Err() == nil {
				logger.Error("editor_cleanup_failed", "error", err)
				continue
			}
			for _, storageID := range storageIDs {
				if err := media.RemoveStoredAssets(dataDir, storageID); err != nil && ctx.Err() == nil {
					logger.Error("editor_asset_cleanup_failed", "error", err)
				}
			}
		}
	}
}

// PurgeOnce is exported so the one-pass behavior can be tested without waiting
// for the hourly ticker.
func PurgeOnce(ctx context.Context, data *store.Store, dataDir string, now time.Time) (int, error) {
	return data.PurgeExpiredTrash(ctx, now.Add(-retentionAge), func(storageID string) error {
		return media.RemoveStoredAssets(dataDir, storageID)
	})
}

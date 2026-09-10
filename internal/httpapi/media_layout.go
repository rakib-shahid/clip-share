package httpapi

import (
	"context"

	"clip-share/internal/media"
)

// reconcileMediaLayout keeps human-readable disk paths in step with successful
// metadata mutations. AssetDirectory retains a legacy fallback if this fails, so
// public URLs remain available while the next mutation or explicit migration fixes it.
func (a *API) reconcileMediaLayout(ctx context.Context) {
	entries, err := a.store.MediaLayoutEntries(ctx)
	if err == nil {
		err = media.ReconcileLayout(a.cfg.DataDir, entries)
	}
	if err != nil {
		a.logger.Error("media_layout_reconcile_failed", "error", err)
	}
}

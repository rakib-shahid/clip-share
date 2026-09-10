package httpapi

import (
	"clip-share/internal/media"
	"clip-share/internal/store"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

func (a *API) listTrash(w http.ResponseWriter, r *http.Request) {
	actor := userFromContext(r.Context())
	var owner *int64
	if actor.Role != "admin" {
		owner = &actor.ID
	}
	items, err := a.store.ListTrash(r.Context(), owner)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) previewTrashClip(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		a.notFound(w, r)
		return
	}
	asset := r.PathValue("asset")
	name, contentType := "video.mp4", "video/mp4"
	if asset == "poster" {
		name, contentType = "poster.jpg", "image/jpeg"
	} else if asset != "video" {
		a.notFound(w, r)
		return
	}
	actor := userFromContext(r.Context())
	clip, err := a.store.TrashClipForPreview(r.Context(), id, requiredOwner(actor))
	if errors.Is(err, store.ErrTrashNotFound) {
		a.notFound(w, r)
		return
	}
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	entry, err := a.store.MediaLayoutEntry(r.Context(), clip.ID)
	if err != nil {
		a.notFound(w, r)
		return
	}
	directory, err := media.AssetDirectory(a.cfg.DataDir, entry)
	if err != nil {
		a.notFound(w, r)
		return
	}
	file, err := os.Open(filepath.Join(directory, name))
	if err != nil {
		a.notFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		a.notFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, name, info.ModTime(), file)
}

func (a *API) purgeTrashItem(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		a.notFound(w, r)
		return
	}
	kind := r.PathValue("kind")
	count, err := a.store.PurgeTrashItem(r.Context(), kind, id, nil, func(storageID string) error {
		return media.RemoveStoredAssets(a.cfg.DataDir, storageID)
	})
	if errors.Is(err, store.ErrTrashNotFound) {
		a.notFound(w, r)
		return
	}
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	a.logger.Info("trash_purged", "kind", kind, "itemId", id, "mediaItems", count, "actorUserId", userFromContext(r.Context()).ID)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) restoreTrashClip(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		a.notFound(w, r)
		return
	}
	actor := userFromContext(r.Context())
	var owner *int64
	if actor.Role != "admin" {
		owner = &actor.ID
	}
	if err := a.store.RestoreClip(r.Context(), id, owner); errors.Is(err, store.ErrClipNotFound) {
		a.notFound(w, r)
		return
	} else if errors.Is(err, store.ErrTrashConflict) {
		writeError(w, http.StatusConflict, "restore_conflict", "A clip with this title already exists at the restore destination.", "")
		return
	} else if err != nil {
		a.internalError(w, r, err)
		return
	}
	a.reconcileMediaLayout(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) restoreTrashFolder(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		a.notFound(w, r)
		return
	}
	actor := userFromContext(r.Context())
	err = a.store.RestoreFolder(r.Context(), id, requiredOwner(actor))
	switch {
	case errors.Is(err, store.ErrFolderNotFound):
		a.notFound(w, r)
	case errors.Is(err, store.ErrTrashConflict):
		writeError(w, http.StatusConflict, "restore_conflict", "A folder with this name already exists at the restore destination.", "")
	case err != nil:
		a.internalError(w, r, err)
	default:
		a.reconcileMediaLayout(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}
}

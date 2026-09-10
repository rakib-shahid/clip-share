package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"clip-share/internal/media"
	"clip-share/internal/store"
)

type copyFolderRequest struct {
	DestinationFolderID int64 `json:"destinationFolderId"`
}

func (a *API) copyFolder(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	sourceID, ok := folderID(w, r)
	if !ok {
		return
	}
	var input copyFolderRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.DestinationFolderID < 1 {
		writeError(w, http.StatusBadRequest, "invalid_destination", "Choose a valid destination folder.", "destinationFolderId")
		return
	}
	plan, err := a.store.PrepareFolderCopy(r.Context(), sourceID, input.DestinationFolderID)
	if err != nil {
		a.folderCopyError(w, r, err)
		return
	}

	requestID, err := randomID()
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	assignments := make([]store.ClipCopyAssignment, 0, len(plan.Clips))
	assetCopies := make([]media.StoredAssetCopy, 0, len(plan.Clips))
	for _, clip := range plan.Clips {
		source, layoutErr := a.store.MediaLayoutEntry(r.Context(), clip.ID)
		if layoutErr != nil {
			a.folderCopyError(w, r, layoutErr)
			return
		}
		publicID, randomErr := randomID()
		if randomErr != nil {
			a.internalError(w, r, randomErr)
			return
		}
		storageID, randomErr := randomID()
		if randomErr != nil {
			a.internalError(w, r, randomErr)
			return
		}
		assignments = append(assignments, store.ClipCopyAssignment{SourceClipID: clip.ID, PublicID: publicID, StorageID: storageID})
		assetCopies = append(assetCopies, media.StoredAssetCopy{Source: source, DestinationStorageID: storageID})
	}

	requiredBytes, err := media.StoredAssetCopiesSize(a.cfg.DataDir, assetCopies)
	if err != nil {
		a.folderCopyError(w, r, err)
		return
	}
	availableBytes, err := a.freeBytes(a.cfg.DataDir)
	if err != nil {
		a.internalError(w, r, fmt.Errorf("check free space: %w", err))
		return
	}
	if availableBytes < minimumFreeBytes || requiredBytes > availableBytes-minimumFreeBytes {
		writeError(w, http.StatusInsufficientStorage, "insufficient_storage", "The server does not have enough free space to copy this folder.", "")
		return
	}

	committed := false
	defer func() {
		_ = media.CleanupStagedCopy(a.cfg.DataDir, requestID)
		if !committed {
			_ = media.RemovePublishedCopies(a.cfg.DataDir, assetCopies)
		}
	}()
	if err := media.StageStoredAssetCopies(r.Context(), a.cfg.DataDir, requestID, assetCopies); err != nil {
		a.folderCopyError(w, r, err)
		return
	}
	copied, err := a.store.CommitFolderCopy(r.Context(), plan, assignments, func(destinations []store.MediaLayoutEntry) error {
		return media.PublishStagedCopies(a.cfg.DataDir, requestID, assetCopies, destinations)
	})
	if err != nil {
		a.folderCopyError(w, r, err)
		return
	}
	committed = true
	a.logger.Info("folder_copied", "sourceFolderId", sourceID, "folderId", copied.ID, "destinationFolderId", input.DestinationFolderID, "ownerUserId", copied.OwnerUserID, "clipCount", len(plan.Clips), "actorUserId", userFromContext(r.Context()).ID)
	writeJSON(w, http.StatusCreated, copied)
}

func (a *API) folderCopyError(w http.ResponseWriter, r *http.Request, err error) {
	var conflict *store.CopyConflictError
	switch {
	case errors.As(err, &conflict):
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"code": "copy_conflict", "message": "A folder with that name already exists at the destination.", "field": "destinationFolderId", "conflicts": conflict.Conflicts}})
	case errors.Is(err, store.ErrFolderNotFound):
		a.notFound(w, r)
	case errors.Is(err, store.ErrProtectedRoot):
		writeError(w, http.StatusConflict, "protected_root", "Account library roots cannot be copied.", "")
	case errors.Is(err, store.ErrInvalidCopy):
		writeError(w, http.StatusConflict, "invalid_folder_copy", "A folder cannot be copied into itself or one of its descendants.", "destinationFolderId")
	case errors.Is(err, store.ErrFolderDepth):
		writeError(w, http.StatusConflict, "folder_depth_exceeded", "The copied folder would exceed 20 levels.", "destinationFolderId")
	case errors.Is(err, store.ErrCopyNotReady):
		writeError(w, http.StatusConflict, "copy_not_ready", "Wait for every clip in this folder to finish processing before copying it.", "")
	case errors.Is(err, store.ErrCopyChanged):
		writeError(w, http.StatusConflict, "copy_changed", "The source folder changed during the copy. Please try again.", "")
	case errors.Is(err, media.ErrStoredAssetsMissing):
		writeError(w, http.StatusConflict, "copy_media_unavailable", "A source clip's stored media is unavailable, so nothing was copied.", "")
	case errors.Is(err, store.ErrFolderNameTaken):
		writeError(w, http.StatusConflict, "copy_conflict", "A folder with that name already exists at the destination.", "destinationFolderId")
	default:
		a.internalError(w, r, err)
	}
}

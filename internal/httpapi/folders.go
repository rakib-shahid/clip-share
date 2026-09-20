package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"clip-share/internal/library"
	"clip-share/internal/store"
)

type createFolderRequest struct {
	ParentFolderID int64  `json:"parentFolderId"`
	Name           string `json:"name"`
}

type renameFolderRequest struct {
	Name string `json:"name"`
}

type moveFolderRequest struct {
	DestinationFolderID int64 `json:"destinationFolderId"`
}

func (a *API) getFolder(w http.ResponseWriter, r *http.Request) {
	id, ok := folderID(w, r)
	if !ok {
		return
	}
	sort, err := store.ParseFolderSort(r.URL.Query().Get("sort"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_sort", "The folder sort is invalid.", "sort")
		return
	}
	var decodedCursor *store.FolderCursor
	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		decoded, err := store.DecodeFolderCursor(cursor, id, sort)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_cursor", "The page cursor is invalid.", "cursor")
			return
		}
		decodedCursor = &decoded
	}
	page, err := a.store.FolderPage(r.Context(), id, store.FolderPageOptions{Sort: sort, Cursor: decodedCursor})
	if err != nil {
		a.folderError(w, r, err)
		return
	}
	if !canAccessFolder(userFromContext(r.Context()), page.Folder) {
		a.notFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (a *API) createFolder(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	var input createFolderRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	parent, err := a.store.FolderPage(r.Context(), input.ParentFolderID, store.FolderPageOptions{Sort: store.SortLatest})
	if err != nil {
		a.folderError(w, r, err)
		return
	}
	actor := userFromContext(r.Context())
	if !canAccessFolder(actor, parent.Folder) {
		a.notFound(w, r)
		return
	}
	display, normalized, err := library.NormalizeFolderName(input.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_folder_name", err.Error(), "name")
		return
	}
	folder, err := a.store.CreateFolder(r.Context(), input.ParentFolderID, display, normalized, requiredOwner(actor))
	if err != nil {
		a.folderError(w, r, err)
		return
	}
	a.logger.Info("folder_created", "folderId", folder.ID, "ownerUserId", folder.OwnerUserID, "actorUserId", actor.ID)
	writeJSON(w, http.StatusCreated, folder)
}

func (a *API) renameFolder(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	id, ok := folderID(w, r)
	if !ok {
		return
	}
	page, err := a.store.FolderPage(r.Context(), id, store.FolderPageOptions{Sort: store.SortLatest})
	if err != nil {
		a.folderError(w, r, err)
		return
	}
	actor := userFromContext(r.Context())
	if !canAccessFolder(actor, page.Folder) {
		a.notFound(w, r)
		return
	}
	var input renameFolderRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	display, normalized, err := library.NormalizeFolderName(input.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_folder_name", err.Error(), "name")
		return
	}
	folder, err := a.store.RenameFolder(r.Context(), id, display, normalized, requiredOwner(actor))
	if err != nil {
		a.folderError(w, r, err)
		return
	}
	a.reconcileMediaLayout(r.Context())
	a.logger.Info("folder_renamed", "folderId", folder.ID, "ownerUserId", folder.OwnerUserID, "actorUserId", actor.ID)
	writeJSON(w, http.StatusOK, folder)
}

func (a *API) moveFolder(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	id, ok := folderID(w, r)
	if !ok {
		return
	}
	source, err := a.store.FolderPage(r.Context(), id, store.FolderPageOptions{Sort: store.SortLatest})
	if err != nil {
		a.folderError(w, r, err)
		return
	}
	actor := userFromContext(r.Context())
	if !canAccessFolder(actor, source.Folder) {
		a.notFound(w, r)
		return
	}
	var input moveFolderRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	destination, err := a.store.FolderPage(r.Context(), input.DestinationFolderID, store.FolderPageOptions{Sort: store.SortLatest})
	if err != nil {
		a.folderError(w, r, err)
		return
	}
	if !canAccessFolder(actor, destination.Folder) {
		a.notFound(w, r)
		return
	}
	if actor.Role != "admin" && source.Folder.OwnerUserID != destination.Folder.OwnerUserID {
		a.notFound(w, r)
		return
	}
	folder, err := a.store.MoveFolder(r.Context(), id, input.DestinationFolderID, requiredOwner(actor))
	if err != nil {
		a.folderError(w, r, err)
		return
	}
	a.reconcileMediaLayout(r.Context())
	a.logger.Info("folder_moved", "folderId", folder.ID, "ownerUserId", folder.OwnerUserID, "actorUserId", actor.ID)
	writeJSON(w, http.StatusOK, folder)
}

func (a *API) folderDeletionSummary(w http.ResponseWriter, r *http.Request) {
	id, ok := folderID(w, r)
	if !ok {
		return
	}
	actor := userFromContext(r.Context())
	summary, err := a.store.FolderDeletionSummary(r.Context(), id, requiredOwner(actor))
	if err != nil {
		a.folderError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (a *API) trashFolder(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	id, ok := folderID(w, r)
	if !ok {
		return
	}
	actor := userFromContext(r.Context())
	if err := a.store.TrashFolder(r.Context(), id, requiredOwner(actor)); errors.Is(err, store.ErrFolderNotFound) || errors.Is(err, store.ErrProtectedRoot) {
		a.notFound(w, r)
		return
	} else if err != nil {
		a.internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) authorizeMutation(w http.ResponseWriter, r *http.Request) bool {
	cookie, _ := r.Cookie(sessionCookieName)
	if !a.validMutation(r, cookie.Value) {
		writeError(w, http.StatusForbidden, "csrf_failed", "The request could not be verified.", "")
		return false
	}
	return true
}

func folderID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("folderID"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusNotFound, "not_found", "Not found.", "")
		return 0, false
	}
	return id, true
}

func canAccessFolder(user store.User, folder store.Folder) bool {
	return user.Role == "admin" || user.ID == folder.OwnerUserID
}

func requiredOwner(user store.User) *int64 {
	if user.Role == "admin" {
		return nil
	}
	return &user.ID
}

func (a *API) folderError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrFolderNotFound):
		a.notFound(w, r)
	case errors.Is(err, store.ErrFolderNameTaken):
		writeError(w, http.StatusConflict, "folder_name_taken", "A folder with that name already exists here.", "name")
	case errors.Is(err, store.ErrProtectedRoot):
		writeError(w, http.StatusConflict, "protected_root", "Account library roots cannot be changed.", "")
	case errors.Is(err, store.ErrInvalidMove):
		writeError(w, http.StatusConflict, "invalid_folder_move", "A folder cannot be moved into itself or one of its descendants.", "destinationFolderId")
	case errors.Is(err, store.ErrFolderDepth):
		writeError(w, http.StatusConflict, "folder_depth_exceeded", "Folder nesting cannot exceed 20 levels.", "destinationFolderId")
	default:
		a.internalError(w, r, err)
	}
}

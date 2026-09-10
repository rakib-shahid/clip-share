package httpapi

import (
	"clip-share/internal/library"
	"clip-share/internal/store"
	"errors"
	"net/http"
	"strconv"
)

func (a *API) renameClip(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(sessionCookieName)
	if !a.validMutation(r, cookie.Value) {
		writeError(w, 403, "csrf_failed", "The request could not be verified.", "")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("clipID"), 10, 64)
	if err != nil {
		a.notFound(w, r)
		return
	}
	var input struct {
		Title string `json:"title"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	title, normalized, err := library.NormalizeClipTitle(input.Title)
	if err != nil {
		writeError(w, 400, "invalid_title", err.Error(), "title")
		return
	}
	actor := userFromContext(r.Context())
	var owner *int64
	if actor.Role != "admin" {
		owner = &actor.ID
	}
	err = a.store.RenameClip(r.Context(), id, title, normalized, owner)
	if errors.Is(err, store.ErrClipNotFound) {
		a.notFound(w, r)
		return
	}
	if errors.Is(err, store.ErrClipTitleTaken) {
		writeError(w, 409, "title_conflict", "A clip with this title already exists in that folder.", "title")
		return
	}
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	a.reconcileMediaLayout(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) trashClip(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("clipID"), 10, 64)
	if err != nil {
		a.notFound(w, r)
		return
	}
	actor := userFromContext(r.Context())
	if err := a.store.TrashClip(r.Context(), id, requiredOwner(actor)); errors.Is(err, store.ErrClipNotFound) {
		a.notFound(w, r)
		return
	} else if err != nil {
		a.internalError(w, r, err)
		return
	}
	a.reconcileMediaLayout(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) moveClip(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("clipID"), 10, 64)
	if err != nil {
		a.notFound(w, r)
		return
	}
	var input struct {
		DestinationFolderID int64 `json:"destinationFolderId"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	actor := userFromContext(r.Context())
	if err := a.store.MoveClip(r.Context(), id, input.DestinationFolderID, requiredOwner(actor)); errors.Is(err, store.ErrClipNotFound) || errors.Is(err, store.ErrFolderNotFound) {
		a.notFound(w, r)
		return
	} else if errors.Is(err, store.ErrClipTitleTaken) {
		writeError(w, 409, "title_conflict", "A clip with this title already exists in that folder.", "")
		return
	} else if err != nil {
		a.internalError(w, r, err)
		return
	}
	a.reconcileMediaLayout(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

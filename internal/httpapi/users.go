package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"clip-share/internal/auth"
	"clip-share/internal/store"
)

type updateUserRequest struct {
	Username          string `json:"username"`
	StoredFileLimitMB int64  `json:"storedFileLimitMb"`
}

type userPasswordRequest struct {
	Password string `json:"password"`
}

type ownPasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

func (a *API) changeOwnPassword(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	var input ownPasswordRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.NewPassword != input.ConfirmPassword {
		writeError(w, http.StatusBadRequest, "password_confirmation_mismatch", "The new passwords do not match.", "confirmPassword")
		return
	}
	if err := auth.ValidatePassword(input.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_password", err.Error(), "newPassword")
		return
	}
	actor := userFromContext(r.Context())
	currentHash, err := a.store.PasswordHashForActiveUser(r.Context(), actor.ID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "current_password_incorrect", "The current password is incorrect.", "currentPassword")
		return
	}
	valid, err := auth.VerifyPassword(input.CurrentPassword, currentHash)
	if err != nil || !valid {
		writeError(w, http.StatusUnauthorized, "current_password_incorrect", "The current password is incorrect.", "currentPassword")
		return
	}
	unchanged, err := auth.VerifyPassword(input.NewPassword, currentHash)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	if unchanged {
		writeError(w, http.StatusConflict, "new_password_unchanged", "Choose a new password that differs from the current password.", "newPassword")
		return
	}
	replacementHash, err := auth.HashPassword(input.NewPassword)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	if err := a.store.ChangeOwnPassword(r.Context(), actor.ID, currentHash, replacementHash); err != nil {
		if errors.Is(err, store.ErrPasswordChanged) {
			writeError(w, http.StatusUnauthorized, "current_password_incorrect", "The current password is incorrect.", "currentPassword")
			return
		}
		a.internalError(w, r, err)
		return
	}
	a.logger.Info("user_password_changed", "userId", actor.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) updateUser(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	id, ok := requestedUserID(w, r)
	if !ok {
		return
	}
	var input updateUserRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	normalized, err := auth.NormalizeUsername(input.Username)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_username", err.Error(), "username")
		return
	}
	if input.StoredFileLimitMB < 1 || input.StoredFileLimitMB > 500 {
		writeError(w, http.StatusBadRequest, "invalid_limit", "Stored file limit must be 1-500 MB.", "storedFileLimitMb")
		return
	}
	user, err := a.store.UpdateUser(r.Context(), id, input.Username, normalized, input.StoredFileLimitMB*1_000_000)
	if a.handleUserError(w, r, err) {
		return
	}
	a.reconcileMediaLayout(r.Context())
	a.logger.Info("user_updated", "userId", user.ID, "actorUserId", userFromContext(r.Context()).ID)
	writeJSON(w, http.StatusOK, user)
}

func (a *API) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	id, ok := requestedUserID(w, r)
	if !ok {
		return
	}
	password, ok := decodeAndHashPassword(a, w, r)
	if !ok {
		return
	}
	if err := a.store.SetUserPassword(r.Context(), id, password); a.handleUserError(w, r, err) {
		return
	}
	a.logger.Info("user_password_reset", "userId", id, "actorUserId", userFromContext(r.Context()).ID)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) disableUser(w http.ResponseWriter, r *http.Request) {
	a.changeUserState(w, r, "disabled", a.store.DisableUser)
}

func (a *API) enableUser(w http.ResponseWriter, r *http.Request) {
	a.changeUserState(w, r, "enabled", a.store.EnableUser)
}

func (a *API) archiveUser(w http.ResponseWriter, r *http.Request) {
	a.changeUserState(w, r, "archived", a.store.ArchiveUser)
}

func (a *API) restoreUser(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	id, ok := requestedUserID(w, r)
	if !ok {
		return
	}
	password, ok := decodeAndHashPassword(a, w, r)
	if !ok {
		return
	}
	user, err := a.store.RestoreUser(r.Context(), id, password)
	if a.handleUserError(w, r, err) {
		return
	}
	a.logger.Info("user_restored", "userId", id, "actorUserId", userFromContext(r.Context()).ID)
	writeJSON(w, http.StatusOK, user)
}

func (a *API) deleteArchivedLibrary(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	id, ok := requestedUserID(w, r)
	if !ok {
		return
	}
	if err := a.store.TrashArchivedLibrary(r.Context(), id); err != nil {
		switch {
		case errors.Is(err, store.ErrTrashNotFound):
			writeError(w, http.StatusConflict, "library_already_trashed", "This library is already in the recycle bin.", "")
		case errors.Is(err, store.ErrLibraryNotArchived), errors.Is(err, store.ErrLibraryBusy), errors.Is(err, store.ErrProtectedAdmin):
			writeError(w, http.StatusConflict, "library_not_deletable", err.Error(), "")
		default:
			a.handleUserError(w, r, err)
		}
		return
	}
	a.logger.Info("archived_library_trashed", "userId", id, "actorUserId", userFromContext(r.Context()).ID)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) changeUserState(w http.ResponseWriter, r *http.Request, action string, change func(context.Context, int64) (store.User, error)) {
	if !a.authorizeMutation(w, r) {
		return
	}
	id, ok := requestedUserID(w, r)
	if !ok {
		return
	}
	user, err := change(r.Context(), id)
	if a.handleUserError(w, r, err) {
		return
	}
	a.logger.Info("user_"+action, "userId", id, "actorUserId", userFromContext(r.Context()).ID)
	writeJSON(w, http.StatusOK, user)
}

func requestedUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusNotFound, "not_found", "Not found.", "")
		return 0, false
	}
	return id, true
}

func decodeAndHashPassword(a *API, w http.ResponseWriter, r *http.Request) (string, bool) {
	var input userPasswordRequest
	if !decodeJSON(w, r, &input) {
		return "", false
	}
	if err := auth.ValidatePassword(input.Password); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_password", err.Error(), "password")
		return "", false
	}
	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		a.internalError(w, r, err)
		return "", false
	}
	return hash, true
}

func (a *API) handleUserError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, sql.ErrNoRows):
		writeError(w, http.StatusNotFound, "not_found", "Not found.", "")
	case errors.Is(err, store.ErrUsernameTaken):
		writeError(w, http.StatusConflict, "username_taken", "That username is already in use.", "username")
	case errors.Is(err, store.ErrProtectedAdmin):
		writeError(w, http.StatusConflict, "protected_admin", "The super-admin account cannot be disabled or archived.", "")
	case errors.Is(err, store.ErrInvalidUserState):
		writeError(w, http.StatusConflict, "invalid_user_state", "That action is not available for the account's current state.", "")
	case errors.Is(err, store.ErrLibraryTrashed):
		writeError(w, http.StatusConflict, "library_trashed", err.Error(), "")
	default:
		a.internalError(w, r, err)
	}
	return true
}

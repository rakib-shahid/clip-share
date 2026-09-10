package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"clip-share/internal/media"
	"clip-share/internal/store"
)

func (a *API) cancelJob(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	id, ok := requestJobID(w, r)
	if !ok {
		return
	}
	actor := userFromContext(r.Context())
	assets, err := a.store.CancelJob(r.Context(), id, requiredOwner(actor))
	if err != nil {
		a.jobControlError(w, r, err)
		return
	}
	if err := media.RemoveStoredAssets(a.cfg.DataDir, assets.StorageID); err != nil {
		a.internalError(w, r, err)
		return
	}
	a.logger.Info("job_cancelled", "jobId", assets.JobID, "clipId", assets.ClipID, "ownerUserId", assets.OwnerUserID, "actorUserId", actor.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) dismissFailedJob(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	id, ok := requestJobID(w, r)
	if !ok {
		return
	}
	actor := userFromContext(r.Context())
	if err := a.store.DismissFailedJob(r.Context(), id, requiredOwner(actor)); err != nil {
		a.jobControlError(w, r, err)
		return
	}
	a.logger.Info("failed_job_dismissed", "jobId", id, "actorUserId", actor.ID)
	w.WriteHeader(http.StatusNoContent)
}

func requestJobID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("jobID"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusNotFound, "not_found", "Not found.", "")
		return 0, false
	}
	return id, true
}

func (a *API) jobControlError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrJobNotFound):
		a.notFound(w, r)
	case errors.Is(err, store.ErrJobNotCancellable):
		writeError(w, http.StatusConflict, "job_not_cancellable", "Only queued or processing uploads can be cancelled.", "")
	case errors.Is(err, store.ErrJobNotFailed):
		writeError(w, http.StatusConflict, "job_not_failed", "Only failed upload notices can be dismissed.", "")
	default:
		a.internalError(w, r, err)
	}
}

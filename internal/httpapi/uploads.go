package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"clip-share/internal/library"
	"clip-share/internal/media"
	"clip-share/internal/store"
)

const (
	maximumSourceBytes  = int64(500_000_000)
	maximumRequestBytes = int64(501_000_000)
	minimumFreeBytes    = uint64(5_000_000_000)
)

type uploadFields struct {
	Title                string
	DestinationFolderID  int64
	CompressionRequested bool
	QualityCRF           int
	MaxHeight            int
	EditingRequested     bool
}

func (a *API) upload(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	if r.ContentLength > maximumRequestBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "source_too_large", "Source videos cannot exceed 500 MB.", "video")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Hour)
	defer cancel()
	r = r.WithContext(ctx)
	r.Body = http.MaxBytesReader(w, r.Body, maximumRequestBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_multipart", "The upload request is invalid.", "")
		return
	}

	fields := uploadFields{QualityCRF: 24, MaxHeight: 1080}
	seen := map[string]bool{}
	var record store.UploadRecord
	var reservationKey, temporaryDirectory, sourcePath string
	committed := false
	defer func() {
		if !committed && record.ClipID != 0 {
			_ = a.store.AbortUpload(context.Background(), record.ClipID, reservationKey)
			_ = os.RemoveAll(temporaryDirectory)
		}
	}()

	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			a.uploadReadError(w, nextErr)
			return
		}
		name := part.FormName()
		if name != "video" {
			if seen[name] {
				part.Close()
				writeError(w, http.StatusBadRequest, "duplicate_field", "An upload field was provided more than once.", name)
				return
			}
			seen[name] = true
			value, readErr := readSmallPart(part)
			part.Close()
			if readErr != nil {
				writeError(w, http.StatusBadRequest, "invalid_field", "An upload field is too large.", name)
				return
			}
			if !parseUploadField(w, &fields, name, value) {
				return
			}
			continue
		}
		if record.ClipID != 0 {
			part.Close()
			writeError(w, http.StatusBadRequest, "multiple_videos", "Upload exactly one video.", "video")
			return
		}
		if !seen["title"] || !seen["destinationFolderId"] || !seen["compressionRequested"] {
			part.Close()
			writeError(w, http.StatusBadRequest, "metadata_before_video", "Upload metadata must be provided before the video.", "video")
			return
		}
		displayTitle, normalizedTitle, titleErr := library.NormalizeClipTitle(fields.Title)
		if titleErr != nil {
			part.Close()
			writeError(w, http.StatusBadRequest, "invalid_clip_title", titleErr.Error(), "title")
			return
		}
		actor := userFromContext(r.Context())
		available, freeErr := a.freeBytes(a.cfg.DataDir)
		if freeErr != nil {
			part.Close()
			a.internalError(w, r, fmt.Errorf("check free space: %w", freeErr))
			return
		}
		publicID, randomErr := randomID()
		if randomErr != nil {
			part.Close()
			a.internalError(w, r, randomErr)
			return
		}
		storageID, randomErr := randomID()
		if randomErr != nil {
			part.Close()
			a.internalError(w, r, randomErr)
			return
		}
		reservationKey, randomErr = randomID()
		if randomErr != nil {
			part.Close()
			a.internalError(w, r, randomErr)
			return
		}
		projectedBytes := maximumSourceBytes * 2
		if r.ContentLength > 0 && r.ContentLength < maximumSourceBytes {
			projectedBytes = r.ContentLength * 2
		}
		record, err = a.store.BeginUpload(r.Context(), fields.DestinationFolderID, displayTitle, normalizedTitle, publicID, storageID, reservationKey, projectedBytes, minimumFreeBytes, available, requiredOwner(actor))
		if err != nil {
			part.Close()
			a.uploadStoreError(w, r, err)
			return
		}
		temporaryDirectory = filepath.Join(a.cfg.DataDir, "temporary", storageID)
		if err := os.MkdirAll(temporaryDirectory, 0o750); err != nil {
			part.Close()
			a.internalError(w, r, err)
			return
		}
		sourcePath = filepath.Join(temporaryDirectory, "source")
		file, createErr := os.OpenFile(sourcePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
		if createErr != nil {
			part.Close()
			a.internalError(w, r, createErr)
			return
		}
		written, copyErr := io.Copy(file, io.LimitReader(part, maximumSourceBytes+1))
		closeErr := file.Close()
		part.Close()
		if copyErr != nil {
			a.uploadReadError(w, copyErr)
			return
		}
		if closeErr != nil {
			a.internalError(w, r, closeErr)
			return
		}
		if written > maximumSourceBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "source_too_large", "Source videos cannot exceed 500 MB.", "video")
			return
		}
		if actor.Role != "admin" && written > actor.StoredFileLimitBytes && !fields.CompressionRequested {
			writeError(w, http.StatusBadRequest, "compression_required", "Compression is required because the source exceeds your stored-file limit.", "compressionRequested")
			return
		}
		metadata, probeErr := a.probe.Probe(r.Context(), sourcePath)
		if probeErr != nil {
			a.uploadProbeError(w, probeErr)
			return
		}
		if fields.EditingRequested && usableAudioStreams(metadata) > 8 {
			writeError(w, http.StatusUnprocessableEntity, "too_many_audio_tracks", "This source has more than eight usable audio tracks.", "video")
			return
		}
		sessionID, idErr := randomID()
		if idErr != nil {
			a.internalError(w, r, idErr)
			return
		}
		options := store.UploadOptions{CompressionRequested: fields.CompressionRequested, QualityCRF: fields.QualityCRF, MaxHeight: fields.MaxHeight, TargetSizeBytes: actor.StoredFileLimitBytes, StoredLimitBypassed: actor.Role == "admin"}
		if !fields.EditingRequested {
			job, completeErr := a.store.CompleteUpload(r.Context(), record, reservationKey, written, metadata, options)
			if completeErr != nil {
				a.internalError(w, r, completeErr)
				return
			}
			committed = true
			a.logger.Info("upload_queued", "jobId", job.ID, "clipId", job.ClipID, "ownerUserId", record.OwnerUserID, "actorUserId", actor.ID, "sourceBytes", written)
			writeJSON(w, http.StatusCreated, map[string]any{"job": job, "clip": map[string]any{"id": record.ClipID, "title": record.Title, "state": "queued"}})
			return
		}
		session, completeErr := a.store.CreateEditorSession(r.Context(), sessionID, actor.ID, record, reservationKey, written, metadata, options)
		if completeErr != nil {
			a.internalError(w, r, completeErr)
			return
		}
		committed = true
		a.logger.Info("upload_editor_ready", "clipId", record.ClipID, "ownerUserId", record.OwnerUserID, "actorUserId", actor.ID, "sourceBytes", written)
		writeJSON(w, http.StatusCreated, session)
		return
	}
	writeError(w, http.StatusBadRequest, "video_required", "Choose one video to upload.", "video")
}

func (a *API) getResumableEditorSession(w http.ResponseWriter, r *http.Request) {
	destinationID, err := strconv.ParseInt(r.URL.Query().Get("destinationFolderId"), 10, 64)
	if err != nil || destinationID < 1 {
		writeError(w, http.StatusBadRequest, "invalid_destination", "Choose a valid destination folder.", "destinationFolderId")
		return
	}
	sourceSizeBytes, err := strconv.ParseInt(r.URL.Query().Get("sourceSizeBytes"), 10, 64)
	if err != nil || sourceSizeBytes < 1 || sourceSizeBytes > maximumSourceBytes {
		writeError(w, http.StatusBadRequest, "invalid_source_size", "Choose a valid source video.", "sourceSizeBytes")
		return
	}
	_, normalizedTitle, err := library.NormalizeClipTitle(r.URL.Query().Get("title"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_clip_title", err.Error(), "title")
		return
	}
	now := time.Now()
	storageIDs, err := a.store.ExpireEditorSessions(r.Context(), now)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	for _, storageID := range storageIDs {
		if err := media.RemoveStoredAssets(a.cfg.DataDir, storageID); err != nil {
			a.logger.Error("editor_asset_cleanup_failed", "storageId", storageID, "error", err)
		}
	}
	session, err := a.store.ResumableEditorSession(r.Context(), userFromContext(r.Context()).ID, destinationID, sourceSizeBytes, normalizedTitle, now)
	if errors.Is(err, store.ErrEditorSessionNotFound) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func usableAudioStreams(metadata store.ProbeMetadata) int {
	count := 0
	for _, stream := range metadata.AudioStreams {
		if stream.Usable {
			count++
		}
	}
	return count
}

func (a *API) getEditorSession(w http.ResponseWriter, r *http.Request) {
	session, err := a.store.EditorSessionByID(r.Context(), r.PathValue("uploadID"), userFromContext(r.Context()).ID)
	if errors.Is(err, store.ErrEditorSessionNotFound) {
		a.notFound(w, r)
		return
	}
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

type editRequest struct {
	TrimStartMS int                 `json:"trimStartMs"`
	TrimEndMS   int                 `json:"trimEndMs"`
	Audio       []store.AudioStream `json:"audio"`
}

func (a *API) saveEditorRecipe(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	revision, err := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), "\" "))
	if err != nil || revision < 1 {
		writeError(w, http.StatusBadRequest, "invalid_edit", "A current edit revision is required.", "")
		return
	}
	var input editRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	session, err := a.store.SaveEditorRecipe(r.Context(), r.PathValue("uploadID"), userFromContext(r.Context()).ID, revision, store.EditorRecipe{TrimStartMS: input.TrimStartMS, TrimEndMS: input.TrimEndMS, Audio: input.Audio})
	switch {
	case errors.Is(err, store.ErrEditorSessionNotFound):
		a.notFound(w, r)
	case errors.Is(err, store.ErrEditConflict):
		writeError(w, http.StatusConflict, "edit_conflict", "This editor was changed elsewhere. Reload or overwrite your local changes.", "")
	case errors.Is(err, store.ErrSessionNotEditable):
		writeError(w, http.StatusConflict, "session_not_editable", "This editor can no longer be changed.", "")
	case errors.Is(err, store.ErrInvalidTrim):
		writeError(w, http.StatusUnprocessableEntity, "invalid_trim", "Choose a trim range of at least 0.25 seconds within the source video.", "")
	case errors.Is(err, store.ErrInvalidAudioGain):
		writeError(w, http.StatusUnprocessableEntity, "invalid_audio_gain", "Audio gains must be between -60 and +12 dB in 0.5 dB steps.", "")
	case errors.Is(err, store.ErrAudioTrackUnusable):
		writeError(w, http.StatusUnprocessableEntity, "audio_track_unusable", "One selected audio track cannot be used.", "")
	case err != nil:
		a.internalError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, session)
	}
}

type editorMetadataRequest struct {
	Title               string `json:"title"`
	DestinationFolderID int64  `json:"destinationFolderId"`
}

func (a *API) updateEditorMetadata(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	revision, err := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), "\" "))
	if err != nil || revision < 1 {
		writeError(w, http.StatusBadRequest, "invalid_edit", "A current edit revision is required.", "")
		return
	}
	var input editorMetadataRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	title, normalized, err := library.NormalizeClipTitle(input.Title)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_clip_title", err.Error(), "title")
		return
	}
	actor := userFromContext(r.Context())
	session, err := a.store.UpdateEditorMetadata(r.Context(), r.PathValue("uploadID"), actor.ID, revision, title, normalized, input.DestinationFolderID, requiredOwner(actor))
	switch {
	case errors.Is(err, store.ErrEditorSessionNotFound):
		a.notFound(w, r)
	case errors.Is(err, store.ErrEditConflict):
		writeError(w, http.StatusConflict, "edit_conflict", "This editor was changed elsewhere. Reload it before saving.", "")
	case errors.Is(err, store.ErrSessionNotEditable):
		writeError(w, http.StatusConflict, "session_not_editable", "This editor can no longer be changed.", "")
	case errors.Is(err, store.ErrFolderNotFound):
		a.notFound(w, r)
	case errors.Is(err, store.ErrClipTitleTaken):
		writeError(w, http.StatusConflict, "title_conflict", "A clip with this title already exists in that folder.", "title")
	case err != nil:
		a.internalError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, session)
	}
}

func (a *API) heartbeatEditorSession(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	err := a.store.HeartbeatEditorSession(r.Context(), r.PathValue("uploadID"), userFromContext(r.Context()).ID)
	if errors.Is(err, store.ErrEditorSessionNotFound) {
		a.notFound(w, r)
		return
	}
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) finalizeEditorSession(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	revision, err := strconv.Atoi(strings.Trim(r.Header.Get("If-Match"), "\" "))
	if err != nil || revision < 1 {
		writeError(w, http.StatusBadRequest, "invalid_edit", "A current edit revision is required.", "")
		return
	}
	job, err := a.store.FinalizeEditorSession(r.Context(), r.PathValue("uploadID"), userFromContext(r.Context()).ID, revision)
	switch {
	case errors.Is(err, store.ErrEditorSessionNotFound):
		a.notFound(w, r)
	case errors.Is(err, store.ErrEditConflict):
		writeError(w, http.StatusConflict, "edit_conflict", "This editor was changed elsewhere. Reload it before finalizing.", "")
	case errors.Is(err, store.ErrSessionNotEditable):
		writeError(w, http.StatusConflict, "session_not_editable", "This editor can no longer be finalized.", "")
	case errors.Is(err, store.ErrInvalidTrim):
		writeError(w, http.StatusUnprocessableEntity, "invalid_trim", "Choose a valid trim range.", "")
	case err != nil:
		a.internalError(w, r, err)
	default:
		writeJSON(w, http.StatusAccepted, map[string]any{"job": job})
	}
}

type previewRequest struct {
	PlayheadMS int  `json:"playheadMs"`
	FullSource bool `json:"fullSource"`
}

func (a *API) renderEditorPreview(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	var input previewRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	actor := userFromContext(r.Context())
	id := r.PathValue("uploadID")
	session, err := a.store.EditorSessionByID(r.Context(), id, actor.ID)
	if errors.Is(err, store.ErrEditorSessionNotFound) {
		a.notFound(w, r)
		return
	}
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	preview, err := a.store.BeginEditorPreview(r.Context(), id, actor.ID, session.EditRevision, input.PlayheadMS, input.FullSource)
	if errors.Is(err, store.ErrEditConflict) {
		writeError(w, http.StatusConflict, "edit_conflict", "Save or reload the current edit before previewing.", "")
		return
	}
	if errors.Is(err, store.ErrSessionNotEditable) {
		writeError(w, http.StatusConflict, "session_not_editable", "This editor can no longer be previewed.", "")
		return
	}
	if errors.Is(err, store.ErrPreviewInProgress) {
		writeError(w, http.StatusConflict, "preview_in_progress", "A preview is already rendering.", "")
		return
	}
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	output := media.PreviewPath(a.cfg.DataDir, preview.StorageID)
	pendingOutput := media.PendingPreviewPath(a.cfg.DataDir, preview.StorageID)
	_ = os.Remove(pendingOutput)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		err := media.RenderPreview(ctx, media.PreviewRequest{SourcePath: filepath.Join(a.cfg.DataDir, "temporary", preview.StorageID, "source"), OutputPath: pendingOutput, StartMS: preview.PreviewStartMS, EndMS: preview.PreviewEndMS, Audio: preview.Edit.Audio})
		if err == nil {
			err = media.PublishPreview(pendingOutput, output)
		}
		_ = a.store.FinishEditorPreview(context.Background(), id, actor.ID, preview.EditRevision, err == nil)
		if err != nil {
			_ = os.Remove(pendingOutput)
			a.logger.Warn("editor_preview_failed", "clipId", preview.ClipID, "error", err)
		}
	}()
	writeJSON(w, http.StatusAccepted, map[string]any{"state": "rendering", "startMs": preview.PreviewStartMS, "endMs": preview.PreviewEndMS, "revision": preview.EditRevision})
}

func (a *API) getEditorPreview(w http.ResponseWriter, r *http.Request) {
	session, err := a.store.EditorSessionByID(r.Context(), r.PathValue("uploadID"), userFromContext(r.Context()).ID)
	if errors.Is(err, store.ErrEditorSessionNotFound) {
		a.notFound(w, r)
		return
	}
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	if session.PreviewState != "ready" {
		a.notFound(w, r)
		return
	}
	path := media.PreviewPath(a.cfg.DataDir, session.StorageID)
	if _, err := os.Stat(path); err != nil {
		a.notFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, path)
}

func (a *API) discardEditorSession(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeMutation(w, r) {
		return
	}
	storageID, err := a.store.DiscardEditorSession(r.Context(), r.PathValue("uploadID"), userFromContext(r.Context()).ID)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	if storageID != "" {
		_ = os.RemoveAll(filepath.Join(a.cfg.DataDir, "temporary", storageID))
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) getJob(w http.ResponseWriter, r *http.Request) {
	id, ok := requestJobID(w, r)
	if !ok {
		return
	}
	job, err := a.store.JobByID(r.Context(), id)
	if errors.Is(err, store.ErrJobNotFound) {
		a.notFound(w, r)
		return
	}
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	actor := userFromContext(r.Context())
	if actor.Role != "admin" && actor.ID != job.OwnerUserID {
		a.notFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func readSmallPart(part *multipart.Part) (string, error) {
	value, err := io.ReadAll(io.LimitReader(part, 64*1024+1))
	if err != nil || len(value) > 64*1024 {
		return "", errors.New("field too large")
	}
	return string(value), nil
}

func parseUploadField(w http.ResponseWriter, fields *uploadFields, name, value string) bool {
	switch name {
	case "title":
		fields.Title = value
	case "destinationFolderId":
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id < 1 {
			writeError(w, http.StatusBadRequest, "invalid_destination", "Choose a valid destination folder.", name)
			return false
		}
		fields.DestinationFolderID = id
	case "compressionRequested":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_compression", "Compression selection is invalid.", name)
			return false
		}
		fields.CompressionRequested = parsed
	case "qualityCrf":
		parsed, err := strconv.Atoi(value)
		if err != nil || !oneOf(parsed, 18, 21, 24, 27, 30) {
			writeError(w, http.StatusBadRequest, "invalid_quality", "Choose a valid quality setting.", name)
			return false
		}
		fields.QualityCRF = parsed
	case "maxHeight":
		parsed, err := strconv.Atoi(value)
		if err != nil || !oneOf(parsed, 480, 720, 1080) {
			writeError(w, http.StatusBadRequest, "invalid_resolution", "Choose a valid maximum resolution.", name)
			return false
		}
		fields.MaxHeight = parsed
	case "editingRequested":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_editing_selection", "Editing selection is invalid.", name)
			return false
		}
		fields.EditingRequested = parsed
	default:
		writeError(w, http.StatusBadRequest, "unknown_field", "The upload contains an unknown field.", name)
		return false
	}
	return true
}

func oneOf(value int, values ...int) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func randomID() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (a *API) uploadReadError(w http.ResponseWriter, err error) {
	if strings.Contains(err.Error(), "request body too large") {
		writeError(w, http.StatusRequestEntityTooLarge, "source_too_large", "Source videos cannot exceed 500 MB.", "video")
		return
	}
	writeError(w, http.StatusBadRequest, "upload_interrupted", "The upload was interrupted. Please try again.", "video")
}
func (a *API) uploadStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrFolderNotFound):
		a.notFound(w, r)
	case errors.Is(err, store.ErrClipTitleTaken):
		writeError(w, http.StatusConflict, "title_conflict", "A clip with this title already exists in that folder.", "title")
	case errors.Is(err, store.ErrInsufficientStorage):
		writeError(w, http.StatusInsufficientStorage, "insufficient_storage", "The server does not have enough temporary storage for this upload.", "video")
	default:
		a.internalError(w, r, err)
	}
}
func (a *API) uploadProbeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, media.ErrEncryptedMedia):
		writeError(w, http.StatusBadRequest, "encrypted_media", err.Error(), "video")
	case errors.Is(err, media.ErrDurationExceeded):
		writeError(w, http.StatusBadRequest, "duration_exceeded", err.Error(), "video")
	case errors.Is(err, media.ErrResolutionExceeded):
		writeError(w, http.StatusBadRequest, "resolution_exceeded", err.Error(), "video")
	case errors.Is(err, media.ErrFrameRateExceeded):
		writeError(w, http.StatusBadRequest, "frame_rate_exceeded", err.Error(), "video")
	default:
		writeError(w, http.StatusBadRequest, "unsupported_media", "The file is not a supported or readable video.", "video")
	}
}

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"
)

var (
	ErrEditorSessionNotFound = errors.New("editor session not found")
	ErrEditConflict          = errors.New("editor revision conflict")
	ErrSessionNotEditable    = errors.New("editor session is not editable")
	ErrPreviewInProgress     = errors.New("editor preview is already rendering")
	ErrInvalidTrim           = errors.New("invalid trim")
	ErrInvalidAudioGain      = errors.New("invalid audio gain")
	ErrAudioTrackUnusable    = errors.New("audio track is unusable")
)

const editorLease = 5 * time.Minute

type AudioStream struct {
	Index          int     `json:"index"`
	Codec          string  `json:"codec"`
	Channels       int     `json:"channels"`
	ChannelLayout  string  `json:"channelLayout,omitempty"`
	SampleRate     int     `json:"sampleRate,omitempty"`
	Language       string  `json:"language,omitempty"`
	Default        bool    `json:"default"`
	Usable         bool    `json:"usable"`
	UnusableReason string  `json:"unusableReason,omitempty"`
	Included       bool    `json:"included"`
	GainDB         float64 `json:"gainDb"`
}

type EditorRecipe struct {
	TrimStartMS int           `json:"trimStartMs"`
	TrimEndMS   int           `json:"trimEndMs"`
	Audio       []AudioStream `json:"audio"`
}

type EditorSession struct {
	ID                   string       `json:"id"`
	ClipID               int64        `json:"clipId"`
	State                string       `json:"state"`
	EditRevision         int          `json:"editRevision"`
	Title                string       `json:"title"`
	DestinationFolderID  int64        `json:"destinationFolderId"`
	DurationMS           int          `json:"durationMs"`
	VideoStreamIndex     int          `json:"videoStreamIndex"`
	SourceSizeBytes      int64        `json:"sourceSizeBytes"`
	SourceContainer      string       `json:"sourceContainer"`
	SourceVideoCodec     string       `json:"sourceVideoCodec"`
	SourceWidth          int          `json:"sourceWidth"`
	SourceHeight         int          `json:"sourceHeight"`
	SourceFrameRate      float64      `json:"sourceFrameRate"`
	CompressionRequested bool         `json:"compressionRequested"`
	QualityCRF           int          `json:"qualityCrf"`
	MaxHeight            int          `json:"maxHeight"`
	Edit                 EditorRecipe `json:"edit"`
	FinalJob             *Job         `json:"finalJob,omitempty"`
	PreviewState         string       `json:"previewState"`
	PreviewStartMS       int          `json:"previewStartMs,omitempty"`
	PreviewEndMS         int          `json:"previewEndMs,omitempty"`
	PreviewRevision      int          `json:"previewRevision,omitempty"`
	PreviewRendering     bool         `json:"previewRendering"`
	StorageID            string       `json:"-"`
}

func (s *Store) CreateEditorSession(ctx context.Context, id string, actingUserID int64, record UploadRecord, reservationKey string, sourceBytes int64, metadata ProbeMetadata, options UploadOptions) (EditorSession, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EditorSession{}, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	bypassed := 0
	if options.StoredLimitBypassed {
		bypassed = 1
	}
	compressed := 0
	if options.CompressionRequested {
		compressed = 1
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO upload_sessions(id,clip_id,acting_user_id,reservation_key,state,trim_end_ms,source_size_bytes,source_duration_ms,video_stream_index,compression_requested,quality_crf,max_height,target_size_bytes,stored_limit_bypassed,source_container,source_video_codec,source_audio_codec,source_width,source_height,source_frame_rate,source_pixel_format,source_profile,last_heartbeat_at,created_at,updated_at) VALUES (?,?,? ,?,'editing_session',?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, record.ClipID, actingUserID, reservationKey, int(math.Round(metadata.DurationSeconds*1000)), sourceBytes, int(math.Round(metadata.DurationSeconds*1000)), metadata.VideoStreamIndex, compressed, options.QualityCRF, options.MaxHeight, options.TargetSizeBytes, bypassed, metadata.Container, metadata.VideoCodec, metadata.AudioCodec, metadata.Width, metadata.Height, metadata.FrameRate, metadata.PixelFormat, metadata.Profile, now, now, now); err != nil {
		return EditorSession{}, err
	}
	defaultIndex := firstDefaultUsable(metadata.AudioStreams)
	for _, audio := range metadata.AudioStreams {
		included := 0
		if audio.Usable && audio.Index == defaultIndex {
			included = 1
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO upload_session_audio(session_id,stream_index,codec,channels,channel_layout,sample_rate,language,is_default,usable,unusable_reason,included,gain_db) VALUES(?,?,?,?,?,?,?,?,?,?,?,0)`, id, audio.Index, audio.Codec, audio.Channels, audio.ChannelLayout, audio.SampleRate, audio.Language, boolInt(audio.Default), boolInt(audio.Usable), audio.UnusableReason, included); err != nil {
			return EditorSession{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE storage_reservations SET reserved_bytes = ? WHERE reservation_key = ?`, sourceBytes*2+100_000_000, reservationKey); err != nil {
		return EditorSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return EditorSession{}, err
	}
	return s.EditorSessionByID(ctx, id, actingUserID)
}

func firstDefaultUsable(audio []AudioStream) int {
	for _, a := range audio {
		if a.Usable && a.Default {
			return a.Index
		}
	}
	for _, a := range audio {
		if a.Usable {
			return a.Index
		}
	}
	return -1
}
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (s *Store) EditorSessionByID(ctx context.Context, id string, actorID int64) (EditorSession, error) {
	var session EditorSession
	var compressed, previewRendering int
	err := s.db.QueryRowContext(ctx, `SELECT us.id,us.clip_id,us.state,us.revision,c.title,c.parent_folder_id,c.storage_id,us.source_duration_ms,us.video_stream_index,us.source_size_bytes,us.source_container,us.source_video_codec,us.source_width,us.source_height,us.source_frame_rate,us.compression_requested,us.quality_crf,us.max_height,us.trim_start_ms,us.trim_end_ms,us.preview_state,COALESCE(us.preview_start_ms,0),COALESCE(us.preview_end_ms,0),COALESCE(us.preview_revision,0),us.preview_rendering FROM upload_sessions us JOIN clips c ON c.id=us.clip_id WHERE us.id=? AND us.acting_user_id=?`, id, actorID).Scan(&session.ID, &session.ClipID, &session.State, &session.EditRevision, &session.Title, &session.DestinationFolderID, &session.StorageID, &session.DurationMS, &session.VideoStreamIndex, &session.SourceSizeBytes, &session.SourceContainer, &session.SourceVideoCodec, &session.SourceWidth, &session.SourceHeight, &session.SourceFrameRate, &compressed, &session.QualityCRF, &session.MaxHeight, &session.Edit.TrimStartMS, &session.Edit.TrimEndMS, &session.PreviewState, &session.PreviewStartMS, &session.PreviewEndMS, &session.PreviewRevision, &previewRendering)
	if errors.Is(err, sql.ErrNoRows) {
		return EditorSession{}, ErrEditorSessionNotFound
	}
	if err != nil {
		return EditorSession{}, err
	}
	session.CompressionRequested = compressed == 1
	session.PreviewRendering = previewRendering == 1
	rows, err := s.db.QueryContext(ctx, `SELECT stream_index,codec,channels,COALESCE(channel_layout,''),sample_rate,COALESCE(language,''),is_default,usable,COALESCE(unusable_reason,''),included,gain_db FROM upload_session_audio WHERE session_id=? ORDER BY stream_index`, id)
	if err != nil {
		return EditorSession{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var audio AudioStream
		var def, usable, included int
		if err := rows.Scan(&audio.Index, &audio.Codec, &audio.Channels, &audio.ChannelLayout, &audio.SampleRate, &audio.Language, &def, &usable, &audio.UnusableReason, &included, &audio.GainDB); err != nil {
			return EditorSession{}, err
		}
		audio.Default = def == 1
		audio.Usable = usable == 1
		audio.Included = included == 1
		session.Edit.Audio = append(session.Edit.Audio, audio)
	}
	return session, rows.Err()
}

// ResumableEditorSession returns the recent session reserved by the same user,
// title, and destination. Re-selecting the source in the upload dialog can then
// restore the saved recipe without transferring or duplicating the source.
func (s *Store) ResumableEditorSession(ctx context.Context, actorID, destinationID, sourceSizeBytes int64, normalizedTitle string, now time.Time) (EditorSession, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT us.id FROM upload_sessions us JOIN clips c ON c.id=us.clip_id WHERE us.acting_user_id=? AND us.state='editing_session' AND c.parent_folder_id=? AND c.title_normalized=? AND us.source_size_bytes=? AND us.last_heartbeat_at>=? ORDER BY us.updated_at DESC LIMIT 1`, actorID, destinationID, normalizedTitle, sourceSizeBytes, now.Add(-editorLease).UTC().Format(time.RFC3339Nano)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return EditorSession{}, ErrEditorSessionNotFound
	}
	if err != nil {
		return EditorSession{}, err
	}
	if err := s.HeartbeatEditorSession(ctx, id, actorID); err != nil {
		return EditorSession{}, err
	}
	return s.EditorSessionByID(ctx, id, actorID)
}

func (s *Store) BeginEditorPreview(ctx context.Context, id string, actorID int64, revision, playheadMS int, fullSource bool) (EditorSession, error) {
	session, err := s.EditorSessionByID(ctx, id, actorID)
	if err != nil {
		return EditorSession{}, err
	}
	if session.State != "editing_session" {
		return EditorSession{}, ErrSessionNotEditable
	}
	if revision != session.EditRevision {
		return EditorSession{}, ErrEditConflict
	}
	if session.PreviewRendering {
		return EditorSession{}, ErrPreviewInProgress
	}
	start, end := session.Edit.TrimStartMS, session.Edit.TrimEndMS
	if fullSource {
		start, end = 0, session.DurationMS
	} else if end-start > 60_000 {
		start = playheadMS - 30_000
		if start < session.Edit.TrimStartMS {
			start = session.Edit.TrimStartMS
		}
		end = start + 60_000
		if end > session.Edit.TrimEndMS {
			end = session.Edit.TrimEndMS
			start = end - 60_000
		}
	}
	_, err = s.db.ExecContext(ctx, `UPDATE upload_sessions SET preview_rendering=1,pending_preview_start_ms=?,pending_preview_end_ms=?,pending_preview_revision=?,updated_at=? WHERE id=? AND acting_user_id=? AND state='editing_session' AND revision=? AND preview_rendering=0`, start, end, revision, time.Now().UTC().Format(time.RFC3339Nano), id, actorID, revision)
	if err != nil {
		return EditorSession{}, err
	}
	session.PreviewRendering = true
	// These values describe the pending render to the caller. Stored ready-preview
	// metadata remains unchanged until the pending file is successfully published.
	session.PreviewStartMS = start
	session.PreviewEndMS = end
	session.PreviewRevision = revision
	return session, nil
}

func (s *Store) FinishEditorPreview(ctx context.Context, id string, actorID int64, revision int, succeeded bool) error {
	if succeeded {
		_, err := s.db.ExecContext(ctx, `UPDATE upload_sessions SET preview_state='ready',preview_start_ms=pending_preview_start_ms,preview_end_ms=pending_preview_end_ms,preview_revision=pending_preview_revision,preview_rendering=0,pending_preview_start_ms=NULL,pending_preview_end_ms=NULL,pending_preview_revision=NULL,updated_at=? WHERE id=? AND acting_user_id=? AND state='editing_session' AND pending_preview_revision=?`, time.Now().UTC().Format(time.RFC3339Nano), id, actorID, revision)
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE upload_sessions SET preview_state=CASE WHEN preview_state='ready' THEN 'ready' ELSE 'failed' END,preview_rendering=0,pending_preview_start_ms=NULL,pending_preview_end_ms=NULL,pending_preview_revision=NULL,updated_at=? WHERE id=? AND acting_user_id=? AND state='editing_session' AND pending_preview_revision=?`, time.Now().UTC().Format(time.RFC3339Nano), id, actorID, revision)
	return err
}

func (s *Store) SaveEditorRecipe(ctx context.Context, id string, actorID int64, revision int, recipe EditorRecipe) (EditorSession, error) {
	if err := validateRecipe(recipe); err != nil {
		return EditorSession{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EditorSession{}, err
	}
	defer tx.Rollback()
	var duration int
	var state string
	if err := tx.QueryRowContext(ctx, `SELECT source_duration_ms,state FROM upload_sessions WHERE id=? AND acting_user_id=?`, id, actorID).Scan(&duration, &state); errors.Is(err, sql.ErrNoRows) {
		return EditorSession{}, ErrEditorSessionNotFound
	} else if err != nil {
		return EditorSession{}, err
	}
	if state != "editing_session" {
		return EditorSession{}, ErrSessionNotEditable
	}
	if recipe.TrimEndMS > duration {
		return EditorSession{}, ErrInvalidTrim
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `UPDATE upload_sessions SET revision=revision+1,trim_start_ms=?,trim_end_ms=?,last_heartbeat_at=?,updated_at=? WHERE id=? AND acting_user_id=? AND revision=? AND state='editing_session'`, recipe.TrimStartMS, recipe.TrimEndMS, now, now, id, actorID, revision)
	if err != nil {
		return EditorSession{}, err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return EditorSession{}, ErrEditConflict
	}
	for _, audio := range recipe.Audio {
		result, err := tx.ExecContext(ctx, `UPDATE upload_session_audio SET included=?,gain_db=? WHERE session_id=? AND stream_index=? AND usable=1`, boolInt(audio.Included), audio.GainDB, id, audio.Index)
		if err != nil {
			return EditorSession{}, err
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			return EditorSession{}, ErrAudioTrackUnusable
		}
	}
	if err := tx.Commit(); err != nil {
		return EditorSession{}, err
	}
	return s.EditorSessionByID(ctx, id, actorID)
}

func (s *Store) UpdateEditorMetadata(ctx context.Context, id string, actorID int64, revision int, title, normalized string, destinationID int64, requiredOwnerID *int64) (EditorSession, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EditorSession{}, err
	}
	defer tx.Rollback()
	var clipID, currentRevision int64
	var state string
	if err := tx.QueryRowContext(ctx, `SELECT us.clip_id,us.revision,us.state FROM upload_sessions us WHERE us.id=? AND us.acting_user_id=?`, id, actorID).Scan(&clipID, &currentRevision, &state); errors.Is(err, sql.ErrNoRows) {
		return EditorSession{}, ErrEditorSessionNotFound
	} else if err != nil {
		return EditorSession{}, err
	}
	if state != "editing_session" {
		return EditorSession{}, ErrSessionNotEditable
	}
	if int(currentRevision) != revision {
		return EditorSession{}, ErrEditConflict
	}
	destination, err := folderByID(ctx, tx, destinationID)
	if err != nil {
		return EditorSession{}, err
	}
	if requiredOwnerID != nil && destination.OwnerUserID != *requiredOwnerID {
		return EditorSession{}, ErrFolderNotFound
	}
	var conflicts int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM clips WHERE parent_folder_id=? AND title_normalized=? AND id!=? AND deleted_at IS NULL AND state NOT IN ('failed','cancelled')`, destinationID, normalized, clipID).Scan(&conflicts); err != nil {
		return EditorSession{}, err
	}
	if conflicts != 0 {
		return EditorSession{}, ErrClipTitleTaken
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `UPDATE clips SET title=?,title_normalized=?,parent_folder_id=?,owner_user_id=?,updated_at=? WHERE id=? AND state='uploading'`, title, normalized, destinationID, destination.OwnerUserID, now, clipID); err != nil {
		return EditorSession{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE upload_sessions SET revision=revision+1,last_heartbeat_at=?,updated_at=? WHERE id=?`, now, now, id); err != nil {
		return EditorSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return EditorSession{}, err
	}
	return s.EditorSessionByID(ctx, id, actorID)
}

func validateRecipe(recipe EditorRecipe) error {
	if recipe.TrimStartMS < 0 || recipe.TrimEndMS-recipe.TrimStartMS < 250 {
		return ErrInvalidTrim
	}
	seen := map[int]bool{}
	for _, audio := range recipe.Audio {
		if seen[audio.Index] || math.IsNaN(audio.GainDB) || math.IsInf(audio.GainDB, 0) || audio.GainDB < -60 || audio.GainDB > 12 || math.Abs(audio.GainDB*2-math.Round(audio.GainDB*2)) > .000001 {
			return ErrInvalidAudioGain
		}
		seen[audio.Index] = true
	}
	return nil
}

func (s *Store) HeartbeatEditorSession(ctx context.Context, id string, actorID int64) error {
	result, err := s.db.ExecContext(ctx, `UPDATE upload_sessions SET last_heartbeat_at=?,updated_at=? WHERE id=? AND acting_user_id=? AND state='editing_session'`, time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), id, actorID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return ErrEditorSessionNotFound
	}
	return nil
}

func (s *Store) DiscardEditorSession(ctx context.Context, id string, actorID int64) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var storage, reservation string
	err = tx.QueryRowContext(ctx, `SELECT c.storage_id,us.reservation_key FROM upload_sessions us JOIN clips c ON c.id=us.clip_id WHERE us.id=? AND us.acting_user_id=?`, id, actorID).Scan(&storage, &reservation)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM storage_reservations WHERE reservation_key=?`, reservation); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM clips WHERE id=(SELECT clip_id FROM upload_sessions WHERE id=?)`, id); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return storage, nil
}

// FinalizeEditorSession snapshots an immutable recipe into the ordinary job queue.
func (s *Store) FinalizeEditorSession(ctx context.Context, id string, actorID int64, revision int) (Job, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback()
	var clipID int64
	var state string
	var currentRevision, start, end, duration int
	err = tx.QueryRowContext(ctx, `SELECT clip_id,state,revision,trim_start_ms,trim_end_ms,source_duration_ms FROM upload_sessions WHERE id=? AND acting_user_id=?`, id, actorID).Scan(&clipID, &state, &currentRevision, &start, &end, &duration)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrEditorSessionNotFound
	}
	if err != nil {
		return Job{}, err
	}
	if state == "finalizing" {
		var job Job
		err = tx.QueryRowContext(ctx, `SELECT j.id,j.clip_id,c.owner_user_id,j.state,j.progress,j.error_code,j.error_message FROM jobs j JOIN clips c ON c.id=j.clip_id JOIN upload_sessions us ON us.final_job_id=j.id WHERE us.id=?`, id).Scan(&job.ID, &job.ClipID, &job.OwnerUserID, &job.State, &job.Progress, &job.ErrorCode, &job.ErrorMessage)
		if err == nil && revision == currentRevision {
			return job, nil
		}
		return Job{}, ErrEditConflict
	}
	if state != "editing_session" {
		return Job{}, ErrSessionNotEditable
	}
	if revision != currentRevision {
		return Job{}, ErrEditConflict
	}
	if start < 0 || end-start < 250 || end > duration {
		return Job{}, ErrInvalidTrim
	}
	rows, err := tx.QueryContext(ctx, `SELECT stream_index,codec,channels,COALESCE(channel_layout,''),sample_rate,COALESCE(language,''),is_default,usable,COALESCE(unusable_reason,''),included,gain_db FROM upload_session_audio WHERE session_id=? ORDER BY stream_index`, id)
	if err != nil {
		return Job{}, err
	}
	defer rows.Close()
	var audio []AudioStream
	for rows.Next() {
		var a AudioStream
		var d, u, included int
		if err := rows.Scan(&a.Index, &a.Codec, &a.Channels, &a.ChannelLayout, &a.SampleRate, &a.Language, &d, &u, &a.UnusableReason, &included, &a.GainDB); err != nil {
			return Job{}, err
		}
		a.Default = d == 1
		a.Usable = u == 1
		a.Included = included == 1
		audio = append(audio, a)
	}
	if err := rows.Err(); err != nil {
		return Job{}, err
	}
	recipe, err := json.Marshal(sortedAudio(audio))
	if err != nil {
		return Job{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `INSERT INTO jobs(clip_id,state,progress,compression_requested,quality_crf,max_height,source_size_bytes,source_container,source_video_codec,source_audio_codec,source_duration_seconds,source_width,source_height,source_frame_rate,source_pixel_format,source_profile,target_size_bytes,stored_limit_bypassed,trim_start_ms,trim_end_ms,audio_recipe_json,created_at,updated_at) SELECT clip_id,'queued',0,compression_requested,quality_crf,max_height,source_size_bytes,source_container,source_video_codec,source_audio_codec,source_duration_ms/1000.0,source_width,source_height,source_frame_rate,source_pixel_format,source_profile,target_size_bytes,stored_limit_bypassed,trim_start_ms,trim_end_ms,?,?,? FROM upload_sessions WHERE id=?`, string(recipe), now, now, id)
	if err != nil {
		return Job{}, err
	}
	jobID, err := result.LastInsertId()
	if err != nil {
		return Job{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE clips SET state='queued',updated_at=? WHERE id=? AND state='uploading'`, now, clipID); err != nil {
		return Job{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE storage_reservations SET job_id=?,reserved_bytes=(SELECT source_size_bytes*2 FROM upload_sessions WHERE id=?) WHERE reservation_key=(SELECT reservation_key FROM upload_sessions WHERE id=?)`, jobID, id, id); err != nil {
		return Job{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE upload_sessions SET state='finalizing',final_job_id=?,updated_at=? WHERE id=?`, jobID, now, id); err != nil {
		return Job{}, err
	}
	if err := tx.Commit(); err != nil {
		return Job{}, err
	}
	return Job{ID: jobID, ClipID: clipID, OwnerUserID: actorID, State: "queued"}, nil
}

func (s *Store) ExpiredEditorStorageIDs(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.storage_id FROM upload_sessions us JOIN clips c ON c.id=us.clip_id WHERE us.state='editing_session' AND us.last_heartbeat_at < ?`, now.Add(-editorLease).UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// RecoverInterruptedEditorPreviews clears render locks left by a stopped process
// without discarding the editor or its last complete preview.
func (s *Store) RecoverInterruptedEditorPreviews(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.storage_id FROM upload_sessions us JOIN clips c ON c.id=us.clip_id WHERE us.state='editing_session' AND us.preview_rendering=1`)
	if err != nil {
		return nil, err
	}
	var storageIDs []string
	for rows.Next() {
		var storageID string
		if err := rows.Scan(&storageID); err != nil {
			rows.Close()
			return nil, err
		}
		storageIDs = append(storageIDs, storageID)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE upload_sessions SET preview_state=CASE WHEN preview_state='ready' THEN 'ready' ELSE 'failed' END,preview_rendering=0,pending_preview_start_ms=NULL,pending_preview_end_ms=NULL,pending_preview_revision=NULL,updated_at=? WHERE state='editing_session' AND preview_rendering=1`, time.Now().UTC().Format(time.RFC3339Nano))
	return storageIDs, err
}

func (s *Store) ExpireEditorSessions(ctx context.Context, now time.Time) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	cutoff := now.Add(-editorLease).UTC().Format(time.RFC3339Nano)
	rows, err := tx.QueryContext(ctx, `SELECT us.id,c.id,c.storage_id FROM upload_sessions us JOIN clips c ON c.id=us.clip_id WHERE us.state='editing_session' AND us.last_heartbeat_at < ?`, cutoff)
	if err != nil {
		return nil, err
	}
	type expiredSession struct {
		id      string
		clipID  int64
		storage string
	}
	var expired []expiredSession
	for rows.Next() {
		var item expiredSession
		if err := rows.Scan(&item.id, &item.clipID, &item.storage); err != nil {
			rows.Close()
			return nil, err
		}
		expired = append(expired, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var removed []string
	for _, item := range expired {
		result, err := tx.ExecContext(ctx, `DELETE FROM clips WHERE id=? AND EXISTS (SELECT 1 FROM upload_sessions WHERE id=? AND state='editing_session' AND last_heartbeat_at < ?)`, item.clipID, item.id, cutoff)
		if err != nil {
			return nil, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if count == 1 {
			removed = append(removed, item.storage)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return removed, nil
}

func sortedAudio(audio []AudioStream) []AudioStream {
	result := append([]AudioStream(nil), audio...)
	sort.Slice(result, func(i, j int) bool { return result[i].Index < result[j].Index })
	return result
}

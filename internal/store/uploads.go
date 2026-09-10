package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var (
	ErrClipTitleTaken      = errors.New("clip title is already in use")
	ErrInsufficientStorage = errors.New("insufficient temporary storage")
	ErrJobNotFound         = errors.New("job not found")
)

func (s *Store) RenameClip(ctx context.Context, clipID int64, title, normalized string, requiredOwnerID *int64) error {
	var owner, parent int64
	if err := s.db.QueryRowContext(ctx, `SELECT owner_user_id, parent_folder_id FROM clips WHERE id = ? AND deleted_at IS NULL`, clipID).Scan(&owner, &parent); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrClipNotFound
		}
		return err
	}
	if requiredOwnerID != nil && owner != *requiredOwnerID {
		return ErrClipNotFound
	}
	var conflicts int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM clips WHERE parent_folder_id = ? AND title_normalized = ? AND id != ? AND deleted_at IS NULL AND state NOT IN ('failed', 'cancelled')`, parent, normalized, clipID).Scan(&conflicts); err != nil {
		return err
	}
	if conflicts != 0 {
		return ErrClipTitleTaken
	}
	_, err := s.db.ExecContext(ctx, `UPDATE clips SET title = ?, title_normalized = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, title, normalized, time.Now().UTC().Format(time.RFC3339Nano), clipID)
	if err != nil {
		return classifyClipConstraint(err)
	}
	return nil
}

func (s *Store) TrashClip(ctx context.Context, clipID int64, requiredOwnerID *int64) error {
	var owner int64
	if err := s.db.QueryRowContext(ctx, `SELECT owner_user_id FROM clips WHERE id = ? AND deleted_at IS NULL AND state = 'ready'`, clipID).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrClipNotFound
		}
		return err
	}
	if requiredOwnerID != nil && owner != *requiredOwnerID {
		return ErrClipNotFound
	}
	_, err := s.db.ExecContext(ctx, `UPDATE clips SET deleted_at = ?, original_parent_folder_id = parent_folder_id, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), clipID)
	return err
}

func (s *Store) MoveClip(ctx context.Context, clipID, destinationID int64, requiredOwnerID *int64) error {
	var owner int64
	if err := s.db.QueryRowContext(ctx, `SELECT owner_user_id FROM clips WHERE id=? AND deleted_at IS NULL`, clipID).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrClipNotFound
		}
		return err
	}
	if requiredOwnerID != nil && owner != *requiredOwnerID {
		return ErrClipNotFound
	}
	var destOwner int64
	if err := s.db.QueryRowContext(ctx, `SELECT owner_user_id FROM folders WHERE id=? AND deleted_at IS NULL`, destinationID).Scan(&destOwner); err != nil {
		return ErrFolderNotFound
	}
	if requiredOwnerID != nil && destOwner != *requiredOwnerID {
		return ErrClipNotFound
	}
	var normalized string
	if err := s.db.QueryRowContext(ctx, `SELECT title_normalized FROM clips WHERE id = ?`, clipID).Scan(&normalized); err != nil {
		return err
	}
	var conflicts int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM clips WHERE parent_folder_id = ? AND title_normalized = ? AND id != ? AND deleted_at IS NULL AND state NOT IN ('failed', 'cancelled')`, destinationID, normalized, clipID).Scan(&conflicts); err != nil {
		return err
	}
	if conflicts != 0 {
		return ErrClipTitleTaken
	}
	_, err := s.db.ExecContext(ctx, `UPDATE clips SET parent_folder_id=?, owner_user_id=?, updated_at=? WHERE id=? AND deleted_at IS NULL`, destinationID, destOwner, time.Now().UTC().Format(time.RFC3339Nano), clipID)
	if err != nil {
		return classifyClipConstraint(err)
	}
	return nil
}

type UploadRecord struct {
	ClipID      int64
	OwnerUserID int64
	StorageID   string
	PublicID    string
	Title       string
}

type ProbeMetadata struct {
	Container        string
	VideoCodec       string
	AudioCodec       string
	DurationSeconds  float64
	Width            int
	Height           int
	FrameRate        float64
	PixelFormat      string
	Profile          string
	VideoStreamIndex int
	AudioStreams     []AudioStream
}

type UploadOptions struct {
	CompressionRequested bool
	QualityCRF           int
	MaxHeight            int
	TargetSizeBytes      int64
	StoredLimitBypassed  bool
}

type Job struct {
	ID           int64   `json:"id"`
	ClipID       int64   `json:"clipId"`
	OwnerUserID  int64   `json:"ownerUserId"`
	State        string  `json:"state"`
	Progress     int     `json:"progress"`
	ErrorCode    *string `json:"errorCode"`
	ErrorMessage *string `json:"errorMessage"`
}

func (s *Store) BeginUpload(ctx context.Context, parentID int64, title, normalized, publicID, storageID, reservationKey string, reservedBytes int64, minimumFreeBytes uint64, availableBytes uint64, requiredOwnerID *int64) (UploadRecord, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return UploadRecord{}, err
	}
	defer tx.Rollback()
	parent, err := folderByID(ctx, tx, parentID)
	if err != nil {
		return UploadRecord{}, err
	}
	if requiredOwnerID != nil && parent.OwnerUserID != *requiredOwnerID {
		return UploadRecord{}, ErrFolderNotFound
	}
	var activeReserved int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(reserved_bytes), 0) FROM storage_reservations`).Scan(&activeReserved); err != nil {
		return UploadRecord{}, err
	}
	if availableBytes < minimumFreeBytes || uint64(activeReserved)+uint64(reservedBytes) > availableBytes-minimumFreeBytes {
		return UploadRecord{}, ErrInsufficientStorage
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `INSERT INTO clips(owner_user_id, parent_folder_id, public_id, storage_id, title, title_normalized, state, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 'uploading', ?, ?)`, parent.OwnerUserID, parentID, publicID, storageID, title, normalized, now, now)
	if err != nil {
		return UploadRecord{}, classifyClipConstraint(err)
	}
	clipID, err := result.LastInsertId()
	if err != nil {
		return UploadRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO storage_reservations(reservation_key, reserved_bytes, created_at) VALUES (?, ?, ?)`, reservationKey, reservedBytes, now); err != nil {
		return UploadRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return UploadRecord{}, err
	}
	return UploadRecord{ClipID: clipID, OwnerUserID: parent.OwnerUserID, StorageID: storageID, PublicID: publicID, Title: title}, nil
}

func (s *Store) CompleteUpload(ctx context.Context, record UploadRecord, reservationKey string, sourceBytes int64, metadata ProbeMetadata, options UploadOptions) (Job, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	compression := 0
	if options.CompressionRequested {
		compression = 1
	}
	bypassed := 0
	if options.StoredLimitBypassed {
		bypassed = 1
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO jobs(clip_id, state, progress, compression_requested, quality_crf, max_height, source_size_bytes, source_container, source_video_codec, source_audio_codec, source_duration_seconds, source_width, source_height, source_frame_rate, source_pixel_format, source_profile, target_size_bytes, stored_limit_bypassed, created_at, updated_at) VALUES (?, 'queued', 0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, record.ClipID, compression, options.QualityCRF, options.MaxHeight, sourceBytes, metadata.Container, metadata.VideoCodec, metadata.AudioCodec, metadata.DurationSeconds, metadata.Width, metadata.Height, metadata.FrameRate, metadata.PixelFormat, metadata.Profile, options.TargetSizeBytes, bypassed, now, now)
	if err != nil {
		return Job{}, err
	}
	jobID, err := result.LastInsertId()
	if err != nil {
		return Job{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE clips SET state = 'queued', duration_seconds = ?, width = ?, height = ?, frame_rate = ?, updated_at = ? WHERE id = ? AND state = 'uploading'`, metadata.DurationSeconds, metadata.Width, metadata.Height, metadata.FrameRate, now, record.ClipID); err != nil {
		return Job{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE storage_reservations SET job_id = ?, reserved_bytes = ? WHERE reservation_key = ?`, jobID, sourceBytes*2, reservationKey); err != nil {
		return Job{}, err
	}
	if err := tx.Commit(); err != nil {
		return Job{}, err
	}
	return Job{ID: jobID, ClipID: record.ClipID, OwnerUserID: record.OwnerUserID, State: "queued", Progress: 0}, nil
}

func (s *Store) AbortUpload(ctx context.Context, clipID int64, reservationKey string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM storage_reservations WHERE reservation_key = ?`, reservationKey); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM clips WHERE id = ? AND state = 'uploading'`, clipID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) JobByID(ctx context.Context, id int64) (Job, error) {
	var job Job
	err := s.db.QueryRowContext(ctx, `SELECT j.id, j.clip_id, c.owner_user_id, j.state, j.progress, j.error_code, j.error_message FROM jobs j JOIN clips c ON c.id = j.clip_id WHERE j.id = ?`, id).Scan(&job.ID, &job.ClipID, &job.OwnerUserID, &job.State, &job.Progress, &job.ErrorCode, &job.ErrorMessage)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrJobNotFound
	}
	return job, err
}

func classifyClipConstraint(err error) error {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "clips.parent_folder_id") && strings.Contains(message, "clips.title_normalized") {
		return ErrClipTitleTaken
	}
	return err
}

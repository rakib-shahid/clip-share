package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var ErrNoQueuedJob = errors.New("no queued job")

type ProcessingJob struct {
	ID                   int64
	ClipID               int64
	StorageID            string
	CompressionRequested bool
	QualityCRF           int
	MaxHeight            int
	SourceSizeBytes      int64
	Source               ProbeMetadata
	TargetSizeBytes      int64
	StoredLimitBypassed  bool
	TrimStartMS          int
	TrimEndMS            int
	AudioRecipe          []AudioStream
	UploadStartedAt      time.Time
}

func (s *Store) RecoverInterruptedJobs(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `UPDATE jobs SET state = 'queued', progress = 0, claimed_at = NULL, updated_at = ? WHERE state = 'processing'`, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE clips SET state = 'queued', updated_at = ? WHERE state = 'processing'`, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ClaimNextJob(ctx context.Context) (ProcessingJob, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ProcessingJob{}, err
	}
	defer tx.Rollback()
	var job ProcessingJob
	var compression, bypassed int
	var uploadStartedAt string
	var audioRecipe string
	err = tx.QueryRowContext(ctx, `SELECT j.id, j.clip_id, c.storage_id, j.compression_requested, j.quality_crf, j.max_height, j.source_size_bytes, j.source_container, j.source_video_codec, j.source_audio_codec, j.source_duration_seconds, j.source_width, j.source_height, j.source_frame_rate, j.source_pixel_format, j.source_profile, j.target_size_bytes, j.stored_limit_bypassed, j.trim_start_ms, COALESCE(j.trim_end_ms, 0), COALESCE(j.audio_recipe_json, ''), c.created_at FROM jobs j JOIN clips c ON c.id = j.clip_id WHERE j.state = 'queued' AND c.state = 'queued' ORDER BY j.created_at, j.id LIMIT 1`).Scan(&job.ID, &job.ClipID, &job.StorageID, &compression, &job.QualityCRF, &job.MaxHeight, &job.SourceSizeBytes, &job.Source.Container, &job.Source.VideoCodec, &job.Source.AudioCodec, &job.Source.DurationSeconds, &job.Source.Width, &job.Source.Height, &job.Source.FrameRate, &job.Source.PixelFormat, &job.Source.Profile, &job.TargetSizeBytes, &bypassed, &job.TrimStartMS, &job.TrimEndMS, &audioRecipe, &uploadStartedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ProcessingJob{}, ErrNoQueuedJob
	}
	if err != nil {
		return ProcessingJob{}, err
	}
	job.UploadStartedAt, err = time.Parse(time.RFC3339Nano, uploadStartedAt)
	if err != nil {
		return ProcessingJob{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `UPDATE jobs SET state = 'processing', progress = 1, claimed_at = ?, updated_at = ? WHERE id = ? AND state = 'queued'`, now, now, job.ID)
	if err != nil {
		return ProcessingJob{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return ProcessingJob{}, ErrNoQueuedJob
	}
	if _, err := tx.ExecContext(ctx, `UPDATE clips SET state = 'processing', updated_at = ? WHERE id = ?`, now, job.ClipID); err != nil {
		return ProcessingJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return ProcessingJob{}, err
	}
	job.CompressionRequested = compression == 1
	job.StoredLimitBypassed = bypassed == 1
	if audioRecipe != "" {
		if err := json.Unmarshal([]byte(audioRecipe), &job.AudioRecipe); err != nil {
			return ProcessingJob{}, err
		}
	}
	if job.TrimEndMS > job.TrimStartMS {
		job.Source.DurationSeconds = float64(job.TrimEndMS-job.TrimStartMS) / 1000
	}
	return job, nil
}

func (s *Store) UpdateJobProgress(ctx context.Context, jobID int64, progress int) error {
	if progress < 1 {
		progress = 1
	}
	if progress > 99 {
		progress = 99
	}
	result, err := s.db.ExecContext(ctx, `UPDATE jobs SET progress = ?, updated_at = ? WHERE id = ? AND state = 'processing'`, progress, time.Now().UTC().Format(time.RFC3339Nano), jobID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return inactiveJobError(ctx, s.db, jobID)
	}
	return nil
}

func (s *Store) CompleteProcessing(ctx context.Context, job ProcessingJob, sizeBytes int64, metadata ProbeMetadata) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `UPDATE clips SET state = 'ready', size_bytes = ?, duration_seconds = ?, width = ?, height = ?, frame_rate = ?, updated_at = ? WHERE id = ? AND state = 'processing'`, sizeBytes, metadata.DurationSeconds, metadata.Width, metadata.Height, metadata.FrameRate, now, job.ClipID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return inactiveJobError(ctx, tx, job.ID)
	}
	result, err = tx.ExecContext(ctx, `UPDATE jobs SET state = 'ready', progress = 100, updated_at = ? WHERE id = ? AND state = 'processing'`, now, job.ID)
	if err != nil {
		return err
	}
	changed, err = result.RowsAffected()
	if err != nil || changed != 1 {
		return ErrJobNotActive
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM storage_reservations WHERE job_id = ?`, job.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM upload_sessions WHERE final_job_id = ?`, job.ID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) FailProcessing(ctx context.Context, job ProcessingJob, code, message string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var editorJob int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM upload_sessions WHERE final_job_id = ?`, job.ID).Scan(&editorJob); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `UPDATE clips SET state = 'failed', updated_at = ? WHERE id = ? AND state = 'processing'`, now, job.ClipID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return inactiveJobError(ctx, tx, job.ID)
	}
	result, err = tx.ExecContext(ctx, `UPDATE jobs SET state = 'failed', error_code = ?, error_message = ?, updated_at = ? WHERE id = ? AND state = 'processing'`, code, message, now, job.ID)
	if err != nil {
		return err
	}
	changed, err = result.RowsAffected()
	if err != nil || changed != 1 {
		return ErrJobNotActive
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM storage_reservations WHERE job_id = ?`, job.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM upload_sessions WHERE final_job_id = ?`, job.ID); err != nil {
		return err
	}
	if editorJob != 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM jobs WHERE id = ?`, job.ID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM clips WHERE id = ?`, job.ClipID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

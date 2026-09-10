package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrJobNotCancellable = errors.New("job is not cancellable")
	ErrJobNotFailed      = errors.New("job is not failed")
	ErrJobCancelled      = errors.New("job was cancelled")
	ErrJobNotActive      = errors.New("job is not active")
)

type JobAssets struct {
	JobID       int64
	ClipID      int64
	OwnerUserID int64
	StorageID   string
	State       string
}

// CancelJob commits the durable cancelled state before filesystem cleanup. An
// already-cancelled job is returned again so callers can safely retry cleanup.
func (s *Store) CancelJob(ctx context.Context, jobID int64, requiredOwnerID *int64) (JobAssets, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return JobAssets{}, err
	}
	defer tx.Rollback()
	assets, err := jobAssetsByID(ctx, tx, jobID)
	if err != nil {
		return JobAssets{}, err
	}
	if requiredOwnerID != nil && assets.OwnerUserID != *requiredOwnerID {
		return JobAssets{}, ErrJobNotFound
	}
	if assets.State == "cancelled" {
		return assets, nil
	}
	if assets.State != "queued" && assets.State != "validating" && assets.State != "processing" {
		return JobAssets{}, ErrJobNotCancellable
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `UPDATE jobs SET state = 'cancelled', claimed_at = NULL, updated_at = ? WHERE id = ? AND state IN ('queued','validating','processing')`, now, jobID)
	if err != nil {
		return JobAssets{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return JobAssets{}, ErrJobNotCancellable
	}
	if _, err := tx.ExecContext(ctx, `UPDATE clips SET state = 'cancelled', updated_at = ? WHERE id = ? AND state IN ('queued','validating','processing')`, now, assets.ClipID); err != nil {
		return JobAssets{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM storage_reservations WHERE job_id = ?`, jobID); err != nil {
		return JobAssets{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM upload_sessions WHERE final_job_id = ?`, jobID); err != nil {
		return JobAssets{}, err
	}
	if err := tx.Commit(); err != nil {
		return JobAssets{}, err
	}
	assets.State = "cancelled"
	return assets, nil
}

func (s *Store) DismissFailedJob(ctx context.Context, jobID int64, requiredOwnerID *int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	assets, err := jobAssetsByID(ctx, tx, jobID)
	if err != nil {
		return err
	}
	if requiredOwnerID != nil && assets.OwnerUserID != *requiredOwnerID {
		return ErrJobNotFound
	}
	if assets.State != "failed" {
		return ErrJobNotFailed
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM storage_reservations WHERE job_id = ?`, jobID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM jobs WHERE id = ? AND state = 'failed'`, jobID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM clips WHERE id = ? AND state = 'failed'`, assets.ClipID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return ErrJobNotFailed
	}
	return tx.Commit()
}

func jobAssetsByID(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, jobID int64) (JobAssets, error) {
	var assets JobAssets
	err := queryer.QueryRowContext(ctx, `SELECT j.id, j.clip_id, c.owner_user_id, c.storage_id, j.state FROM jobs j JOIN clips c ON c.id = j.clip_id WHERE j.id = ?`, jobID).
		Scan(&assets.JobID, &assets.ClipID, &assets.OwnerUserID, &assets.StorageID, &assets.State)
	if errors.Is(err, sql.ErrNoRows) {
		return JobAssets{}, ErrJobNotFound
	}
	return assets, err
}

func inactiveJobError(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, jobID int64) error {
	var state string
	err := queryer.QueryRowContext(ctx, `SELECT state FROM jobs WHERE id = ?`, jobID).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrJobNotFound
	}
	if err != nil {
		return err
	}
	if state == "cancelled" {
		return ErrJobCancelled
	}
	return ErrJobNotActive
}

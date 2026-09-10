package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClaimNextJobCarriesOriginalUploadDeadlineAcrossRecovery(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "deadline.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	admin, err := database.CreateAdmin(ctx, "Admin", "admin", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	job := createJobForControlTest(t, database, admin, "Deadline", "deadline")
	originalStart := time.Now().UTC().Add(-59 * time.Minute).Truncate(time.Microsecond)
	if _, err := database.db.ExecContext(ctx, `UPDATE clips SET created_at = ? WHERE id = ?`, originalStart.Format(time.RFC3339Nano), job.ClipID); err != nil {
		t.Fatal(err)
	}
	claimed, err := database.ClaimNextJob(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !claimed.UploadStartedAt.Equal(originalStart) {
		t.Fatalf("upload start=%s want=%s", claimed.UploadStartedAt, originalStart)
	}
	if err := database.RecoverInterruptedJobs(ctx); err != nil {
		t.Fatal(err)
	}
	reclaimed, err := database.ClaimNextJob(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reclaimed.UploadStartedAt.Equal(originalStart) {
		t.Fatalf("recovered upload start=%s want=%s", reclaimed.UploadStartedAt, originalStart)
	}
}

func TestJobCancellationIsAuthorizedIdempotentAndStopsCompletion(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	admin, err := database.CreateAdmin(ctx, "Admin", "admin", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	alice, err := database.CreateUser(ctx, "Alice", "alice", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}

	queued := createJobForControlTest(t, database, alice, "Queued", "queued")
	if _, err := database.CancelJob(ctx, queued.ID, &admin.ID); !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("cross-owner cancel error = %v", err)
	}
	assets, err := database.CancelJob(ctx, queued.ID, &alice.ID)
	if err != nil || assets.State != "cancelled" || assets.OwnerUserID != alice.ID {
		t.Fatalf("cancel assets=%+v err=%v", assets, err)
	}
	if _, err := database.CancelJob(ctx, queued.ID, &alice.ID); err != nil {
		t.Fatalf("idempotent cancel: %v", err)
	}
	stored, err := database.JobByID(ctx, queued.ID)
	if err != nil || stored.State != "cancelled" {
		t.Fatalf("stored job=%+v err=%v", stored, err)
	}
	var reservations int
	if err := database.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM storage_reservations WHERE job_id = ?`, queued.ID).Scan(&reservations); err != nil || reservations != 0 {
		t.Fatalf("reservations=%d err=%v", reservations, err)
	}

	processingJob := createJobForControlTest(t, database, alice, "Processing", "processing")
	processing, err := database.ClaimNextJob(ctx)
	if err != nil || processing.ID != processingJob.ID {
		t.Fatalf("claimed=%+v err=%v", processing, err)
	}
	if _, err := database.CancelJob(ctx, processing.ID, &alice.ID); err != nil {
		t.Fatal(err)
	}
	if err := database.UpdateJobProgress(ctx, processing.ID, 50); !errors.Is(err, ErrJobCancelled) {
		t.Fatalf("progress after cancellation error = %v", err)
	}
	metadata := ProbeMetadata{DurationSeconds: 1, Width: 640, Height: 360, FrameRate: 30}
	if err := database.CompleteProcessing(ctx, processing, 100, metadata); !errors.Is(err, ErrJobCancelled) {
		t.Fatalf("complete after cancellation error = %v", err)
	}
	if err := database.FailProcessing(ctx, processing, "processing_failed", "failed"); !errors.Is(err, ErrJobCancelled) {
		t.Fatalf("failure after cancellation error = %v", err)
	}
}

func TestFailedJobDismissalDeletesOnlyFailedNotice(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "failed.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	admin, err := database.CreateAdmin(ctx, "Admin", "admin", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	alice, err := database.CreateUser(ctx, "Alice", "alice", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}

	failedJob := createJobForControlTest(t, database, alice, "Broken", "failed")
	processing, err := database.ClaimNextJob(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.FailProcessing(ctx, processing, "processing_failed", "Could not process video."); err != nil {
		t.Fatal(err)
	}
	if err := database.DismissFailedJob(ctx, failedJob.ID, &admin.ID); !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("cross-owner dismiss error = %v", err)
	}
	if _, err := database.CancelJob(ctx, failedJob.ID, &alice.ID); !errors.Is(err, ErrJobNotCancellable) {
		t.Fatalf("failed cancel error = %v", err)
	}
	if err := database.DismissFailedJob(ctx, failedJob.ID, &alice.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.JobByID(ctx, failedJob.ID); !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("dismissed job lookup error = %v", err)
	}
	var clips int
	if err := database.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM clips WHERE id = ?`, failedJob.ClipID).Scan(&clips); err != nil || clips != 0 {
		t.Fatalf("failed clip count=%d err=%v", clips, err)
	}
	if err := database.DismissFailedJob(ctx, failedJob.ID, &alice.ID); !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("second dismiss error = %v", err)
	}
}

func createJobForControlTest(t *testing.T, database *Store, owner User, title, suffix string) Job {
	t.Helper()
	ctx := context.Background()
	record, err := database.BeginUpload(ctx, owner.RootFolderID, title, strings.ToLower(title), "job-public-"+suffix, "job-storage-"+suffix, "job-reservation-"+suffix, 2_000, 0, 1_000_000, &owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	job, err := database.CompleteUpload(ctx, record, "job-reservation-"+suffix, 1_000, ProbeMetadata{Container: "mp4", VideoCodec: "h264", DurationSeconds: 1, Width: 640, Height: 360, FrameRate: 30}, UploadOptions{QualityCRF: 24, MaxHeight: 720, TargetSizeBytes: owner.StoredFileLimitBytes})
	if err != nil {
		t.Fatal(err)
	}
	return job
}

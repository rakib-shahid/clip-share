package media

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"clip-share/internal/store"
)

func TestProcessorFailsAndCleansAnExpiredUpload(t *testing.T) {
	dataDir := t.TempDir()
	for _, name := range []string{"database", "temporary", "media"} {
		if err := os.MkdirAll(filepath.Join(dataDir, name), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	database, err := store.Open(filepath.Join(dataDir, "database", "timeout.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	admin, err := database.CreateAdmin(ctx, "Admin", "admin", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	storageID := "timeout-storage"
	temporaryDir := filepath.Join(dataDir, "temporary", storageID)
	if err := os.MkdirAll(temporaryDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(temporaryDir, "source"), []byte("partial source"), 0o640); err != nil {
		t.Fatal(err)
	}
	record, err := database.BeginUpload(ctx, admin.RootFolderID, "Timeout", "timeout", "timeout-public", storageID, "timeout-reservation", 2_000, 0, 1_000_000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.CompleteUpload(ctx, record, "timeout-reservation", 1_000, store.ProbeMetadata{Container: "mp4", VideoCodec: "h264", DurationSeconds: 10, Width: 640, Height: 360, FrameRate: 30}, store.UploadOptions{QualityCRF: 24, MaxHeight: 720, TargetSizeBytes: admin.StoredFileLimitBytes, StoredLimitBypassed: true}); err != nil {
		t.Fatal(err)
	}
	job, err := database.ClaimNextJob(ctx)
	if err != nil {
		t.Fatal(err)
	}
	job.UploadStartedAt = time.Now().Add(-2 * maximumUploadLifetime)
	jobContext, cancel := processingContext(ctx, job.UploadStartedAt)
	defer cancel()
	<-jobContext.Done()

	processor := &Processor{store: database, dataDir: dataDir, ffmpeg: "unused-after-deadline", probe: FFProbe{Command: "unused-after-deadline"}, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	processor.process(jobContext, job)
	stored, err := database.JobByID(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.State != "failed" || stored.ErrorCode == nil || *stored.ErrorCode != "upload_timeout" || stored.ErrorMessage == nil || *stored.ErrorMessage != "This upload exceeded the one-hour total limit." {
		t.Fatalf("timed-out job=%+v", stored)
	}
	if _, err := os.Stat(temporaryDir); !os.IsNotExist(err) {
		t.Fatalf("temporary directory remains after timeout: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "media", storageID)); !os.IsNotExist(err) {
		t.Fatalf("final directory remains after timeout: %v", err)
	}
}

func TestExceedsStoredLimit(t *testing.T) {
	for _, test := range []struct {
		name           string
		size, target   int64
		bypassed, want bool
	}{
		{"normal at limit", 100, 100, false, false},
		{"normal at accepted 110 percent", 110, 100, false, false},
		{"normal above accepted threshold", 111, 100, false, true},
		{"admin bypass", 111, 100, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := exceedsStoredLimit(test.size, test.target, test.bypassed); got != test.want {
				t.Fatalf("got %t want %t", got, test.want)
			}
		})
	}
}

func TestProcessorPublishesCompatibleAssets(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is provided by the Docker development image")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is provided by the Docker development image")
	}
	dataDir := t.TempDir()
	for _, name := range []string{"database", "temporary", "media"} {
		if err := os.MkdirAll(filepath.Join(dataDir, name), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	database, err := store.Open(filepath.Join(dataDir, "database", "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	admin, err := database.CreateAdmin(ctx, "Admin", "admin", "hash", 50_000_000)
	if err != nil {
		t.Fatal(err)
	}
	storageID := "processor-test-storage"
	temporaryDir := filepath.Join(dataDir, "temporary", storageID)
	if err := os.MkdirAll(temporaryDir, 0o750); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(temporaryDir, "source")
	command := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=640x360:r=30:d=2", "-f", "lavfi", "-i", "sine=frequency=440:duration=2", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", "-f", "mp4", source)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate source: %v: %s", err, output)
	}
	metadata, err := (FFProbe{Command: ffprobe}).Probe(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	record, err := database.BeginUpload(ctx, admin.RootFolderID, "Processor test", "processor test", "processor-public", storageID, "processor-reservation", info.Size()*2, 0, 1_000_000_000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.CompleteUpload(ctx, record, "processor-reservation", info.Size(), metadata, store.UploadOptions{CompressionRequested: true, QualityCRF: 24, MaxHeight: 720, TargetSizeBytes: admin.StoredFileLimitBytes, StoredLimitBypassed: true}); err != nil {
		t.Fatal(err)
	}
	job, err := database.ClaimNextJob(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.RecoverInterruptedJobs(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := database.JobByID(ctx, job.ID)
	if err != nil || recovered.State != "queued" {
		t.Fatalf("recovered job = %+v err=%v", recovered, err)
	}
	job, err = database.ClaimNextJob(ctx)
	if err != nil {
		t.Fatal(err)
	}
	processor := &Processor{store: database, dataDir: dataDir, ffmpeg: ffmpeg, probe: FFProbe{Command: ffprobe}, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	processor.process(ctx, job)
	storedJob, err := database.JobByID(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedJob.State != "ready" || storedJob.Progress != 100 {
		t.Fatalf("job = %+v", storedJob)
	}
	for _, name := range []string{"video.mp4", "poster.jpg"} {
		info, err := os.Stat(filepath.Join(dataDir, "media", storageID, name))
		if err != nil || info.Size() == 0 {
			t.Fatalf("asset %s: size=%v err=%v", name, info, err)
		}
	}
	if _, err := os.Stat(temporaryDir); !os.IsNotExist(err) {
		t.Fatalf("temporary directory still exists: %v", err)
	}
}

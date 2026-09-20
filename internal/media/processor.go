package media

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"clip-share/internal/store"
)

const maximumUploadLifetime = time.Hour

var errUploadTimeout = errors.New("upload exceeded its one-hour deadline")

type Processor struct {
	store   *store.Store
	dataDir string
	ffmpeg  string
	probe   Prober
	logger  *slog.Logger
	workers sync.WaitGroup
}

func NewProcessor(data *store.Store, dataDir string, logger *slog.Logger) *Processor {
	return &Processor{store: data, dataDir: dataDir, ffmpeg: "ffmpeg", probe: FFProbe{Command: "ffprobe"}, logger: logger}
}

func (p *Processor) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	p.dispatch(ctx)
	for {
		select {
		case <-ctx.Done():
			p.workers.Wait()
			return
		case <-ticker.C:
			p.dispatch(ctx)
		}
	}
}

func (p *Processor) dispatch(ctx context.Context) {
	for ctx.Err() == nil {
		job, err := p.store.ClaimNextJob(ctx)
		if errors.Is(err, store.ErrNoQueuedJob) {
			return
		}
		if err != nil {
			p.logger.Error("job_claim_failed", "error", err)
			return
		}
		p.workers.Add(1)
		go func(claimed store.ProcessingJob) {
			defer p.workers.Done()
			jobContext, cancel := processingContext(ctx, claimed.UploadStartedAt)
			defer cancel()
			p.process(jobContext, claimed)
		}(job)
	}
}

func processingContext(parent context.Context, uploadStartedAt time.Time) (context.Context, context.CancelFunc) {
	return context.WithDeadlineCause(parent, uploadStartedAt.Add(maximumUploadLifetime), errUploadTimeout)
}

func uploadTimedOut(ctx context.Context) bool {
	return errors.Is(context.Cause(ctx), errUploadTimeout)
}

func (p *Processor) process(ctx context.Context, job store.ProcessingJob) {
	temporaryDir := filepath.Join(p.dataDir, "temporary", job.StorageID)
	select {
	case ffmpegGate <- struct{}{}:
		defer func() { <-ffmpegGate }()
	case <-ctx.Done():
		if uploadTimedOut(ctx) {
			p.fail(ctx, job, temporaryDir, "", "upload_timeout", "This upload exceeded the one-hour total limit.", context.Cause(ctx))
		}
		return
	}
	p.logger.Info("job_started", "jobId", job.ID, "clipId", job.ClipID)
	sourcePath := filepath.Join(temporaryDir, "source")
	outputDir := filepath.Join(temporaryDir, "output")
	entry, err := p.store.MediaLayoutEntry(ctx, job.ClipID)
	if err != nil {
		p.fail(ctx, job, temporaryDir, "", "processing_failed", "Could not prepare media storage.", err)
		return
	}
	finalDir := filepath.Join(p.dataDir, "media", filepath.FromSlash(entry.RelativeDir))
	_ = os.RemoveAll(outputDir)
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		p.fail(ctx, job, temporaryDir, finalDir, "processing_failed", "Could not prepare media processing.", err)
		return
	}
	videoPath := filepath.Join(outputDir, "video.mp4")

	compatible := compatibleSource(job.Source)
	edited := job.TrimEndMS != 0 || len(job.AudioRecipe) != 0
	encodeErr := error(nil)
	mandatoryTarget := !job.StoredLimitBypassed && job.SourceSizeBytes > job.TargetSizeBytes
	if edited {
		encodeErr = p.editEncode(ctx, job, sourcePath, videoPath)
	} else if !job.CompressionRequested && compatible {
		encodeErr = p.remux(ctx, job, sourcePath, videoPath)
	} else if mandatoryTarget {
		encodeErr = p.targetEncode(ctx, job, sourcePath, videoPath, temporaryDir)
	} else {
		encodeErr = p.crfEncode(ctx, job, sourcePath, videoPath)
	}
	if encodeErr != nil {
		if errors.Is(encodeErr, store.ErrJobCancelled) {
			p.cancelled(job, temporaryDir, finalDir)
			return
		}
		if ctx.Err() != nil && !uploadTimedOut(ctx) {
			_ = os.RemoveAll(outputDir)
			p.logger.Info("job_interrupted", "jobId", job.ID)
			return
		}
		p.fail(ctx, job, temporaryDir, finalDir, "processing_failed", "FFmpeg could not process this video.", encodeErr)
		return
	}

	info, err := os.Stat(videoPath)
	if err != nil {
		p.fail(ctx, job, temporaryDir, finalDir, "processing_failed", "The processed video could not be verified.", err)
		return
	}
	if job.CompressionRequested && compatible && !mandatoryTarget && info.Size() > job.SourceSizeBytes {
		if err := os.Remove(videoPath); err != nil {
			p.fail(ctx, job, temporaryDir, finalDir, "processing_failed", "Could not apply the safer original-quality fallback.", err)
			return
		}
		if err := p.remux(ctx, job, sourcePath, videoPath); err != nil {
			p.fail(ctx, job, temporaryDir, finalDir, "processing_failed", "Could not apply the safer original-quality fallback.", err)
			return
		}
		info, err = os.Stat(videoPath)
		if err != nil {
			p.fail(ctx, job, temporaryDir, finalDir, "processing_failed", "The processed video could not be verified.", err)
			return
		}
	}
	if exceedsStoredLimit(info.Size(), job.TargetSizeBytes, job.StoredLimitBypassed) {
		p.fail(ctx, job, temporaryDir, finalDir, "size_exceeded", "The processed video exceeded the allowed stored size.", nil)
		return
	}
	metadata, err := p.probe.Probe(ctx, videoPath)
	if err != nil || !compatibleOutput(metadata) {
		p.fail(ctx, job, temporaryDir, finalDir, "validation_failed", "The processed video did not meet the compatibility requirements.", err)
		return
	}
	if err := p.poster(ctx, job, videoPath, filepath.Join(outputDir, "poster.jpg")); err != nil {
		p.fail(ctx, job, temporaryDir, finalDir, "processing_failed", "Could not create the video poster.", err)
		return
	}
	if err := p.store.UpdateJobProgress(ctx, job.ID, 96); err != nil {
		p.fail(ctx, job, temporaryDir, finalDir, "processing_failed", "Could not save processing progress.", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(finalDir), 0o750); err != nil {
		p.fail(ctx, job, temporaryDir, finalDir, "processing_failed", "Could not prepare media storage.", err)
		return
	}
	if err := os.Rename(outputDir, finalDir); err != nil {
		p.fail(ctx, job, temporaryDir, finalDir, "processing_failed", "Could not publish the processed video.", err)
		return
	}
	if err := p.store.CompleteProcessing(ctx, job, info.Size(), metadata); err != nil {
		_ = os.RemoveAll(finalDir)
		if errors.Is(err, store.ErrJobCancelled) {
			p.cancelled(job, temporaryDir, finalDir)
			return
		}
		p.fail(ctx, job, temporaryDir, finalDir, "processing_failed", "Could not commit the processed video.", err)
		return
	}
	_ = os.RemoveAll(temporaryDir)
	p.logger.Info("job_ready", "jobId", job.ID, "clipId", job.ClipID, "sizeBytes", info.Size())
}

// editEncode uses filter trimming before encoding so saved boundaries are accurate.
// The audio graph uses only explicit source stream indices and emits no stream when
// every track is muted.
func (p *Processor) editEncode(ctx context.Context, job store.ProcessingJob, source, output string) error {
	start := float64(job.TrimStartMS) / 1000
	end := float64(job.TrimEndMS) / 1000
	if job.TrimEndMS == 0 {
		end = job.Source.DurationSeconds
	}
	filter := fmt.Sprintf("[0:v:0]trim=start=%.3f:end=%.3f,setpts=PTS-STARTPTS[v]", start, end)
	args := []string{"-y", "-v", "error", "-i", source, "-filter_complex", ""}
	selected := make([]store.AudioStream, 0)
	for _, track := range job.AudioRecipe {
		if track.Usable && track.Included {
			selected = append(selected, track)
		}
	}
	if len(selected) != 0 {
		parts := []string{filter}
		labels := make([]string, 0, len(selected))
		for i, track := range selected {
			label := fmt.Sprintf("a%d", i)
			parts = append(parts, fmt.Sprintf("[0:%d]atrim=start=%.3f:end=%.3f,asetpts=PTS-STARTPTS,volume=%.1fdB[%s]", track.Index, start, end, track.GainDB, label))
			labels = append(labels, "["+label+"]")
		}
		parts = append(parts, fmt.Sprintf("%samix=inputs=%d:normalize=0,alimiter=limit=0.8912509,atrim=duration=%.3f[a]", strings.Join(labels, ""), len(labels), end-start))
		args[6] = strings.Join(parts, ";")
		args = append(args, "-map", "[v]", "-map", "[a]")
	} else {
		args[6] = filter
		args = append(args, "-map", "[v]")
	}
	args = append(args, "-c:v", "libx264", "-preset", "medium", "-profile:v", "high", "-level:v", "4.2", "-pix_fmt", "yuv420p", "-crf", strconv.Itoa(job.QualityCRF))
	if len(selected) != 0 {
		args = append(args, "-c:a", "aac", "-b:a", "128k", "-ac", "2")
	}
	args = append(args, "-movflags", "+faststart", output)
	return p.run(ctx, job, args, 5, 88)
}

// exceedsStoredLimit is kept separate so the exact 110% acceptance boundary is
// tested without depending on a particular FFmpeg encoder version.
func exceedsStoredLimit(size, target int64, bypassed bool) bool {
	return !bypassed && size > target*110/100
}

func (p *Processor) remux(ctx context.Context, job store.ProcessingJob, source, output string) error {
	args := []string{"-y", "-v", "error", "-i", source, "-map", "0:v:0", "-map", "0:a:0?", "-c", "copy", "-movflags", "+faststart", output}
	return p.run(ctx, job, args, 5, 88)
}

func (p *Processor) crfEncode(ctx context.Context, job store.ProcessingJob, source, output string) error {
	args := append([]string{"-y", "-v", "error", "-i", source}, p.videoArgs(job)...)
	args = append(args, "-crf", strconv.Itoa(job.QualityCRF), "-c:a", "aac", "-b:a", "128k", "-ac", "2", "-movflags", "+faststart", output)
	return p.run(ctx, job, args, 5, 88)
}

func (p *Processor) targetEncode(ctx context.Context, job store.ProcessingJob, source, output, temporaryDir string) error {
	audioRate := int64(0)
	if job.Source.AudioCodec != "" {
		audioRate = 128_000
	}
	videoRate := int64(float64(job.TargetSizeBytes)*0.98*8/job.Source.DurationSeconds) - audioRate
	if videoRate < 100_000 {
		return errors.New("target bitrate is too low")
	}
	passLog := filepath.Join(temporaryDir, "ffmpeg-pass")
	base := append([]string{"-y", "-v", "error", "-i", source}, p.videoArgs(job)...)
	first := append(append([]string{}, base...), "-b:v", strconv.FormatInt(videoRate, 10), "-pass", "1", "-passlogfile", passLog, "-an", "-f", "mp4", os.DevNull)
	if err := p.run(ctx, job, first, 5, 45); err != nil {
		return err
	}
	second := append(append([]string{}, base...), "-b:v", strconv.FormatInt(videoRate, 10), "-pass", "2", "-passlogfile", passLog, "-c:a", "aac", "-b:a", "128k", "-ac", "2", "-movflags", "+faststart", output)
	err := p.run(ctx, job, second, 45, 88)
	matches, _ := filepath.Glob(passLog + "*")
	for _, path := range matches {
		_ = os.Remove(path)
	}
	return err
}

func (p *Processor) videoArgs(job store.ProcessingJob) []string {
	maxWidth := map[int]int{480: 854, 720: 1280, 1080: 1920}[job.MaxHeight]
	filter := fmt.Sprintf("scale=w=min(iw\\,%d):h=min(ih\\,%d):force_original_aspect_ratio=decrease:force_divisible_by=2", maxWidth, job.MaxHeight)
	args := []string{"-map", "0:v:0", "-map", "0:a:0?", "-c:v", "libx264", "-preset", "medium", "-profile:v", "high", "-level:v", "4.2", "-pix_fmt", "yuv420p", "-vf", filter}
	if job.Source.FrameRate > 60 {
		args = append(args, "-r", "60")
	}
	return args
}

func (p *Processor) poster(ctx context.Context, job store.ProcessingJob, video, output string) error {
	position := job.Source.DurationSeconds * 0.1
	if position > 5 {
		position = 5
	}
	args := []string{"-y", "-v", "error", "-ss", strconv.FormatFloat(position, 'f', 3, 64), "-i", video, "-frames:v", "1", "-vf", "scale=1280:720:force_original_aspect_ratio=decrease", "-q:v", "3", output}
	if err := p.run(ctx, job, args, 90, 95); err != nil {
		return err
	}
	info, err := os.Stat(output)
	if err != nil || info.Size() == 0 {
		return errors.New("poster is empty")
	}
	return nil
}

func (p *Processor) run(ctx context.Context, job store.ProcessingJob, args []string, startProgress, endProgress int) error {
	args = append([]string{"-progress", "pipe:1", "-nostats"}, args...)
	command := exec.CommandContext(ctx, p.ffmpeg, args...)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	last := startProgress
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "out_time_us=") {
			continue
		}
		micros, _ := strconv.ParseFloat(strings.TrimPrefix(line, "out_time_us="), 64)
		fraction := micros / 1_000_000 / job.Source.DurationSeconds
		progress := startProgress + int(fraction*float64(endProgress-startProgress))
		if progress > endProgress {
			progress = endProgress
		}
		if progress >= last+2 {
			if err := p.store.UpdateJobProgress(ctx, job.ID, progress); err != nil {
				_ = command.Process.Kill()
				_ = command.Wait()
				return err
			}
			last = progress
		}
	}
	if err := scanner.Err(); err != nil {
		_ = command.Process.Kill()
		return err
	}
	if err := command.Wait(); err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return p.store.UpdateJobProgress(ctx, job.ID, endProgress)
}

func (p *Processor) fail(ctx context.Context, job store.ProcessingJob, temporaryDir, finalDir, code, message string, cause error) {
	saveContext := ctx
	if uploadTimedOut(ctx) {
		code = "upload_timeout"
		message = "This upload exceeded the one-hour total limit."
		var cancel context.CancelFunc
		saveContext, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	} else if ctx.Err() != nil {
		_ = os.RemoveAll(filepath.Join(temporaryDir, "output"))
		_ = os.RemoveAll(finalDir)
		p.logger.Info("job_interrupted", "jobId", job.ID, "clipId", job.ClipID)
		return
	}
	_ = os.RemoveAll(temporaryDir)
	_ = os.RemoveAll(finalDir)
	if err := p.store.FailProcessing(saveContext, job, code, message); errors.Is(err, store.ErrJobCancelled) {
		p.logger.Info("job_cancelled", "jobId", job.ID, "clipId", job.ClipID)
		return
	} else if err != nil {
		p.logger.Error("job_failure_save_failed", "jobId", job.ID, "error", err)
	}
	attributes := []any{"jobId", job.ID, "clipId", job.ClipID, "code", code}
	if cause != nil {
		attributes = append(attributes, "error", cause)
	}
	p.logger.Error("job_failed", attributes...)
}

func (p *Processor) cancelled(job store.ProcessingJob, temporaryDir, finalDir string) {
	_ = os.RemoveAll(temporaryDir)
	_ = os.RemoveAll(finalDir)
	p.logger.Info("job_cancelled", "jobId", job.ID, "clipId", job.ClipID)
}

func compatibleSource(metadata store.ProbeMetadata) bool {
	container := strings.ToLower(metadata.Container)
	profile := strings.ToLower(metadata.Profile)
	return strings.Contains(container, "mp4") && metadata.VideoCodec == "h264" && (metadata.AudioCodec == "" || metadata.AudioCodec == "aac") && metadata.PixelFormat == "yuv420p" && strings.Contains(profile, "high") && metadata.Width <= 1920 && metadata.Height <= 1080 && metadata.FrameRate <= 60
}

func compatibleOutput(metadata store.ProbeMetadata) bool {
	return strings.Contains(strings.ToLower(metadata.Container), "mp4") && metadata.VideoCodec == "h264" && (metadata.AudioCodec == "" || metadata.AudioCodec == "aac") && metadata.PixelFormat == "yuv420p" && metadata.Width <= 1920 && metadata.Height <= 1080 && metadata.FrameRate <= 60
}

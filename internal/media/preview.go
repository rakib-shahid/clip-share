package media

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"clip-share/internal/store"
)

// ffmpegGate deliberately limits this self-hosted installation to one active
// render. A preview never preempts an active final render.
var ffmpegGate = make(chan struct{}, 1)

type PreviewRequest struct {
	SourcePath string
	OutputPath string
	StartMS    int
	EndMS      int
	Audio      []store.AudioStream
}

func RenderPreview(ctx context.Context, request PreviewRequest) error {
	select {
	case ffmpegGate <- struct{}{}:
		defer func() { <-ffmpegGate }()
	case <-ctx.Done():
		return ctx.Err()
	}
	start, end := float64(request.StartMS)/1000, float64(request.EndMS)/1000
	filter := fmt.Sprintf("[0:v:0]trim=start=%.3f:end=%.3f,setpts=PTS-STARTPTS,scale=w=min(iw\\,1280):h=min(ih\\,720):force_original_aspect_ratio=decrease:force_divisible_by=2[v]", start, end)
	args := []string{"-y", "-v", "error", "-i", request.SourcePath, "-filter_complex", ""}
	selected := make([]store.AudioStream, 0)
	for _, track := range request.Audio {
		if track.Usable && track.Included {
			selected = append(selected, track)
		}
	}
	if len(selected) > 0 {
		parts := []string{filter}
		labels := make([]string, 0, len(selected))
		for i, track := range selected {
			label := fmt.Sprintf("a%d", i)
			parts = append(parts, fmt.Sprintf("[0:%d]atrim=start=%.3f:end=%.3f,asetpts=PTS-STARTPTS,volume=%.1fdB[%s]", track.Index, start, end, track.GainDB, label))
			labels = append(labels, "["+label+"]")
		}
		parts = append(parts, fmt.Sprintf("%samix=inputs=%d:normalize=0,alimiter=limit=0.8912509,atrim=duration=%.3f[a]", strings.Join(labels, ""), len(labels), end-start))
		args[6] = strings.Join(parts, ";")
		args = append(args, "-map", "[v]", "-map", "[a]", "-c:a", "aac", "-b:a", "128k", "-ac", "2")
	} else {
		args[6] = filter
		args = append(args, "-map", "[v]")
	}
	args = append(args, "-c:v", "libx264", "-preset", "medium", "-profile:v", "high", "-pix_fmt", "yuv420p", "-b:v", "2M", "-movflags", "+faststart", request.OutputPath)
	if output, err := exec.CommandContext(ctx, "ffmpeg", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg preview: %w: %s", err, strings.TrimSpace(string(output)))
	}
	info, err := os.Stat(request.OutputPath)
	if err != nil || info.Size() == 0 {
		if err == nil {
			err = fmt.Errorf("preview is empty")
		}
		return err
	}
	return nil
}

func PreviewPath(dataDir, storageID string) string {
	return filepath.Join(dataDir, "temporary", storageID, "preview.mp4")
}

func PendingPreviewPath(dataDir, storageID string) string {
	return filepath.Join(dataDir, "temporary", storageID, "preview.pending.mp4")
}

// PublishPreview replaces the visible proxy only after FFmpeg has completely
// closed and validated its replacement, so playback and filmstrip extraction
// can continue using the previous render while a new one is produced.
func PublishPreview(pendingPath, outputPath string) error {
	if err := os.Rename(pendingPath, outputPath); err == nil {
		return nil
	}
	if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(pendingPath, outputPath)
}

package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"clip-share/internal/store"
)

var (
	ErrUnsupportedMedia   = errors.New("the file is not a supported video")
	ErrDurationExceeded   = errors.New("videos cannot be longer than 30 minutes")
	ErrResolutionExceeded = errors.New("videos cannot exceed 8K resolution")
	ErrFrameRateExceeded  = errors.New("videos cannot exceed 240 FPS")
	ErrEncryptedMedia     = errors.New("encrypted videos are not supported")
)

type Prober interface {
	Probe(context.Context, string) (store.ProbeMetadata, error)
}

type FFProbe struct{ Command string }

type probeOutput struct {
	Format struct {
		FormatName       string `json:"format_name"`
		Duration         string `json:"duration"`
		EncryptionScheme string `json:"encryption_scheme"`
	} `json:"format"`
	Streams []struct {
		Index         int    `json:"index"`
		CodecName     string `json:"codec_name"`
		CodecType     string `json:"codec_type"`
		Width         int    `json:"width"`
		Height        int    `json:"height"`
		AverageRate   string `json:"avg_frame_rate"`
		Duration      string `json:"duration"`
		PixelFormat   string `json:"pix_fmt"`
		Profile       string `json:"profile"`
		Channels      int    `json:"channels"`
		ChannelLayout string `json:"channel_layout"`
		SampleRate    string `json:"sample_rate"`
		Tags          struct {
			Language string `json:"language"`
		} `json:"tags"`
		EncryptionScheme string `json:"encryption_scheme"`
		EncryptionKID    string `json:"encryption_kid"`
		SideDataList     []struct {
			Type string `json:"side_data_type"`
		} `json:"side_data_list"`
		Disposition struct {
			Default         int `json:"default"`
			AttachedPicture int `json:"attached_pic"`
		} `json:"disposition"`
	} `json:"streams"`
}

func (p FFProbe) Probe(ctx context.Context, path string) (store.ProbeMetadata, error) {
	command := p.Command
	if command == "" {
		command = "ffprobe"
	}
	output, err := exec.CommandContext(ctx, command, "-v", "error", "-show_format", "-show_streams", "-of", "json", path).Output()
	if err != nil {
		return store.ProbeMetadata{}, ErrUnsupportedMedia
	}
	var result probeOutput
	if err := json.Unmarshal(output, &result); err != nil {
		return store.ProbeMetadata{}, ErrUnsupportedMedia
	}
	return validateProbeOutput(result)
}

// validateProbeOutput keeps the ffprobe parsing rules small and directly testable.
// FFProbe.Probe is deliberately only responsible for running the external command.
func validateProbeOutput(result probeOutput) (store.ProbeMetadata, error) {
	if strings.TrimSpace(result.Format.EncryptionScheme) != "" {
		return store.ProbeMetadata{}, ErrEncryptedMedia
	}
	videoIndex := -1
	for index, stream := range result.Streams {
		if strings.TrimSpace(stream.EncryptionScheme) != "" || strings.TrimSpace(stream.EncryptionKID) != "" || hasEncryptionSideData(stream.SideDataList) {
			return store.ProbeMetadata{}, ErrEncryptedMedia
		}
		if stream.CodecType == "video" && stream.CodecName != "" && stream.Disposition.AttachedPicture == 0 {
			if videoIndex == -1 || stream.Disposition.Default == 1 {
				videoIndex = index
			}
			if stream.Disposition.Default == 1 {
				break
			}
		}
	}
	if videoIndex == -1 {
		return store.ProbeMetadata{}, ErrUnsupportedMedia
	}
	video := result.Streams[videoIndex]
	duration := parseFloat(video.Duration)
	if duration <= 0 {
		duration = parseFloat(result.Format.Duration)
	}
	if duration <= 0 {
		return store.ProbeMetadata{}, ErrUnsupportedMedia
	}
	if duration > 1800 {
		return store.ProbeMetadata{}, ErrDurationExceeded
	}
	if video.Width < 1 || video.Height < 1 {
		return store.ProbeMetadata{}, ErrUnsupportedMedia
	}
	if video.Width > 7680 || video.Height > 4320 {
		return store.ProbeMetadata{}, ErrResolutionExceeded
	}
	frameRate, err := parseRate(video.AverageRate)
	if err != nil || frameRate <= 0 {
		return store.ProbeMetadata{}, ErrUnsupportedMedia
	}
	if frameRate > 240 {
		return store.ProbeMetadata{}, ErrFrameRateExceeded
	}
	audioCodec := ""
	audioStreams := make([]store.AudioStream, 0)
	for _, stream := range result.Streams {
		if stream.CodecType == "audio" && stream.CodecName != "" {
			audioCodec = stream.CodecName
			audioStreams = append(audioStreams, store.AudioStream{Index: stream.Index, Codec: stream.CodecName, Channels: stream.Channels, ChannelLayout: stream.ChannelLayout, SampleRate: int(parseFloat(stream.SampleRate)), Language: stream.Tags.Language, Default: stream.Disposition.Default == 1, Usable: true})
		}
	}
	return store.ProbeMetadata{Container: result.Format.FormatName, VideoCodec: video.CodecName, AudioCodec: audioCodec, DurationSeconds: duration, Width: video.Width, Height: video.Height, FrameRate: frameRate, PixelFormat: video.PixelFormat, Profile: video.Profile, VideoStreamIndex: video.Index, AudioStreams: audioStreams}, nil
}

func hasEncryptionSideData(items []struct {
	Type string `json:"side_data_type"`
}) bool {
	for _, item := range items {
		value := strings.ToLower(item.Type)
		if strings.Contains(value, "encryption") || strings.Contains(value, "cenc") || strings.Contains(value, "drm") {
			return true
		}
	}
	return false
}

func parseFloat(value string) float64 { number, _ := strconv.ParseFloat(value, 64); return number }
func parseRate(value string) (float64, error) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid frame rate")
	}
	numerator, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, err
	}
	denominator, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || denominator == 0 {
		return 0, fmt.Errorf("invalid frame rate")
	}
	return numerator / denominator, nil
}

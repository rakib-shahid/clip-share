package media

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestValidateProbeOutput(t *testing.T) {
	valid := func() probeOutput {
		var result probeOutput
		result.Format.FormatName, result.Format.Duration = "mov,mp4,m4a,3gp,3g2,mj2", "12"
		result.Streams = append(result.Streams, struct {
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
		}{CodecName: "h264", CodecType: "video", Width: 1920, Height: 1080, AverageRate: "60/1", Duration: "12"})
		return result
	}
	tests := []struct {
		name   string
		change func(*probeOutput)
		want   error
	}{
		{"valid", func(*probeOutput) {}, nil},
		{"corrupt/no video", func(v *probeOutput) { v.Streams = nil }, ErrUnsupportedMedia},
		{"attachment only", func(v *probeOutput) { v.Streams[0].Disposition.AttachedPicture = 1 }, ErrUnsupportedMedia},
		{"missing duration", func(v *probeOutput) { v.Streams[0].Duration = ""; v.Format.Duration = "" }, ErrUnsupportedMedia},
		{"invalid rate", func(v *probeOutput) { v.Streams[0].AverageRate = "not-a-rate" }, ErrUnsupportedMedia},
		{"over 8k", func(v *probeOutput) { v.Streams[0].Width = 7681 }, ErrResolutionExceeded},
		{"over 240 fps", func(v *probeOutput) { v.Streams[0].AverageRate = "241/1" }, ErrFrameRateExceeded},
		{"encrypted stream", func(v *probeOutput) { v.Streams[0].EncryptionScheme = "cenc" }, ErrEncryptedMedia},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := valid()
			test.change(&value)
			_, err := validateProbeOutput(value)
			if !errors.Is(err, test.want) {
				t.Fatalf("error=%v want=%v", err, test.want)
			}
		})
	}
}

func TestParseRate(t *testing.T) {
	value, err := parseRate("30000/1001")
	if err != nil || value < 29.96 || value > 29.98 {
		t.Fatalf("value=%f err=%v", value, err)
	}
	if _, err := parseRate("1/0"); err == nil {
		t.Fatal("expected zero denominator error")
	}
}

func TestFFProbeReadsGeneratedVideo(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is provided by the Docker development image")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is provided by the Docker development image")
	}
	path := filepath.Join(t.TempDir(), "sample.mp4")
	command := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=320x180:d=1", "-c:v", "libx264", "-pix_fmt", "yuv420p", path)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate video: %v: %s", err, output)
	}
	metadata, err := (FFProbe{Command: ffprobe}).Probe(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.VideoCodec != "h264" || metadata.Width != 320 || metadata.Height != 180 || metadata.DurationSeconds <= 0 {
		t.Fatalf("metadata = %+v", metadata)
	}
}

package transcode

import (
	"bufio"
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
)

var (
	ErrTranscodeCancelled   = errors.New("transcode cancelled")
	ErrHardwareUnavailable  = errors.New("hardware encoder unavailable or failed")
	ErrInvalidSourceFile    = errors.New("source file does not exist or is invalid")
	ErrOutputValidationFail = errors.New("transcoded output file is missing or empty")
)

// HardwareCapability represents detected hardware video encoding support.
type HardwareCapability string

const (
	HardwareAuto     HardwareCapability = "auto"
	HardwareQSV      HardwareCapability = "qsv"
	HardwareNVENC    HardwareCapability = "nvenc"
	HardwareVAAPI    HardwareCapability = "vaapi"
	HardwareSoftware HardwareCapability = "software"
)

// TranscodeOptions defines parameters for a transcoding job.
type TranscodeOptions struct {
	SourcePath  string
	TargetPath  string
	Profile     string // "universal", "h264", "h265"
	HardwareAcc HardwareCapability
	ProgressCb  func(percent float64)
}

// TranscodeResult records the outcome of a completed transcode operation.
type TranscodeResult struct {
	TargetPath  string
	EncoderUsed string
	Duration    time.Duration
}

// Transcoder defines the interface for video encoding operations.
type Transcoder interface {
	Transcode(ctx context.Context, opts TranscodeOptions) (*TranscodeResult, error)
	DetectHardware(ctx context.Context) HardwareCapability
}

// HardwareDetector defines a function that detects available hardware capabilities.
type HardwareDetector func(ctx context.Context) HardwareCapability

// DefaultHardwareDetector inspects host device nodes and DRM sysfs to discover GPU acceleration.
func DefaultHardwareDetector(ctx context.Context) HardwareCapability {
	// 1. Check for Nvidia NVENC (/dev/nvidia0, /dev/nvidiactl, or nvidia-smi)
	if _, err := os.Stat("/dev/nvidia0"); err == nil {
		return HardwareNVENC
	}
	if _, err := os.Stat("/dev/nvidiactl"); err == nil {
		return HardwareNVENC
	}
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		return HardwareNVENC
	}

	// 2. Check DRM vendor if render node is available
	if data, err := os.ReadFile("/sys/class/drm/renderD128/device/vendor"); err == nil {
		vendor := strings.ToLower(strings.TrimSpace(string(data)))
		switch vendor {
		case "0x8086": // Intel QSV
			return HardwareQSV
		case "0x1002": // AMD VAAPI
			return HardwareVAAPI
		case "0x10de": // Nvidia NVENC
			return HardwareNVENC
		}
	}

	// 3. Fallback check for /dev/dri render node presence
	if _, err := os.Stat("/dev/dri/renderD128"); err == nil {
		return HardwareQSV
	}
	if _, err := os.Stat("/dev/dri"); err == nil {
		return HardwareVAAPI
	}

	return HardwareSoftware
}

// FFmpegTranscoder executes real transcoding via the system ffmpeg binary.
type FFmpegTranscoder struct {
	ffmpegPath string
	detector   HardwareDetector
	logger     *slog.Logger
}

// NewFFmpegTranscoder creates an FFmpegTranscoder instance.
func NewFFmpegTranscoder(logger *slog.Logger, detector HardwareDetector) *FFmpegTranscoder {
	if logger == nil {
		logger = slog.Default()
	}
	if detector == nil {
		detector = DefaultHardwareDetector
	}
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		ffmpegPath = "ffmpeg"
	}
	return &FFmpegTranscoder{
		ffmpegPath: ffmpegPath,
		detector:   detector,
		logger:     logger,
	}
}

// DetectHardware discovers available host video acceleration.
func (f *FFmpegTranscoder) DetectHardware(ctx context.Context) HardwareCapability {
	return f.detector(ctx)
}

// Transcode runs an FFmpeg process to produce universal direct-play MP4 media.
func (f *FFmpegTranscoder) Transcode(ctx context.Context, opts TranscodeOptions) (*TranscodeResult, error) {
	if fi, err := os.Stat(opts.SourcePath); err != nil || fi.IsDir() {
		return nil, ErrInvalidSourceFile
	}

	// Ensure destination directory exists prior to launching FFmpeg
	targetDir := filepath.Dir(opts.TargetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create target directory %q: %w", targetDir, err)
	}

	hw := opts.HardwareAcc
	if hw == "" || hw == HardwareAuto {
		hw = f.DetectHardware(ctx)
	}

	start := time.Now()
	tmpOutput := fmt.Sprintf("%s.transcoding-%d.mp4", opts.TargetPath, time.Now().UnixNano())
	defer func() {
		// Clean up temporary file if leftover on failure
		_ = os.Remove(tmpOutput)
	}()

	args, encoderUsed := f.buildFFmpegArgs(opts.SourcePath, tmpOutput, opts.Profile, hw)
	f.logger.Info("starting ffmpeg transcoding job",
		"source", opts.SourcePath,
		"target", opts.TargetPath,
		"hw", string(hw),
		"encoder", encoderUsed,
	)

	cmd := exec.CommandContext(ctx, f.ffmpegPath, args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to open ffmpeg stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Read duration and progress lines from stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "time=") && opts.ProgressCb != nil {
				// Parse time=HH:MM:SS.SS or similar if possible
				if pct := parseFFmpegProgress(line); pct > 0 {
					opts.ProgressCb(pct)
				}
			}
		}
	}()

	err = cmd.Wait()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ErrTranscodeCancelled
		}
		f.logger.Warn("ffmpeg command failed with encoder", "encoder", encoderUsed, "error", err)
		return nil, fmt.Errorf("ffmpeg execution failed (%s): %w", encoderUsed, err)
	}

	// Validate output
	fi, err := os.Stat(tmpOutput)
	if err != nil || fi.Size() == 0 {
		return nil, ErrOutputValidationFail
	}

	// Atomic rename to final target path
	if err := os.Rename(tmpOutput, opts.TargetPath); err != nil {
		return nil, fmt.Errorf("failed to atomically place transcoded file: %w", err)
	}

	if opts.ProgressCb != nil {
		opts.ProgressCb(100.0)
	}

	return &TranscodeResult{
		TargetPath:  opts.TargetPath,
		EncoderUsed: encoderUsed,
		Duration:    time.Since(start),
	}, nil
}

func (f *FFmpegTranscoder) buildFFmpegArgs(src, dst, profile string, hw HardwareCapability) ([]string, string) {
	return f.BuildFFmpegArgs(src, dst, profile, hw)
}

// BuildFFmpegArgs constructs FFmpeg CLI flags for the specified profile, source, destination, and hardware encoder.
func (f *FFmpegTranscoder) BuildFFmpegArgs(src, dst, profile string, hw HardwareCapability) ([]string, string) {
	args := []string{"-y"}

	// Hardware acceleration input flags (must precede -i)
	switch hw {
	case HardwareQSV:
		args = append(args, "-hwaccel", "qsv")
	case HardwareNVENC:
		args = append(args, "-hwaccel", "cuda")
	case HardwareVAAPI:
		args = append(args, "-vaapi_device", "/dev/dri/renderD128", "-hwaccel", "vaapi")
	}

	// Input file specification
	args = append(args, "-i", src)

	// If mobile profile, apply standardized resolution scaling and bounded bitrates
	if mobCfg, isMobile := GetMobileProfile(profile); isMobile {
		args = append(args, "-vf", mobCfg.ScaleFilter)

		var encoder string
		switch hw {
		case HardwareQSV:
			args = append(args, "-c:v", "h264_qsv")
			encoder = "qsv"
		case HardwareNVENC:
			args = append(args, "-c:v", "h264_nvenc")
			encoder = "nvenc"
		case HardwareVAAPI:
			args = append(args, "-c:v", "h264_vaapi")
			encoder = "vaapi"
		default:
			args = append(args, "-c:v", "libx264", "-preset", "fast", "-crf", "22", "-pix_fmt", "yuv420p")
			encoder = "cpu-x264"
		}

		args = append(args,
			"-b:v", mobCfg.VideoBitrate,
			"-maxrate", mobCfg.MaxRate,
			"-bufsize", mobCfg.BufSize,
			"-c:a", mobCfg.AudioCodec,
			"-b:a", mobCfg.AudioBitrate,
			"-ac", fmt.Sprintf("%d", mobCfg.AudioChannels),
			"-movflags", "+faststart",
			"-f", mobCfg.Container,
			dst,
		)

		return args, encoder
	}

	// Output video codec & options for standard profiles (must follow -i)
	var encoder string
	isH265 := strings.EqualFold(profile, "h265")

	switch hw {
	case HardwareQSV:
		if isH265 {
			args = append(args, "-c:v", "hevc_qsv")
			encoder = "qsv-hevc"
		} else {
			args = append(args, "-c:v", "h264_qsv")
			encoder = "qsv"
		}
	case HardwareNVENC:
		if isH265 {
			args = append(args, "-c:v", "hevc_nvenc")
			encoder = "nvenc-hevc"
		} else {
			args = append(args, "-c:v", "h264_nvenc")
			encoder = "nvenc"
		}
	case HardwareVAAPI:
		if isH265 {
			args = append(args, "-c:v", "hevc_vaapi")
			encoder = "vaapi-hevc"
		} else {
			args = append(args, "-c:v", "h264_vaapi")
			encoder = "vaapi"
		}
	default:
		if isH265 {
			args = append(args, "-c:v", "libx265", "-preset", "fast", "-crf", "24")
			encoder = "cpu-x265"
		} else {
			args = append(args, "-c:v", "libx264", "-preset", "fast", "-crf", "22", "-pix_fmt", "yuv420p")
			encoder = "cpu-x264"
		}
	}

	// Output audio & container flags
	args = append(args,
		"-c:a", "aac",
		"-b:a", "192k",
		"-ac", "2",
		"-movflags", "+faststart",
		"-f", "mp4",
		dst,
	)

	return args, encoder
}

func parseFFmpegProgress(line string) float64 {
	// Simple heuristic extractor for percentage or progress
	idx := strings.Index(line, "time=")
	if idx == -1 {
		return 0.0
	}
	sub := line[idx+5:]
	parts := strings.Fields(sub)
	if len(parts) == 0 {
		return 0.0
	}
	timeStr := parts[0]
	tParts := strings.Split(timeStr, ":")
	if len(tParts) == 3 {
		h, _ := strconv.ParseFloat(tParts[0], 64)
		m, _ := strconv.ParseFloat(tParts[1], 64)
		s, _ := strconv.ParseFloat(tParts[2], 64)
		totalSec := h*3600 + m*60 + s
		// As a synthetic normalized heuristic: cap at 99.0 until finished
		if totalSec > 0 {
			pct := totalSec / 36.0 // rough approximation
			if pct > 99.0 {
				pct = 99.0
			}
			return pct
		}
	}
	return 0.0
}

// MockTranscoder provides a deterministic mock implementation for testing without invoking FFmpeg.
type MockTranscoder struct {
	mu                   sync.RWMutex
	DetectedHW           HardwareCapability
	FailHardwareOnce     bool
	FailHardwareCount    int
	AlwaysFail           bool
	FailErr              error
	StepDelay            time.Duration
	TranscodeCalls       int
	LastOpts             *TranscodeOptions
	HardwareFailCallback func()
}

// NewMockTranscoder returns a default configured MockTranscoder.
func NewMockTranscoder() *MockTranscoder {
	return &MockTranscoder{
		DetectedHW: HardwareQSV,
	}
}

// DetectHardware returns the configured simulated hardware capability.
func (m *MockTranscoder) DetectHardware(ctx context.Context) HardwareCapability {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.DetectedHW
}

// SetDetectedHardware updates the simulated hardware capability.
func (m *MockTranscoder) SetDetectedHardware(hw HardwareCapability) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DetectedHW = hw
}

// Transcode simulates transcoding with deterministic progress and fallback support.
func (m *MockTranscoder) Transcode(ctx context.Context, opts TranscodeOptions) (*TranscodeResult, error) {
	m.mu.Lock()
	m.TranscodeCalls++
	optsCopy := opts
	m.LastOpts = &optsCopy

	if m.AlwaysFail {
		err := m.FailErr
		if err == nil {
			err = errors.New("simulated fatal transcode error")
		}
		m.mu.Unlock()
		return nil, err
	}

	hw := opts.HardwareAcc
	if hw == "" || hw == HardwareAuto {
		hw = m.DetectedHW
	}

	// Check if simulated hardware failure is triggered
	isHW := hw == HardwareQSV || hw == HardwareNVENC || hw == HardwareVAAPI
	if isHW && (m.FailHardwareOnce || m.FailHardwareCount > 0) {
		if m.FailHardwareCount > 0 {
			m.FailHardwareCount--
		} else {
			m.FailHardwareOnce = false
		}
		cb := m.HardwareFailCallback
		m.mu.Unlock()
		if cb != nil {
			cb()
		}
		return nil, ErrHardwareUnavailable
	}

	stepDelay := m.StepDelay
	m.mu.Unlock()

	// Verify source path exists
	if _, err := os.Stat(opts.SourcePath); err != nil {
		return nil, ErrInvalidSourceFile
	}

	start := time.Now()

	// Simulate progress callbacks with cancellation responsiveness
	progressSteps := []float64{25.0, 50.0, 75.0, 99.0}
	for _, p := range progressSteps {
		select {
		case <-ctx.Done():
			return nil, ErrTranscodeCancelled
		default:
			if opts.ProgressCb != nil {
				opts.ProgressCb(p)
			}
			if stepDelay > 0 {
				select {
				case <-ctx.Done():
					return nil, ErrTranscodeCancelled
				case <-time.After(stepDelay):
				}
			}
		}
	}

	// Check cancellation again before writing output
	if err := ctx.Err(); err != nil {
		return nil, ErrTranscodeCancelled
	}

	// Ensure destination directory exists
	targetDir := filepath.Dir(opts.TargetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create target directory: %w", err)
	}

	// Create temporary output file first, then atomically rename
	tmpOutput := fmt.Sprintf("%s.transcoding-%d.mp4", opts.TargetPath, time.Now().UnixNano())
	content := []byte(fmt.Sprintf("MOCK_DIRECT_PLAY_MP4_CONTENT_%s_%s", opts.Profile, hw))
	if err := os.WriteFile(tmpOutput, content, 0644); err != nil {
		return nil, fmt.Errorf("failed to write mock transcode output: %w", err)
	}

	if err := os.Rename(tmpOutput, opts.TargetPath); err != nil {
		return nil, fmt.Errorf("failed to atomically place mock output: %w", err)
	}

	if opts.ProgressCb != nil {
		opts.ProgressCb(100.0)
	}

	encoderUsed := string(hw)
	if hw == HardwareSoftware || hw == "" {
		encoderUsed = "cpu-x264"
	}

	return &TranscodeResult{
		TargetPath:  opts.TargetPath,
		EncoderUsed: encoderUsed,
		Duration:    time.Since(start),
	}, nil
}

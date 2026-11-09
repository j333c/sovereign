package transcode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

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

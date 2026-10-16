package transcode

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrInvalidSchedule = errors.New("invalid schedule configuration: hours must be 0-23 and minutes 0-59")
)

const (
	GPUStatusDaytime = "Daytime (AI Priority)"
	GPUStatusOffPeak = "Off-Peak (Transcoding Active)"
)

// ScheduleConfig defines the daily time window for off-peak batch transcoding.
type ScheduleConfig struct {
	StartHour      int            `json:"start_hour"`
	StartMinute    int            `json:"start_minute"`
	EndHour        int            `json:"end_hour"`
	EndMinute      int            `json:"end_minute"`
	ManualOverride bool           `json:"manual_override"`
	Location       *time.Location `json:"-"`
}

// DefaultScheduleConfig returns the default 01:00 to 07:00 off-peak schedule.
func DefaultScheduleConfig() ScheduleConfig {
	return ScheduleConfig{
		StartHour:      1,
		StartMinute:    0,
		EndHour:        7,
		EndMinute:      0,
		ManualOverride: false,
		Location:       time.Local,
	}
}

// Scheduler evaluates whether batch transcoding should run based on the daily off-peak window
// and manual user override state.
type Scheduler struct {
	mu             sync.RWMutex
	startHour      int
	startMinute    int
	endHour        int
	endMinute      int
	manualOverride bool
	location       *time.Location
	nowFunc        func() time.Time
}

// NewScheduler creates and initializes a new Scheduler.
func NewScheduler(cfg ScheduleConfig) (*Scheduler, error) {
	s := &Scheduler{
		nowFunc: time.Now,
	}
	if err := s.SetConfig(cfg); err != nil {
		return nil, err
	}
	return s, nil
}

// SetNowFunc overrides the current time supplier (useful for deterministic unit testing).
func (s *Scheduler) SetNowFunc(fn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if fn != nil {
		s.nowFunc = fn
	} else {
		s.nowFunc = time.Now
	}
}

// SetConfig updates the schedule configuration with validation.
func (s *Scheduler) SetConfig(cfg ScheduleConfig) error {
	if cfg.StartHour < 0 || cfg.StartHour > 23 || cfg.EndHour < 0 || cfg.EndHour > 23 {
		return ErrInvalidSchedule
	}
	if cfg.StartMinute < 0 || cfg.StartMinute > 59 || cfg.EndMinute < 0 || cfg.EndMinute > 59 {
		return ErrInvalidSchedule
	}

	loc := cfg.Location
	if loc == nil {
		loc = time.Local
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.startHour = cfg.StartHour
	s.startMinute = cfg.StartMinute
	s.endHour = cfg.EndHour
	s.endMinute = cfg.EndMinute
	s.manualOverride = cfg.ManualOverride
	s.location = loc

	return nil
}

// GetConfig returns a copy of current schedule settings.
func (s *Scheduler) GetConfig() ScheduleConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return ScheduleConfig{
		StartHour:      s.startHour,
		StartMinute:    s.startMinute,
		EndHour:        s.endHour,
		EndMinute:      s.endMinute,
		ManualOverride: s.manualOverride,
		Location:       s.location,
	}
}

// SetManualOverride enables or disables manual override bypassing the daytime restriction.
func (s *Scheduler) SetManualOverride(force bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.manualOverride = force
}

// ManualOverride reports whether manual override is currently active.
func (s *Scheduler) ManualOverride() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.manualOverride
}

// IsOffPeak reports whether the given timestamp falls within the configured off-peak window.
func (s *Scheduler) IsOffPeak(t time.Time) bool {
	s.mu.RLock()
	startH := s.startHour
	startM := s.startMinute
	endH := s.endHour
	endM := s.endMinute
	loc := s.location
	s.mu.RUnlock()

	if loc != nil {
		t = t.In(loc)
	}

	curMin := t.Hour()*60 + t.Minute()
	startMin := startH*60 + startM
	endMin := endH*60 + endM

	if startMin < endMin {
		// Normal window, e.g. 01:00 to 07:00
		return curMin >= startMin && curMin < endMin
	} else if startMin > endMin {
		// Spans midnight, e.g. 23:00 to 06:00
		return curMin >= startMin || curMin < endMin
	} else {
		// start == end: 0-duration window
		return false
	}
}

// IsOffPeakNow reports whether the current time is in the off-peak window.
func (s *Scheduler) IsOffPeakNow() bool {
	s.mu.RLock()
	now := s.nowFunc()
	s.mu.RUnlock()
	return s.IsOffPeak(now)
}

// IsActive reports whether batch transcoding should run right now, considering
// both scheduled off-peak hours and any manual override.
func (s *Scheduler) IsActive(t time.Time) bool {
	s.mu.RLock()
	override := s.manualOverride
	s.mu.RUnlock()

	if override {
		return true
	}
	return s.IsOffPeak(t)
}

// IsActiveNow reports whether batch transcoding is currently active.
func (s *Scheduler) IsActiveNow() bool {
	s.mu.RLock()
	now := s.nowFunc()
	s.mu.RUnlock()
	return s.IsActive(now)
}

// GetGPUStatus returns the formatted status string for SSE telemetry ("gpu_status").
func (s *Scheduler) GetGPUStatus(t time.Time) string {
	if s.IsActive(t) {
		return GPUStatusOffPeak
	}
	return GPUStatusDaytime
}

// GetGPUStatusNow returns the formatted status string for the current time.
func (s *Scheduler) GetGPUStatusNow() string {
	s.mu.RLock()
	now := s.nowFunc()
	s.mu.RUnlock()
	return s.GetGPUStatus(now)
}

// String returns a human-readable representation of the schedule window.
func (s *Scheduler) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return fmt.Sprintf("%02d:%02d - %02d:%02d (override=%v)", s.startHour, s.startMinute, s.endHour, s.endMinute, s.manualOverride)
}

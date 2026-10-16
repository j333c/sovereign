package ai

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// Dynamic GPU Arbitrator state definitions.
const (
	ArbitratorStateReady           = "ready"
	ArbitratorStateWarmingUp       = "warming_up"
	ArbitratorStateIdleSurrendered = "idle_surrendered"
	ArbitratorStateSafeStub        = "safe_stub"

	DefaultIdleTimeout = 300 * time.Second
	DefaultWakeTimeout = 90 * time.Second

	ReassuranceWarmingUp    = "Waking up private AI (ready in 5s)..."
	ReassuranceCPUOptimized = "CPU-Optimized Mode"
)

var (
	ErrWakeTimeout = errors.New("ai warm-up timeout expired")
	ErrSafeStub    = errors.New("ai safe stub mode active; no models provisioned")
)

// ArbitratorStatus contains current telemetry and state of the GPU Arbitrator.
type ArbitratorStatus struct {
	State              string            `json:"state"`
	HardwareMode       string            `json:"hardware_mode"`
	IsGPU              bool              `json:"is_gpu"`
	IsHybrid           bool              `json:"is_hybrid"`
	CPUOptimized       bool              `json:"cpu_optimized"`
	IsDaytime          bool              `json:"is_daytime"`
	IdleSeconds        int               `json:"idle_seconds"`
	IdleTimeoutSeconds int               `json:"idle_timeout_seconds"`
	ReassuranceMessage string            `json:"reassurance_message,omitempty"`
	HybridAssignments  map[string]string `json:"hybrid_assignments,omitempty"`
}

// ArbitratorConfig configures the GPUArbitrator.
type ArbitratorConfig struct {
	IdleTimeout       time.Duration
	WakeTimeout       time.Duration
	TickInterval      time.Duration
	NowFunc           func() time.Time
	UnloadFunc        func(ctx context.Context) error
	WakeFunc          func(ctx context.Context) error
	OnStateChange     func(oldState, newState string)
	HardwareMode      string
	IsGPU             bool
	IsHybrid          bool
	CPUOptimized      bool
	HybridAssignments map[string]string
	Logger            *slog.Logger
	InitialState      string
}

// GPUArbitrator manages dynamic VRAM surrender (300s inactivity), day/night scheduling,
// and smooth daytime wake-up transitions.
type GPUArbitrator struct {
	mu                sync.RWMutex
	state             string
	idleTimeout       time.Duration
	wakeTimeout       time.Duration
	tickInterval      time.Duration
	nowFunc           func() time.Time
	unloadFunc        func(ctx context.Context) error
	wakeFunc          func(ctx context.Context) error
	onStateChange     func(oldState, newState string)
	hardwareMode      string
	isGPU             bool
	isHybrid          bool
	cpuOptimized      bool
	hybridAssignments map[string]string
	logger            *slog.Logger

	inFlight     int
	lastActivity time.Time
	wakeDone     chan struct{}
	wakeErr      error

	stopChan  chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// NewGPUArbitrator instantiates and starts a GPUArbitrator.
func NewGPUArbitrator(cfg ArbitratorConfig) *GPUArbitrator {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = DefaultIdleTimeout
	}
	if cfg.WakeTimeout <= 0 {
		cfg.WakeTimeout = DefaultWakeTimeout
	}
	if cfg.TickInterval <= 0 {
		cfg.TickInterval = 1 * time.Second
	}
	nowFn := cfg.NowFunc
	if nowFn == nil {
		nowFn = time.Now
	}

	initialState := cfg.InitialState
	if initialState == "" {
		initialState = ArbitratorStateReady
	}

	hwMode := cfg.HardwareMode
	if hwMode == "" {
		if cfg.IsGPU {
			hwMode = "gpu"
		} else {
			hwMode = "cpu"
		}
	}

	a := &GPUArbitrator{
		state:             initialState,
		idleTimeout:       cfg.IdleTimeout,
		wakeTimeout:       cfg.WakeTimeout,
		tickInterval:      cfg.TickInterval,
		nowFunc:           nowFn,
		unloadFunc:        cfg.UnloadFunc,
		wakeFunc:          cfg.WakeFunc,
		onStateChange:     cfg.OnStateChange,
		hardwareMode:      hwMode,
		isGPU:             cfg.IsGPU,
		isHybrid:          cfg.IsHybrid,
		cpuOptimized:      cfg.CPUOptimized,
		hybridAssignments: cfg.HybridAssignments,
		logger:            cfg.Logger,
		lastActivity:      nowFn(),
		stopChan:          make(chan struct{}),
	}

	a.wg.Add(1)
	go a.runTicker()

	return a
}

func (a *GPUArbitrator) now() time.Time {
	if a.nowFunc != nil {
		return a.nowFunc()
	}
	return time.Now()
}

// IsDaytime returns true if the specified time falls within daytime hours (07:00 to 01:00).
// Nighttime off-peak hours are 01:00 to 07:00.
func IsDaytime(t time.Time) bool {
	hour := t.Hour()
	return hour >= 7 || hour < 1
}

// IsCurrentDaytime returns whether the arbitrator currently considers it daytime.
func (a *GPUArbitrator) IsCurrentDaytime() bool {
	return IsDaytime(a.now())
}

// RecordActivity updates the last activity timestamp, resetting the idle countdown.
func (a *GPUArbitrator) RecordActivity() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastActivity = a.now()
}

// BeginInference marks an in-flight inference request to prevent idle VRAM surrender during generation.
// Returns a release function that must be deferred until generation finishes.
func (a *GPUArbitrator) BeginInference() func() {
	a.mu.Lock()
	a.inFlight++
	a.lastActivity = a.now()
	a.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			a.mu.Lock()
			a.inFlight--
			if a.inFlight < 0 {
				a.inFlight = 0
			}
			a.lastActivity = a.now()
			a.mu.Unlock()
		})
	}
}

// State returns the current arbitrator state.
func (a *GPUArbitrator) State() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// SetState explicitly sets arbitrator state and fires onStateChange if transitioned.
func (a *GPUArbitrator) SetState(newState string) {
	a.mu.Lock()
	oldState := a.state
	if oldState == newState {
		a.mu.Unlock()
		return
	}
	a.state = newState
	onStateChange := a.onStateChange
	a.mu.Unlock()

	if onStateChange != nil {
		onStateChange(oldState, newState)
	}
}

// SurrenderVRAM unconditionally unloads model weights or releases GPU memory.
// Remains completely resilient and idempotent if already surrendered.
func (a *GPUArbitrator) SurrenderVRAM(ctx context.Context) error {
	a.mu.Lock()
	if a.state == ArbitratorStateIdleSurrendered {
		a.mu.Unlock()
		return nil
	}
	if a.state == ArbitratorStateSafeStub {
		a.mu.Unlock()
		return nil
	}
	if a.state == ArbitratorStateWarmingUp {
		a.mu.Unlock()
		return errors.New("cannot surrender VRAM while model is warming up")
	}
	if a.inFlight > 0 {
		a.mu.Unlock()
		return errors.New("cannot surrender VRAM while inference is in flight")
	}
	oldState := a.state
	unload := a.unloadFunc
	onStateChange := a.onStateChange
	a.mu.Unlock()

	if unload != nil {
		if err := unload(ctx); err != nil {
			a.logger.Warn("gpu arbitrator: unload func returned error", "error", err)
			return err
		}
	}

	a.mu.Lock()
	if a.state == ArbitratorStateWarmingUp {
		a.mu.Unlock()
		return errors.New("cannot surrender VRAM: state transitioned to warming_up")
	}
	a.state = ArbitratorStateIdleSurrendered
	a.mu.Unlock()

	a.logger.Info("gpu arbitrator: surrendered VRAM after idle inactivity", "idle_seconds", a.idleTimeout.Seconds())

	if onStateChange != nil {
		onStateChange(oldState, ArbitratorStateIdleSurrendered)
	}

	return nil
}

// EnsureReady ensures the AI model is loaded and ready to serve inference requests.
// If the model is idle_surrendered, initiates a warm-up sequence with extended buffer (>=60s).
// Multiple simultaneous waking requests safely wait for the ongoing warm-up to finish.
func (a *GPUArbitrator) EnsureReady(ctx context.Context) error {
	a.mu.Lock()

	if a.state == ArbitratorStateSafeStub {
		a.mu.Unlock()
		return ErrSafeStub
	}

	if a.state == ArbitratorStateReady {
		a.lastActivity = a.now()
		a.mu.Unlock()
		return nil
	}

	if a.state == ArbitratorStateWarmingUp {
		// Another request is actively warming up the model; wait for readiness
		doneCh := a.wakeDone
		a.mu.Unlock()

		select {
		case <-doneCh:
			a.mu.RLock()
			err := a.wakeErr
			a.mu.RUnlock()
			if err == nil {
				a.RecordActivity()
			}
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// State is idle_surrendered: initiate warm-up transition
	oldState := a.state
	a.state = ArbitratorStateWarmingUp
	doneCh := make(chan struct{})
	a.wakeDone = doneCh
	a.wakeErr = nil
	onStateChange := a.onStateChange
	wake := a.wakeFunc
	wakeTimeout := a.wakeTimeout
	a.mu.Unlock()

	a.logger.Info("gpu arbitrator: initiating warm-up from surrendered idle state", "wake_timeout", wakeTimeout)

	if onStateChange != nil {
		onStateChange(oldState, ArbitratorStateWarmingUp)
	}

	wakeResultCh := make(chan error, 1)
	go func() {
		wakeCtx, cancel := context.WithTimeout(context.Background(), wakeTimeout)
		defer cancel()
		if wake != nil {
			wakeResultCh <- wake(wakeCtx)
		} else {
			wakeResultCh <- nil
		}
	}()

	select {
	case wakeErr := <-wakeResultCh:
		a.mu.Lock()
		var finalState string
		if wakeErr == nil {
			a.state = ArbitratorStateReady
			a.lastActivity = a.now()
			a.wakeErr = nil
			finalState = ArbitratorStateReady
		} else {
			if errors.Is(wakeErr, context.DeadlineExceeded) {
				wakeErr = ErrWakeTimeout
			}
			a.state = ArbitratorStateIdleSurrendered
			a.wakeErr = wakeErr
			finalState = ArbitratorStateIdleSurrendered
		}
		close(doneCh)
		a.wakeDone = nil
		a.mu.Unlock()

		if onStateChange != nil {
			onStateChange(ArbitratorStateWarmingUp, finalState)
		}

		if wakeErr != nil {
			a.logger.Warn("gpu arbitrator: warm-up failed", "error", wakeErr)
			return wakeErr
		}

		a.logger.Info("gpu arbitrator: warm-up completed successfully; model is ready")
		return nil

	case <-ctx.Done():
		// Caller disconnected or timed out. Keep background wake running to completion to preserve state consistency.
		go func() {
			err := <-wakeResultCh
			a.mu.Lock()
			var finalState string
			if err == nil {
				a.state = ArbitratorStateReady
				a.lastActivity = a.now()
				a.wakeErr = nil
				finalState = ArbitratorStateReady
			} else {
				if errors.Is(err, context.DeadlineExceeded) {
					err = ErrWakeTimeout
				}
				a.state = ArbitratorStateIdleSurrendered
				a.wakeErr = err
				finalState = ArbitratorStateIdleSurrendered
			}
			close(doneCh)
			a.wakeDone = nil
			a.mu.Unlock()

			if onStateChange != nil {
				onStateChange(ArbitratorStateWarmingUp, finalState)
			}
		}()
		return ctx.Err()
	}
}

// Status returns a snapshot of current arbitrator telemetry and state.
func (a *GPUArbitrator) Status() ArbitratorStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	now := a.now()
	idleSec := int(now.Sub(a.lastActivity).Seconds())
	if idleSec < 0 {
		idleSec = 0
	}

	var reassuranceMsg string
	if a.state == ArbitratorStateWarmingUp {
		reassuranceMsg = ReassuranceWarmingUp
	} else if a.cpuOptimized {
		reassuranceMsg = ReassuranceCPUOptimized
	}

	return ArbitratorStatus{
		State:              a.state,
		HardwareMode:       a.hardwareMode,
		IsGPU:              a.isGPU,
		IsHybrid:           a.isHybrid,
		CPUOptimized:       a.cpuOptimized,
		IsDaytime:          IsDaytime(now),
		IdleSeconds:        idleSec,
		IdleTimeoutSeconds: int(a.idleTimeout.Seconds()),
		ReassuranceMessage: reassuranceMsg,
		HybridAssignments:  a.hybridAssignments,
	}
}

// Close stops the arbitrator background ticker.
func (a *GPUArbitrator) Close() {
	a.closeOnce.Do(func() {
		close(a.stopChan)
	})
	a.wg.Wait()
}

func (a *GPUArbitrator) runTicker() {
	defer a.wg.Done()

	ticker := time.NewTicker(a.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopChan:
			return
		case <-ticker.C:
			a.mu.Lock()
			if a.state == ArbitratorStateReady && a.inFlight == 0 {
				now := a.now()
				if now.Sub(a.lastActivity) >= a.idleTimeout {
					a.mu.Unlock()
					_ = a.SurrenderVRAM(context.Background())
					continue
				}
			}
			a.mu.Unlock()
		}
	}
}

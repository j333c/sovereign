package ai

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	// ErrQueueFull indicates the governor queue capacity was exceeded.
	ErrQueueFull = errors.New("inference queue is full")

	// ErrQueueTimeout indicates a request waited too long in the governor queue.
	ErrQueueTimeout = errors.New("inference queue wait timeout expired")
)

// GovernorConfig configures the Two-Lane Priority Request Governor.
type GovernorConfig struct {
	TotalSlots          int           // Total execution slots (default: 2)
	ReservedInteractive int           // Slots reserved for interactive requests (default: 1)
	MaxBackgroundQueue  int           // Max queued background requests (default: 50)
	MaxInteractiveQueue int           // Max queued interactive requests (default: 50)
	QueueTimeout        time.Duration // Wait timeout if not set on context (default: 60s)
}

// DefaultGovernorConfig returns recommended appliance defaults.
func DefaultGovernorConfig() GovernorConfig {
	return GovernorConfig{
		TotalSlots:          2,
		ReservedInteractive: 1,
		MaxBackgroundQueue:  50,
		MaxInteractiveQueue: 50,
		QueueTimeout:        60 * time.Second,
	}
}

// GovernorStatus represents a snapshot of the RequestGovernor's operational state.
type GovernorStatus struct {
	ActiveInteractive   int   `json:"active_interactive"`
	ActiveBackground    int   `json:"active_background"`
	QueuedInteractive   int   `json:"queued_interactive"`
	QueuedBackground    int   `json:"queued_background"`
	TotalSlots          int   `json:"total_slots"`
	ReservedInteractive int   `json:"reserved_interactive"`
	TotalServed         int64 `json:"total_served"`
	TotalCanceled       int64 `json:"total_canceled"`
	TotalRejected       int64 `json:"total_rejected"`
	TotalTimedOut       int64 `json:"total_timed_out"`
}

// RequestGovernor implements a Two-Lane Priority Request Governor coordinating
// inference slots between high-priority interactive human chat and lower-priority
// background automated tasks.
type RequestGovernor struct {
	mu                  sync.Mutex
	totalSlots          int
	reservedInteractive int
	maxBackgroundQueue  int
	maxInteractiveQueue int
	queueTimeout        time.Duration

	activeInteractive int
	activeBackground  int
	queuedInteractive int
	queuedBackground  int

	interactiveWaiters []chan struct{}
	backgroundWaiters  []chan struct{}

	totalServed   int64
	totalCanceled int64
	totalRejected int64
	totalTimedOut int64
}

// NewRequestGovernor creates and initializes a RequestGovernor with the given config.
func NewRequestGovernor(cfg GovernorConfig) *RequestGovernor {
	if cfg.TotalSlots <= 0 {
		cfg.TotalSlots = 2
	}
	if cfg.ReservedInteractive < 0 {
		cfg.ReservedInteractive = 1
	}
	if cfg.ReservedInteractive >= cfg.TotalSlots {
		cfg.ReservedInteractive = cfg.TotalSlots - 1
		if cfg.ReservedInteractive < 0 {
			cfg.ReservedInteractive = 0
		}
	}
	if cfg.MaxBackgroundQueue <= 0 {
		cfg.MaxBackgroundQueue = 50
	}
	if cfg.MaxInteractiveQueue <= 0 {
		cfg.MaxInteractiveQueue = 50
	}
	if cfg.QueueTimeout <= 0 {
		cfg.QueueTimeout = 60 * time.Second
	}

	return &RequestGovernor{
		totalSlots:          cfg.TotalSlots,
		reservedInteractive: cfg.ReservedInteractive,
		maxBackgroundQueue:  cfg.MaxBackgroundQueue,
		maxInteractiveQueue: cfg.MaxInteractiveQueue,
		queueTimeout:        cfg.QueueTimeout,
	}
}

// Acquire requests an execution slot for the specified priority lane.
// If a slot is not immediately available, it queues the request respecting
// priority (interactive requests jump ahead of background requests).
// Returns a release function that MUST be invoked when inference completes.
func (g *RequestGovernor) Acquire(ctx context.Context, priority InferencePriority) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	g.mu.Lock()

	if priority == PriorityInteractive {
		// Interactive lane can acquire immediately if total active slots < totalSlots
		// and no prior interactive requests are waiting in queue.
		if g.activeInteractive+g.activeBackground < g.totalSlots && len(g.interactiveWaiters) == 0 {
			g.activeInteractive++
			g.mu.Unlock()
			return g.makeReleaseFunc(ctx, PriorityInteractive), nil
		}

		// Check interactive queue capacity
		if len(g.interactiveWaiters) >= g.maxInteractiveQueue {
			g.totalRejected++
			g.mu.Unlock()
			return nil, ErrQueueFull
		}

		waitCh := make(chan struct{})
		g.interactiveWaiters = append(g.interactiveWaiters, waitCh)
		g.queuedInteractive++
		g.mu.Unlock()

		return g.waitForSlot(ctx, PriorityInteractive, waitCh)
	}

	// PriorityBackground lane:
	// Background requests can acquire immediately only if:
	// 1. Total active slots < totalSlots - reservedInteractive (reserving capacity for interactive chat)
	// 2. Zero interactive requests are waiting in queue
	// 3. Zero background requests are already waiting ahead in queue
	if (g.activeInteractive+g.activeBackground < g.totalSlots-g.reservedInteractive) &&
		g.queuedInteractive == 0 &&
		len(g.backgroundWaiters) == 0 {
		g.activeBackground++
		g.mu.Unlock()
		return g.makeReleaseFunc(ctx, PriorityBackground), nil
	}

	// Check background queue capacity
	if len(g.backgroundWaiters) >= g.maxBackgroundQueue {
		g.totalRejected++
		g.mu.Unlock()
		return nil, ErrQueueFull
	}

	waitCh := make(chan struct{})
	g.backgroundWaiters = append(g.backgroundWaiters, waitCh)
	g.queuedBackground++
	g.mu.Unlock()

	return g.waitForSlot(ctx, PriorityBackground, waitCh)
}

func (g *RequestGovernor) waitForSlot(ctx context.Context, priority InferencePriority, waitCh chan struct{}) (func(), error) {
	var timeoutCh <-chan time.Time
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && g.queueTimeout > 0 {
		timer := time.NewTimer(g.queueTimeout)
		defer timer.Stop()
		timeoutCh = timer.C
	}

	select {
	case <-waitCh:
		// Slot was granted by scheduleNext (active count was already incremented)
		return g.makeReleaseFunc(ctx, priority), nil

	case <-ctx.Done():
		g.mu.Lock()
		select {
		case <-waitCh:
			// Acquired just as context finished; release immediately
			if priority == PriorityInteractive {
				g.activeInteractive--
			} else {
				g.activeBackground--
			}
			g.totalCanceled++
			g.scheduleNext()
		default:
			g.removeWaiter(priority, waitCh)
			g.scheduleNext()
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			g.totalTimedOut++
			g.mu.Unlock()
			return nil, ErrQueueTimeout
		}
		g.mu.Unlock()
		return nil, ctx.Err()

	case <-timeoutCh:
		g.mu.Lock()
		select {
		case <-waitCh:
			if priority == PriorityInteractive {
				g.activeInteractive--
			} else {
				g.activeBackground--
			}
			g.totalTimedOut++
			g.scheduleNext()
		default:
			g.removeWaiter(priority, waitCh)
			g.totalTimedOut++
			g.scheduleNext()
		}
		g.mu.Unlock()
		return nil, ErrQueueTimeout
	}
}

func (g *RequestGovernor) makeReleaseFunc(ctx context.Context, priority InferencePriority) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			defer g.mu.Unlock()

			if priority == PriorityInteractive {
				g.activeInteractive--
			} else {
				g.activeBackground--
			}
			if ctx != nil && ctx.Err() != nil {
				g.totalCanceled++
			} else {
				g.totalServed++
			}
			g.scheduleNext()
		})
	}
}

// scheduleNext assigns free slots to pending requests, strictly prioritizing
// Lane 1 (interactive) over Lane 2 (background). Must be called with g.mu held.
func (g *RequestGovernor) scheduleNext() {
	for {
		// 1. Interactive lane takes priority over all queued requests
		if len(g.interactiveWaiters) > 0 && g.activeInteractive+g.activeBackground < g.totalSlots {
			ch := g.interactiveWaiters[0]
			g.interactiveWaiters = g.interactiveWaiters[1:]
			g.queuedInteractive--
			g.activeInteractive++
			close(ch)
			continue
		}

		// 2. Background lane can only proceed if:
		// - No interactive requests are queued
		// - Active slots do not exceed (totalSlots - reservedInteractive)
		if len(g.interactiveWaiters) == 0 &&
			len(g.backgroundWaiters) > 0 &&
			(g.activeInteractive+g.activeBackground < g.totalSlots-g.reservedInteractive) {
			ch := g.backgroundWaiters[0]
			g.backgroundWaiters = g.backgroundWaiters[1:]
			g.queuedBackground--
			g.activeBackground++
			close(ch)
			continue
		}

		break
	}
}

func (g *RequestGovernor) removeWaiter(priority InferencePriority, target chan struct{}) {
	if priority == PriorityInteractive {
		for i, ch := range g.interactiveWaiters {
			if ch == target {
				g.interactiveWaiters = append(g.interactiveWaiters[:i], g.interactiveWaiters[i+1:]...)
				g.queuedInteractive--
				return
			}
		}
		return
	}

	for i, ch := range g.backgroundWaiters {
		if ch == target {
			g.backgroundWaiters = append(g.backgroundWaiters[:i], g.backgroundWaiters[i+1:]...)
			g.queuedBackground--
			return
		}
	}
}

// Status returns a point-in-time snapshot of the governor's queue and execution metrics.
func (g *RequestGovernor) Status() GovernorStatus {
	g.mu.Lock()
	defer g.mu.Unlock()

	return GovernorStatus{
		ActiveInteractive:   g.activeInteractive,
		ActiveBackground:    g.activeBackground,
		QueuedInteractive:   g.queuedInteractive,
		QueuedBackground:    g.queuedBackground,
		TotalSlots:          g.totalSlots,
		ReservedInteractive: g.reservedInteractive,
		TotalServed:         g.totalServed,
		TotalCanceled:       g.totalCanceled,
		TotalRejected:       g.totalRejected,
		TotalTimedOut:       g.totalTimedOut,
	}
}

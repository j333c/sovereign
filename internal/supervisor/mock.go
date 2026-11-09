package supervisor

import (
	"context"
	"fmt"
	"sync"
)

// MockRuntimeEngine provides an in-memory, deterministic implementation of RuntimeEngine.
type MockRuntimeEngine struct {
	mu                 sync.RWMutex
	images             map[string]bool
	containers         map[string]*ContainerInfo
	pullFailures       map[string]error
	probeHealthMap     map[string]bool
	pulledHistory      []string
	startedHistory     []ContainerSpec
	stoppedHistory     []string
	restartedHistory   []string
	OnPullImage        func(ctx context.Context, image string, progressCb func(percent float64, status string)) error
	OnStartContainer   func(ctx context.Context, spec ContainerSpec) error
	OnStopContainer    func(ctx context.Context, name string) error
	OnRestartContainer func(ctx context.Context, name string) error
	OnProbeHealth      func(ctx context.Context, name string) (bool, error)
}

// NewMockRuntimeEngine creates a new mock runtime engine.
func NewMockRuntimeEngine() *MockRuntimeEngine {
	return &MockRuntimeEngine{
		images:         make(map[string]bool),
		containers:     make(map[string]*ContainerInfo),
		pullFailures:   make(map[string]error),
		probeHealthMap: make(map[string]bool),
	}
}

func (m *MockRuntimeEngine) SetImagePresent(image string, present bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.images[image] = present
}

func (m *MockRuntimeEngine) SetPullFailure(image string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err == nil {
		delete(m.pullFailures, image)
	} else {
		m.pullFailures[image] = err
	}
}

func (m *MockRuntimeEngine) SetContainerHealth(name string, healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.probeHealthMap[name] = healthy
}

func (m *MockRuntimeEngine) PullImage(ctx context.Context, image string, progressCb func(percent float64, status string)) error {
	if m.OnPullImage != nil {
		return m.OnPullImage(ctx, image, progressCb)
	}

	m.mu.Lock()
	if err, exists := m.pullFailures[image]; exists && err != nil {
		m.mu.Unlock()
		return err
	}
	m.mu.Unlock()

	// Simulate streaming progress updates
	steps := []struct {
		percent float64
		status  string
	}{
		{10.0, "Waiting to pull image"},
		{35.0, "Downloading layers"},
		{70.0, "Extracting layers"},
		{100.0, "Image pull complete"},
	}

	for _, step := range steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if progressCb != nil {
				progressCb(step.percent, step.status)
			}
		}
	}

	m.mu.Lock()
	m.images[image] = true
	m.pulledHistory = append(m.pulledHistory, image)
	m.mu.Unlock()

	return nil
}

func (m *MockRuntimeEngine) IsImagePresent(ctx context.Context, image string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.images[image], nil
}

func (m *MockRuntimeEngine) StartContainer(ctx context.Context, spec ContainerSpec) error {
	if m.OnStartContainer != nil {
		return m.OnStartContainer(ctx, spec)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	cName := containerName(spec.Name)
	info := &ContainerInfo{
		ID:      "mock-" + cName,
		Name:    cName,
		Image:   spec.Image,
		Running: true,
		Status:  "running",
		Spec:    spec,
	}
	m.containers[spec.Name] = info
	m.containers[cName] = info
	m.probeHealthMap[spec.Name] = true
	m.probeHealthMap[cName] = true
	m.startedHistory = append(m.startedHistory, spec)
	return nil
}

func (m *MockRuntimeEngine) StopContainer(ctx context.Context, name string) error {
	if m.OnStopContainer != nil {
		return m.OnStopContainer(ctx, name)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	cName := containerName(name)
	if info, exists := m.containers[name]; exists {
		info.Running = false
		info.Status = "stopped"
	}
	if info, exists := m.containers[cName]; exists {
		info.Running = false
		info.Status = "stopped"
	}
	m.stoppedHistory = append(m.stoppedHistory, name)
	return nil
}

func (m *MockRuntimeEngine) RestartContainer(ctx context.Context, name string) error {
	if m.OnRestartContainer != nil {
		return m.OnRestartContainer(ctx, name)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	cName := containerName(name)
	if info, exists := m.containers[name]; exists {
		info.Running = true
		info.Status = "running"
	}
	if info, exists := m.containers[cName]; exists {
		info.Running = true
		info.Status = "running"
	}
	m.probeHealthMap[name] = true
	m.probeHealthMap[cName] = true
	m.restartedHistory = append(m.restartedHistory, name)
	return nil
}

func (m *MockRuntimeEngine) InspectContainer(ctx context.Context, name string) (*ContainerInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if info, exists := m.containers[name]; exists {
		return info, nil
	}
	cName := containerName(name)
	if info, exists := m.containers[cName]; exists {
		return info, nil
	}
	return nil, fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntimeEngine) ProbeHealth(ctx context.Context, name string) (bool, error) {
	if m.OnProbeHealth != nil {
		return m.OnProbeHealth(ctx, name)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	info, exists := m.containers[name]
	if !exists {
		info, exists = m.containers[containerName(name)]
	}
	if !exists || !info.Running {
		return false, nil
	}

	if healthy, exists := m.probeHealthMap[name]; exists {
		return healthy, nil
	}
	if healthy, exists := m.probeHealthMap[containerName(name)]; exists {
		return healthy, nil
	}
	return true, nil
}

// GetPulledHistory returns history of pulled images for assertion.
func (m *MockRuntimeEngine) GetPulledHistory() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]string, len(m.pulledHistory))
	copy(res, m.pulledHistory)
	return res
}

// GetStartedHistory returns history of started containers.
func (m *MockRuntimeEngine) GetStartedHistory() []ContainerSpec {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]ContainerSpec, len(m.startedHistory))
	copy(res, m.startedHistory)
	return res
}

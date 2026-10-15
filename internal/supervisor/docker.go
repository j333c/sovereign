package supervisor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ContainerSpec defines the deployment parameters for a managed container spoke.
type ContainerSpec struct {
	Name                   string            `yaml:"name"`
	Image                  string            `yaml:"image"`
	MemoryLimitBytes       int64             `yaml:"mem_limit_bytes"`
	MemoryReservationBytes int64             `yaml:"mem_reservation_bytes"`
	Ports                  map[int]int       `yaml:"ports"`   // hostPort: containerPort
	Volumes                []string          `yaml:"volumes"` // hostPath:containerPath[:ro]
	Networks               []string          `yaml:"networks"`
	Env                    map[string]string `yaml:"environment,omitempty"`
}

// ContainerInfo describes the runtime status of a container instance.
type ContainerInfo struct {
	ID      string
	Name    string
	Image   string
	Running bool
	Status  string
	Spec    ContainerSpec
}

// RuntimeEngine defines container engine operations for on-demand lifecycle management.
type RuntimeEngine interface {
	PullImage(ctx context.Context, image string, progressCb func(percent float64, status string)) error
	IsImagePresent(ctx context.Context, image string) (bool, error)
	StartContainer(ctx context.Context, spec ContainerSpec) error
	StopContainer(ctx context.Context, name string) error
	RestartContainer(ctx context.Context, name string) error
	InspectContainer(ctx context.Context, name string) (*ContainerInfo, error)
	ProbeHealth(ctx context.Context, name string) (bool, error)
}

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

func containerName(name string) string {
	if strings.HasPrefix(name, "sovereign-") {
		return name
	}
	return "sovereign-" + name
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

// DockerEngine connects to the local Docker socket via Unix domain socket HTTP.
type DockerEngine struct {
	socketPath string
	client     *http.Client
	logger     *slog.Logger
}

// NewDockerEngine initializes a DockerEngine interacting with /var/run/docker.sock.
func NewDockerEngine(socketPath string, logger *slog.Logger) *DockerEngine {
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}
	if logger == nil {
		logger = slog.Default()
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		},
		Timeout: 300 * time.Second, // Long timeout for large image pulls
	}

	return &DockerEngine{
		socketPath: socketPath,
		client:     client,
		logger:     logger,
	}
}

// PullImage pulls the OCI image from upstream registry and parses JSON progress stream.
func (d *DockerEngine) PullImage(ctx context.Context, image string, progressCb func(percent float64, status string)) error {
	pullURL := fmt.Sprintf("http://docker/images/create?fromImage=%s", image)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pullURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create pull request: %w", err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to docker daemon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker pull failed with status %d: %s", resp.StatusCode, string(body))
	}

	type progressEntry struct {
		current int64
		total   int64
	}
	layers := make(map[string]progressEntry)
	var maxPercent float64

	// Stream progress lines (Docker emits JSON objects per newline)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		var msg struct {
			ID             string `json:"id"`
			Status         string `json:"status"`
			ProgressDetail struct {
				Current int64 `json:"current"`
				Total   int64 `json:"total"`
			} `json:"progressDetail"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(line, &msg); err == nil {
			if msg.Error != "" {
				return fmt.Errorf("docker pull error: %s", msg.Error)
			}
			if msg.ID != "" && msg.ProgressDetail.Total > 0 {
				layers[msg.ID] = progressEntry{
					current: msg.ProgressDetail.Current,
					total:   msg.ProgressDetail.Total,
				}
				var sumCurrent, sumTotal int64
				for _, l := range layers {
					sumCurrent += l.current
					sumTotal += l.total
				}
				if sumTotal > 0 {
					pct := (float64(sumCurrent) / float64(sumTotal)) * 100.0
					if pct > maxPercent {
						maxPercent = pct
					}
				}
			} else if strings.HasPrefix(msg.Status, "Status: Downloaded") || strings.HasPrefix(msg.Status, "Status: Image is up to date") {
				maxPercent = 100.0
			}

			if progressCb != nil && (msg.Status != "" || maxPercent > 0) {
				progressCb(maxPercent, msg.Status)
			}
		}
	}

	return scanner.Err()
}

// IsImagePresent checks if the image exists in local Docker image cache.
func (d *DockerEngine) IsImagePresent(ctx context.Context, image string) (bool, error) {
	reqURL := fmt.Sprintf("http://docker/images/%s/json", image)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("unexpected status checking image: %d", resp.StatusCode)
}

// StartContainer creates and starts a container matching ContainerSpec.
func (d *DockerEngine) StartContainer(ctx context.Context, spec ContainerSpec) error {
	cName := containerName(spec.Name)

	// First inspect if container exists
	existing, err := d.InspectContainer(ctx, cName)
	if err == nil && existing != nil {
		if existing.Running {
			return nil
		}
		// Start existing container
		startURL := fmt.Sprintf("http://docker/containers/%s/start", cName)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, startURL, nil)
		if err != nil {
			return err
		}
		resp, err := d.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			d.attachSecondaryNetworks(ctx, cName, spec.Networks)
			return nil
		}
	}

	// Create container with resource limits and volume binds
	type portBinding struct {
		HostIP   string `json:"HostIp"`
		HostPort string `json:"HostPort"`
	}

	portBindings := make(map[string][]portBinding)
	exposedPorts := make(map[string]struct{})
	for hostPort, containerPort := range spec.Ports {
		key := fmt.Sprintf("%d/tcp", containerPort)
		portBindings[key] = []portBinding{{
			HostIP:   "127.0.0.1",
			HostPort: fmt.Sprintf("%d", hostPort),
		}}
		exposedPorts[key] = struct{}{}
	}

	primaryNet := "sovereign-control"
	if len(spec.Networks) > 0 {
		primaryNet = spec.Networks[0]
	}

	createPayload := map[string]any{
		"Image":        spec.Image,
		"ExposedPorts": exposedPorts,
		"HostConfig": map[string]any{
			"Memory":            spec.MemoryLimitBytes,
			"MemoryReservation": spec.MemoryReservationBytes,
			"Binds":             spec.Volumes,
			"PortBindings":      portBindings,
			"RestartPolicy":     map[string]string{"Name": "unless-stopped"},
			"NetworkMode":       primaryNet,
		},
	}

	payloadBytes, err := json.Marshal(createPayload)
	if err != nil {
		return err
	}

	createURL := fmt.Sprintf("http://docker/containers/create?name=%s", cName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, createURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("container create failed status %d: %s", resp.StatusCode, string(body))
	}

	// Start created container
	startURL := fmt.Sprintf("http://docker/containers/%s/start", cName)
	startReq, err := http.NewRequestWithContext(ctx, http.MethodPost, startURL, nil)
	if err != nil {
		return err
	}
	startResp, err := d.client.Do(startReq)
	if err != nil {
		return err
	}
	defer startResp.Body.Close()

	if startResp.StatusCode < 200 || startResp.StatusCode >= 300 {
		body, _ := io.ReadAll(startResp.Body)
		return fmt.Errorf("container start failed status %d: %s", startResp.StatusCode, string(body))
	}

	d.attachSecondaryNetworks(ctx, cName, spec.Networks)
	return nil
}

func (d *DockerEngine) attachSecondaryNetworks(ctx context.Context, cName string, networks []string) {
	if len(networks) <= 1 {
		return
	}
	primaryNet := networks[0]
	for _, netName := range networks[1:] {
		if netName == primaryNet {
			continue
		}
		connectURL := fmt.Sprintf("http://docker/networks/%s/connect", netName)
		bodyData, _ := json.Marshal(map[string]string{"Container": cName})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, connectURL, bytes.NewReader(bodyData))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := d.client.Do(req)
		if err != nil {
			d.logger.Warn("failed to attach container to secondary network", "container", cName, "network", netName, "error", err)
			continue
		}
		_ = resp.Body.Close()
	}
}

// StopContainer stops running container instance.
func (d *DockerEngine) StopContainer(ctx context.Context, name string) error {
	cName := containerName(name)
	stopURL := fmt.Sprintf("http://docker/containers/%s/stop", cName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, stopURL, nil)
	if err != nil {
		return err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || (resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("stop container %s failed status %d: %s", name, resp.StatusCode, string(body))
}

// RestartContainer restarts container instance.
func (d *DockerEngine) RestartContainer(ctx context.Context, name string) error {
	cName := containerName(name)
	restartURL := fmt.Sprintf("http://docker/containers/%s/restart", cName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, restartURL, nil)
	if err != nil {
		return err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("restart container %s failed status %d: %s", name, resp.StatusCode, string(body))
	}
	return nil
}

// InspectContainer gets container details.
func (d *DockerEngine) InspectContainer(ctx context.Context, name string) (*ContainerInfo, error) {
	cName := containerName(name)
	url := fmt.Sprintf("http://docker/containers/%s/json", cName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("container not found")
	}

	var data struct {
		ID    string `json:"Id"`
		Name  string `json:"Name"`
		Image string `json:"Image"`
		State struct {
			Running bool   `json:"Running"`
			Status  string `json:"Status"`
		} `json:"State"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return &ContainerInfo{
		ID:      data.ID,
		Name:    strings.TrimPrefix(data.Name, "/"),
		Image:   data.Image,
		Running: data.State.Running,
		Status:  data.State.Status,
	}, nil
}

// ProbeHealth checks if the container is healthy by inspecting running state.
func (d *DockerEngine) ProbeHealth(ctx context.Context, name string) (bool, error) {
	info, err := d.InspectContainer(ctx, name)
	if err != nil {
		return false, err
	}
	return info.Running, nil
}

// ComposeServiceSpec represents a service entry in Docker Compose YAML.
type ComposeServiceSpec struct {
	Image         string            `yaml:"image"`
	ContainerName string            `yaml:"container_name"`
	Restart       string            `yaml:"restart"`
	Ports         []string          `yaml:"ports,omitempty"`
	Volumes       []string          `yaml:"volumes,omitempty"`
	Environment   map[string]string `yaml:"environment,omitempty"`
	Deploy        *ComposeDeploy    `yaml:"deploy,omitempty"`
	Networks      []string          `yaml:"networks,omitempty"`
}

type ComposeDeploy struct {
	Resources ComposeResources `yaml:"resources"`
}

type ComposeResources struct {
	Limits       ComposeResourceLimits `yaml:"limits"`
	Reservations ComposeResourceLimits `yaml:"reservations"`
}

type ComposeResourceLimits struct {
	Memory string `yaml:"memory"`
}

type ComposeFile struct {
	Services map[string]ComposeServiceSpec `yaml:"services"`
	Networks map[string]map[string]any     `yaml:"networks"`
}

// SynthesizeComposeSpec generates and atomically persists a Compose v2 spec for a service.
func SynthesizeComposeSpec(dataDir string, spec ContainerSpec) (string, error) {
	composeDir := filepath.Join(dataDir, "state", "compose")
	if err := os.MkdirAll(composeDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create compose directory: %w", err)
	}

	ports := make([]string, 0, len(spec.Ports))
	for hostPort, containerPort := range spec.Ports {
		ports = append(ports, fmt.Sprintf("127.0.0.1:%d:%d", hostPort, containerPort))
	}

	// Format memory bounds in megabytes for Compose compatibility
	limitMB := spec.MemoryLimitBytes / (1024 * 1024)
	resMB := spec.MemoryReservationBytes / (1024 * 1024)

	composeData := ComposeFile{
		Services: map[string]ComposeServiceSpec{
			spec.Name: {
				Image:         spec.Image,
				ContainerName: "sovereign-" + spec.Name,
				Restart:       "unless-stopped",
				Ports:         ports,
				Volumes:       spec.Volumes,
				Environment:   spec.Env,
				Deploy: &ComposeDeploy{
					Resources: ComposeResources{
						Limits: ComposeResourceLimits{
							Memory: fmt.Sprintf("%dM", limitMB),
						},
						Reservations: ComposeResourceLimits{
							Memory: fmt.Sprintf("%dM", resMB),
						},
					},
				},
				Networks: []string{"sovereign-ingress", "sovereign-control"},
			},
		},
		Networks: map[string]map[string]any{
			"sovereign-ingress": {"external": true},
			"sovereign-control": {"external": true},
		},
	}

	yamlBytes, err := yaml.Marshal(composeData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal compose yaml: %w", err)
	}

	destPath := filepath.Join(composeDir, spec.Name+".yaml")
	tmpPath := destPath + fmt.Sprintf(".tmp-%d", time.Now().UnixNano())

	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create temp compose file: %w", err)
	}

	if _, err := f.Write(yamlBytes); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write temp compose file: %w", err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to fsync temp compose file: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to atomically rename compose file: %w", err)
	}

	return destPath, nil
}

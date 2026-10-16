package supervisor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"sovereign/internal/ai"
)

// Official llama.cpp OCI image and canonical service ID.
const (
	ServiceLlama = "llama"
	ImageLlama   = "ghcr.io/ggml-org/llama.cpp:b11647"

	MemLimitLlama       int64 = 8192 * 1024 * 1024 // 8192 MB
	MemReservationLlama int64 = 1024 * 1024 * 1024 // 1024 MB

	DefaultLlamaPort = 8080
)

// LlamaProvisionResult contains synthesis and runtime parameters for starting the llama-server spoke.
type LlamaProvisionResult struct {
	ContainerSpec ContainerSpec
	HWAccel       HardwareDetectResult
	Devices       []string
	Threads       int
	IsGPU         bool
	ModelPath     string
	InSafeStub    bool
	CPUOptimized  bool
}

// SettingGetter provides access to key-value configuration settings.
type SettingGetter interface {
	GetSetting(ctx context.Context, key string) (string, error)
}

// LlamaProvisionerOptions allows dependency injection for hardware and core detection during testing.
type LlamaProvisionerOptions struct {
	NumCPU       func() int
	DevRoot      string
	SysfsRoot    string
	ProcRoot     string
	ModelsDir    string
	SettingStore SettingGetter
}

// LlamaProvisioner manages hardware autodetection, thread capping, GGUF pre-flight verification,
// and safe container specification synthesis for the llama-server inference spoke.
type LlamaProvisioner struct {
	mu           sync.RWMutex
	dataDir      string
	modelsDir    string
	logger       *slog.Logger
	hwOpts       HardwareProbeOptions
	numCPU       func() int
	settingStore SettingGetter
}

// NewLlamaProvisioner constructs a new LlamaProvisioner.
func NewLlamaProvisioner(dataDir string, logger *slog.Logger, opts ...LlamaProvisionerOptions) *LlamaProvisioner {
	if dataDir == "" {
		dataDir = "/data"
	}
	if logger == nil {
		logger = slog.Default()
	}

	hwOpts := HardwareProbeOptions{
		DevRoot:   "/dev",
		SysfsRoot: "/sys",
		ProcRoot:  "/proc",
	}
	numCPUFn := runtime.NumCPU
	modelsDir := filepath.Join(dataDir, "models")

	var settingStore SettingGetter
	if len(opts) > 0 {
		o := opts[0]
		if o.DevRoot != "" {
			hwOpts.DevRoot = o.DevRoot
		}
		if o.SysfsRoot != "" {
			hwOpts.SysfsRoot = o.SysfsRoot
		}
		if o.ProcRoot != "" {
			hwOpts.ProcRoot = o.ProcRoot
		}
		if o.NumCPU != nil {
			numCPUFn = o.NumCPU
		}
		if o.ModelsDir != "" {
			modelsDir = o.ModelsDir
		}
		if o.SettingStore != nil {
			settingStore = o.SettingStore
		}
	}

	return &LlamaProvisioner{
		dataDir:      dataDir,
		modelsDir:    modelsDir,
		logger:       logger,
		hwOpts:       hwOpts,
		numCPU:       numCPUFn,
		settingStore: settingStore,
	}
}

// SetSettingStore sets or updates the settings store.
func (p *LlamaProvisioner) SetSettingStore(s SettingGetter) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.settingStore = s
}

// CalculateLlamaThreads caps CPU inference threads to max(1, physical_cores - 2)
// to prevent CPU inference from saturating cores and freezing household services.
func CalculateLlamaThreads(numCores int) int {
	if numCores <= 0 {
		numCores = runtime.NumCPU()
	}
	threads := numCores - 2
	if threads < 1 {
		threads = 1
	}
	return threads
}

// DetectLlamaHardware probes for NVIDIA CUDA (/dev/nvidia*) and Vulkan/DRI (/dev/dri/renderD128) devices.
// In hybrid GPU mode, pins Llama to dGPU (NVIDIA CUDA).
// On CPU-only hosts, sets CPUOptimized to true and returns isGPU = false.
func DetectLlamaHardware(opts HardwareProbeOptions) (HardwareDetectResult, bool) {
	if opts.DevRoot == "" {
		opts.DevRoot = "/dev"
	}
	if opts.SysfsRoot == "" {
		opts.SysfsRoot = "/sys"
	}
	if opts.ProcRoot == "" {
		opts.ProcRoot = "/proc"
	}

	// In hybrid mode, pin Llama to dGPU (NVIDIA CUDA)
	if IsHybridGPU(opts) {
		var nvidiaDevices []string
		candidates := []string{"nvidia0", "nvidiactl", "nvidia-uvm", "nvidia-modeset"}
		for _, c := range candidates {
			devPath := filepath.Join(opts.DevRoot, c)
			if _, err := os.Stat(devPath); err == nil {
				nvidiaDevices = append(nvidiaDevices, fmt.Sprintf("%s:%s", devPath, devPath))
			}
		}
		if len(nvidiaDevices) == 0 {
			nvidiaDevices = []string{"/dev/nvidia0:/dev/nvidia0"}
		}
		sort.Strings(nvidiaDevices)
		return HardwareDetectResult{
			Type:     HWAccelNVENC,
			Devices:  nvidiaDevices,
			IsHybrid: true,
		}, true
	}

	// 1. Probe NVIDIA CUDA
	nvidia0 := filepath.Join(opts.DevRoot, "nvidia0")
	nvidiactl := filepath.Join(opts.DevRoot, "nvidiactl")
	nvidiaProc := filepath.Join(opts.ProcRoot, "driver/nvidia/version")
	nvidiaProcDir := filepath.Join(opts.ProcRoot, "driver/nvidia")

	nvidiaFound := false
	if _, err := os.Stat(nvidia0); err == nil {
		nvidiaFound = true
	} else if _, err := os.Stat(nvidiactl); err == nil {
		nvidiaFound = true
	} else if _, err := os.Stat(nvidiaProc); err == nil {
		nvidiaFound = true
	} else if fi, err := os.Stat(nvidiaProcDir); err == nil && fi.IsDir() {
		nvidiaFound = true
	}

	if nvidiaFound {
		var nvidiaDevices []string
		candidates := []string{"nvidia0", "nvidiactl", "nvidia-uvm", "nvidia-modeset"}
		for _, c := range candidates {
			devPath := filepath.Join(opts.DevRoot, c)
			if _, err := os.Stat(devPath); err == nil {
				nvidiaDevices = append(nvidiaDevices, fmt.Sprintf("%s:%s", devPath, devPath))
			}
		}
		if len(nvidiaDevices) == 0 {
			nvidiaDevices = []string{"/dev/nvidia0:/dev/nvidia0"}
		}
		sort.Strings(nvidiaDevices)
		return HardwareDetectResult{
			Type:    HWAccelNVENC,
			Devices: nvidiaDevices,
		}, true
	}

	// 2. Probe Vulkan / DRI character device nodes
	renderPath := filepath.Join(opts.DevRoot, "dri", "renderD128")
	if _, err := os.Stat(renderPath); err == nil {
		driDevices := []string{fmt.Sprintf("%s:%s", renderPath, renderPath)}
		cardPath := filepath.Join(opts.DevRoot, "dri", "card0")
		if _, err := os.Stat(cardPath); err == nil {
			driDevices = append(driDevices, fmt.Sprintf("%s:%s", cardPath, cardPath))
		}
		sort.Strings(driDevices)
		return HardwareDetectResult{
			Type:        HWAccelVAAPI,
			Devices:     driDevices,
			VaapiDevice: renderPath,
		}, true
	}

	// 3. CPU fallback
	return HardwareDetectResult{
		Type:         HWAccelNone,
		Devices:      nil,
		CPUOptimized: true,
	}, false
}

// Provision inspects /data/models, verifies GGUF headers, probes hardware, and synthesizes ContainerSpec.
// If /data/models contains no valid GGUF model, returns InSafeStub: true without launching a container.
// If a model has a corrupted GGUF header, rejects it with ai.ErrInvalidGGUFMagic.
func (p *LlamaProvisioner) Provision(ctx context.Context) (*LlamaProvisionResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	p.mu.RLock()
	modelsDir := p.modelsDir
	hwOpts := p.hwOpts
	cpuFn := p.numCPU
	p.mu.RUnlock()

	// 1. Ensure models directory exists with 0755 permissions
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to ensure models directory %q: %w", modelsDir, err)
	}

	// 2. Probe hardware acceleration & calculate thread budget
	hw, isGPU := DetectLlamaHardware(hwOpts)
	threads := CalculateLlamaThreads(cpuFn())

	// 3. Scan models directory for candidate model files
	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read models directory %q: %w", modelsDir, err)
	}

	var candidateFiles []string
	var activeSymlink string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".downloading") {
			continue
		}
		if name == "active.gguf" || name == "default.gguf" {
			activeSymlink = filepath.Join(modelsDir, name)
			break
		}
		if strings.HasSuffix(strings.ToLower(name), ".gguf") {
			candidateFiles = append(candidateFiles, filepath.Join(modelsDir, name))
		}
	}

	var targetModel string
	if activeSymlink != "" {
		if _, err := ai.ValidateGGUFHeader(activeSymlink); err == nil {
			targetModel = activeSymlink
		}
	}

	if targetModel == "" {
		sort.Strings(candidateFiles)
		for _, cand := range candidateFiles {
			if _, err := ai.ValidateGGUFHeader(cand); err == nil {
				targetModel = cand
				break
			}
		}
	}

	if targetModel == "" {
		// No valid GGUF models provisioned in /data/models: enter Safe Stub Mode gracefully
		p.logger.Info("no valid GGUF models provisioned in /data/models; entering safe stub mode")
		return &LlamaProvisionResult{
			HWAccel:      hw,
			Devices:      hw.Devices,
			Threads:      threads,
			IsGPU:        isGPU,
			InSafeStub:   true,
			CPUOptimized: !isGPU,
		}, nil
	}

	// Resolve symlinks to avoid container mount path mismatch
	resolvedModel, err := filepath.EvalSymlinks(targetModel)
	if err != nil {
		resolvedModel = targetModel
	}
	modelFilename := filepath.Base(resolvedModel)
	containerModelPath := "/models/" + modelFilename

	// 5. Build standardized command line arguments
	// Always: -np 2 (parallel slots), -fa (FlashAttention)
	// Context length and GPU offload layers are dynamically read from settingStore (defaults: 4096, 99)
	ctxLength := "4096"
	gpuLayers := "99"
	p.mu.RLock()
	store := p.settingStore
	p.mu.RUnlock()
	if store != nil {
		if val, err := store.GetSetting(ctx, "ai_context_length"); err == nil && val != "" {
			ctxLength = val
		}
		if val, err := store.GetSetting(ctx, "ai_gpu_layers"); err == nil && val != "" {
			gpuLayers = val
		}
	}

	cmd := []string{
		"--host", "0.0.0.0",
		"--port", strconv.Itoa(DefaultLlamaPort),
		"-m", containerModelPath,
		"-c", ctxLength,
		"-np", "2",
		"-fa",
	}

	env := map[string]string{
		"LLAMA_ARG_HOST":       "0.0.0.0",
		"LLAMA_ARG_PORT":       strconv.Itoa(DefaultLlamaPort),
		"LLAMA_ARG_MODEL":      containerModelPath,
		"LLAMA_ARG_CTX_SIZE":   ctxLength,
		"LLAMA_ARG_PARALLEL":   "2",
		"LLAMA_ARG_FLASH_ATTN": "1",
	}

	if isGPU {
		cmd = append(cmd, "-ngl", gpuLayers)
		env["LLAMA_ARG_N_GPU_LAYERS"] = gpuLayers
		if hw.Type == HWAccelNVENC {
			env["NVIDIA_VISIBLE_DEVICES"] = "all"
			env["NVIDIA_DRIVER_CAPABILITIES"] = "compute,utility"
		}
	} else {
		cmd = append(cmd, "-t", strconv.Itoa(threads))
		env["LLAMA_ARG_THREADS"] = strconv.Itoa(threads)
	}

	// 6. Synthesize ContainerSpec
	// Boundaries & Constraints:
	// - Isolated inside sovereign-ai bridge network
	// - Ports: nil (zero published host ports)
	// - Volumes: /data/models:/models:ro
	// - No docker.sock mount
	containerSpec := ContainerSpec{
		Name:                   ServiceLlama,
		Image:                  ImageLlama,
		MemoryLimitBytes:       MemLimitLlama,
		MemoryReservationBytes: MemReservationLlama,
		Ports:                  nil, // Strict zero host port publishing
		Devices:                hw.Devices,
		Volumes:                []string{fmt.Sprintf("%s:/models:ro", modelsDir)},
		Networks:               []string{"sovereign-ai"},
		Env:                    env,
		Command:                cmd,
	}

	return &LlamaProvisionResult{
		ContainerSpec: containerSpec,
		HWAccel:       hw,
		Devices:       hw.Devices,
		Threads:       threads,
		IsGPU:         isGPU,
		ModelPath:     targetModel,
		InSafeStub:    false,
		CPUOptimized:  !isGPU,
	}, nil
}

package supervisor

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sovereign/internal/ai"
)

func createMockGGUFFile(t *testing.T, dir, filename string, version uint32) string {
	t.Helper()
	buf := new(bytes.Buffer)
	buf.Write([]byte{0x47, 0x47, 0x55, 0x46}) // "GGUF"
	_ = binary.Write(buf, binary.LittleEndian, version)
	_ = binary.Write(buf, binary.LittleEndian, uint64(100))
	_ = binary.Write(buf, binary.LittleEndian, uint64(10))

	fullPath := filepath.Join(dir, filename)
	if err := os.WriteFile(fullPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to create mock GGUF file: %v", err)
	}
	return fullPath
}

func TestCalculateLlamaThreads(t *testing.T) {
	tests := []struct {
		name     string
		numCores int
		expected int
	}{
		{"1 core", 1, 1},
		{"2 cores", 2, 1},
		{"3 cores", 3, 1},
		{"4 cores", 4, 2},
		{"6 cores", 6, 4},
		{"8 cores", 8, 6},
		{"16 cores", 16, 14},
		{"32 cores", 32, 30},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateLlamaThreads(tc.numCores)
			if got != tc.expected {
				t.Errorf("CalculateLlamaThreads(%d) = %d; expected %d", tc.numCores, got, tc.expected)
			}
		})
	}
}

func TestDetectLlamaHardware_CPUFallback(t *testing.T) {
	tmpDir := t.TempDir()
	opts := HardwareProbeOptions{
		DevRoot:   filepath.Join(tmpDir, "dev"),
		SysfsRoot: filepath.Join(tmpDir, "sys"),
		ProcRoot:  filepath.Join(tmpDir, "proc"),
	}

	hw, isGPU := DetectLlamaHardware(opts)
	if isGPU {
		t.Fatal("expected isGPU to be false on clean directory")
	}
	if hw.Type != HWAccelNone {
		t.Errorf("expected HWAccelNone, got %s", hw.Type)
	}
	if len(hw.Devices) != 0 {
		t.Errorf("expected 0 devices on CPU fallback, got %v", hw.Devices)
	}
	if !hw.CPUOptimized {
		t.Errorf("expected CPUOptimized to be true on CPU fallback")
	}
}

func TestDetectLlamaHardware_HybridGPUDetection(t *testing.T) {
	tmpDir := t.TempDir()
	devDir := filepath.Join(tmpDir, "dev")
	driDir := filepath.Join(devDir, "dri")
	_ = os.MkdirAll(driDir, 0755)
	_ = os.WriteFile(filepath.Join(devDir, "nvidia0"), []byte{}, 0644)
	_ = os.WriteFile(filepath.Join(devDir, "nvidiactl"), []byte{}, 0644)
	_ = os.WriteFile(filepath.Join(driDir, "renderD128"), []byte{}, 0644)

	opts := HardwareProbeOptions{
		DevRoot:   devDir,
		SysfsRoot: filepath.Join(tmpDir, "sys"),
		ProcRoot:  filepath.Join(tmpDir, "proc"),
	}

	if !IsHybridGPU(opts) {
		t.Fatalf("expected IsHybridGPU to be true")
	}

	hw, isGPU := DetectLlamaHardware(opts)
	if !isGPU {
		t.Fatal("expected isGPU to be true for hybrid setup")
	}
	if !hw.IsHybrid {
		t.Errorf("expected IsHybrid to be true on HardwareDetectResult")
	}
	// In hybrid mode, Llama must be pinned to the dGPU (NVIDIA CUDA), not iGPU
	if hw.Type != HWAccelNVENC {
		t.Errorf("expected HWAccelNVENC for dGPU pinning in hybrid mode, got %s", hw.Type)
	}
}

func TestDetectLlamaHardware_CUDADetection(t *testing.T) {
	tmpDir := t.TempDir()
	devDir := filepath.Join(tmpDir, "dev")
	_ = os.MkdirAll(devDir, 0755)
	_ = os.WriteFile(filepath.Join(devDir, "nvidia0"), []byte{}, 0644)
	_ = os.WriteFile(filepath.Join(devDir, "nvidiactl"), []byte{}, 0644)

	opts := HardwareProbeOptions{
		DevRoot:   devDir,
		SysfsRoot: filepath.Join(tmpDir, "sys"),
		ProcRoot:  filepath.Join(tmpDir, "proc"),
	}

	hw, isGPU := DetectLlamaHardware(opts)
	if !isGPU {
		t.Fatal("expected isGPU to be true when nvidia devices present")
	}
	if hw.Type != HWAccelNVENC {
		t.Errorf("expected HWAccelNVENC, got %s", hw.Type)
	}
	if len(hw.Devices) == 0 {
		t.Fatal("expected non-empty devices for nvidia")
	}
}

func TestDetectLlamaHardware_VulkanDRIDetection(t *testing.T) {
	tmpDir := t.TempDir()
	driDir := filepath.Join(tmpDir, "dev", "dri")
	_ = os.MkdirAll(driDir, 0755)
	_ = os.WriteFile(filepath.Join(driDir, "renderD128"), []byte{}, 0644)

	opts := HardwareProbeOptions{
		DevRoot:   filepath.Join(tmpDir, "dev"),
		SysfsRoot: filepath.Join(tmpDir, "sys"),
		ProcRoot:  filepath.Join(tmpDir, "proc"),
	}

	hw, isGPU := DetectLlamaHardware(opts)
	if !isGPU {
		t.Fatal("expected isGPU to be true when renderD128 present")
	}
	if hw.Type != HWAccelVAAPI {
		t.Errorf("expected HWAccelVAAPI, got %s", hw.Type)
	}
	if len(hw.Devices) == 0 {
		t.Fatal("expected non-empty devices for DRI")
	}
}

func TestLlamaProvisioner_SafeStubWhenEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	prov := NewLlamaProvisioner(tmpDir, nil, LlamaProvisionerOptions{
		NumCPU: func() int { return 4 },
	})

	res, err := prov.Provision(context.Background())
	if err != nil {
		t.Fatalf("expected nil error on empty models dir, got: %v", err)
	}
	if !res.InSafeStub {
		t.Fatal("expected InSafeStub to be true when no models provisioned")
	}
}

func TestLlamaProvisioner_RejectCorruptGGUF(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "models")
	_ = os.MkdirAll(modelsDir, 0755)

	corruptPath := filepath.Join(modelsDir, "bad.gguf")
	_ = os.WriteFile(corruptPath, []byte("NOT_A_VALID_GGUF_HEADER"), 0644)

	// ValidateGGUFHeader itself directly rejects corrupt headers with ErrInvalidGGUFMagic
	_, err := ai.ValidateGGUFHeader(corruptPath)
	if err == nil || !errors.Is(err, ai.ErrInvalidGGUFMagic) {
		t.Fatalf("expected ValidateGGUFHeader to return ErrInvalidGGUFMagic, got: %v", err)
	}

	prov := NewLlamaProvisioner(tmpDir, nil, LlamaProvisionerOptions{
		NumCPU: func() int { return 4 },
	})

	// When only corrupt models are present, Provision gracefully falls back to Safe Stub Mode without failing
	res, err := prov.Provision(context.Background())
	if err != nil {
		t.Fatalf("Provision must not return an error when corrupt models exist; it must enter safe stub: %v", err)
	}
	if !res.InSafeStub {
		t.Fatal("expected InSafeStub=true on corrupt model fallback")
	}
}

func TestLlamaProvisioner_SuccessCPU(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "models")
	_ = os.MkdirAll(modelsDir, 0755)

	createMockGGUFFile(t, modelsDir, "qwen2.5-7b.gguf", 3)

	prov := NewLlamaProvisioner(tmpDir, nil, LlamaProvisionerOptions{
		NumCPU:  func() int { return 8 }, // 8 cores -> capped at 6 threads
		DevRoot: filepath.Join(tmpDir, "dev"),
	})

	res, err := prov.Provision(context.Background())
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	if res.InSafeStub {
		t.Fatal("expected InSafeStub to be false when valid model exists")
	}
	if res.IsGPU {
		t.Fatal("expected isGPU to be false on CPU host")
	}
	if res.Threads != 6 {
		t.Errorf("expected 6 threads on 8 cores, got %d", res.Threads)
	}

	spec := res.ContainerSpec
	// Invariant 1: Air-gapped network isolation
	if len(spec.Networks) != 1 || spec.Networks[0] != "sovereign-ai" {
		t.Errorf("expected network sovereign-ai, got %v", spec.Networks)
	}
	// Invariant 2: Zero host ports exposed
	if len(spec.Ports) != 0 {
		t.Errorf("expected nil or empty Ports, got %v", spec.Ports)
	}
	// Invariant 3: Official image
	if spec.Image != ImageLlama {
		t.Errorf("expected image %s, got %s", ImageLlama, spec.Image)
	}
	// Invariant 4: Volume mount
	expectedVol := fmt.Sprintf("%s:/models:ro", modelsDir)
	if len(spec.Volumes) != 1 || spec.Volumes[0] != expectedVol {
		t.Errorf("expected volume %s, got %v", expectedVol, spec.Volumes)
	}

	// Invariant 5: Standard execution parameters (-np 2, -fa, -c 4096, -t 6)
	cmdStr := " " + strings.Join(spec.Command, " ") + " "
	if !strings.Contains(cmdStr, " -np 2 ") {
		t.Errorf("expected -np 2 in command: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, " -fa ") {
		t.Errorf("expected -fa in command: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, " -c 4096 ") {
		t.Errorf("expected -c 4096 in command: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, " -t 6 ") {
		t.Errorf("expected -t 6 in command: %s", cmdStr)
	}
	if strings.Contains(cmdStr, " -ngl ") {
		t.Errorf("should not contain -ngl on CPU host: %s", cmdStr)
	}

	if spec.Env["LLAMA_ARG_THREADS"] != "6" {
		t.Errorf("expected LLAMA_ARG_THREADS=6, got %s", spec.Env["LLAMA_ARG_THREADS"])
	}
}

func TestLlamaProvisioner_SuccessGPU(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "models")
	_ = os.MkdirAll(modelsDir, 0755)
	createMockGGUFFile(t, modelsDir, "model.gguf", 3)

	devDir := filepath.Join(tmpDir, "dev")
	_ = os.MkdirAll(devDir, 0755)
	_ = os.WriteFile(filepath.Join(devDir, "nvidia0"), []byte{}, 0644)

	prov := NewLlamaProvisioner(tmpDir, nil, LlamaProvisionerOptions{
		NumCPU:  func() int { return 8 },
		DevRoot: devDir,
	})

	res, err := prov.Provision(context.Background())
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	if !res.IsGPU {
		t.Fatal("expected isGPU to be true on GPU host")
	}

	cmdStr := " " + strings.Join(res.ContainerSpec.Command, " ") + " "
	if !strings.Contains(cmdStr, " -ngl 99 ") {
		t.Errorf("expected -ngl 99 in command for GPU host: %s", cmdStr)
	}
	if res.ContainerSpec.Env["LLAMA_ARG_N_GPU_LAYERS"] != "99" {
		t.Errorf("expected LLAMA_ARG_N_GPU_LAYERS=99, got %s", res.ContainerSpec.Env["LLAMA_ARG_N_GPU_LAYERS"])
	}
	if len(res.ContainerSpec.Devices) == 0 {
		t.Fatal("expected devices mounted into container spec for GPU host")
	}
	if res.ContainerSpec.Env["NVIDIA_VISIBLE_DEVICES"] != "all" {
		t.Errorf("expected NVIDIA_VISIBLE_DEVICES=all, got %s", res.ContainerSpec.Env["NVIDIA_VISIBLE_DEVICES"])
	}
	if res.ContainerSpec.Env["NVIDIA_DRIVER_CAPABILITIES"] != "compute,utility" {
		t.Errorf("expected NVIDIA_DRIVER_CAPABILITIES=compute,utility, got %s", res.ContainerSpec.Env["NVIDIA_DRIVER_CAPABILITIES"])
	}
}

func TestLlamaProvisioner_CorruptCandidateHandling(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "models")
	_ = os.MkdirAll(modelsDir, 0755)

	// a_bad.gguf is alphabetically first but corrupt
	_ = os.WriteFile(filepath.Join(modelsDir, "a_bad.gguf"), []byte("not-a-gguf-file"), 0644)
	// b_good.gguf is valid
	createMockGGUFFile(t, modelsDir, "b_good.gguf", 3)

	prov := NewLlamaProvisioner(tmpDir, nil)
	res, err := prov.Provision(context.Background())
	if err != nil {
		t.Fatalf("Provision should not fail when valid candidate exists: %v", err)
	}
	if res.InSafeStub {
		t.Fatal("expected InSafeStub to be false when a valid candidate exists")
	}
	if !strings.HasSuffix(res.ModelPath, "b_good.gguf") {
		t.Errorf("expected selected model to be b_good.gguf, got %s", res.ModelPath)
	}
}

func TestLlamaProvisioner_AllCorrupt_EntersSafeStub(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "models")
	_ = os.MkdirAll(modelsDir, 0755)

	_ = os.WriteFile(filepath.Join(modelsDir, "corrupt1.gguf"), []byte("garbage"), 0644)
	_ = os.WriteFile(filepath.Join(modelsDir, "corrupt2.gguf"), []byte("corrupt"), 0644)

	prov := NewLlamaProvisioner(tmpDir, nil)
	res, err := prov.Provision(context.Background())
	if err != nil {
		t.Fatalf("Provision should not fail when models are corrupt; it must enter safe stub: %v", err)
	}
	if !res.InSafeStub {
		t.Fatal("expected InSafeStub=true when all candidate models are corrupt")
	}
}

func TestLlamaProvisioner_SymlinkResolution(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "models")
	_ = os.MkdirAll(modelsDir, 0755)

	realModel := createMockGGUFFile(t, modelsDir, "qwen2.5-7b.gguf", 3)
	symlinkPath := filepath.Join(modelsDir, "active.gguf")
	if err := os.Symlink(realModel, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	prov := NewLlamaProvisioner(tmpDir, nil)
	res, err := prov.Provision(context.Background())
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}
	if res.InSafeStub {
		t.Fatal("expected InSafeStub=false")
	}

	// The container path should resolve to /models/qwen2.5-7b.gguf
	expectedContainerPath := "/models/qwen2.5-7b.gguf"
	cmdStr := strings.Join(res.ContainerSpec.Command, " ")
	if !strings.Contains(cmdStr, expectedContainerPath) {
		t.Errorf("expected container command to reference %s, got: %s", expectedContainerPath, cmdStr)
	}
}

func TestLlamaProvisioner_SynthesizeComposeSpec(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "models")
	_ = os.MkdirAll(modelsDir, 0755)
	createMockGGUFFile(t, modelsDir, "test.gguf", 3)

	prov := NewLlamaProvisioner(tmpDir, nil)
	res, err := prov.Provision(context.Background())
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	composePath, err := SynthesizeComposeSpec(tmpDir, res.ContainerSpec)
	if err != nil {
		t.Fatalf("SynthesizeComposeSpec failed: %v", err)
	}

	content, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("failed to read synthesized compose spec: %v", err)
	}

	specStr := string(content)
	if !strings.Contains(specStr, "sovereign-ai") {
		t.Errorf("expected compose spec to include sovereign-ai network: %s", specStr)
	}
	if !strings.Contains(specStr, "external: true") {
		t.Errorf("expected external: true for sovereign-ai network: %s", specStr)
	}
	if !strings.Contains(specStr, "/models/test.gguf") {
		t.Errorf("expected container model path in command: %s", specStr)
	}
	if strings.Contains(specStr, "ports:") && strings.Contains(specStr, "8080:8080") {
		t.Errorf("llama container must not publish host ports: %s", specStr)
	}
}

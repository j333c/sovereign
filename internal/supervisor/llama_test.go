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

	prov := NewLlamaProvisioner(tmpDir, nil, LlamaProvisionerOptions{
		NumCPU: func() int { return 4 },
	})

	_, err := prov.Provision(context.Background())
	if err == nil {
		t.Fatal("expected error on corrupt GGUF file, got nil")
	}
	if !errors.Is(err, ai.ErrInvalidGGUFMagic) {
		t.Errorf("expected errors.Is(err, ai.ErrInvalidGGUFMagic), got: %v", err)
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
}

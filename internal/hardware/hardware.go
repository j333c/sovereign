// Package hardware collects appliance telemetry and hardware metrics across CPU, memory,
// storage SMART diagnostics, thermals, and GPU resources.
//
// Invariants & Boundaries:
// - Hardware Sensors: Intel/AMD DRM sysfs, NVML for NVIDIA GPUs, thermal sysfs, and SMART disk telemetry.
// - GPU Idle Surrender: Monitors GPU activity and orchestrates VRAM surrender after 300 seconds of idle.
// - Air-Gapped Operation: Telemetry is computed strictly on-device without external telemetry phoning.
package hardware

import (
	"context"
)

// TelemetrySnapshot represents instantaneous hardware performance metrics.
type TelemetrySnapshot struct {
	CPUUsagePercent    float64 `json:"cpu_usage_percent"`
	MemoryUsedBytes    uint64  `json:"memory_used_bytes"`
	MemoryTotalBytes   uint64  `json:"memory_total_bytes"`
	StorageUsedBytes   uint64  `json:"storage_used_bytes"`
	StorageTotalBytes  uint64  `json:"storage_total_bytes"`
	TemperatureCelsius float64 `json:"temperature_celsius"`
	GPUUsagePercent    float64 `json:"gpu_usage_percent,omitempty"`
	GPUMemoryUsedBytes uint64  `json:"gpu_memory_used_bytes,omitempty"`
	GPUMemoryTotalBytes uint64 `json:"gpu_memory_total_bytes,omitempty"`
}

// Collector provides telemetry reads for the real-time SSE stream.
type Collector interface {
	CollectSnapshot(ctx context.Context) (*TelemetrySnapshot, error)
}

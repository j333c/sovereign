package hardware

import (
	"context"
	"testing"
)

func TestSystemCollector_CollectSnapshot(t *testing.T) {
	collector := NewSystemCollector()
	snapshot, err := collector.CollectSnapshot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error collecting snapshot: %v", err)
	}

	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}

	if snapshot.CPUUsagePercent < 0 || snapshot.CPUUsagePercent > 100 {
		t.Errorf("expected CPU usage between 0 and 100, got %f", snapshot.CPUUsagePercent)
	}

	if snapshot.MemoryTotalBytes == 0 {
		t.Error("expected non-zero memory total bytes")
	}

	if snapshot.TemperatureCelsius <= 0 {
		t.Errorf("expected positive temperature, got %f", snapshot.TemperatureCelsius)
	}
}

func TestSystemCollector_DeltaCPU(t *testing.T) {
	collector := NewSystemCollector()
	snap1, err := collector.CollectSnapshot(context.Background())
	if err != nil {
		t.Fatalf("first snapshot failed: %v", err)
	}
	if snap1 == nil {
		t.Fatal("expected non-nil first snapshot")
	}

	snap2, err := collector.CollectSnapshot(context.Background())
	if err != nil {
		t.Fatalf("second snapshot failed: %v", err)
	}
	if snap2 == nil {
		t.Fatal("expected non-nil second snapshot")
	}
	if snap2.CPUUsagePercent < 0 || snap2.CPUUsagePercent > 100 {
		t.Errorf("expected CPU percentage between 0 and 100, got %f", snap2.CPUUsagePercent)
	}
}

func TestMockCollector(t *testing.T) {
	mock := &MockCollector{
		Snapshot: &TelemetrySnapshot{
			CPUUsagePercent:   25.5,
			MemoryUsedBytes:   2000,
			MemoryTotalBytes:  4000,
			StorageUsedBytes:  500,
			StorageTotalBytes: 1000,
		},
	}

	snapshot, err := mock.CollectSnapshot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snapshot.CPUUsagePercent != 25.5 {
		t.Errorf("expected 25.5, got %f", snapshot.CPUUsagePercent)
	}
}

package hardware

import "context"

// MockCollector allows injecting static snapshots for verification tests.
type MockCollector struct {
	Snapshot *TelemetrySnapshot
	Err      error
}

// CollectSnapshot returns the configured mock snapshot.
func (m *MockCollector) CollectSnapshot(ctx context.Context) (*TelemetrySnapshot, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Snapshot != nil {
		return m.Snapshot, nil
	}
	return &TelemetrySnapshot{
		CPUUsagePercent:    12.4,
		MemoryUsedBytes:    3221225472, // 3 GB
		MemoryTotalBytes:   8589934592, // 8 GB
		TemperatureCelsius: 38.5,
	}, nil
}

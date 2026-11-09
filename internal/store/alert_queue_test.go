package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestAlertQueue_CRUDAndStatusTransitions(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "alert_queue_test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	qStore, err := NewAlertQueueStore(db)
	if err != nil {
		t.Fatalf("NewAlertQueueStore failed: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()

	item := &AlertQueueItem{
		ID:            "test-alert-1",
		Channel:       "webhook",
		Recipient:     "http://homeassistant.local:8123/api/webhook/sovereign",
		Payload:       `{"event":"document_deadline","title":"Tax Return"}`,
		Status:        AlertQueueStatusPending,
		RetryCount:    0,
		MaxRetries:    5,
		NextAttemptAt: now.Add(-1 * time.Minute), // Ready now
		LastError:     "connection refused",
	}

	// 1. Enqueue
	if err := qStore.Enqueue(ctx, item); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	// 2. Get by ID
	fetched, err := qStore.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if fetched.ID != item.ID {
		t.Errorf("expected ID %q, got %q", item.ID, fetched.ID)
	}
	if fetched.Status != AlertQueueStatusPending {
		t.Errorf("expected status 'pending', got %q", fetched.Status)
	}
	if fetched.MaxRetries != 5 {
		t.Errorf("expected max_retries 5, got %d", fetched.MaxRetries)
	}

	// 3. GetPending
	pending, err := qStore.GetPending(ctx, now)
	if err != nil {
		t.Fatalf("GetPending failed: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending item, got %d", len(pending))
	}
	if pending[0].ID != item.ID {
		t.Errorf("expected pending item ID %q, got %q", item.ID, pending[0].ID)
	}

	// Future items should not be returned by GetPending
	futurePending, err := qStore.GetPending(ctx, now.Add(-2*time.Minute))
	if err != nil {
		t.Fatalf("GetPending with past time failed: %v", err)
	}
	if len(futurePending) != 0 {
		t.Errorf("expected 0 pending items for past time, got %d", len(futurePending))
	}

	// 4. MarkDelivered
	if err := qStore.MarkDelivered(ctx, item.ID); err != nil {
		t.Fatalf("MarkDelivered failed: %v", err)
	}

	delivered, err := qStore.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("Get after delivery failed: %v", err)
	}
	if delivered.Status != AlertQueueStatusDelivered {
		t.Errorf("expected status 'delivered', got %q", delivered.Status)
	}

	// Delivered items should not appear in GetPending
	pendingAfterDelivery, err := qStore.GetPending(ctx, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("GetPending after delivery failed: %v", err)
	}
	if len(pendingAfterDelivery) != 0 {
		t.Errorf("expected 0 pending items after delivery, got %d", len(pendingAfterDelivery))
	}
}

func TestAlertQueue_RetryAndMaxRetries(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "alert_retry_test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	qStore, err := NewAlertQueueStore(db)
	if err != nil {
		t.Fatalf("NewAlertQueueStore failed: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()

	item := &AlertQueueItem{
		ID:         "test-retry-alert",
		Channel:    "email",
		Recipient:  "admin@sovereign.local",
		Payload:    `{"type":"digest"}`,
		MaxRetries: 3, // Lower max_retries for quick test
	}
	if err := qStore.Enqueue(ctx, item); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	// Failure attempt 1
	nextAttempt1 := now.Add(10 * time.Second)
	if err := qStore.RecordFailure(ctx, item.ID, "smtp dial timeout", nextAttempt1); err != nil {
		t.Fatalf("RecordFailure attempt 1 failed: %v", err)
	}
	item1, _ := qStore.Get(ctx, item.ID)
	if item1.RetryCount != 1 {
		t.Errorf("expected retry_count 1, got %d", item1.RetryCount)
	}
	if item1.Status != AlertQueueStatusPending {
		t.Errorf("expected status 'pending' on retry 1, got %q", item1.Status)
	}
	if item1.LastError != "smtp dial timeout" {
		t.Errorf("expected last error updated, got %q", item1.LastError)
	}

	// Failure attempt 2
	nextAttempt2 := now.Add(20 * time.Second)
	if err := qStore.RecordFailure(ctx, item.ID, "smtp dial timeout 2", nextAttempt2); err != nil {
		t.Fatalf("RecordFailure attempt 2 failed: %v", err)
	}
	item2, _ := qStore.Get(ctx, item.ID)
	if item2.RetryCount != 2 {
		t.Errorf("expected retry_count 2, got %d", item2.RetryCount)
	}
	if item2.Status != AlertQueueStatusPending {
		t.Errorf("expected status 'pending' on retry 2, got %q", item2.Status)
	}

	// Failure attempt 3 (reaches max_retries = 3)
	if err := qStore.RecordFailure(ctx, item.ID, "smtp unreachable", time.Time{}); err != nil {
		t.Fatalf("RecordFailure attempt 3 failed: %v", err)
	}
	item3, _ := qStore.Get(ctx, item.ID)
	if item3.RetryCount != 3 {
		t.Errorf("expected retry_count 3, got %d", item3.RetryCount)
	}
	if item3.Status != AlertQueueStatusFailed {
		t.Errorf("expected status 'failed' after reaching max_retries, got %q", item3.Status)
	}

	// Failed items must not appear in GetPending
	pending, err := qStore.GetPending(ctx, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("GetPending failed: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 pending items for failed alert, got %d", len(pending))
	}

	// List by status 'failed'
	failedList, err := qStore.List(ctx, AlertQueueStatusFailed)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(failedList) != 1 || failedList[0].ID != item.ID {
		t.Errorf("expected 1 failed item in list, got %+v", failedList)
	}
}

func TestAlertQueue_CalculateBackoff(t *testing.T) {
	tests := []struct {
		retries  int
		expected time.Duration
	}{
		{0, 10 * time.Second},
		{1, 20 * time.Second},
		{2, 40 * time.Second},
		{3, 80 * time.Second},
		{4, 160 * time.Second},
		{5, 320 * time.Second},
		{10, 10240 * time.Second}, // Capped at 1 hour
	}

	for _, tt := range tests {
		actual := CalculateBackoff(tt.retries)
		expected := tt.expected
		if expected > time.Hour {
			expected = time.Hour
		}
		if actual != expected {
			t.Errorf("CalculateBackoff(%d) = %v; expected %v", tt.retries, actual, expected)
		}
	}
}

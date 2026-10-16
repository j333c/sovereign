package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestDeadlineStore_CRUDAndDeduplication(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "deadlines_test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	store, err := NewDeadlineStore(db)
	if err != nil {
		t.Fatalf("NewDeadlineStore failed: %v", err)
	}

	ctx := context.Background()
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	amt := 150.00
	d1 := &DocumentDeadline{
		DocumentID: "doc-1",
		Title:      "Invoice #482",
		DueDate:    "2026-10-28",
		Amount:     &amt,
		Currency:   "USD",
		Snippet:    "Invoice #482. Total Due: $150.00 by 2026-10-28",
		Status:     DeadlineStatusPending,
	}

	if err := store.Save(ctx, d1); err != nil {
		t.Fatalf("Save d1 failed: %v", err)
	}
	if d1.ID == "" {
		t.Errorf("expected generated ID, got empty")
	}

	firstID := d1.ID

	// Rescan same document and due_date with updated amount
	newAmt := 175.50
	d1Rescan := &DocumentDeadline{
		DocumentID: "doc-1",
		Title:      "Invoice #482 Updated",
		DueDate:    "2026-10-28",
		Amount:     &newAmt,
		Currency:   "USD",
		Snippet:    "Invoice #482. Total Due: $175.50 by 2026-10-28",
		Status:     DeadlineStatusPending,
	}

	if err := store.Save(ctx, d1Rescan); err != nil {
		t.Fatalf("Save d1Rescan failed: %v", err)
	}

	// Deduplication check: ID must be identical!
	if d1Rescan.ID != firstID {
		t.Errorf("expected deduplicated ID %q, got %q", firstID, d1Rescan.ID)
	}

	// Verify updated fields in DB
	fetched, err := store.Get(ctx, firstID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if fetched.Title != "Invoice #482 Updated" {
		t.Errorf("expected title 'Invoice #482 Updated', got %q", fetched.Title)
	}
	if fetched.Amount == nil || *fetched.Amount != 175.50 {
		t.Errorf("expected amount 175.50, got %v", fetched.Amount)
	}

	// Add second deadline (future, beyond 14 days)
	d2 := &DocumentDeadline{
		DocumentID: "doc-2",
		Title:      "Contract Renewal",
		DueDate:    "2026-11-30",
		Status:     DeadlineStatusFuture,
	}
	if err := store.Save(ctx, d2); err != nil {
		t.Fatalf("Save d2 failed: %v", err)
	}

	// List pending within 14 days (now is 2026-10-16 -> max date is 2026-10-30)
	// d1 is 2026-10-28 (within 12 days, pending) -> should be included
	// d2 is 2026-11-30 (future) -> excluded
	pendingList, err := store.List(ctx, DeadlineStatusPending, 14, now)
	if err != nil {
		t.Fatalf("List pending failed: %v", err)
	}
	if len(pendingList) != 1 {
		t.Fatalf("expected 1 pending deadline within 14 days, got %d", len(pendingList))
	}
	if pendingList[0].ID != firstID {
		t.Errorf("expected ID %q, got %q", firstID, pendingList[0].ID)
	}

	// Dismiss d1
	dismissed, err := store.Dismiss(ctx, firstID)
	if err != nil {
		t.Fatalf("Dismiss failed: %v", err)
	}
	if dismissed.Status != DeadlineStatusDismissed {
		t.Errorf("expected status 'dismissed', got %q", dismissed.Status)
	}

	// Verify excluded from pending list
	pendingAfterDismiss, err := store.List(ctx, DeadlineStatusPending, 14, now)
	if err != nil {
		t.Fatalf("List pending after dismiss failed: %v", err)
	}
	if len(pendingAfterDismiss) != 0 {
		t.Errorf("expected 0 pending deadlines after dismiss, got %d", len(pendingAfterDismiss))
	}

	// Rescanning a dismissed document should preserve dismissed status
	d1Rescan2 := &DocumentDeadline{
		DocumentID: "doc-1",
		Title:      "Invoice #482",
		DueDate:    "2026-10-28",
		Status:     DeadlineStatusPending,
	}
	if err := store.Save(ctx, d1Rescan2); err != nil {
		t.Fatalf("Save d1Rescan2 failed: %v", err)
	}
	if d1Rescan2.Status != DeadlineStatusDismissed {
		t.Errorf("expected preserved dismissed status, got %q", d1Rescan2.Status)
	}

	// Dismiss non-existent ID -> ErrDeadlineNotFound
	_, err = store.Dismiss(ctx, "non-existent-id")
	if err != ErrDeadlineNotFound {
		t.Errorf("expected ErrDeadlineNotFound, got %v", err)
	}

	// Add d3 (pending) and mark it notified
	d3 := &DocumentDeadline{
		DocumentID: "doc-3",
		Title:      "Utility Bill",
		DueDate:    "2026-10-25",
		Status:     DeadlineStatusPending,
	}
	if err := store.Save(ctx, d3); err != nil {
		t.Fatalf("Save d3 failed: %v", err)
	}

	if err := store.MarkNotified(ctx, d3.ID); err != nil {
		t.Fatalf("MarkNotified failed: %v", err)
	}

	fetchedD3, err := store.Get(ctx, d3.ID)
	if err != nil {
		t.Fatalf("Get d3 failed: %v", err)
	}
	if fetchedD3.Status != DeadlineStatusNotified {
		t.Errorf("expected status 'notified', got %q", fetchedD3.Status)
	}

	// Rescanning d3 with status pending should preserve notified status
	d3Rescan := &DocumentDeadline{
		DocumentID: "doc-3",
		Title:      "Utility Bill Rescanned",
		DueDate:    "2026-10-25",
		Status:     DeadlineStatusPending,
	}
	if err := store.Save(ctx, d3Rescan); err != nil {
		t.Fatalf("Save d3Rescan failed: %v", err)
	}
	if d3Rescan.Status != DeadlineStatusNotified {
		t.Errorf("expected preserved notified status, got %q", d3Rescan.Status)
	}

	// MarkNotified on non-existent ID -> ErrDeadlineNotFound
	if err := store.MarkNotified(ctx, "non-existent-id"); err != ErrDeadlineNotFound {
		t.Errorf("expected ErrDeadlineNotFound on non-existent ID, got %v", err)
	}
}

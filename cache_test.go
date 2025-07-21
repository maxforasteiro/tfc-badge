package main

import (
	"strings"
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	cache := NewCache()
	if cache == nil {
		t.Fatal("NewCache returned nil")
	}
	if cache.store == nil {
		t.Fatal("cache.store is nil")
	}
	if len(cache.store) != 0 {
		t.Errorf("expected empty cache, got %d entries", len(cache.store))
	}
}

func TestRunCache_SetAndGet(t *testing.T) {
	cache := NewCache()

	run := &Run{
		ID:            "run-123",
		WorkspaceID:   "ws-abc",
		WorkspaceName: "test-workspace",
		Message:       "Test run",
		CreatedAt:     time.Now(),
		Notifications: []Notification{
			{
				Message:      "Run Created",
				Trigger:      "run:created",
				RunStatus:    "pending",
				RunUpdatedAt: time.Now(),
			},
		},
	}

	// Test Set
	cache.Set("ws-abc", run)

	// Test Get - existing key
	retrieved, err := cache.Get("ws-abc")
	if err != nil {
		t.Fatalf("unexpected error getting run: %v", err)
	}
	if retrieved.ID != run.ID {
		t.Errorf("expected run ID %s, got %s", run.ID, retrieved.ID)
	}
	if retrieved.WorkspaceID != run.WorkspaceID {
		t.Errorf("expected workspace ID %s, got %s", run.WorkspaceID, retrieved.WorkspaceID)
	}

	// Test Get - non-existing key
	_, err = cache.Get("non-existing")
	if err != NotFoundError {
		t.Errorf("expected NotFoundError, got %v", err)
	}
}

func TestRunCache_List(t *testing.T) {
	cache := NewCache()

	// Test empty cache
	runs := cache.List()
	if len(runs) != 0 {
		t.Errorf("expected empty list, got %d entries", len(runs))
	}

	// Add some runs
	run1 := &Run{ID: "run-1", WorkspaceID: "ws-1"}
	run2 := &Run{ID: "run-2", WorkspaceID: "ws-2"}

	cache.Set("ws-1", run1)
	cache.Set("ws-2", run2)

	runs = cache.List()
	if len(runs) != 2 {
		t.Errorf("expected 2 runs, got %d", len(runs))
	}

	// Check that all runs are present
	foundRun1, foundRun2 := false, false
	for _, run := range runs {
		if run.ID == "run-1" {
			foundRun1 = true
		}
		if run.ID == "run-2" {
			foundRun2 = true
		}
	}
	if !foundRun1 || !foundRun2 {
		t.Error("not all runs found in list")
	}
}

func TestRunCache_DumpAndRestore(t *testing.T) {
	cache := NewCache()

	// Create test data
	run1 := &Run{
		ID:            "run-1",
		WorkspaceID:   "ws-1",
		WorkspaceName: "workspace-1",
		Message:       "Test run 1",
		CreatedAt:     time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		Notifications: []Notification{
			{
				Message:      "Run Created",
				Trigger:      "run:created",
				RunStatus:    "pending",
				RunUpdatedAt: time.Date(2023, 1, 1, 12, 1, 0, 0, time.UTC),
			},
		},
	}

	run2 := &Run{
		ID:            "run-2",
		WorkspaceID:   "ws-2",
		WorkspaceName: "workspace-2",
		Message:       "Test run 2",
		CreatedAt:     time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC),
		Notifications: []Notification{
			{
				Message:      "Run Completed",
				Trigger:      "run:completed",
				RunStatus:    "applied",
				RunUpdatedAt: time.Date(2023, 1, 2, 12, 5, 0, 0, time.UTC),
			},
		},
	}

	cache.Set("ws-1", run1)
	cache.Set("ws-2", run2)

	// Test Dump
	var buf strings.Builder
	err := cache.Dump(&buf)
	if err != nil {
		t.Fatalf("unexpected error during dump: %v", err)
	}

	dumped := buf.String()
	if dumped == "" {
		t.Fatal("dump produced empty output")
	}

	// Test Restore
	newCache := NewCache()
	reader := strings.NewReader(dumped)
	err = newCache.Restore(reader)
	if err != nil {
		t.Fatalf("unexpected error during restore: %v", err)
	}

	// Verify restored data
	restoredRuns := newCache.List()
	if len(restoredRuns) != 2 {
		t.Errorf("expected 2 restored runs, got %d", len(restoredRuns))
	}

	// Check specific run
	restored1, err := newCache.Get("ws-1")
	if err != nil {
		t.Fatalf("error getting restored run: %v", err)
	}
	if restored1.ID != run1.ID {
		t.Errorf("expected run ID %s, got %s", run1.ID, restored1.ID)
	}
	if restored1.WorkspaceName != run1.WorkspaceName {
		t.Errorf("expected workspace name %s, got %s", run1.WorkspaceName, restored1.WorkspaceName)
	}
	if len(restored1.Notifications) != 1 {
		t.Errorf("expected 1 notification, got %d", len(restored1.Notifications))
	}
	if restored1.Notifications[0].Trigger != run1.Notifications[0].Trigger {
		t.Errorf("expected trigger %s, got %s", run1.Notifications[0].Trigger, restored1.Notifications[0].Trigger)
	}
}

func TestRunCache_RestoreInvalidJSON(t *testing.T) {
	cache := NewCache()

	// Test with invalid JSON
	invalidJSON := `{"invalid": json}`
	reader := strings.NewReader(invalidJSON)
	err := cache.Restore(reader)
	if err == nil {
		t.Error("expected error when restoring invalid JSON, got nil")
	}
}

func TestRunCache_ConcurrentAccess(t *testing.T) {
	cache := NewCache()

	// Test concurrent writes and reads
	done := make(chan bool, 2)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			run := &Run{
				ID:          "run-writer",
				WorkspaceID: "ws-writer",
			}
			cache.Set("ws-writer", run)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			cache.Get("ws-writer")
			cache.List()
		}
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// If we reach here without panic, concurrent access works
}

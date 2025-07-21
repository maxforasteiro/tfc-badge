package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHookRunner(t *testing.T) {
	hr := NewHookRunner()
	if hr.Fn == nil {
		t.Fatal("NewHookRunner returned nil Fn map")
	}
	if len(hr.Fn) != 0 {
		t.Errorf("expected empty Fn map, got %d entries", len(hr.Fn))
	}
}

func TestHookRunner_Hook(t *testing.T) {
	hr := NewHookRunner()

	// Test with no hooks
	hook := hr.Hook()
	run := &Run{ID: "test-run"}
	err := hook(run)
	if err != nil {
		t.Errorf("expected no error with empty hooks, got %v", err)
	}

	// Test with successful hooks
	var called1, called2 bool
	hr.Fn["hook1"] = func(r *Run) error {
		called1 = true
		if r.ID != "test-run" {
			return fmt.Errorf("unexpected run ID: %s", r.ID)
		}
		return nil
	}
	hr.Fn["hook2"] = func(r *Run) error {
		called2 = true
		return nil
	}

	hook = hr.Hook()
	err = hook(run)
	if err != nil {
		t.Errorf("expected no error with successful hooks, got %v", err)
	}
	if !called1 || !called2 {
		t.Error("not all hooks were called")
	}

	// Test with failing hook
	hr.Fn["failing_hook"] = func(r *Run) error {
		return fmt.Errorf("hook failed")
	}

	hook = hr.Hook()
	err = hook(run)
	if err == nil {
		t.Error("expected error from failing hook, got nil")
	}
	expectedError := `error running hook "failing_hook" for run test-run: hook failed`
	if err.Error() != expectedError {
		t.Errorf("expected error message '%s', got '%s'", expectedError, err.Error())
	}
}

func TestGrafanaAnnotation(t *testing.T) {
	// Test with no notifications (should return without error)
	t.Run("no notifications", func(t *testing.T) {
		hookFn := GrafanaAnnotation("http://grafana.test", "test-key")
		run := &Run{
			ID:            "test-run",
			WorkspaceName: "test-workspace",
			Message:       "test message",
			Notifications: []Notification{},
		}
		err := hookFn(run)
		if err != nil {
			t.Errorf("expected no error with empty notifications, got %v", err)
		}
	})

	// Test with multiple notifications (should return without error)
	t.Run("multiple notifications", func(t *testing.T) {
		hookFn := GrafanaAnnotation("http://grafana.test", "test-key")
		run := &Run{
			ID:            "test-run",
			WorkspaceName: "test-workspace",
			Message:       "test message",
			Notifications: []Notification{
				{Message: "Run Created", Trigger: "run:created"},
				{Message: "Run Completed", Trigger: "run:completed"},
			},
		}
		err := hookFn(run)
		if err != nil {
			t.Errorf("expected no error with multiple notifications, got %v", err)
		}
	})

	// Test successful annotation creation
	t.Run("successful annotation", func(t *testing.T) {
		// Create test server
		var receivedPayload map[string]interface{}
		var receivedHeaders http.Header
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedHeaders = r.Header
			if r.URL.Path != "/api/annotations" {
				t.Errorf("expected path /api/annotations, got %s", r.URL.Path)
			}
			if r.Method != http.MethodPost {
				t.Errorf("expected POST method, got %s", r.Method)
			}

			err := json.NewDecoder(r.Body).Decode(&receivedPayload)
			if err != nil {
				t.Errorf("error decoding request body: %v", err)
			}

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		hookFn := GrafanaAnnotation(server.URL, "test-api-key")
		testTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		run := &Run{
			ID:            "test-run-123",
			WorkspaceName: "test-workspace",
			Message:       "Apply completed successfully",
			Notifications: []Notification{
				{
					Message:      "Run Completed",
					Trigger:      "run:completed",
					RunStatus:    "applied",
					RunUpdatedAt: testTime,
				},
			},
		}

		err := hookFn(run)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Check headers
		if receivedHeaders.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", receivedHeaders.Get("Content-Type"))
		}
		expectedAuth := "Bearer test-api-key"
		if receivedHeaders.Get("Authorization") != expectedAuth {
			t.Errorf("expected Authorization %s, got %s", expectedAuth, receivedHeaders.Get("Authorization"))
		}

		// Check payload
		if receivedPayload == nil {
			t.Fatal("no payload received")
		}

		expectedTime := testTime.UnixMilli()
		if int64(receivedPayload["time"].(float64)) != expectedTime {
			t.Errorf("expected time %d, got %f", expectedTime, receivedPayload["time"])
		}

		expectedText := `applied workspace "test-workspace": "Apply completed successfully"`
		if receivedPayload["text"] != expectedText {
			t.Errorf("expected text %s, got %s", expectedText, receivedPayload["text"])
		}

		tags, ok := receivedPayload["tags"].([]interface{})
		if !ok {
			t.Fatal("tags field is not an array")
		}

		expectedTags := []string{
			"tfc-badge",
			"status:applied",
			"workspace:test-workspace",
		}

		if len(tags) != len(expectedTags) {
			t.Errorf("expected %d tags, got %d", len(expectedTags), len(tags))
		}

		for i, tag := range tags {
			if tag != expectedTags[i] {
				t.Errorf("expected tag %s, got %s", expectedTags[i], tag)
			}
		}
	})

	// Test HTTP error response
	t.Run("http error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		hookFn := GrafanaAnnotation(server.URL, "test-key")
		run := &Run{
			ID:            "test-run",
			WorkspaceName: "test-workspace",
			Message:       "test message",
			Notifications: []Notification{
				{
					Message:      "Run Created",
					Trigger:      "run:created",
					RunUpdatedAt: time.Now(),
				},
			},
		}

		err := hookFn(run)
		if err == nil {
			t.Error("expected error from HTTP 500 response, got nil")
		}
		expectedError := "http request unsucessful: 500 Internal Server Error"
		if err.Error() != expectedError {
			t.Errorf("expected error '%s', got '%s'", expectedError, err.Error())
		}
	})

	// Test invalid Grafana URL
	t.Run("invalid url", func(t *testing.T) {
		hookFn := GrafanaAnnotation("://invalid-url", "test-key")
		run := &Run{
			ID:            "test-run",
			WorkspaceName: "test-workspace",
			Message:       "test message",
			Notifications: []Notification{
				{
					Message:      "Run Created",
					Trigger:      "run:created",
					RunUpdatedAt: time.Now(),
				},
			},
		}

		err := hookFn(run)
		if err == nil {
			t.Error("expected error from invalid URL, got nil")
		}
	})
}

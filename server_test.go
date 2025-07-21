package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAppServer_Routes(t *testing.T) {
	server := &AppServer{
		Store: NewCache(),
		Debug: false,
		Addr:  ":8080",
	}

	handler := server.routes()

	testCases := []struct {
		path   string
		method string
		status int
	}{
		{"/", "GET", http.StatusOK},
		{"/", "POST", http.StatusOK},
		{"/badge/ws-123", "GET", http.StatusOK},
		{"/badge/ws-123", "POST", http.StatusMethodNotAllowed},
		{"/run", "POST", http.StatusBadRequest}, // Bad request due to empty body
		{"/run", "GET", http.StatusMethodNotAllowed},
		{"/nonexistent", "GET", http.StatusOK}, // Falls back to index handler
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s %s", tc.method, tc.path), func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tc.status {
				t.Errorf("expected status %d, got %d", tc.status, w.Code)
			}
		})
	}
}

func TestAppServer_HandleIndex(t *testing.T) {
	server := &AppServer{}
	handler := server.handleIndex()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestAppServer_HandleRun(t *testing.T) {
	store := NewCache()
	var hookCalled bool
	var hookedRun *Run

	server := &AppServer{
		Store: store,
		Debug: true,
		Hook: func(r *Run) error {
			hookCalled = true
			hookedRun = r
			return nil
		},
	}

	// Test valid run payload
	t.Run("valid run", func(t *testing.T) {
		run := Run{
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

		body, _ := json.Marshal(run)
		req := httptest.NewRequest("POST", "/run", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler := server.handleRun()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		// Check that run was stored
		storedRun, err := store.Get("ws-abc")
		if err != nil {
			t.Fatalf("error retrieving stored run: %v", err)
		}
		if storedRun.ID != "run-123" {
			t.Errorf("expected stored run ID run-123, got %s", storedRun.ID)
		}

		// Check that hook was called
		if !hookCalled {
			t.Error("expected hook to be called")
		}
		if hookedRun.ID != "run-123" {
			t.Errorf("expected hooked run ID run-123, got %s", hookedRun.ID)
		}
	})

	// Test invalid JSON
	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/run", strings.NewReader("{invalid json}"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler := server.handleRun()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	// Test empty workspace ID
	t.Run("empty workspace id", func(t *testing.T) {
		run := Run{
			ID:          "run-123",
			WorkspaceID: "", // Empty workspace ID
			Message:     "Test run",
		}

		body, _ := json.Marshal(run)
		req := httptest.NewRequest("POST", "/run", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler := server.handleRun()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		// Should not be stored due to empty workspace ID
		_, err := store.Get("")
		if err == nil {
			t.Error("expected error for empty workspace ID, but run was stored")
		}
	})

	// Test wrong HTTP method
	t.Run("wrong method", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/run", nil)
		w := httptest.NewRecorder()

		handler := server.handleRun()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})

	// Test hook error
	t.Run("hook error", func(t *testing.T) {
		serverWithFailingHook := &AppServer{
			Store: store,
			Debug: false,
			Hook: func(r *Run) error {
				return fmt.Errorf("hook failed")
			},
		}

		run := Run{
			ID:          "run-456",
			WorkspaceID: "ws-def",
			Message:     "Test run",
		}

		body, _ := json.Marshal(run)
		req := httptest.NewRequest("POST", "/run", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler := serverWithFailingHook.handleRun()
		handler.ServeHTTP(w, req)

		// Should still return 200 even if hook fails
		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		// Run should still be stored
		storedRun, err := store.Get("ws-def")
		if err != nil {
			t.Errorf("expected run to be stored despite hook failure: %v", err)
		}
		if storedRun.ID != "run-456" {
			t.Errorf("expected stored run ID run-456, got %s", storedRun.ID)
		}
	})
}

func TestAppServer_HandleBadge(t *testing.T) {
	store := NewCache()
	server := &AppServer{
		Store: store,
	}

	// Test badge for existing workspace
	t.Run("existing workspace", func(t *testing.T) {
		// Store a run
		run := &Run{
			ID:      "run-123",
			RunURL:  "https://app.terraform.io/run-123",
			Message: "Apply completed",
			Notifications: []Notification{
				{
					Message: "Applied Successfully",
					Trigger: "run:completed",
				},
			},
		}
		store.Set("ws-abc", run)

		req := httptest.NewRequest("GET", "/badge/ws-abc", nil)
		w := httptest.NewRecorder()

		handler := server.handleBadge()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		// Check content type
		contentType := w.Header().Get("Content-Type")
		if contentType != "image/svg+xml" {
			t.Errorf("expected content type image/svg+xml, got %s", contentType)
		}

		// Check cache headers
		cacheControl := w.Header().Get("Cache-Control")
		if cacheControl != "max-age=0, no-cache, no-store, must-revalidate" {
			t.Errorf("unexpected cache control header: %s", cacheControl)
		}

		// Check that it's SVG
		body := w.Body.String()
		if !strings.Contains(body, "<svg") {
			t.Error("response should contain SVG")
		}
		if !strings.Contains(body, "Applied Successfully") {
			t.Error("response should contain run message")
		}
	})

	// Test badge for non-existing workspace
	t.Run("non-existing workspace", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/badge/non-existing", nil)
		w := httptest.NewRecorder()

		handler := server.handleBadge()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		// Should return default badge
		body := w.Body.String()
		if !strings.Contains(body, "unknown") {
			t.Error("response should contain default 'unknown' message")
		}
	})

	// Test wrong HTTP method
	t.Run("wrong method", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/badge/ws-abc", nil)
		w := httptest.NewRecorder()

		handler := server.handleBadge()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})
}

func TestAppServer_Lifecycle(t *testing.T) {
	server := &AppServer{
		Store: NewCache(),
		Addr:  ":0", // Use any available port
	}

	// Test initialization
	if server.initialized {
		t.Error("server should not be initialized initially")
	}

	server.init()

	if !server.initialized {
		t.Error("server should be initialized after init()")
	}
	if server.server == nil {
		t.Error("http.Server should be set after init()")
	}

	// Test shutdown
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	if err != nil {
		t.Errorf("unexpected error during shutdown: %v", err)
	}
}

func TestAppServer_DebugLogging(t *testing.T) {
	store := NewCache()
	server := &AppServer{
		Store: store,
		Debug: true, // Enable debug logging
	}

	run := Run{
		ID:          "run-123",
		WorkspaceID: "ws-abc",
		Message:     "Test run",
	}

	body, _ := json.Marshal(run)
	req := httptest.NewRequest("POST", "/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler := server.handleRun()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// In debug mode, the body should be logged
	// This test mainly ensures no panic occurs with debug logging
}

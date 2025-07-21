package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRun_UnmarshalJSON(t *testing.T) {
	// Test with real Terraform Cloud webhook payload structure
	jsonPayload := `{
		"payload_version": 1,
		"notification_configuration_id": "nc-VQ6mp8VceWRjUPU8",
		"run_url": "https://app.terraform.io/app/sumup/cloudflare-sumup_com/runs/run-F6a5598jQoT23MGH",
		"run_id": "run-F6a5598jQoT23MGH",
		"run_message": "[WBPL-447] Set response headers for static.sumup.com (#1026)",
		"run_created_at": "2025-07-21T08:20:19.000Z",
		"run_created_by": "connor-baer",
		"workspace_id": "ws-XANEgSdLDk5Lxs9e",
		"workspace_name": "cloudflare-sumup_com",
		"organization_name": "sumup",
		"notifications": [
			{
				"message": "Run Created",
				"trigger": "run:created",
				"run_status": "pending",
				"run_updated_at": "2025-07-21T08:20:19.000Z",
				"run_updated_by": "connor-baer"
			}
		]
	}`

	var run Run
	err := json.Unmarshal([]byte(jsonPayload), &run)
	if err != nil {
		t.Fatalf("error unmarshaling run: %v", err)
	}

	// Verify fields
	if run.ID != "run-F6a5598jQoT23MGH" {
		t.Errorf("expected ID run-F6a5598jQoT23MGH, got %s", run.ID)
	}
	if run.PayloadVersion != 1 {
		t.Errorf("expected PayloadVersion 1, got %d", run.PayloadVersion)
	}
	if run.RunURL != "https://app.terraform.io/app/sumup/cloudflare-sumup_com/runs/run-F6a5598jQoT23MGH" {
		t.Errorf("unexpected RunURL: %s", run.RunURL)
	}
	if run.WorkspaceID != "ws-XANEgSdLDk5Lxs9e" {
		t.Errorf("expected WorkspaceID ws-XANEgSdLDk5Lxs9e, got %s", run.WorkspaceID)
	}
	if run.WorkspaceName != "cloudflare-sumup_com" {
		t.Errorf("expected WorkspaceName cloudflare-sumup_com, got %s", run.WorkspaceName)
	}
	if run.Message != "[WBPL-447] Set response headers for static.sumup.com (#1026)" {
		t.Errorf("unexpected Message: %s", run.Message)
	}
	if run.CreatedBy != "connor-baer" {
		t.Errorf("expected CreatedBy connor-baer, got %s", run.CreatedBy)
	}

	// Check notifications
	if len(run.Notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(run.Notifications))
	}

	notification := run.Notifications[0]
	if notification.Message != "Run Created" {
		t.Errorf("expected notification message 'Run Created', got %s", notification.Message)
	}
	if notification.Trigger != "run:created" {
		t.Errorf("expected trigger run:created, got %s", notification.Trigger)
	}
	if notification.RunStatus != "pending" {
		t.Errorf("expected run status pending, got %s", notification.RunStatus)
	}

	// Check time parsing
	expectedTime := time.Date(2025, 7, 21, 8, 20, 19, 0, time.UTC)
	if !run.CreatedAt.Equal(expectedTime) {
		t.Errorf("expected CreatedAt %v, got %v", expectedTime, run.CreatedAt)
	}
	if !notification.RunUpdatedAt.Equal(expectedTime) {
		t.Errorf("expected RunUpdatedAt %v, got %v", expectedTime, notification.RunUpdatedAt)
	}
}

func TestRun_MarshalJSON(t *testing.T) {
	// Test marshaling a Run struct back to JSON
	run := Run{
		ID:             "run-test-marshal",
		PayloadVersion: 1,
		RunURL:         "https://app.terraform.io/run-test-marshal",
		WorkspaceID:    "ws-test",
		WorkspaceName:  "test-workspace",
		Message:        "Test marshaling",
		CreatedAt:      time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		CreatedBy:      "test-user",
		Notifications: []Notification{
			{
				Message:      "Test Notification",
				Trigger:      "run:completed",
				RunStatus:    "applied",
				RunUpdatedAt: time.Date(2023, 1, 1, 12, 5, 0, 0, time.UTC),
			},
		},
	}

	data, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("error marshaling run: %v", err)
	}

	jsonString := string(data)

	// Check that key fields are present in JSON
	expectedFields := []string{
		`"run_id":"run-test-marshal"`,
		`"workspace_id":"ws-test"`,
		`"workspace_name":"test-workspace"`,
		`"run_message":"Test marshaling"`,
		`"run_created_by":"test-user"`,
		`"payload_version":1`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(jsonString, field) {
			t.Errorf("marshaled JSON missing expected field: %s", field)
		}
	}

	// Verify notifications array
	if !strings.Contains(jsonString, `"notifications":[{`) {
		t.Error("marshaled JSON missing notifications array")
	}
	if !strings.Contains(jsonString, `"trigger":"run:completed"`) {
		t.Error("marshaled JSON missing notification trigger")
	}
}

func TestNotification_Fields(t *testing.T) {
	// Test individual notification field handling
	notification := Notification{
		Message:      "Applied Successfully",
		Trigger:      "run:completed",
		RunStatus:    "applied",
		RunUpdatedAt: time.Date(2023, 5, 15, 14, 30, 0, 0, time.UTC),
	}

	// Test marshaling
	data, err := json.Marshal(notification)
	if err != nil {
		t.Fatalf("error marshaling notification: %v", err)
	}

	// Test unmarshaling
	var unmarshaledNotification Notification
	err = json.Unmarshal(data, &unmarshaledNotification)
	if err != nil {
		t.Fatalf("error unmarshaling notification: %v", err)
	}

	// Verify all fields
	if unmarshaledNotification.Message != notification.Message {
		t.Errorf("expected message %s, got %s", notification.Message, unmarshaledNotification.Message)
	}
	if unmarshaledNotification.Trigger != notification.Trigger {
		t.Errorf("expected trigger %s, got %s", notification.Trigger, unmarshaledNotification.Trigger)
	}
	if unmarshaledNotification.RunStatus != notification.RunStatus {
		t.Errorf("expected run status %s, got %s", notification.RunStatus, unmarshaledNotification.RunStatus)
	}
	if !unmarshaledNotification.RunUpdatedAt.Equal(notification.RunUpdatedAt) {
		t.Errorf("expected run updated at %v, got %v", notification.RunUpdatedAt, unmarshaledNotification.RunUpdatedAt)
	}
}

func TestRun_EmptyNotifications(t *testing.T) {
	// Test handling of runs with no notifications
	jsonPayload := `{
		"run_id": "run-no-notifications",
		"workspace_id": "ws-test",
		"workspace_name": "test-workspace",
		"run_message": "Test run without notifications",
		"run_created_at": "2023-01-01T12:00:00.000Z",
		"run_created_by": "test-user",
		"notifications": []
	}`

	var run Run
	err := json.Unmarshal([]byte(jsonPayload), &run)
	if err != nil {
		t.Fatalf("error unmarshaling run with empty notifications: %v", err)
	}

	if len(run.Notifications) != 0 {
		t.Errorf("expected 0 notifications, got %d", len(run.Notifications))
	}
}

func TestRun_MultipleNotifications(t *testing.T) {
	// Test handling of runs with multiple notifications
	jsonPayload := `{
		"run_id": "run-multi-notifications",
		"workspace_id": "ws-test",
		"workspace_name": "test-workspace",
		"run_message": "Test run with multiple notifications",
		"run_created_at": "2023-01-01T12:00:00.000Z",
		"run_created_by": "test-user",
		"notifications": [
			{
				"message": "Run Created",
				"trigger": "run:created",
				"run_status": "pending",
				"run_updated_at": "2023-01-01T12:00:00.000Z"
			},
			{
				"message": "Planning",
				"trigger": "run:planning",
				"run_status": "planning",
				"run_updated_at": "2023-01-01T12:01:00.000Z"
			},
			{
				"message": "Applied Successfully",
				"trigger": "run:completed",
				"run_status": "applied",
				"run_updated_at": "2023-01-01T12:05:00.000Z"
			}
		]
	}`

	var run Run
	err := json.Unmarshal([]byte(jsonPayload), &run)
	if err != nil {
		t.Fatalf("error unmarshaling run with multiple notifications: %v", err)
	}

	if len(run.Notifications) != 3 {
		t.Errorf("expected 3 notifications, got %d", len(run.Notifications))
	}

	// Check progression of status
	expectedTriggers := []string{"run:created", "run:planning", "run:completed"}
	expectedStatuses := []string{"pending", "planning", "applied"}

	for i, notification := range run.Notifications {
		if notification.Trigger != expectedTriggers[i] {
			t.Errorf("expected trigger %s at index %d, got %s", expectedTriggers[i], i, notification.Trigger)
		}
		if notification.RunStatus != expectedStatuses[i] {
			t.Errorf("expected status %s at index %d, got %s", expectedStatuses[i], i, notification.RunStatus)
		}
	}
}

func TestRun_InvalidJSON(t *testing.T) {
	// Test handling of malformed JSON
	invalidPayloads := []string{
		`{"run_id": "test", "invalid": json}`,
		`{"run_id": "test", "run_created_at": "invalid-time"}`,
		`{"run_id": "test", "notifications": [{"run_updated_at": "invalid-time"}]}`,
		`{"run_id": "test", "payload_version": "not-a-number"}`,
	}

	for i, payload := range invalidPayloads {
		var run Run
		err := json.Unmarshal([]byte(payload), &run)
		if err == nil {
			t.Errorf("expected error for invalid payload %d, got nil", i)
		}
	}
}

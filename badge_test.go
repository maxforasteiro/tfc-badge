package main

import (
	"strings"
	"testing"
)

func TestDefaultBadge(t *testing.T) {
	badge := DefaultBadge
	if badge.Color != "#7B42BC" {
		t.Errorf("expected default color #7B42BC, got %s", badge.Color)
	}
	if badge.Message != "unknown" {
		t.Errorf("expected default message 'unknown', got %s", badge.Message)
	}
	if badge.URL != "https://www.terraform.io/cloud" {
		t.Errorf("expected default URL 'https://www.terraform.io/cloud', got %s", badge.URL)
	}
	if badge.Width != MessageDefaultWidth {
		t.Errorf("expected default width %d, got %d", MessageDefaultWidth, badge.Width)
	}
}

func TestBadge_FromRun(t *testing.T) {
	testCases := []struct {
		name          string
		trigger       string
		message       string
		runURL        string
		expectedColor string
	}{
		{
			name:          "run created",
			trigger:       "run:created",
			message:       "Run Created",
			runURL:        "https://app.terraform.io/run-123",
			expectedColor: "#7b42bc",
		},
		{
			name:          "run planning",
			trigger:       "run:planning",
			message:       "Plan Running",
			runURL:        "https://app.terraform.io/run-456",
			expectedColor: "#1563ff",
		},
		{
			name:          "run needs attention",
			trigger:       "run:needs_attention",
			message:       "Needs Attention",
			runURL:        "https://app.terraform.io/run-789",
			expectedColor: "#fa8f37",
		},
		{
			name:          "run applying",
			trigger:       "run:applying",
			message:       "Apply Running",
			runURL:        "https://app.terraform.io/run-abc",
			expectedColor: "#1563ff",
		},
		{
			name:          "run completed",
			trigger:       "run:completed",
			message:       "Applied Successfully",
			runURL:        "https://app.terraform.io/run-def",
			expectedColor: "#2eb039",
		},
		{
			name:          "run errored",
			trigger:       "run:errored",
			message:       "Apply Failed",
			runURL:        "https://app.terraform.io/run-ghi",
			expectedColor: "#c73445",
		},
		{
			name:          "unknown trigger",
			trigger:       "run:unknown",
			message:       "Unknown Status",
			runURL:        "https://app.terraform.io/run-jkl",
			expectedColor: DefaultBadge.Color,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run := &Run{
				ID:      "test-run",
				RunURL:  tc.runURL,
				Message: tc.message,
				Notifications: []Notification{
					{
						Message: tc.message,
						Trigger: tc.trigger,
					},
				},
			}

			var badge Badge
			badge.FromRun(run)

			if badge.Color != tc.expectedColor {
				t.Errorf("expected color %s, got %s", tc.expectedColor, badge.Color)
			}
			if badge.Message != tc.message {
				t.Errorf("expected message %s, got %s", tc.message, badge.Message)
			}
			if badge.URL != tc.runURL {
				t.Errorf("expected URL %s, got %s", tc.runURL, badge.URL)
			}
			if badge.Width < MessageDefaultWidth {
				t.Errorf("expected width to be at least %d, got %d", MessageDefaultWidth, badge.Width)
			}
		})
	}
}

func TestBadge_Render(t *testing.T) {
	badge := Badge{
		Color:   "#2eb039",
		URL:     "https://app.terraform.io/run-123",
		Message: "Applied Successfully",
		Width:   200,
	}

	var buf strings.Builder
	err := badge.Render(&buf)
	if err != nil {
		t.Fatalf("unexpected error rendering badge: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("render produced empty output")
	}

	// Check that SVG contains expected elements
	expectedElements := []string{
		"<svg",
		badge.Color,
		badge.Message,
		badge.URL,
		"Terraform",
	}

	for _, element := range expectedElements {
		if !strings.Contains(output, element) {
			t.Errorf("expected SVG to contain %s", element)
		}
	}

	// Verify it's valid XML structure
	if !strings.HasPrefix(output, "<svg") {
		t.Error("output should start with <svg tag")
	}
	if !strings.HasSuffix(output, "</svg>\n") {
		t.Error("output should end with </svg> tag")
	}
}

func TestGetWidthUTF8String(t *testing.T) {
	testCases := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"hello", 5},
		{"Hello World", 11},
		{"こんにちは", 10}, // Japanese characters (wide)
		{"Test 123", 8},
		{"Mixed こんにちは text", 21}, // Mix of wide and narrow characters
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := GetWidthUTF8String(tc.input)
			if result != tc.expected {
				t.Errorf("expected width %d for '%s', got %d", tc.expected, tc.input, result)
			}
		})
	}
}

func TestBadge_WidthCalculation(t *testing.T) {
	// Test that width is calculated correctly based on message length
	shortMessage := "OK"
	longMessage := "This is a very long status message that should result in a wider badge"

	run1 := &Run{
		RunURL: "https://app.terraform.io/run-1",
		Notifications: []Notification{
			{Message: shortMessage, Trigger: "run:completed"},
		},
	}

	run2 := &Run{
		RunURL: "https://app.terraform.io/run-2",
		Notifications: []Notification{
			{Message: longMessage, Trigger: "run:completed"},
		},
	}

	var badge1, badge2 Badge
	badge1.FromRun(run1)
	badge2.FromRun(run2)

	// Long message should result in wider badge
	if badge2.Width <= badge1.Width {
		t.Errorf("expected badge2 width (%d) to be greater than badge1 width (%d)", badge2.Width, badge1.Width)
	}

	// Both should be at least the minimum width
	if badge1.Width < MessageDefaultWidth {
		t.Errorf("badge1 width (%d) should be at least minimum width (%d)", badge1.Width, MessageDefaultWidth)
	}
	if badge2.Width < MessageDefaultWidth {
		t.Errorf("badge2 width (%d) should be at least minimum width (%d)", badge2.Width, MessageDefaultWidth)
	}
}

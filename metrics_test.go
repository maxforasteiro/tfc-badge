package main

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestMetricsCollector_Describe(t *testing.T) {
	store := NewCache()
	collector := &MetricsCollector{Store: store}

	ch := make(chan *prometheus.Desc, 10)
	collector.Describe(ch)
	close(ch)

	// Count the descriptors
	count := 0
	for range ch {
		count++
	}

	if count == 0 {
		t.Error("expected at least one metric descriptor")
	}
}

func TestMetricsCollector_Collect(t *testing.T) {
	store := NewCache()
	collector := &MetricsCollector{Store: store}

	// Test with empty cache
	t.Run("empty cache", func(t *testing.T) {
		ch := make(chan prometheus.Metric, 10)
		collector.Collect(ch)
		close(ch)

		count := 0
		for range ch {
			count++
		}

		if count != 0 {
			t.Errorf("expected no metrics with empty cache, got %d", count)
		}
	})

	// Test with runs in cache
	t.Run("with runs", func(t *testing.T) {
		// Add test data
		run1 := &Run{
			ID:            "run-123",
			WorkspaceID:   "ws-abc",
			WorkspaceName: "workspace-1",
			CreatedAt:     time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			Notifications: []Notification{
				{
					Message:      "Applied Successfully",
					Trigger:      "run:completed",
					RunStatus:    "applied",
					RunUpdatedAt: time.Date(2023, 1, 1, 12, 5, 0, 0, time.UTC),
				},
			},
		}

		run2 := &Run{
			ID:            "run-456",
			WorkspaceID:   "ws-def",
			WorkspaceName: "workspace-2",
			CreatedAt:     time.Date(2023, 1, 2, 10, 0, 0, 0, time.UTC),
			Notifications: []Notification{
				{
					Message:      "Planning",
					Trigger:      "run:planning",
					RunStatus:    "planning",
					RunUpdatedAt: time.Date(2023, 1, 2, 10, 2, 0, 0, time.UTC),
				},
			},
		}

		store.Set("ws-abc", run1)
		store.Set("ws-def", run2)

		ch := make(chan prometheus.Metric, 20)
		collector.Collect(ch)
		close(ch)

		metrics := make([]prometheus.Metric, 0)
		for m := range ch {
			metrics = append(metrics, m)
		}

		// Should have 2 metrics per run (state and duration)
		expectedCount := 4
		if len(metrics) != expectedCount {
			t.Errorf("expected %d metrics, got %d", expectedCount, len(metrics))
		}

		// Test metric values and labels
		for _, metric := range metrics {
			dto := &dto.Metric{}
			err := metric.Write(dto)
			if err != nil {
				t.Errorf("error writing metric: %v", err)
			}

			// Check that labels are present
			labels := make(map[string]string)
			for _, label := range dto.Label {
				labels[label.GetName()] = label.GetValue()
			}

			requiredLabels := []string{"workspace_id", "workspace_name", "trigger", "status", "run_id"}
			for _, requiredLabel := range requiredLabels {
				if _, exists := labels[requiredLabel]; !exists {
					t.Errorf("missing required label: %s", requiredLabel)
				}
			}

			// Verify specific values for run1
			if labels["run_id"] == "run-123" {
				if labels["workspace_id"] != "ws-abc" {
					t.Errorf("expected workspace_id ws-abc, got %s", labels["workspace_id"])
				}
				if labels["workspace_name"] != "workspace-1" {
					t.Errorf("expected workspace_name workspace-1, got %s", labels["workspace_name"])
				}
				if labels["trigger"] != "run:completed" {
					t.Errorf("expected trigger run:completed, got %s", labels["trigger"])
				}
				if labels["status"] != "applied" {
					t.Errorf("expected status applied, got %s", labels["status"])
				}
			}
		}
	})
}

func TestMetricsCollector_MetricValues(t *testing.T) {
	store := NewCache()
	collector := &MetricsCollector{Store: store}

	// Create a run with known timestamps
	createdAt := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2023, 1, 1, 12, 5, 30, 0, time.UTC)

	run := &Run{
		ID:            "run-123",
		WorkspaceID:   "ws-abc",
		WorkspaceName: "test-workspace",
		CreatedAt:     createdAt,
		Notifications: []Notification{
			{
				Message:      "Applied Successfully",
				Trigger:      "run:completed",
				RunStatus:    "applied",
				RunUpdatedAt: updatedAt,
			},
		},
	}

	store.Set("ws-abc", run)

	ch := make(chan prometheus.Metric, 10)
	collector.Collect(ch)
	close(ch)

	var stateMetric, durationMetric prometheus.Metric
	for metric := range ch {
		desc := metric.Desc()
		if strings.Contains(desc.String(), "state_timestamp") {
			stateMetric = metric
		} else if strings.Contains(desc.String(), "runs_seconds") {
			durationMetric = metric
		}
	}

	if stateMetric == nil {
		t.Fatal("state timestamp metric not found")
	}
	if durationMetric == nil {
		t.Fatal("duration metric not found")
	}

	// Check state metric value
	stateDTO := &dto.Metric{}
	err := stateMetric.Write(stateDTO)
	if err != nil {
		t.Fatalf("error writing state metric: %v", err)
	}

	expectedStateValue := float64(updatedAt.Unix())
	if stateDTO.Counter.GetValue() != expectedStateValue {
		t.Errorf("expected state metric value %f, got %f", expectedStateValue, stateDTO.Counter.GetValue())
	}

	// Check duration metric value
	durationDTO := &dto.Metric{}
	err = durationMetric.Write(durationDTO)
	if err != nil {
		t.Fatalf("error writing duration metric: %v", err)
	}

	expectedDuration := float64(updatedAt.Unix() - createdAt.Unix())
	if durationDTO.Counter.GetValue() != expectedDuration {
		t.Errorf("expected duration metric value %f, got %f", expectedDuration, durationDTO.Counter.GetValue())
	}
}

func TestMetricsCollector_Integration(t *testing.T) {
	// Test that the collector can be registered with Prometheus
	store := NewCache()
	collector := &MetricsCollector{Store: store}

	registry := prometheus.NewRegistry()
	err := registry.Register(collector)
	if err != nil {
		t.Errorf("error registering collector: %v", err)
	}

	// Add some test data
	run := &Run{
		ID:            "run-integration",
		WorkspaceID:   "ws-integration",
		WorkspaceName: "integration-workspace",
		CreatedAt:     time.Now().Add(-5 * time.Minute),
		Notifications: []Notification{
			{
				Message:      "Plan Completed",
				Trigger:      "run:planned",
				RunStatus:    "planned",
				RunUpdatedAt: time.Now(),
			},
		},
	}

	store.Set("ws-integration", run)

	// Gather metrics
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Errorf("error gathering metrics: %v", err)
	}

	if len(metricFamilies) == 0 {
		t.Error("expected metric families, got none")
	}

	// Check that our metrics are present
	var foundState, foundDuration bool
	for _, family := range metricFamilies {
		if strings.Contains(family.GetName(), "state_timestamp") {
			foundState = true
		}
		if strings.Contains(family.GetName(), "runs_seconds") {
			foundDuration = true
		}
	}

	if !foundState {
		t.Error("state timestamp metric family not found")
	}
	if !foundDuration {
		t.Error("duration metric family not found")
	}
}

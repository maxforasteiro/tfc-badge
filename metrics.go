package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	runStateDesc = prometheus.NewDesc(
		"tfcbadge_run_state_timestamp",
		"Current state of runs.",
		[]string{"workspace_id", "workspace_name", "trigger", "status"}, nil)

	runsDesc = prometheus.NewDesc(
		"tfcbadge_runs_seconds",
		"Duration of runs in seconds from created_at to notifications.updated_at.",
		[]string{"workspace_id", "workspace_name", "trigger", "status"}, nil)
)

type MetricsCollector struct {
	Store *RunCache
}

func (c *MetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- runStateDesc
	ch <- runsDesc
}

func (c *MetricsCollector) Collect(ch chan<- prometheus.Metric) {
	runs := c.Store.List()
	for _, run := range runs {
		if len(run.Notifications) == 0 {
			continue
		}
		notification := run.Notifications[0]
		ch <- prometheus.MustNewConstMetric(
			runStateDesc,
			prometheus.CounterValue,
			float64(notification.RunUpdatedAt.Unix()),
			run.WorkspaceID,
			run.WorkspaceName,
			notification.Trigger,
			notification.RunStatus,
		)

		ch <- prometheus.MustNewConstMetric(
			runsDesc,
			prometheus.CounterValue,
			float64(notification.RunUpdatedAt.Unix()-run.CreatedAt.Unix()),
			run.WorkspaceID,
			run.WorkspaceName,
			notification.Trigger,
			notification.RunStatus,
		)
	}
}
